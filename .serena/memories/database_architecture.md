# Gismo Database Architecture

## Overview
The project uses DuckDB as its embedded database for cross-platform compatibility (Windows, macOS, Linux). Previously used libSQL but switched to DuckDB for Windows support.

## Key Components

### Database Package Structure
```
pkg/database/
├── database.go        # Main DB connection and setup
├── models.go         # Data models (Project, CodeChunk, etc.)
├── queries.go        # All database operations
├── migrations/       # SQL migration files
├── project/         # Project management logic
│   └── project.go   # Project isolation and context
└── search/          # Search engine implementation
    └── engine.go    # Vector and text search
```

### Technology Stack
- **Database**: DuckDB (embedded OLAP database)
- **Vector Search**: DuckDB VSS extension (HNSW indexing)
- **Embeddings**: REAL[] arrays for vector storage
- **JSON**: github.com/goccy/go-json for fast parsing
- **No ORM**: Direct SQL queries (sqlc was removed)

### Key Features
1. **Project Isolation**: Each Claude Code project gets its own context
2. **Vector Search**: Similarity search using embeddings
3. **Full-Text Search**: Pattern matching and code search
4. **Cross-Platform**: Works on Windows, macOS, Linux
5. **Embedded**: No external database server required

### Migration Notes
- Changed from SERIAL to SEQUENCE + INTEGER (DuckDB compatibility)
- Removed ON DELETE CASCADE (not supported in DuckDB)
- Use NOW() instead of CURRENT_TIMESTAMP
- Arrays handled as REAL[] type
- Different ON CONFLICT behavior than PostgreSQL/SQLite

### Testing Considerations
- DuckDB returns arrays as []interface{} not strings
- Timing sensitivity in tests (use time.Sleep for timestamps)
- Handle DuckDB-specific SQL behaviors
- Test both with and without embeddings

### Current Limitations
- UpdateCodeChunkEmbedding temporarily disabled (TODO: fix array updates)
- VSS extension may not be available on all systems (graceful fallback)