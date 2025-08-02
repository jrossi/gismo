package python

import (
	"context"
	"os"
	"testing"

	linters2 "github.com/jrossi/gismo/pkg/linters"
)

func TestPythonLinter_BatchExecution(t *testing.T) {
	// Create batch executor
	executor := linters2.NewBatchExecutor(2)
	pythonLinter := NewPythonLinter()

	ctx := context.Background()

	// Prepare test files
	files := make(map[string][]byte)

	// Add valid Python file
	validContent, err := os.ReadFile("testdata/valid.py")
	if err != nil {
		t.Fatalf("Failed to read valid.py: %v", err)
	}
	files["testdata/valid.py"] = validContent

	// Add Python file with syntax errors
	syntaxContent, err := os.ReadFile("testdata/syntax_error.py")
	if err != nil {
		t.Fatalf("Failed to read syntax_error.py: %v", err)
	}
	files["testdata/syntax_error.py"] = syntaxContent

	// Add Python file with lint errors
	lintContent, err := os.ReadFile("testdata/lint_errors.py")
	if err != nil {
		t.Fatalf("Failed to read lint_errors.py: %v", err)
	}
	files["testdata/lint_errors.py"] = lintContent

	// Add a non-Python file that should be ignored
	files["testdata/readme.txt"] = []byte("Not a Python file")

	// Execute batch linting
	results := executor.ExecuteLintersBatched(ctx, []linters2.Linter{pythonLinter}, files)

	// Verify we got results for Python files
	if len(results) != 3 {
		t.Errorf("Expected results for 3 Python files, got %d", len(results))
	}

	// Check valid.py results
	validResults, ok := results["testdata/valid.py"]
	if !ok {
		t.Error("Missing results for valid.py")
	} else if len(validResults) != 1 {
		t.Errorf("Expected 1 result for valid.py, got %d", len(validResults))
	} else if validResults[0].LinterName != "python" {
		t.Errorf("Expected linter name 'python', got '%s'", validResults[0].LinterName)
	} else if !validResults[0].Result.Success {
		t.Error("Expected valid.py to pass linting")
	}

	// Check syntax_error.py results
	syntaxResults, ok := results["testdata/syntax_error.py"]
	if !ok {
		t.Error("Missing results for syntax_error.py")
	} else if len(syntaxResults) != 1 {
		t.Errorf("Expected 1 result for syntax_error.py, got %d", len(syntaxResults))
	} else if syntaxResults[0].Result.Success {
		t.Error("Expected syntax_error.py to fail linting")
	}

	// Verify readme.txt was not processed
	if _, ok := results["testdata/readme.txt"]; ok {
		t.Error("Non-Python file should not have been processed")
	}
}

func TestPythonLinter_MixedBatchExecution(t *testing.T) {
	// Test Python linter alongside other linters
	executor := linters2.NewBatchExecutor(4)

	// Create a mock linter to test alongside Python linter
	mockLinter := &MockOtherLinter{}
	pythonLinter := NewPythonLinter()

	ctx := context.Background()

	// Prepare mixed files
	files := make(map[string][]byte)
	files["test.py"] = []byte("print('Hello, Python!')")
	files["test.txt"] = []byte("Hello, Text!")
	files["main.py"] = []byte("def main():\n    pass")

	// Execute batch linting with multiple linters
	results := executor.ExecuteLintersBatched(ctx, []linters2.Linter{pythonLinter, mockLinter}, files)

	// Should have results for both Python files and text file
	if len(results) != 3 {
		t.Errorf("Expected results for 3 files, got %d", len(results))
	}

	// Check Python files were handled by Python linter
	for _, pyFile := range []string{"test.py", "main.py"} {
		fileResults, ok := results[pyFile]
		if !ok {
			t.Errorf("Missing results for %s", pyFile)
			continue
		}

		pythonHandled := false
		for _, result := range fileResults {
			if result.LinterName == "python" {
				pythonHandled = true
				break
			}
		}
		if !pythonHandled {
			t.Errorf("Python file %s was not handled by Python linter", pyFile)
		}
	}

	// Check text file was handled by mock linter
	txtResults, ok := results["test.txt"]
	if !ok {
		t.Error("Missing results for test.txt")
	} else {
		mockHandled := false
		for _, result := range txtResults {
			if result.LinterName == "mock" {
				mockHandled = true
				break
			}
		}
		if !mockHandled {
			t.Error("Text file was not handled by mock linter")
		}
	}
}

// MockOtherLinter is a simple linter for testing mixed execution
type MockOtherLinter struct{}

func (m *MockOtherLinter) Name() string {
	return "mock"
}

func (m *MockOtherLinter) CanHandle(filePath string) bool {
	return len(filePath) > 4 && filePath[len(filePath)-4:] == ".txt"
}

func (m *MockOtherLinter) Lint(ctx context.Context, filePath string, content []byte) (*linters2.LintResult, error) {
	return &linters2.LintResult{
		Success: true,
		Issues:  []linters2.Issue{},
	}, nil
}

func BenchmarkPythonLinter_Batch(b *testing.B) {
	linter := NewPythonLinter()
	ctx := context.Background()

	// Prepare test data
	validContent, _ := os.ReadFile("testdata/valid.py")
	files := make(map[string][]byte)
	for i := 0; i < 10; i++ {
		files[string(rune('a'+i))+".py"] = validContent
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.LintBatch(ctx, files)
		if err != nil {
			b.Fatalf("LintBatch failed: %v", err)
		}
	}
}

func BenchmarkPythonLinter_Sequential(b *testing.B) {
	linter := NewPythonLinter()
	ctx := context.Background()

	// Prepare test data
	validContent, _ := os.ReadFile("testdata/valid.py")
	files := make(map[string][]byte)
	for i := 0; i < 10; i++ {
		files[string(rune('a'+i))+".py"] = validContent
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for path, content := range files {
			_, err := linter.Lint(ctx, path, content)
			if err != nil {
				b.Fatalf("Lint failed: %v", err)
			}
		}
	}
}
