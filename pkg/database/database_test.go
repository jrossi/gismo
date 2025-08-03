package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	sqlcdb "github.com/jrossi/gismo/pkg/database/sqlc"
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

	if db.queries == nil {
		t.Error("Database queries is nil")
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

	// Verify tables exist by attempting to query them
	var count int
	err = db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM code_chunks").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query code_chunks table: %v", err)
	}

	err = db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM search_history").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query search_history table: %v", err)
	}

	err = db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM index_stats").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query index_stats table: %v", err)
	}
}

func TestDatabaseOperations(t *testing.T) {
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

	// Create a test project first
	testProject, err := db.queries.InsertProject(ctx, sqlcdb.InsertProjectParams{
		ProjectName: "-test-project",
		ProjectPath: "/test/project",
		Description: sql.NullString{String: "Test project", Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// Test basic insert
	_, err = db.queries.InsertCodeChunkWithoutEmbedding(ctx, sqlcdb.InsertCodeChunkWithoutEmbeddingParams{
		ProjectID:    testProject.ID,
		FilePath:     "test.go",
		AbsolutePath: "/test/project/test.go",
		Content:      "func TestFunction() {}",
		ChunkType:    "function",
		Language:     "go",
		StartLine:    1,
		EndLine:      1,
		Metadata:     sql.NullString{},
	})
	if err != nil {
		t.Errorf("Failed to insert code chunk: %v", err)
	}

	// Test query
	chunks, err := db.queries.GetCodeChunksByFilePath(ctx, sqlcdb.GetCodeChunksByFilePathParams{
		ProjectID: testProject.ID,
		FilePath:  "test.go",
	})
	if err != nil {
		t.Errorf("Failed to get code chunks: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0].Content != "func TestFunction() {}" {
		t.Errorf("Expected content 'func TestFunction() {}', got %s", chunks[0].Content)
	}
}

func TestTransactions(t *testing.T) {
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

	// Create a test project first
	testProject, err := db.queries.InsertProject(ctx, sqlcdb.InsertProjectParams{
		ProjectName: "-test-project",
		ProjectPath: "/test/project",
		Description: sql.NullString{String: "Test project", Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Use transaction for insert
	_, err = tx.ExecContext(ctx, `
		INSERT INTO code_chunks (project_id, file_path, absolute_path, content, chunk_type, language, start_line, end_line)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, testProject.ID, "transaction_test.go", "/test/project/transaction_test.go", "func TransactionTest() {}", "function", "go", 1, 1)
	if err != nil {
		t.Errorf("Failed to insert in transaction: %v", err)
	}

	// Rollback and verify no data was saved
	err = tx.Rollback()
	if err != nil {
		t.Errorf("Failed to rollback transaction: %v", err)
	}

	chunks, err := db.queries.GetCodeChunksByFilePath(ctx, sqlcdb.GetCodeChunksByFilePathParams{
		ProjectID: testProject.ID,
		FilePath:  "transaction_test.go",
	})
	if err != nil {
		t.Errorf("Failed to query after rollback: %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks after rollback, got %d", len(chunks))
	}
}
