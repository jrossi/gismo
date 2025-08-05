# Gismo Knowledge System Implementation Plan

## Overview
Extend the existing gismo server with gRPC support to provide a knowledge management system using DuckDB. The system will import and index docsets for semantic and keyword search, making documentation instantly accessible to LLMs.

## Architecture

### Core Components
1. **gRPC Service** over existing Unix socket infrastructure
2. **Global Docset Storage** in `~/.gismo/knowledge.db` (DuckDB)
3. **Docset Import Pipeline** with URL download support
4. **Semantic + Keyword Search** using DuckDB's VSS extension

### Directory Structure
```
pkg/
├── server/              # Existing Unix socket server
│   ├── server.go       # Extend with gRPC server
│   └── handlers/       # NEW: gRPC service implementations
│       └── knowledge.go
├── client/             # Existing client
│   └── knowledge/      # NEW: gRPC client
│       └── client.go
├── proto/              # NEW: Proto definitions
│   └── gismo/
│       └── v1/
│           └── knowledge.proto
├── generated/          # NEW: Generated code (buf)
│   └── gismo/
│       └── v1/
│           ├── knowledge.pb.go
│           └── knowledge_grpc.pb.go
├── docset/            # NEW: Docset handling
│   ├── downloader.go  # Download from URLs
│   ├── parser.go      # Parse docset format
│   ├── importer.go    # Import to DuckDB
│   └── indexer.go     # Generate embeddings
└── knowledge/         # NEW: Knowledge store
    ├── store.go       # DuckDB interface
    ├── search.go      # Search implementation
    └── schema.go      # Database schema
```

## Proto Definition

```protobuf
syntax = "proto3";
package gismo.v1;

import "google/protobuf/timestamp.proto";
import "google/protobuf/struct.proto";

service KnowledgeService {
  // Docset management
  rpc ImportDocset(ImportDocsetRequest) returns (stream ImportProgress);
  rpc ListDocsets(ListDocsetsRequest) returns (ListDocsetsResponse);
  rpc RemoveDocset(RemoveDocsetRequest) returns (RemoveDocsetResponse);

  // Search operations
  rpc Search(SearchRequest) returns (SearchResponse);
  rpc GetContent(GetContentRequest) returns (GetContentResponse);

  // Raw SQL access
  rpc ExecuteQuery(QueryRequest) returns (QueryResponse);
  rpc ExecuteQueryStream(QueryRequest) returns (stream QueryResult);
}
```

## Database Schema

### Global Storage Location
- Database: `~/.gismo/knowledge.db`
- Docset cache: `~/.cache/gismo/docsets/`

### Tables
```sql
-- Docsets registry
CREATE TABLE docsets (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  version TEXT,
  language TEXT,
  source_url TEXT,
  source_type TEXT, -- 'official', 'user', 'custom'
  imported_at TIMESTAMP DEFAULT NOW(),
  metadata JSON
);

-- Docset content with embeddings
CREATE TABLE docset_content (
  id INTEGER PRIMARY KEY,
  docset_id TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  path TEXT NOT NULL,
  content TEXT,
  summary TEXT,
  embedding REAL[],
  metadata JSON,
  FOREIGN KEY (docset_id) REFERENCES docsets(id) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX idx_docset_content_name ON docset_content(name);
CREATE INDEX idx_docset_content_type ON docset_content(docset_id, type);
CREATE INDEX idx_docset_content_embedding ON docset_content
  USING HNSW (embedding);
```

## Implementation Steps

### Phase 1: Proto Setup and Code Generation
1. Set up buf configuration (`buf.yaml`, `buf.gen.yaml`)
2. Define proto schema for knowledge service
3. Generate Go code using buf
4. Set up build integration

### Phase 2: Extend Server with gRPC
1. Add gRPC server to existing Unix socket server
2. Implement basic service handler skeleton
3. Test gRPC connection over Unix socket
4. Add logging and error handling

### Phase 3: Database Layer
1. Create knowledge store package
2. Implement DuckDB connection management
3. Create schema and migrations
4. Add basic CRUD operations

### Phase 4: Docset Import Pipeline
1. Implement docset downloader
   - Support Kapeli official feeds
   - Support user-contributed docsets
   - Support custom repositories
2. Parse docset structure (plist, SQLite index)
3. Extract and clean content
4. Generate embeddings using fastembed
5. Store in DuckDB with progress reporting

### Phase 5: Search Implementation
1. Keyword search using DuckDB full-text
2. Semantic search using VSS extension
3. Hybrid search combining both
4. Result ranking and relevance scoring

### Phase 6: Client Integration
1. Create gRPC client wrapper
2. Add CLI commands for docset management
3. Integrate with existing gismo commands
4. Add search command for testing

## Docset Sources

### Supported Sources
1. **Official Kapeli Feeds**: https://kapeli.com/feeds/
2. **User Contributed**: https://github.com/Kapeli/Dash-User-Contributions
3. **Custom Repositories**: User-specified URLs

### URL Patterns
- Official: `https://kapeli.com/feeds/{DocsetName}.tgz`
- User: `https://github.com/Kapeli/Dash-User-Contributions/tree/master/docsets/{DocsetName}`
- Custom: Any URL pointing to a `.tgz` containing a `.docset`

## Key Design Decisions

1. **Buf for Proto Management**: Cleaner than raw protoc, better tooling
2. **Standard gRPC**: No need for ConnectRPC complexity initially
3. **Global Docsets**: All docsets stored globally, not per-project
4. **All-or-Nothing Import**: No partial imports, ensures consistency
5. **Unrestricted SQL**: Trust the LLM to make appropriate queries
6. **Streaming Progress**: Use gRPC streaming for import progress

## Security Considerations

- Unix socket permissions (0600) restrict access to user only
- No authentication needed due to socket permissions
- SQL injection not a concern (trusted LLM client)
- Resource limits can be added later if needed

## Future Enhancements

1. **Project-Specific Views**: Filter global docsets by project context
2. **Auto-Import**: Detect dependencies and suggest docsets
3. **Incremental Updates**: Refresh docsets when new versions available
4. **Cross-Project Learning**: Track which docs are most useful
5. **Custom Embeddings**: Fine-tune embeddings for code search

## Success Criteria

1. Can import docsets from URLs with progress feedback
2. Sub-second semantic search across all imported content
3. SQL interface allows flexible querying by LLM
4. Integrates seamlessly with existing gismo infrastructure
5. Zero configuration required for basic usage

## Testing Strategy

1. Unit tests for each component
2. Integration tests for import pipeline
3. End-to-end tests with real docsets
4. Performance benchmarks for search
5. Stress tests with large docsets (e.g., AWS)