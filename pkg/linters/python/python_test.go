package python

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/goccy/go-json"

	linters2 "github.com/jrossi/gismo/pkg/linters"
)

func TestPythonLinter_CanHandle(t *testing.T) {
	linter := NewPythonLinter()

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"Python file", "test.py", true},
		{"Python file with path", "/path/to/script.py", true},
		{"Python test file", "test_something.py", true},
		{"Python test file alt", "something_test.py", true},
		{"Go file", "main.go", false},
		{"JavaScript file", "script.js", false},
		{"Text file", "readme.txt", false},
		{"No extension", "Makefile", false},
		{"Hidden Python file", ".hidden.py", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linter.CanHandle(tt.filePath)
			if got != tt.want {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestPythonLinter_Name(t *testing.T) {
	linter := NewPythonLinter()
	if got := linter.Name(); got != "python" {
		t.Errorf("Name() = %v, want %v", got, "python")
	}
}

func TestPythonLinter_SetConfig(t *testing.T) {
	linter := NewPythonLinter()

	config := PythonConfig{
		RuffArgs:      []string{"--line-length", "120"},
		MaxLineLength: intPtr(120),
		TypeChecker:   "pyright",
		TestRunner:    "unittest",
		RunTests:      false,
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = linter.SetConfig(configJSON)
	if err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	if len(linter.config.RuffArgs) != len(config.RuffArgs) {
		t.Errorf("RuffArgs length = %v, want %v", len(linter.config.RuffArgs), len(config.RuffArgs))
	}
	if *linter.config.MaxLineLength != *config.MaxLineLength {
		t.Errorf("MaxLineLength = %v, want %v", *linter.config.MaxLineLength, *config.MaxLineLength)
	}
	if linter.config.TypeChecker != config.TypeChecker {
		t.Errorf("TypeChecker = %v, want %v", linter.config.TypeChecker, config.TypeChecker)
	}
	if linter.config.TestRunner != config.TestRunner {
		t.Errorf("TestRunner = %v, want %v", linter.config.TestRunner, config.TestRunner)
	}
	if linter.config.RunTests != config.RunTests {
		t.Errorf("RunTests = %v, want %v", linter.config.RunTests, config.RunTests)
	}
}

func TestPythonLinter_Lint_ValidFile(t *testing.T) {
	linter := NewPythonLinter()
	ctx := context.Background()

	// Read valid test file
	content, err := os.ReadFile("testdata/valid.py")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	result, err := linter.Lint(ctx, "testdata/valid.py", content)
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}
	if result == nil {
		t.Fatal("Lint returned nil result")
	}

	// Should have no issues for valid file
	if !result.Success {
		t.Errorf("Success = %v, want %v", result.Success, true)
	}
	if len(result.Issues) > 0 {
		t.Errorf("Issues = %v, want empty", result.Issues)
	}
}

func TestPythonLinter_Lint_SyntaxError(t *testing.T) {
	linter := NewPythonLinter()
	ctx := context.Background()

	// Read file with syntax errors
	content, err := os.ReadFile("testdata/syntax_error.py")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	result, err := linter.Lint(ctx, "testdata/syntax_error.py", content)
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}
	if result == nil {
		t.Fatal("Lint returned nil result")
	}

	// Should detect syntax errors
	if result.Success {
		t.Errorf("Success = %v, want %v", result.Success, false)
	}
	if len(result.Issues) == 0 {
		t.Error("Expected syntax errors but found none")
	}

	// Check for syntax error
	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "syntax" && issue.Severity == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Should find syntax error")
	}
}

func TestPythonLinter_Lint_LintingIssues(t *testing.T) {
	linter := NewPythonLinter()
	ctx := context.Background()

	// Skip if UV is not available
	linter.initialize()
	if !linter.hasUV {
		t.Skip("UV not installed, skipping linting test")
	}

	// Read file with linting issues
	content, err := os.ReadFile("testdata/lint_errors.py")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	result, err := linter.Lint(ctx, "testdata/lint_errors.py", content)
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}
	if result == nil {
		t.Fatal("Lint returned nil result")
	}

	// Should have warnings but might still be successful
	if len(result.Issues) == 0 {
		t.Error("Expected linting issues but found none")
	}

	// Check for various linting issues
	issueTypes := make(map[string]bool)
	for _, issue := range result.Issues {
		issueTypes[issue.Rule] = true
	}

	// We expect various linting rules to be triggered
	t.Logf("Found issue types: %v", issueTypes)
}

func TestPythonLinter_Lint_TestFile(t *testing.T) {
	// Create a linter with test running enabled
	config := DefaultPythonConfig()
	config.RunTests = true
	linter := NewPythonLinterWithConfig(config)

	ctx := context.Background()

	// Read test file
	content, err := os.ReadFile("testdata/test_example.py")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Temporarily skip if UV not available
	linter.initialize()
	if !linter.hasUV {
		t.Skip("UV not installed, skipping test execution")
	}

	result, err := linter.Lint(ctx, "testdata/test_example.py", content)
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}
	if result == nil {
		t.Fatal("Lint returned nil result")
	}

	// Should have test output
	if result.TestOutput == "" {
		t.Error("Expected test output but found none")
	}
}

func TestPythonLinter_LintBatch(t *testing.T) {
	linter := NewPythonLinter()
	ctx := context.Background()

	// Prepare batch of files
	files := make(map[string][]byte)

	// Add valid file
	validContent, err := os.ReadFile("testdata/valid.py")
	if err != nil {
		t.Fatalf("Failed to read valid.py: %v", err)
	}
	files["testdata/valid.py"] = validContent

	// Add file with syntax error
	syntaxContent, err := os.ReadFile("testdata/syntax_error.py")
	if err != nil {
		t.Fatalf("Failed to read syntax_error.py: %v", err)
	}
	files["testdata/syntax_error.py"] = syntaxContent

	// Add non-Python file (should be ignored)
	files["testdata/readme.txt"] = []byte("This is not a Python file")

	// Run batch linting
	results, err := linter.LintBatch(ctx, files)
	if err != nil {
		t.Fatalf("LintBatch failed: %v", err)
	}
	if results == nil {
		t.Fatal("LintBatch returned nil results")
	}

	// Should have results for Python files only
	if len(results) != 2 {
		t.Errorf("Results length = %v, want %v", len(results), 2)
	}

	// Check valid file result
	validResult, ok := results["testdata/valid.py"]
	if !ok {
		t.Error("Missing result for valid.py")
	} else {
		if !validResult.Success {
			t.Error("Valid file should have Success = true")
		}
		if len(validResult.Issues) > 0 {
			t.Errorf("Valid file should have no issues, got %v", validResult.Issues)
		}
	}

	// Check syntax error file result
	syntaxResult, ok := results["testdata/syntax_error.py"]
	if !ok {
		t.Error("Missing result for syntax_error.py")
	} else {
		if syntaxResult.Success {
			t.Error("Syntax error file should have Success = false")
		}
		if len(syntaxResult.Issues) == 0 {
			t.Error("Syntax error file should have issues")
		}
	}
}

func TestPythonLinter_IsTestFile(t *testing.T) {
	linter := NewPythonLinter()

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"Test prefix", "test_something.py", true},
		{"Test suffix", "something_test.py", true},
		{"Test in path", "/path/to/test_file.py", true},
		{"Regular file", "main.py", false},
		{"Test in middle", "my_test_file.py", false},
		{"No extension", "test_file", false},
		{"Test directory", "tests/conftest.py", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linter.isTestFile(tt.filePath)
			if got != tt.want {
				t.Errorf("isTestFile(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestPythonLinter_ContextCancellation(t *testing.T) {
	linter := NewPythonLinter()

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	content := []byte("print('Hello, world!')")

	// The syntax check should still work with canceled context
	// as it's a quick operation
	result, err := linter.Lint(ctx, "test.py", content)
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}
	if result == nil {
		t.Fatal("Lint returned nil result")
	}
}

func TestPythonLinter_EmptyFile(t *testing.T) {
	linter := NewPythonLinter()
	ctx := context.Background()

	result, err := linter.Lint(ctx, "empty.py", []byte(""))
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}
	if result == nil {
		t.Fatal("Lint returned nil result")
	}
	if !result.Success {
		t.Errorf("Success = %v, want %v", result.Success, true)
	}
	if len(result.Issues) > 0 {
		t.Errorf("Issues = %v, want empty", result.Issues)
	}
}

func TestPythonLinter_BatchingLinterInterface(t *testing.T) {
	// Test that PythonLinter implements BatchingLinter interface
	var _ linters2.BatchingLinter = (*PythonLinter)(nil)
}

func TestDefaultPythonConfig(t *testing.T) {
	config := DefaultPythonConfig()

	if config == nil {
		t.Fatal("DefaultPythonConfig returned nil")
	}
	if config.MaxLineLength == nil {
		t.Error("MaxLineLength is nil")
	} else if *config.MaxLineLength != 88 {
		t.Errorf("MaxLineLength = %v, want %v", *config.MaxLineLength, 88)
	}
	if config.TypeChecker != "mypy" {
		t.Errorf("TypeChecker = %v, want %v", config.TypeChecker, "mypy")
	}
	if config.TestRunner != "pytest" {
		t.Errorf("TestRunner = %v, want %v", config.TestRunner, "pytest")
	}
	if !config.RunTests {
		t.Error("RunTests should be true by default")
	}
	if config.TestTimeout == nil {
		t.Error("TestTimeout is nil")
	} else if config.TestTimeout.Duration != 2*time.Minute {
		t.Errorf("TestTimeout = %v, want %v", config.TestTimeout.Duration, 2*time.Minute)
	}
}

func TestPythonLinter_SyntaxCheck(t *testing.T) {
	linter := NewPythonLinter()
	ctx := context.Background()

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "valid syntax",
			content: "print('hello')",
			wantErr: false,
		},
		{
			name:    "invalid syntax - missing colon",
			content: "if True\n    print('oops')",
			wantErr: true,
		},
		{
			name:    "invalid syntax - bad indentation",
			content: "def foo():\nprint('bad')",
			wantErr: true,
		},
		{
			name:    "empty file",
			content: "",
			wantErr: false,
		},
		{
			name: "complex valid syntax",
			content: `
def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)

class Example:
    def __init__(self):
        self.value = 42
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := linter.checkSyntax(ctx, "test.py", []byte(tt.content))
			if (err != nil) != tt.wantErr {
				t.Errorf("checkSyntax() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test helper to verify interfaces at compile time
func TestInterfaceCompliance(t *testing.T) {
	// These will fail to compile if the interfaces aren't properly implemented
	var _ linters2.Linter = (*PythonLinter)(nil)
	var _ linters2.BatchingLinter = (*PythonLinter)(nil)
}

// Test Duration JSON marshaling/unmarshaling
func TestDuration_JSON(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		json     string
	}{
		{"1 minute", time.Minute, `"1m0s"`},
		{"30 seconds", 30 * time.Second, `"30s"`},
		{"2 hours", 2 * time.Hour, `"2h0m0s"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Duration{Duration: tt.duration}

			// Test marshaling
			data, err := json.Marshal(d)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if string(data) != tt.json {
				t.Errorf("Marshal = %v, want %v", string(data), tt.json)
			}

			// Test unmarshaling
			var d2 Duration
			err = json.Unmarshal(data, &d2)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if d2.Duration != tt.duration {
				t.Errorf("Unmarshal = %v, want %v", d2.Duration, tt.duration)
			}
		})
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
