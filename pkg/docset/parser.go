package docset

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Docset represents a parsed docset
type Docset struct {
	Path     string
	Info     DocsetInfo
	Entries  []DocsetEntry
	BasePath string // Path to the Documents directory
}

// DocsetInfo represents the metadata from Info.plist
type DocsetInfo struct {
	ID                  string `plist:"CFBundleIdentifier"`
	Name                string `plist:"CFBundleName"`
	Family              string `plist:"DocSetPlatformFamily"`
	Version             string `plist:"DashDocSetVersion"`
	FallbackURL         string `plist:"DashDocSetFallbackURL"`
	DownloadURL         string `plist:"DashDocSetDownloadURL"`
	FeedURL             string `plist:"DashDocSetFeedURL"`
	IsJavaScriptEnabled bool   `plist:"isJavaScriptEnabled"`
	IsDashDocset        bool   `plist:"isDashDocset"`
}

// DocsetEntry represents an entry in the docset index
type DocsetEntry struct {
	Name string
	Type string
	Path string
}

// Parser handles parsing of docset files
type Parser struct{}

// NewParser creates a new docset parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a docset directory and returns the structured data
func (p *Parser) Parse(docsetPath string) (*Docset, error) {
	// Verify the docset exists
	if _, err := os.Stat(docsetPath); err != nil {
		return nil, fmt.Errorf("docset not found: %w", err)
	}

	docset := &Docset{
		Path: docsetPath,
	}

	// Parse Info.plist
	infoPlistPath := filepath.Join(docsetPath, "Contents", "Info.plist")
	if err := p.parseInfoPlist(infoPlistPath, &docset.Info); err != nil {
		return nil, fmt.Errorf("failed to parse Info.plist: %w", err)
	}

	// Set the base path for documents
	docset.BasePath = filepath.Join(docsetPath, "Contents", "Resources", "Documents")

	// Parse SQLite index
	indexPath := filepath.Join(docsetPath, "Contents", "Resources", "docSet.dsidx")
	entries, err := p.parseIndex(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse index: %w", err)
	}
	docset.Entries = entries

	return docset, nil
}

// parseInfoPlist parses the Info.plist file
func (p *Parser) parseInfoPlist(path string, info *DocsetInfo) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Parse as property list (simple XML parsing for now)
	// In a real implementation, we'd use a proper plist parser
	// For now, we'll extract key fields manually
	type plistDict struct {
		XMLName xml.Name `xml:"dict"`
		Items   []string `xml:",any"`
	}

	type plist struct {
		XMLName xml.Name  `xml:"plist"`
		Dict    plistDict `xml:"dict"`
	}

	var pl plist
	if err := xml.Unmarshal(data, &pl); err != nil {
		return fmt.Errorf("failed to parse plist: %w", err)
	}

	// Extract key-value pairs from the plist
	// This is a simplified parser - a proper implementation would use howett.net/plist
	items := pl.Dict.Items
	for i := 0; i < len(items)-1; i += 2 {
		if i+1 >= len(items) {
			break
		}

		// Simple extraction of common fields
		// In production, use a proper plist library
		if contains(items[i], "CFBundleName") && contains(items[i+1], "string") {
			info.Name = extractStringValue(items[i+1])
		} else if contains(items[i], "CFBundleIdentifier") && contains(items[i+1], "string") {
			info.ID = extractStringValue(items[i+1])
		} else if contains(items[i], "DocSetPlatformFamily") && contains(items[i+1], "string") {
			info.Family = extractStringValue(items[i+1])
		}
	}

	// Default values if not found
	if info.Name == "" {
		info.Name = filepath.Base(path)
		info.Name = info.Name[:len(info.Name)-len(filepath.Ext(info.Name))]
	}

	return nil
}

// parseIndex parses the SQLite index file
func (p *Parser) parseIndex(indexPath string) ([]DocsetEntry, error) {
	db, err := sql.Open("sqlite3", indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}
	defer db.Close()

	// Query the searchIndex table
	rows, err := db.Query("SELECT name, type, path FROM searchIndex ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to query index: %w", err)
	}
	defer rows.Close()

	var entries []DocsetEntry
	for rows.Next() {
		var entry DocsetEntry
		if err := rows.Scan(&entry.Name, &entry.Type, &entry.Path); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return entries, nil
}

// Helper functions for simple plist parsing
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr
}

func extractStringValue(s string) string {
	// Extract value between <string> tags
	start := "<string>"
	end := "</string>"

	startIdx := -1
	for i := 0; i <= len(s)-len(start); i++ {
		if s[i:i+len(start)] == start {
			startIdx = i + len(start)
			break
		}
	}

	if startIdx == -1 {
		return ""
	}

	endIdx := -1
	for i := startIdx; i <= len(s)-len(end); i++ {
		if s[i:i+len(end)] == end {
			endIdx = i
			break
		}
	}

	if endIdx == -1 || endIdx <= startIdx {
		return ""
	}

	return s[startIdx:endIdx]
}
