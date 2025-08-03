package search_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/jrossi/gismo/pkg/database"
	"github.com/jrossi/gismo/pkg/database/search"
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
	project, err := db.InsertProject(ctx, "test-project", "/test/path", sql.NullString{})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create search engine
	engine, err := search.NewEngineWithProject(db, project.ID)
	if err != nil {
		t.Fatalf("Failed to create search engine: %v", err)
	}

	// Test indexing a code chunk
	chunk := search.CodeChunk{
		FilePath:     "test.go",
		AbsolutePath: "/test/path/test.go",
		Content:      "func main() { fmt.Println(\"Hello, World!\") }",
		ChunkType:    "function",
		Language:     "go",
		StartLine:    1,
		EndLine:      1,
		Metadata: map[string]interface{}{
			"function": "main",
		},
	}

	err = engine.IndexCodeChunk(ctx, chunk)
	if err != nil {
		t.Fatalf("Failed to index code chunk: %v", err)
	}

	// Test batch indexing
	chunks := []search.CodeChunk{
		{
			FilePath:     "test2.go",
			AbsolutePath: "/test/path/test2.go",
			Content:      "func add(a, b int) int { return a + b }",
			ChunkType:    "function",
			Language:     "go",
			StartLine:    1,
			EndLine:      1,
			Metadata: map[string]interface{}{
				"function": "add",
			},
		},
		{
			FilePath:     "test2.go",
			AbsolutePath: "/test/path/test2.go",
			Content:      "func subtract(a, b int) int { return a - b }",
			ChunkType:    "function",
			Language:     "go",
			StartLine:    3,
			EndLine:      3,
			Metadata: map[string]interface{}{
				"function": "subtract",
			},
		},
	}

	err = engine.IndexCodeChunksBatch(ctx, chunks)
	if err != nil {
		t.Fatalf("Failed to index code chunks batch: %v", err)
	}

	// Test search - text search since embeddings might not be available in test
	results, err := engine.SearchCode(ctx, "return", search.SearchOptions{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Failed to search code: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Test search with language filter
	results, err = engine.SearchCode(ctx, "func", search.SearchOptions{
		Limit:    10,
		Language: "go",
	})
	if err != nil {
		t.Fatalf("Failed to search code with language filter: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results with language filter, got %d", len(results))
	}

	// Test get chunks by file
	fileChunks, err := engine.GetCodeChunksByFile(ctx, "test2.go")
	if err != nil {
		t.Fatalf("Failed to get code chunks by file: %v", err)
	}

	if len(fileChunks) != 2 {
		t.Errorf("Expected 2 chunks for test2.go, got %d", len(fileChunks))
	}

	// Test get stats
	stats, err := engine.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	totalChunks, ok := stats["total_chunks"].(int64)
	if !ok {
		t.Error("total_chunks not found in stats")
	} else if totalChunks != 3 {
		t.Errorf("Expected 3 total chunks, got %d", totalChunks)
	}

	// Test delete chunks by file
	err = engine.DeleteChunksByFile(ctx, "test.go")
	if err != nil {
		t.Fatalf("Failed to delete chunks by file: %v", err)
	}

	stats, err = engine.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats after delete: %v", err)
	}

	totalChunks, ok = stats["total_chunks"].(int64)
	if !ok {
		t.Error("total_chunks not found in stats after delete")
	} else if totalChunks != 2 {
		t.Errorf("Expected 2 total chunks after delete, got %d", totalChunks)
	}

	// Test delete all chunks
	err = engine.DeleteAllChunks(ctx)
	if err != nil {
		t.Fatalf("Failed to delete all chunks: %v", err)
	}

	stats, err = engine.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats after delete all: %v", err)
	}

	totalChunks, ok = stats["total_chunks"].(int64)
	if !ok {
		t.Error("total_chunks not found in stats after delete all")
	} else if totalChunks != 0 {
		t.Errorf("Expected 0 total chunks after delete all, got %d", totalChunks)
	}
}

func TestSearchEngineWithoutEmbeddings(t *testing.T) {
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
	project, err := db.InsertProject(ctx, "test-project", "/test/path", sql.NullString{})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create search engine without embeddings
	engine, err := search.NewEngineWithOptions(db, false)
	if err != nil {
		t.Fatalf("Failed to create search engine: %v", err)
	}
	engine.SetProject(project.ID)

	// Test indexing without embeddings
	chunk := search.CodeChunk{
		FilePath:     "test.go",
		AbsolutePath: "/test/path/test.go",
		Content:      "func main() { fmt.Println(\"Hello, World!\") }",
		ChunkType:    "function",
		Language:     "go",
		StartLine:    1,
		EndLine:      1,
	}

	err = engine.IndexCodeChunk(ctx, chunk)
	if err != nil {
		t.Fatalf("Failed to index code chunk without embeddings: %v", err)
	}

	// Test search without embeddings (falls back to text search)
	results, err := engine.SearchCode(ctx, "Hello", search.SearchOptions{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Failed to search code without embeddings: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].Content != chunk.Content {
		t.Errorf("Result content doesn't match: got %s", results[0].Content)
	}
}
