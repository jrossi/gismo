package search

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/anush008/fastembed-go"
	db "github.com/jrossi/gismo/pkg/database/sqlc"
	libsqlvector "github.com/ryanskidmore/libsql-vector-go"
)

type Engine struct {
	db        *sql.DB
	queries   *db.Queries
	embedder  *fastembed.FlagEmbedding
	projectID int64
}

type CodeChunk struct {
	ID           int64
	ProjectID    int64
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

func NewEngine(database *sql.DB, queries *db.Queries) (*Engine, error) {
	return NewEngineWithOptions(database, queries, true)
}

func NewEngineWithOptions(database *sql.DB, queries *db.Queries, initEmbedder bool) (*Engine, error) {
	engine := &Engine{
		db:      database,
		queries: queries,
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

func NewEngineWithProject(database *sql.DB, queries *db.Queries, projectID int64) (*Engine, error) {
	engine, err := NewEngine(database, queries)
	if err != nil {
		return nil, err
	}
	engine.projectID = projectID
	return engine, nil
}

// SetProject sets the current project for the search engine
func (e *Engine) SetProject(projectID int64) {
	e.projectID = projectID
}

// GetProject returns the current project ID
func (e *Engine) GetProject() int64 {
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

	if e.embedder != nil {
		input := fmt.Sprintf("passage: %s", chunk.Content)
		embeddings, err := e.embedder.Embed([]string{input}, 1)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}

		vector := libsqlvector.NewVector(embeddings[0])

		query := `
		INSERT INTO code_chunks (project_id, file_path, absolute_path, content, chunk_type, language, start_line, end_line, embedding, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, vector(?), ?)
		`

		_, err = e.db.ExecContext(ctx, query,
			chunk.ProjectID,
			chunk.FilePath,
			chunk.AbsolutePath,
			chunk.Content,
			chunk.ChunkType,
			chunk.Language,
			chunk.StartLine,
			chunk.EndLine,
			vector.String(),
			string(metadata),
		)
		return err
	}

	// Without embedder, insert without embedding
	_, err = e.queries.InsertCodeChunkWithoutEmbedding(ctx, db.InsertCodeChunkWithoutEmbeddingParams{
		ProjectID:    chunk.ProjectID,
		FilePath:     chunk.FilePath,
		AbsolutePath: chunk.AbsolutePath,
		Content:      chunk.Content,
		ChunkType:    chunk.ChunkType,
		Language:     chunk.Language,
		StartLine:    int64(chunk.StartLine),
		EndLine:      int64(chunk.EndLine),
		Metadata:     sql.NullString{String: string(metadata), Valid: true},
	})

	return err
}

func (e *Engine) IndexCodeChunksBatch(ctx context.Context, chunks []CodeChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

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

		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO code_chunks (project_id, file_path, absolute_path, content, chunk_type, language, start_line, end_line, embedding, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, vector(?), ?)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for i, chunk := range chunks {
			vector := libsqlvector.NewVector(embeddings[i])

			metadata, err := json.Marshal(chunk.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}

			_, err = stmt.ExecContext(ctx,
				chunk.ProjectID,
				chunk.FilePath,
				chunk.AbsolutePath,
				chunk.Content,
				chunk.ChunkType,
				chunk.Language,
				chunk.StartLine,
				chunk.EndLine,
				vector.String(),
				string(metadata),
			)
			if err != nil {
				return fmt.Errorf("failed to insert chunk: %w", err)
			}
		}
	} else {
		// Without embedder, insert without embeddings
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO code_chunks (project_id, file_path, absolute_path, content, chunk_type, language, start_line, end_line, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, chunk := range chunks {
			metadata, err := json.Marshal(chunk.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}

			_, err = stmt.ExecContext(ctx,
				chunk.ProjectID,
				chunk.FilePath,
				chunk.AbsolutePath,
				chunk.Content,
				chunk.ChunkType,
				chunk.Language,
				chunk.StartLine,
				chunk.EndLine,
				string(metadata),
			)
			if err != nil {
				return fmt.Errorf("failed to insert chunk: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (e *Engine) SearchSemantic(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	startTime := time.Now()

	if e.embedder == nil {
		// Fall back to keyword search when embedder is not available
		return e.SearchKeyword(ctx, query, opts)
	}

	queryInput := fmt.Sprintf("query: %s", query)
	queryEmbeddings, err := e.embedder.QueryEmbed(queryInput)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	queryVector := libsqlvector.NewVector(queryEmbeddings)

	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	sqlQuery := `
	SELECT 
		cc.id, cc.project_id, cc.file_path, cc.absolute_path, cc.content, cc.chunk_type, 
		cc.language, cc.start_line, cc.end_line, cc.metadata,
		(1.0 - vector_distance_cos(cc.embedding, vector(?))) as similarity
	FROM vector_top_k('idx_code_chunks_embedding', vector(?), ?) vt
	JOIN code_chunks cc ON cc.rowid = vt.id
	WHERE cc.project_id = ?
	`

	args := []interface{}{queryVector.String(), queryVector.String(), opts.Limit, e.projectID}

	var whereConditions []string
	if opts.Language != "" {
		whereConditions = append(whereConditions, "cc.language = ?")
		args = append(args, opts.Language)
	}
	if opts.ChunkType != "" {
		whereConditions = append(whereConditions, "cc.chunk_type = ?")
		args = append(args, opts.ChunkType)
	}

	if len(whereConditions) > 0 {
		sqlQuery += " AND " + strings.Join(whereConditions, " AND ")
	}

	sqlQuery += " ORDER BY similarity DESC"

	rows, err := e.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
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
			&result.Similarity,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}

		if metadataStr.Valid && metadataStr.String != "" {
			if err := json.Unmarshal([]byte(metadataStr.String), &result.Metadata); err != nil {
				log.Printf("Failed to unmarshal metadata: %v", err)
			}
		}

		results = append(results, result)
	}

	executionTime := time.Since(startTime).Milliseconds()
	filters := map[string]interface{}{
		"language":   opts.Language,
		"chunk_type": opts.ChunkType,
		"project_id": e.projectID,
	}
	filtersJSON, _ := json.Marshal(filters)

	_, err = e.queries.InsertSearchHistory(ctx, db.InsertSearchHistoryParams{
		Query:           query,
		ResultCount:     int64(len(results)),
		Filters:         sql.NullString{String: string(filtersJSON), Valid: true},
		ExecutionTimeMs: sql.NullInt64{Int64: executionTime, Valid: true},
	})
	if err != nil {
		log.Printf("Failed to record search history: %v", err)
	}

	return results, nil
}

func (e *Engine) SearchKeyword(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if e.projectID == 0 {
		return nil, fmt.Errorf("project ID not set")
	}

	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	sqlQuery := `
	SELECT 
		id, project_id, file_path, absolute_path, content, chunk_type, language, start_line, end_line,
		metadata, 1.0 as similarity
	FROM code_chunks 
	WHERE project_id = ? AND content LIKE ?
	`

	args := []interface{}{e.projectID, "%" + query + "%"}

	var whereConditions []string
	if opts.Language != "" {
		whereConditions = append(whereConditions, "language = ?")
		args = append(args, opts.Language)
	}
	if opts.ChunkType != "" {
		whereConditions = append(whereConditions, "chunk_type = ?")
		args = append(args, opts.ChunkType)
	}

	if len(whereConditions) > 0 {
		sqlQuery += " AND " + strings.Join(whereConditions, " AND ")
	}

	sqlQuery += " LIMIT ?"
	args = append(args, opts.Limit)

	rows, err := e.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute keyword search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
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
			&result.Similarity,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}

		if metadataStr.Valid && metadataStr.String != "" {
			if err := json.Unmarshal([]byte(metadataStr.String), &result.Metadata); err != nil {
				log.Printf("Failed to unmarshal metadata: %v", err)
			}
		}

		results = append(results, result)
	}

	return results, nil
}

func (e *Engine) SearchHybrid(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	semanticResults, err := e.SearchSemantic(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	keywordResults, err := e.SearchKeyword(ctx, query, opts)
	if err != nil {
		log.Printf("Keyword search failed: %v", err)
		return semanticResults, nil
	}

	return e.combineResults(semanticResults, keywordResults, opts.Limit), nil
}

func (e *Engine) combineResults(semantic, keyword []SearchResult, limit int) []SearchResult {
	if limit <= 0 {
		limit = 10
	}

	seen := make(map[int64]bool)
	var combined []SearchResult

	for _, result := range semantic {
		if !seen[result.ID] {
			combined = append(combined, result)
			seen[result.ID] = true
		}
	}

	for _, result := range keyword {
		if !seen[result.ID] && len(combined) < limit {
			combined = append(combined, result)
			seen[result.ID] = true
		}
	}

	if len(combined) > limit {
		combined = combined[:limit]
	}

	return combined
}

func (e *Engine) GetStats(ctx context.Context) (map[string]interface{}, error) {
	if e.projectID == 0 {
		return nil, fmt.Errorf("project ID not set")
	}

	stats := make(map[string]interface{})
	stats["project_id"] = e.projectID

	totalChunks, err := e.queries.GetTotalChunksCount(ctx, e.projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total chunks: %w", err)
	}
	stats["total_chunks"] = totalChunks

	fileCount, err := e.queries.GetFileCount(ctx, e.projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file count: %w", err)
	}
	stats["total_files"] = fileCount

	languageStats, err := e.queries.GetLanguageStats(ctx, e.projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get language stats: %w", err)
	}

	languages := make(map[string]int64)
	for _, stat := range languageStats {
		languages[stat.Language] = stat.Count
	}
	stats["languages"] = languages

	chunkTypeStats, err := e.queries.GetChunkTypeStats(ctx, e.projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk type stats: %w", err)
	}

	chunkTypes := make(map[string]int64)
	for _, stat := range chunkTypeStats {
		chunkTypes[stat.ChunkType] = stat.Count
	}
	stats["chunk_types"] = chunkTypes

	return stats, nil
}

func (e *Engine) UpdateFileChunks(ctx context.Context, filePath string, chunks []CodeChunk) error {
	if e.projectID == 0 {
		return fmt.Errorf("project ID not set")
	}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, "DELETE FROM code_chunks WHERE project_id = ? AND file_path = ?", e.projectID, filePath); err != nil {
		return fmt.Errorf("failed to delete old chunks: %w", err)
	}

	// Set file path and project ID for all chunks
	for i := range chunks {
		chunks[i].FilePath = filePath
		chunks[i].ProjectID = e.projectID
	}

	if e.embedder != nil {
		// Prepare statement for batch insert within transaction
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO code_chunks (project_id, file_path, absolute_path, content, chunk_type, language, start_line, end_line, embedding, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, vector(?), ?)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		// Generate embeddings for all chunks
		var inputs []string
		for _, chunk := range chunks {
			inputs = append(inputs, fmt.Sprintf("passage: %s", chunk.Content))
		}

		embeddings, err := e.embedder.Embed(inputs, 32)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings: %w", err)
		}

		// Insert all chunks with their embeddings
		for i, chunk := range chunks {
			vector := libsqlvector.NewVector(embeddings[i])

			metadata, err := json.Marshal(chunk.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}

			_, err = stmt.ExecContext(ctx,
				chunk.ProjectID,
				chunk.FilePath,
				chunk.AbsolutePath,
				chunk.Content,
				chunk.ChunkType,
				chunk.Language,
				chunk.StartLine,
				chunk.EndLine,
				vector.String(),
				string(metadata),
			)
			if err != nil {
				return fmt.Errorf("failed to insert chunk: %w", err)
			}
		}
	} else {
		// Without embedder, insert without embeddings
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO code_chunks (project_id, file_path, absolute_path, content, chunk_type, language, start_line, end_line, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, chunk := range chunks {
			metadata, err := json.Marshal(chunk.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}

			_, err = stmt.ExecContext(ctx,
				chunk.ProjectID,
				chunk.FilePath,
				chunk.AbsolutePath,
				chunk.Content,
				chunk.ChunkType,
				chunk.Language,
				chunk.StartLine,
				chunk.EndLine,
				string(metadata),
			)
			if err != nil {
				return fmt.Errorf("failed to insert chunk: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (e *Engine) Close() error {
	if e.embedder != nil {
		_ = e.embedder.Destroy()
	}
	return nil
}
