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

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
	"github.com/jrossi/gismo/pkg/socket"
)

var (
	// Commands
	initCmd     = flag.NewFlagSet("init", flag.ExitOnError)
	queryCmd    = flag.NewFlagSet("query", flag.ExitOnError)
	findCmd     = flag.NewFlagSet("find", flag.ExitOnError)
	analyzeCmd  = flag.NewFlagSet("analyze", flag.ExitOnError)
	validateCmd = flag.NewFlagSet("validate", flag.ExitOnError)
	watchCmd    = flag.NewFlagSet("watch", flag.ExitOnError)
	searchCmd   = flag.NewFlagSet("search", flag.ExitOnError)
	overviewCmd = flag.NewFlagSet("overview", flag.ExitOnError)
	symbolCmd   = flag.NewFlagSet("symbol", flag.ExitOnError)
	refsCmd     = flag.NewFlagSet("refs", flag.ExitOnError)

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

	// Search command flags (search_for_pattern)
	searchCmd.String("pattern", "", "Pattern to search for")
	searchCmd.String("files", "", "File patterns to search in")
	searchCmd.Bool("regex", false, "Use regex pattern")
	searchCmd.Bool("case", false, "Case sensitive search")
	searchCmd.Int("before", 0, "Lines of context before")
	searchCmd.Int("after", 0, "Lines of context after")

	// Overview command flags (get_symbols_overview)
	overviewCmd.String("file", "", "File to get symbols overview for")
	overviewCmd.Int("depth", 2, "Max depth of symbol tree")

	// Symbol command flags (find_symbol)
	symbolCmd.String("name", "", "Symbol name pattern (e.g., 'MyClass/myMethod')")
	symbolCmd.String("file", "", "Limit to specific file")
	symbolCmd.Bool("substring", false, "Use substring matching")
	symbolCmd.Int("max", 50, "Maximum results")

	// Refs command flags (find_referencing_symbols)
	refsCmd.String("symbol", "", "Symbol name to find references for")
	refsCmd.String("file", "", "File containing the symbol")
	refsCmd.String("patterns", "", "File patterns to search in")
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
		if err := initCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing init flags: %v", err)
		}
		runInit(ctx, codeSitterClient)

	case "query":
		if err := queryCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing query flags: %v", err)
		}
		runQuery(ctx, codeSitterClient)

	case "find":
		if err := findCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing find flags: %v", err)
		}
		runFind(ctx, codeSitterClient)

	case "analyze":
		if err := analyzeCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing analyze flags: %v", err)
		}
		runAnalyze(ctx, codeSitterClient)

	case "validate":
		if err := validateCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing validate flags: %v", err)
		}
		runValidate(ctx, codeSitterClient)

	case "watch":
		if err := watchCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing watch flags: %v", err)
		}
		runWatch(ctx, codeSitterClient)

	case "search":
		if err := searchCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing search flags: %v", err)
		}
		runSearch(ctx, codeSitterClient)

	case "overview":
		if err := overviewCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing overview flags: %v", err)
		}
		runOverview(ctx, codeSitterClient)

	case "symbol":
		if err := symbolCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing symbol flags: %v", err)
		}
		runSymbol(ctx, codeSitterClient)

	case "refs":
		if err := refsCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("Error parsing refs flags: %v", err)
		}
		runRefs(ctx, codeSitterClient)

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func connectToServer(ctx context.Context) (*grpc.ClientConn, error) {
	// Use common socket library to connect
	return socket.ConnectWithFallback(ctx, "localhost:50051")
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
	fmt.Println("  search      Search for patterns in code (AST-aware grep)")
	fmt.Println("  overview    Get hierarchical overview of symbols in a file")
	fmt.Println("  symbol      Find symbols by name pattern")
	fmt.Println("  refs        Find all references to a symbol")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  gismo-codesitter init -workspace /path/to/code")
	fmt.Println("  gismo-codesitter query -query '(function_declaration) @func' -language go")
	fmt.Println("  gismo-codesitter find -definition MyFunction")
	fmt.Println("  gismo-codesitter analyze -security all")
	fmt.Println("  gismo-codesitter watch -files '*.go'")
}

func runSearch(ctx context.Context, client gismov1.CodeSitterClient) {
	pattern := searchCmd.Lookup("pattern").Value.String()
	files := searchCmd.Lookup("files").Value.String()

	// Use the values that were already parsed
	useRegexFlag := searchCmd.Lookup("regex")
	useRegex := useRegexFlag != nil && useRegexFlag.Value.String() == "true"

	caseSensitiveFlag := searchCmd.Lookup("case")
	caseSensitive := caseSensitiveFlag != nil && caseSensitiveFlag.Value.String() == "true"

	beforeFlag := searchCmd.Lookup("before")
	before := 0
	if beforeFlag != nil {
		if _, err := fmt.Sscanf(beforeFlag.Value.String(), "%d", &before); err != nil {
			log.Printf("Warning: invalid 'before' value: %v", err)
		}
	}

	afterFlag := searchCmd.Lookup("after")
	after := 0
	if afterFlag != nil {
		if _, err := fmt.Sscanf(afterFlag.Value.String(), "%d", &after); err != nil {
			log.Printf("Warning: invalid 'after' value: %v", err)
		}
	}

	if pattern == "" {
		log.Fatal("Pattern is required")
	}

	var filePatterns []string
	if files != "" {
		filePatterns = strings.Split(files, ",")
	}

	resp, err := client.SearchForPattern(ctx, &gismov1.SearchForPatternRequest{
		Pattern:            pattern,
		FilePatterns:       filePatterns,
		UseRegex:           useRegex,
		CaseSensitive:      caseSensitive,
		ContextLinesBefore: int32(before),
		ContextLinesAfter:  int32(after),
	})
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	fmt.Printf("Found %d matches in %d files:\n", resp.TotalMatches, resp.FilesSearched)
	for _, match := range resp.Matches {
		// Print context before
		for _, line := range match.ContextBefore {
			fmt.Printf("  %s\n", line)
		}
		// Print matching line with highlight
		fmt.Printf("→ %s:%d: %s\n", match.FilePath, match.LineNumber, match.LineText)
		// Print context after
		for _, line := range match.ContextAfter {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println()
	}
}

func runOverview(ctx context.Context, client gismov1.CodeSitterClient) {
	file := overviewCmd.Lookup("file").Value.String()

	depthFlag := overviewCmd.Lookup("depth")
	depth := 2
	if depthFlag != nil {
		if _, err := fmt.Sscanf(depthFlag.Value.String(), "%d", &depth); err != nil {
			log.Printf("Warning: invalid 'depth' value: %v", err)
		}
	}

	if file == "" {
		log.Fatal("File path is required")
	}

	resp, err := client.GetSymbolsOverview(ctx, &gismov1.GetSymbolsOverviewRequest{
		FilePath: file,
		MaxDepth: int32(depth),
	})
	if err != nil {
		log.Fatalf("Overview failed: %v", err)
	}

	fmt.Printf("File: %s\n", file)
	fmt.Printf("Total symbols: %d\n\n", resp.TotalSymbols)

	printSymbolTree(resp.Symbols, 0)
}

func printSymbolTree(trees []*gismov1.GetSymbolsOverviewResponse_SymbolTree, indent int) {
	for _, tree := range trees {
		fmt.Printf("%s%s (%s) at line %d\n",
			strings.Repeat("  ", indent),
			tree.Symbol.Name,
			tree.Symbol.Kind,
			tree.Symbol.Location.StartLine)
		if len(tree.Children) > 0 {
			printSymbolTree(tree.Children, indent+1)
		}
	}
}

func runSymbol(ctx context.Context, client gismov1.CodeSitterClient) {
	name := symbolCmd.Lookup("name").Value.String()
	file := symbolCmd.Lookup("file").Value.String()

	substringFlag := symbolCmd.Lookup("substring")
	substring := substringFlag != nil && substringFlag.Value.String() == "true"

	maxFlag := symbolCmd.Lookup("max")
	maxResults := 50
	if maxFlag != nil {
		if _, err := fmt.Sscanf(maxFlag.Value.String(), "%d", &maxResults); err != nil {
			log.Printf("Warning: invalid 'max' value: %v", err)
		}
	}

	if name == "" {
		log.Fatal("Symbol name pattern is required")
	}

	resp, err := client.FindSymbol(ctx, &gismov1.FindSymbolRequest{
		NamePattern:       name,
		FilePath:          file,
		SubstringMatching: substring,
		MaxResults:        int32(maxResults),
	})
	if err != nil {
		log.Fatalf("Find symbol failed: %v", err)
	}

	fmt.Printf("Found %d symbols:\n", resp.TotalFound)
	for _, symbol := range resp.Symbols {
		fmt.Printf("  %s (%s)\n", symbol.Name, symbol.Kind)
		fmt.Printf("    Location: %s:%d-%d\n",
			symbol.Location.FilePath,
			symbol.Location.StartLine,
			symbol.Location.EndLine)
		if symbol.Signature != "" {
			fmt.Printf("    Signature: %s\n", symbol.Signature)
		}
		if symbol.ParentSymbol != "" {
			fmt.Printf("    Parent: %s\n", symbol.ParentSymbol)
		}
	}
}

func runRefs(ctx context.Context, client gismov1.CodeSitterClient) {
	symbol := refsCmd.Lookup("symbol").Value.String()
	file := refsCmd.Lookup("file").Value.String()
	patterns := refsCmd.Lookup("patterns").Value.String()

	if symbol == "" {
		log.Fatal("Symbol name is required")
	}

	var filePatterns []string
	if patterns != "" {
		filePatterns = strings.Split(patterns, ",")
	}

	var location *gismov1.Location
	if file != "" {
		// Create a basic location with just the file path
		location = &gismov1.Location{
			FilePath: file,
		}
	}

	resp, err := client.FindReferencingSymbols(ctx, &gismov1.FindReferencingSymbolsRequest{
		SymbolName:     symbol,
		SymbolLocation: location,
		FilePatterns:   filePatterns,
	})
	if err != nil {
		log.Fatalf("Find references failed: %v", err)
	}

	fmt.Printf("Found %d references to '%s':\n", resp.TotalReferences, symbol)
	for _, ref := range resp.References {
		fmt.Printf("  %s:%d\n",
			ref.ReferenceLocation.FilePath,
			ref.ReferenceLocation.StartLine)
		if ref.ContainingSymbol != nil {
			fmt.Printf("    In: %s (%s)\n",
				ref.ContainingSymbol.Name,
				ref.ContainingSymbol.Kind)
		}
		fmt.Printf("    Text: %s\n", ref.ReferenceText)
		fmt.Printf("    Kind: %s\n", ref.Kind)
		fmt.Println()
	}
}
