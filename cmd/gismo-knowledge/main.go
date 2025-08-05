package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jrossi/gismo/pkg/client/knowledge"
)

const version = "v0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "import":
		err = importCommand(os.Args[2:])
	case "list":
		err = listCommand(os.Args[2:])
	case "search":
		err = searchCommand(os.Args[2:])
	case "get":
		err = getCommand(os.Args[2:])
	case "remove":
		err = removeCommand(os.Args[2:])
	case "push":
		err = pushCommand(os.Args[2:])
	case "index":
		err = indexCommand(os.Args[2:])
	case "stats":
		err = statsCommand(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("gismo-knowledge", version)
		os.Exit(0)
	case "help", "--help", "-h":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`gismo-knowledge - Manage knowledge base, docsets, and indexed content

Usage:
  gismo-knowledge <command> [options]

Commands:
  import    Import a docset from URL or local path
  list      List installed docsets
  search    Search across all knowledge sources
  get       Get full content by ID
  remove    Remove a docset or knowledge source
  push      Push files or content to knowledge base
  index     Manage search indexes
  stats     Show knowledge base statistics
  version   Show version information
  help      Show this help message

Examples:
  # Import Go documentation
  gismo-knowledge import --url https://kapeli.com/feeds/Go.xml

  # Import from local docset
  gismo-knowledge import --path ~/Downloads/Go.docset

  # List all docsets
  gismo-knowledge list

  # Search for HTTP-related content
  gismo-knowledge search "http handler"

  # Push a file to knowledge base
  gismo-knowledge push --file README.md --type documentation

  # Show statistics
  gismo-knowledge stats

Run 'gismo-knowledge <command> --help' for more information on a command.
`)
}

func importCommand(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	url := fs.String("url", "", "Docset feed URL (e.g., https://kapeli.com/feeds/Go.xml)")
	path := fs.String("path", "", "Local docset path")
	name := fs.String("name", "", "Override docset name")
	force := fs.Bool("force", false, "Force reimport if already exists")
	timeout := fs.Duration("timeout", 10*time.Minute, "Import timeout")

	fs.Usage = func() {
		fmt.Print(`Import a docset from URL or local path

Usage:
  gismo-knowledge import [options]

Options:
`)
		fs.PrintDefaults()
		fmt.Print(`
Examples:
  # Import Go documentation from Kapeli
  gismo-knowledge import --url https://kapeli.com/feeds/Go.xml

  # Import from local docset with custom name
  gismo-knowledge import --path ~/Downloads/Go.docset --name "Go 1.21"

  # Force reimport
  gismo-knowledge import --url https://kapeli.com/feeds/Go.xml --force

Supported sources:
  - Kapeli Dash feeds (https://kapeli.com/feeds/)
  - Local .docset directories
  - Direct docset archive URLs (.tgz)
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *url == "" && *path == "" {
		fs.Usage()
		return fmt.Errorf("either --url or --path must be specified")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client, err := knowledge.New()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// For now, use the simplified client API
	if *url != "" {
		fmt.Printf("Importing docset from %s...\n", *url)
		err = client.ImportDocset(ctx, *url, func(stage, message string, percent int) {
			if percent > 0 {
				fmt.Printf("[%s] %s (%.1f%%)\n", stage, message, float64(percent))
			} else {
				fmt.Printf("[%s] %s\n", stage, message)
			}
		})
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}
		fmt.Println("✓ Import completed successfully")
	} else {
		// TODO: Add support for local path imports through client
		return fmt.Errorf("local path imports not yet implemented in client")
	}

	_ = name  // TODO: Use when client supports name override
	_ = force // TODO: Use when client supports force reimport

	return nil
}

func listCommand(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	format := fs.String("format", "table", "Output format: table, json, yaml")
	verbose := fs.Bool("v", false, "Verbose output")

	fs.Usage = func() {
		fmt.Print(`List installed docsets and knowledge sources

Usage:
  gismo-knowledge list [options]

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := knowledge.New()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	docsets, err := client.ListDocsets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list docsets: %w", err)
	}

	if len(docsets) == 0 {
		fmt.Println("No docsets installed. Use 'gismo-knowledge import' to add docsets.")
		return nil
	}

	switch *format {
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if *verbose {
			fmt.Fprintln(w, "ID\tNAME\tVERSION\tLANGUAGE\tENTRIES\tIMPORTED\tSOURCE")
			for _, ds := range docsets {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
					ds.Id,
					ds.Name,
					ds.Version,
					ds.Language,
					ds.ContentCount,
					ds.ImportedAt.AsTime().Format("2006-01-02"),
					ds.SourceUrl,
				)
			}
		} else {
			fmt.Fprintln(w, "NAME\tVERSION\tENTRIES\tIMPORTED")
			for _, ds := range docsets {
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
					ds.Name,
					ds.Version,
					ds.ContentCount,
					ds.ImportedAt.AsTime().Format("2006-01-02"),
				)
			}
		}
		w.Flush()
	default:
		return fmt.Errorf("unsupported format: %s", *format)
	}

	return nil
}

func searchCommand(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	docset := fs.String("docset", "", "Limit search to specific docset")
	searchType := fs.String("type", "keyword", "Search type: keyword, semantic, hybrid")
	limit := fs.Int("limit", 20, "Maximum results")
	verbose := fs.Bool("v", false, "Show content preview")
	showContent := fs.Bool("content", false, "Show full content for first result")
	showID := fs.Bool("id", false, "Show content IDs (for use with 'get' command)")

	fs.Usage = func() {
		fmt.Print(`Search across knowledge sources

Usage:
  gismo-knowledge search [options] <query>

Options:
`)
		fs.PrintDefaults()
		fmt.Print(`
Examples:
  # Simple keyword search
  gismo-knowledge search "http handler"

  # Search with content preview
  gismo-knowledge search -v "error handling"

  # Show full content for first result
  gismo-knowledge search --content "panic recover"

  # Show content IDs for retrieval
  gismo-knowledge search --id "context"

  # Search in specific docset
  gismo-knowledge search --docset Go "context deadline"

  # Show more results
  gismo-knowledge search --limit 50 "database connection"
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(fs.Args()) == 0 {
		fs.Usage()
		return fmt.Errorf("search query required")
	}

	query := strings.Join(fs.Args(), " ")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := knowledge.New()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	opts := knowledge.SearchOptions{
		Limit: *limit,
	}

	if *docset != "" {
		opts.DocsetIDs = []string{*docset}
	}

	switch *searchType {
	case "semantic":
		opts.Type = knowledge.SearchTypeSemantic
	case "hybrid":
		opts.Type = knowledge.SearchTypeHybrid
	default:
		opts.Type = knowledge.SearchTypeKeyword
	}

	results, err := client.Search(ctx, query, opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	fmt.Printf("Found %d results:\n\n", len(results))

	for i, result := range results {
		fmt.Printf("%d. %s (%s)\n", i+1, result.ItemName, result.ItemType)
		if *showID {
			fmt.Printf("   ID: %d\n", result.ContentId)
		}
		fmt.Printf("   Docset: %s\n", result.DocsetName)
		fmt.Printf("   Path: %s\n", result.Path)
		if result.RelevanceScore > 0 {
			fmt.Printf("   Score: %.3f\n", result.RelevanceScore)
		}
		if *verbose && result.ContentPreview != "" {
			fmt.Printf("   Preview:\n%s\n", indent(result.ContentPreview, "     "))
		}

		// Show full content for first result if requested
		if *showContent && i == 0 {
			fmt.Println("\n   --- Full Content ---")
			content, err := client.GetContent(ctx, result.ContentId)
			if err != nil {
				fmt.Printf("   Error fetching content: %v\n", err)
			} else if content != nil {
				if content.Summary != "" {
					fmt.Printf("   Summary: %s\n", content.Summary)
				}
				if content.Content != "" {
					fmt.Printf("   Content:\n%s\n", indent(content.Content, "   "))
				} else {
					fmt.Println("   (No content available)")
				}
			}
			fmt.Println("   --- End Content ---")
		}

		fmt.Println()
	}

	return nil
}

func getCommand(args []string) error {
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	format := fs.String("format", "text", "Output format: text, json, markdown")

	fs.Usage = func() {
		fmt.Print(`Get full content by ID

Usage:
  gismo-knowledge get [options] <content-id>

Options:
`)
		fs.PrintDefaults()
		fmt.Print(`
Examples:
  # Get content by ID
  gismo-knowledge get 123

  # Get content as JSON
  gismo-knowledge get --format json 123

  # Get content as Markdown
  gismo-knowledge get --format markdown 123
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(fs.Args()) == 0 {
		fs.Usage()
		return fmt.Errorf("content ID required")
	}

	// Parse content ID
	contentID := 0
	if _, err := fmt.Sscanf(fs.Args()[0], "%d", &contentID); err != nil {
		return fmt.Errorf("invalid content ID: %s", fs.Args()[0])
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := knowledge.New()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	content, err := client.GetContent(ctx, int32(contentID))
	if err != nil {
		return fmt.Errorf("failed to get content: %w", err)
	}

	switch *format {
	case "json":
		// Output as JSON (would need to implement JSON marshaling)
		fmt.Printf("{\n")
		fmt.Printf("  \"id\": %d,\n", contentID)
		fmt.Printf("  \"name\": %q,\n", content.Name)
		fmt.Printf("  \"type\": %q,\n", content.Type)
		fmt.Printf("  \"docset\": %q,\n", content.DocsetId)
		fmt.Printf("  \"path\": %q,\n", content.Path)
		if content.Summary != "" {
			fmt.Printf("  \"summary\": %q,\n", content.Summary)
		}
		fmt.Printf("  \"content\": %q\n", content.Content)
		fmt.Printf("}\n")

	case "markdown":
		fmt.Printf("# %s\n\n", content.Name)
		fmt.Printf("**Type:** %s  \n", content.Type)
		fmt.Printf("**Docset:** %s  \n", content.DocsetId)
		fmt.Printf("**Path:** `%s`  \n\n", content.Path)
		if content.Summary != "" {
			fmt.Printf("## Summary\n\n%s\n\n", content.Summary)
		}
		if content.Content != "" {
			fmt.Printf("## Content\n\n```\n%s\n```\n", content.Content)
		}

	default: // text
		fmt.Printf("Name: %s\n", content.Name)
		fmt.Printf("Type: %s\n", content.Type)
		fmt.Printf("Docset: %s\n", content.DocsetId)
		fmt.Printf("Path: %s\n", content.Path)
		if content.Summary != "" {
			fmt.Printf("\nSummary:\n%s\n", content.Summary)
		}
		if content.Content != "" {
			fmt.Printf("\nContent:\n%s\n", content.Content)
		} else {
			fmt.Println("\n(No content available)")
		}
	}

	return nil
}

func removeCommand(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	force := fs.Bool("force", false, "Skip confirmation")

	fs.Usage = func() {
		fmt.Print(`Remove a docset or knowledge source

Usage:
  gismo-knowledge remove [options] <docset-name>

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(fs.Args()) == 0 {
		fs.Usage()
		return fmt.Errorf("docset name required")
	}

	docsetID := fs.Args()[0]

	if !*force {
		fmt.Printf("Remove docset '%s'? [y/N] ", docsetID)
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Canceled")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := knowledge.New()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	err = client.RemoveDocset(ctx, docsetID)
	if err != nil {
		return fmt.Errorf("failed to remove docset: %w", err)
	}

	fmt.Printf("✓ Removed docset '%s'\n", docsetID)
	return nil
}

func pushCommand(args []string) error {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	_ = fs.String("file", "", "File to push")
	_ = fs.String("dir", "", "Directory to push recursively")
	_ = fs.String("type", "document", "Content type: document, memory, cache, note")
	_ = fs.String("tags", "", "Comma-separated tags")
	_ = fs.String("namespace", "default", "Knowledge namespace")

	fs.Usage = func() {
		fmt.Print(`Push files or content to knowledge base

Usage:
  gismo-knowledge push [options]

Options:
`)
		fs.PrintDefaults()
		fmt.Print(`
Examples:
  # Push a single file
  gismo-knowledge push --file README.md --type documentation

  # Push directory of notes
  gismo-knowledge push --dir ./notes --type note --tags "project,design"

  # Push to specific namespace
  gismo-knowledge push --file memory.md --type memory --namespace "project-x"

Content types:
  - document: General documentation
  - memory: Persistent memories/context
  - cache: Cached web content or API responses
  - note: Personal notes or annotations
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Implementation would handle pushing files to knowledge base
	fmt.Println("Push functionality will be implemented to add files and content to knowledge base")
	return nil
}

func indexCommand(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	_ = fs.Bool("rebuild", false, "Rebuild all indexes")
	_ = fs.String("docset", "", "Index specific docset")
	_ = fs.String("type", "all", "Index type: text, semantic, all")

	fs.Usage = func() {
		fmt.Print(`Manage search indexes

Usage:
  gismo-knowledge index [options]

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Implementation would handle index management
	fmt.Println("Index management functionality will handle search index operations")
	return nil
}

func statsCommand(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	verbose := fs.Bool("v", false, "Verbose output")

	fs.Usage = func() {
		fmt.Print(`Show knowledge base statistics

Usage:
  gismo-knowledge stats [options]

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := knowledge.New()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// Get docsets for statistics
	docsets, err := client.ListDocsets(ctx)
	if err != nil {
		return fmt.Errorf("failed to get statistics: %w", err)
	}

	fmt.Println("Knowledge Base Statistics:")
	fmt.Println("=========================")
	fmt.Printf("Total docsets: %d\n", len(docsets))

	var totalEntries int32
	for _, ds := range docsets {
		totalEntries += ds.ContentCount
	}

	fmt.Printf("Total entries: %d\n", totalEntries)

	if *verbose && len(docsets) > 0 {
		fmt.Println("\nDocsets:")
		for _, ds := range docsets {
			fmt.Printf("  %s v%s: %d entries (imported %s)\n",
				ds.Name,
				ds.Version,
				ds.ContentCount,
				ds.ImportedAt.AsTime().Format("2006-01-02"),
			)
		}
	}

	return nil
}

func indent(text string, prefix string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		if lines[i] != "" {
			lines[i] = prefix + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}
