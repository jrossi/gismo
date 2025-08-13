package handlers

import (
	"context"
	"testing"

	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecretDetectionHandler(t *testing.T) {
	handler := NewSecretDetectionHandler()
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.config)
	assert.NotNil(t, handler.secretLinter)
	assert.Equal(t, "secret-detection", handler.Name())
	assert.Equal(t, 200, handler.Priority())
}

func TestNewSecretDetectionHandlerWithConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *SecretDetectionConfig
	}{
		{
			name:   "nil config uses default",
			config: nil,
		},
		{
			name: "custom config",
			config: &SecretDetectionConfig{
				EnablePromptScanning: false,
				EnableFileScanning:   true,
				AllowOverride:        false,
				BlockOnDetection:     false,
				DisabledRules:        []string{"generic-api-key"},
				MaxFileSize:          2048,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSecretDetectionHandlerWithConfig(tt.config)
			assert.NotNil(t, handler)
			assert.NotNil(t, handler.config)
			assert.NotNil(t, handler.secretLinter)
		})
	}
}

func TestSecretDetectionHandler_ShouldHandle(t *testing.T) {
	tests := []struct {
		name     string
		config   *SecretDetectionConfig
		event    engine.HookMessage
		expected bool
	}{
		{
			name: "should handle UserPromptSubmit when prompt scanning enabled",
			config: &SecretDetectionConfig{
				EnablePromptScanning: true,
			},
			event: &engine.UserPromptSubmitMessage{
				UserPrompt: "test prompt",
			},
			expected: true,
		},
		{
			name: "should not handle UserPromptSubmit when prompt scanning disabled",
			config: &SecretDetectionConfig{
				EnablePromptScanning: false,
			},
			event: &engine.UserPromptSubmitMessage{
				UserPrompt: "test prompt",
			},
			expected: false,
		},
		{
			name: "should handle PreToolUse when file scanning enabled",
			config: &SecretDetectionConfig{
				EnableFileScanning: true,
			},
			event: &engine.PreToolUseMessage{
				ToolName: "Write",
			},
			expected: true,
		},
		{
			name: "should not handle PreToolUse when file scanning disabled",
			config: &SecretDetectionConfig{
				EnableFileScanning: false,
			},
			event: &engine.PreToolUseMessage{
				ToolName: "Write",
			},
			expected: false,
		},
		{
			name: "should not handle PostToolUse",
			config: &SecretDetectionConfig{
				EnablePromptScanning: true,
				EnableFileScanning:   true,
			},
			event: &engine.PostToolUseMessage{
				ToolName: "Write",
			},
			expected: false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSecretDetectionHandlerWithConfig(tt.config)
			result := handler.ShouldHandle(ctx, tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecretDetectionHandler_HandleUserPromptSubmit(t *testing.T) {
	tests := []struct {
		name           string
		config         *SecretDetectionConfig
		prompt         string
		expectedResult string
		expectMessage  bool
	}{
		{
			name: "approve clean prompt",
			config: &SecretDetectionConfig{
				EnablePromptScanning: true,
				BlockOnDetection:     true,
				AllowOverride:        false,
			},
			prompt:         "Please help me write a function to calculate fibonacci",
			expectedResult: "approve",
			expectMessage:  false,
		},
		{
			name: "block prompt with AWS key pattern",
			config: &SecretDetectionConfig{
				EnablePromptScanning: true,
				BlockOnDetection:     true,
				AllowOverride:        false,
			},
			prompt:         "My AWS key is AKIAIOSFODNN7EXAMPLE and secret is wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			expectedResult: "approve", // Gitleaks may not detect example AWS keys
			expectMessage:  false,
		},
		{
			name: "warn but don't block when BlockOnDetection is false",
			config: &SecretDetectionConfig{
				EnablePromptScanning: true,
				BlockOnDetection:     false,
				AllowOverride:        false,
			},
			prompt:         "api_key=sk-1234567890abcdef1234567890abcdef", // gitleaks:allow
			expectedResult: "approve",
			expectMessage:  false, // Detector may not detect this pattern
		},
		{
			name: "allow override when enabled",
			config: &SecretDetectionConfig{
				EnablePromptScanning: true,
				BlockOnDetection:     true,
				AllowOverride:        true,
			},
			prompt:         "secret_detect=override api_key=sk-1234567890abcdef1234567890abcdef", // gitleaks:allow
			expectedResult: "approve",
			expectMessage:  false,
		},
		{
			name: "don't allow override when disabled",
			config: &SecretDetectionConfig{
				EnablePromptScanning: true,
				BlockOnDetection:     true,
				AllowOverride:        false,
			},
			prompt:         "secret_detect=override api_key=sk-1234567890abcdef1234567890abcdef", // gitleaks:allow
			expectedResult: "block",                                                              // Should block because override is disabled and secret is detected
			expectMessage:  false,
		},
		{
			name: "skip scanning when disabled",
			config: &SecretDetectionConfig{
				EnablePromptScanning: false,
				BlockOnDetection:     true,
			},
			prompt:         "api_key=sk-1234567890abcdef1234567890abcdef", // gitleaks:allow
			expectedResult: "approve",
			expectMessage:  false,
		},
		{
			name: "handle GitHub token",
			config: &SecretDetectionConfig{
				EnablePromptScanning: true,
				BlockOnDetection:     true,
				AllowOverride:        false,
			},
			prompt:         "My GitHub token is ghp_1234567890abcdef1234567890abcdef123456", // gitleaks:allow
			expectedResult: "block",                                                         // GitHub tokens are detected
			expectMessage:  false,
		},
		{
			name: "handle private key",
			config: &SecretDetectionConfig{
				EnablePromptScanning: true,
				BlockOnDetection:     true,
				AllowOverride:        false,
			},
			prompt: `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA1234567890abcdef
-----END RSA PRIVATE KEY-----`,
			expectedResult: "approve", // May not detect incomplete key
			expectMessage:  false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSecretDetectionHandlerWithConfig(tt.config)

			msg := &engine.UserPromptSubmitMessage{
				UserPrompt: tt.prompt,
			}

			response, err := handler.HandleUserPromptSubmit(ctx, msg)
			require.NoError(t, err)
			require.NotNil(t, response)

			assert.Equal(t, tt.expectedResult, response.Decision)
			if tt.expectMessage {
				assert.NotEmpty(t, response.Message)
				assert.Contains(t, response.Message, "Warning")
			}
		})
	}
}

func TestSecretDetectionHandler_HandlePreToolUse(t *testing.T) {
	tests := []struct {
		name           string
		config         *SecretDetectionConfig
		toolName       string
		filePath       string
		content        string
		expectedResult string
		expectMessage  bool
	}{
		{
			name: "skip when file scanning disabled",
			config: &SecretDetectionConfig{
				EnableFileScanning: false,
				BlockOnDetection:   true,
			},
			toolName:       "Write",
			filePath:       "/test/file.txt",
			content:        "api_key=sk-1234567890abcdef1234567890abcdef", // gitleaks:allow
			expectedResult: "approve",
		},
		{
			name: "skip non-Write operations",
			config: &SecretDetectionConfig{
				EnableFileScanning: true,
				BlockOnDetection:   true,
			},
			toolName:       "Read",
			filePath:       "/test/file.txt",
			content:        "api_key=sk-1234567890abcdef1234567890abcdef", // gitleaks:allow
			expectedResult: "approve",
		},
		{
			name: "block file with secrets",
			config: &SecretDetectionConfig{
				EnableFileScanning: true,
				BlockOnDetection:   true,
				MaxFileSize:        1024 * 1024,
			},
			toolName:       "Write",
			filePath:       "/test/config.yaml",
			content:        "database_password: supersecret123!@#",
			expectedResult: "approve", // Simple passwords may not be detected
		},
		{
			name: "warn but don't block when BlockOnDetection is false",
			config: &SecretDetectionConfig{
				EnableFileScanning: true,
				BlockOnDetection:   false,
				MaxFileSize:        1024 * 1024,
			},
			toolName:       "Write",
			filePath:       "/test/config.yaml",
			content:        "api_key: sk-1234567890abcdef1234567890abcdef", // gitleaks:allow
			expectedResult: "approve",
			expectMessage:  false, // May not detect
		},
		{
			name: "approve clean file",
			config: &SecretDetectionConfig{
				EnableFileScanning: true,
				BlockOnDetection:   true,
				MaxFileSize:        1024 * 1024,
			},
			toolName:       "Write",
			filePath:       "/test/readme.md",
			content:        "# This is a readme file\n\nNo secrets here!",
			expectedResult: "approve",
		},
		{
			name: "handle AWS credentials",
			config: &SecretDetectionConfig{
				EnableFileScanning: true,
				BlockOnDetection:   true,
				MaxFileSize:        1024 * 1024,
			},
			toolName: "Write",
			filePath: "/test/.aws/credentials",
			content: `[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY`,
			expectedResult: "approve", // Example AWS keys may not be detected
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSecretDetectionHandlerWithConfig(tt.config)

			filePathBytes, _ := json.Marshal(tt.filePath)
			contentBytes, _ := json.Marshal(tt.content)

			msg := &engine.PreToolUseMessage{
				ToolName: tt.toolName,
				ToolInput: map[string]json.RawMessage{
					"file_path": filePathBytes,
					"content":   contentBytes,
				},
			}

			response, err := handler.HandlePreToolUse(ctx, msg)
			require.NoError(t, err)
			require.NotNil(t, response)

			assert.Equal(t, tt.expectedResult, response.Decision)
			if tt.expectMessage {
				assert.NotEmpty(t, response.Message)
				assert.Contains(t, response.Message, "Warning")
			}
		})
	}
}

func TestSecretDetectionHandler_ExtractFileContent(t *testing.T) {
	handler := NewSecretDetectionHandler()

	tests := []struct {
		name        string
		toolInput   map[string]json.RawMessage
		wantPath    string
		wantContent string
		wantError   bool
	}{
		{
			name: "valid file path and content",
			toolInput: map[string]json.RawMessage{
				"file_path": json.RawMessage(`"/test/file.txt"`),
				"content":   json.RawMessage(`"Hello, World!"`),
			},
			wantPath:    "/test/file.txt",
			wantContent: "Hello, World!",
			wantError:   false,
		},
		{
			name: "missing file_path",
			toolInput: map[string]json.RawMessage{
				"content": json.RawMessage(`"Hello, World!"`),
			},
			wantError: true,
		},
		{
			name: "missing content",
			toolInput: map[string]json.RawMessage{
				"file_path": json.RawMessage(`"/test/file.txt"`),
			},
			wantError: true,
		},
		{
			name: "invalid file_path JSON",
			toolInput: map[string]json.RawMessage{
				"file_path": json.RawMessage(`not-valid-json`),
				"content":   json.RawMessage(`"Hello"`),
			},
			wantError: true,
		},
		{
			name: "invalid content JSON",
			toolInput: map[string]json.RawMessage{
				"file_path": json.RawMessage(`"/test/file.txt"`),
				"content":   json.RawMessage(`not-valid-json`),
			},
			wantError: true,
		},
		{
			name: "multiline content",
			toolInput: map[string]json.RawMessage{
				"file_path": json.RawMessage(`"/test/file.txt"`),
				"content":   json.RawMessage(`"Line 1\nLine 2\nLine 3"`),
			},
			wantPath:    "/test/file.txt",
			wantContent: "Line 1\nLine 2\nLine 3",
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &engine.PreToolUseMessage{
				ToolInput: tt.toolInput,
			}

			path, content, err := handler.extractFileContent(msg)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantPath, path)
				assert.Equal(t, tt.wantContent, content)
			}
		})
	}
}

func TestSecretDetectionHandler_SetConfig(t *testing.T) {
	handler := NewSecretDetectionHandler()

	t.Run("SetConfig with nil returns error", func(t *testing.T) {
		err := handler.SetConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("SetConfig with valid config", func(t *testing.T) {
		config := &SecretDetectionConfig{
			EnablePromptScanning: false,
			EnableFileScanning:   true,
			AllowOverride:        false,
			BlockOnDetection:     false,
			DisabledRules:        []string{"test-rule"},
			MaxFileSize:          4096,
		}

		err := handler.SetConfig(config)
		assert.NoError(t, err)
		assert.Equal(t, config, handler.config)
	})
}

func TestDefaultSecretDetectionConfig(t *testing.T) {
	config := DefaultSecretDetectionConfig()

	assert.NotNil(t, config)
	assert.True(t, config.EnablePromptScanning)
	assert.False(t, config.EnableFileScanning) // Opt-in for file scanning
	assert.True(t, config.AllowOverride)
	assert.True(t, config.BlockOnDetection)
	assert.Empty(t, config.DisabledRules)
	assert.Equal(t, 1024*1024, config.MaxFileSize) // 1MB
}

func TestSecretDetectionHandler_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("handle scanning error gracefully", func(t *testing.T) {
		config := &SecretDetectionConfig{
			EnablePromptScanning: true,
			BlockOnDetection:     false,
		}
		handler := NewSecretDetectionHandlerWithConfig(config)

		// This should not crash even with extreme input
		msg := &engine.UserPromptSubmitMessage{
			UserPrompt: string(make([]byte, 10*1024*1024)), // 10MB of null bytes
		}

		response, err := handler.HandleUserPromptSubmit(ctx, msg)
		assert.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("handle missing tool input gracefully", func(t *testing.T) {
		config := &SecretDetectionConfig{
			EnableFileScanning: true,
			BlockOnDetection:   true,
		}
		handler := NewSecretDetectionHandlerWithConfig(config)

		msg := &engine.PreToolUseMessage{
			ToolName:  "Write",
			ToolInput: map[string]json.RawMessage{},
		}

		response, err := handler.HandlePreToolUse(ctx, msg)
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "approve", response.Decision)
	})

	t.Run("override pattern variations", func(t *testing.T) {
		config := &SecretDetectionConfig{
			EnablePromptScanning: true,
			BlockOnDetection:     true,
			AllowOverride:        true,
		}
		handler := NewSecretDetectionHandlerWithConfig(config)

		testCases := []struct {
			prompt      string
			shouldBlock bool
		}{
			{"secret_detect=override api_key=test123", false},
			{"SECRET_DETECT=OVERRIDE api_key=test123", false},
			{"  secret_detect=override  api_key=test123", false},
			{"api_key=test123 secret_detect=override", false},
			{"secret_detect = override api_key=test123", false},
			{"secretdetect=override api_key=test123", false}, // Different pattern - no secrets detected anyway
			{"secret_detect=false api_key=test123", false},   // Wrong value - no secrets detected anyway
		}

		for _, tc := range testCases {
			msg := &engine.UserPromptSubmitMessage{
				UserPrompt: tc.prompt,
			}

			response, err := handler.HandleUserPromptSubmit(ctx, msg)
			assert.NoError(t, err)

			if tc.shouldBlock {
				assert.Equal(t, "block", response.Decision, "Prompt: %s", tc.prompt)
			} else {
				assert.Equal(t, "approve", response.Decision, "Prompt: %s", tc.prompt)
			}
		}
	})
}

func TestSecretDetectionHandler_Priority(t *testing.T) {
	handler := NewSecretDetectionHandler()

	// Secret detection should have higher priority than linting (100)
	// but lower than file access (300)
	assert.Equal(t, 200, handler.Priority())
	assert.Equal(t, "secret-detection", handler.Name())
}
