package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/marcboeker/go-duckdb"
)

// Store represents the knowledge database
type Store struct {
	db *sql.DB
}

// New creates a new knowledge store at the default location
func New(ctx context.Context) (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create ~/.gismo directory if it doesn't exist
	gismoDir := filepath.Join(homeDir, ".gismo")
	if err := os.MkdirAll(gismoDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .gismo directory: %w", err)
	}

	dbPath := filepath.Join(gismoDir, "knowledge.db")
	return NewWithPath(ctx, dbPath)
}

// NewWithPath creates a new knowledge store at the specified path
func NewWithPath(ctx context.Context, dbPath string) (*Store, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open DuckDB connection
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Install and load VSS extension for vector search
	_, _ = db.ExecContext(ctx, "INSTALL vss")
	_, _ = db.ExecContext(ctx, "LOAD vss")

	s := &Store{db: db}

	// Initialize schema
	if err := s.initSchema(ctx); err != nil {
		s.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return s, nil
}

// initSchema creates the database schema if it doesn't exist
func (s *Store) initSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, createSchema)
	return err
}

// DB returns the underlying database connection
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Reset drops all tables and recreates the schema
func (s *Store) Reset(ctx context.Context) error {
	// Drop existing schema
	if _, err := s.db.ExecContext(ctx, dropSchema); err != nil {
		return fmt.Errorf("failed to drop schema: %w", err)
	}

	// Recreate schema
	if err := s.initSchema(ctx); err != nil {
		return fmt.Errorf("failed to recreate schema: %w", err)
	}

	return nil
}
