package markdown

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMarkdownLinter_Integration_RealFiles(t *testing.T) {
	linter := NewMarkdownLinter()

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "markdown_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name         string
		filename     string
		content      string
		expectErrors bool
	}{
		{
			name:     "good_markdown_file",
			filename: "good.md",
			content: `# Good Document

This is a well-formatted markdown document.

## Section

- Item 1
  - Nested item
- Item 2

` + "```go" + `
fmt.Println("Hello")
` + "```" + `

Short lines only.`,
			expectErrors: false,
		},
		{
			name:     "bad_markdown_file",
			filename: "bad.md",
			content: `# Bad Document

This line has trailing spaces.   

##### Skipped heading levels

- Item 1
   - Wrong indentation
   
` + "```" + `
no language specified
` + "```" + `

This line is extremely long and violates our line length policy by continuing way past the 120 character limit that we have established.`,
			expectErrors: true,
		},
		{
			name:         "empty_markdown_file",
			filename:     "empty.md",
			content:      "",
			expectErrors: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test file
			filePath := filepath.Join(tmpDir, tt.filename)
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Test that linter can handle the file
			if !linter.CanHandle(filePath) {
				t.Errorf("Linter should handle %s", filePath)
			}

			// Lint the file
			result, err := linter.Lint(context.Background(), filePath, []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			// Check error expectations
			hasErrors := false
			for _, issue := range result.Issues {
				if issue.Severity == "error" {
					hasErrors = true
					break
				}
			}

			if hasErrors != tt.expectErrors {
				t.Errorf("Expected errors: %v, got errors: %v", tt.expectErrors, hasErrors)
				if hasErrors {
					t.Logf("Errors found:")
					for _, issue := range result.Issues {
						if issue.Severity == "error" {
							t.Logf("  %s:%d:%d %s (%s)", issue.File, issue.Line, issue.Column, issue.Message, issue.Rule)
						}
					}
				}
			}

			// Success should be false if there are errors
			if hasErrors && result.Success {
				t.Errorf("Expected Success=false when errors present, got true")
			}

			// Should always have formatted output (even if same as input)
			// except for empty files
			if len(tt.content) > 0 && len(result.Formatted) == 0 {
				t.Error("Expected formatted output for non-empty content")
			}
		})
	}
}

func TestMarkdownLinter_Performance(t *testing.T) {
	linter := NewMarkdownLinter()

	// Create a large markdown document
	content := `# Performance Test Document

This is a performance test for the markdown linter.

## Large Content Section
`

	// Add many sections to test performance
	for i := 0; i < 100; i++ {
		content += `
### Section ` + string(rune(i+1)) + `

This is section content with some text that should be long enough to test
line length checking and other rules. We want to make sure our linter
performs well even on larger documents.

- List item 1
  - Nested item
- List item 2

` + "```go" + `
// Code example ` + string(rune(i+1)) + `
func example() {
    fmt.Printf("Section %d\n", ` + string(rune(i+1)) + `)
}
` + "```" + `
`
	}

	start := time.Now()
	result, err := linter.Lint(context.Background(), "perf_test.md", []byte(content))
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Performance test failed: %v", err)
	}

	// Should complete in reasonable time (adjust threshold as needed)
	if duration > 5*time.Second {
		t.Errorf("Linting took too long: %v", duration)
	}

	t.Logf("Linted %d bytes in %v", len(content), duration)

	// Should still produce results
	if result == nil {
		t.Error("Expected result, got nil")
	} else {
		t.Logf("Found %d issues", len(result.Issues))
	}
}

func TestMarkdownLinter_ConcurrentAccess(t *testing.T) {
	linter := NewMarkdownLinter()

	content := `# Concurrent Test

This is a test for concurrent access to the markdown linter.

## Section

- Item 1
  - Nested
- Item 2

` + "```go" + `
fmt.Println("Hello")
` + "```" + `

Normal content here.`

	// Test concurrent linting
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			_, err := linter.Lint(context.Background(), "concurrent_test.md", []byte(content))
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Errorf("Concurrent linting failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("Timeout waiting for goroutine %d", i)
		}
	}
}

func TestMarkdownLinter_ContextCancellation(t *testing.T) {
	linter := NewMarkdownLinter()

	// Create a context that will be canceled
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	content := `# Context Test

Small document for context cancellation test.`

	// This should still work for small documents even with canceled context
	// (since goldmark parsing is fast)
	_, err := linter.Lint(ctx, "context_test.md", []byte(content))

	// We don't expect this to fail due to cancellation for small documents,
	// but we're testing that the context is properly passed through
	if err != nil {
		// Only fail if it's not a context cancellation error
		if err != context.Canceled {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestMarkdownLinter_EdgeCases(t *testing.T) {
	linter := NewMarkdownLinter()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "only_whitespace",
			content: "   \n\t\n   ",
		},
		{
			name:    "unicode_content",
			content: "# 测试文档\n\n这是一个包含Unicode字符的测试。\n\n## Раздел\n\nТекст на кириллице.",
		},
		{
			name:    "mixed_line_endings",
			content: "# Title\r\n\r\nContent with Windows line endings.\n\nAnd Unix endings.",
		},
		{
			name:    "very_long_single_line",
			content: "# " + string(make([]byte, 1000)), // Very long heading
		},
		{
			name: "nested_markdown_structures",
			content: `# Title

> Quote
> 
> - List in quote
>   - Nested in quote
> 
> ` + "```go" + `
> // Code in quote
> fmt.Println("test")
> ` + "```" + `

Normal content.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := linter.Lint(context.Background(), "edge_test.md", []byte(tt.content))
			if err != nil {
				t.Errorf("Edge case %s failed: %v", tt.name, err)
			}

			// Should always produce a result
			if result == nil {
				t.Errorf("Expected result for edge case %s", tt.name)
			}
		})
	}
}

func TestMarkdownLinter_AllRules(t *testing.T) {
	linter := NewMarkdownLinter()

	// Document designed to trigger all rule types
	content := `# Document with All Issues

This line has trailing whitespace.   

##### Skipped H2 and H3

# Multiple H1 headings

- List
   - Bad indentation (3 spaces)
  - Good indentation
 - Wrong indentation (1 space)

` + "```" + `
code without language
` + "```" + `

` + "```go" + `
// Good code block
fmt.Println("test")
` + "```" + `

This line is extremely long and exceeds our 120 character limit by being way too verbose and continuing past what is reasonable for readable documentation.

_Using underscores for emphasis instead of asterisks_



Too many blank lines above (more than 2).

## Normal Section

Content here.`

	result, err := linter.Lint(context.Background(), "all_rules_test.md", []byte(content))
	if err != nil {
		t.Fatalf("All rules test failed: %v", err)
	}

	// Should trigger multiple rule types
	rulesSeen := make(map[string]bool)
	for _, issue := range result.Issues {
		rulesSeen[issue.Rule] = true
	}

	expectedRules := []string{
		"heading-hierarchy",
		"trailing-whitespace",
		"list-indentation",
		"code-block-language",
		"line-length",
		"blank-line-spacing",
	}

	for _, rule := range expectedRules {
		if !rulesSeen[rule] {
			t.Errorf("Expected to see rule %s triggered, but didn't", rule)
		}
	}

	// Should have multiple issues
	if len(result.Issues) < 5 {
		t.Errorf("Expected at least 5 issues, got %d", len(result.Issues))
	}

	// Should fail due to errors
	if result.Success {
		t.Error("Expected Success=false due to multiple errors")
	}

	t.Logf("All rules test found %d total issues across %d rule types",
		len(result.Issues), len(rulesSeen))
}
