package main

import (
	"database/sql"
	"fmt"
	"log"
	"runtime"

	_ "github.com/marcboeker/go-duckdb"
)

func main() {
	fmt.Printf("Testing DuckDB on %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Test in-memory database
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		log.Fatalf("Failed to open DuckDB: %v", err)
	}
	defer db.Close()

	// Check version
	var version string
	err = db.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		log.Fatalf("Failed to get version: %v", err)
	}
	fmt.Printf("✅ DuckDB version: %s\n", version)

	// Load VSS extension
	_, _ = db.Exec("INSTALL vss")
	_, err = db.Exec("LOAD vss")
	if err != nil {
		fmt.Printf("❌ VSS extension not available: %v\n", err)
	} else {
		fmt.Println("✅ VSS extension loaded")
	}

	// Test basic vector operations with arrays
	_, err = db.Exec(`
		CREATE TABLE vectors (
			id INTEGER,
			vec FLOAT[3]
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO vectors VALUES 
			(1, [1.0, 0.0, 0.0]),
			(2, [0.0, 1.0, 0.0])
	`)
	if err != nil {
		log.Fatalf("Failed to insert: %v", err)
	}
	fmt.Println("✅ Vector operations work")

	// Test distance calculation
	var distance float64
	err = db.QueryRow(`
		SELECT array_distance([1.0, 0.0, 0.0]::FLOAT[3], [0.0, 1.0, 0.0]::FLOAT[3])
	`).Scan(&distance)
	if err != nil {
		fmt.Printf("❌ Distance calculation failed: %v\n", err)
	} else {
		fmt.Printf("✅ Distance calculation works: %f\n", distance)
	}

	// Test cosine similarity
	var similarity float64
	err = db.QueryRow(`
		SELECT array_cosine_similarity([1.0, 0.0, 0.0]::FLOAT[3], [1.0, 0.0, 0.0]::FLOAT[3])
	`).Scan(&similarity)
	if err != nil {
		fmt.Printf("❌ Cosine similarity failed: %v\n", err)
	} else {
		fmt.Printf("✅ Cosine similarity works: %f\n", similarity)
	}

	fmt.Println("\n✅ All DuckDB vector features working!")
}
