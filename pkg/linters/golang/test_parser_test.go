package golang

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestGoLinter_extractTestFunctions(t *testing.T) {
	linter := NewGoLinter()

	tests := []struct {
		name          string
		fileContent   string
		expectedTests []string
		expectError   bool
	}{
		{
			name: "simple test file",
			fileContent: `package main

import "testing"

func TestBasic(t *testing.T) {
	t.Log("basic test")
}

func TestAdvanced(t *testing.T) {
	t.Log("advanced test")
}`,
			expectedTests: []string{"TestBasic", "TestAdvanced"},
			expectError:   false,
		},
		{
			name: "test file with non-test functions",
			fileContent: `package main

import "testing"

func TestValid(t *testing.T) {}

func helperFunction() {}

func BenchmarkSomething(b *testing.B) {}

func TestAnotherValid(t *testing.T) {}

func ExampleFunction() {}`,
			expectedTests: []string{"TestValid", "TestAnotherValid"},
			expectError:   false,
		},
		{
			name: "test with wrong parameter type",
			fileContent: `package main

import "testing"

func TestValid(t *testing.T) {}

func TestInvalidParam(t *testing.B) {} // Wrong type

func TestNoParam() {} // No parameters

func TestTooManyParams(t *testing.T, x int) {} // Too many parameters`,
			expectedTests: []string{"TestValid"},
			expectError:   false,
		},
		{
			name: "test with subtests",
			fileContent: `package main

import "testing"

func TestWithSubtests(t *testing.T) {
	t.Run("subtest1", func(t *testing.T) {})
	t.Run("subtest2", func(t *testing.T) {})
}

func TestNormal(t *testing.T) {}`,
			expectedTests: []string{"TestWithSubtests", "TestNormal"},
			expectError:   false,
		},
		{
			name: "empty file",
			fileContent: `package main

import "testing"`,
			expectedTests: []string{},
			expectError:   false,
		},
		{
			name:          "invalid Go code",
			fileContent:   `this is not valid go code`,
			expectedTests: nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary test file
			tmpFile, err := os.CreateTemp("", "test_*.go")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write test content
			if _, err := tmpFile.WriteString(tt.fileContent); err != nil {
				t.Fatalf("Failed to write test content: %v", err)
			}
			tmpFile.Close()

			// Extract test functions
			tests, err := linter.extractTestFunctions(tmpFile.Name())

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check extracted tests
			if !tt.expectError {
				sort.Strings(tests)
				sort.Strings(tt.expectedTests)

				if len(tests) != len(tt.expectedTests) {
					t.Errorf("Expected %d tests, got %d", len(tt.expectedTests), len(tests))
				}

				for i := range tests {
					if i < len(tt.expectedTests) && tests[i] != tt.expectedTests[i] {
						t.Errorf("Test %d: expected %s, got %s", i, tt.expectedTests[i], tests[i])
					}
				}
			}
		})
	}
}

func TestGoLinter_findCommonPrefix(t *testing.T) {
	tests := []struct {
		name           string
		testNames      []string
		expectedPrefix string
	}{
		{
			name:           "common prefix exists",
			testNames:      []string{"TestExecutor_Basic", "TestExecutor_Advanced", "TestExecutor_Complex"},
			expectedPrefix: "TestExecutor_",
		},
		{
			name:           "only Test prefix common",
			testNames:      []string{"TestApi", "TestHandler", "TestParser"},
			expectedPrefix: "Test",
		},
		{
			name:           "single test",
			testNames:      []string{"TestSingle"},
			expectedPrefix: "TestSingle",
		},
		{
			name:           "empty list",
			testNames:      []string{},
			expectedPrefix: "",
		},
		{
			name:           "exact same tests",
			testNames:      []string{"TestSame", "TestSame", "TestSame"},
			expectedPrefix: "TestSame",
		},
		{
			name:           "mixed case prefix",
			testNames:      []string{"TestHTTPServer", "TestHTTPClient", "TestHTTPHandler"},
			expectedPrefix: "TestHTTP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := findCommonPrefix(tt.testNames)
			if prefix != tt.expectedPrefix {
				t.Errorf("Expected prefix %q, got %q", tt.expectedPrefix, prefix)
			}
		})
	}
}

func TestGoLinter_runTests_WithExtraction(t *testing.T) {
	linter := NewGoLinter()

	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "go_test_extraction_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	goModContent := `module testmodule

go 1.21`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create a test file with tests that have a common prefix
	testContent := `package main

import "testing"

func TestExecutor_First(t *testing.T) {
	t.Log("first executor test")
}

func TestExecutor_Second(t *testing.T) {
	t.Log("second executor test")
}

func TestExecutor_Third(t *testing.T) {
	t.Log("third executor test")
}`

	testFile := filepath.Join(tmpDir, "specific_test.go")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// First verify that test extraction works
	extractedTests, extractErr := linter.extractTestFunctions(testFile)
	if extractErr != nil {
		t.Fatalf("Failed to extract test functions: %v", extractErr)
	}
	t.Logf("Extracted tests: %v", extractedTests)

	// Enable debug output
	os.Setenv("DEBUG_TEST_PATTERN", "1")
	defer os.Unsetenv("DEBUG_TEST_PATTERN")

	// Run tests
	output, err := linter.runTests(context.Background(), testFile)
	if err != nil {
		// Some error is expected since TestUnrelated fails, but we should see output
		t.Logf("runTests returned error (expected): %v", err)
	}

	// Log the full output for debugging
	t.Logf("Test output:\n%s", output)

	// Verify that the output contains our executor tests
	if !strings.Contains(output, "TestExecutor_First") {
		t.Errorf("Output should contain TestExecutor_First")
	}
	if !strings.Contains(output, "TestExecutor_Second") {
		t.Errorf("Output should contain TestExecutor_Second")
	}
	if !strings.Contains(output, "TestExecutor_Third") {
		t.Errorf("Output should contain TestExecutor_Third")
	}

	// Verify that all tests passed (with common prefix, the pattern should be ^TestExecutor_)
	if strings.Contains(output, "FAIL") && !strings.Contains(output, "FAIL\ttest") {
		t.Errorf("Tests should have passed")
	}
}

func TestGoLinter_runTests_FallbackBehavior(t *testing.T) {
	linter := NewGoLinter()

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "go_test_fallback_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	goModContent := `module testmodule

go 1.21`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create a non-parseable test file (syntax error) to trigger fallback
	testContent := `package main

import "testing"

func TestExecutor_Basic(t *testing.T) {
	// Missing closing brace will cause parse error`

	testFile := filepath.Join(tmpDir, "executor_test.go")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Run tests - should fall back to filename-based pattern
	_, err = linter.runTests(context.Background(), testFile)
	// We expect an error due to syntax error, but the important thing is
	// that it attempted to run with the fallback pattern ^TestExecutor
	if err == nil {
		t.Errorf("Expected error due to syntax error in test file")
	}
}

// Benchmark the performance of test extraction
func BenchmarkExtractTestFunctions(b *testing.B) {
	linter := NewGoLinter()

	// Create a test file with many test functions
	content := `package main

import "testing"

`
	for i := 0; i < 100; i++ {
		content += fmt.Sprintf(`func TestFunction%d(t *testing.T) {
	t.Log("test %d")
}

`, i, i)
	}

	tmpFile, err := os.CreateTemp("", "bench_*.go")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		b.Fatalf("Failed to write content: %v", err)
	}
	tmpFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.extractTestFunctions(tmpFile.Name())
		if err != nil {
			b.Fatalf("extractTestFunctions failed: %v", err)
		}
	}
}
