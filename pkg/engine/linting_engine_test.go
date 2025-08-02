package engine

import (
	"context"
	"testing"

	"github.com/jrossi/gismo/pkg/linters"
)

// MockLinter for testing
type MockLinter struct {
	name      string
	canHandle bool
	result    *linters.LintResult
	err       error
}

func (m *MockLinter) Name() string {
	return m.name
}

func (m *MockLinter) CanHandle(filePath string) bool {
	return m.canHandle
}

func (m *MockLinter) Lint(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	return m.result, m.err
}

func TestLintingRuleEngine_EvaluatePreToolUse(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]interface{}
		linter   *MockLinter
		want     string // decision
	}{
		{
			name:     "non-file operation",
			toolName: "Read",
			input:    map[string]interface{}{},
			want:     "approve",
		},
		{
			name:     "edit operation",
			toolName: "Edit",
			input: map[string]interface{}{
				"file_path": "test.go",
			},
			want: "approve", // Edit is approved, linting happens after
		},
		{
			name:     "write with no file path",
			toolName: "Write",
			input:    map[string]interface{}{},
			want:     "approve",
		},
		{
			name:     "write with no content",
			toolName: "Write",
			input: map[string]interface{}{
				"file_path": "test.go",
			},
			want: "approve",
		},
		{
			name:     "write with syntax error",
			toolName: "Write",
			input: map[string]interface{}{
				"file_path": "test.go",
				"content":   "package main\nfunc main() { invalid syntax",
			},
			linter: &MockLinter{
				canHandle: true,
				result: &linters.LintResult{
					Success: false,
					Issues: []linters.Issue{
						{
							Severity: "error",
							Message:  "syntax error",
							Rule:     "syntax",
						},
					},
				},
			},
			want: "block",
		},
		{
			name:     "write with formatting issue",
			toolName: "Write",
			input: map[string]interface{}{
				"file_path": "test.go",
				"content":   "package main\nfunc main(){\nfmt.Println(\"hello\")\n}",
			},
			linter: &MockLinter{
				canHandle: true,
				result: &linters.LintResult{
					Success:   true,
					Formatted: []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"),
					Issues: []linters.Issue{
						{
							Severity: "warning",
							Message:  "formatting needed",
							Rule:     "format",
						},
					},
				},
			},
			want: "approve", // Formatting issues don't block
		},
		{
			name:     "write with no issues",
			toolName: "Write",
			input: map[string]interface{}{
				"file_path": "test.go",
				"content":   "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
			},
			linter: &MockLinter{
				canHandle: true,
				result: &linters.LintResult{
					Success: true,
					Issues:  []linters.Issue{},
				},
			},
			want: "approve",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewLintingRuleEngine()

			// Replace linters with mock if provided
			if tt.linter != nil {
				engine.linters = []linters.Linter{tt.linter}
			}

			msg := &PreToolUseMessage{
				BaseHookMessage: BaseHookMessage{
					HookEventName: PreToolUseEvent,
				},
				ToolName:  tt.toolName,
				ToolInput: testConvertToRawMessage(tt.input),
			}

			ctx := context.Background()
			resp, err := engine.EvaluatePreToolUse(ctx, msg)
			if err != nil {
				t.Fatalf("EvaluatePreToolUse() error = %v", err)
			}

			if resp.Decision != tt.want {
				t.Errorf("EvaluatePreToolUse() decision = %v, want %v", resp.Decision, tt.want)
			}
		})
	}
}

func TestLintingRuleEngine_AddLinter(t *testing.T) {
	engine := NewLintingRuleEngine()
	initialCount := len(engine.linters)

	mockLinter := &MockLinter{name: "test"}
	engine.AddLinter(mockLinter)

	if len(engine.linters) != initialCount+1 {
		t.Errorf("expected %d linters, got %d", initialCount+1, len(engine.linters))
	}

	if engine.linters[len(engine.linters)-1] != mockLinter {
		t.Errorf("last linter should be the added mock linter")
	}
}

func TestLintingRuleEngine_OtherEvaluateMethods(t *testing.T) {
	engine := NewLintingRuleEngine()
	ctx := context.Background()

	// Test that other evaluate methods return nil (no-op)
	resp, err := engine.EvaluatePostToolUse(ctx, &PostToolUseMessage{})
	if err != nil {
		t.Errorf("EvaluatePostToolUse() error = %v", err)
	}
	if resp != nil {
		t.Errorf("EvaluatePostToolUse() should return nil")
	}

	resp, err = engine.EvaluateNotification(ctx, &NotificationMessage{})
	if err != nil {
		t.Errorf("EvaluateNotification() error = %v", err)
	}
	if resp != nil {
		t.Errorf("EvaluateNotification() should return nil")
	}

	resp, err = engine.EvaluateStop(ctx, &StopMessage{})
	if err != nil {
		t.Errorf("EvaluateStop() error = %v", err)
	}
	if resp != nil {
		t.Errorf("EvaluateStop() should return nil")
	}

	resp, err = engine.EvaluateSubagentStop(ctx, &SubagentStopMessage{})
	if err != nil {
		t.Errorf("EvaluateSubagentStop() error = %v", err)
	}
	if resp != nil {
		t.Errorf("EvaluateSubagentStop() should return nil")
	}

	resp, err = engine.EvaluatePreCompact(ctx, &PreCompactMessage{})
	if err != nil {
		t.Errorf("EvaluatePreCompact() error = %v", err)
	}
	if resp != nil {
		t.Errorf("EvaluatePreCompact() should return nil")
	}
}
