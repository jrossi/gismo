package database

import (
	"database/sql"
	"time"
)

// Project represents a code project
type Project struct {
	ID            int            `json:"id"`
	ProjectName   string         `json:"project_name"`
	ProjectPath   string         `json:"project_path"`
	Description   sql.NullString `json:"description"`
	LastIndexedAt sql.NullTime   `json:"last_indexed_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// CodeChunk represents a piece of code with its metadata and embedding
type CodeChunk struct {
	ID           int            `json:"id"`
	ProjectID    int            `json:"project_id"`
	FilePath     string         `json:"file_path"`
	AbsolutePath string         `json:"absolute_path"`
	Content      string         `json:"content"`
	ChunkType    string         `json:"chunk_type"`
	Language     string         `json:"language"`
	StartLine    int            `json:"start_line"`
	EndLine      int            `json:"end_line"`
	Embedding    []float32      `json:"embedding"`
	Metadata     sql.NullString `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// SearchHistory represents a search query and its results
type SearchHistory struct {
	ID              int            `json:"id"`
	Query           string         `json:"query"`
	ResultCount     int            `json:"result_count"`
	Filters         sql.NullString `json:"filters"`
	ExecutionTimeMs sql.NullInt32  `json:"execution_time_ms"`
	CreatedAt       time.Time      `json:"created_at"`
}

// IndexStats represents statistics about the indexed codebase
type IndexStats struct {
	ID            int       `json:"id"`
	TotalChunks   int       `json:"total_chunks"`
	TotalFiles    int       `json:"total_files"`
	Languages     string    `json:"languages"`
	LastIndexedAt time.Time `json:"last_indexed_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
