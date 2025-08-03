package search_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/jrossi/gismo/pkg/database"
	"github.com/jrossi/gismo/pkg/database/search"
	sqlcdb "github.com/jrossi/gismo/pkg/database/sqlc"
)

func TestSearchEngine(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "gismo_search_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.NewWithPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a test project
	testProject, err := db.Queries().InsertProject(ctx, sqlcdb.InsertProjectParams{
		ProjectName: "-test-project",
		ProjectPath: "/test/project",
		Description: sql.NullString{String: "Test project", Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	engine, err := search.NewEngineWithProject(db.Conn(), db.Queries(), testProject.ID)
	if err != nil {
		t.Fatalf("Failed to create search engine: %v", err)
	}
	defer engine.Close()

	// Test indexing a single code chunk
	chunk := search.CodeChunk{
		FilePath:     "test.go",
		AbsolutePath: "/test/project/test.go",
		Content:      "func ParseJSON(data []byte) (interface{}, error) {\n    var result interface{}\n    err := json.Unmarshal(data, &result)\n    return result, err\n}",
		ChunkType:    "function",
		Language:     "go",
		StartLine:    10,
		EndLine:      15,
		Metadata: map[string]interface{}{
			"exported": true,
			"package":  "parser",
		},
	}

	err = engine.IndexCodeChunk(ctx, chunk)
	if err != nil {
		t.Errorf("Failed to index code chunk: %v", err)
	}

	// Test batch indexing
	chunks := []search.CodeChunk{
		{
			FilePath:     "handler.go",
			AbsolutePath: "/test/project/handler.go",
			Content:      "func HandleRequest(w http.ResponseWriter, r *http.Request) {\n    w.WriteHeader(http.StatusOK)\n    w.Write([]byte(\"OK\"))\n}",
			ChunkType:    "function",
			Language:     "go",
			StartLine:    20,
			EndLine:      24,
		},
		{
			FilePath:     "utils.go",
			AbsolutePath: "/test/project/utils.go",
			Content:      "func FormatString(s string) string {\n    return strings.TrimSpace(s)\n}",
			ChunkType:    "function",
			Language:     "go",
			StartLine:    5,
			EndLine:      7,
		},
	}

	err = engine.IndexCodeChunksBatch(ctx, chunks)
	if err != nil {
		t.Errorf("Failed to batch index code chunks: %v", err)
	}

	// Test semantic search (falls back to keyword when embedder unavailable)
	results, err := engine.SearchSemantic(ctx, "JSON", search.SearchOptions{
		Limit:    5,
		Language: "go",
	})
	if err != nil {
		t.Errorf("Failed to perform semantic search: %v", err)
	}

	// With embedder disabled, semantic search falls back to keyword search
	// so we need to use a keyword that exists in the content
	if len(results) == 0 {
		t.Error("Expected at least one search result for keyword 'JSON'")
	}

	// Test keyword search
	keywordResults, err := engine.SearchKeyword(ctx, "http", search.SearchOptions{
		Limit: 5,
	})
	if err != nil {
		t.Errorf("Failed to perform keyword search: %v", err)
	}

	if len(keywordResults) == 0 {
		t.Error("Expected at least one keyword search result")
	}

	// Test hybrid search (falls back to keyword when embedder unavailable)
	hybridResults, err := engine.SearchHybrid(ctx, "Handle", search.SearchOptions{
		Limit: 5,
	})
	if err != nil {
		t.Errorf("Failed to perform hybrid search: %v", err)
	}

	if len(hybridResults) == 0 {
		t.Error("Expected at least one hybrid search result for keyword 'Handle'")
	}

	// Test stats
	stats, err := engine.GetStats(ctx)
	if err != nil {
		t.Errorf("Failed to get stats: %v", err)
	}

	if stats["total_chunks"].(int64) != 3 {
		t.Errorf("Expected 3 chunks, got %v", stats["total_chunks"])
	}

	// Test update file chunks
	newChunks := []search.CodeChunk{
		{
			Content:   "func NewHandler() *Handler {\n    return &Handler{}\n}",
			ChunkType: "function",
			Language:  "go",
			StartLine: 1,
			EndLine:   3,
		},
	}

	err = engine.UpdateFileChunks(ctx, "handler.go", newChunks)
	if err != nil {
		t.Errorf("Failed to update file chunks: %v", err)
	}
}

func TestSearchOptions(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "gismo_search_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.NewWithPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a test project
	testProject, err := db.Queries().InsertProject(ctx, sqlcdb.InsertProjectParams{
		ProjectName: "-test-project",
		ProjectPath: "/test/project",
		Description: sql.NullString{String: "Test project", Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	engine, err := search.NewEngineWithProject(db.Conn(), db.Queries(), testProject.ID)
	if err != nil {
		t.Fatalf("Failed to create search engine: %v", err)
	}
	defer engine.Close()

	// Index chunks with different languages and types
	chunks := []search.CodeChunk{
		{
			FilePath:     "main.go",
			AbsolutePath: "/test/project/main.go",
			Content:      "func main() {}",
			ChunkType:    "function",
			Language:     "go",
			StartLine:    1,
			EndLine:      1,
		},
		{
			FilePath:     "test.py",
			AbsolutePath: "/test/project/test.py",
			Content:      "def test(): pass",
			ChunkType:    "function",
			Language:     "python",
			StartLine:    1,
			EndLine:      1,
		},
		{
			FilePath:     "app.js",
			AbsolutePath: "/test/project/app.js",
			Content:      "class App {}",
			ChunkType:    "class",
			Language:     "javascript",
			StartLine:    1,
			EndLine:      1,
		},
	}

	err = engine.IndexCodeChunksBatch(ctx, chunks)
	if err != nil {
		t.Errorf("Failed to index chunks: %v", err)
	}

	// Test language filter
	results, err := engine.SearchKeyword(ctx, "", search.SearchOptions{
		Language: "go",
		Limit:    10,
	})
	if err != nil {
		t.Errorf("Failed to search by language: %v", err)
	}

	for _, result := range results {
		if result.Language != "go" {
			t.Errorf("Expected language 'go', got %s", result.Language)
		}
	}

	// Test chunk type filter
	results, err = engine.SearchKeyword(ctx, "", search.SearchOptions{
		ChunkType: "class",
		Limit:     10,
	})
	if err != nil {
		t.Errorf("Failed to search by chunk type: %v", err)
	}

	for _, result := range results {
		if result.ChunkType != "class" {
			t.Errorf("Expected chunk type 'class', got %s", result.ChunkType)
		}
	}

	// All chunks are in the same project now, so we can test that they're all found
	results, err = engine.SearchKeyword(ctx, "", search.SearchOptions{
		Limit: 10,
	})
	if err != nil {
		t.Errorf("Failed to search all chunks: %v", err)
	}

	// Should find all 3 chunks
	if len(results) != 3 {
		t.Errorf("Expected 3 chunks, got %d", len(results))
	}
}
