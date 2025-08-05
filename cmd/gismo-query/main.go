package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jrossi/gismo/pkg/client/knowledge"
	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

var (
	versionFlag = flag.Bool("version", false, "Show version information")
	helpFlag    = flag.Bool("help", false, "Show help information")
	formatFlag  = flag.String("format", "table", "Output format: table, json, csv")
	streamFlag  = flag.Bool("stream", false, "Stream results for large queries")
	maxRows     = flag.Int("max-rows", 0, "Maximum number of rows to return (0 = unlimited)")
	timeoutFlag = flag.Duration("timeout", 30*time.Second, "Query timeout")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		fmt.Println("gismo-query v0.1.0")
		os.Exit(0)
	}

	if *helpFlag {
		usage()
		os.Exit(0)
	}

	// Get SQL query from arguments
	args := flag.Args()

	if len(args) > 0 {
		// Query from command line arguments
		query := strings.Join(args, " ")
		if err := executeQuery(query); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Check if stdin is a pipe
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Reading from pipe
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
				os.Exit(1)
			}
			query := strings.TrimSpace(string(data))
			if query == "" {
				fmt.Fprintf(os.Stderr, "Error: No SQL query provided\n")
				usage()
				os.Exit(1)
			}
			if err := executeQuery(query); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Interactive mode
			if err := runInteractive(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `gismo-query - Execute SQL queries against the gismo knowledge database

Usage:
  gismo-query [flags] [SQL query]
  echo "SELECT * FROM docsets" | gismo-query [flags]

Examples:
  # List all docsets
  gismo-query "SELECT * FROM docsets"

  # Search for content
  gismo-query "SELECT name, type, path FROM docset_content WHERE name LIKE '%%http%%' LIMIT 10"

  # Get docset statistics
  gismo-query "SELECT docset_id, COUNT(*) as count FROM docset_content GROUP BY docset_id"

  # Stream large results
  gismo-query --stream "SELECT * FROM docset_content"

  # Output as JSON
  gismo-query --format json "SELECT id, name, version FROM docsets"

Flags:
`)
	flag.PrintDefaults()
}

func executeQuery(query string) error {
	// Connect to gismo server
	client, err := knowledge.New()
	if err != nil {
		return fmt.Errorf("failed to connect to gismo server: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	return executeQueryWithClient(ctx, client, query)
}

func executeStreamingQuery(ctx context.Context, client *knowledge.Client, query string) error {
	var columns []string
	rowCount := 0

	err := client.ExecuteQueryStream(ctx, query, func(result *gismov1.QueryResult) error {
		switch r := result.Result.(type) {
		case *gismov1.QueryResult_Metadata:
			columns = r.Metadata.Columns
			if *formatFlag == "table" {
				printTableHeader(columns)
			} else if *formatFlag == "csv" {
				fmt.Println(strings.Join(columns, ","))
			}

		case *gismov1.QueryResult_Row:
			rowCount++
			switch *formatFlag {
			case "json":
				data, _ := json.Marshal(r.Row.AsMap())
				fmt.Println(string(data))
			case "csv":
				values := make([]string, len(columns))
				rowMap := r.Row.AsMap()
				for i, col := range columns {
					if val, ok := rowMap[col]; ok {
						values[i] = fmt.Sprintf("%v", val)
					}
				}
				fmt.Println(strings.Join(values, ","))
			default:
				printTableRow(columns, r.Row.AsMap())
			}

		case *gismov1.QueryResult_Error:
			return fmt.Errorf("query error: %s", r.Error)

		case *gismov1.QueryResult_Complete:
			if *formatFlag != "json" {
				fmt.Fprintf(os.Stderr, "\n%d rows returned\n", r.Complete.TotalRows)
			}
		}
		return nil
	})

	return err
}

func outputTable(resp *gismov1.QueryResponse) error {
	if len(resp.Rows) == 0 {
		fmt.Println("No results")
		return nil
	}

	// Create tabwriter
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Print header
	fmt.Fprintln(w, strings.Join(resp.Columns, "\t"))
	fmt.Fprintln(w, strings.Repeat("-", 80))

	// Print rows
	for _, row := range resp.Rows {
		values := make([]string, len(resp.Columns))
		rowMap := row.AsMap()
		for i, col := range resp.Columns {
			if val, ok := rowMap[col]; ok {
				values[i] = fmt.Sprintf("%v", val)
			} else {
				values[i] = "NULL"
			}
		}
		fmt.Fprintln(w, strings.Join(values, "\t"))
	}

	fmt.Fprintf(os.Stderr, "\n%d rows returned\n", len(resp.Rows))
	return nil
}

func outputJSON(resp *gismov1.QueryResponse) error {
	// Convert to simple structure
	results := make([]map[string]interface{}, len(resp.Rows))
	for i, row := range resp.Rows {
		results[i] = row.AsMap()
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

func outputCSV(resp *gismov1.QueryResponse) error {
	// Print header
	fmt.Println(strings.Join(resp.Columns, ","))

	// Print rows
	for _, row := range resp.Rows {
		values := make([]string, len(resp.Columns))
		rowMap := row.AsMap()
		for i, col := range resp.Columns {
			if val, ok := rowMap[col]; ok {
				// Simple CSV escaping
				str := fmt.Sprintf("%v", val)
				if strings.Contains(str, ",") || strings.Contains(str, "\"") || strings.Contains(str, "\n") {
					str = "\"" + strings.ReplaceAll(str, "\"", "\"\"") + "\""
				}
				values[i] = str
			} else {
				values[i] = ""
			}
		}
		fmt.Println(strings.Join(values, ","))
	}

	return nil
}

func printTableHeader(columns []string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(columns, "\t"))
	fmt.Fprintln(w, strings.Repeat("-", 80))
	w.Flush()
}

func printTableRow(columns []string, rowMap map[string]interface{}) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	values := make([]string, len(columns))
	for i, col := range columns {
		if val, ok := rowMap[col]; ok {
			values[i] = fmt.Sprintf("%v", val)
		} else {
			values[i] = "NULL"
		}
	}
	fmt.Fprintln(w, strings.Join(values, "\t"))
	w.Flush()
}

func runInteractive() error {
	// Connect to gismo server
	client, err := knowledge.New()
	if err != nil {
		return fmt.Errorf("failed to connect to gismo server: %w", err)
	}
	defer client.Close()

	fmt.Println("gismo-query interactive mode")
	fmt.Println("Type your SQL queries, or use:")
	fmt.Println("  .help     - Show this help")
	fmt.Println("  .tables   - List all tables")
	fmt.Println("  .schema   - Show table schemas")
	fmt.Println("  .quit     - Exit")
	fmt.Println("  .exit     - Exit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("gismo> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Handle special commands
		switch strings.ToLower(line) {
		case ".quit", ".exit":
			fmt.Println("Goodbye!")
			return nil

		case ".help":
			fmt.Println("Commands:")
			fmt.Println("  .help     - Show this help")
			fmt.Println("  .tables   - List all tables")
			fmt.Println("  .schema   - Show table schemas")
			fmt.Println("  .quit     - Exit")
			fmt.Println("  .exit     - Exit")
			fmt.Println()
			fmt.Println("Or enter any valid SQL query.")
			continue

		case ".tables":
			line = "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"

		case ".schema":
			line = "SELECT sql FROM sqlite_master WHERE type='table' ORDER BY name"
		}

		// Skip other dot commands
		if strings.HasPrefix(line, ".") {
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", line)
			continue
		}

		// Execute the query
		ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
		err := executeQueryWithClient(ctx, client, line)
		cancel()

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func executeQueryWithClient(ctx context.Context, client *knowledge.Client, query string) error {
	if *streamFlag {
		return executeStreamingQuery(ctx, client, query)
	}

	// Execute non-streaming query
	resp, err := client.ExecuteQuery(ctx, query, int32(*maxRows))
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// Check for SQL errors
	if resp.Error != "" {
		return fmt.Errorf("SQL error: %s", resp.Error)
	}

	// Format and display results
	switch *formatFlag {
	case "json":
		return outputJSON(resp)
	case "csv":
		return outputCSV(resp)
	default:
		return outputTable(resp)
	}
}
