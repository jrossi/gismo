package codesitter

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

// Implement gRPC service methods

// ShutdownWorkspace cleans up workspace resources
func (s *Server) ShutdownWorkspace(ctx context.Context, req *gismov1.ShutdownWorkspaceRequest) (*gismov1.ShutdownWorkspaceResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify session ID
	if req.SessionId != "" && req.SessionId != s.sessionID {
		return nil, status.Errorf(codes.InvalidArgument, "invalid session ID")
	}

	// Stop file watcher
	if s.watcher != nil {
		s.watcher.Close()
		s.watcher = nil
	}

	// Clean up parse trees
	if req.CleanupCache {
		for _, tree := range s.trees {
			if tree.Tree != nil {
				tree.Tree.Close()
			}
		}
		s.trees = make(map[string]*FileTree)
		s.symbolIndex.Clear()
	}

	// Close streaming channels
	for id, ch := range s.fileWatchers {
		close(ch)
		delete(s.fileWatchers, id)
	}
	for id, ch := range s.symbolWatchers {
		close(ch)
		delete(s.symbolWatchers, id)
	}
	for id, ch := range s.diagnosticWatchers {
		close(ch)
		delete(s.diagnosticWatchers, id)
	}

	return &gismov1.ShutdownWorkspaceResponse{
		Success: true,
		Message: "Workspace shutdown successfully",
	}, nil
}

// QuerySymbols searches for symbols using tree-sitter queries
func (s *Server) QuerySymbols(ctx context.Context, req *gismov1.QuerySymbolsRequest) (*gismov1.QuerySymbolsResponse, error) {
	engine := NewQueryEngine(s)
	return engine.QuerySymbols(ctx, req)
}

// FindReferences finds all references to a symbol
func (s *Server) FindReferences(ctx context.Context, req *gismov1.FindReferencesRequest) (*gismov1.FindReferencesResponse, error) {
	engine := NewQueryEngine(s)
	return engine.FindReferences(ctx, req)
}

// GetSymbolDefinition finds the definition of a symbol
func (s *Server) GetSymbolDefinition(ctx context.Context, req *gismov1.GetSymbolDefinitionRequest) (*gismov1.GetSymbolDefinitionResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Search through indexed symbols
	symbols := s.symbolIndex.FindByName(req.SymbolName, true)

	if len(symbols) == 0 {
		// Try partial match
		symbols = s.symbolIndex.FindByName(req.SymbolName, false)
	}

	if len(symbols) == 0 {
		return &gismov1.GetSymbolDefinitionResponse{
			Found: false,
		}, nil
	}

	// If context location provided, find closest match
	var bestMatch *gismov1.Symbol
	if req.ContextLocation != nil {
		bestMatch = s.findClosestSymbol(symbols, req.ContextLocation)
	} else {
		bestMatch = symbols[0]
	}

	return &gismov1.GetSymbolDefinitionResponse{
		Definition: bestMatch,
		Found:      true,
	}, nil
}

// GetCallGraph builds a call graph for a function
func (s *Server) GetCallGraph(ctx context.Context, req *gismov1.GetCallGraphRequest) (*gismov1.GetCallGraphResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find the target function
	var targetSymbol *gismov1.Symbol
	if req.FunctionLocation != nil {
		// Find by location
		for _, tree := range s.trees {
			for _, sym := range tree.Symbols {
				if s.locationsOverlap(sym.Location, req.FunctionLocation) {
					targetSymbol = sym
					break
				}
			}
			if targetSymbol != nil {
				break
			}
		}
	} else {
		// Find by name
		symbols := s.symbolIndex.FindByName(req.FunctionName, true)
		if len(symbols) > 0 {
			targetSymbol = symbols[0]
		}
	}

	if targetSymbol == nil {
		return &gismov1.GetCallGraphResponse{}, nil
	}

	// Build call graph
	visited := make(map[string]bool)
	root := s.buildCallGraphNode(targetSymbol, req.MaxDepth, 0, visited, req.IncludeIndirectCalls)

	return &gismov1.GetCallGraphResponse{
		Root:            root,
		TotalNodes:      int32(len(visited)),
		MaxDepthReached: req.MaxDepth,
	}, nil
}

// AnalyzeSecurity performs security analysis on code
func (s *Server) AnalyzeSecurity(ctx context.Context, req *gismov1.AnalyzeSecurityRequest) (*gismov1.AnalyzeSecurityResponse, error) {
	engine := NewQueryEngine(s)
	return engine.AnalyzeSecurity(ctx, req)
}

// DetectPatterns searches for code patterns
func (s *Server) DetectPatterns(ctx context.Context, req *gismov1.DetectPatternsRequest) (*gismov1.DetectPatternsResponse, error) {
	engine := NewQueryEngine(s)
	return engine.DetectPatterns(ctx, req)
}

// ValidateEdit validates whether an edit would be safe
func (s *Server) ValidateEdit(ctx context.Context, req *gismov1.ValidateEditRequest) (*gismov1.ValidateEditResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tree, exists := s.trees[req.FilePath]
	if !exists {
		return &gismov1.ValidateEditResponse{
			IsValid: false,
			Issues: []*gismov1.ValidationIssue{{
				Message:   "File not found in workspace",
				Severity:  gismov1.Diagnostic_SEVERITY_ERROR,
				IssueType: "file_not_found",
			}},
		}, nil
	}

	issues := make([]*gismov1.ValidationIssue, 0)
	affectedSymbols := make([]string, 0)
	wouldBreakSyntax := false

	// Apply edits temporarily to check syntax
	if req.CheckSyntax {
		tempContent := make([]byte, len(tree.Content))
		copy(tempContent, tree.Content)

		for _, edit := range req.Edits {
			// Apply edit to temp content
			if edit.Location != nil {
				start := edit.Location.StartByte
				end := edit.Location.EndByte

				if start >= 0 && end <= int32(len(tempContent)) {
					// Replace old text with new text
					newContent := append(tempContent[:start], []byte(edit.NewText)...)
					newContent = append(newContent, tempContent[end:]...)
					tempContent = newContent
				}
			}
		}

		// Try to parse the edited content
		parser, ok := s.parsers[tree.Language]
		if ok {
			testTree, err := parser.ParseCtx(ctx, nil, tempContent)
			if err != nil {
				wouldBreakSyntax = true
				issues = append(issues, &gismov1.ValidationIssue{
					Message:   fmt.Sprintf("Edit would break syntax: %v", err),
					Severity:  gismov1.Diagnostic_SEVERITY_ERROR,
					IssueType: "syntax_error",
				})
			} else {
				testTree.Close()
			}
		}
	}

	// Check which symbols would be affected
	if req.CheckReferences {
		for _, edit := range req.Edits {
			// Find symbols at edit location
			for _, sym := range tree.Symbols {
				if s.locationsOverlap(sym.Location, edit.Location) {
					affectedSymbols = append(affectedSymbols, sym.Name)
				}
			}
		}
	}

	// Run security analysis on the edits
	if req.CheckSecurity {
		securityIssues := s.checkEditSecurity(req.Edits)
		if len(securityIssues) > 0 {
			for _, issue := range securityIssues {
				issues = append(issues, &gismov1.ValidationIssue{
					Message:   issue.Message,
					Severity:  issue.Severity,
					Location:  issue.Location,
					IssueType: "security_risk",
				})
			}
		}
	}

	return &gismov1.ValidateEditResponse{
		IsValid:          len(issues) == 0 && !wouldBreakSyntax,
		Issues:           issues,
		WouldBreakSyntax: wouldBreakSyntax,
		AffectedSymbols:  affectedSymbols,
	}, nil
}

// GetDiagnostics returns diagnostics for files
func (s *Server) GetDiagnostics(ctx context.Context, req *gismov1.GetDiagnosticsRequest) (*gismov1.GetDiagnosticsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fileDiagnostics := make([]*gismov1.FileDiagnostics, 0)
	totalDiagnostics := int32(0)
	countsBySeverity := make(map[string]int32)

	for _, path := range req.FilePaths {
		tree, exists := s.trees[path]
		if !exists {
			continue
		}

		// Filter diagnostics by severity and source
		filteredDiags := make([]*gismov1.Diagnostic, 0)
		for _, diag := range tree.Diagnostics {
			// Check severity filter
			if len(req.IncludeSeverities) > 0 {
				found := false
				for _, sev := range req.IncludeSeverities {
					if diag.Severity == sev {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			// Check source filter
			if len(req.IncludeSources) > 0 {
				found := false
				for _, src := range req.IncludeSources {
					if diag.Source == src {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			filteredDiags = append(filteredDiags, diag)
			totalDiagnostics++
			countsBySeverity[diag.Severity.String()]++
		}

		if len(filteredDiags) > 0 {
			fileDiagnostics = append(fileDiagnostics, &gismov1.FileDiagnostics{
				FilePath:     path,
				Diagnostics:  filteredDiags,
				LastAnalyzed: timestamppb.New(tree.LastModified),
			})
		}
	}

	return &gismov1.GetDiagnosticsResponse{
		FileDiagnostics:  fileDiagnostics,
		TotalDiagnostics: totalDiagnostics,
		CountsBySeverity: countsBySeverity,
	}, nil
}

// GetTypeInfo returns type information for a location
func (s *Server) GetTypeInfo(ctx context.Context, req *gismov1.GetTypeInfoRequest) (*gismov1.GetTypeInfoResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find the file
	tree, exists := s.trees[req.Location.FilePath]
	if !exists {
		return &gismov1.GetTypeInfoResponse{Found: false}, nil
	}

	// Find symbol at location
	var targetSymbol *gismov1.Symbol
	for _, sym := range tree.Symbols {
		if s.locationsOverlap(sym.Location, req.Location) {
			targetSymbol = sym
			break
		}
	}

	if targetSymbol == nil {
		return &gismov1.GetTypeInfoResponse{Found: false}, nil
	}

	// Build type info
	typeInfo := &gismov1.TypeInfo{
		TypeName:      targetSymbol.Name,
		Kind:          s.symbolKindToTypeKind(targetSymbol.Kind),
		Documentation: targetSymbol.Documentation,
	}

	// Find members if it's a class/struct
	if targetSymbol.Kind == gismov1.SymbolKind_SYMBOL_KIND_CLASS ||
		targetSymbol.Kind == gismov1.SymbolKind_SYMBOL_KIND_STRUCT {
		typeInfo.Members = s.findMembers(targetSymbol.Name)
	}

	return &gismov1.GetTypeInfoResponse{
		TypeInfo: typeInfo,
		Found:    true,
	}, nil
}

// SuggestRefactoring suggests refactoring options
func (s *Server) SuggestRefactoring(ctx context.Context, req *gismov1.SuggestRefactoringRequest) (*gismov1.SuggestRefactoringResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	suggestions := make([]*gismov1.RefactoringSuggestion, 0)

	// Find the file and node at location
	tree, exists := s.trees[req.Location.FilePath]
	if !exists {
		return &gismov1.SuggestRefactoringResponse{}, nil
	}

	// Get the node at the location
	node := tree.Tree.RootNode().NamedDescendantForPointRange(
		sitter.Point{
			Row:    uint32(req.Location.StartLine - 1),
			Column: uint32(req.Location.StartColumn - 1),
		},
		sitter.Point{
			Row:    uint32(req.Location.EndLine - 1),
			Column: uint32(req.Location.EndColumn - 1),
		},
	)

	if node == nil {
		return &gismov1.SuggestRefactoringResponse{}, nil
	}

	// Suggest refactorings based on node type
	nodeType := node.Type()

	// Extract function
	if nodeType == "block" || nodeType == "statement_block" {
		suggestions = append(suggestions, &gismov1.RefactoringSuggestion{
			Kind:        "extract_function",
			Title:       "Extract Function",
			Description: "Extract selected code into a new function",
			Priority:    1,
		})
	}

	// Rename symbol
	if nodeType == "identifier" {
		suggestions = append(suggestions, &gismov1.RefactoringSuggestion{
			Kind:        "rename",
			Title:       "Rename Symbol",
			Description: "Rename this symbol throughout the codebase",
			Priority:    2,
		})
	}

	// Extract variable
	if nodeType == "call_expression" || nodeType == "binary_expression" {
		suggestions = append(suggestions, &gismov1.RefactoringSuggestion{
			Kind:        "extract_variable",
			Title:       "Extract Variable",
			Description: "Extract expression into a variable",
			Priority:    3,
		})
	}

	return &gismov1.SuggestRefactoringResponse{
		Suggestions: suggestions,
	}, nil
}

// GetCodeMetrics calculates code metrics
func (s *Server) GetCodeMetrics(ctx context.Context, req *gismov1.GetCodeMetricsRequest) (*gismov1.GetCodeMetricsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fileMetrics := make([]*gismov1.FileMetrics, 0)
	aggregates := make(map[string]float64)
	totals := make(map[string]float64)

	for path, tree := range s.trees {
		if !s.matchesPatterns(path, req.FilePatterns) {
			continue
		}

		metrics := s.calculateFileMetrics(tree, req.MetricTypes)

		// Calculate function-level metrics
		funcMetrics := make([]*gismov1.FunctionMetrics, 0)
		for _, sym := range tree.Symbols {
			if sym.Kind == gismov1.SymbolKind_SYMBOL_KIND_FUNCTION ||
				sym.Kind == gismov1.SymbolKind_SYMBOL_KIND_METHOD {
				fm := &gismov1.FunctionMetrics{
					FunctionName: sym.Name,
					Location:     sym.Location,
					Metrics:      s.calculateFunctionMetrics(tree, sym),
				}
				funcMetrics = append(funcMetrics, fm)
			}
		}

		fileMetrics = append(fileMetrics, &gismov1.FileMetrics{
			FilePath:        path,
			Metrics:         metrics,
			FunctionMetrics: funcMetrics,
		})

		// Update aggregates
		for key, value := range metrics {
			totals[key] += value
			aggregates[key]++ // Count for averaging
		}
	}

	// Calculate averages
	averages := make(map[string]float64)
	for key, total := range totals {
		if count := aggregates[key]; count > 0 {
			averages[key] = total / count
		}
	}

	return &gismov1.GetCodeMetricsResponse{
		FileMetrics: fileMetrics,
		Aggregate: &gismov1.AggregateMetrics{
			Averages: averages,
			Totals:   totals,
		},
	}, nil
}

// Streaming methods

// WatchFiles streams file change events
func (s *Server) WatchFiles(req *gismov1.WatchFilesRequest, stream gismov1.CodeSitter_WatchFilesServer) error {
	s.mu.Lock()
	watcherID := fmt.Sprintf("file-watcher-%d", len(s.fileWatchers))
	ch := make(chan *gismov1.FileChangeEvent, 100)
	s.fileWatchers[watcherID] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.fileWatchers, watcherID)
		s.mu.Unlock()
	}()

	for event := range ch {
		// Filter by patterns and change kinds
		if len(req.FilePatterns) > 0 && !s.matchesPatterns(event.FilePath, req.FilePatterns) {
			continue
		}

		if len(req.ChangeKinds) > 0 {
			found := false
			for _, kind := range req.ChangeKinds {
				if event.Kind == kind {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if err := stream.Send(event); err != nil {
			return err
		}
	}

	return nil
}

// WatchSymbols streams symbol change events
func (s *Server) WatchSymbols(req *gismov1.WatchSymbolsRequest, stream gismov1.CodeSitter_WatchSymbolsServer) error {
	s.mu.Lock()
	watcherID := fmt.Sprintf("symbol-watcher-%d", len(s.symbolWatchers))
	ch := make(chan *gismov1.SymbolChangeEvent, 100)
	s.symbolWatchers[watcherID] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.symbolWatchers, watcherID)
		s.mu.Unlock()
	}()

	for event := range ch {
		// Filter by patterns and symbol kinds
		if event.Symbol != nil {
			if len(req.FilePatterns) > 0 && !s.matchesPatterns(event.Symbol.Location.FilePath, req.FilePatterns) {
				continue
			}

			if len(req.SymbolKinds) > 0 {
				found := false
				for _, kind := range req.SymbolKinds {
					if event.Symbol.Kind == kind {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
		}

		if err := stream.Send(event); err != nil {
			return err
		}
	}

	return nil
}

// WatchDiagnostics streams diagnostic events
func (s *Server) WatchDiagnostics(req *gismov1.WatchDiagnosticsRequest, stream gismov1.CodeSitter_WatchDiagnosticsServer) error {
	s.mu.Lock()
	watcherID := fmt.Sprintf("diagnostic-watcher-%d", len(s.diagnosticWatchers))
	ch := make(chan *gismov1.DiagnosticEvent, 100)
	s.diagnosticWatchers[watcherID] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.diagnosticWatchers, watcherID)
		s.mu.Unlock()
	}()

	for event := range ch {
		// Filter by patterns and severity
		if len(req.FilePatterns) > 0 && !s.matchesPatterns(event.FilePath, req.FilePatterns) {
			continue
		}

		// Filter diagnostics by minimum severity
		if len(req.MinSeverity) > 0 {
			// Check if any diagnostic meets minimum severity
			hasValidSeverity := false
			for _, diag := range event.Added {
				for _, minSev := range req.MinSeverity {
					if diag.Severity <= minSev {
						hasValidSeverity = true
						break
					}
				}
				if hasValidSeverity {
					break
				}
			}
			if !hasValidSeverity {
				continue
			}
		}

		if err := stream.Send(event); err != nil {
			return err
		}
	}

	return nil
}

// Helper methods

func (s *Server) findClosestSymbol(symbols []*gismov1.Symbol, location *gismov1.Location) *gismov1.Symbol {
	var closest *gismov1.Symbol
	minDistance := int32(999999)

	for _, sym := range symbols {
		if sym.Location == nil {
			continue
		}

		// Calculate distance (simple line difference)
		distance := abs(sym.Location.StartLine - location.StartLine)
		if distance < minDistance {
			minDistance = distance
			closest = sym
		}
	}

	return closest
}

func (s *Server) locationsOverlap(loc1, loc2 *gismov1.Location) bool {
	if loc1 == nil || loc2 == nil {
		return false
	}

	// Check if locations overlap
	return loc1.FilePath == loc2.FilePath &&
		loc1.StartByte <= loc2.EndByte &&
		loc1.EndByte >= loc2.StartByte
}

func (s *Server) buildCallGraphNode(symbol *gismov1.Symbol, maxDepth, currentDepth int32, visited map[string]bool, includeIndirect bool) *gismov1.CallGraphNode {
	if symbol == nil || currentDepth >= maxDepth {
		return nil
	}

	key := fmt.Sprintf("%s:%s", symbol.Location.FilePath, symbol.Name)
	if visited[key] {
		return nil // Avoid cycles
	}
	visited[key] = true

	node := &gismov1.CallGraphNode{
		Symbol:   symbol,
		Calls:    make([]*gismov1.CallGraphEdge, 0),
		CalledBy: make([]*gismov1.CallGraphEdge, 0),
	}

	// Find calls from this function
	// This is simplified - in reality would need to parse function body
	// and find call expressions

	return node
}

func (s *Server) checkEditSecurity(edits []*gismov1.Edit) []*gismov1.SecurityIssue {
	issues := make([]*gismov1.SecurityIssue, 0)

	for _, edit := range edits {
		// Check for dangerous patterns in new text
		newText := strings.ToLower(edit.NewText)

		// Check for hardcoded secrets
		if strings.Contains(newText, "password") || strings.Contains(newText, "secret") || strings.Contains(newText, "api_key") {
			if strings.Contains(newText, "=") && strings.Contains(newText, "\"") {
				issues = append(issues, &gismov1.SecurityIssue{
					RuleId:   "hardcoded-secret",
					RuleName: "Potential Hardcoded Secret",
					Location: edit.Location,
					Message:  "Edit appears to introduce a hardcoded secret",
					Severity: gismov1.Diagnostic_SEVERITY_ERROR,
				})
			}
		}

		// Check for SQL injection patterns
		if strings.Contains(newText, "query") || strings.Contains(newText, "exec") {
			if strings.Contains(newText, "+") || strings.Contains(newText, "concat") {
				issues = append(issues, &gismov1.SecurityIssue{
					RuleId:   "sql-injection",
					RuleName: "Potential SQL Injection",
					Location: edit.Location,
					Message:  "Edit may introduce SQL injection vulnerability",
					Severity: gismov1.Diagnostic_SEVERITY_WARNING,
				})
			}
		}
	}

	return issues
}

func (s *Server) symbolKindToTypeKind(kind gismov1.SymbolKind) string {
	switch kind {
	case gismov1.SymbolKind_SYMBOL_KIND_CLASS:
		return "class"
	case gismov1.SymbolKind_SYMBOL_KIND_INTERFACE:
		return "interface"
	case gismov1.SymbolKind_SYMBOL_KIND_STRUCT:
		return "struct"
	case gismov1.SymbolKind_SYMBOL_KIND_ENUM:
		return "enum"
	default:
		return "unknown"
	}
}

func (s *Server) findMembers(typeName string) []*gismov1.Symbol {
	members := make([]*gismov1.Symbol, 0)

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find all symbols that have this type as parent
	for _, tree := range s.trees {
		for _, sym := range tree.Symbols {
			if sym.ParentSymbol == typeName {
				members = append(members, sym)
			}
		}
	}

	return members
}

func (s *Server) calculateFileMetrics(tree *FileTree, metricTypes []string) map[string]float64 {
	metrics := make(map[string]float64)

	// Lines of code
	if contains(metricTypes, "loc") || len(metricTypes) == 0 {
		lines := strings.Split(string(tree.Content), "\n")
		loc := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "#") {
				loc++
			}
		}
		metrics["loc"] = float64(loc)
	}

	// Number of functions
	if contains(metricTypes, "functions") || len(metricTypes) == 0 {
		funcCount := 0
		for _, sym := range tree.Symbols {
			if sym.Kind == gismov1.SymbolKind_SYMBOL_KIND_FUNCTION ||
				sym.Kind == gismov1.SymbolKind_SYMBOL_KIND_METHOD {
				funcCount++
			}
		}
		metrics["functions"] = float64(funcCount)
	}

	// Number of classes
	if contains(metricTypes, "classes") || len(metricTypes) == 0 {
		classCount := 0
		for _, sym := range tree.Symbols {
			if sym.Kind == gismov1.SymbolKind_SYMBOL_KIND_CLASS {
				classCount++
			}
		}
		metrics["classes"] = float64(classCount)
	}

	return metrics
}

func (s *Server) calculateFunctionMetrics(tree *FileTree, symbol *gismov1.Symbol) map[string]float64 {
	metrics := make(map[string]float64)

	if symbol.Location == nil {
		return metrics
	}

	// Calculate function length
	metrics["length"] = float64(symbol.Location.EndLine - symbol.Location.StartLine + 1)

	// Simple complexity estimation (would need proper AST analysis)
	content := string(tree.Content[symbol.Location.StartByte:symbol.Location.EndByte])

	// Count control flow statements as complexity
	complexity := 1.0
	controlFlow := []string{"if ", "for ", "while ", "switch ", "case ", "catch "}
	for _, cf := range controlFlow {
		complexity += float64(strings.Count(content, cf))
	}
	metrics["complexity"] = complexity

	return metrics
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SearchForPattern searches for patterns in code (AST-aware grep)
func (s *Server) SearchForPattern(ctx context.Context, req *gismov1.SearchForPatternRequest) (*gismov1.SearchForPatternResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matches := []*gismov1.SearchForPatternResponse_Match{}
	filesSearched := 0

	// Search through all parsed files
	for path, tree := range s.trees {
		// Check if file matches patterns
		if len(req.FilePatterns) > 0 && !s.matchesPatterns(path, req.FilePatterns) {
			continue
		}
		filesSearched++

		// Search in file content
		lines := strings.Split(string(tree.Content), "\n")
		for i, line := range lines {
			matched := false
			if req.UseRegex {
				if re, err := regexp.Compile(req.Pattern); err == nil {
					matched = re.MatchString(line)
				}
			} else {
				if req.CaseSensitive {
					matched = strings.Contains(line, req.Pattern)
				} else {
					matched = strings.Contains(strings.ToLower(line), strings.ToLower(req.Pattern))
				}
			}

			if matched {
				// Collect context lines
				contextBefore := []string{}
				for j := max(0, i-int(req.ContextLinesBefore)); j < i; j++ {
					contextBefore = append(contextBefore, lines[j])
				}

				contextAfter := []string{}
				for j := i + 1; j <= min(len(lines)-1, i+int(req.ContextLinesAfter)); j++ {
					contextAfter = append(contextAfter, lines[j])
				}

				matches = append(matches, &gismov1.SearchForPatternResponse_Match{
					FilePath:      path,
					LineNumber:    int32(i + 1),
					LineText:      line,
					ContextBefore: contextBefore,
					ContextAfter:  contextAfter,
					Location: &gismov1.Location{
						FilePath:    path,
						StartLine:   int32(i + 1),
						StartColumn: 1,
						EndLine:     int32(i + 1),
						EndColumn:   int32(len(line)),
					},
				})
			}
		}
	}

	return &gismov1.SearchForPatternResponse{
		Matches:        matches,
		TotalMatches:   int32(len(matches)),
		FilesSearched:  int32(filesSearched),
	}, nil
}

// GetSymbolsOverview returns a hierarchical view of symbols in a file
func (s *Server) GetSymbolsOverview(ctx context.Context, req *gismov1.GetSymbolsOverviewRequest) (*gismov1.GetSymbolsOverviewResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tree, exists := s.trees[req.FilePath]
	if !exists {
		return nil, fmt.Errorf("file not found: %s", req.FilePath)
	}

	// Extract all symbols from the file
	symbols := s.extractSymbols(tree)
	
	// Build symbol tree
	symbolTrees := s.buildSymbolTree(symbols, req.IncludeKinds, req.MaxDepth)

	return &gismov1.GetSymbolsOverviewResponse{
		Symbols:      symbolTrees,
		TotalSymbols: int32(len(symbols)),
	}, nil
}

// FindSymbol finds symbols by name pattern (supports path patterns like "class/method")
func (s *Server) FindSymbol(ctx context.Context, req *gismov1.FindSymbolRequest) (*gismov1.FindSymbolResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	foundSymbols := []*gismov1.Symbol{}
	
	// Parse the name pattern (e.g., "MyClass/myMethod" or just "myMethod")
	patternParts := strings.Split(req.NamePattern, "/")
	
	// Search through all symbols in the index
	for _, symbols := range s.symbolIndex.byName {
		for _, symbol := range symbols {
			// Check file path filter if specified
			if req.FilePath != "" && symbol.Location.FilePath != req.FilePath {
				continue
			}

			// Check symbol kind filter
			if len(req.IncludeKinds) > 0 {
				found := false
				for _, kind := range req.IncludeKinds {
					if symbol.Kind == kind {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			// Match against pattern
			matched := false
			if len(patternParts) == 1 {
				// Simple name matching
				if req.SubstringMatching {
					matched = strings.Contains(symbol.Name, patternParts[0])
				} else {
					matched = symbol.Name == patternParts[0]
				}
			} else {
				// Path pattern matching (e.g., "class/method")
				// Check if symbol has the right parent
				if symbol.ParentSymbol != "" {
					parentParts := strings.Split(symbol.ParentSymbol, "/")
					if len(parentParts) > 0 {
						lastParent := parentParts[len(parentParts)-1]
						if lastParent == patternParts[0] {
							if req.SubstringMatching {
								matched = strings.Contains(symbol.Name, patternParts[1])
							} else {
								matched = symbol.Name == patternParts[1]
							}
						}
					}
				}
			}

			if matched {
				foundSymbols = append(foundSymbols, symbol)
				if req.MaxResults > 0 && int32(len(foundSymbols)) >= req.MaxResults {
					break
				}
			}
		}
		
		if req.MaxResults > 0 && int32(len(foundSymbols)) >= req.MaxResults {
			break
		}
	}

	return &gismov1.FindSymbolResponse{
		Symbols:    foundSymbols,
		TotalFound: int32(len(foundSymbols)),
	}, nil
}

// FindReferencingSymbols finds all symbols that reference a given symbol
func (s *Server) FindReferencingSymbols(ctx context.Context, req *gismov1.FindReferencingSymbolsRequest) (*gismov1.FindReferencingSymbolsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	references := []*gismov1.FindReferencingSymbolsResponse_ReferencingSymbol{}

	// Find the target symbol
	targetSymbol := s.findSymbolAtLocation(req.SymbolLocation)
	if targetSymbol == nil {
		// Fall back to searching by name
		if symbols, ok := s.symbolIndex.byName[req.SymbolName]; ok && len(symbols) > 0 {
			targetSymbol = symbols[0]
		}
	}

	if targetSymbol == nil {
		return &gismov1.FindReferencingSymbolsResponse{
			References:       references,
			TotalReferences: 0,
		}, nil
	}

	// Search for references in all files
	for path, tree := range s.trees {
		// Check file pattern filter
		if len(req.FilePatterns) > 0 && !s.matchesPatterns(path, req.FilePatterns) {
			continue
		}

		// Search for the symbol name in the file content
		content := string(tree.Content)
		lines := strings.Split(content, "\n")
		
		for lineNum, line := range lines {
			if strings.Contains(line, targetSymbol.Name) {
				// Find the containing symbol at this location
				loc := &gismov1.Location{
					FilePath:  path,
					StartLine: int32(lineNum + 1),
				}
				
				containingSymbol := s.findContainingSymbol(path, loc)
				
				// Determine reference kind
				kind := gismov1.ReferenceKind_REFERENCE_KIND_READ
				if strings.Contains(line, targetSymbol.Name+"(") {
					kind = gismov1.ReferenceKind_REFERENCE_KIND_CALL
				} else if strings.Contains(line, targetSymbol.Name+" =") || strings.Contains(line, targetSymbol.Name+" :=") {
					kind = gismov1.ReferenceKind_REFERENCE_KIND_WRITE
				}

				references = append(references, &gismov1.FindReferencingSymbolsResponse_ReferencingSymbol{
					ContainingSymbol:   containingSymbol,
					ReferenceLocation:  loc,
					ReferenceText:     strings.TrimSpace(line),
					Kind:              kind,
				})
			}
		}
	}

	return &gismov1.FindReferencingSymbolsResponse{
		References:       references,
		TotalReferences: int32(len(references)),
	}, nil
}

// Helper: Build hierarchical symbol tree
func (s *Server) buildSymbolTree(symbols []*gismov1.Symbol, includeKinds []gismov1.SymbolKind, maxDepth int32) []*gismov1.GetSymbolsOverviewResponse_SymbolTree {
	trees := []*gismov1.GetSymbolsOverviewResponse_SymbolTree{}
	
	// First pass: create all tree nodes
	nodeMap := make(map[string]*gismov1.GetSymbolsOverviewResponse_SymbolTree)
	for _, symbol := range symbols {
		// Filter by kind if specified
		if len(includeKinds) > 0 {
			found := false
			for _, kind := range includeKinds {
				if symbol.Kind == kind {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		node := &gismov1.GetSymbolsOverviewResponse_SymbolTree{
			Symbol:   symbol,
			Children: []*gismov1.GetSymbolsOverviewResponse_SymbolTree{},
		}
		nodeMap[symbol.Name] = node
	}

	// Second pass: build hierarchy
	for _, symbol := range symbols {
		node, exists := nodeMap[symbol.Name]
		if !exists {
			continue
		}

		if symbol.ParentSymbol == "" {
			// Top-level symbol
			trees = append(trees, node)
		} else {
			// Find parent and add as child
			if parent, ok := nodeMap[symbol.ParentSymbol]; ok {
				parent.Children = append(parent.Children, node)
			} else {
				// Parent not in filtered list, add as top-level
				trees = append(trees, node)
			}
		}
	}

	// Apply max depth if specified
	if maxDepth > 0 {
		trees = s.pruneTreeDepth(trees, 0, maxDepth)
	}

	return trees
}

// Helper: Prune tree to max depth
func (s *Server) pruneTreeDepth(trees []*gismov1.GetSymbolsOverviewResponse_SymbolTree, currentDepth, maxDepth int32) []*gismov1.GetSymbolsOverviewResponse_SymbolTree {
	if currentDepth >= maxDepth {
		for _, tree := range trees {
			tree.Children = nil
		}
		return trees
	}

	for _, tree := range trees {
		tree.Children = s.pruneTreeDepth(tree.Children, currentDepth+1, maxDepth)
	}
	return trees
}

// Helper: Find symbol at a specific location
func (s *Server) findSymbolAtLocation(location *gismov1.Location) *gismov1.Symbol {
	if location == nil {
		return nil
	}

	symbols, ok := s.symbolIndex.byFile[location.FilePath]
	if !ok {
		return nil
	}

	for _, symbol := range symbols {
		if symbol.Location.StartLine <= location.StartLine &&
		   symbol.Location.EndLine >= location.StartLine {
			return symbol
		}
	}
	return nil
}

// Helper: Find containing symbol for a location
func (s *Server) findContainingSymbol(filePath string, location *gismov1.Location) *gismov1.Symbol {
	symbols, ok := s.symbolIndex.byFile[filePath]
	if !ok {
		return nil
	}

	var bestMatch *gismov1.Symbol
	for _, symbol := range symbols {
		// Check if location is within symbol bounds
		if symbol.Location.StartLine <= location.StartLine &&
		   symbol.Location.EndLine >= location.StartLine {
			// Keep the smallest containing symbol
			if bestMatch == nil ||
			   (symbol.Location.EndLine - symbol.Location.StartLine) < (bestMatch.Location.EndLine - bestMatch.Location.StartLine) {
				bestMatch = symbol
			}
		}
	}
	return bestMatch
}

// Helper: min/max functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// syncMap provides a thread-safe string map
type syncMap struct {
	mu sync.RWMutex
	m  map[string]interface{}
}

func (sm *syncMap) Get(key string) (interface{}, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	v, ok := sm.m[key]
	return v, ok
}

func (sm *syncMap) Set(key string, value interface{}) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}

func (sm *syncMap) Delete(key string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.m, key)
}
