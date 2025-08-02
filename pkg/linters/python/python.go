package python

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jrossi/gismo/pkg/linters"
)

// PythonLinter handles linting of Python files using UV/UVX
type PythonLinter struct {
	config   *PythonConfig
	hasUV    bool
	uvPath   string
	initOnce sync.Once
}

// RuffIssue represents a single issue from ruff's JSON output
type RuffIssue struct {
	Code     string    `json:"code"`
	Message  string    `json:"message"`
	Location *Location `json:"location"`
	End      *Location `json:"end_location"`
}

// Location represents a position in the file
type Location struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

// RuffOutput represents the JSON output from ruff
type RuffOutput struct {
	Issues []RuffIssue `json:"issues"`
}

// NewPythonLinter creates a new Python linter with default configuration
func NewPythonLinter() *PythonLinter {
	return &PythonLinter{
		config: DefaultPythonConfig(),
	}
}

// NewPythonLinterWithConfig creates a new Python linter with custom configuration
func NewPythonLinterWithConfig(config *PythonConfig) *PythonLinter {
	if config == nil {
		config = DefaultPythonConfig()
	}
	return &PythonLinter{
		config: config,
	}
}

// Name returns the linter name
func (l *PythonLinter) Name() string {
	return "python"
}

// CanHandle returns true if this linter can handle the given file
func (l *PythonLinter) CanHandle(filePath string) bool {
	return strings.HasSuffix(filePath, ".py")
}

// SetConfig updates the linter configuration
func (l *PythonLinter) SetConfig(config json.RawMessage) error {
	var pythonConfig PythonConfig
	if err := json.Unmarshal(config, &pythonConfig); err != nil {
		return fmt.Errorf("failed to unmarshal python config: %w", err)
	}
	l.config = &pythonConfig
	return nil
}

// Initialize checks for UV availability
func (l *PythonLinter) initialize() {
	l.initOnce.Do(func() {
		if path, err := exec.LookPath("uv"); err == nil {
			l.hasUV = true
			l.uvPath = path
		}
	})
}

// Lint performs linting on a single Python file
func (l *PythonLinter) Lint(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	l.initialize()

	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Basic syntax check using Python's ast module
	if err := l.checkSyntax(ctx, filePath, content); err != nil {
		result.Success = false
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("Syntax error: %v", err),
			Rule:     "syntax",
		})
		return result, nil
	}

	// If UV is not available, return with just syntax check
	if !l.hasUV {
		return result, nil
	}

	// Run ruff linting
	ruffIssues, err := l.runRuffCheck(ctx, filePath, content)
	if err != nil {
		// Log the error but don't fail the entire lint
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "warning",
			Message:  fmt.Sprintf("Ruff check failed: %v", err),
			Rule:     "ruff",
		})
	} else {
		result.Issues = append(result.Issues, ruffIssues...)
	}

	// Run format check
	formatIssues, formatted, err := l.runRuffFormat(ctx, filePath, content)
	if err != nil {
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "warning",
			Message:  fmt.Sprintf("Ruff format check failed: %v", err),
			Rule:     "format",
		})
	} else {
		result.Issues = append(result.Issues, formatIssues...)
		if formatted != nil {
			result.Formatted = formatted
		}
	}

	// Run tests if this is a test file
	if l.isTestFile(filePath) && l.config.RunTests {
		testOutput, testErr := l.runTests(ctx, filePath, content)
		result.TestOutput = testOutput
		if testErr != nil {
			result.Success = false
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     1,
				Column:   1,
				Severity: "error",
				Message:  fmt.Sprintf("Tests failed: %v", testErr),
				Rule:     "test",
			})
		}
	}

	// Update success based on issues
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			result.Success = false
			break
		}
	}

	return result, nil
}

// LintBatch performs linting on multiple Python files at once for better performance
func (l *PythonLinter) LintBatch(ctx context.Context, files map[string][]byte) (map[string]*linters.LintResult, error) {
	l.initialize()

	results := make(map[string]*linters.LintResult)
	var mu sync.Mutex

	// Filter Python files
	pythonFiles := make(map[string][]byte)
	for path, content := range files {
		if l.CanHandle(path) {
			pythonFiles[path] = content
		}
	}

	if len(pythonFiles) == 0 {
		return results, nil
	}

	// Initialize results
	for path := range pythonFiles {
		results[path] = &linters.LintResult{
			Success: true,
			Issues:  []linters.Issue{},
		}
	}

	// First pass: syntax check all files
	var wg sync.WaitGroup
	for filePath, content := range pythonFiles {
		wg.Add(1)
		go func(path string, data []byte) {
			defer wg.Done()
			if err := l.checkSyntax(ctx, path, data); err != nil {
				mu.Lock()
				results[path].Success = false
				results[path].Issues = append(results[path].Issues, linters.Issue{
					File:     path,
					Line:     1,
					Column:   1,
					Severity: "error",
					Message:  fmt.Sprintf("Syntax error: %v", err),
					Rule:     "syntax",
				})
				mu.Unlock()
			}
		}(filePath, content)
	}
	wg.Wait()

	// If UV is not available, return with just syntax checks
	if !l.hasUV {
		return results, nil
	}

	// Collect files that passed syntax check
	validFiles := make([]string, 0, len(pythonFiles))
	for filePath, result := range results {
		if result.Success {
			validFiles = append(validFiles, filePath)
		}
	}

	if len(validFiles) > 0 {
		// Run ruff check on all valid files at once
		if err := l.runRuffBatch(ctx, validFiles, pythonFiles, results); err != nil {
			// Log error but continue
			for _, path := range validFiles {
				results[path].Issues = append(results[path].Issues, linters.Issue{
					File:     path,
					Line:     1,
					Column:   1,
					Severity: "warning",
					Message:  fmt.Sprintf("Batch ruff check failed: %v", err),
					Rule:     "ruff",
				})
			}
		}

		// Run format check on all valid files
		if err := l.runRuffFormatBatch(ctx, validFiles, pythonFiles, results); err != nil {
			for _, path := range validFiles {
				results[path].Issues = append(results[path].Issues, linters.Issue{
					File:     path,
					Line:     1,
					Column:   1,
					Severity: "warning",
					Message:  fmt.Sprintf("Batch format check failed: %v", err),
					Rule:     "format",
				})
			}
		}
	}

	// Run tests for test files
	for path, content := range pythonFiles {
		if l.isTestFile(path) && l.config.RunTests && results[path].Success {
			wg.Add(1)
			go func(filePath string, data []byte) {
				defer wg.Done()
				testOutput, testErr := l.runTests(ctx, filePath, data)
				mu.Lock()
				results[filePath].TestOutput = testOutput
				if testErr != nil {
					results[filePath].Success = false
					results[filePath].Issues = append(results[filePath].Issues, linters.Issue{
						File:     filePath,
						Line:     1,
						Column:   1,
						Severity: "error",
						Message:  fmt.Sprintf("Tests failed: %v", testErr),
						Rule:     "test",
					})
				}
				mu.Unlock()
			}(path, content)
		}
	}
	wg.Wait()

	// Update success status based on issues
	for _, result := range results {
		for _, issue := range result.Issues {
			if issue.Severity == "error" {
				result.Success = false
				break
			}
		}
	}

	return results, nil
}

// checkSyntax performs basic syntax checking using Python's ast module
func (l *PythonLinter) checkSyntax(ctx context.Context, filePath string, content []byte) error {
	// Use Python's ast module to check syntax
	cmd := exec.CommandContext(ctx, "python3", "-m", "ast", "-")
	cmd.Stdin = bytes.NewReader(content)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}

	return nil
}

// runRuffCheck runs ruff linting on a single file
func (l *PythonLinter) runRuffCheck(ctx context.Context, filePath string, content []byte) ([]linters.Issue, error) {
	args := []string{"ruff", "check", "--output-format", "json"}

	// Add custom arguments from config
	if l.config.RuffArgs != nil {
		args = append(args, l.config.RuffArgs...)
	}

	// Use stdin to avoid writing temp files
	args = append(args, "--stdin-filename", filePath, "-")

	cmd := exec.CommandContext(ctx, l.uvPath, append([]string{"tool", "run"}, args...)...) //#nosec G204 -- uvPath is validated
	cmd.Stdin = bytes.NewReader(content)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// ruff returns non-zero exit code when issues are found
	_ = cmd.Run()

	// Parse JSON output
	var ruffOutput []RuffIssue
	if len(stdout.Bytes()) > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &ruffOutput); err != nil {
			// Try alternative format or return error
			return nil, fmt.Errorf("failed to parse ruff output: %w", err)
		}
	}

	// Convert to linters.Issue
	issues := make([]linters.Issue, 0, len(ruffOutput))
	for _, ruffIssue := range ruffOutput {
		issue := linters.Issue{
			File:     filePath,
			Message:  ruffIssue.Message,
			Rule:     ruffIssue.Code,
			Severity: "warning",
		}

		if ruffIssue.Location != nil {
			issue.Line = ruffIssue.Location.Row
			issue.Column = ruffIssue.Location.Column
		}

		issues = append(issues, issue)
	}

	return issues, nil
}

// runRuffFormat checks formatting and optionally returns formatted content
func (l *PythonLinter) runRuffFormat(ctx context.Context, filePath string, content []byte) ([]linters.Issue, []byte, error) {
	// First check if formatting is needed
	args := []string{"ruff", "format", "--check", "--stdin-filename", filePath, "-"}

	cmd := exec.CommandContext(ctx, l.uvPath, append([]string{"tool", "run"}, args...)...) //#nosec G204 -- uvPath is validated
	cmd.Stdin = bytes.NewReader(content)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// File needs formatting
		issue := linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "warning",
			Message:  "File needs formatting",
			Rule:     "format",
		}

		// Get the formatted version
		args[2] = "--"                                                                               // Remove --check
		formatCmd := exec.CommandContext(ctx, l.uvPath, append([]string{"tool", "run"}, args...)...) //#nosec G204 -- uvPath is validated
		formatCmd.Stdin = bytes.NewReader(content)

		var formatOut bytes.Buffer
		formatCmd.Stdout = &formatOut

		if err := formatCmd.Run(); err == nil {
			return []linters.Issue{issue}, formatOut.Bytes(), nil
		}

		return []linters.Issue{issue}, nil, nil
	}

	// File is already formatted
	return nil, nil, nil
}

// runRuffBatch runs ruff on multiple files at once
func (l *PythonLinter) runRuffBatch(ctx context.Context, files []string, contents map[string][]byte, results map[string]*linters.LintResult) error {
	// For batch processing, we need to write temp files or use a different approach
	// Since ruff doesn't support multiple stdin files, we'll process them individually but in parallel

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, filePath := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			issues, err := l.runRuffCheck(ctx, path, contents[path])
			if err != nil {
				mu.Lock()
				results[path].Issues = append(results[path].Issues, linters.Issue{
					File:     path,
					Line:     1,
					Column:   1,
					Severity: "warning",
					Message:  fmt.Sprintf("Ruff check failed: %v", err),
					Rule:     "ruff",
				})
				mu.Unlock()
				return
			}

			mu.Lock()
			results[path].Issues = append(results[path].Issues, issues...)
			mu.Unlock()
		}(filePath)
	}

	wg.Wait()
	return nil
}

// runRuffFormatBatch runs format check on multiple files
func (l *PythonLinter) runRuffFormatBatch(ctx context.Context, files []string, contents map[string][]byte, results map[string]*linters.LintResult) error {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, filePath := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			issues, formatted, err := l.runRuffFormat(ctx, path, contents[path])
			if err != nil {
				mu.Lock()
				results[path].Issues = append(results[path].Issues, linters.Issue{
					File:     path,
					Line:     1,
					Column:   1,
					Severity: "warning",
					Message:  fmt.Sprintf("Format check failed: %v", err),
					Rule:     "format",
				})
				mu.Unlock()
				return
			}

			mu.Lock()
			results[path].Issues = append(results[path].Issues, issues...)
			if formatted != nil {
				results[path].Formatted = formatted
			}
			mu.Unlock()
		}(filePath)
	}

	wg.Wait()
	return nil
}

// isTestFile checks if a file is a test file
func (l *PythonLinter) isTestFile(filePath string) bool {
	base := filepath.Base(filePath)
	return (strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py")) ||
		strings.HasSuffix(base, "_test.py")
}

// runTests runs Python tests for a file
func (l *PythonLinter) runTests(ctx context.Context, filePath string, content []byte) (string, error) {
	// Create a temp file for testing
	tmpFile := filepath.Join("/tmp", filepath.Base(filePath))
	// #nosec G204 -- tmpFile is generated from safe filepath
	if err := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("cat > %s", tmpFile)).Run(); err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = exec.Command("rm", "-f", tmpFile).Run()
	}()

	// Write content to temp file
	cmd := exec.Command("bash", "-c", fmt.Sprintf("cat > %s", tmpFile)) //#nosec G204 -- tmpFile is safe
	cmd.Stdin = bytes.NewReader(content)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	// Run tests based on configured test runner
	testRunner := l.config.TestRunner
	if testRunner == "" {
		testRunner = "pytest"
	}

	args := []string{"run", testRunner}
	if l.config.TestArgs != nil {
		args = append(args, l.config.TestArgs...)
	}
	args = append(args, tmpFile)

	testCmd := exec.CommandContext(ctx, l.uvPath, args...) //#nosec G204 -- uvPath is validated

	var stdout, stderr bytes.Buffer
	testCmd.Stdout = &stdout
	testCmd.Stderr = &stderr

	if err := testCmd.Run(); err != nil {
		output := stdout.String() + "\n" + stderr.String()
		return output, fmt.Errorf("tests failed")
	}

	return stdout.String(), nil
}
