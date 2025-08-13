package handlers

import (
	"context"
	"testing"

	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo/pkg/engine"
	"github.com/jrossi/gismo/pkg/linters"
	"github.com/stretchr/testify/assert"
)

func TestNewLintingHandler(t *testing.T) {
	handler := NewLintingHandler()
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.linters)
	assert.NotNil(t, handler.executor)
	assert.NotNil(t, handler.config)
	assert.Equal(t, "linting", handler.Name())
	assert.Equal(t, 100, handler.Priority())

	// Should have default linters
	assert.Greater(t, len(handler.linters), 0)
}

func TestNewLintingHandlerWithConfig(t *testing.T) {
	tests := []struct {
		name   string
		config LintingConfig
	}{
		{
			name:   "default config",
			config: LintingConfig{},
		},
		{
			name: "with max workers",
			config: LintingConfig{
				MaxWorkers: 4,
			},
		},
		{
			name: "disable parallel",
			config: LintingConfig{
				DisableParallel: true,
			},
		},
		{
			name: "max workers with parallel disabled",
			config: LintingConfig{
				MaxWorkers:      8,
				DisableParallel: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewLintingHandlerWithConfig(tt.config)
			assert.NotNil(t, handler)
			assert.NotNil(t, handler.executor)
		})
	}
}

func TestLintingHandler_ShouldHandle(t *testing.T) {
	handler := NewLintingHandler()
	ctx := context.Background()

	tests := []struct {
		name     string
		event    engine.HookMessage
		expected bool
	}{
		{
			name: "should handle PreToolUse Write",
			event: &engine.PreToolUseMessage{
				ToolName: "Write",
			},
			expected: true,
		},
		{
			name: "should handle PreToolUse Edit",
			event: &engine.PreToolUseMessage{
				ToolName: "Edit",
			},
			expected: true,
		},
		{
			name: "should handle PreToolUse MultiEdit",
			event: &engine.PreToolUseMessage{
				ToolName: "MultiEdit",
			},
			expected: true,
		},
		{
			name: "should handle PostToolUse Write",
			event: &engine.PostToolUseMessage{
				ToolName: "Write",
			},
			expected: true,
		},
		{
			name: "should handle PostToolUse Edit",
			event: &engine.PostToolUseMessage{
				ToolName: "Edit",
			},
			expected: true,
		},
		{
			name: "should handle PostToolUse MultiEdit",
			event: &engine.PostToolUseMessage{
				ToolName: "MultiEdit",
			},
			expected: true,
		},
		{
			name: "should not handle PreToolUse Read",
			event: &engine.PreToolUseMessage{
				ToolName: "Read",
			},
			expected: false,
		},
		{
			name: "should not handle PostToolUse Bash",
			event: &engine.PostToolUseMessage{
				ToolName: "Bash",
			},
			expected: false,
		},
		{
			name: "should not handle UserPromptSubmit",
			event: &engine.UserPromptSubmitMessage{
				UserPrompt: "test",
			},
			expected: false,
		},
		{
			name: "should not handle NotificationMessage",
			event: &engine.NotificationMessage{
				NotificationType: "test",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.ShouldHandle(ctx, tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLintingHandler_HandlePreToolUse(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		filePath       string
		content        string
		expectedResult string
		expectReason   bool
		expectMessage  bool
	}{
		{
			name:           "approve non-Write operations",
			toolName:       "Edit",
			filePath:       "/test/file.go",
			content:        "package main",
			expectedResult: "approve",
		},
		{
			name:           "approve when no file path",
			toolName:       "Write",
			filePath:       "",
			content:        "test content",
			expectedResult: "approve",
		},
		{
			name:           "approve when no content",
			toolName:       "Write",
			filePath:       "/test/file.go",
			content:        "",
			expectedResult: "approve",
		},
		{
			name:           "approve temporary test files",
			toolName:       "Write",
			filePath:       "/tmp/test_file.go",
			content:        "package main",
			expectedResult: "approve",
		},
		{
			name:           "approve valid Go code",
			toolName:       "Write",
			filePath:       "/test/valid.go",
			content:        "package main\n\nfunc main() {}\n",
			expectedResult: "approve",
		},
		{
			name:           "approve valid JSON",
			toolName:       "Write",
			filePath:       "/test/config.json",
			content:        `{"key": "value"}`,
			expectedResult: "approve",
		},
		{
			name:           "handle invalid JSON",
			toolName:       "Write",
			filePath:       "/test/invalid.json",
			content:        `{"key": "value"`,
			expectedResult: "block", // JSON linter blocks invalid syntax
			expectReason:   true,
		},
		{
			name:           "approve markdown",
			toolName:       "Write",
			filePath:       "/test/README.md",
			content:        "# Title\n\nContent here.\n",
			expectedResult: "approve",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewLintingHandler()

			var toolInput map[string]json.RawMessage
			if tt.filePath != "" || tt.content != "" {
				toolInput = make(map[string]json.RawMessage)
				if tt.filePath != "" {
					filePathBytes, _ := json.Marshal(tt.filePath)
					toolInput["file_path"] = filePathBytes
				}
				if tt.content != "" {
					contentBytes, _ := json.Marshal(tt.content)
					toolInput["content"] = contentBytes
				}
			}

			msg := &engine.PreToolUseMessage{
				ToolName:  tt.toolName,
				ToolInput: toolInput,
			}

			response, err := handler.HandlePreToolUse(ctx, msg)
			assert.NoError(t, err)
			assert.NotNil(t, response)
			assert.Equal(t, tt.expectedResult, response.Decision)

			if tt.expectReason {
				assert.NotEmpty(t, response.Reason)
			}
			if tt.expectMessage {
				assert.NotEmpty(t, response.Message)
			}
		})
	}
}

func TestLintingHandler_HandlePostToolUse(t *testing.T) {
	tests := []struct {
		name       string
		toolName   string
		filePath   string
		toolError  string
		fileExists bool
		expectNil  bool
	}{
		{
			name:      "skip on tool error",
			toolName:  "Write",
			filePath:  "/test/file.go",
			toolError: "write failed",
			expectNil: true,
		},
		{
			name:      "skip when no file path",
			toolName:  "Write",
			filePath:  "",
			expectNil: true,
		},
		{
			name:      "skip temporary test files",
			toolName:  "Write",
			filePath:  "/tmp/test.go",
			expectNil: true,
		},
		{
			name:       "handle file not found",
			toolName:   "Write",
			filePath:   "/nonexistent/file.go",
			fileExists: false,
			expectNil:  true,
		},
		{
			name:       "process existing file",
			toolName:   "Edit",
			filePath:   "/test/existing.go",
			fileExists: true,
			expectNil:  true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewLintingHandler()

			var toolInput map[string]json.RawMessage
			if tt.filePath != "" {
				toolInput = make(map[string]json.RawMessage)
				filePathBytes, _ := json.Marshal(tt.filePath)
				toolInput["file_path"] = filePathBytes
			}

			msg := &engine.PostToolUseMessage{
				ToolName:  tt.toolName,
				ToolInput: toolInput,
				ToolError: tt.toolError,
			}

			response, err := handler.HandlePostToolUse(ctx, msg)
			assert.NoError(t, err)

			if tt.expectNil {
				assert.Nil(t, response)
			}
		})
	}
}

func TestLintingHandler_SetConfig(t *testing.T) {
	handler := NewLintingHandler()

	t.Run("set nil config", func(t *testing.T) {
		// Should not panic
		handler.SetConfig(nil)
		assert.NotNil(t, handler.linters)
	})

	t.Run("set valid config", func(t *testing.T) {
		config := engine.NewAppConfig()
		handler.SetConfig(config)
		assert.Equal(t, config, handler.config)
	})

	t.Run("disable linter in config", func(t *testing.T) {
		config := engine.NewAppConfig()
		// This would normally disable a linter
		handler.SetConfig(config)
		assert.NotNil(t, handler.config)
	})
}

func TestLintingHandler_IsTemporaryTestFile(t *testing.T) {
	handler := NewLintingHandler()

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{
			name:     "success_test.go without path",
			filePath: "success_test.go",
			expected: true,
		},
		{
			name:     "success_test.md without path",
			filePath: "success_test.md",
			expected: true,
		},
		{
			name:     "large_test.go without path",
			filePath: "large_test.go",
			expected: true,
		},
		{
			name:     "concurrent_test.go without path",
			filePath: "concurrent_test.go",
			expected: true,
		},
		{
			name:     "regular file with path",
			filePath: "/home/user/main.go",
			expected: false,
		},
		{
			name:     "test file with path",
			filePath: "/home/user/main_test.go",
			expected: false,
		},
		{
			name:     "success_test.go with path",
			filePath: "/tmp/success_test.go",
			expected: false,
		},
		{
			name:     "regular file without path",
			filePath: "main.go",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.isTemporaryTestFile(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLintingHandler_ApplyRuleOverrides(t *testing.T) {
	handler := NewLintingHandler()

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "Go file",
			filePath: "/test/main.go",
		},
		{
			name:     "JavaScript file",
			filePath: "/test/app.js",
		},
		{
			name:     "Python file",
			filePath: "/test/script.py",
		},
		{
			name:     "generated file",
			filePath: "/test/file_gen.go",
		},
		{
			name:     "protobuf generated",
			filePath: "/test/api.pb.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			handler.applyRuleOverrides(tt.filePath)
			assert.NotNil(t, handler.linters)
		})
	}
}

func TestLintingHandler_FormatLintOutput(t *testing.T) {
	handler := NewLintingHandler()

	tests := []struct {
		name     string
		filePath string
		issues   []linters.Issue
		isError  bool
	}{
		{
			name:     "format error issues",
			filePath: "/test/file.go",
			issues: []linters.Issue{
				{
					Line:     10,
					Column:   5,
					Rule:     "syntax-error",
					Severity: "error",
					Message:  "Unexpected token",
				},
			},
			isError: true,
		},
		{
			name:     "format warning issues",
			filePath: "/test/file.go",
			issues: []linters.Issue{
				{
					Line:     20,
					Column:   1,
					Rule:     "gofmt",
					Severity: "warning",
					Message:  "File is not properly formatted",
				},
			},
			isError: false,
		},
		{
			name:     "multiple issues",
			filePath: "/test/file.py",
			issues: []linters.Issue{
				{
					Line:     5,
					Column:   1,
					Rule:     "indentation",
					Severity: "error",
					Message:  "Unexpected indent",
				},
				{
					Line:     10,
					Column:   8,
					Rule:     "unused-variable",
					Severity: "warning",
					Message:  "Variable 'x' is never used",
				},
			},
			isError: true,
		},
		{
			name:     "empty issues",
			filePath: "/test/file.js",
			issues:   []linters.Issue{},
			isError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := handler.formatLintOutput(tt.filePath, tt.issues, tt.isError)
			assert.NotEmpty(t, output)

			// Check that file path is included
			assert.Contains(t, output, tt.filePath)

			// Check for issue content
			for _, issue := range tt.issues {
				assert.Contains(t, output, issue.Message)
			}

			// Check for error/warning indicators
			if tt.isError && len(tt.issues) > 0 {
				assert.Contains(t, output, "❌")
			}
		})
	}
}

func TestLintingHandler_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("handle invalid JSON in tool input", func(t *testing.T) {
		handler := NewLintingHandler()

		msg := &engine.PreToolUseMessage{
			ToolName: "Write",
			ToolInput: map[string]json.RawMessage{
				"file_path": json.RawMessage(`not-valid-json`),
				"content":   json.RawMessage(`"test"`),
			},
		}

		response, err := handler.HandlePreToolUse(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "approve", response.Decision)
	})

	t.Run("handle malformed file path", func(t *testing.T) {
		handler := NewLintingHandler()

		filePathBytes, _ := json.Marshal("")
		contentBytes, _ := json.Marshal("test content")

		msg := &engine.PreToolUseMessage{
			ToolName: "Write",
			ToolInput: map[string]json.RawMessage{
				"file_path": filePathBytes,
				"content":   contentBytes,
			},
		}

		response, err := handler.HandlePreToolUse(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "approve", response.Decision)
	})

	t.Run("handle empty tool input", func(t *testing.T) {
		handler := NewLintingHandler()

		msg := &engine.PreToolUseMessage{
			ToolName:  "Write",
			ToolInput: nil,
		}

		response, err := handler.HandlePreToolUse(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "approve", response.Decision)
	})

	t.Run("handle PostToolUse with invalid path", func(t *testing.T) {
		handler := NewLintingHandler()

		msg := &engine.PostToolUseMessage{
			ToolName: "Write",
			ToolInput: map[string]json.RawMessage{
				"file_path": json.RawMessage(`not-valid-json`),
			},
		}

		response, err := handler.HandlePostToolUse(ctx, msg)
		assert.NoError(t, err)
		assert.Nil(t, response)
	})
}

func TestLintingHandler_Priority(t *testing.T) {
	handler := NewLintingHandler()

	// Linting handler should have medium priority
	assert.Equal(t, 100, handler.Priority())
	assert.Equal(t, "linting", handler.Name())
}

func TestLintingHandler_LinterInitialization(t *testing.T) {
	handler := NewLintingHandler()

	// Should have initialized default linters
	assert.Greater(t, len(handler.linters), 0)

	// Check for specific linters by name
	linterNames := make(map[string]bool)
	for _, linter := range handler.linters {
		linterNames[linter.Name()] = true
	}

	// Verify key linters are present
	expectedLinters := []string{
		"go",
		"javascript",
		"json",
		"markdown",
		"python",
		"rust",
		"protobuf",
	}

	for _, expected := range expectedLinters {
		assert.True(t, linterNames[expected], "Should have %s linter", expected)
	}
}
