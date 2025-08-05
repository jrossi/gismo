package docset

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jrossi/gismo/pkg/knowledge"
)

// Importer handles importing docsets into the knowledge store
type Importer struct {
	store  *knowledge.Store
	parser *Parser
}

// NewImporter creates a new docset importer
func NewImporter(store *knowledge.Store) *Importer {
	return &Importer{
		store:  store,
		parser: NewParser(),
	}
}

// ImportProgress represents the progress of an import operation
type ImportProgress struct {
	Current     int
	Total       int
	CurrentItem string
	Message     string
}

// Import imports a docset from the given path with progress reporting
func (i *Importer) Import(ctx context.Context, docsetPath string, sourceURL string, sourceType string, progress func(ImportProgress)) error {
	// Parse the docset
	docset, err := i.parser.Parse(docsetPath)
	if err != nil {
		return fmt.Errorf("failed to parse docset: %w", err)
	}

	// Generate docset ID
	docsetID := generateDocsetID(docset.Info.Name, docset.Info.Version)

	// Begin transaction
	tx, err := i.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Check if docset already exists
	var exists bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM docsets WHERE id = ?)", docsetID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check docset existence: %w", err)
	}

	if exists {
		// Remove existing docset content
		_, err = tx.ExecContext(ctx, "DELETE FROM docset_content WHERE docset_id = ?", docsetID)
		if err != nil {
			return fmt.Errorf("failed to delete existing content: %w", err)
		}

		_, err = tx.ExecContext(ctx, "DELETE FROM docsets WHERE id = ?", docsetID)
		if err != nil {
			return fmt.Errorf("failed to delete existing docset: %w", err)
		}
	}

	// Insert docset metadata
	metadata := map[string]interface{}{
		"family":       docset.Info.Family,
		"fallbackURL":  docset.Info.FallbackURL,
		"downloadURL":  docset.Info.DownloadURL,
		"feedURL":      docset.Info.FeedURL,
		"isDashDocset": docset.Info.IsDashDocset,
	}
	metadataJSON, _ := json.Marshal(metadata)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO docsets (id, name, version, language, source_url, source_type, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, docsetID, docset.Info.Name, docset.Info.Version, inferLanguage(docset.Info.Name, docset.Info.Family), sourceURL, sourceType, string(metadataJSON))
	if err != nil {
		return fmt.Errorf("failed to insert docset: %w", err)
	}

	// Import entries
	total := len(docset.Entries)
	for idx, entry := range docset.Entries {
		if progress != nil {
			progress(ImportProgress{
				Current:     idx + 1,
				Total:       total,
				CurrentItem: entry.Name,
				Message:     fmt.Sprintf("Importing %s (%s)", entry.Name, entry.Type),
			})
		}

		// Read content if the file exists
		content, summary := i.readEntryContent(docset.BasePath, entry.Path)

		// For now, we'll skip embedding generation
		// In a real implementation, we'd generate embeddings here

		_, err = tx.ExecContext(ctx, `
			INSERT INTO docset_content (docset_id, name, type, path, content, summary)
			VALUES (?, ?, ?, ?, ?, ?)
		`, docsetID, entry.Name, entry.Type, entry.Path, content, summary)
		if err != nil {
			return fmt.Errorf("failed to insert entry %s: %w", entry.Name, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	if progress != nil {
		progress(ImportProgress{
			Current:     total,
			Total:       total,
			CurrentItem: "",
			Message:     "Import complete",
		})
	}

	return nil
}

// readEntryContent reads the content of a docset entry
func (i *Importer) readEntryContent(basePath, entryPath string) (content string, summary string) {
	// Construct full path
	fullPath := filepath.Join(basePath, entryPath)

	// Handle URL fragments (e.g., "index.html#section")
	if idx := strings.Index(fullPath, "#"); idx >= 0 {
		fullPath = fullPath[:idx]
	}

	// Read file content
	file, err := os.Open(fullPath)
	if err != nil {
		return "", ""
	}
	defer file.Close()

	// Limit content size (100KB)
	limitedReader := io.LimitReader(file, 100*1024)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", ""
	}

	content = string(data)

	// Generate a simple summary (first 200 chars of text content)
	// In a real implementation, we'd extract text from HTML
	summary = content
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}

	// Basic HTML stripping for summary
	summary = stripHTML(summary)

	return content, summary
}

// RemoveDocset removes a docset from the store
func (i *Importer) RemoveDocset(ctx context.Context, docsetID string) error {
	db := i.store.DB()

	// Begin transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Delete content first (foreign key constraint)
	result, err := tx.ExecContext(ctx, "DELETE FROM docset_content WHERE docset_id = ?", docsetID)
	if err != nil {
		return fmt.Errorf("failed to delete docset content: %w", err)
	}

	contentRows, _ := result.RowsAffected()

	// Delete docset
	result, err = tx.ExecContext(ctx, "DELETE FROM docsets WHERE id = ?", docsetID)
	if err != nil {
		return fmt.Errorf("failed to delete docset: %w", err)
	}

	docsetRows, _ := result.RowsAffected()
	if docsetRows == 0 {
		return sql.ErrNoRows
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Log deletion stats
	_ = contentRows // In production, we'd log this

	return nil
}

// ListDocsets returns all imported docsets
func (i *Importer) ListDocsets(ctx context.Context) ([]DocsetInfo, error) {
	rows, err := i.store.DB().QueryContext(ctx, `
		SELECT id, name, version, language, source_url, source_type, imported_at, metadata
		FROM docsets
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query docsets: %w", err)
	}
	defer rows.Close()

	var docsets []DocsetInfo
	for rows.Next() {
		var info DocsetInfo
		var metadata sql.NullString
		err := rows.Scan(&info.ID, &info.Name, &info.Version, &info.Family,
			&info.DownloadURL, &info.FeedURL, &info.Version, &metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		docsets = append(docsets, info)
	}

	return docsets, rows.Err()
}

// generateDocsetID creates a unique ID for a docset
func generateDocsetID(name, version string) string {
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, "/", "-")
	id = strings.ReplaceAll(id, ".", "-")

	if version != "" {
		id = fmt.Sprintf("%s-%s", id, version)
	}

	return id
}

// inferLanguage attempts to infer the programming language from docset info
func inferLanguage(name, family string) string {
	// Common language mappings
	langMap := map[string]string{
		"go":         "go",
		"golang":     "go",
		"python":     "python",
		"javascript": "javascript",
		"typescript": "typescript",
		"rust":       "rust",
		"java":       "java",
		"c++":        "cpp",
		"cpp":        "cpp",
		"ruby":       "ruby",
		"swift":      "swift",
		"kotlin":     "kotlin",
		"php":        "php",
		"csharp":     "csharp",
		"c#":         "csharp",
	}

	// Check name first
	nameLower := strings.ToLower(name)
	for key, lang := range langMap {
		if strings.Contains(nameLower, key) {
			return lang
		}
	}

	// Check family
	familyLower := strings.ToLower(family)
	for key, lang := range langMap {
		if strings.Contains(familyLower, key) {
			return lang
		}
	}

	// Default
	return "unknown"
}
