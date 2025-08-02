package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jrossi/gismo/pkg/engine"
)

func TestLintingRuleEngine_GolangIntegration(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	tests := []struct {
		name           string
		toolName       string
		filePath       string
		content        string
		expectBlocking bool
		expectApproval bool
	}{
		{
			name:     "good_golang_write",
			toolName: "Write",
			filePath: "good.go",
			content: `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`,
			expectBlocking: false,
			expectApproval: true,
		},
		{
			name:     "bad_golang_formatting",
			toolName: "Write",
			filePath: "bad.go",
			content: `package main

import "fmt"

func main(){
fmt.Println("Bad formatting")
}`,
			expectBlocking: false, // Formatting issues are warnings, not blocking
			expectApproval: true,
		},
		{
			name:           "golang_edit_operation",
			toolName:       "Edit",
			filePath:       "test.go",
			content:        "package main\n\nfunc main() {}",
			expectBlocking: false,
			expectApproval: true,
		},
		{
			name:           "non_go_file",
			toolName:       "Write",
			filePath:       "test.md",
			content:        "# Test Document\n\nContent here.",
			expectBlocking: false,
			expectApproval: true,
		},
		{
			name:           "golang_multiedit",
			toolName:       "MultiEdit",
			filePath:       "multi.go",
			content:        "package main\n\nfunc test() {}",
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

func TestLintingRuleEngine_GolangErrorHandling(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	tests := []struct {
		name    string
		input   map[string]interface{}
		wantErr bool
	}{
		{
			name: "missing_file_path",
			input: map[string]interface{}{
				"content": "package main",
			},
			wantErr: false, // Should not error, just skip
		},
		{
			name: "missing_content",
			input: map[string]interface{}{
				"file_path": "test.go",
			},
			wantErr: false, // Should not error, just skip
		},
		{
			name: "invalid_content_type",
			input: map[string]interface{}{
				"file_path": "test.go",
				"content":   123, // Not a string
			},
			wantErr: false, // Should not error, just skip
		},
		{
			name: "empty_content",
			input: map[string]interface{}{
				"file_path": "test.go",
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

func TestLintingRuleEngine_GolangOutputFormatting(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	badGolang := `package main

import "fmt"

func main(){
fmt.Println("Bad formatting")
if true{
fmt.Println("More bad formatting")
}
}`

	msg := &engine.PreToolUseMessage{
		BaseHookMessage: engine.BaseHookMessage{
			HookEventName: engine.PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "format_test.go",
			"content":   badGolang,
		}),
	}

	ctx := context.Background()
	resp, err := lintEngine.EvaluatePreToolUse(ctx, msg)
	if err != nil {
		t.Fatalf("EvaluatePreToolUse() error = %v", err)
	}

	// Should not block for formatting issues (they're warnings)
	if resp == nil || resp.Decision != "approve" {
		t.Errorf("Expected approval response for formatting issues, got %v", resp)
	}

	// Should have informative reason about warnings
	if resp != nil && resp.Message == "" {
		t.Error("Expected message in response")
	}
}

func TestLintingRuleEngine_GolangSuccessCase(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	goodGolang := `package main

import (
	"context"
	"fmt"
)

// GoodFunction demonstrates proper Go code
func GoodFunction(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	
	fmt.Printf("Hello, %s!\n", name)
	return nil
}`

	msg := &engine.PreToolUseMessage{
		BaseHookMessage: engine.BaseHookMessage{
			HookEventName: engine.PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "success_test.go",
			"content":   goodGolang,
		}),
	}

	ctx := context.Background()
	resp, err := lintEngine.EvaluatePreToolUse(ctx, msg)
	if err != nil {
		t.Fatalf("EvaluatePreToolUse() error = %v", err)
	}

	// Should approve
	if resp == nil || resp.Decision != "approve" {
		t.Errorf("Expected approval response, got %v", resp)
	}
}

func TestLintingRuleEngine_GolangPerformance(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	// Create large Go content
	largeContent := "package main\n\nimport \"fmt\"\n\n"
	for i := 0; i < 1000; i++ {
		largeContent += fmt.Sprintf("func Function%d() {\n\tfmt.Println(\"Function %d\")\n}\n\n", i, i)
	}

	msg := &engine.PreToolUseMessage{
		BaseHookMessage: engine.BaseHookMessage{
			HookEventName: engine.PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "large_test.go",
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

	t.Logf("Successfully processed %d bytes of Go code", len(largeContent))
}

func TestLintingRuleEngine_GolangConcurrency(t *testing.T) {
	lintEngine := engine.NewLintingRuleEngine()

	content := `package main

import "fmt"

func main() {
	fmt.Println("Concurrent test")
}`

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
					"file_path": "concurrent_test.go",
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

func TestLintingRuleEngine_GolangWithMarkdown(t *testing.T) {
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
			name:     "go_mod_file",
			filePath: "go.mod",
			content:  "module test\n\ngo 1.21",
			wantGo:   false,
			wantMd:   false,
		},
		{
			name:     "other_file",
			filePath: "config.yaml",
			content:  "key: value",
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
