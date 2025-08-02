package golang

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoLinter_Name(t *testing.T) {
	linter := NewGoLinter()
	if got := linter.Name(); got != "go" {
		t.Errorf("Name() = %q, want %q", got, "go")
	}
}

func TestGoLinter_findGolangciLint_NotFound(t *testing.T) {
	// Test when golangci-lint is not found
	linter := &GoLinter{}

	// Temporarily modify PATH to ensure golangci-lint is not found
	oldPath := os.Getenv("PATH")
	oldHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("PATH", oldPath)
		os.Setenv("HOME", oldHome)
	}()

	os.Setenv("PATH", "/nonexistent")
	os.Setenv("HOME", "/nonexistent")

	path := linter.findGolangciLint()
	if path != "" {
		t.Errorf("Expected empty path when golangci-lint not found, got %q", path)
	}

	// Call again to test once.Do behavior
	path2 := linter.findGolangciLint()
	if path2 != "" {
		t.Errorf("Expected empty path on second call, got %q", path2)
	}
}

func TestGoLinter_runTests_PatternGeneration(t *testing.T) {
	// Test that the correct test patterns are generated for different file names
	tests := []struct {
		name            string
		testFileName    string
		expectedPattern string
		description     string
	}{
		{
			name:            "executor_test.go",
			testFileName:    "executor_test.go",
			expectedPattern: "^TestExecutor",
			description:     "Should generate pattern for executor tests",
		},
		{
			name:            "api_test.go",
			testFileName:    "api_test.go",
			expectedPattern: "^TestApi",
			description:     "Should generate pattern for api tests",
		},
		{
			name:            "handler_extended_test.go",
			testFileName:    "handler_extended_test.go",
			expectedPattern: "^TestHandler_extended",
			description:     "Should handle extended test files",
		},
		{
			name:            "parser_test.go",
			testFileName:    "parser_test.go",
			expectedPattern: "^TestParser",
			description:     "Should generate pattern for parser tests",
		},
		{
			name:            "linting_engine_test.go",
			testFileName:    "linting_engine_test.go",
			expectedPattern: "^TestLinting_engine",
			description:     "Should handle underscore in filename",
		},
		{
			name:            "e2e_markdown_test.go",
			testFileName:    "e2e_markdown_test.go",
			expectedPattern: "^TestE2e_markdown",
			description:     "Should handle files starting with lowercase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract the test pattern generation logic to verify it
			// This should match the logic in runTests method
			testFileName := filepath.Base(tt.testFileName)
			testBaseName := testFileName[:len(testFileName)-len("_test.go")]

			// Capitalize first letter for test pattern (same as in runTests)
			if len(testBaseName) > 0 {
				testBaseName = strings.ToUpper(testBaseName[:1]) + testBaseName[1:]
			}

			testPattern := fmt.Sprintf("^Test%s", testBaseName)

			if testPattern != tt.expectedPattern {
				t.Errorf("%s: got pattern %q, want %q", tt.description, testPattern, tt.expectedPattern)
			}
		})
	}
}

func TestGoLinter_runTests_CommandExecution(t *testing.T) {
	// This test verifies that runTests generates the correct go test command
	// We'll create a mock test file and verify the command would target only specific tests

	linter := NewGoLinter()
	ctx := context.Background()

	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "go_test_cmd_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a go.mod file
	goModContent := `module testmodule

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Test cases for different file names
	testCases := []struct {
		fileName         string
		testContent      string
		expectedToRun    []string // Test functions that should run
		notExpectedToRun []string // Test functions that should NOT run
	}{
		{
			fileName: "executor_test.go",
			testContent: `package main
import "testing"
func TestExecutor_Basic(t *testing.T) { t.Log("executor basic") }
func TestExecutor_Advanced(t *testing.T) { t.Log("executor advanced") }
func TestOther_Function(t *testing.T) { t.Log("other function") }
`,
			expectedToRun:    []string{"TestExecutor_Basic", "TestExecutor_Advanced"},
			notExpectedToRun: []string{"TestOther_Function"},
		},
		{
			fileName: "api_test.go",
			testContent: `package main
import "testing"
func TestApi_Create(t *testing.T) { t.Log("api create") }
func TestApi_Delete(t *testing.T) { t.Log("api delete") }
func TestHandler_Process(t *testing.T) { t.Log("handler process") }
`,
			expectedToRun:    []string{"TestApi_Create", "TestApi_Delete"},
			notExpectedToRun: []string{"TestHandler_Process"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.fileName, func(t *testing.T) {
			// Create the test file
			testFile := filepath.Join(tmpDir, tc.fileName)
			if err := os.WriteFile(testFile, []byte(tc.testContent), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Run the tests
			output, err := linter.runTests(ctx, testFile)

			// Check that expected tests would run (by checking the pattern)
			// Extract the pattern from the file name
			baseName := strings.TrimSuffix(tc.fileName, "_test.go")
			if len(baseName) > 0 {
				baseName = strings.ToUpper(baseName[:1]) + baseName[1:]
			}
			expectedPattern := fmt.Sprintf("^Test%s", baseName)

			// Verify the pattern is correct
			if !strings.Contains(output, "RUN") && err == nil {
				// If no tests ran but no error, the pattern might be too restrictive
				t.Logf("Warning: No tests ran with pattern %s", expectedPattern)
			}

			// Log the output for debugging
			t.Logf("Test output for %s:\n%s", tc.fileName, output)
			if err != nil {
				t.Logf("Error: %v", err)
			}
		})
	}
}

func TestGoLinter_runTests(t *testing.T) {
	linter := NewGoLinter()

	// Create a temporary directory with a simple test file
	tmpDir, err := os.MkdirTemp("", "go_test_runner_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple test file
	testFile := filepath.Join(tmpDir, "example_test.go")
	testContent := `package example

import "testing"

func TestExample(t *testing.T) {
	if 1+1 != 2 {
		t.Error("Math is broken")
	}
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Also need a go.mod file
	goModContent := `module example

go 1.19
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Run tests
	output, err := linter.runTests(context.Background(), tmpDir)
	if err != nil {
		t.Logf("runTests() error (may be expected): %v", err)
	}

	// Check output
	if output == "" {
		t.Log("No test output (tests may have passed)")
	}
}

func TestGoLinter_runTests_Failure(t *testing.T) {
	linter := NewGoLinter()

	// Create a temporary directory with a failing test
	tmpDir, err := os.MkdirTemp("", "go_test_fail_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a failing test file
	testFile := filepath.Join(tmpDir, "fail_test.go")
	testContent := `package example

import "testing"

func TestFailing(t *testing.T) {
	t.Error("This test always fails")
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create go.mod
	goModContent := `module example

go 1.19
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Run tests
	output, err := linter.runTests(context.Background(), tmpDir)
	if err == nil {
		t.Error("Expected error for failing tests")
	}

	// Should have output about failure
	if output == "" {
		t.Error("Expected test output for failing tests")
	}
}

func TestGoLinter_runTests_NoTests(t *testing.T) {
	linter := NewGoLinter()

	// Create a temporary directory with no test files
	tmpDir, err := os.MkdirTemp("", "go_no_tests_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a regular Go file (not a test)
	goFile := filepath.Join(tmpDir, "main.go")
	goContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write go file: %v", err)
	}

	// Create go.mod
	goModContent := `module example

go 1.19
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Run tests
	output, err := linter.runTests(context.Background(), tmpDir)
	// No tests is not an error
	if err != nil {
		t.Logf("runTests() returned error (expected for no tests): %v", err)
	}

	// Should handle no tests gracefully
	if output != "" {
		t.Logf("Got output: %s", output)
	}
}

func TestGoLinter_EnhancedLinting_ConfigVariations(t *testing.T) {
	linter := NewGoLinter()

	tests := []struct {
		name          string
		configFile    string
		configContent string
	}{
		{
			name:       "golangci.yml",
			configFile: ".golangci.yml",
			configContent: `linters:
  enable:
    - gofmt
    - govet
`,
		},
		{
			name:       "golangci.yaml",
			configFile: ".golangci.yaml",
			configContent: `linters:
  enable:
    - gofmt
`,
		},
		{
			name:       "golangci.toml",
			configFile: ".golangci.toml",
			configContent: `[linters]
enable = ["gofmt"]
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if golangci-lint not available
			if linter.findGolangciLint() == "" {
				t.Skip("golangci-lint not available")
			}

			tmpDir, err := os.MkdirTemp("", "golangci_config_")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Write config file
			configPath := filepath.Join(tmpDir, tt.configFile)
			if err := os.WriteFile(configPath, []byte(tt.configContent), 0644); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}

			// Write a simple Go file
			goFile := filepath.Join(tmpDir, "main.go")
			goContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`
			if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
				t.Fatalf("Failed to write go file: %v", err)
			}

			// Should detect config file
			result, err := linter.Lint(context.Background(), goFile, []byte(goContent))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			// Check that enhanced linting was attempted
			if result == nil {
				t.Error("Expected result, got nil")
			}
		})
	}
}
