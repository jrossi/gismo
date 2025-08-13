package handlers

import (
	"context"
	"testing"

	"github.com/jrossi/gismo/pkg/engine"
	"github.com/stretchr/testify/assert"
)

func TestNewNotificationHandler(t *testing.T) {
	handler := NewNotificationHandler()
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.config)
	assert.Equal(t, "notification", handler.Name())
	assert.Equal(t, 50, handler.Priority())
}

func TestNewNotificationHandlerWithConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *NotificationConfig
	}{
		{
			name:   "nil config uses default",
			config: nil,
		},
		{
			name: "custom config",
			config: &NotificationConfig{
				LogNotifications: false,
				OutputFormat:     "json",
				TimestampFormat:  "15:04:05",
				FilterRules:      []NotificationRule{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewNotificationHandlerWithConfig(tt.config)
			assert.NotNil(t, handler)
			assert.NotNil(t, handler.config)
		})
	}
}

func TestNotificationHandler_ShouldHandle(t *testing.T) {
	handler := NewNotificationHandler()
	ctx := context.Background()

	tests := []struct {
		name     string
		event    engine.HookMessage
		expected bool
	}{
		{
			name: "should handle NotificationMessage",
			event: &engine.NotificationMessage{
				NotificationType: "test",
				Message:          "test message",
			},
			expected: true,
		},
		{
			name: "should not handle UserPromptSubmit",
			event: &engine.UserPromptSubmitMessage{
				UserPrompt: "test",
			},
			expected: false,
		},
		{
			name: "should not handle PreToolUse",
			event: &engine.PreToolUseMessage{
				ToolName: "test",
			},
			expected: false,
		},
		{
			name: "should not handle PostToolUse",
			event: &engine.PostToolUseMessage{
				ToolName: "test",
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

func TestNotificationHandler_HandleNotification(t *testing.T) {
	tests := []struct {
		name   string
		config *NotificationConfig
		msg    *engine.NotificationMessage
	}{
		{
			name: "log notification with matching rule",
			config: &NotificationConfig{
				LogNotifications: true,
				FilterRules: []NotificationRule{
					{
						Name:           "test-rule",
						TypePattern:    "test_type",
						MessagePattern: ".*",
						Action:         "log",
						LogLevel:       "info",
						Enabled:        true,
					},
				},
				OutputFormat:    "simple",
				TimestampFormat: "15:04:05",
			},
			msg: &engine.NotificationMessage{
				BaseHookMessage: engine.BaseHookMessage{
					SessionID: "session123",
				},
				NotificationType: "test_type",
				Message:          "Test notification",
			},
		},
		{
			name: "alert notification",
			config: &NotificationConfig{
				LogNotifications: true,
				FilterRules: []NotificationRule{
					{
						Name:           "alert-rule",
						TypePattern:    "critical.*",
						MessagePattern: ".*",
						Action:         "alert",
						LogLevel:       "error",
						CustomMessage:  "Critical event detected!",
						Enabled:        true,
					},
				},
				OutputFormat:    "structured",
				TimestampFormat: "2006-01-02 15:04:05",
			},
			msg: &engine.NotificationMessage{
				BaseHookMessage: engine.BaseHookMessage{
					SessionID: "session456",
				},
				NotificationType: "critical_error",
				Message:          "System failure",
			},
		},
		{
			name: "ignore notification",
			config: &NotificationConfig{
				LogNotifications: true,
				FilterRules: []NotificationRule{
					{
						Name:           "ignore-rule",
						TypePattern:    "debug.*",
						MessagePattern: ".*",
						Action:         "ignore",
						Enabled:        true,
					},
				},
			},
			msg: &engine.NotificationMessage{
				BaseHookMessage: engine.BaseHookMessage{
					SessionID: "session789",
				},
				NotificationType: "debug_info",
				Message:          "Debug message",
			},
		},
		{
			name: "forward notification",
			config: &NotificationConfig{
				LogNotifications: true,
				FilterRules: []NotificationRule{
					{
						Name:           "forward-rule",
						TypePattern:    "audit.*",
						MessagePattern: ".*",
						Action:         "forward",
						LogLevel:       "info",
						Enabled:        true,
					},
				},
				OutputFormat: "simple",
			},
			msg: &engine.NotificationMessage{
				BaseHookMessage: engine.BaseHookMessage{
					SessionID: "session012",
				},
				NotificationType: "audit_log",
				Message:          "User action logged",
			},
		},
		{
			name: "disabled rule is skipped",
			config: &NotificationConfig{
				LogNotifications: true,
				FilterRules: []NotificationRule{
					{
						Name:           "disabled-rule",
						TypePattern:    ".*",
						MessagePattern: ".*",
						Action:         "alert",
						Enabled:        false,
					},
				},
			},
			msg: &engine.NotificationMessage{
				BaseHookMessage: engine.BaseHookMessage{
					SessionID: "session345",
				},
				NotificationType: "any_type",
				Message:          "Should use default logging",
			},
		},
		{
			name: "multiple matching rules",
			config: &NotificationConfig{
				LogNotifications: true,
				FilterRules: []NotificationRule{
					{
						Name:           "rule1",
						TypePattern:    "multi.*",
						MessagePattern: ".*",
						Action:         "log",
						LogLevel:       "info",
						Enabled:        true,
					},
					{
						Name:           "rule2",
						TypePattern:    "multi.*",
						MessagePattern: ".*test.*",
						Action:         "alert",
						LogLevel:       "warn",
						Enabled:        true,
					},
				},
			},
			msg: &engine.NotificationMessage{
				BaseHookMessage: engine.BaseHookMessage{
					SessionID: "session678",
				},
				NotificationType: "multi_match",
				Message:          "This is a test message",
			},
		},
		{
			name: "logging disabled",
			config: &NotificationConfig{
				LogNotifications: false,
				FilterRules:      []NotificationRule{},
			},
			msg: &engine.NotificationMessage{
				BaseHookMessage: engine.BaseHookMessage{
					SessionID: "session901",
				},
				NotificationType: "any",
				Message:          "Should not be processed",
			},
		},
		{
			name: "json output format",
			config: &NotificationConfig{
				LogNotifications: true,
				FilterRules:      []NotificationRule{},
				OutputFormat:     "json",
				TimestampFormat:  "2006-01-02T15:04:05Z",
			},
			msg: &engine.NotificationMessage{
				BaseHookMessage: engine.BaseHookMessage{
					SessionID: "session234",
				},
				NotificationType: "json_test",
				Message:          "JSON formatted message",
			},
		},
		{
			name: "custom message override",
			config: &NotificationConfig{
				LogNotifications: true,
				FilterRules: []NotificationRule{
					{
						Name:           "custom-msg",
						TypePattern:    "override.*",
						MessagePattern: ".*",
						Action:         "log",
						LogLevel:       "warn",
						CustomMessage:  "This is a custom message",
						Enabled:        true,
					},
				},
			},
			msg: &engine.NotificationMessage{
				BaseHookMessage: engine.BaseHookMessage{
					SessionID: "session567",
				},
				NotificationType: "override_test",
				Message:          "Original message",
			},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewNotificationHandlerWithConfig(tt.config)

			// HandleNotification returns nil, nil for all cases
			response, err := handler.HandleNotification(ctx, tt.msg)
			assert.Nil(t, response)
			assert.NoError(t, err)
		})
	}
}

func TestNotificationHandler_RuleMatches(t *testing.T) {
	handler := NewNotificationHandler()

	tests := []struct {
		name     string
		rule     NotificationRule
		msg      *engine.NotificationMessage
		expected bool
	}{
		{
			name: "matches type pattern",
			rule: NotificationRule{
				TypePattern:    "test.*",
				MessagePattern: "",
			},
			msg: &engine.NotificationMessage{
				NotificationType: "test_notification",
				Message:          "Any message",
			},
			expected: true,
		},
		{
			name: "matches message pattern",
			rule: NotificationRule{
				TypePattern:    "",
				MessagePattern: ".*error.*",
			},
			msg: &engine.NotificationMessage{
				NotificationType: "any_type",
				Message:          "An error occurred",
			},
			expected: true,
		},
		{
			name: "matches both patterns",
			rule: NotificationRule{
				TypePattern:    "system.*",
				MessagePattern: ".*[Cc]ritical.*",
			},
			msg: &engine.NotificationMessage{
				NotificationType: "system_alert",
				Message:          "Critical system failure",
			},
			expected: true,
		},
		{
			name: "type pattern doesn't match",
			rule: NotificationRule{
				TypePattern:    "user.*",
				MessagePattern: ".*",
			},
			msg: &engine.NotificationMessage{
				NotificationType: "system_event",
				Message:          "Any message",
			},
			expected: false,
		},
		{
			name: "message pattern doesn't match",
			rule: NotificationRule{
				TypePattern:    ".*",
				MessagePattern: "error",
			},
			msg: &engine.NotificationMessage{
				NotificationType: "any_type",
				Message:          "Success message",
			},
			expected: false,
		},
		{
			name: "one pattern matches, one doesn't",
			rule: NotificationRule{
				TypePattern:    "test.*",
				MessagePattern: "error",
			},
			msg: &engine.NotificationMessage{
				NotificationType: "test_type",
				Message:          "Success",
			},
			expected: false,
		},
		{
			name: "empty patterns match everything",
			rule: NotificationRule{
				TypePattern:    "",
				MessagePattern: "",
			},
			msg: &engine.NotificationMessage{
				NotificationType: "any",
				Message:          "any",
			},
			expected: true,
		},
		{
			name: "invalid regex in type pattern",
			rule: NotificationRule{
				TypePattern:    "[",
				MessagePattern: ".*",
			},
			msg: &engine.NotificationMessage{
				NotificationType: "test",
				Message:          "test",
			},
			expected: false,
		},
		{
			name: "invalid regex in message pattern",
			rule: NotificationRule{
				TypePattern:    ".*",
				MessagePattern: "[",
			},
			msg: &engine.NotificationMessage{
				NotificationType: "test",
				Message:          "test",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.ruleMatches(tt.rule, tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultNotificationConfig(t *testing.T) {
	config := DefaultNotificationConfig()

	assert.NotNil(t, config)
	assert.True(t, config.LogNotifications)
	assert.Equal(t, "structured", config.OutputFormat)
	assert.Equal(t, "2006-01-02 15:04:05", config.TimestampFormat)

	// Check default filter rules
	assert.Greater(t, len(config.FilterRules), 0)

	// Verify tool-permissions rule exists
	hasToolPermRule := false
	for _, r := range config.FilterRules {
		if r.Name == "tool-permissions" {
			hasToolPermRule = true
			assert.Equal(t, "tool_permission", r.TypePattern)
			assert.Equal(t, "log", r.Action)
			assert.Equal(t, "info", r.LogLevel)
			assert.True(t, r.Enabled)
			break
		}
	}
	assert.True(t, hasToolPermRule, "Should have tool-permissions rule")

	// Verify idle-timeout rule exists
	hasIdleRule := false
	for _, r := range config.FilterRules {
		if r.Name == "idle-timeout" {
			hasIdleRule = true
			assert.Equal(t, "idle_timeout", r.TypePattern)
			assert.Equal(t, "log", r.Action)
			assert.Equal(t, "warn", r.LogLevel)
			assert.True(t, r.Enabled)
			break
		}
	}
	assert.True(t, hasIdleRule, "Should have idle-timeout rule")

	// Verify system-alerts rule exists
	hasSystemRule := false
	for _, r := range config.FilterRules {
		if r.Name == "system-alerts" {
			hasSystemRule = true
			assert.Equal(t, "system_.*", r.TypePattern)
			assert.Equal(t, "alert", r.Action)
			assert.Equal(t, "warn", r.LogLevel)
			assert.True(t, r.Enabled)
			break
		}
	}
	assert.True(t, hasSystemRule, "Should have system-alerts rule")
}

func TestNotificationHandler_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("nil notification message fields", func(t *testing.T) {
		handler := NewNotificationHandler()
		msg := &engine.NotificationMessage{
			BaseHookMessage: engine.BaseHookMessage{
				SessionID: "",
			},
			NotificationType: "",
			Message:          "",
		}

		// Should not crash
		response, err := handler.HandleNotification(ctx, msg)
		assert.Nil(t, response)
		assert.NoError(t, err)
	})

	t.Run("very long message", func(t *testing.T) {
		handler := NewNotificationHandler()
		longMessage := string(make([]byte, 10000))
		msg := &engine.NotificationMessage{
			BaseHookMessage: engine.BaseHookMessage{
				SessionID: "test",
			},
			NotificationType: "test",
			Message:          longMessage,
		}

		// Should handle long messages gracefully
		response, err := handler.HandleNotification(ctx, msg)
		assert.Nil(t, response)
		assert.NoError(t, err)
	})

	t.Run("unknown output format defaults to simple", func(t *testing.T) {
		config := &NotificationConfig{
			LogNotifications: true,
			FilterRules:      []NotificationRule{},
			OutputFormat:     "unknown_format",
			TimestampFormat:  "15:04:05",
		}
		handler := NewNotificationHandlerWithConfig(config)

		msg := &engine.NotificationMessage{
			BaseHookMessage: engine.BaseHookMessage{
				SessionID: "test",
			},
			NotificationType: "test",
			Message:          "Test message",
		}

		// Should use default format without crashing
		response, err := handler.HandleNotification(ctx, msg)
		assert.Nil(t, response)
		assert.NoError(t, err)
	})

	t.Run("unknown log level in rule", func(t *testing.T) {
		config := &NotificationConfig{
			LogNotifications: true,
			FilterRules: []NotificationRule{
				{
					Name:           "unknown-level",
					TypePattern:    ".*",
					MessagePattern: ".*",
					Action:         "log",
					LogLevel:       "unknown_level",
					Enabled:        true,
				},
			},
			OutputFormat: "simple",
		}
		handler := NewNotificationHandlerWithConfig(config)

		msg := &engine.NotificationMessage{
			BaseHookMessage: engine.BaseHookMessage{
				SessionID: "test",
			},
			NotificationType: "test",
			Message:          "Test message",
		}

		// Should handle unknown log level gracefully
		response, err := handler.HandleNotification(ctx, msg)
		assert.Nil(t, response)
		assert.NoError(t, err)
	})

	t.Run("unknown action in rule", func(t *testing.T) {
		config := &NotificationConfig{
			LogNotifications: true,
			FilterRules: []NotificationRule{
				{
					Name:           "unknown-action",
					TypePattern:    ".*",
					MessagePattern: ".*",
					Action:         "unknown_action",
					LogLevel:       "info",
					Enabled:        true,
				},
			},
		}
		handler := NewNotificationHandlerWithConfig(config)

		msg := &engine.NotificationMessage{
			BaseHookMessage: engine.BaseHookMessage{
				SessionID: "test",
			},
			NotificationType: "test",
			Message:          "Test message",
		}

		// Should default to logging for unknown actions
		response, err := handler.HandleNotification(ctx, msg)
		assert.Nil(t, response)
		assert.NoError(t, err)
	})
}

func TestNotificationHandler_Priority(t *testing.T) {
	handler := NewNotificationHandler()

	// Notification handler should have low priority
	assert.Equal(t, 50, handler.Priority())
	assert.Equal(t, "notification", handler.Name())
}

func TestNotificationHandler_OutputFormats(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		format string
	}{
		{
			name:   "structured format",
			format: "structured",
		},
		{
			name:   "simple format",
			format: "simple",
		},
		{
			name:   "json format",
			format: "json",
		},
		{
			name:   "default format",
			format: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &NotificationConfig{
				LogNotifications: true,
				FilterRules:      []NotificationRule{},
				OutputFormat:     tt.format,
				TimestampFormat:  "15:04:05",
			}
			handler := NewNotificationHandlerWithConfig(config)

			msg := &engine.NotificationMessage{
				BaseHookMessage: engine.BaseHookMessage{
					SessionID: "session123",
				},
				NotificationType: "test_type",
				Message:          "Test message",
			}

			// Should handle all formats without error
			response, err := handler.HandleNotification(ctx, msg)
			assert.Nil(t, response)
			assert.NoError(t, err)
		})
	}
}
