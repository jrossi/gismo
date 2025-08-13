package handlers

import (
	"context"
	"testing"

	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileAccessHandler(t *testing.T) {
	handler := NewFileAccessHandler()
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.config)
	assert.Equal(t, "file-access", handler.Name())
	assert.Equal(t, 300, handler.Priority())
}

func TestNewFileAccessHandlerWithConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *FileAccessConfig
	}{
		{
			name:   "nil config uses default",
			config: nil,
		},
		{
			name: "custom config",
			config: &FileAccessConfig{
				BlockOnViolation: false,
				ReadRestrictions: []FileRestriction{
					{
						Pattern: `\.test$`,
						Action:  "warn",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewFileAccessHandlerWithConfig(tt.config)
			assert.NotNil(t, handler)
			assert.NotNil(t, handler.config)
		})
	}
}

func TestFileAccessHandler_ShouldHandle(t *testing.T) {
	handler := NewFileAccessHandler()
	ctx := context.Background()

	tests := []struct {
		name     string
		event    engine.HookMessage
		expected bool
	}{
		{
			name: "should handle Read tool",
			event: &engine.PreToolUseMessage{
				ToolName: "Read",
			},
			expected: true,
		},
		{
			name: "should handle Write tool",
			event: &engine.PreToolUseMessage{
				ToolName: "Write",
			},
			expected: true,
		},
		{
			name: "should handle Edit tool",
			event: &engine.PreToolUseMessage{
				ToolName: "Edit",
			},
			expected: true,
		},
		{
			name: "should handle MultiEdit tool",
			event: &engine.PreToolUseMessage{
				ToolName: "MultiEdit",
			},
			expected: true,
		},
		{
			name: "should not handle other tools",
			event: &engine.PreToolUseMessage{
				ToolName: "Bash",
			},
			expected: false,
		},
		{
			name: "should not handle UserPromptSubmit",
			event: &engine.UserPromptSubmitMessage{
				UserPrompt: "test prompt",
			},
			expected: false,
		},
		{
			name: "should not handle PostToolUse",
			event: &engine.PostToolUseMessage{
				ToolName: "Read",
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

func TestFileAccessHandler_HandlePreToolUse_ReadRestrictions(t *testing.T) {
	tests := []struct {
		name           string
		config         *FileAccessConfig
		toolName       string
		filePath       string
		expectedResult string
		expectedReason string
	}{
		{
			name: "block reading .pem files",
			config: &FileAccessConfig{
				ReadRestrictions: []FileRestriction{
					{
						Pattern: `\.pem$`,
						Action:  "block",
						Message: "PEM files are restricted",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "Read",
			filePath:       "/path/to/cert.pem",
			expectedResult: "block",
			expectedReason: "PEM files are restricted",
		},
		{
			name: "block reading .key files",
			config: &FileAccessConfig{
				ReadRestrictions: []FileRestriction{
					{
						Pattern: `\.key$`,
						Action:  "block",
						Message: "Private keys are restricted",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "Read",
			filePath:       "/home/user/.ssh/id_rsa.key",
			expectedResult: "block",
			expectedReason: "Private keys are restricted",
		},
		{
			name: "warn on restricted file but don't block",
			config: &FileAccessConfig{
				ReadRestrictions: []FileRestriction{
					{
						Pattern: `\.env$`,
						Action:  "warn",
						Message: "Environment files may contain secrets",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "Read",
			filePath:       "/app/.env",
			expectedResult: "approve",
		},
		{
			name: "allow reading unrestricted files",
			config: &FileAccessConfig{
				ReadRestrictions: []FileRestriction{
					{
						Pattern: `\.pem$`,
						Action:  "block",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "Read",
			filePath:       "/path/to/file.txt",
			expectedResult: "approve",
		},
		{
			name: "log action only logs, doesn't block",
			config: &FileAccessConfig{
				ReadRestrictions: []FileRestriction{
					{
						Pattern: `\.log$`,
						Action:  "log",
						Message: "Log file access",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "Read",
			filePath:       "/var/log/app.log",
			expectedResult: "approve",
		},
		{
			name: "block disabled doesn't block even with block action",
			config: &FileAccessConfig{
				ReadRestrictions: []FileRestriction{
					{
						Pattern: `\.pem$`,
						Action:  "block",
						Message: "PEM restricted",
					},
				},
				BlockOnViolation: false,
			},
			toolName:       "Read",
			filePath:       "/cert.pem",
			expectedResult: "approve",
		},
		{
			name: "multiple restrictions - first match wins",
			config: &FileAccessConfig{
				ReadRestrictions: []FileRestriction{
					{
						Pattern: `\.pem$`,
						Action:  "block",
						Message: "PEM blocked",
					},
					{
						Pattern: `\.pem$`,
						Action:  "warn",
						Message: "PEM warning",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "Read",
			filePath:       "/cert.pem",
			expectedResult: "block",
			expectedReason: "PEM blocked",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewFileAccessHandlerWithConfig(tt.config)

			filePathBytes, _ := json.Marshal(tt.filePath)
			msg := &engine.PreToolUseMessage{
				ToolName: tt.toolName,
				ToolInput: map[string]json.RawMessage{
					"file_path": filePathBytes,
				},
			}

			response, err := handler.HandlePreToolUse(ctx, msg)
			require.NoError(t, err)
			require.NotNil(t, response)

			assert.Equal(t, tt.expectedResult, response.Decision)
			if tt.expectedReason != "" {
				assert.Equal(t, tt.expectedReason, response.Reason)
			}
		})
	}
}

func TestFileAccessHandler_HandlePreToolUse_WriteRestrictions(t *testing.T) {
	tests := []struct {
		name           string
		config         *FileAccessConfig
		toolName       string
		filePath       string
		expectedResult string
		expectedReason string
	}{
		{
			name: "block writing to /etc",
			config: &FileAccessConfig{
				WriteRestrictions: []FileRestriction{
					{
						Pattern: `^/etc/`,
						Action:  "block",
						Message: "System directories are protected",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "Write",
			filePath:       "/etc/passwd",
			expectedResult: "block",
			expectedReason: "System directories are protected",
		},
		{
			name: "block writing to /usr",
			config: &FileAccessConfig{
				WriteRestrictions: []FileRestriction{
					{
						Pattern: `^/usr/`,
						Action:  "block",
						Message: "System directories are protected",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "Edit",
			filePath:       "/usr/bin/script",
			expectedResult: "block",
			expectedReason: "System directories are protected",
		},
		{
			name: "allow writing to user directories",
			config: &FileAccessConfig{
				WriteRestrictions: []FileRestriction{
					{
						Pattern: `^/etc/`,
						Action:  "block",
					},
					{
						Pattern: `^/usr/`,
						Action:  "block",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "Write",
			filePath:       "/home/user/file.txt",
			expectedResult: "approve",
		},
		{
			name: "warn on sensitive directories",
			config: &FileAccessConfig{
				WriteRestrictions: []FileRestriction{
					{
						Pattern: `\.git/`,
						Action:  "warn",
						Message: "Writing to .git directory is dangerous",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "MultiEdit",
			filePath:       "/project/.git/config",
			expectedResult: "approve",
		},
		{
			name: "handle Edit tool",
			config: &FileAccessConfig{
				WriteRestrictions: []FileRestriction{
					{
						Pattern: `^/etc/`,
						Action:  "block",
						Message: "System files protected",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "Edit",
			filePath:       "/etc/hosts",
			expectedResult: "block",
			expectedReason: "System files protected",
		},
		{
			name: "handle MultiEdit tool",
			config: &FileAccessConfig{
				WriteRestrictions: []FileRestriction{
					{
						Pattern: `^/etc/`,
						Action:  "block",
						Message: "System files protected",
					},
				},
				BlockOnViolation: true,
			},
			toolName:       "MultiEdit",
			filePath:       "/etc/hosts",
			expectedResult: "block",
			expectedReason: "System files protected",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewFileAccessHandlerWithConfig(tt.config)

			filePathBytes, _ := json.Marshal(tt.filePath)
			msg := &engine.PreToolUseMessage{
				ToolName: tt.toolName,
				ToolInput: map[string]json.RawMessage{
					"file_path": filePathBytes,
				},
			}

			response, err := handler.HandlePreToolUse(ctx, msg)
			require.NoError(t, err)
			require.NotNil(t, response)

			assert.Equal(t, tt.expectedResult, response.Decision)
			if tt.expectedReason != "" {
				assert.Equal(t, tt.expectedReason, response.Reason)
			}
		})
	}
}

func TestFileAccessHandler_ExtractFilePath(t *testing.T) {
	handler := NewFileAccessHandler()

	tests := []struct {
		name        string
		toolInput   map[string]json.RawMessage
		expected    string
		expectError bool
	}{
		{
			name: "valid file path",
			toolInput: map[string]json.RawMessage{
				"file_path": json.RawMessage(`"/path/to/file.txt"`),
			},
			expected:    "/path/to/file.txt",
			expectError: false,
		},
		{
			name:        "missing file_path",
			toolInput:   map[string]json.RawMessage{},
			expected:    "",
			expectError: true,
		},
		{
			name: "invalid json",
			toolInput: map[string]json.RawMessage{
				"file_path": json.RawMessage(`not-valid-json`),
			},
			expected:    "",
			expectError: true,
		},
		{
			name: "file path with spaces",
			toolInput: map[string]json.RawMessage{
				"file_path": json.RawMessage(`"/path with spaces/file.txt"`),
			},
			expected:    "/path with spaces/file.txt",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &engine.PreToolUseMessage{
				ToolInput: tt.toolInput,
			}

			result, err := handler.extractFilePath(msg)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFileAccessHandler_NormalizePath(t *testing.T) {
	handler := NewFileAccessHandler()

	tests := []struct {
		name     string
		input    string
		contains string // Since absolute paths vary, we check if result contains certain strings
	}{
		{
			name:     "relative path",
			input:    "./file.txt",
			contains: "file.txt",
		},
		{
			name:     "path with ..",
			input:    "/path/../file.txt",
			contains: "file.txt",
		},
		{
			name:     "path with double slashes",
			input:    "/path//to///file.txt",
			contains: "/path/to/file.txt",
		},
		{
			name:     "windows style path",
			input:    `C:\Users\test\file.txt`,
			contains: "/Users/test/file.txt",
		},
		{
			name:     "already normalized",
			input:    "/path/to/file.txt",
			contains: "/path/to/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.normalizePath(tt.input)
			assert.Contains(t, result, tt.contains)
			// Ensure no backslashes in result
			assert.NotContains(t, result, "\\")
		})
	}
}

func TestFileAccessHandler_ConfigManagement(t *testing.T) {
	handler := NewFileAccessHandler()

	t.Run("SetConfig with nil returns error", func(t *testing.T) {
		err := handler.SetConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("SetConfig with valid config", func(t *testing.T) {
		config := &FileAccessConfig{
			BlockOnViolation: false,
		}
		err := handler.SetConfig(config)
		assert.NoError(t, err)
		assert.Equal(t, config, handler.config)
	})

	t.Run("AddReadRestriction", func(t *testing.T) {
		handler := NewFileAccessHandler()
		initialCount := len(handler.config.ReadRestrictions)

		restriction := FileRestriction{
			Pattern: `\.test$`,
			Action:  "block",
			Message: "Test files blocked",
		}

		handler.AddReadRestriction(restriction)
		assert.Len(t, handler.config.ReadRestrictions, initialCount+1)
		assert.Contains(t, handler.config.ReadRestrictions, restriction)
	})

	t.Run("AddWriteRestriction", func(t *testing.T) {
		handler := NewFileAccessHandler()
		initialCount := len(handler.config.WriteRestrictions)

		restriction := FileRestriction{
			Pattern: `\.test$`,
			Action:  "block",
			Message: "Test files blocked",
		}

		handler.AddWriteRestriction(restriction)
		assert.Len(t, handler.config.WriteRestrictions, initialCount+1)
		assert.Contains(t, handler.config.WriteRestrictions, restriction)
	})

	t.Run("RemoveReadRestriction", func(t *testing.T) {
		handler := NewFileAccessHandler()
		restriction := FileRestriction{
			Pattern: `\.test$`,
			Action:  "block",
		}
		handler.AddReadRestriction(restriction)

		handler.RemoveReadRestriction(`\.test$`)
		for _, r := range handler.config.ReadRestrictions {
			assert.NotEqual(t, `\.test$`, r.Pattern)
		}
	})

	t.Run("RemoveWriteRestriction", func(t *testing.T) {
		handler := NewFileAccessHandler()
		restriction := FileRestriction{
			Pattern: `\.test$`,
			Action:  "block",
		}
		handler.AddWriteRestriction(restriction)

		handler.RemoveWriteRestriction(`\.test$`)
		for _, r := range handler.config.WriteRestrictions {
			assert.NotEqual(t, `\.test$`, r.Pattern)
		}
	})
}

func TestFileAccessHandler_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("empty pattern in restriction", func(t *testing.T) {
		config := &FileAccessConfig{
			ReadRestrictions: []FileRestriction{
				{
					Pattern: "",
					Action:  "block",
					Message: "Empty pattern",
				},
			},
			BlockOnViolation: true,
		}
		handler := NewFileAccessHandlerWithConfig(config)

		filePathBytes, _ := json.Marshal("/test/file.txt")
		msg := &engine.PreToolUseMessage{
			ToolName: "Read",
			ToolInput: map[string]json.RawMessage{
				"file_path": filePathBytes,
			},
		}

		response, err := handler.HandlePreToolUse(ctx, msg)
		assert.NoError(t, err)
		// Empty pattern matches everything, so it should block
		assert.Equal(t, "block", response.Decision)
	})

	t.Run("invalid regex pattern", func(t *testing.T) {
		config := &FileAccessConfig{
			ReadRestrictions: []FileRestriction{
				{
					Pattern: `[`, // Invalid regex
					Action:  "block",
					Message: "Invalid pattern",
				},
			},
			BlockOnViolation: true,
		}
		handler := NewFileAccessHandlerWithConfig(config)

		filePathBytes, _ := json.Marshal("/test/file.txt")
		msg := &engine.PreToolUseMessage{
			ToolName: "Read",
			ToolInput: map[string]json.RawMessage{
				"file_path": filePathBytes,
			},
		}

		// Should handle invalid regex gracefully
		response, err := handler.HandlePreToolUse(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "approve", response.Decision)
	})

	t.Run("message fallback to description", func(t *testing.T) {
		config := &FileAccessConfig{
			ReadRestrictions: []FileRestriction{
				{
					Pattern:     `\.test$`,
					Action:      "block",
					Message:     "", // Empty message
					Description: "Test files are restricted",
				},
			},
			BlockOnViolation: true,
		}
		handler := NewFileAccessHandlerWithConfig(config)

		filePathBytes, _ := json.Marshal("/file.test")
		msg := &engine.PreToolUseMessage{
			ToolName: "Read",
			ToolInput: map[string]json.RawMessage{
				"file_path": filePathBytes,
			},
		}

		response, err := handler.HandlePreToolUse(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "block", response.Decision)
		assert.Contains(t, response.Reason, "Test files are restricted")
	})

	t.Run("non-file tools are approved", func(t *testing.T) {
		handler := NewFileAccessHandler()

		msg := &engine.PreToolUseMessage{
			ToolName:  "Bash",
			ToolInput: map[string]json.RawMessage{},
		}

		response, err := handler.HandlePreToolUse(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "approve", response.Decision)
	})

	t.Run("missing tool input for file operation", func(t *testing.T) {
		handler := NewFileAccessHandler()

		msg := &engine.PreToolUseMessage{
			ToolName:  "Read",
			ToolInput: map[string]json.RawMessage{}, // Missing file_path
		}

		response, err := handler.HandlePreToolUse(ctx, msg)
		assert.NoError(t, err)
		assert.Equal(t, "approve", response.Decision) // Gracefully approves when can't extract path
	})
}

func TestDefaultFileAccessConfig(t *testing.T) {
	config := DefaultFileAccessConfig()

	assert.NotNil(t, config)
	assert.True(t, config.BlockOnViolation)

	// Check default read restrictions
	assert.Greater(t, len(config.ReadRestrictions), 0)

	// Verify PEM file restriction exists
	hasPemRestriction := false
	for _, r := range config.ReadRestrictions {
		if r.Pattern == `\.pem$` {
			hasPemRestriction = true
			assert.Equal(t, "block", r.Action)
			break
		}
	}
	assert.True(t, hasPemRestriction, "Should have PEM file restriction")

	// Verify key file restriction exists
	hasKeyRestriction := false
	for _, r := range config.ReadRestrictions {
		if r.Pattern == `\.key$` {
			hasKeyRestriction = true
			assert.Equal(t, "block", r.Action)
			break
		}
	}
	assert.True(t, hasKeyRestriction, "Should have key file restriction")

	// Check default write restrictions
	assert.Greater(t, len(config.WriteRestrictions), 0)

	// Verify /etc restriction exists
	hasEtcRestriction := false
	for _, r := range config.WriteRestrictions {
		if r.Pattern == `^/etc/` {
			hasEtcRestriction = true
			assert.Equal(t, "block", r.Action)
			break
		}
	}
	assert.True(t, hasEtcRestriction, "Should have /etc restriction")
}
