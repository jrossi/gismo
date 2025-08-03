package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDatabase(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "gismo_db_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := NewWithPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	if db.conn == nil {
		t.Error("Database connection is nil")
	}

	if db.dbPath != dbPath {
		t.Errorf("Expected dbPath to be %s, got %s", dbPath, db.dbPath)
	}
}

func TestDatabaseMigrations(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "gismo_db_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := NewWithPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Check that tables exist by attempting to query them
	tables := []string{
		"projects",
		"code_chunks",
		"search_history",
		"index_stats",
	}

	for _, table := range tables {
		query := "SELECT COUNT(*) FROM " + table
		var count int
		err := db.conn.QueryRowContext(ctx, query).Scan(&count)
		if err != nil {
			t.Errorf("Failed to query table %s: %v", table, err)
		}
	}
}

func TestProjectOperations(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "gismo_db_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := NewWithPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test InsertProject
	project, err := db.InsertProject(ctx, "test-project", "/test/path", sql.NullString{
		String: "Test project description",
		Valid:  true,
	})
	if err != nil {
		t.Fatalf("Failed to insert project: %v", err)
	}

	if project.ProjectName != "test-project" {
		t.Errorf("Expected project name to be test-project, got %s", project.ProjectName)
	}

	// Test GetProjectByName
	retrieved, err := db.GetProjectByName(ctx, "test-project")
	if err != nil {
		t.Fatalf("Failed to get project by name: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved project is nil")
	}

	if retrieved.ID != project.ID {
		t.Errorf("Expected project ID to be %d, got %d", project.ID, retrieved.ID)
	}

	// Test GetProjectByPath
	byPath, err := db.GetProjectByPath(ctx, "/test/path")
	if err != nil {
		t.Fatalf("Failed to get project by path: %v", err)
	}

	if byPath == nil {
		t.Fatal("Retrieved project by path is nil")
	}

	if byPath.ID != project.ID {
		t.Errorf("Expected project ID to be %d, got %d", project.ID, byPath.ID)
	}

	// Test GetAllProjects
	projects, err := db.GetAllProjects(ctx)
	if err != nil {
		t.Fatalf("Failed to get all projects: %v", err)
	}

	if len(projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(projects))
	}

	// Test UpdateProjectIndexTime
	err = db.UpdateProjectIndexTime(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to update project index time: %v", err)
	}
}

func TestCodeChunkOperations(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "gismo_db_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := NewWithPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a project first
	project, err := db.InsertProject(ctx, "test-project", "/test/path", sql.NullString{})
	if err != nil {
		t.Fatalf("Failed to insert project: %v", err)
	}

	// Test InsertCodeChunkWithoutEmbedding
	chunk := &CodeChunk{
		ProjectID:    project.ID,
		FilePath:     "test.go",
		AbsolutePath: "/test/path/test.go",
		Content:      "func main() {}",
		ChunkType:    "function",
		Language:     "go",
		StartLine:    1,
		EndLine:      1,
		Metadata:     sql.NullString{String: `{"test": "value"}`, Valid: true},
	}

	inserted, err := db.InsertCodeChunkWithoutEmbedding(ctx, chunk)
	if err != nil {
		t.Fatalf("Failed to insert code chunk: %v", err)
	}

	if inserted.ID == 0 {
		t.Error("Inserted chunk has zero ID")
	}

	// Test GetCodeChunkByID
	retrieved, err := db.GetCodeChunkByID(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("Failed to get code chunk by ID: %v", err)
	}

	if retrieved.Content != chunk.Content {
		t.Errorf("Expected content to be %s, got %s", chunk.Content, retrieved.Content)
	}

	// Test InsertCodeChunk with embedding
	chunkWithEmbedding := &CodeChunk{
		ProjectID:    project.ID,
		FilePath:     "test2.go",
		AbsolutePath: "/test/path/test2.go",
		Content:      "func test() {}",
		ChunkType:    "function",
		Language:     "go",
		StartLine:    1,
		EndLine:      1,
		Embedding:    []float32{0.1, 0.2, 0.3},
		Metadata:     sql.NullString{},
	}

	insertedWithEmbedding, err := db.InsertCodeChunk(ctx, chunkWithEmbedding)
	if err != nil {
		t.Fatalf("Failed to insert code chunk with embedding: %v", err)
	}

	if insertedWithEmbedding.ID == 0 {
		t.Error("Inserted chunk with embedding has zero ID")
	}

	// Test GetCodeChunksByFilePath
	chunks, err := db.GetCodeChunksByFilePath(ctx, project.ID, "test.go")
	if err != nil {
		t.Fatalf("Failed to get code chunks by file path: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(chunks))
	}

	// Test GetTotalChunksCount
	count, err := db.GetTotalChunksCount(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get total chunks count: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 chunks, got %d", count)
	}

	// Test GetFileCount
	fileCount, err := db.GetFileCount(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get file count: %v", err)
	}

	if fileCount != 2 {
		t.Errorf("Expected 2 files, got %d", fileCount)
	}

	// Test UpdateCodeChunkEmbedding
	t.Logf("Updating embedding for chunk ID: %d", inserted.ID)
	err = db.UpdateCodeChunkEmbedding(ctx, inserted.ID, []float32{0.4, 0.5, 0.6})
	if err != nil {
		t.Fatalf("Failed to update code chunk embedding for ID %d: %v", inserted.ID, err)
	}

	// Test DeleteCodeChunksByFilePath
	err = db.DeleteCodeChunksByFilePath(ctx, project.ID, "test.go")
	if err != nil {
		t.Fatalf("Failed to delete code chunks by file path: %v", err)
	}

	count, err = db.GetTotalChunksCount(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get total chunks count after delete: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 chunk after delete, got %d", count)
	}

	// Test DeleteCodeChunksByProject
	err = db.DeleteCodeChunksByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to delete code chunks by project: %v", err)
	}

	count, err = db.GetTotalChunksCount(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get total chunks count after project delete: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 chunks after project delete, got %d", count)
	}
}

func TestSearchHistory(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "gismo_db_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := NewWithPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test InsertSearchHistory
	search, err := db.InsertSearchHistory(ctx, "test query", 5,
		sql.NullString{String: "language:go", Valid: true},
		sql.NullInt32{Int32: 100, Valid: true})
	if err != nil {
		t.Fatalf("Failed to insert search history: %v", err)
	}

	if search.Query != "test query" {
		t.Errorf("Expected query to be 'test query', got %s", search.Query)
	}

	// Sleep to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Insert another search
	_, err = db.InsertSearchHistory(ctx, "another query", 10,
		sql.NullString{}, sql.NullInt32{})
	if err != nil {
		t.Fatalf("Failed to insert second search history: %v", err)
	}

	// Test GetRecentSearches
	searches, err := db.GetRecentSearches(ctx, 10)
	if err != nil {
		t.Fatalf("Failed to get recent searches: %v", err)
	}

	if len(searches) != 2 {
		t.Errorf("Expected 2 searches, got %d", len(searches))
	}

	// Most recent should be first
	if searches[0].Query != "another query" {
		t.Errorf("Expected most recent query to be 'another query', got %s", searches[0].Query)
	}
}
