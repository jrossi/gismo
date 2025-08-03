package golang

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jrossi/gismo/pkg/linters"

	json "github.com/goccy/go-json"
)

// GoLinter handles Go file linting, formatting, and test running with golangci-lint integration
type GoLinter struct {
	// Cache module roots to avoid repeated filesystem walks
	moduleCache map[string]*ModuleInfo
	// Cache golangci-lint binary path for performance
	golangciPath string
	golangciOnce sync.Once
	mu           sync.RWMutex
	config       *GolangConfig
}

// GolangConfig represents golang linter specific configuration
type GolangConfig struct {
	GolangciConfig     *string                      `json:"golangciConfig,omitempty"` // path to golangci.yml
	DisabledChecks     []string                     `json:"disabledChecks,omitempty"`
	TestTimeout        *Duration                    `json:"testTimeout,omitempty"`
	ImportRestrictions map[string]ImportRestriction `json:"importRestrictions,omitempty"` // package import restrictions
}

// ImportRestriction defines rules for restricting package imports
type ImportRestriction struct {
	Blocked     bool   `json:"blocked,omitempty"`     // if true, this import is completely blocked
	Replacement string `json:"replacement,omitempty"` // suggested replacement package
	Reason      string `json:"reason,omitempty"`      // explanation for the restriction
}

// Duration is a wrapper around time.Duration for JSON unmarshaling
type Duration struct {
	time.Duration
}

// GolangciLintIssue represents an issue from golangci-lint JSON output
type GolangciLintIssue struct {
	FromLinter  string   `json:"FromLinter"`
	Text        string   `json:"Text"`
	Severity    string   `json:"Severity"`
	SourceLines []string `json:"SourceLines"`
	Replacement struct {
		NewLines []string `json:"NewLines"`
	} `json:"Replacement"`
	Pos struct {
		Filename string `json:"Filename"`
		Offset   int    `json:"Offset"`
		Line     int    `json:"Line"`
		Column   int    `json:"Column"`
	} `json:"Pos"`
}

// GolangciLintOutput represents the complete JSON output from golangci-lint
type GolangciLintOutput struct {
	Issues []GolangciLintIssue `json:"Issues"`
}

// ModuleInfo contains information about a Go module
type ModuleInfo struct {
	Root      string // Module root directory (where go.mod is located)
	Path      string // Module path from go.mod
	GoModPath string // Full path to go.mod file
}

// NewGoLinter creates a new Go linter with default configuration
func NewGoLinter() *GoLinter {
	return NewGoLinterWithConfig(nil)
}

// NewGoLinterWithConfig creates a new Go linter with the given configuration
func NewGoLinterWithConfig(config *GolangConfig) *GoLinter {
	// Apply default configuration if not provided
	if config == nil {
		config = &GolangConfig{}
	}

	// Set defaults
	if config.TestTimeout == nil {
		defaultTimeout := &Duration{Duration: 10 * time.Minute}
		config.TestTimeout = defaultTimeout
	}

	return &GoLinter{
		moduleCache: make(map[string]*ModuleInfo),
		config:      config,
	}
}

// SetConfig updates the linter configuration
func (l *GoLinter) SetConfig(configData json.RawMessage) error {
	var config GolangConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse golang config: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Update config
	l.config = &config

	// Set defaults if not provided
	if l.config.TestTimeout == nil {
		defaultTimeout := &Duration{Duration: 10 * time.Minute}
		l.config.TestTimeout = defaultTimeout
	}

	return nil
}

// isCheckDisabled returns true if the given check/linter is disabled in config
func (l *GoLinter) isCheckDisabled(checkName string) bool {
	if l.config == nil || len(l.config.DisabledChecks) == 0 {
		return false
	}

	for _, disabled := range l.config.DisabledChecks {
		if disabled == checkName {
			return true
		}
	}

	return false
}

// Name returns the linter name
func (l *GoLinter) Name() string {
	return "go"
}

// CanHandle returns true for Go files
func (l *GoLinter) CanHandle(filePath string) bool {
	return strings.HasSuffix(filePath, ".go")
}

// findGolangciLint locates the golangci-lint binary and caches the path
func (l *GoLinter) findGolangciLint() string {
	l.golangciOnce.Do(func() {
		// Check standard Go installation location first
		standardPath := filepath.Join(os.Getenv("HOME"), "go", "bin", "golangci-lint")
		if _, err := os.Stat(standardPath); err == nil {
			l.golangciPath = standardPath
			return
		}

		// Check PATH
		if path, err := exec.LookPath("golangci-lint"); err == nil {
			l.golangciPath = path
			return
		}

		// Not found
		l.golangciPath = ""
	})
	return l.golangciPath
}

// runGolangciLint executes golangci-lint with fast mode on the specified file
func (l *GoLinter) runGolangciLint(ctx context.Context, filePath string) (*GolangciLintOutput, error) {
	return l.runGolangciLintMultiple(ctx, []string{filePath})
}

// runGolangciLintMultiple executes golangci-lint on multiple files at once for better performance
func (l *GoLinter) runGolangciLintMultiple(ctx context.Context, filePaths []string) (*GolangciLintOutput, error) {
	if len(filePaths) == 0 {
		return &GolangciLintOutput{}, nil
	}

	golangciPath := l.findGolangciLint()
	if golangciPath == "" {
		return nil, fmt.Errorf("golangci-lint not found")
	}

	// Find module root for proper context (use first file)
	moduleInfo, err := l.FindModuleRoot(filePaths[0])
	if err != nil {
		return nil, fmt.Errorf("failed to find module root: %w", err)
	}

	// Build golangci-lint arguments
	args := []string{"run", "--fast", "--out-format=json"}

	// Check for configured golangci config file first
	if l.config != nil && l.config.GolangciConfig != nil && *l.config.GolangciConfig != "" {
		// Use the configured path
		args = append(args, "--config="+*l.config.GolangciConfig)
	} else {
		// Check for default .golangci.yml config file
		configPath := filepath.Join(moduleInfo.Root, ".golangci.yml")
		if _, err := os.Stat(configPath); err == nil {
			args = append(args, "--config="+configPath)
		}
	}

	// Add parallelism flag for better performance
	args = append(args, "--concurrency", fmt.Sprintf("%d", runtime.NumCPU()))

	// Add all file paths
	args = append(args, filePaths...)

	// Execute golangci-lint
	cmd := exec.CommandContext(ctx, golangciPath, args...)
	cmd.Dir = moduleInfo.Root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// golangci-lint returns non-zero exit code when issues are found, which is expected
	err = cmd.Run()

	// Check if the error is due to issues found (expected) or actual failure
	if err != nil && stdout.Len() == 0 {
		return nil, fmt.Errorf("golangci-lint failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse JSON output
	var output GolangciLintOutput
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			return nil, fmt.Errorf("failed to parse golangci-lint output: %w", err)
		}
	}

	return &output, nil
}

// convertGolangciIssues converts golangci-lint issues to our internal Issue format
func (l *GoLinter) convertGolangciIssues(golangciIssues []GolangciLintIssue) []linters.Issue {
	var issues []linters.Issue
	for _, issue := range golangciIssues {
		// Skip disabled checks
		if l.isCheckDisabled(issue.FromLinter) {
			continue
		}

		severity := "warning"
		if issue.Severity == "error" {
			severity = "error"
		}

		issues = append(issues, linters.Issue{
			File:     issue.Pos.Filename,
			Line:     issue.Pos.Line,
			Column:   issue.Pos.Column,
			Severity: severity,
			Message:  issue.Text,
			Rule:     issue.FromLinter,
		})
	}
	return issues
}

// Lint performs enhanced linting on a Go file using golangci-lint with fallback
func (l *GoLinter) Lint(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Skip generated files
	if bytes.Contains(content, []byte("// Code generated")) {
		return result, nil
	}

	// Skip test data files
	if strings.Contains(filePath, "/testdata/") {
		return result, nil
	}

	// Always check basic syntax first with go/format (fast and reliable)
	formatted, err := format.Source(content)
	if err != nil {
		result.Success = false
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("Go syntax error: %v", err),
			Rule:     "syntax",
		})
		return result, nil
	}

	// Check if formatting is needed
	if !bytes.Equal(content, formatted) {
		result.Formatted = formatted
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "warning",
			Message:  "File is not properly formatted with gofmt",
			Rule:     "gofmt",
		})
	}

	// Check import restrictions using AST parsing
	importIssues := l.checkImportRestrictions(filePath, content)
	result.Issues = append(result.Issues, importIssues...)

	// Mark as failed if any import restrictions are violated
	for _, issue := range importIssues {
		if issue.Severity == "error" {
			result.Success = false
		}
	}

	// Try enhanced linting with golangci-lint fast mode
	if golangciOutput, err := l.runGolangciLint(ctx, filePath); err == nil {
		// Successfully ran golangci-lint, add its issues
		golangciIssues := l.convertGolangciIssues(golangciOutput.Issues)
		result.Issues = append(result.Issues, golangciIssues...)

		// Check if any issues are errors (should block)
		for _, issue := range golangciIssues {
			if issue.Severity == "error" {
				result.Success = false
			}
		}
	}
	// If golangci-lint fails, we continue with basic linting (graceful fallback)

	// Run tests if this is a test file
	if strings.HasSuffix(filePath, "_test.go") {
		if output, err := l.runTests(ctx, filePath); err != nil {
			result.Success = false
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     1,
				Column:   1,
				Severity: "error",
				Message:  fmt.Sprintf("Tests failed: %v", err),
				Rule:     "test",
			})
			result.TestOutput = output
		} else {
			result.TestOutput = output
		}
	} else {
		// For non-test files, check if corresponding test file exists and run it
		testFile := strings.TrimSuffix(filePath, ".go") + "_test.go"
		if _, err := os.Stat(testFile); err == nil {
			if output, err := l.runTests(ctx, testFile); err != nil {
				result.Success = false
				result.Issues = append(result.Issues, linters.Issue{
					File:     testFile,
					Line:     1,
					Column:   1,
					Severity: "error",
					Message:  fmt.Sprintf("Tests failed: %v", err),
					Rule:     "test",
				})
				result.TestOutput = output
			} else {
				result.TestOutput = output
			}
		}
	}

	return result, nil
}

// findCommonPrefix finds the longest common prefix among test names
func findCommonPrefix(tests []string) string {
	if len(tests) == 0 {
		return ""
	}
	if len(tests) == 1 {
		return tests[0]
	}

	// Start with the first test name as the initial prefix
	prefix := tests[0]

	// Compare with each subsequent test name
	for i := 1; i < len(tests); i++ {
		// Find the common prefix between current prefix and this test
		j := 0
		for j < len(prefix) && j < len(tests[i]) && prefix[j] == tests[i][j] {
			j++
		}
		// Update prefix to the common part
		prefix = prefix[:j]

		// If no common prefix remains, we can stop
		if prefix == "" {
			break
		}
	}

	return prefix
}

// extractTestFunctions parses a Go test file and extracts all test function names
func (l *GoLinter) extractTestFunctions(filePath string) ([]string, error) {
	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the file - we only need function declarations, not the full AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	var testFunctions []string

	// Walk through all declarations in the file
	for _, decl := range file.Decls {
		// Check if it's a function declaration
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Check if the function name starts with "Test"
		if !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}

		// Check if it has exactly one parameter
		if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
			continue
		}

		// Check if the parameter is of type *testing.T
		param := fn.Type.Params.List[0]
		if len(param.Names) != 1 {
			continue
		}

		// Check the parameter type
		starExpr, ok := param.Type.(*ast.StarExpr)
		if !ok {
			continue
		}

		// Check if it's *testing.T
		selectorExpr, ok := starExpr.X.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		// Check the package and type name
		if ident, ok := selectorExpr.X.(*ast.Ident); ok && ident.Name == "testing" && selectorExpr.Sel.Name == "T" {
			testFunctions = append(testFunctions, fn.Name.Name)
		}
	}

	return testFunctions, nil
}

// checkImportRestrictions analyzes Go source code for restricted imports using AST parsing
func (l *GoLinter) checkImportRestrictions(filePath string, content []byte) []linters.Issue {
	var issues []linters.Issue

	// Skip if no import restrictions are configured
	if l.config == nil || len(l.config.ImportRestrictions) == 0 {
		return issues
	}

	// Parse the file into an AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		// If we can't parse the file, skip import checking (syntax errors will be caught elsewhere)
		return issues
	}

	// Walk through all imports in the file
	for _, importSpec := range file.Imports {
		if importSpec.Path == nil {
			continue
		}

		// Extract the import path (remove quotes)
		importPath := strings.Trim(importSpec.Path.Value, `"`)

		// Check if this import is restricted
		if restriction, exists := l.config.ImportRestrictions[importPath]; exists && restriction.Blocked {
			// Get the position of the import
			pos := fset.Position(importSpec.Pos())

			// Build the error message
			message := fmt.Sprintf("Import '%s' is not allowed", importPath)
			if restriction.Reason != "" {
				message += ": " + restriction.Reason
			}
			if restriction.Replacement != "" {
				message += fmt.Sprintf(". Use '%s' instead", restriction.Replacement)
			}

			issues = append(issues, linters.Issue{
				File:     filePath,
				Line:     pos.Line,
				Column:   pos.Column,
				Severity: "error",
				Message:  message,
				Rule:     "import-restriction",
			})
		}
	}

	return issues
}

// FindModuleRoot walks up the directory tree to find go.mod
func (l *GoLinter) FindModuleRoot(startPath string) (*ModuleInfo, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check cache first
	l.mu.RLock()
	if moduleInfo, exists := l.moduleCache[absPath]; exists {
		l.mu.RUnlock()
		return moduleInfo, nil
	}
	l.mu.RUnlock()

	// Walk up the directory tree
	currentPath := absPath
	if info, err := os.Stat(currentPath); err == nil && !info.IsDir() {
		currentPath = filepath.Dir(currentPath)
	}

	for {
		goModPath := filepath.Join(currentPath, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Found go.mod file
			moduleInfo := &ModuleInfo{
				Root:      currentPath,
				GoModPath: goModPath,
			}

			// Read module path from go.mod
			if data, err := os.ReadFile(goModPath); err == nil {
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "module ") {
						moduleInfo.Path = strings.TrimSpace(strings.TrimPrefix(line, "module "))
						break
					}
				}
			}

			// Cache the result
			l.mu.Lock()
			l.moduleCache[absPath] = moduleInfo
			l.mu.Unlock()

			return moduleInfo, nil
		}

		// Get parent directory
		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			// Reached root of filesystem
			return nil, fmt.Errorf("go.mod not found")
		}
		currentPath = parent
	}
}

// runTests runs tests for a specific Go file
func (l *GoLinter) runTests(ctx context.Context, testFile string) (string, error) {
	// Find module root
	moduleInfo, err := l.FindModuleRoot(testFile)
	if err != nil {
		// If we can't find a module root, skip running tests
		// This can happen in test scenarios or with isolated files
		return "", nil
	}

	// Calculate relative path from module root
	relPath, err := filepath.Rel(moduleInfo.Root, filepath.Dir(testFile))
	if err != nil {
		// If we can't calculate relative path, skip running tests
		// This can happen when the file is outside the module
		return "", nil
	}

	// Convert to Unix-style path for go test
	testPath := "./" + filepath.ToSlash(relPath)

	var testPattern string

	// Try to extract actual test functions from the file
	testFunctions, err := l.extractTestFunctions(testFile)
	if err == nil && len(testFunctions) > 0 {
		// Successfully extracted test functions
		if len(testFunctions) == 1 {
			// Only one test, run it specifically
			testPattern = fmt.Sprintf("^%s$", testFunctions[0])
		} else {
			// Multiple tests, find common prefix and build optimized pattern
			commonPrefix := findCommonPrefix(testFunctions)
			if len(commonPrefix) > 4 { // More than just "Test"
				// Use the common prefix for more precise matching
				testPattern = fmt.Sprintf("^%s", commonPrefix)
			} else {
				// No meaningful common prefix, build pattern from all test names
				// This creates a pattern like ^(TestFoo|TestBar|TestBaz)$
				testPattern = fmt.Sprintf("^(%s)$", strings.Join(testFunctions, "|"))
			}
		}
	} else {
		// Fallback to filename-based pattern if extraction fails
		testFileName := filepath.Base(testFile)
		// Remove the .go extension to get the base name
		testBaseName := strings.TrimSuffix(testFileName, "_test.go")

		// Capitalize first letter for test pattern
		if len(testBaseName) > 0 {
			testBaseName = strings.ToUpper(testBaseName[:1]) + testBaseName[1:]
		}

		// Build a regex pattern that matches only tests from this specific file
		// For example: if the file is executor_test.go, this will match ^TestExecutor
		// This ensures we only run tests that start with Test<Filename>
		testPattern = fmt.Sprintf("^Test%s", testBaseName)
	}

	// Build test command with timeout
	args := []string{"test", "-v", "-run", testPattern}

	// Add timeout if configured
	if l.config != nil && l.config.TestTimeout != nil {
		args = append(args, "-timeout", l.config.TestTimeout.Duration.String())
	}

	args = append(args, testPath)

	// Run go test with -run flag to only run tests matching the pattern
	// This ensures we only run tests from the specific test file
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleInfo.Root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	if err != nil {
		return output, fmt.Errorf("go test failed: %w", err)
	}

	return output, nil
}

// FormatFile formats a Go file using gofmt
func (l *GoLinter) FormatFile(content []byte) ([]byte, error) {
	return format.Source(content)
}

// LintBatch processes multiple Go files in a single golangci-lint run for better performance
func (l *GoLinter) LintBatch(ctx context.Context, files map[string][]byte) (map[string]*linters.LintResult, error) {
	results := make(map[string]*linters.LintResult)

	// First, check syntax for all files with go/format
	var goFiles []string
	for filePath, content := range files {
		result := &linters.LintResult{
			Success: true,
			Issues:  []linters.Issue{},
		}

		// Skip generated files
		if bytes.Contains(content, []byte("// Code generated")) {
			results[filePath] = result
			continue
		}

		// Skip test data files
		if strings.Contains(filePath, "/testdata/") {
			results[filePath] = result
			continue
		}

		// Check basic syntax first
		formatted, err := format.Source(content)
		if err != nil {
			result.Success = false
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     1,
				Column:   1,
				Severity: "error",
				Message:  fmt.Sprintf("Go syntax error: %v", err),
				Rule:     "syntax",
			})
			results[filePath] = result
			continue
		}

		// Check if formatting is needed
		if !bytes.Equal(content, formatted) {
			result.Formatted = formatted
			// Only add formatting issue if gofmt is not disabled
			if !l.isCheckDisabled("gofmt") {
				result.Issues = append(result.Issues, linters.Issue{
					File:     filePath,
					Line:     1,
					Column:   1,
					Severity: "warning",
					Message:  "File is not properly formatted with gofmt",
					Rule:     "gofmt",
				})
			}
		}

		// Check import restrictions using AST parsing
		importIssues := l.checkImportRestrictions(filePath, content)
		result.Issues = append(result.Issues, importIssues...)

		// Mark as failed if any import restrictions are violated
		for _, issue := range importIssues {
			if issue.Severity == "error" {
				result.Success = false
			}
		}

		results[filePath] = result
		goFiles = append(goFiles, filePath)
	}

	// Run golangci-lint on all valid Go files at once
	if len(goFiles) > 0 {
		if golangciOutput, err := l.runGolangciLintMultiple(ctx, goFiles); err == nil {
			// Map issues back to their files
			for _, issue := range golangciOutput.Issues {
				// Skip disabled checks
				if l.isCheckDisabled(issue.FromLinter) {
					continue
				}

				if result, exists := results[issue.Pos.Filename]; exists {
					severity := "warning"
					if issue.Severity == "error" {
						severity = "error"
						result.Success = false
					}

					result.Issues = append(result.Issues, linters.Issue{
						File:     issue.Pos.Filename,
						Line:     issue.Pos.Line,
						Column:   issue.Pos.Column,
						Severity: severity,
						Message:  issue.Text,
						Rule:     issue.FromLinter,
					})
				}
			}
		}
		// If golangci-lint fails, we continue with basic linting results
	}

	// Run tests for test files
	var wg sync.WaitGroup
	var mu sync.Mutex

	for filePath, content := range files {
		if strings.HasSuffix(filePath, "_test.go") {
			wg.Add(1)
			go func(path string, content []byte) {
				defer wg.Done()

				if output, err := l.runTests(ctx, path); err != nil {
					mu.Lock()
					if result, exists := results[path]; exists {
						result.Success = false
						result.Issues = append(result.Issues, linters.Issue{
							File:     path,
							Line:     1,
							Column:   1,
							Severity: "error",
							Message:  fmt.Sprintf("Tests failed: %v", err),
							Rule:     "test",
						})
						result.TestOutput = output
					}
					mu.Unlock()
				} else {
					mu.Lock()
					if result, exists := results[path]; exists {
						result.TestOutput = output
					}
					mu.Unlock()
				}
			}(filePath, content)
		}
	}

	wg.Wait()
	return results, nil
}
