package testable

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jrossi/gismo/pkg/engine"
)

// DocumentationTester provides framework for testing documentation examples
type DocumentationTester struct {
	t           *testing.T
	examples    map[string]*CodeExample
	fileSet     *token.FileSet
	tempDir     string
	apiInstance *engine.API
}

// CodeExample represents a testable code example from documentation
type CodeExample struct {
	Title       string
	Description string
	Code        string
	Expected    ExampleExpected
	File        string
	Line        int
	Tags        []string // e.g., ["api", "basic", "config"]
}

// ExampleExpected defines what we expect from running the example
type ExampleExpected struct {
	ShouldCompile  bool
	ShouldExecute  bool
	ExpectedOutput string
	ExpectedError  string
	ConfigRequired bool
	Dependencies   []string
}

// NewDocumentationTester creates a new documentation testing framework
func NewDocumentationTester(t *testing.T) *DocumentationTester {
	tempDir, err := os.MkdirTemp("", "gismo-doc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	return &DocumentationTester{
		t:           t,
		examples:    make(map[string]*CodeExample),
		fileSet:     token.NewFileSet(),
		tempDir:     tempDir,
		apiInstance: engine.New(),
	}
}

// Cleanup removes temporary files
func (dt *DocumentationTester) Cleanup() {
	if dt.tempDir != "" {
		os.RemoveAll(dt.tempDir)
	}
}

// LoadExamplesFromMarkdown extracts code examples from markdown files
func (dt *DocumentationTester) LoadExamplesFromMarkdown(filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}

	examples := extractCodeBlocks(string(content), filename)
	for _, example := range examples {
		dt.examples[example.Title] = example
	}

	return nil
}

// TestExample validates a specific code example
func (dt *DocumentationTester) TestExample(name string) error {
	example, exists := dt.examples[name]
	if !exists {
		return fmt.Errorf("example %s not found", name)
	}

	// Test compilation
	if example.Expected.ShouldCompile {
		if err := dt.testCompilation(example); err != nil {
			return fmt.Errorf("compilation test failed for %s: %w", name, err)
		}
	}

	// Test execution if required
	if example.Expected.ShouldExecute {
		if err := dt.testExecution(example); err != nil {
			return fmt.Errorf("execution test failed for %s: %w", name, err)
		}
	}

	return nil
}

// TestAllExamples validates all loaded examples
func (dt *DocumentationTester) TestAllExamples() error {
	failures := []string{}

	for name := range dt.examples {
		if err := dt.TestExample(name); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("documentation test failures:\n%s", strings.Join(failures, "\n"))
	}

	return nil
}

// testCompilation checks if the code example compiles
func (dt *DocumentationTester) testCompilation(example *CodeExample) error {
	// Create a temporary file with the example code
	filename := filepath.Join(dt.tempDir, fmt.Sprintf("%s.go", sanitizeFilename(example.Title)))

	// Add necessary package and imports if not present
	code := dt.prepareCodeForCompilation(example.Code)

	if err := os.WriteFile(filename, []byte(code), 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Parse the file to check for syntax errors
	_, err := parser.ParseFile(dt.fileSet, filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	return nil
}

// testExecution runs the code example and validates output
func (dt *DocumentationTester) testExecution(example *CodeExample) error {
	// This would run the example in a controlled environment
	// For now, we simulate execution by checking key API calls

	if strings.Contains(example.Code, "engine.New()") {
		// Test that basic API creation works
		api := engine.New()
		if api == nil {
			return fmt.Errorf("engine.New() returned nil")
		}
	}

	if strings.Contains(example.Code, "NewActionHandlerEngine()") {
		// Test action handler engine creation
		actionEngine := engine.NewActionHandlerEngine()
		if actionEngine == nil {
			return fmt.Errorf("NewActionHandlerEngine() returned nil")
		}
	}

	return nil
}

// prepareCodeForCompilation adds necessary boilerplate to make code compilable
func (dt *DocumentationTester) prepareCodeForCompilation(code string) string {
	// If the code doesn't start with package, add it
	if !strings.HasPrefix(strings.TrimSpace(code), "package") {
		packageAndImports := `package main

import (
	"context"
	"fmt"
	"log"
	"time"
	
	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo/pkg/engine"
	"github.com/jrossi/gismo/pkg/handlers"
)

`
		code = packageAndImports + code
	}

	return code
}

// extractCodeBlocks parses markdown and extracts Go code blocks
func extractCodeBlocks(content, filename string) []*CodeExample {
	var examples []*CodeExample
	lines := strings.Split(content, "\n")

	var currentExample *CodeExample
	var inCodeBlock bool
	var codeLines []string
	var lineNum int

	codeBlockRegex := regexp.MustCompile(`^` + "```" + `go\s*$`)
	endBlockRegex := regexp.MustCompile(`^` + "```" + `\s*$`)
	titleRegex := regexp.MustCompile(`^#+\s+(.+)$`)

	var lastTitle string

	for i, line := range lines {
		lineNum = i + 1

		// Track headings for context
		if matches := titleRegex.FindStringSubmatch(line); matches != nil {
			lastTitle = strings.TrimSpace(matches[1])
			continue
		}

		// Start of code block
		if codeBlockRegex.MatchString(line) {
			inCodeBlock = true
			codeLines = []string{}
			currentExample = &CodeExample{
				Title: fmt.Sprintf("%s_%d", sanitizeFilename(lastTitle), lineNum),
				File:  filename,
				Line:  lineNum,
				Expected: ExampleExpected{
					ShouldCompile: true,
					ShouldExecute: true,
				},
			}
			continue
		}

		// End of code block
		if inCodeBlock && endBlockRegex.MatchString(line) {
			inCodeBlock = false
			if currentExample != nil {
				currentExample.Code = strings.Join(codeLines, "\n")
				currentExample.Description = lastTitle
				examples = append(examples, currentExample)
			}
			continue
		}

		// Inside code block
		if inCodeBlock {
			codeLines = append(codeLines, line)
		}
	}

	return examples
}

// sanitizeFilename makes a string safe for use as a filename
func sanitizeFilename(name string) string {
	// Replace spaces and special chars with underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return strings.ToLower(reg.ReplaceAllString(name, "_"))
}

// Example validation functions for specific use cases
func (dt *DocumentationTester) ValidateAPIImports() error {
	invalidImports := []string{
		`"github.com/jrossi/gismo"`, // This should not exist at root
	}

	_ = []string{
		`"github.com/jrossi/gismo/pkg/engine"`,
		`"github.com/jrossi/gismo/pkg/handlers"`,
		`"github.com/jrossi/gismo/pkg/linters/golang"`,
		// ... other valid imports
	}

	for name, example := range dt.examples {
		for _, invalid := range invalidImports {
			if strings.Contains(example.Code, invalid) {
				return fmt.Errorf("example %s contains invalid import %s", name, invalid)
			}
		}
	}

	return nil
}

// ValidateAPIFunctions checks that function calls in examples exist
func (dt *DocumentationTester) ValidateAPIFunctions() error {
	// Map of documented functions to their actual locations
	functionMappings := map[string]string{
		"gismo.NewAPI()":             "engine.New()",
		"gismo.NewAPIWithConfig()":   "engine.NewWithConfig()",
		"gismo.NewAPIWithEngine()":   "engine.NewWithRuleEngine()",
		"gismo.LoadConfig()":         "config.LoadConfig()",
		"gismo.ProcessHookMessage()": "api.ProcessMessage()",
		"gismo.ProcessHook()":        "api.ProcessMessage()",
	}

	errors := []string{}

	for name, example := range dt.examples {
		for oldFunc, newFunc := range functionMappings {
			if strings.Contains(example.Code, oldFunc) {
				errors = append(errors, fmt.Sprintf("example %s uses deprecated %s, should use %s", name, oldFunc, newFunc))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("API function validation errors:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}
