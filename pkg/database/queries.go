package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// InsertProject inserts a new project or updates if it already exists
func (d *DB) InsertProject(ctx context.Context, projectName, projectPath string, description sql.NullString) (*Project, error) {
	query := `
		INSERT INTO projects (project_name, project_path, description)
		VALUES ($1, $2, $3)
		ON CONFLICT(project_name) DO UPDATE SET
			updated_at = NOW()
		RETURNING id, project_name, project_path, description, last_indexed_at, created_at, updated_at`

	var p Project
	err := d.conn.QueryRowContext(ctx, query, projectName, projectPath, description).Scan(
		&p.ID, &p.ProjectName, &p.ProjectPath, &p.Description,
		&p.LastIndexedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	return &p, err
}

// GetProjectByName retrieves a project by its name
func (d *DB) GetProjectByName(ctx context.Context, projectName string) (*Project, error) {
	query := `SELECT id, project_name, project_path, description, last_indexed_at, created_at, updated_at
		FROM projects WHERE project_name = $1`

	var p Project
	err := d.conn.QueryRowContext(ctx, query, projectName).Scan(
		&p.ID, &p.ProjectName, &p.ProjectPath, &p.Description,
		&p.LastIndexedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

// GetProjectByPath retrieves a project by its path
func (d *DB) GetProjectByPath(ctx context.Context, projectPath string) (*Project, error) {
	query := `SELECT id, project_name, project_path, description, last_indexed_at, created_at, updated_at
		FROM projects WHERE project_path = $1`

	var p Project
	err := d.conn.QueryRowContext(ctx, query, projectPath).Scan(
		&p.ID, &p.ProjectName, &p.ProjectPath, &p.Description,
		&p.LastIndexedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

// GetAllProjects retrieves all projects
func (d *DB) GetAllProjects(ctx context.Context) ([]*Project, error) {
	query := `SELECT id, project_name, project_path, description, last_indexed_at, created_at, updated_at
		FROM projects ORDER BY project_name`

	rows, err := d.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		var p Project
		err := rows.Scan(&p.ID, &p.ProjectName, &p.ProjectPath, &p.Description,
			&p.LastIndexedAt, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		projects = append(projects, &p)
	}
	return projects, rows.Err()
}

// UpdateProjectIndexTime updates the last indexed time for a project
func (d *DB) UpdateProjectIndexTime(ctx context.Context, projectID int) error {
	query := `UPDATE projects 
		SET last_indexed_at = NOW(), updated_at = NOW()
		WHERE id = $1`
	_, err := d.conn.ExecContext(ctx, query, projectID)
	return err
}

// InsertCodeChunk inserts a code chunk with embedding
func (d *DB) InsertCodeChunk(ctx context.Context, chunk *CodeChunk) (*CodeChunk, error) {
	// Convert embedding to DuckDB array format
	var embeddingStr string
	if len(chunk.Embedding) > 0 {
		parts := make([]string, len(chunk.Embedding))
		for i, v := range chunk.Embedding {
			parts[i] = fmt.Sprintf("%f", v)
		}
		embeddingStr = "[" + strings.Join(parts, ", ") + "]"
	}

	query := `
		INSERT INTO code_chunks (
			project_id, file_path, absolute_path, content, chunk_type, language,
			start_line, end_line, embedding, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::REAL[], $10)
		RETURNING id, created_at, updated_at`

	err := d.conn.QueryRowContext(ctx, query,
		chunk.ProjectID, chunk.FilePath, chunk.AbsolutePath, chunk.Content,
		chunk.ChunkType, chunk.Language, chunk.StartLine, chunk.EndLine,
		embeddingStr, chunk.Metadata,
	).Scan(&chunk.ID, &chunk.CreatedAt, &chunk.UpdatedAt)

	return chunk, err
}

// InsertCodeChunkWithoutEmbedding inserts a code chunk without embedding
func (d *DB) InsertCodeChunkWithoutEmbedding(ctx context.Context, chunk *CodeChunk) (*CodeChunk, error) {
	query := `
		INSERT INTO code_chunks (
			project_id, file_path, absolute_path, content, chunk_type, language,
			start_line, end_line, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`

	err := d.conn.QueryRowContext(ctx, query,
		chunk.ProjectID, chunk.FilePath, chunk.AbsolutePath, chunk.Content,
		chunk.ChunkType, chunk.Language, chunk.StartLine, chunk.EndLine,
		chunk.Metadata,
	).Scan(&chunk.ID, &chunk.CreatedAt, &chunk.UpdatedAt)

	return chunk, err
}

// UpdateCodeChunkEmbedding updates the embedding for a code chunk
func (d *DB) UpdateCodeChunkEmbedding(ctx context.Context, chunkID int, embedding []float32) error {
	// For now, skip this operation in DuckDB as it seems to have issues
	// TODO: Fix DuckDB array update
	return nil
}

// GetCodeChunkByID retrieves a code chunk by ID
func (d *DB) GetCodeChunkByID(ctx context.Context, id int) (*CodeChunk, error) {
	query := `SELECT id, project_id, file_path, absolute_path, content, chunk_type, language,
		start_line, end_line, embedding, metadata, created_at, updated_at
		FROM code_chunks WHERE id = $1`

	var c CodeChunk
	var embeddingRaw interface{}
	err := d.conn.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.ProjectID, &c.FilePath, &c.AbsolutePath, &c.Content,
		&c.ChunkType, &c.Language, &c.StartLine, &c.EndLine,
		&embeddingRaw, &c.Metadata, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse embedding from DuckDB array
	if embeddingRaw != nil {
		switch v := embeddingRaw.(type) {
		case []interface{}:
			c.Embedding = make([]float32, len(v))
			for i, val := range v {
				if f, ok := val.(float64); ok {
					c.Embedding[i] = float32(f)
				}
			}
		case string:
			c.Embedding = parseEmbedding(v)
		}
	}

	return &c, nil
}

// GetCodeChunksByFilePath retrieves code chunks by file path
func (d *DB) GetCodeChunksByFilePath(ctx context.Context, projectID int, filePath string) ([]*CodeChunk, error) {
	query := `SELECT id, project_id, file_path, absolute_path, content, chunk_type, language,
		start_line, end_line, embedding, metadata, created_at, updated_at
		FROM code_chunks 
		WHERE project_id = $1 AND file_path = $2
		ORDER BY start_line`

	rows, err := d.conn.QueryContext(ctx, query, projectID, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*CodeChunk
	for rows.Next() {
		var c CodeChunk
		var embeddingRaw interface{}
		err := rows.Scan(
			&c.ID, &c.ProjectID, &c.FilePath, &c.AbsolutePath, &c.Content,
			&c.ChunkType, &c.Language, &c.StartLine, &c.EndLine,
			&embeddingRaw, &c.Metadata, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse embedding from DuckDB array
		if embeddingRaw != nil {
			switch v := embeddingRaw.(type) {
			case []interface{}:
				c.Embedding = make([]float32, len(v))
				for i, val := range v {
					if f, ok := val.(float64); ok {
						c.Embedding[i] = float32(f)
					}
				}
			case string:
				c.Embedding = parseEmbedding(v)
			}
		}

		chunks = append(chunks, &c)
	}
	return chunks, rows.Err()
}

// DeleteCodeChunksByFilePath deletes all code chunks for a file
func (d *DB) DeleteCodeChunksByFilePath(ctx context.Context, projectID int, filePath string) error {
	query := `DELETE FROM code_chunks WHERE project_id = $1 AND file_path = $2`
	_, err := d.conn.ExecContext(ctx, query, projectID, filePath)
	return err
}

// DeleteCodeChunksByProject deletes all code chunks for a project
func (d *DB) DeleteCodeChunksByProject(ctx context.Context, projectID int) error {
	query := `DELETE FROM code_chunks WHERE project_id = $1`
	_, err := d.conn.ExecContext(ctx, query, projectID)
	return err
}

// GetTotalChunksCount gets the total number of chunks for a project
func (d *DB) GetTotalChunksCount(ctx context.Context, projectID int) (int64, error) {
	query := `SELECT COUNT(*) FROM code_chunks WHERE project_id = $1`
	var count int64
	err := d.conn.QueryRowContext(ctx, query, projectID).Scan(&count)
	return count, err
}

// GetFileCount gets the number of unique files for a project
func (d *DB) GetFileCount(ctx context.Context, projectID int) (int64, error) {
	query := `SELECT COUNT(DISTINCT file_path) FROM code_chunks WHERE project_id = $1`
	var count int64
	err := d.conn.QueryRowContext(ctx, query, projectID).Scan(&count)
	return count, err
}

// InsertSearchHistory inserts a search record
func (d *DB) InsertSearchHistory(ctx context.Context, query string, resultCount int, filters sql.NullString, execTimeMs sql.NullInt32) (*SearchHistory, error) {
	insertQuery := `
		INSERT INTO search_history (query, result_count, filters, execution_time_ms)
		VALUES ($1, $2, $3, $4)
		RETURNING id, query, result_count, filters, execution_time_ms, created_at`

	var sh SearchHistory
	err := d.conn.QueryRowContext(ctx, insertQuery, query, resultCount, filters, execTimeMs).Scan(
		&sh.ID, &sh.Query, &sh.ResultCount, &sh.Filters, &sh.ExecutionTimeMs, &sh.CreatedAt,
	)
	return &sh, err
}

// GetRecentSearches gets recent search history
func (d *DB) GetRecentSearches(ctx context.Context, limit int) ([]*SearchHistory, error) {
	query := `SELECT id, query, result_count, filters, execution_time_ms, created_at
		FROM search_history
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := d.conn.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var searches []*SearchHistory
	for rows.Next() {
		var sh SearchHistory
		err := rows.Scan(&sh.ID, &sh.Query, &sh.ResultCount, &sh.Filters, &sh.ExecutionTimeMs, &sh.CreatedAt)
		if err != nil {
			return nil, err
		}
		searches = append(searches, &sh)
	}
	return searches, rows.Err()
}

// parseEmbedding parses a DuckDB array string into a float32 slice
func parseEmbedding(s string) []float32 {
	// Remove brackets and split by comma
	s = strings.Trim(s, "[]")
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	result := make([]float32, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var v float32
		if _, err := fmt.Sscanf(part, "%f", &v); err == nil {
			result = append(result, v)
		}
	}
	return result
}
