package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewWithPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("Database file not created: %v", err)
	}

	// Test we can get the DB connection
	db := store.DB()
	if db == nil {
		t.Error("DB() returned nil")
	}

	// Verify we can query the database
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM docsets").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query docsets table: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 docsets, got %d", count)
	}
}

func TestStoreReset(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewWithPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Insert a test docset
	_, err = store.DB().ExecContext(ctx, `
		INSERT INTO docsets (id, name, version, language, source_url, source_type)
		VALUES ('test-id', 'Test Docset', '1.0', 'go', 'http://example.com', 'custom')
	`)
	if err != nil {
		t.Fatalf("Failed to insert test docset: %v", err)
	}

	// Verify docset exists
	var count int
	err = store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM docsets").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count docsets: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 docset before reset, got %d", count)
	}

	// Reset the store
	if err := store.Reset(ctx); err != nil {
		t.Fatalf("Failed to reset store: %v", err)
	}

	// Verify docset is gone
	err = store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM docsets").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count docsets after reset: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 docsets after reset, got %d", count)
	}
}

func TestStoreSchema(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewWithPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Test that all expected tables exist
	tables := []string{"docsets", "docset_content"}
	for _, table := range tables {
		var name string
		err := store.DB().QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s does not exist: %v", table, err)
		}
	}

	// Test that we can insert into docset_content with all fields
	_, err = store.DB().ExecContext(ctx, `
		INSERT INTO docsets (id, name, version, language, source_url, source_type)
		VALUES ('test-id', 'Test', '1.0', 'go', 'http://test.com', 'custom')
	`)
	if err != nil {
		t.Fatalf("Failed to insert docset: %v", err)
	}

	_, err = store.DB().ExecContext(ctx, `
		INSERT INTO docset_content (docset_id, name, type, path, content, summary)
		VALUES ('test-id', 'TestFunc', 'Function', '/test/func', 'func TestFunc()', 'Test function')
	`)
	if err != nil {
		t.Fatalf("Failed to insert docset content: %v", err)
	}
}
