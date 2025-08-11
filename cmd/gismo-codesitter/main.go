package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jrossi/gismo/pkg/client"
	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

var (
	// Commands
	initCmd     = flag.NewFlagSet("init", flag.ExitOnError)
	queryCmd    = flag.NewFlagSet("query", flag.ExitOnError)
	findCmd     = flag.NewFlagSet("find", flag.ExitOnError)
	analyzeCmd  = flag.NewFlagSet("analyze", flag.ExitOnError)
	validateCmd = flag.NewFlagSet("validate", flag.ExitOnError)
	watchCmd    = flag.NewFlagSet("watch", flag.ExitOnError)

	// Common flags
	workspace = ""
	debug     = false
)

func init() {
	// Init command flags
	initCmd.StringVar(&workspace, "workspace", ".", "Workspace root directory")
	initCmd.BoolVar(&debug, "debug", false, "Enable debug output")

	// Query command flags
	queryCmd.StringVar(&workspace, "workspace", ".", "Workspace root (for context)")
	queryCmd.String("query", "", "Tree-sitter query pattern")
	queryCmd.String("language", "", "Target language (go, javascript, python)")
	queryCmd.String("files", "", "File patterns to search (e.g., '*.go')")

	// Find command flags
	findCmd.String("symbol", "", "Symbol name to find")
	findCmd.String("references", "", "Find references to this symbol")
	findCmd.String("definition", "", "Find definition of this symbol")

	// Analyze command flags
	analyzeCmd.String("security", "", "Run security analysis on files")
	analyzeCmd.String("metrics", "", "Calculate code metrics")
	analyzeCmd.String("patterns", "", "Detect code patterns")

	// Validate command flags
	validateCmd.String("file", "", "File to validate edits for")
	validateCmd.String("edit", "", "Edit to validate (old:new)")

	// Watch command flags
	watchCmd.String("files", "", "Watch file patterns")
	watchCmd.String("symbols", "", "Watch symbol changes")
	watchCmd.Bool("diagnostics", false, "Watch diagnostics")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Connect to gismo-server
	ctx := context.Background()
	conn, err := connectToServer(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to gismo-server: %v", err)
	}
	defer conn.Close()

	// Create CodeSitter client
	codeSitterClient := gismov1.NewCodeSitterClient(conn)

	// Parse command
	switch os.Args[1] {
	case "init":
		initCmd.Parse(os.Args[2:])
		runInit(ctx, codeSitterClient)

	case "query":
		queryCmd.Parse(os.Args[2:])
		runQuery(ctx, codeSitterClient)

	case "find":
		findCmd.Parse(os.Args[2:])
		runFind(ctx, codeSitterClient)

	case "analyze":
		analyzeCmd.Parse(os.Args[2:])
		runAnalyze(ctx, codeSitterClient)

	case "validate":
		validateCmd.Parse(os.Args[2:])
		runValidate(ctx, codeSitterClient)

	case "watch":
		watchCmd.Parse(os.Args[2:])
		runWatch(ctx, codeSitterClient)

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func connectToServer(ctx context.Context) (*grpc.ClientConn, error) {
	// Try to connect to Unix socket first
	socketPath := filepath.Join(os.TempDir(), "gismo.sock")
	if runtime := os.Getenv("XDG_RUNTIME_DIR"); runtime != "" {
		socketPath = filepath.Join(runtime, "gismo.sock")
	}

	// Check if socket exists
	if _, err := os.Stat(socketPath); err == nil {
		return grpc.DialContext(ctx, "unix://"+socketPath,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Fall back to TCP connection
	return grpc.DialContext(ctx, "localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func runInit(ctx context.Context, client gismov1.CodeSitterClient) {
	absPath, err := filepath.Abs(workspace)
	if err != nil {
		log.Fatalf("Failed to resolve workspace path: %v", err)
	}

	resp, err := client.InitializeWorkspace(ctx, &gismov1.InitializeWorkspaceRequest{
		WorkspaceRoot:            absPath,
		EnableFileWatching:       true,
		EnableIncrementalParsing: true,
	})
	if err != nil {
		log.Fatalf("Failed to initialize workspace: %v", err)
	}

	fmt.Printf("Workspace initialized successfully!\n")
	fmt.Printf("  Files parsed: %d\n", resp.FilesParsed)
	fmt.Printf("  Total symbols: %d\n", resp.TotalSymbols)
	fmt.Printf("  Session ID: %s\n", resp.SessionId)
	fmt.Printf("  Supported languages: %s\n", strings.Join(resp.SupportedLanguages, ", "))

	if len(resp.FileCountsByLanguage) > 0 {
		fmt.Printf("\nFiles by language:\n")
		for lang, count := range resp.FileCountsByLanguage {
			fmt.Printf("  %s: %d files\n", lang, count)
		}
	}
}

func runQuery(ctx context.Context, client gismov1.CodeSitterClient) {
	query := queryCmd.Lookup("query").Value.String()
	language := queryCmd.Lookup("language").Value.String()
	files := queryCmd.Lookup("files").Value.String()

	var filePatterns []string
	if files != "" {
		filePatterns = strings.Split(files, ",")
	}

	resp, err := client.QuerySymbols(ctx, &gismov1.QuerySymbolsRequest{
		Query:        query,
		Language:     language,
		FilePatterns: filePatterns,
		MaxResults:   100,
	})
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Printf("Found %d symbols (total: %d)\n", len(resp.Symbols), resp.TotalMatches)
	for _, sym := range resp.Symbols {
		fmt.Printf("  %s (%s) at %s:%d\n",
			sym.Name, sym.Kind,
			sym.Location.FilePath, sym.Location.StartLine)
	}
}

func runFind(ctx context.Context, client gismov1.CodeSitterClient) {
	// symbol := findCmd.Lookup("symbol").Value.String() // Reserved for future use
	references := findCmd.Lookup("references").Value.String()
	definition := findCmd.Lookup("definition").Value.String()

	if definition != "" {
		resp, err := client.GetSymbolDefinition(ctx, &gismov1.GetSymbolDefinitionRequest{
			SymbolName: definition,
		})
		if err != nil {
			log.Fatalf("Failed to find definition: %v", err)
		}

		if !resp.Found {
			fmt.Printf("Symbol '%s' not found\n", definition)
			return
		}

		fmt.Printf("Definition of '%s':\n", definition)
		fmt.Printf("  Type: %s\n", resp.Definition.Kind)
		fmt.Printf("  Location: %s:%d\n",
			resp.Definition.Location.FilePath,
			resp.Definition.Location.StartLine)
		if resp.Definition.Signature != "" {
			fmt.Printf("  Signature: %s\n", resp.Definition.Signature)
		}
	}

	if references != "" {
		resp, err := client.FindReferences(ctx, &gismov1.FindReferencesRequest{
			SymbolName: references,
			MaxResults: 100,
		})
		if err != nil {
			log.Fatalf("Failed to find references: %v", err)
		}

		fmt.Printf("Found %d references to '%s':\n", resp.TotalReferences, references)
		for _, ref := range resp.References {
			fmt.Printf("  %s:%d - %s (%s)\n",
				ref.Location.FilePath,
				ref.Location.StartLine,
				ref.Text,
				ref.Kind)
		}
	}
}

func runAnalyze(ctx context.Context, client gismov1.CodeSitterClient) {
	security := analyzeCmd.Lookup("security").Value.String()
	metrics := analyzeCmd.Lookup("metrics").Value.String()

	if security != "" {
		var filePatterns []string
		if security != "all" {
			filePatterns = strings.Split(security, ",")
		}

		resp, err := client.AnalyzeSecurity(ctx, &gismov1.AnalyzeSecurityRequest{
			FilePatterns: filePatterns,
		})
		if err != nil {
			log.Fatalf("Security analysis failed: %v", err)
		}

		fmt.Printf("Security Analysis Results:\n")
		fmt.Printf("  Files analyzed: %d\n", resp.FilesAnalyzed)
		fmt.Printf("  Total issues: %d\n", len(resp.Issues))

		if len(resp.IssuesBySeverity) > 0 {
			fmt.Printf("\nIssues by severity:\n")
			for sev, count := range resp.IssuesBySeverity {
				fmt.Printf("  %s: %d\n", sev, count)
			}
		}

		if len(resp.Issues) > 0 {
			fmt.Printf("\nIssues found:\n")
			for _, issue := range resp.Issues {
				fmt.Printf("  [%s] %s\n", issue.Severity, issue.RuleName)
				fmt.Printf("    %s:%d - %s\n",
					issue.Location.FilePath,
					issue.Location.StartLine,
					issue.Message)
				if len(issue.FixSuggestions) > 0 {
					fmt.Printf("    Suggestions:\n")
					for _, fix := range issue.FixSuggestions {
						fmt.Printf("      - %s\n", fix)
					}
				}
			}
		}
	}

	if metrics != "" {
		var filePatterns []string
		if metrics != "all" {
			filePatterns = strings.Split(metrics, ",")
		}

		resp, err := client.GetCodeMetrics(ctx, &gismov1.GetCodeMetricsRequest{
			FilePatterns: filePatterns,
			MetricTypes:  []string{"loc", "functions", "classes", "complexity"},
		})
		if err != nil {
			log.Fatalf("Metrics calculation failed: %v", err)
		}

		fmt.Printf("Code Metrics:\n")
		if resp.Aggregate != nil {
			fmt.Printf("\nAggregate metrics:\n")
			for metric, value := range resp.Aggregate.Totals {
				fmt.Printf("  Total %s: %.0f\n", metric, value)
			}
			for metric, value := range resp.Aggregate.Averages {
				fmt.Printf("  Average %s: %.2f\n", metric, value)
			}
		}

		if len(resp.FileMetrics) > 0 && debug {
			fmt.Printf("\nPer-file metrics:\n")
			for _, fm := range resp.FileMetrics {
				fmt.Printf("  %s:\n", fm.FilePath)
				for metric, value := range fm.Metrics {
					fmt.Printf("    %s: %.0f\n", metric, value)
				}
			}
		}
	}
}

func runValidate(ctx context.Context, client gismov1.CodeSitterClient) {
	file := validateCmd.Lookup("file").Value.String()
	edit := validateCmd.Lookup("edit").Value.String()

	if file == "" || edit == "" {
		log.Fatal("Both -file and -edit flags are required")
	}

	// Parse edit (format: "old:new")
	parts := strings.SplitN(edit, ":", 2)
	if len(parts) != 2 {
		log.Fatal("Edit must be in format 'old:new'")
	}

	resp, err := client.ValidateEdit(ctx, &gismov1.ValidateEditRequest{
		FilePath: file,
		Edits: []*gismov1.Edit{{
			OldText: parts[0],
			NewText: parts[1],
		}},
		CheckSyntax:     true,
		CheckReferences: true,
		CheckSecurity:   true,
	})
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	if resp.IsValid {
		fmt.Println("✅ Edit is valid")
	} else {
		fmt.Println("❌ Edit validation failed")
		if resp.WouldBreakSyntax {
			fmt.Println("  - Would break syntax")
		}
		for _, issue := range resp.Issues {
			fmt.Printf("  - [%s] %s: %s\n", issue.Severity, issue.IssueType, issue.Message)
		}
	}

	if len(resp.AffectedSymbols) > 0 {
		fmt.Printf("\nAffected symbols:\n")
		for _, sym := range resp.AffectedSymbols {
			fmt.Printf("  - %s\n", sym)
		}
	}
}

func runWatch(ctx context.Context, client gismov1.CodeSitterClient) {
	files := watchCmd.Lookup("files").Value.String()
	symbols := watchCmd.Lookup("symbols").Value.String()
	diagnosticsFlag := watchCmd.Lookup("diagnostics")
	diagnostics := false
	if diagnosticsFlag != nil {
		diagnostics = diagnosticsFlag.Value.String() == "true"
	}

	if files != "" {
		fmt.Printf("Watching files matching: %s\n", files)
		stream, err := client.WatchFiles(ctx, &gismov1.WatchFilesRequest{
			FilePatterns: strings.Split(files, ","),
		})
		if err != nil {
			log.Fatalf("Failed to watch files: %v", err)
		}

		for {
			event, err := stream.Recv()
			if err != nil {
				log.Fatalf("Watch error: %v", err)
			}
			fmt.Printf("[%s] %s: %s\n",
				event.Timestamp.AsTime().Format("15:04:05"),
				event.Kind, event.FilePath)
		}
	}

	if symbols != "" {
		fmt.Printf("Watching symbols in: %s\n", symbols)
		stream, err := client.WatchSymbols(ctx, &gismov1.WatchSymbolsRequest{
			FilePatterns: strings.Split(symbols, ","),
		})
		if err != nil {
			log.Fatalf("Failed to watch symbols: %v", err)
		}

		for {
			event, err := stream.Recv()
			if err != nil {
				log.Fatalf("Watch error: %v", err)
			}
			fmt.Printf("[%s] %s: %s (%s)\n",
				event.Timestamp.AsTime().Format("15:04:05"),
				event.Kind, event.Symbol.Name, event.Symbol.Kind)
		}
	}

	if diagnostics {
		fmt.Println("Watching diagnostics...")
		stream, err := client.WatchDiagnostics(ctx, &gismov1.WatchDiagnosticsRequest{})
		if err != nil {
			log.Fatalf("Failed to watch diagnostics: %v", err)
		}

		for {
			event, err := stream.Recv()
			if err != nil {
				log.Fatalf("Watch error: %v", err)
			}
			fmt.Printf("[%s] %s:\n",
				event.Timestamp.AsTime().Format("15:04:05"),
				event.FilePath)
			for _, diag := range event.Added {
				fmt.Printf("  + [%s] %s\n", diag.Severity, diag.Message)
			}
			for _, diag := range event.Removed {
				fmt.Printf("  - [%s] %s\n", diag.Severity, diag.Message)
			}
		}
	}
}

func printUsage() {
	fmt.Println("gismo-codesitter - Code analysis client for gismo-server")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gismo-codesitter <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init        Initialize workspace for analysis")
	fmt.Println("  query       Query symbols using tree-sitter syntax")
	fmt.Println("  find        Find symbol definitions and references")
	fmt.Println("  analyze     Analyze code for security and metrics")
	fmt.Println("  validate    Validate code edits")
	fmt.Println("  watch       Watch for real-time changes")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  gismo-codesitter init -workspace /path/to/code")
	fmt.Println("  gismo-codesitter query -query '(function_declaration) @func' -language go")
	fmt.Println("  gismo-codesitter find -definition MyFunction")
	fmt.Println("  gismo-codesitter analyze -security all")
	fmt.Println("  gismo-codesitter watch -files '*.go'")
}
