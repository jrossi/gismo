package codesitter

import (
	"context"
	"fmt"
	"strings"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
	sitter "github.com/smacker/go-tree-sitter"
)

// QueryEngine provides tree-sitter query capabilities
type QueryEngine struct {
	server *Server
}

// NewQueryEngine creates a new query engine
func NewQueryEngine(server *Server) *QueryEngine {
	return &QueryEngine{
		server: server,
	}
}

// QuerySymbols executes a tree-sitter query to find symbols
func (qe *QueryEngine) QuerySymbols(ctx context.Context, req *gismov1.QuerySymbolsRequest) (*gismov1.QuerySymbolsResponse, error) {
	qe.server.mu.RLock()
	defer qe.server.mu.RUnlock()

	results := make([]*gismov1.Symbol, 0)
	totalMatches := 0

	// Filter files by pattern
	for path, tree := range qe.server.trees {
		if !qe.server.matchesPatterns(path, req.FilePatterns) {
			continue
		}

		// Skip if language doesn't match
		if req.Language != "" && tree.Language != req.Language {
			continue
		}

		// Execute query on this tree
		matches, err := qe.executeQuery(tree, req.Query)
		if err != nil {
			// Skip files with query errors
			continue
		}

		// Filter by symbol kind if specified
		for _, match := range matches {
			if len(req.Kinds) > 0 && !containsKind(req.Kinds, match.Kind) {
				continue
			}

			totalMatches++
			if len(results) < int(req.MaxResults) || req.MaxResults == 0 {
				results = append(results, match)
			}
		}
	}

	return &gismov1.QuerySymbolsResponse{
		Symbols:      results,
		TotalMatches: int32(totalMatches),
		Truncated:    totalMatches > len(results),
	}, nil
}

// executeQuery runs a tree-sitter query on a file tree
func (qe *QueryEngine) executeQuery(ft *FileTree, queryStr string) ([]*gismov1.Symbol, error) {
	if queryStr == "" {
		// If no query provided, return all symbols
		return ft.Symbols, nil
	}

	// Get language for query
	lang := qe.getLanguage(ft.Language)
	if lang == nil {
		return nil, fmt.Errorf("no language support for %s", ft.Language)
	}

	// Parse query
	query, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}
	defer query.Close()

	// Create query cursor
	qc := sitter.NewQueryCursor()
	defer qc.Close()

	// Execute query
	qc.Exec(query, ft.Tree.RootNode())

	// Collect matches
	symbols := make([]*gismov1.Symbol, 0)
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		for _, capture := range m.Captures {
			node := capture.Node

			// Create symbol from captured node
			symbol := &gismov1.Symbol{
				Name:     qe.server.getNodeText(node, ft.Content),
				Kind:     qe.nodeTypeToSymbolKind(node.Type()),
				Location: qe.server.nodeToLocation(node, ft.Path),
			}

			// Try to get more context
			if parent := node.Parent(); parent != nil {
				if parent.Type() == "function_declaration" || parent.Type() == "method_declaration" {
					symbol.Signature = qe.server.getFunctionSignature(parent, ft.Content)
				}
			}

			symbols = append(symbols, symbol)
		}
	}

	return symbols, nil
}

// FindReferences finds all references to a symbol
func (qe *QueryEngine) FindReferences(ctx context.Context, req *gismov1.FindReferencesRequest) (*gismov1.FindReferencesResponse, error) {
	qe.server.mu.RLock()
	defer qe.server.mu.RUnlock()

	references := make([]*gismov1.Reference, 0)
	totalRefs := 0

	// Build language-specific query for finding references
	query := qe.buildReferenceQuery(req.SymbolName)

	for path, tree := range qe.server.trees {
		if !qe.server.matchesPatterns(path, req.FilePatterns) {
			continue
		}

		// Find references in this file
		refs, err := qe.findReferencesInFile(tree, req.SymbolName, query)
		if err != nil {
			continue
		}

		for _, ref := range refs {
			totalRefs++
			if len(references) < int(req.MaxResults) || req.MaxResults == 0 {
				references = append(references, ref)
			}
		}
	}

	return &gismov1.FindReferencesResponse{
		References:      references,
		TotalReferences: int32(totalRefs),
		Truncated:       totalRefs > len(references),
	}, nil
}

// findReferencesInFile finds references in a single file
func (qe *QueryEngine) findReferencesInFile(ft *FileTree, symbolName string, queryStr string) ([]*gismov1.Reference, error) {
	lang := qe.getLanguage(ft.Language)
	if lang == nil {
		return nil, fmt.Errorf("no language support for %s", ft.Language)
	}

	query, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(query, ft.Tree.RootNode())

	references := make([]*gismov1.Reference, 0)
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		for _, capture := range m.Captures {
			node := capture.Node
			text := qe.server.getNodeText(node, ft.Content)

			// Only include if it matches the symbol name
			if text != symbolName {
				continue
			}

			ref := &gismov1.Reference{
				Location: qe.server.nodeToLocation(node, ft.Path),
				Text:     text,
				Kind:     qe.determineReferenceKind(node),
			}

			// Find containing symbol
			if container := qe.findContainingSymbol(node, ft); container != nil {
				ref.ContainingSymbol = container.Name
			}

			references = append(references, ref)
		}
	}

	return references, nil
}

// DetectPatterns searches for specific code patterns
func (qe *QueryEngine) DetectPatterns(ctx context.Context, req *gismov1.DetectPatternsRequest) (*gismov1.DetectPatternsResponse, error) {
	qe.server.mu.RLock()
	defer qe.server.mu.RUnlock()

	allMatches := make([]*gismov1.PatternMatch, 0)
	matchesByPattern := make(map[string]int32)

	for _, pattern := range req.Patterns {
		patternMatches := 0

		for path, tree := range qe.server.trees {
			if !qe.server.matchesPatterns(path, req.FilePatterns) {
				continue
			}

			// Skip if language doesn't match
			if pattern.Language != "" && tree.Language != pattern.Language {
				continue
			}

			// Execute pattern query
			matches, err := qe.executePatternQuery(tree, pattern)
			if err != nil {
				continue
			}

			for _, match := range matches {
				patternMatches++
				if patternMatches <= int(req.MaxResultsPerPattern) || req.MaxResultsPerPattern == 0 {
					allMatches = append(allMatches, match)
				}
			}
		}

		matchesByPattern[pattern.Id] = int32(patternMatches)
	}

	return &gismov1.DetectPatternsResponse{
		Matches:          allMatches,
		MatchesByPattern: matchesByPattern,
	}, nil
}

// executePatternQuery executes a pattern detection query
func (qe *QueryEngine) executePatternQuery(ft *FileTree, pattern *gismov1.PatternQuery) ([]*gismov1.PatternMatch, error) {
	lang := qe.getLanguage(ft.Language)
	if lang == nil {
		return nil, fmt.Errorf("no language support for %s", ft.Language)
	}

	query, err := sitter.NewQuery([]byte(pattern.Query), lang)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(query, ft.Tree.RootNode())

	matches := make([]*gismov1.PatternMatch, 0)
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		// Get captured nodes
		capturedNodes := make(map[string]string)
		var primaryNode *sitter.Node

		for _, capture := range m.Captures {
			captureName := query.CaptureNameForId(capture.Index)
			capturedNodes[captureName] = qe.server.getNodeText(capture.Node, ft.Content)

			if primaryNode == nil {
				primaryNode = capture.Node
			}
		}

		if primaryNode != nil {
			match := &gismov1.PatternMatch{
				PatternId:     pattern.Id,
				PatternName:   pattern.Name,
				Location:      qe.server.nodeToLocation(primaryNode, ft.Path),
				MatchedText:   qe.server.getNodeText(primaryNode, ft.Content),
				CapturedNodes: capturedNodes,
			}
			matches = append(matches, match)
		}
	}

	return matches, nil
}

// Security analysis queries

// AnalyzeSecurity performs security analysis on code
func (qe *QueryEngine) AnalyzeSecurity(ctx context.Context, req *gismov1.AnalyzeSecurityRequest) (*gismov1.AnalyzeSecurityResponse, error) {
	qe.server.mu.RLock()
	defer qe.server.mu.RUnlock()

	issues := make([]*gismov1.SecurityIssue, 0)
	filesAnalyzed := int32(0)
	issuesBySeverity := make(map[string]int32)

	// Default security rules if none provided
	rules := req.Rules
	if len(rules) == 0 {
		rules = qe.getDefaultSecurityRules()
	}

	for path, tree := range qe.server.trees {
		if !qe.server.matchesPatterns(path, req.FilePatterns) {
			continue
		}

		filesAnalyzed++

		// Run security rules on this file
		fileIssues := qe.analyzeFileSecuritly(tree, rules, req.IncludeInfoLevel)

		for _, issue := range fileIssues {
			issues = append(issues, issue)
			severityKey := issue.Severity.String()
			issuesBySeverity[severityKey]++
		}
	}

	return &gismov1.AnalyzeSecurityResponse{
		Issues:           issues,
		FilesAnalyzed:    filesAnalyzed,
		IssuesBySeverity: issuesBySeverity,
	}, nil
}

// analyzeFileSecuritly runs security rules on a file
func (qe *QueryEngine) analyzeFileSecuritly(ft *FileTree, rules []*gismov1.SecurityRule, includeInfo bool) []*gismov1.SecurityIssue {
	issues := make([]*gismov1.SecurityIssue, 0)

	for _, rule := range rules {
		// Skip info level if not requested
		if !includeInfo && rule.Severity == gismov1.Diagnostic_SEVERITY_INFO {
			continue
		}

		// Get language-specific rule pattern
		pattern := qe.getLanguagePattern(rule.Pattern, ft.Language)
		if pattern == "" {
			continue
		}

		// Execute security pattern
		matches, err := qe.executeQuery(ft, pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			issue := &gismov1.SecurityIssue{
				RuleId:      rule.Id,
				RuleName:    rule.Name,
				Location:    match.Location,
				Message:     qe.formatSecurityMessage(rule.Message, match),
				Severity:    rule.Severity,
				CodeSnippet: qe.getCodeSnippet(ft, match.Location),
			}

			// Add fix suggestions based on rule
			issue.FixSuggestions = qe.getFixSuggestions(rule.Id, ft.Language)

			issues = append(issues, issue)
		}
	}

	return issues
}

// getDefaultSecurityRules returns built-in security rules
func (qe *QueryEngine) getDefaultSecurityRules() []*gismov1.SecurityRule {
	return []*gismov1.SecurityRule{
		// SQL Injection
		{
			Id:       "sql-injection",
			Name:     "SQL Injection Risk",
			Pattern:  qe.getSQLInjectionPattern(),
			Severity: gismov1.Diagnostic_SEVERITY_ERROR,
			Message:  "Potential SQL injection vulnerability: user input concatenated with SQL query",
		},
		// Command Injection
		{
			Id:       "command-injection",
			Name:     "Command Injection Risk",
			Pattern:  qe.getCommandInjectionPattern(),
			Severity: gismov1.Diagnostic_SEVERITY_ERROR,
			Message:  "Potential command injection: user input passed to system command",
		},
		// Path Traversal
		{
			Id:       "path-traversal",
			Name:     "Path Traversal Risk",
			Pattern:  qe.getPathTraversalPattern(),
			Severity: gismov1.Diagnostic_SEVERITY_WARNING,
			Message:  "Potential path traversal: user input used in file path",
		},
		// Hardcoded Secrets
		{
			Id:       "hardcoded-secrets",
			Name:     "Hardcoded Secrets",
			Pattern:  qe.getHardcodedSecretsPattern(),
			Severity: gismov1.Diagnostic_SEVERITY_ERROR,
			Message:  "Hardcoded secret or API key detected",
		},
		// Weak Crypto
		{
			Id:       "weak-crypto",
			Name:     "Weak Cryptography",
			Pattern:  qe.getWeakCryptoPattern(),
			Severity: gismov1.Diagnostic_SEVERITY_WARNING,
			Message:  "Weak or deprecated cryptographic algorithm",
		},
	}
}

// Pattern builders for security rules

func (qe *QueryEngine) getSQLInjectionPattern() string {
	return `
	(call_expression
		function: (selector_expression
			field: (field_identifier) @method (#match? @method "Query|Exec|Prepare"))
		arguments: (argument_list
			[(binary_expression
				operator: "+"
				right: (identifier) @user_input)
			 (template_string) @template]))
	`
}

func (qe *QueryEngine) getCommandInjectionPattern() string {
	return `
	(call_expression
		function: (selector_expression
			operand: (identifier) @pkg (#match? @pkg "exec|os")
			field: (field_identifier) @method (#match? @method "Command|Run|System"))
		arguments: (argument_list
			(identifier) @user_input))
	`
}

func (qe *QueryEngine) getPathTraversalPattern() string {
	return `
	(call_expression
		function: (selector_expression
			field: (field_identifier) @method (#match? @method "Open|ReadFile|WriteFile"))
		arguments: (argument_list
			(binary_expression
				left: (identifier) @base_path
				operator: "+"
				right: (identifier) @user_input)))
	`
}

func (qe *QueryEngine) getHardcodedSecretsPattern() string {
	return `
	[(assignment
		left: (identifier) @var (#match? @var "(?i)(password|secret|key|token|api)")
		right: (interpreted_string_literal) @value)
	 (short_var_declaration
		left: (identifier_list
			(identifier) @var (#match? @var "(?i)(password|secret|key|token|api)"))
		right: (expression_list
			(interpreted_string_literal) @value))]
	`
}

func (qe *QueryEngine) getWeakCryptoPattern() string {
	return `
	(call_expression
		function: (selector_expression
			field: (field_identifier) @method (#match? @method "(?i)(md5|sha1|des|rc4)"))
		arguments: (argument_list))
	`
}

// Helper functions

func (qe *QueryEngine) getLanguage(langName string) *sitter.Language {
	parser, ok := qe.server.parsers[langName]
	if !ok {
		return nil
	}
	return parser.Language()
}

func (qe *QueryEngine) nodeTypeToSymbolKind(nodeType string) gismov1.SymbolKind {
	switch nodeType {
	case "function_declaration", "function_definition", "arrow_function":
		return gismov1.SymbolKind_SYMBOL_KIND_FUNCTION
	case "method_declaration", "method_definition":
		return gismov1.SymbolKind_SYMBOL_KIND_METHOD
	case "class_declaration", "class_definition":
		return gismov1.SymbolKind_SYMBOL_KIND_CLASS
	case "interface_declaration":
		return gismov1.SymbolKind_SYMBOL_KIND_INTERFACE
	case "struct_type":
		return gismov1.SymbolKind_SYMBOL_KIND_STRUCT
	case "variable_declaration", "var_declaration":
		return gismov1.SymbolKind_SYMBOL_KIND_VARIABLE
	case "const_declaration":
		return gismov1.SymbolKind_SYMBOL_KIND_CONSTANT
	case "field_declaration":
		return gismov1.SymbolKind_SYMBOL_KIND_FIELD
	case "property_definition":
		return gismov1.SymbolKind_SYMBOL_KIND_PROPERTY
	default:
		return gismov1.SymbolKind_SYMBOL_KIND_UNSPECIFIED
	}
}

func (qe *QueryEngine) buildReferenceQuery(symbolName string) string {
	// Generic identifier query that works across languages
	return fmt.Sprintf(`(identifier) @ref (#eq? @ref "%s")`, symbolName)
}

func (qe *QueryEngine) determineReferenceKind(node *sitter.Node) gismov1.ReferenceKind {
	parent := node.Parent()
	if parent == nil {
		return gismov1.ReferenceKind_REFERENCE_KIND_READ
	}

	switch parent.Type() {
	case "call_expression":
		return gismov1.ReferenceKind_REFERENCE_KIND_CALL
	case "assignment", "short_var_declaration":
		// Check if it's on the left side (write) or right side (read)
		if parent.ChildByFieldName("left") == node {
			return gismov1.ReferenceKind_REFERENCE_KIND_WRITE
		}
		return gismov1.ReferenceKind_REFERENCE_KIND_READ
	case "import_statement", "import_declaration":
		return gismov1.ReferenceKind_REFERENCE_KIND_IMPORT
	case "type_identifier":
		return gismov1.ReferenceKind_REFERENCE_KIND_TYPE
	default:
		return gismov1.ReferenceKind_REFERENCE_KIND_READ
	}
}

func (qe *QueryEngine) findContainingSymbol(node *sitter.Node, ft *FileTree) *gismov1.Symbol {
	// Walk up the tree to find containing function/method/class
	current := node.Parent()
	for current != nil {
		switch current.Type() {
		case "function_declaration", "method_declaration", "class_declaration":
			// Find this in our symbols
			for _, sym := range ft.Symbols {
				if sym.Location.StartByte <= int32(node.StartByte()) &&
					sym.Location.EndByte >= int32(node.EndByte()) {
					return sym
				}
			}
		}
		current = current.Parent()
	}
	return nil
}

func (qe *QueryEngine) getLanguagePattern(pattern, language string) string {
	// In a real implementation, this would translate generic patterns
	// to language-specific ones
	return pattern
}

func (qe *QueryEngine) formatSecurityMessage(template string, symbol *gismov1.Symbol) string {
	// Simple template replacement
	result := strings.ReplaceAll(template, "{name}", symbol.Name)
	result = strings.ReplaceAll(result, "{kind}", symbol.Kind.String())
	return result
}

func (qe *QueryEngine) getCodeSnippet(ft *FileTree, location *gismov1.Location) string {
	if location == nil {
		return ""
	}

	lines := strings.Split(string(ft.Content), "\n")
	startLine := int(location.StartLine) - 1
	endLine := int(location.EndLine) - 1

	if startLine < 0 || startLine >= len(lines) {
		return ""
	}
	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	snippet := strings.Join(lines[startLine:endLine+1], "\n")
	if len(snippet) > 200 {
		snippet = snippet[:200] + "..."
	}
	return snippet
}

func (qe *QueryEngine) getFixSuggestions(ruleID, language string) []string {
	suggestions := make([]string, 0)

	switch ruleID {
	case "sql-injection":
		suggestions = append(suggestions, "Use parameterized queries or prepared statements")
		suggestions = append(suggestions, "Validate and sanitize user input")
	case "command-injection":
		suggestions = append(suggestions, "Use command builders that escape arguments")
		suggestions = append(suggestions, "Avoid passing user input directly to system commands")
	case "path-traversal":
		suggestions = append(suggestions, "Use filepath.Clean() to sanitize paths")
		suggestions = append(suggestions, "Validate paths don't contain '..' sequences")
	case "hardcoded-secrets":
		suggestions = append(suggestions, "Use environment variables for secrets")
		suggestions = append(suggestions, "Use a secrets management service")
	case "weak-crypto":
		suggestions = append(suggestions, "Use SHA-256 or SHA-3 instead of MD5/SHA1")
		suggestions = append(suggestions, "Use AES instead of DES/RC4")
	}

	return suggestions
}

func containsKind(kinds []gismov1.SymbolKind, kind gismov1.SymbolKind) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}
