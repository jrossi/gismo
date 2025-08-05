package main

import (
	"database/sql"
	"fmt"
	"log"
	"runtime"

	_ "github.com/marcboeker/go-duckdb"
)

func main() {
	fmt.Printf("Testing DuckDB vector search on %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Create in-memory DuckDB database
	db, err := sql.Open("duckdb", "")
	if err != nil {
		log.Fatalf("Failed to open DuckDB: %v", err)
	}
	defer db.Close()

	// Check DuckDB version
	var version string
	err = db.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		log.Fatalf("Failed to get DuckDB version: %v", err)
	}
	fmt.Printf("DuckDB version: %s\n", version)

	// Install and load the VSS extension
	fmt.Println("\nInstalling VSS extension...")
	_, err = db.Exec("INSTALL vss")
	if err != nil {
		log.Printf("Warning: Failed to install VSS extension: %v", err)
		fmt.Println("This might be because it's already installed or not available")
	}

	_, err = db.Exec("LOAD vss")
	if err != nil {
		log.Printf("Warning: Failed to load VSS extension: %v", err)
		fmt.Println("VSS extension not available, trying basic array operations")
		testBasicArrays(db)
		return
	}

	fmt.Println("✅ VSS extension loaded successfully!")
	testVectorSearch(db)
}

func testBasicArrays(db *sql.DB) {
	fmt.Println("\n=== Testing Basic Array Operations ===")

	// Create table with array column
	_, err := db.Exec(`
		CREATE TABLE items (
			id INTEGER PRIMARY KEY,
			name TEXT,
			embedding FLOAT[3]
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table with array: %v", err)
	}

	// Insert vectors
	_, err = db.Exec(`
		INSERT INTO items VALUES 
			(1, 'item1', [1.0, 0.0, 0.0]),
			(2, 'item2', [0.0, 1.0, 0.0]),
			(3, 'item3', [0.0, 0.0, 1.0]),
			(4, 'item4', [0.5, 0.5, 0.0])
	`)
	if err != nil {
		log.Fatalf("Failed to insert vectors: %v", err)
	}
	fmt.Println("✅ Inserted test vectors")

	// Query with array distance
	fmt.Println("\nSearching for vectors similar to [1.0, 0.0, 0.0]:")
	rows, err := db.Query(`
		SELECT 
			id, 
			name,
			array_distance(embedding, [1.0, 0.0, 0.0]::FLOAT[3]) as distance
		FROM items
		ORDER BY distance
		LIMIT 3
	`)
	if err != nil {
		// Try alternative distance function
		rows, err = db.Query(`
			SELECT 
				id, 
				name,
				list_distance(embedding, [1.0, 0.0, 0.0]) as distance
			FROM items
			ORDER BY distance
			LIMIT 3
		`)
		if err != nil {
			log.Printf("Distance functions not available: %v", err)
			// Try basic query without distance
			rows, err = db.Query("SELECT id, name FROM items LIMIT 3")
			if err != nil {
				log.Fatalf("Failed to query: %v", err)
			}
			defer rows.Close()

			fmt.Println("Basic query results (no distance calculation):")
			for rows.Next() {
				var id int
				var name string
				err := rows.Scan(&id, &name)
				if err != nil {
					log.Printf("Failed to scan: %v", err)
					continue
				}
				fmt.Printf("  ID: %d, Name: %s\n", id, name)
			}
			return
		}
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name string
		var distance float64
		err := rows.Scan(&id, &name, &distance)
		if err != nil {
			log.Printf("Failed to scan: %v", err)
			continue
		}
		fmt.Printf("  ID: %d, Name: %s, Distance: %f\n", id, name, distance)
	}
}

func testVectorSearch(db *sql.DB) {
	fmt.Println("\n=== Testing Vector Search with VSS ===")

	// Create table with vectors
	_, err := db.Exec(`
		CREATE TABLE documents (
			id INTEGER PRIMARY KEY,
			content TEXT,
			embedding FLOAT[384]  -- Using a realistic embedding size
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Generate some test embeddings
	fmt.Println("Generating test embeddings...")
	for i := 1; i <= 100; i++ {
		// Create a simple pattern for test embeddings
		embeddingStr := "ARRAY["
		for j := 0; j < 384; j++ {
			if j > 0 {
				embeddingStr += ", "
			}
			// Simple pattern: different documents have different distributions
			val := float64(i) * 0.01 * float64(j%10)
			embeddingStr += fmt.Sprintf("%f", val)
		}
		embeddingStr += "]::FLOAT[384]"

		query := fmt.Sprintf(`
			INSERT INTO documents VALUES (
				%d, 
				'Document %d content', 
				%s
			)
		`, i, i, embeddingStr)

		_, err = db.Exec(query)
		if err != nil {
			log.Printf("Failed to insert document %d: %v", i, err)
			// Try simpler approach
			break
		}
	}

	// Create HNSW index
	fmt.Println("\nCreating HNSW index...")
	_, err = db.Exec(`
		CREATE INDEX idx_embedding ON documents 
		USING HNSW (embedding) 
		WITH (metric = 'cosine')
	`)
	if err != nil {
		log.Printf("Failed to create HNSW index: %v", err)
		fmt.Println("Will proceed with unindexed search")
	} else {
		fmt.Println("✅ HNSW index created")
	}

	// Search for similar vectors
	fmt.Println("\nPerforming similarity search...")

	// Create a query vector (same size as our embeddings)
	queryVector := "ARRAY["
	for i := 0; i < 384; i++ {
		if i > 0 {
			queryVector += ", "
		}
		queryVector += fmt.Sprintf("%f", float64(i%10)*0.1)
	}
	queryVector += "]::FLOAT[384]"

	query := fmt.Sprintf(`
		SELECT 
			id,
			content,
			array_cosine_similarity(embedding, %s) as similarity
		FROM documents
		ORDER BY similarity DESC
		LIMIT 5
	`, queryVector)

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Failed to perform similarity search: %v", err)
		return
	}
	defer rows.Close()

	fmt.Println("\nTop 5 similar documents:")
	for rows.Next() {
		var id int
		var content string
		var similarity float64
		err := rows.Scan(&id, &content, &similarity)
		if err != nil {
			log.Printf("Failed to scan: %v", err)
			continue
		}
		fmt.Printf("  ID: %d, Content: %s, Similarity: %f\n", id, content, similarity)
	}
}
