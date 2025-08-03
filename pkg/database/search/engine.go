package search

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/anush008/fastembed-go"
	"github.com/jrossi/gismo/pkg/database"
)

type Engine struct {
	db        *database.DB
	embedder  *fastembed.FlagEmbedding
	projectID int
}

type CodeChunk struct {
	ID           int
	ProjectID    int
	FilePath     string // Relative path within project
	AbsolutePath string // Full file path
	Content      string
	ChunkType    string
	Language     string
	StartLine    int
	EndLine      int
	Metadata     map[string]interface{}
}

type SearchResult struct {
	CodeChunk
	ProjectName string
	ProjectPath string
	Similarity  float64
}

type SearchOptions struct {
	Limit     int
	Language  string
	ChunkType string
}

func NewEngine(db *database.DB) (*Engine, error) {
	return NewEngineWithOptions(db, true)
}

func NewEngineWithOptions(db *database.DB, initEmbedder bool) (*Engine, error) {
	engine := &Engine{
		db: db,
	}

	if initEmbedder {
		options := &fastembed.InitOptions{
			Model:     fastembed.BGESmallEN,
			CacheDir:  ".fastembed_cache",
			MaxLength: 512,
		}

		embedder, err := fastembed.NewFlagEmbedding(options)
		if err != nil {
			// In testing or environments without ONNX, we can work without embeddings
			log.Printf("Warning: Failed to initialize embedder: %v. Vector search will be disabled.", err)
		} else {
			engine.embedder = embedder
		}
	}

	return engine, nil
}

func NewEngineWithProject(db *database.DB, projectID int) (*Engine, error) {
	engine, err := NewEngine(db)
	if err != nil {
		return nil, err
	}
	engine.projectID = projectID
	return engine, nil
}

// SetProject sets the current project for the search engine
func (e *Engine) SetProject(projectID int) {
	e.projectID = projectID
}

// GetProject returns the current project ID
func (e *Engine) GetProject() int {
	return e.projectID
}

func (e *Engine) IndexCodeChunk(ctx context.Context, chunk CodeChunk) error {
	if e.projectID == 0 {
		return fmt.Errorf("project ID not set")
	}

	// Ensure chunk has the correct project ID
	chunk.ProjectID = e.projectID

	metadata, err := json.Marshal(chunk.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	dbChunk := &database.CodeChunk{
		ProjectID:    chunk.ProjectID,
		FilePath:     chunk.FilePath,
		AbsolutePath: chunk.AbsolutePath,
		Content:      chunk.Content,
		ChunkType:    chunk.ChunkType,
		Language:     chunk.Language,
		StartLine:    chunk.StartLine,
		EndLine:      chunk.EndLine,
		Metadata:     sql.NullString{String: string(metadata), Valid: true},
	}

	if e.embedder != nil {
		input := fmt.Sprintf("passage: %s", chunk.Content)
		embeddings, err := e.embedder.Embed([]string{input}, 1)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}
		dbChunk.Embedding = embeddings[0]
		_, err = e.db.InsertCodeChunk(ctx, dbChunk)
		return err
	}

	// Without embedder, insert without embedding
	_, err = e.db.InsertCodeChunkWithoutEmbedding(ctx, dbChunk)
	return err
}

func (e *Engine) IndexCodeChunksBatch(ctx context.Context, chunks []CodeChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Set project ID for all chunks
	for i := range chunks {
		chunks[i].ProjectID = e.projectID
	}

	if e.embedder != nil {
		var inputs []string
		for _, chunk := range chunks {
			inputs = append(inputs, fmt.Sprintf("passage: %s", chunk.Content))
		}

		embeddings, err := e.embedder.Embed(inputs, 32)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings: %w", err)
		}

		for i, chunk := range chunks {
			metadata, err := json.Marshal(chunk.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}

			dbChunk := &database.CodeChunk{
				ProjectID:    chunk.ProjectID,
				FilePath:     chunk.FilePath,
				AbsolutePath: chunk.AbsolutePath,
				Content:      chunk.Content,
				ChunkType:    chunk.ChunkType,
				Language:     chunk.Language,
				StartLine:    chunk.StartLine,
				EndLine:      chunk.EndLine,
				Embedding:    embeddings[i],
				Metadata:     sql.NullString{String: string(metadata), Valid: true},
			}

			if _, err = e.db.InsertCodeChunk(ctx, dbChunk); err != nil {
				return fmt.Errorf("failed to insert chunk: %w", err)
			}
		}
	} else {
		// Without embedder, insert without embeddings
		for _, chunk := range chunks {
			metadata, err := json.Marshal(chunk.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}

			dbChunk := &database.CodeChunk{
				ProjectID:    chunk.ProjectID,
				FilePath:     chunk.FilePath,
				AbsolutePath: chunk.AbsolutePath,
				Content:      chunk.Content,
				ChunkType:    chunk.ChunkType,
				Language:     chunk.Language,
				StartLine:    chunk.StartLine,
				EndLine:      chunk.EndLine,
				Metadata:     sql.NullString{String: string(metadata), Valid: true},
			}

			if _, err = e.db.InsertCodeChunkWithoutEmbedding(ctx, dbChunk); err != nil {
				return fmt.Errorf("failed to insert chunk: %w", err)
			}
		}
	}

	return nil
}

func (e *Engine) SearchCode(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if e.projectID == 0 {
		return nil, fmt.Errorf("project ID not set")
	}

	if opts.Limit == 0 {
		opts.Limit = 10
	}

	startTime := time.Now()
	var results []SearchResult

	if e.embedder != nil {
		// Generate embedding for the query
		input := fmt.Sprintf("query: %s", query)
		embeddings, err := e.embedder.Embed([]string{input}, 1)
		if err != nil {
			return nil, fmt.Errorf("failed to generate query embedding: %w", err)
		}

		// Convert embedding to DuckDB array format
		parts := make([]string, len(embeddings[0]))
		for i, v := range embeddings[0] {
			parts[i] = fmt.Sprintf("%f", v)
		}
		embeddingStr := "[" + strings.Join(parts, ", ") + "]"

		// Use DuckDB's array_cosine_similarity for vector search
		searchQuery := `
			SELECT 
				cc.id, cc.project_id, cc.file_path, cc.absolute_path, cc.content,
				cc.chunk_type, cc.language, cc.start_line, cc.end_line, cc.metadata,
				p.project_name, p.project_path,
				array_cosine_similarity(cc.embedding, $1::REAL[]) as similarity
			FROM code_chunks cc
			JOIN projects p ON cc.project_id = p.id
			WHERE cc.project_id = $2
				AND cc.embedding IS NOT NULL`

		args := []interface{}{embeddingStr, e.projectID}
		argIdx := 3

		if opts.Language != "" {
			searchQuery += fmt.Sprintf(" AND cc.language = $%d", argIdx)
			args = append(args, opts.Language)
			argIdx++
		}

		if opts.ChunkType != "" {
			searchQuery += fmt.Sprintf(" AND cc.chunk_type = $%d", argIdx)
			args = append(args, opts.ChunkType)
			argIdx++
		}

		searchQuery += fmt.Sprintf(" ORDER BY similarity DESC LIMIT $%d", argIdx)
		args = append(args, opts.Limit)

		rows, err := e.db.Conn().QueryContext(ctx, searchQuery, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to search code chunks: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var result SearchResult
			var metadataStr sql.NullString
			var similarity sql.NullFloat64

			err := rows.Scan(
				&result.ID,
				&result.ProjectID,
				&result.FilePath,
				&result.AbsolutePath,
				&result.Content,
				&result.ChunkType,
				&result.Language,
				&result.StartLine,
				&result.EndLine,
				&metadataStr,
				&result.ProjectName,
				&result.ProjectPath,
				&similarity,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to scan result: %w", err)
			}

			if metadataStr.Valid {
				_ = json.Unmarshal([]byte(metadataStr.String), &result.Metadata)
			}

			if similarity.Valid {
				result.Similarity = similarity.Float64
			}

			results = append(results, result)
		}
	} else {
		// Fallback to text search without embeddings
		searchQuery := `
			SELECT 
				cc.id, cc.project_id, cc.file_path, cc.absolute_path, cc.content,
				cc.chunk_type, cc.language, cc.start_line, cc.end_line, cc.metadata,
				p.project_name, p.project_path
			FROM code_chunks cc
			JOIN projects p ON cc.project_id = p.id
			WHERE cc.project_id = $1
				AND LOWER(cc.content) LIKE LOWER($2)`

		args := []interface{}{e.projectID, "%" + query + "%"}
		argIdx := 3

		if opts.Language != "" {
			searchQuery += fmt.Sprintf(" AND cc.language = $%d", argIdx)
			args = append(args, opts.Language)
			argIdx++
		}

		if opts.ChunkType != "" {
			searchQuery += fmt.Sprintf(" AND cc.chunk_type = $%d", argIdx)
			args = append(args, opts.ChunkType)
			argIdx++
		}

		searchQuery += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, opts.Limit)

		rows, err := e.db.Conn().QueryContext(ctx, searchQuery, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to search code chunks: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var result SearchResult
			var metadataStr sql.NullString

			err := rows.Scan(
				&result.ID,
				&result.ProjectID,
				&result.FilePath,
				&result.AbsolutePath,
				&result.Content,
				&result.ChunkType,
				&result.Language,
				&result.StartLine,
				&result.EndLine,
				&metadataStr,
				&result.ProjectName,
				&result.ProjectPath,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to scan result: %w", err)
			}

			if metadataStr.Valid {
				_ = json.Unmarshal([]byte(metadataStr.String), &result.Metadata)
			}

			results = append(results, result)
		}
	}

	// Record search history
	ms := time.Since(startTime).Milliseconds()
	if ms > math.MaxInt32 {
		ms = math.MaxInt32
	}
	execTimeMs := int32(ms) // #nosec G115 -- bounded check above
	_, _ = e.db.InsertSearchHistory(ctx, query, len(results), sql.NullString{}, sql.NullInt32{Int32: execTimeMs, Valid: true})

	return results, nil
}

func (e *Engine) GetCodeChunksByFile(ctx context.Context, filePath string) ([]CodeChunk, error) {
	if e.projectID == 0 {
		return nil, fmt.Errorf("project ID not set")
	}

	chunks, err := e.db.GetCodeChunksByFilePath(ctx, e.projectID, filePath)
	if err != nil {
		return nil, err
	}

	var result []CodeChunk
	for _, chunk := range chunks {
		cc := CodeChunk{
			ID:           chunk.ID,
			ProjectID:    chunk.ProjectID,
			FilePath:     chunk.FilePath,
			AbsolutePath: chunk.AbsolutePath,
			Content:      chunk.Content,
			ChunkType:    chunk.ChunkType,
			Language:     chunk.Language,
			StartLine:    chunk.StartLine,
			EndLine:      chunk.EndLine,
		}

		if chunk.Metadata.Valid {
			_ = json.Unmarshal([]byte(chunk.Metadata.String), &cc.Metadata)
		}

		result = append(result, cc)
	}

	return result, nil
}

func (e *Engine) DeleteChunksByFile(ctx context.Context, filePath string) error {
	if e.projectID == 0 {
		return fmt.Errorf("project ID not set")
	}

	return e.db.DeleteCodeChunksByFilePath(ctx, e.projectID, filePath)
}

func (e *Engine) DeleteAllChunks(ctx context.Context) error {
	if e.projectID == 0 {
		return fmt.Errorf("project ID not set")
	}

	return e.db.DeleteCodeChunksByProject(ctx, e.projectID)
}

func (e *Engine) GetStats(ctx context.Context) (map[string]interface{}, error) {
	if e.projectID == 0 {
		return nil, fmt.Errorf("project ID not set")
	}

	totalChunks, err := e.db.GetTotalChunksCount(ctx, e.projectID)
	if err != nil {
		return nil, err
	}

	fileCount, err := e.db.GetFileCount(ctx, e.projectID)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_chunks": totalChunks,
		"total_files":  fileCount,
		"project_id":   e.projectID,
	}

	return stats, nil
}

// Close cleans up resources
func (e *Engine) Close() error {
	// No need to close the database connection here as it's managed externally
	return nil
}
