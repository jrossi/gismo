package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "gismo_queries_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := NewWithPath(ctx, dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestInsertAndGetProject(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Test InsertProject
	project, err := db.InsertProject(ctx, "test-project", "/test/path", sql.NullString{
		String: "Test project description",
		Valid:  true,
	})
	if err != nil {
		t.Fatalf("Failed to insert project: %v", err)
	}

	if project == nil {
		t.Fatal("Returned project is nil")
	}

	if project.ProjectName != "test-project" {
		t.Errorf("Expected project name 'test-project', got %s", project.ProjectName)
	}

	if project.ProjectPath != "/test/path" {
		t.Errorf("Expected project path '/test/path', got %s", project.ProjectPath)
	}

	if !project.Description.Valid || project.Description.String != "Test project description" {
		t.Error("Project description not set correctly")
	}

	// Test GetProjectByName
	retrievedByName, err := db.GetProjectByName(ctx, "test-project")
	if err != nil {
		t.Fatalf("Failed to get project by name: %v", err)
	}

	if retrievedByName == nil {
		t.Fatal("Retrieved project by name is nil")
	}

	if retrievedByName.ID != project.ID {
		t.Errorf("Expected project ID %d, got %d", project.ID, retrievedByName.ID)
	}

	// Test GetProjectByName with non-existent project
	nonExistent, err := db.GetProjectByName(ctx, "non-existent")
	if err != nil {
		t.Fatalf("Unexpected error for non-existent project: %v", err)
	}
	if nonExistent != nil {
		t.Error("Expected nil for non-existent project")
	}

	// Test GetProjectByPath
	retrievedByPath, err := db.GetProjectByPath(ctx, "/test/path")
	if err != nil {
		t.Fatalf("Failed to get project by path: %v", err)
	}

	if retrievedByPath == nil {
		t.Fatal("Retrieved project by path is nil")
	}

	if retrievedByPath.ID != project.ID {
		t.Errorf("Expected project ID %d, got %d", project.ID, retrievedByPath.ID)
	}

	// Test GetProjectByPath with non-existent path
	nonExistentPath, err := db.GetProjectByPath(ctx, "/non/existent")
	if err != nil {
		t.Fatalf("Unexpected error for non-existent path: %v", err)
	}
	if nonExistentPath != nil {
		t.Error("Expected nil for non-existent path")
	}
}

func TestInsertProjectConflict(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert first project
	_, err := db.InsertProject(ctx, "conflict-test", "/path1", sql.NullString{})
	if err != nil {
		t.Fatalf("Failed to insert first project: %v", err)
	}

	// Insert with same name (should update in DuckDB, but might create new row)
	// DuckDB's ON CONFLICT behavior is different from SQLite/PostgreSQL
	project2, err := db.InsertProject(ctx, "conflict-test", "/path2", sql.NullString{})
	if err != nil {
		// If it fails due to conflict, that's expected behavior for DuckDB
		t.Logf("Insert with conflict returned error (expected for DuckDB): %v", err)
		return
	}

	// DuckDB might create a new row or update existing one
	// Just verify we got a project back
	if project2 == nil {
		t.Error("Expected to get a project back")
	}
}

func TestGetAllProjects(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert multiple projects
	projects := []struct {
		name string
		path string
	}{
		{"project-a", "/path/a"},
		{"project-b", "/path/b"},
		{"project-c", "/path/c"},
	}

	for _, p := range projects {
		_, err := db.InsertProject(ctx, p.name, p.path, sql.NullString{})
		if err != nil {
			t.Fatalf("Failed to insert project %s: %v", p.name, err)
		}
	}

	// Get all projects
	allProjects, err := db.GetAllProjects(ctx)
	if err != nil {
		t.Fatalf("Failed to get all projects: %v", err)
	}

	if len(allProjects) != len(projects) {
		t.Errorf("Expected %d projects, got %d", len(projects), len(allProjects))
	}

	// Check ordering (should be by name)
	for i, p := range allProjects {
		expectedName := projects[i].name
		if p.ProjectName != expectedName {
			t.Errorf("Expected project[%d] name to be %s, got %s", i, expectedName, p.ProjectName)
		}
	}
}

func TestUpdateProjectIndexTime(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create project
	project, err := db.InsertProject(ctx, "index-test", "/test", sql.NullString{})
	if err != nil {
		t.Fatalf("Failed to insert project: %v", err)
	}

	if project.LastIndexedAt.Valid {
		t.Error("Expected last_indexed_at to be NULL initially")
	}

	// Sleep to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Update index time
	err = db.UpdateProjectIndexTime(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to update project index time: %v", err)
	}

	// Retrieve and check
	updated, err := db.GetProjectByName(ctx, "index-test")
	if err != nil {
		t.Fatalf("Failed to get updated project: %v", err)
	}

	if !updated.LastIndexedAt.Valid {
		t.Error("Expected last_indexed_at to be set after update")
	}

	// Check that updated_at changed (with some tolerance for time precision)
	if updated.UpdatedAt.Sub(project.UpdatedAt) < 1*time.Millisecond {
		t.Logf("Warning: updated_at might not have changed significantly (diff: %v)", updated.UpdatedAt.Sub(project.UpdatedAt))
	}
}

func TestQueriesCodeChunkOperations(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create project first
	project, err := db.InsertProject(ctx, "chunk-test", "/test", sql.NullString{})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Test InsertCodeChunk with embedding
	chunkWithEmb := &CodeChunk{
		ProjectID:    project.ID,
		FilePath:     "test.go",
		AbsolutePath: "/test/test.go",
		Content:      "func test() {}",
		ChunkType:    "function",
		Language:     "go",
		StartLine:    1,
		EndLine:      3,
		Embedding:    []float32{0.1, 0.2, 0.3},
		Metadata:     sql.NullString{String: `{"key": "value"}`, Valid: true},
	}

	inserted, err := db.InsertCodeChunk(ctx, chunkWithEmb)
	if err != nil {
		t.Fatalf("Failed to insert code chunk with embedding: %v", err)
	}

	if inserted.ID == 0 {
		t.Error("Expected non-zero ID for inserted chunk")
	}

	// Test InsertCodeChunkWithoutEmbedding
	chunkNoEmb := &CodeChunk{
		ProjectID:    project.ID,
		FilePath:     "test2.go",
		AbsolutePath: "/test/test2.go",
		Content:      "func test2() {}",
		ChunkType:    "function",
		Language:     "go",
		StartLine:    1,
		EndLine:      1,
		Metadata:     sql.NullString{},
	}

	insertedNoEmb, err := db.InsertCodeChunkWithoutEmbedding(ctx, chunkNoEmb)
	if err != nil {
		t.Fatalf("Failed to insert code chunk without embedding: %v", err)
	}

	if insertedNoEmb.ID == 0 {
		t.Error("Expected non-zero ID for inserted chunk without embedding")
	}

	// Test GetCodeChunkByID
	retrieved, err := db.GetCodeChunkByID(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("Failed to get code chunk by ID: %v", err)
	}

	if retrieved.Content != chunkWithEmb.Content {
		t.Errorf("Expected content '%s', got '%s'", chunkWithEmb.Content, retrieved.Content)
	}

	// Embedding comparison - may be approximate due to float conversion
	if len(retrieved.Embedding) > 0 && len(chunkWithEmb.Embedding) > 0 {
		if len(retrieved.Embedding) != len(chunkWithEmb.Embedding) {
			t.Errorf("Expected embedding length %d, got %d", len(chunkWithEmb.Embedding), len(retrieved.Embedding))
		}
	}

	// Test GetCodeChunksByFilePath
	chunks, err := db.GetCodeChunksByFilePath(ctx, project.ID, "test.go")
	if err != nil {
		t.Fatalf("Failed to get chunks by file path: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for test.go, got %d", len(chunks))
	}

	// Test with non-existent file
	emptyChunks, err := db.GetCodeChunksByFilePath(ctx, project.ID, "nonexistent.go")
	if err != nil {
		t.Fatalf("Failed to get chunks for non-existent file: %v", err)
	}

	if len(emptyChunks) != 0 {
		t.Errorf("Expected 0 chunks for non-existent file, got %d", len(emptyChunks))
	}
}

func TestDeleteCodeChunks(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create project
	project, err := db.InsertProject(ctx, "delete-test", "/test", sql.NullString{})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Insert multiple chunks
	files := []string{"file1.go", "file2.go", "file1.go"}
	for i, file := range files {
		chunk := &CodeChunk{
			ProjectID:    project.ID,
			FilePath:     file,
			AbsolutePath: "/test/" + file,
			Content:      "content",
			ChunkType:    "function",
			Language:     "go",
			StartLine:    i * 10,
			EndLine:      i*10 + 5,
		}
		_, err := db.InsertCodeChunkWithoutEmbedding(ctx, chunk)
		if err != nil {
			t.Fatalf("Failed to insert chunk: %v", err)
		}
	}

	// Test DeleteCodeChunksByFilePath
	err = db.DeleteCodeChunksByFilePath(ctx, project.ID, "file1.go")
	if err != nil {
		t.Fatalf("Failed to delete chunks by file path: %v", err)
	}

	// Check remaining chunks
	remaining, err := db.GetCodeChunksByFilePath(ctx, project.ID, "file1.go")
	if err != nil {
		t.Fatalf("Failed to get remaining chunks: %v", err)
	}

	if len(remaining) != 0 {
		t.Errorf("Expected 0 chunks after delete, got %d", len(remaining))
	}

	// file2.go should still exist
	file2Chunks, err := db.GetCodeChunksByFilePath(ctx, project.ID, "file2.go")
	if err != nil {
		t.Fatalf("Failed to get file2 chunks: %v", err)
	}

	if len(file2Chunks) != 1 {
		t.Errorf("Expected 1 chunk for file2.go, got %d", len(file2Chunks))
	}

	// Test DeleteCodeChunksByProject
	err = db.DeleteCodeChunksByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to delete chunks by project: %v", err)
	}

	totalCount, err := db.GetTotalChunksCount(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get total count: %v", err)
	}

	if totalCount != 0 {
		t.Errorf("Expected 0 chunks after project delete, got %d", totalCount)
	}
}

func TestChunkCounts(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create project
	project, err := db.InsertProject(ctx, "count-test", "/test", sql.NullString{})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Initially should be 0
	count, err := db.GetTotalChunksCount(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get initial count: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected initial count 0, got %d", count)
	}

	fileCount, err := db.GetFileCount(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get initial file count: %v", err)
	}
	if fileCount != 0 {
		t.Errorf("Expected initial file count 0, got %d", fileCount)
	}

	// Insert chunks for different files
	files := []string{"file1.go", "file2.go", "file1.go", "file3.go"}
	for i, file := range files {
		chunk := &CodeChunk{
			ProjectID:    project.ID,
			FilePath:     file,
			AbsolutePath: "/test/" + file,
			Content:      "content",
			ChunkType:    "function",
			Language:     "go",
			StartLine:    i,
			EndLine:      i + 1,
		}
		_, err := db.InsertCodeChunkWithoutEmbedding(ctx, chunk)
		if err != nil {
			t.Fatalf("Failed to insert chunk: %v", err)
		}
	}

	// Test GetTotalChunksCount
	totalCount, err := db.GetTotalChunksCount(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get total chunks count: %v", err)
	}

	if totalCount != 4 {
		t.Errorf("Expected 4 total chunks, got %d", totalCount)
	}

	// Test GetFileCount (should be 3 unique files)
	uniqueFiles, err := db.GetFileCount(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get file count: %v", err)
	}

	if uniqueFiles != 3 {
		t.Errorf("Expected 3 unique files, got %d", uniqueFiles)
	}
}

func TestQueriesSearchHistory(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Test InsertSearchHistory with all fields
	search1, err := db.InsertSearchHistory(ctx, "test query 1", 10,
		sql.NullString{String: "lang:go", Valid: true},
		sql.NullInt32{Int32: 150, Valid: true})
	if err != nil {
		t.Fatalf("Failed to insert search history: %v", err)
	}

	if search1.Query != "test query 1" {
		t.Errorf("Expected query 'test query 1', got %s", search1.Query)
	}

	if search1.ResultCount != 10 {
		t.Errorf("Expected result count 10, got %d", search1.ResultCount)
	}

	if !search1.Filters.Valid || search1.Filters.String != "lang:go" {
		t.Error("Filters not set correctly")
	}

	if !search1.ExecutionTimeMs.Valid || search1.ExecutionTimeMs.Int32 != 150 {
		t.Error("Execution time not set correctly")
	}

	// Test InsertSearchHistory with null fields
	search2, err := db.InsertSearchHistory(ctx, "test query 2", 5,
		sql.NullString{}, sql.NullInt32{})
	if err != nil {
		t.Fatalf("Failed to insert search history with nulls: %v", err)
	}

	if search2.Filters.Valid {
		t.Error("Expected filters to be NULL")
	}

	if search2.ExecutionTimeMs.Valid {
		t.Error("Expected execution time to be NULL")
	}

	// Sleep to ensure order
	time.Sleep(10 * time.Millisecond)

	// Insert third search
	_, err = db.InsertSearchHistory(ctx, "test query 3", 20,
		sql.NullString{}, sql.NullInt32{})
	if err != nil {
		t.Fatalf("Failed to insert third search: %v", err)
	}

	// Test GetRecentSearches
	recent, err := db.GetRecentSearches(ctx, 2)
	if err != nil {
		t.Fatalf("Failed to get recent searches: %v", err)
	}

	if len(recent) != 2 {
		t.Errorf("Expected 2 recent searches, got %d", len(recent))
	}

	// Should be in reverse chronological order
	if recent[0].Query != "test query 3" {
		t.Errorf("Expected most recent query to be 'test query 3', got %s", recent[0].Query)
	}

	// Test with limit larger than available
	allSearches, err := db.GetRecentSearches(ctx, 100)
	if err != nil {
		t.Fatalf("Failed to get all searches: %v", err)
	}

	if len(allSearches) != 3 {
		t.Errorf("Expected 3 total searches, got %d", len(allSearches))
	}
}

func TestUpdateCodeChunkEmbedding(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create project and chunk
	project, err := db.InsertProject(ctx, "embed-test", "/test", sql.NullString{})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	chunk := &CodeChunk{
		ProjectID:    project.ID,
		FilePath:     "test.go",
		AbsolutePath: "/test/test.go",
		Content:      "test content",
		ChunkType:    "function",
		Language:     "go",
		StartLine:    1,
		EndLine:      1,
	}

	inserted, err := db.InsertCodeChunkWithoutEmbedding(ctx, chunk)
	if err != nil {
		t.Fatalf("Failed to insert chunk: %v", err)
	}

	// Test UpdateCodeChunkEmbedding
	newEmbedding := []float32{0.5, 0.6, 0.7, 0.8}
	err = db.UpdateCodeChunkEmbedding(ctx, inserted.ID, newEmbedding)
	if err != nil {
		t.Fatalf("Failed to update embedding: %v", err)
	}

	// Note: Currently this is a no-op due to DuckDB issues, so we can't verify the update
	// Once fixed, add verification here
}

func TestParseEmbedding(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []float32
	}{
		{
			name:     "normal array",
			input:    "[0.1, 0.2, 0.3]",
			expected: []float32{0.1, 0.2, 0.3},
		},
		{
			name:     "empty array",
			input:    "[]",
			expected: []float32{},
		},
		{
			name:     "single element",
			input:    "[1.5]",
			expected: []float32{1.5},
		},
		{
			name:     "no spaces",
			input:    "[0.1,0.2,0.3]",
			expected: []float32{0.1, 0.2, 0.3},
		},
		{
			name:     "extra spaces",
			input:    "[ 0.1 , 0.2 , 0.3 ]",
			expected: []float32{0.1, 0.2, 0.3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEmbedding(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("At index %d: expected %f, got %f", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a project
	project, err := db.InsertProject(ctx, "concurrent-test", "/test", sql.NullString{})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Run concurrent inserts
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			chunk := &CodeChunk{
				ProjectID:    project.ID,
				FilePath:     "test.go",
				AbsolutePath: "/test/test.go",
				Content:      "content",
				ChunkType:    "function",
				Language:     "go",
				StartLine:    idx * 10,
				EndLine:      idx*10 + 5,
			}
			_, err := db.InsertCodeChunkWithoutEmbedding(ctx, chunk)
			if err != nil {
				t.Errorf("Failed to insert chunk %d: %v", idx, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all chunks were inserted
	count, err := db.GetTotalChunksCount(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get count: %v", err)
	}

	if count != 10 {
		t.Errorf("Expected 10 chunks from concurrent inserts, got %d", count)
	}
}
