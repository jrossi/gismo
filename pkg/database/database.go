package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	db "github.com/jrossi/gismo/pkg/database/sqlc"
	_ "github.com/tursodatabase/go-libsql"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	conn    *sql.DB
	queries *db.Queries
	dbPath  string
}

func New(ctx context.Context) (*DB, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .claude directory: %w", err)
	}

	dbPath := filepath.Join(claudeDir, "gismo.db")
	return NewWithPath(ctx, dbPath)
}

func NewWithPath(ctx context.Context, dbPath string) (*DB, error) {
	connStr := fmt.Sprintf("file:%s", dbPath)
	conn, err := sql.Open("libsql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	d := &DB{
		conn:    conn,
		queries: db.New(conn),
		dbPath:  dbPath,
	}

	if err := d.migrate(ctx); err != nil {
		d.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return d, nil
}

func (d *DB) migrate(ctx context.Context) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		content, err := migrationsFS.ReadFile(filepath.Join("migrations", entry.Name()))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", entry.Name(), err)
		}

		// Split migration content by semicolons to handle multiple statements
		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}

			// Execute each statement
			if _, err := d.conn.ExecContext(ctx, stmt); err != nil {
				// If this is the vector index migration, we can ignore the error
				if entry.Name() == "002_vector_index.sql" {
					// Vector extension not available, but that's okay for basic functionality
					continue
				}
				return fmt.Errorf("failed to execute statement in migration %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

func (d *DB) Queries() *db.Queries {
	return d.queries
}

func (d *DB) Conn() *sql.DB {
	return d.conn
}

func (d *DB) Close() error {
	if d.conn != nil {
		return d.conn.Close()
	}
	return nil
}

func (d *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return d.conn.BeginTx(ctx, opts)
}

func (d *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.conn.ExecContext(ctx, query, args...)
}

func (d *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.conn.QueryContext(ctx, query, args...)
}

func (d *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return d.conn.QueryRowContext(ctx, query, args...)
}
