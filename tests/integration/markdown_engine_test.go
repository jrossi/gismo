package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrossi/gismo/pkg/engine"
)

func TestLintingRuleEngine_MarkdownIntegration(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	// Load test fixtures
	goodContent, err := os.ReadFile(filepath.Join("..", "fixtures", "markdown", "good.md"))
	if err != nil {
		t.Fatalf("Failed to read good.md fixture: %v", err)
	}

	badTrailingContent, err := os.ReadFile(filepath.Join("..", "fixtures", "markdown", "bad_trailing.md"))
	if err != nil {
		t.Fatalf("Failed to read bad_trailing.md fixture: %v", err)
	}

	tests := []struct {
		name           string
		toolName       string
		filePath       string
		content        string
		expectBlocking bool
		expectApproval bool
	}{
		{
			name:           "good_markdown_write",
			toolName:       "Write",
			filePath:       "good.md",
			content:        string(goodContent),
			expectBlocking: false,
			expectApproval: true,
		},
		{
			name:           "bad_markdown_with_errors",
			toolName:       "Write",
			filePath:       "bad.md",
			content:        string(badTrailingContent),
			expectBlocking: true,
			expectApproval: false,
		},
		{
			name:           "markdown_edit_operation",
			toolName:       "Edit",
			filePath:       "test.md",
			content:        "# Simple edit",
			expectBlocking: false,
			expectApproval: true,
		},
		{
			name:           "non_markdown_file",
			toolName:       "Write",
			filePath:       "test.go",
			content:        "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
			expectBlocking: false,
			expectApproval: true,
		},
		{
			name:           "markdown_multiedit",
			toolName:       "MultiEdit",
			filePath:       "multi.md",
			content:        "# Multi Edit Test\n\nContent here.",
			expectBlocking: false,
			expectApproval: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &engine.PreToolUseMessage{
				BaseHookMessage: engine.BaseHookMessage{
					HookEventName: engine.PreToolUseEvent,
				},
				ToolName: tt.toolName,
				ToolInput: testConvertToRawMessage(map[string]interface{}{
					"file_path": tt.filePath,
					"content":   tt.content,
				}),
			}

			ctx := context.Background()
			resp, err := lintEngine.EvaluatePreToolUse(ctx, msg)
			if err != nil {
				t.Fatalf("EvaluatePreToolUse() error = %v", err)
			}

			if tt.expectBlocking {
				if resp == nil || resp.Decision != "block" {
					t.Errorf("Expected blocking response, got %v", resp)
				}
			} else if tt.expectApproval {
				if resp == nil || resp.Decision != "approve" {
					t.Errorf("Expected approval response, got %v", resp)
				}
			}
		})
	}
}

func TestLintingRuleEngine_MarkdownErrorHandling(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	tests := []struct {
		name    string
		input   map[string]interface{}
		wantErr bool
	}{
		{
			name: "missing_file_path",
			input: map[string]interface{}{
				"content": "# Test",
			},
			wantErr: false, // Should not error, just skip
		},
		{
			name: "missing_content",
			input: map[string]interface{}{
				"file_path": "test.md",
			},
			wantErr: false, // Should not error, just skip
		},
		{
			name: "invalid_content_type",
			input: map[string]interface{}{
				"file_path": "test.md",
				"content":   123, // Not a string
			},
			wantErr: false, // Should not error, just skip
		},
		{
			name: "empty_content",
			input: map[string]interface{}{
				"file_path": "test.md",
				"content":   "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &engine.PreToolUseMessage{
				BaseHookMessage: engine.BaseHookMessage{
					HookEventName: engine.PreToolUseEvent,
				},
				ToolName:  "Write",
				ToolInput: testConvertToRawMessage(tt.input),
			}

			ctx := context.Background()
			resp, err := lintEngine.EvaluatePreToolUse(ctx, msg)

			if tt.wantErr && err == nil {
				t.Error("Expected error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Should always get some response (approve by default)
			if err == nil && resp == nil {
				t.Error("Expected response, got nil")
			}
		})
	}
}

func TestLintingRuleEngine_MarkdownOutputFormatting(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	badMarkdown := `# Bad Document

Line with trailing spaces.   

##### Skipped levels

- Item
   - Wrong indent

` + "```" + `
no language
` + "```" + `

Very long line that exceeds our 120 character limit and should trigger a line length warning from our linter implementation.`

	msg := &engine.PreToolUseMessage{
		BaseHookMessage: engine.BaseHookMessage{
			HookEventName: engine.PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "format_test.md",
			"content":   badMarkdown,
		}),
	}

	ctx := context.Background()
	resp, err := lintEngine.EvaluatePreToolUse(ctx, msg)
	if err != nil {
		t.Fatalf("EvaluatePreToolUse() error = %v", err)
	}

	// Should block due to errors
	if resp == nil || resp.Decision != "block" {
		t.Errorf("Expected blocking response, got %v", resp)
	}

	// Should have informative reason
	if resp != nil && resp.Reason == "" {
		t.Error("Expected reason in blocking response")
	}

	// Should mention multiple errors
	if resp != nil && !strings.Contains(resp.Reason, "error") {
		t.Errorf("Expected reason to mention errors, got: %s", resp.Reason)
	}
}

func TestLintingRuleEngine_MarkdownSuccessCase(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	goodMarkdown := `# Good Document

This is a well-formatted markdown document.

## Section

- Item 1
  - Nested item
- Item 2

` + "```go" + `
fmt.Println("Hello")
` + "```" + `

Content here is properly formatted.`

	msg := &engine.PreToolUseMessage{
		BaseHookMessage: engine.BaseHookMessage{
			HookEventName: engine.PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "success_test.md",
			"content":   goodMarkdown,
		}),
	}

	ctx := context.Background()
	resp, err := lintEngine.EvaluatePreToolUse(ctx, msg)
	if err != nil {
		t.Fatalf("EvaluatePreToolUse() error = %v", err)
	}

	// Should approve (might have warnings but no errors)
	if resp == nil || resp.Decision != "approve" {
		t.Errorf("Expected approval response, got %v", resp)
	}
}

func TestLintingRuleEngine_MarkdownPerformance(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	// Create large markdown content
	largeContent := "# Performance Test\n\n"
	for i := 0; i < 1000; i++ {
		largeContent += "## Section " + string(rune(i)) + "\n\nContent here.\n\n"
	}

	msg := &engine.PreToolUseMessage{
		BaseHookMessage: engine.BaseHookMessage{
			HookEventName: engine.PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "large_test.md",
			"content":   largeContent,
		}),
	}

	ctx := context.Background()
	resp, err := lintEngine.EvaluatePreToolUse(ctx, msg)
	if err != nil {
		t.Fatalf("Large document linting failed: %v", err)
	}

	// Should complete and return response
	if resp == nil {
		t.Error("Expected response for large document")
	}

	t.Logf("Successfully processed %d bytes of markdown", len(largeContent))
}

func TestLintingRuleEngine_MarkdownConcurrency(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	content := `# Concurrent Test

This is a test document for concurrent processing.

## Section

- Item 1
  - Nested
- Item 2`

	// Test concurrent requests
	const numRequests = 50
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			msg := &engine.PreToolUseMessage{
				BaseHookMessage: engine.BaseHookMessage{
					HookEventName: engine.PreToolUseEvent,
				},
				ToolName: "Write",
				ToolInput: testConvertToRawMessage(map[string]interface{}{
					"file_path": "concurrent_test.md",
					"content":   content,
				}),
			}

			ctx := context.Background()
			_, err := lintEngine.EvaluatePreToolUse(ctx, msg)
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent request %d failed: %v", i, err)
		}
	}
}

func TestLintingRuleEngine_MarkdownMixedWithGo(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	tests := []struct {
		name     string
		filePath string
		content  string
		wantGo   bool
		wantMd   bool
	}{
		{
			name:     "go_file",
			filePath: "test.go",
			content:  "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
			wantGo:   true,
			wantMd:   false,
		},
		{
			name:     "markdown_file",
			filePath: "test.md",
			content:  "# Test\n\nContent here.",
			wantGo:   false,
			wantMd:   true,
		},
		{
			name:     "readme_file",
			filePath: "README.md",
			content:  "# Project\n\nDescription here.",
			wantGo:   false,
			wantMd:   true,
		},
		{
			name:     "other_file",
			filePath: "config.json",
			content:  `{"key": "value"}`,
			wantGo:   false,
			wantMd:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &engine.PreToolUseMessage{
				BaseHookMessage: engine.BaseHookMessage{
					HookEventName: engine.PreToolUseEvent,
				},
				ToolName: "Write",
				ToolInput: testConvertToRawMessage(map[string]interface{}{
					"file_path": tt.filePath,
					"content":   tt.content,
				}),
			}

			ctx := context.Background()
			resp, err := lintEngine.EvaluatePreToolUse(ctx, msg)
			if err != nil {
				t.Fatalf("EvaluatePreToolUse() error = %v", err)
			}

			// Should get appropriate response based on file type
			if tt.wantGo || tt.wantMd {
				if resp == nil {
					t.Error("Expected response for handled file type")
				}
			}
		})
	}
}
