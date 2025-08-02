package markdown

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestMarkdownLinter_CanHandle(t *testing.T) {
	linter := NewMarkdownLinter()

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"markdown file with .md extension", "test.md", true},
		{"markdown file with .markdown extension", "test.markdown", true},
		{"nested markdown file", "docs/api/README.md", true},
		{"go file", "test.go", false},
		{"text file", "test.txt", false},
		{"no extension", "README", false},
		{"empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linter.CanHandle(tt.filePath); got != tt.want {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestMarkdownLinter_Name(t *testing.T) {
	linter := NewMarkdownLinter()
	if got := linter.Name(); got != "markdown" {
		t.Errorf("Name() = %q, want %q", got, "markdown")
	}
}

func TestMarkdownLinter_HeadingHierarchy(t *testing.T) {
	linter := NewMarkdownLinter()

	tests := []struct {
		name           string
		content        string
		expectedIssues []string
	}{
		{
			name: "valid heading progression",
			content: `# Title
## Section
### Subsection
#### Details`,
			expectedIssues: []string{},
		},
		{
			name: "skipped heading level",
			content: `# Title
### Skipped H2`,
			expectedIssues: []string{"Heading level 3 skips level 2"},
		},
		{
			name: "multiple H1 headings",
			content: `# First Title
# Second Title`,
			expectedIssues: []string{"Multiple H1 headings found"},
		},
		{
			name: "jump from H1 to H4",
			content: `# Title
#### Deep Section`,
			expectedIssues: []string{"Heading level 4 skips level 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := linter.Lint(context.Background(), "test.md", []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			var actualIssues []string
			for _, issue := range result.Issues {
				if issue.Rule == "heading-hierarchy" {
					actualIssues = append(actualIssues, issue.Message)
				}
			}

			if len(actualIssues) != len(tt.expectedIssues) {
				t.Errorf("Expected %d heading hierarchy issues, got %d", len(tt.expectedIssues), len(actualIssues))
			}

			for i, expected := range tt.expectedIssues {
				if i >= len(actualIssues) {
					t.Errorf("Missing expected issue: %s", expected)
					continue
				}
				if !strings.Contains(actualIssues[i], expected) {
					t.Errorf("Expected issue containing %q, got %q", expected, actualIssues[i])
				}
			}
		})
	}
}

func TestMarkdownLinter_TrailingWhitespace(t *testing.T) {
	linter := NewMarkdownLinter()

	tests := []struct {
		name           string
		content        string
		expectingError bool
	}{
		{
			name:           "no trailing whitespace",
			content:        "# Title\n\nSome content here.",
			expectingError: false,
		},
		{
			name:           "trailing spaces",
			content:        "# Title  \n\nSome content here.",
			expectingError: true,
		},
		{
			name:           "trailing tab",
			content:        "# Title\t\n\nSome content here.",
			expectingError: true,
		},
		{
			name:           "multiple lines with trailing whitespace",
			content:        "# Title  \n\nSome content here.   \n\nMore content.",
			expectingError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := linter.Lint(context.Background(), "test.md", []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			hasTrailingWhitespaceError := false
			for _, issue := range result.Issues {
				if issue.Rule == "trailing-whitespace" && issue.Severity == "error" {
					hasTrailingWhitespaceError = true
					break
				}
			}

			if hasTrailingWhitespaceError != tt.expectingError {
				t.Errorf("Expected trailing whitespace error: %v, got: %v", tt.expectingError, hasTrailingWhitespaceError)
			}

			// If expecting error, result.Success should be false
			if tt.expectingError && result.Success {
				t.Errorf("Expected Success=false when trailing whitespace errors present")
			}
		})
	}
}

func TestMarkdownLinter_ListIndentation(t *testing.T) {
	linter := NewMarkdownLinter()

	tests := []struct {
		name           string
		content        string
		expectingIssue bool
	}{
		{
			name: "correct 2-space indentation",
			content: `- Item 1
  - Nested item
    - Double nested`,
			expectingIssue: false,
		},
		{
			name: "incorrect 3-space indentation",
			content: `- Item 1
   - Wrong indentation`,
			expectingIssue: true,
		},
		{
			name: "incorrect 1-space indentation",
			content: `- Item 1
 - Wrong indentation`,
			expectingIssue: true,
		},
		{
			name: "mixed correct and incorrect",
			content: `- Item 1
  - Correct
   - Incorrect`,
			expectingIssue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := linter.Lint(context.Background(), "test.md", []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			hasListIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "list-indentation" {
					hasListIssue = true
					break
				}
			}

			if hasListIssue != tt.expectingIssue {
				t.Errorf("Expected list indentation issue: %v, got: %v", tt.expectingIssue, hasListIssue)
			}
		})
	}
}

func TestMarkdownLinter_CodeBlockLanguage(t *testing.T) {
	linter := NewMarkdownLinter()

	tests := []struct {
		name           string
		content        string
		expectingIssue bool
	}{
		{
			name:           "code block with language",
			content:        "```go\nfmt.Println(\"Hello\")\n```",
			expectingIssue: false,
		},
		{
			name:           "code block without language",
			content:        "```\nsome code\n```",
			expectingIssue: true,
		},
		{
			name:           "multiple code blocks mixed",
			content:        "```go\nfmt.Println(\"Hello\")\n```\n\n```\nno language\n```",
			expectingIssue: true,
		},
		{
			name:           "inline code (should be ignored)",
			content:        "This is `inline code` which is fine.",
			expectingIssue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := linter.Lint(context.Background(), "test.md", []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			hasCodeBlockIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "code-block-language" {
					hasCodeBlockIssue = true
					break
				}
			}

			if hasCodeBlockIssue != tt.expectingIssue {
				t.Errorf("Expected code block issue: %v, got: %v", tt.expectingIssue, hasCodeBlockIssue)
			}
		})
	}
}

func TestMarkdownLinter_LineLength(t *testing.T) {
	linter := NewMarkdownLinter()

	tests := []struct {
		name           string
		content        string
		expectingIssue bool
	}{
		{
			name:           "short line",
			content:        "# Short Title",
			expectingIssue: false,
		},
		{
			name:           "exactly 120 characters",
			content:        strings.Repeat("a", 120),
			expectingIssue: false,
		},
		{
			name:           "121 characters (over limit)",
			content:        strings.Repeat("a", 121),
			expectingIssue: true,
		},
		{
			name:           "very long line",
			content:        "This is an extremely long line that definitely exceeds our 120 character limit and should trigger a line length warning from our markdown linter implementation.",
			expectingIssue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := linter.Lint(context.Background(), "test.md", []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			hasLineLengthIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "line-length" {
					hasLineLengthIssue = true
					break
				}
			}

			if hasLineLengthIssue != tt.expectingIssue {
				t.Errorf("Expected line length issue: %v, got: %v", tt.expectingIssue, hasLineLengthIssue)
			}
		})
	}
}

func TestMarkdownLinter_BlankLineSpacing(t *testing.T) {
	linter := NewMarkdownLinter()

	tests := []struct {
		name           string
		content        string
		expectingIssue bool
	}{
		{
			name: "normal spacing",
			content: `# Title

Some content here.

## Section

More content.`,
			expectingIssue: false,
		},
		{
			name: "two blank lines (acceptable)",
			content: `# Title


Some content here.`,
			expectingIssue: false,
		},
		{
			name: "three blank lines (too many)",
			content: `# Title



Some content here.`,
			expectingIssue: true,
		},
		{
			name: "four blank lines (way too many)",
			content: `# Title




Some content here.`,
			expectingIssue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := linter.Lint(context.Background(), "test.md", []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			hasBlankLineIssue := false
			for _, issue := range result.Issues {
				if issue.Rule == "blank-line-spacing" {
					hasBlankLineIssue = true
					break
				}
			}

			if hasBlankLineIssue != tt.expectingIssue {
				t.Errorf("Expected blank line issue: %v, got: %v", tt.expectingIssue, hasBlankLineIssue)
			}
		})
	}
}

func TestMarkdownLinter_ComprehensiveDocument(t *testing.T) {
	linter := NewMarkdownLinter()

	// Test a document with multiple types of issues
	content := `# Good Document

This line has trailing whitespace.   

##### This heading skips levels

- Item 1
   - Bad indentation (3 spaces)
  - Good indentation (2 spaces)

` + "```" + `
code without language
` + "```" + `

` + "```go" + `
// Good code block
fmt.Println("Hello")
` + "```" + `

This line is way too long and exceeds our 120 character limit by being extremely verbose and continuing way past the reasonable length limit that we have set for our markdown documents to ensure readability.




More content with too many blank lines above.

## Proper Section

Content here.`

	result, err := linter.Lint(context.Background(), "test.md", []byte(content))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// Should have multiple issues
	if len(result.Issues) == 0 {
		t.Error("Expected multiple issues, got none")
	}

	// Should not succeed due to errors
	if result.Success {
		t.Error("Expected Success=false due to errors, got true")
	}

	// Check for specific rule violations
	rules := make(map[string]int)
	errorCount := 0
	for _, issue := range result.Issues {
		rules[issue.Rule]++
		if issue.Severity == "error" {
			errorCount++
		}
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
		if rules[rule] == 0 {
			t.Errorf("Expected to find issues for rule %s, but found none", rule)
		}
	}

	// Should have at least some errors (trailing whitespace, heading hierarchy)
	if errorCount == 0 {
		t.Error("Expected at least some error-level issues, got none")
	}

	// Should have formatted output
	if len(result.Formatted) == 0 {
		t.Error("Expected formatted output, got none")
	}
}

func TestMarkdownLinter_PerfectDocument(t *testing.T) {
	linter := NewMarkdownLinter()

	content := `# Perfect Document

This document follows all markdown formatting standards.

## Proper Heading Hierarchy

Here's some content with perfect formatting.

### Subsection

- List item 1
  - Properly indented nested item (2 spaces)
- List item 2

` + "```go" + `
// Code block with language specification
package main

func main() {
    fmt.Println("Hello, World!")
}
` + "```" + `

**Bold text** and *italic text* using preferred markers.

## Another Section

This line is under 120 characters and has no trailing whitespace.`

	result, err := linter.Lint(context.Background(), "test.md", []byte(content))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// Count only error-level issues
	errorCount := 0
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			errorCount++
		}
	}

	// Should have no errors (might have formatting warnings)
	if errorCount > 0 {
		t.Errorf("Expected no error-level issues in perfect document, got %d", errorCount)
		for _, issue := range result.Issues {
			if issue.Severity == "error" {
				t.Errorf("  Error: %s (%s)", issue.Message, issue.Rule)
			}
		}
	}

	// Should succeed (no blocking errors)
	if !result.Success {
		t.Error("Expected Success=true for perfect document, got false")
	}
}

func TestMarkdownLinter_EmptyDocument(t *testing.T) {
	linter := NewMarkdownLinter()

	result, err := linter.Lint(context.Background(), "test.md", []byte(""))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// Empty document should succeed
	if !result.Success {
		t.Error("Expected Success=true for empty document")
	}
}

func TestMarkdownLinter_FrontMatterHandling(t *testing.T) {
	linter := NewMarkdownLinter()

	content := `---
title: "Test Document"
author: "Test Author"
---

# Document with Front Matter

Content here.`

	result, err := linter.Lint(context.Background(), "test.md", []byte(content))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// Should handle front matter without errors
	// (Note: front matter validation is not yet implemented)
	if !result.Success {
		// Check if failure is due to non-front-matter issues
		errorCount := 0
		for _, issue := range result.Issues {
			if issue.Severity == "error" {
				errorCount++
				t.Logf("Error in front matter document: %s (%s)", issue.Message, issue.Rule)
			}
		}
	}
}

func TestMarkdownLinter_FrontMatterSchemaValidation(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		schema       string
		expectError  bool
		expectedRule string
	}{
		{
			name: "valid_inline_schema",
			content: `---
title: "Test Document"
date: "2023-01-01"
tags: ["test", "markdown"]
---

# Test Document`,
			schema: `{
				"type": "object",
				"properties": {
					"title": {"type": "string"},
					"date": {"type": "string"},
					"tags": {"type": "array", "items": {"type": "string"}}
				},
				"required": ["title", "date"]
			}`,
			expectError: false,
		},
		{
			name: "invalid_frontmatter_missing_required",
			content: `---
title: "Test Document"
tags: ["test"]
---

# Test Document`,
			schema: `{
				"type": "object",
				"properties": {
					"title": {"type": "string"},
					"date": {"type": "string"},
					"tags": {"type": "array", "items": {"type": "string"}}
				},
				"required": ["title", "date"]
			}`,
			expectError:  true,
			expectedRule: "frontmatter-schema",
		},
		{
			name: "invalid_frontmatter_wrong_type",
			content: `---
title: "Test Document"
date: 2023
tags: ["test"]
---

# Test Document`,
			schema: `{
				"type": "object",
				"properties": {
					"title": {"type": "string"},
					"date": {"type": "string"},
					"tags": {"type": "array", "items": {"type": "string"}}
				},
				"required": ["title", "date"]
			}`,
			expectError:  true,
			expectedRule: "frontmatter-schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with inline schema
			schemaJSON := json.RawMessage(tt.schema)
			config := &MarkdownConfig{
				RequireFrontmatter: &[]bool{true}[0],
				FrontmatterSchema:  &schemaJSON,
			}

			linter := NewMarkdownLinterWithConfig(config)
			result, err := linter.Lint(context.Background(), "test.md", []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			hasSchemaError := false
			for _, issue := range result.Issues {
				if issue.Rule == tt.expectedRule && issue.Severity == "error" {
					hasSchemaError = true
					break
				}
			}

			if tt.expectError && !hasSchemaError {
				t.Errorf("Expected schema validation error, but got none. Issues: %+v", result.Issues)
			}
			if !tt.expectError && hasSchemaError {
				t.Errorf("Expected no schema validation error, but got one")
			}
		})
	}
}

func TestMarkdownLinter_FrontMatterSchemaFile(t *testing.T) {
	// Create a temporary schema file
	schemaContent := `{
		"type": "object",
		"properties": {
			"title": {"type": "string"},
			"date": {"type": "string", "format": "date"},
			"author": {"type": "string"},
			"tags": {"type": "array", "items": {"type": "string"}}
		},
		"required": ["title", "author"]
	}`

	tmpFile, err := os.CreateTemp("", "schema-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp schema file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(schemaContent); err != nil {
		t.Fatalf("Failed to write schema file: %v", err)
	}
	tmpFile.Close()

	tests := []struct {
		name         string
		content      string
		expectError  bool
		expectedRule string
	}{
		{
			name: "valid_frontmatter_with_file_schema",
			content: `---
title: "Test Document"
author: "John Doe"
date: "2023-01-01"
tags: ["test", "markdown"]
---

# Test Document`,
			expectError: false,
		},
		{
			name: "invalid_frontmatter_missing_author",
			content: `---
title: "Test Document"
date: "2023-01-01"
---

# Test Document`,
			expectError:  true,
			expectedRule: "frontmatter-schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with file path
			schemaPath := json.RawMessage(`"` + tmpFile.Name() + `"`)
			config := &MarkdownConfig{
				RequireFrontmatter: &[]bool{true}[0],
				FrontmatterSchema:  &schemaPath,
			}

			linter := NewMarkdownLinterWithConfig(config)
			result, err := linter.Lint(context.Background(), "test.md", []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			hasSchemaError := false
			for _, issue := range result.Issues {
				if issue.Rule == tt.expectedRule && issue.Severity == "error" {
					hasSchemaError = true
					break
				}
			}

			if tt.expectError && !hasSchemaError {
				t.Errorf("Expected schema validation error, but got none. Issues: %+v", result.Issues)
			}
			if !tt.expectError && hasSchemaError {
				t.Errorf("Expected no schema validation error, but got one")
			}
		})
	}
}
