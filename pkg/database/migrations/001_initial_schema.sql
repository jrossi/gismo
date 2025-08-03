-- Initial schema for code search database
-- This creates the tables needed for vector-based code search

-- Projects table for tracking Claude Code projects
CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_name TEXT NOT NULL UNIQUE, -- e.g., "-Users-jrossi-src-gismo"
    project_path TEXT NOT NULL UNIQUE, -- e.g., "/Users/jrossi/src/gismo"
    description TEXT,
    last_indexed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Code chunks table for storing indexed code segments
CREATE TABLE IF NOT EXISTS code_chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    file_path TEXT NOT NULL, -- Relative path within project
    absolute_path TEXT NOT NULL, -- Full file path
    content TEXT NOT NULL,
    chunk_type TEXT NOT NULL, -- function, class, method, etc.
    language TEXT NOT NULL,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    embedding BLOB, -- Vector embeddings stored as BLOB
    metadata TEXT, -- JSON metadata for additional information
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- Traditional indexes for filtering
CREATE INDEX IF NOT EXISTS idx_code_chunks_project_id ON code_chunks(project_id);
CREATE INDEX IF NOT EXISTS idx_code_chunks_language ON code_chunks(language);
CREATE INDEX IF NOT EXISTS idx_code_chunks_type ON code_chunks(chunk_type);
CREATE INDEX IF NOT EXISTS idx_code_chunks_path ON code_chunks(file_path);
CREATE INDEX IF NOT EXISTS idx_code_chunks_absolute_path ON code_chunks(absolute_path);

-- Search history for analytics and improvements
CREATE TABLE IF NOT EXISTS search_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    query TEXT NOT NULL,
    result_count INTEGER NOT NULL,
    filters TEXT, -- JSON representation of applied filters
    execution_time_ms INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index stats for monitoring
CREATE TABLE IF NOT EXISTS index_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    total_chunks INTEGER NOT NULL,
    total_files INTEGER NOT NULL,
    languages TEXT NOT NULL, -- JSON array of language stats
    last_indexed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);