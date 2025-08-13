package handlers

import (
	"context"
	"testing"

	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegexHandler(t *testing.T) {
	handler := NewRegexHandler()
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.config)
	assert.Equal(t, "regex", handler.Name())
	assert.Equal(t, 150, handler.Priority())
}

func TestNewRegexHandlerWithConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *RegexConfig
	}{
		{
			name:   "nil config uses default",
			config: nil,
		},
		{
			name: "custom config",
			config: &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:    "test-rule",
						Pattern: `test`,
						Action:  "block",
						Enabled: true,
					},
				},
				CaseSensitive: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewRegexHandlerWithConfig(tt.config)
			assert.NotNil(t, handler)
			assert.NotNil(t, handler.config)
		})
	}
}

func TestRegexHandler_ShouldHandle(t *testing.T) {
	tests := []struct {
		name     string
		config   *RegexConfig
		event    engine.HookMessage
		expected bool
	}{
		{
			name: "should handle UserPromptSubmit with prompt rules",
			config: &RegexConfig{
				PromptRules: []RegexRule{{Name: "test", Pattern: "test"}},
			},
			event:    &engine.UserPromptSubmitMessage{UserPrompt: "test"},
			expected: true,
		},
		{
			name: "should not handle UserPromptSubmit without prompt rules",
			config: &RegexConfig{
				PromptRules: []RegexRule{},
			},
			event:    &engine.UserPromptSubmitMessage{UserPrompt: "test"},
			expected: false,
		},
		{
			name: "should handle PreToolUse with content rules",
			config: &RegexConfig{
				ContentRules: []RegexRule{{Name: "test", Pattern: "test"}},
			},
			event:    &engine.PreToolUseMessage{ToolName: "Write"},
			expected: true,
		},
		{
			name: "should not handle PreToolUse without content rules",
			config: &RegexConfig{
				ContentRules: []RegexRule{},
			},
			event:    &engine.PreToolUseMessage{ToolName: "Write"},
			expected: false,
		},
		{
			name:     "should not handle PostToolUse",
			config:   &RegexConfig{},
			event:    &engine.PostToolUseMessage{},
			expected: false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewRegexHandlerWithConfig(tt.config)
			result := handler.ShouldHandle(ctx, tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegexHandler_HandleUserPromptSubmit(t *testing.T) {
	tests := []struct {
		name           string
		config         *RegexConfig
		prompt         string
		expectedResult string
		expectedReason string
		expectMessage  bool
	}{
		{
			name: "block on matching pattern",
			config: &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:        "no-secrets",
						Pattern:     `password\s*[:=]\s*["\']?([^"\'\s]+)`,
						Action:      "block",
						Message:     "Password detected",
						Description: "Blocks password patterns",
						Enabled:     true,
					},
				},
				CaseSensitive: false,
			},
			prompt:         "my password: secret123",
			expectedResult: "block",
			expectedReason: "Password detected",
		},
		{
			name: "warn on matching pattern",
			config: &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:    "api-key-warning",
						Pattern: `api[_-]?key`,
						Action:  "warn",
						Message: "API key detected",
						Enabled: true,
					},
				},
				CaseSensitive: false,
			},
			prompt:         "my api_key is here",
			expectedResult: "approve",
			expectMessage:  true,
		},
		{
			name: "log action only logs",
			config: &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:        "debug-log",
						Pattern:     `debug`,
						Action:      "log",
						Description: "Debug pattern",
						Enabled:     true,
					},
				},
			},
			prompt:         "debug mode enabled",
			expectedResult: "approve",
			expectMessage:  false,
		},
		{
			name: "modify action changes prompt",
			config: &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:        "redact-emails",
						Pattern:     `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
						Action:      "modify",
						Replacement: "[REDACTED_EMAIL]",
						Enabled:     true,
					},
				},
			},
			prompt:         "contact me at test@example.com",
			expectedResult: "approve",
			expectMessage:  false,
		},
		{
			name: "disabled rule is skipped",
			config: &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:    "disabled-rule",
						Pattern: `password`,
						Action:  "block",
						Message: "Should not trigger",
						Enabled: false,
					},
				},
			},
			prompt:         "my password is here",
			expectedResult: "approve",
			expectMessage:  false,
		},
		{
			name: "case sensitive matching",
			config: &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:    "case-sensitive",
						Pattern: `PASSWORD`,
						Action:  "block",
						Message: "Uppercase PASSWORD found",
						Enabled: true,
					},
				},
				CaseSensitive: true,
			},
			prompt:         "my password is here",
			expectedResult: "approve",
		},
		{
			name: "case insensitive matching",
			config: &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:    "case-insensitive",
						Pattern: `PASSWORD`,
						Action:  "block",
						Message: "Password found",
						Enabled: true,
					},
				},
				CaseSensitive: false,
			},
			prompt:         "my password is here",
			expectedResult: "block",
			expectedReason: "Password found",
		},
		{
			name: "multiple rules - first block wins",
			config: &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:    "rule1",
						Pattern: `password`,
						Action:  "block",
						Message: "Rule 1 blocks",
						Enabled: true,
					},
					{
						Name:    "rule2",
						Pattern: `password`,
						Action:  "warn",
						Message: "Rule 2 warns",
						Enabled: true,
					},
				},
			},
			prompt:         "my password",
			expectedResult: "block",
			expectedReason: "Rule 1 blocks",
		},
		{
			name: "multiple warnings combined",
			config: &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:    "warn1",
						Pattern: `api`,
						Action:  "warn",
						Message: "API detected",
						Enabled: true,
					},
					{
						Name:    "warn2",
						Pattern: `key`,
						Action:  "warn",
						Message: "Key detected",
						Enabled: true,
					},
				},
			},
			prompt:         "api key",
			expectedResult: "approve",
			expectMessage:  true,
		},
		{
			name:           "no rules returns approve",
			config:         &RegexConfig{},
			prompt:         "any text",
			expectedResult: "approve",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewRegexHandlerWithConfig(tt.config)
			msg := &engine.UserPromptSubmitMessage{
				UserPrompt: tt.prompt,
			}

			response, err := handler.HandleUserPromptSubmit(ctx, msg)
			require.NoError(t, err)
			require.NotNil(t, response)

			assert.Equal(t, tt.expectedResult, response.Decision)
			if tt.expectedReason != "" {
				assert.Equal(t, tt.expectedReason, response.Reason)
			}
			if tt.expectMessage {
				assert.NotEmpty(t, response.Message)
			}
		})
	}
}

func TestRegexHandler_HandlePreToolUse(t *testing.T) {
	tests := []struct {
		name           string
		config         *RegexConfig
		toolName       string
		content        string
		expectedResult string
		expectedReason string
		expectMessage  bool
	}{
		{
			name: "block content with pattern",
			config: &RegexConfig{
				ContentRules: []RegexRule{
					{
						Name:    "no-todos",
						Pattern: `TODO|FIXME`,
						Action:  "block",
						Message: "TODOs not allowed in production",
						Enabled: true,
					},
				},
			},
			toolName:       "Write",
			content:        "// TODO: fix this later",
			expectedResult: "block",
			expectedReason: "TODOs not allowed in production",
		},
		{
			name: "warn on content pattern",
			config: &RegexConfig{
				ContentRules: []RegexRule{
					{
						Name:    "console-warning",
						Pattern: `console\.log`,
						Action:  "warn",
						Message: "Console logging detected",
						Enabled: true,
					},
				},
			},
			toolName:       "Write",
			content:        "console.log('debug');",
			expectedResult: "approve",
			expectMessage:  true,
		},
		{
			name: "skip non-Write tools",
			config: &RegexConfig{
				ContentRules: []RegexRule{
					{
						Name:    "test-rule",
						Pattern: `test`,
						Action:  "block",
						Message: "Should not trigger",
						Enabled: true,
					},
				},
			},
			toolName:       "Read",
			content:        "test content",
			expectedResult: "approve",
		},
		{
			name: "disabled content rule",
			config: &RegexConfig{
				ContentRules: []RegexRule{
					{
						Name:    "disabled",
						Pattern: `password`,
						Action:  "block",
						Message: "Should not trigger",
						Enabled: false,
					},
				},
			},
			toolName:       "Write",
			content:        "password = 'secret'",
			expectedResult: "approve",
		},
		{
			name:           "no content rules",
			config:         &RegexConfig{},
			toolName:       "Write",
			content:        "any content",
			expectedResult: "approve",
		},
		{
			name: "invalid regex pattern handled gracefully",
			config: &RegexConfig{
				ContentRules: []RegexRule{
					{
						Name:    "invalid",
						Pattern: `[`, // Invalid regex
						Action:  "block",
						Message: "Should not crash",
						Enabled: true,
					},
				},
			},
			toolName:       "Write",
			content:        "test content",
			expectedResult: "approve",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewRegexHandlerWithConfig(tt.config)

			contentBytes, _ := json.Marshal(tt.content)
			msg := &engine.PreToolUseMessage{
				ToolName: tt.toolName,
				ToolInput: map[string]json.RawMessage{
					"content": contentBytes,
				},
			}

			response, err := handler.HandlePreToolUse(ctx, msg)
			require.NoError(t, err)
			require.NotNil(t, response)

			assert.Equal(t, tt.expectedResult, response.Decision)
			if tt.expectedReason != "" {
				assert.Equal(t, tt.expectedReason, response.Reason)
			}
			if tt.expectMessage {
				assert.NotEmpty(t, response.Message)
			}
		})
	}
}

func TestRegexHandler_ExtractContent(t *testing.T) {
	handler := NewRegexHandler()

	tests := []struct {
		name        string
		toolInput   map[string]json.RawMessage
		wantContent string
		wantError   bool
	}{
		{
			name: "valid content",
			toolInput: map[string]json.RawMessage{
				"content": json.RawMessage(`"Hello, World!"`),
			},
			wantContent: "Hello, World!",
			wantError:   false,
		},
		{
			name:      "missing content",
			toolInput: map[string]json.RawMessage{},
			wantError: true,
		},
		{
			name: "invalid JSON",
			toolInput: map[string]json.RawMessage{
				"content": json.RawMessage(`not-valid-json`),
			},
			wantError: true,
		},
		{
			name: "multiline content",
			toolInput: map[string]json.RawMessage{
				"content": json.RawMessage(`"Line 1\nLine 2\nLine 3"`),
			},
			wantContent: "Line 1\nLine 2\nLine 3",
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &engine.PreToolUseMessage{
				ToolInput: tt.toolInput,
			}

			content, err := handler.extractContent(msg)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantContent, content)
			}
		})
	}
}

func TestRegexHandler_ConfigManagement(t *testing.T) {
	handler := NewRegexHandler()

	t.Run("SetConfig with nil returns error", func(t *testing.T) {
		err := handler.SetConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("SetConfig with valid config", func(t *testing.T) {
		config := &RegexConfig{
			CaseSensitive: true,
			PromptRules:   []RegexRule{},
		}
		err := handler.SetConfig(config)
		assert.NoError(t, err)
		assert.Equal(t, config, handler.config)
	})

	t.Run("AddPromptRule", func(t *testing.T) {
		handler := NewRegexHandler()
		initialCount := len(handler.config.PromptRules)

		rule := RegexRule{
			Name:    "test-rule",
			Pattern: `test`,
			Action:  "block",
			Enabled: true,
		}

		handler.AddPromptRule(rule)
		assert.Len(t, handler.config.PromptRules, initialCount+1)
		assert.Contains(t, handler.config.PromptRules, rule)
	})

	t.Run("AddContentRule", func(t *testing.T) {
		handler := NewRegexHandler()
		initialCount := len(handler.config.ContentRules)

		rule := RegexRule{
			Name:    "test-rule",
			Pattern: `test`,
			Action:  "block",
			Enabled: true,
		}

		handler.AddContentRule(rule)
		assert.Len(t, handler.config.ContentRules, initialCount+1)
		assert.Contains(t, handler.config.ContentRules, rule)
	})

	t.Run("RemovePromptRule", func(t *testing.T) {
		handler := NewRegexHandler()
		rule := RegexRule{
			Name:    "test-rule",
			Pattern: `test`,
			Action:  "block",
		}
		handler.AddPromptRule(rule)

		handler.RemovePromptRule("test-rule")
		for _, r := range handler.config.PromptRules {
			assert.NotEqual(t, "test-rule", r.Name)
		}
	})

	t.Run("RemoveContentRule", func(t *testing.T) {
		handler := NewRegexHandler()
		rule := RegexRule{
			Name:    "test-rule",
			Pattern: `test`,
			Action:  "block",
		}
		handler.AddContentRule(rule)

		handler.RemoveContentRule("test-rule")
		for _, r := range handler.config.ContentRules {
			assert.NotEqual(t, "test-rule", r.Name)
		}
	})

	t.Run("EnableRule", func(t *testing.T) {
		handler := NewRegexHandler()

		// Add disabled rules
		promptRule := RegexRule{
			Name:    "prompt-rule",
			Pattern: `test`,
			Enabled: false,
		}
		contentRule := RegexRule{
			Name:    "content-rule",
			Pattern: `test`,
			Enabled: false,
		}
		handler.AddPromptRule(promptRule)
		handler.AddContentRule(contentRule)

		// Enable prompt rule
		handler.EnableRule("prompt-rule")
		for _, r := range handler.config.PromptRules {
			if r.Name == "prompt-rule" {
				assert.True(t, r.Enabled)
			}
		}

		// Enable content rule
		handler.EnableRule("content-rule")
		for _, r := range handler.config.ContentRules {
			if r.Name == "content-rule" {
				assert.True(t, r.Enabled)
			}
		}
	})

	t.Run("DisableRule", func(t *testing.T) {
		handler := NewRegexHandler()

		// Add enabled rules
		promptRule := RegexRule{
			Name:    "prompt-rule",
			Pattern: `test`,
			Enabled: true,
		}
		contentRule := RegexRule{
			Name:    "content-rule",
			Pattern: `test`,
			Enabled: true,
		}
		handler.AddPromptRule(promptRule)
		handler.AddContentRule(contentRule)

		// Disable prompt rule
		handler.DisableRule("prompt-rule")
		for _, r := range handler.config.PromptRules {
			if r.Name == "prompt-rule" {
				assert.False(t, r.Enabled)
			}
		}

		// Disable content rule
		handler.DisableRule("content-rule")
		for _, r := range handler.config.ContentRules {
			if r.Name == "content-rule" {
				assert.False(t, r.Enabled)
			}
		}
	})
}

func TestDefaultRegexConfig(t *testing.T) {
	config := DefaultRegexConfig()

	assert.NotNil(t, config)
	assert.False(t, config.CaseSensitive)

	// Check default prompt rules
	assert.Greater(t, len(config.PromptRules), 0)

	// Verify password rule exists
	hasPasswordRule := false
	for _, r := range config.PromptRules {
		if r.Name == "no-passwords" {
			hasPasswordRule = true
			assert.Equal(t, "warn", r.Action)
			assert.True(t, r.Enabled)
			break
		}
	}
	assert.True(t, hasPasswordRule, "Should have password rule")

	// Verify API key rule exists
	hasAPIKeyRule := false
	for _, r := range config.PromptRules {
		if r.Name == "no-api-keys" {
			hasAPIKeyRule = true
			assert.Equal(t, "warn", r.Action)
			assert.True(t, r.Enabled)
			break
		}
	}
	assert.True(t, hasAPIKeyRule, "Should have API key rule")

	// Content rules should be empty by default
	assert.Empty(t, config.ContentRules)
}

func TestRegexHandler_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid regex in prompt rule", func(t *testing.T) {
		config := &RegexConfig{
			PromptRules: []RegexRule{
				{
					Name:    "invalid",
					Pattern: `[`, // Invalid regex
					Action:  "block",
					Message: "Should not crash",
					Enabled: true,
				},
			},
		}
		handler := NewRegexHandlerWithConfig(config)

		msg := &engine.UserPromptSubmitMessage{
			UserPrompt: "test prompt",
		}

		// Should not crash, should approve
		response, err := handler.HandleUserPromptSubmit(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "approve", response.Decision)
	})

	t.Run("missing content in PreToolUse", func(t *testing.T) {
		config := &RegexConfig{
			ContentRules: []RegexRule{
				{
					Name:    "test",
					Pattern: `test`,
					Action:  "block",
					Enabled: true,
				},
			},
		}
		handler := NewRegexHandlerWithConfig(config)

		msg := &engine.PreToolUseMessage{
			ToolName:  "Write",
			ToolInput: map[string]json.RawMessage{},
		}

		response, err := handler.HandlePreToolUse(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "approve", response.Decision)
	})

	t.Run("empty pattern matches everything", func(t *testing.T) {
		config := &RegexConfig{
			PromptRules: []RegexRule{
				{
					Name:    "empty",
					Pattern: ``, // Empty pattern matches everything
					Action:  "warn",
					Message: "Empty pattern warning",
					Enabled: true,
				},
			},
		}
		handler := NewRegexHandlerWithConfig(config)

		msg := &engine.UserPromptSubmitMessage{
			UserPrompt: "any text",
		}

		response, err := handler.HandleUserPromptSubmit(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "approve", response.Decision)
		assert.NotEmpty(t, response.Message)
	})

	t.Run("modify action with empty replacement", func(t *testing.T) {
		config := &RegexConfig{
			PromptRules: []RegexRule{
				{
					Name:        "remove-pattern",
					Pattern:     `remove`,
					Action:      "modify",
					Replacement: "", // Empty replacement removes matches
					Enabled:     true,
				},
			},
		}
		handler := NewRegexHandlerWithConfig(config)

		msg := &engine.UserPromptSubmitMessage{
			UserPrompt: "remove this text",
		}

		response, err := handler.HandleUserPromptSubmit(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "approve", response.Decision)
	})
}

func TestRegexHandler_Priority(t *testing.T) {
	handler := NewRegexHandler()

	// Regex handler should have medium priority
	assert.Equal(t, 150, handler.Priority())
	assert.Equal(t, "regex", handler.Name())
}

func TestRegexHandler_ComplexPatterns(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		pattern       string
		testStrings   []string
		shouldMatch   []bool
		caseSensitive bool
	}{
		{
			name:    "email pattern",
			pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
			testStrings: []string{
				"test@example.com",
				"user.name+tag@domain.co.uk",
				"not an email",
				"@invalid.com",
			},
			shouldMatch: []bool{true, true, false, false},
		},
		{
			name:    "URL pattern",
			pattern: `https?://[^\s]+`,
			testStrings: []string{
				"Visit https://example.com for more",
				"http://localhost:8080/path",
				"ftp://not-http.com",
				"just text",
			},
			shouldMatch: []bool{true, true, false, false},
		},
		{
			name:    "IP address pattern",
			pattern: `\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`,
			testStrings: []string{
				"Server at 192.168.1.1",
				"Invalid 999.999.999.999",
				"Port 8080",
			},
			shouldMatch: []bool{true, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RegexConfig{
				PromptRules: []RegexRule{
					{
						Name:    "test-pattern",
						Pattern: tt.pattern,
						Action:  "warn",
						Message: "Pattern matched",
						Enabled: true,
					},
				},
				CaseSensitive: tt.caseSensitive,
			}
			handler := NewRegexHandlerWithConfig(config)

			for i, testStr := range tt.testStrings {
				msg := &engine.UserPromptSubmitMessage{
					UserPrompt: testStr,
				}

				response, err := handler.HandleUserPromptSubmit(ctx, msg)
				assert.NoError(t, err)

				if tt.shouldMatch[i] {
					assert.NotEmpty(t, response.Message, "Expected match for: %s", testStr)
				} else {
					assert.Empty(t, response.Message, "Expected no match for: %s", testStr)
				}
			}
		})
	}
}
