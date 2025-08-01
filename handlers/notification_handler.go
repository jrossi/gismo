package handlers

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/jrossi/gismo"
)

// NotificationHandler handles system notifications
type NotificationHandler struct {
	*gismo.BaseActionHandler
	config *NotificationConfig
}

// NotificationConfig provides configuration for notification handling
type NotificationConfig struct {
	// LogNotifications determines if notifications should be logged
	LogNotifications bool `json:"log_notifications"`

	// FilterRules defines rules for filtering notifications
	FilterRules []NotificationRule `json:"filter_rules"`

	// OutputFormat defines how to format notification output
	OutputFormat string `json:"output_format"`

	// TimestampFormat defines timestamp format for logs
	TimestampFormat string `json:"timestamp_format"`
}

// NotificationRule defines how to handle specific notification types
type NotificationRule struct {
	// Name is a descriptive name for this rule
	Name string `json:"name"`

	// TypePattern is a regex pattern for notification types to match
	TypePattern string `json:"type_pattern"`

	// MessagePattern is a regex pattern for notification messages to match
	MessagePattern string `json:"message_pattern"`

	// Action defines what to do: "log", "ignore", "alert", "forward"
	Action string `json:"action"`

	// LogLevel defines the log level: "info", "warn", "error", "debug"
	LogLevel string `json:"log_level"`

	// CustomMessage overrides the notification message
	CustomMessage string `json:"custom_message,omitempty"`

	// Enabled determines if this rule is active
	Enabled bool `json:"enabled"`
}

// DefaultNotificationConfig returns default configuration
func DefaultNotificationConfig() *NotificationConfig {
	return &NotificationConfig{
		LogNotifications: true,
		FilterRules: []NotificationRule{
			{
				Name:           "tool-permissions",
				TypePattern:    "tool_permission",
				MessagePattern: ".*",
				Action:         "log",
				LogLevel:       "info",
				CustomMessage:  "",
				Enabled:        true,
			},
			{
				Name:           "idle-timeout",
				TypePattern:    "idle_timeout",
				MessagePattern: ".*",
				Action:         "log",
				LogLevel:       "warn",
				CustomMessage:  "Session idle timeout detected",
				Enabled:        true,
			},
			{
				Name:           "system-alerts",
				TypePattern:    "system_.*",
				MessagePattern: ".*",
				Action:         "alert",
				LogLevel:       "warn",
				CustomMessage:  "",
				Enabled:        true,
			},
		},
		OutputFormat:    "structured",
		TimestampFormat: "2006-01-02 15:04:05",
	}
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler() *NotificationHandler {
	return NewNotificationHandlerWithConfig(DefaultNotificationConfig())
}

// NewNotificationHandlerWithConfig creates a new notification handler with custom configuration
func NewNotificationHandlerWithConfig(config *NotificationConfig) *NotificationHandler {
	if config == nil {
		config = DefaultNotificationConfig()
	}

	return &NotificationHandler{
		BaseActionHandler: gismo.NewBaseActionHandler("notification", 50), // Low priority
		config:            config,
	}
}

// ShouldHandle determines if this handler should process notification events
func (h *NotificationHandler) ShouldHandle(ctx context.Context, event gismo.HookMessage) bool {
	_, ok := event.(*gismo.NotificationMessage)
	return ok
}

// HandleNotification processes system notifications
func (h *NotificationHandler) HandleNotification(ctx context.Context, msg *gismo.NotificationMessage) (*gismo.HookResponse, error) {
	if !h.config.LogNotifications {
		return nil, nil
	}

	// Find matching rules
	var matchedRules []NotificationRule
	for _, rule := range h.config.FilterRules {
		if !rule.Enabled {
			continue
		}

		if h.ruleMatches(rule, msg) {
			matchedRules = append(matchedRules, rule)
		}
	}

	// If no rules match, use default logging
	if len(matchedRules) == 0 {
		h.logNotification(msg, "info", "")
		return nil, nil
	}

	// Process matched rules
	for _, rule := range matchedRules {
		switch rule.Action {
		case "log":
			customMsg := rule.CustomMessage
			if customMsg == "" {
				customMsg = msg.Message
			}
			h.logNotification(msg, rule.LogLevel, customMsg)

		case "alert":
			customMsg := rule.CustomMessage
			if customMsg == "" {
				customMsg = msg.Message
			}
			h.alertNotification(msg, rule.LogLevel, customMsg)

		case "ignore":
			// Do nothing

		case "forward":
			// Could be extended to forward to external systems
			h.logNotification(msg, rule.LogLevel, fmt.Sprintf("[FORWARDED] %s", msg.Message))

		default:
			h.logNotification(msg, "info", msg.Message)
		}
	}

	return nil, nil
}

// ruleMatches checks if a notification rule matches the given message
func (h *NotificationHandler) ruleMatches(rule NotificationRule, msg *gismo.NotificationMessage) bool {
	// Check notification type pattern
	if rule.TypePattern != "" {
		if matched, err := regexp.MatchString(rule.TypePattern, msg.NotificationType); err != nil || !matched {
			return false
		}
	}

	// Check message pattern
	if rule.MessagePattern != "" {
		if matched, err := regexp.MatchString(rule.MessagePattern, msg.Message); err != nil || !matched {
			return false
		}
	}

	return true
}

// logNotification logs a notification with structured format
func (h *NotificationHandler) logNotification(msg *gismo.NotificationMessage, level, customMessage string) {
	timestamp := time.Now().Format(h.config.TimestampFormat)
	message := customMessage
	if message == "" {
		message = msg.Message
	}

	var levelPrefix string
	switch level {
	case "error":
		levelPrefix = "❌ ERROR"
	case "warn":
		levelPrefix = "⚠️  WARN"
	case "info":
		levelPrefix = "ℹ️  INFO"
	case "debug":
		levelPrefix = "🔍 DEBUG"
	default:
		levelPrefix = "📝 LOG"
	}

	switch h.config.OutputFormat {
	case "structured":
		fmt.Fprintf(os.Stderr, "\n> Notification [%s]:\n  - [notification]: %s %s\n  - Type: %s\n  - Session: %s\n  - Message: %s\n",
			timestamp, levelPrefix, msg.NotificationType, msg.NotificationType, msg.SessionID, message)

	case "simple":
		fmt.Fprintf(os.Stderr, "\n> [%s] %s: %s\n", timestamp, levelPrefix, message)

	case "json":
		fmt.Fprintf(os.Stderr, `{"timestamp":"%s","level":"%s","type":"%s","session":"%s","message":"%s"}`+"\n",
			timestamp, level, msg.NotificationType, msg.SessionID, message)

	default:
		fmt.Fprintf(os.Stderr, "\n> [%s] Notification (%s): %s\n", timestamp, msg.NotificationType, message)
	}
}

// alertNotification logs a notification with alert formatting
func (h *NotificationHandler) alertNotification(msg *gismo.NotificationMessage, level, customMessage string) {
	timestamp := time.Now().Format(h.config.TimestampFormat)
	message := customMessage
	if message == "" {
		message = msg.Message
	}

	// Alert format is more prominent
	fmt.Fprintf(os.Stderr, "\n🚨 ALERT [%s] 🚨\n", timestamp)
	fmt.Fprintf(os.Stderr, "  Type: %s\n", msg.NotificationType)
	fmt.Fprintf(os.Stderr, "  Level: %s\n", level)
	fmt.Fprintf(os.Stderr, "  Session: %s\n", msg.SessionID)
	fmt.Fprintf(os.Stderr, "  Message: %s\n", message)
	fmt.Fprintf(os.Stderr, "🚨 END ALERT 🚨\n")
}

// SetConfig updates the handler configuration
func (h *NotificationHandler) SetConfig(config *NotificationConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	h.config = config
	return nil
}

// AddRule adds a new notification rule
func (h *NotificationHandler) AddRule(rule NotificationRule) {
	h.config.FilterRules = append(h.config.FilterRules, rule)
}

// RemoveRule removes a notification rule by name
func (h *NotificationHandler) RemoveRule(name string) {
	var filtered []NotificationRule
	for _, rule := range h.config.FilterRules {
		if rule.Name != name {
			filtered = append(filtered, rule)
		}
	}
	h.config.FilterRules = filtered
}

// EnableRule enables a rule by name
func (h *NotificationHandler) EnableRule(name string) {
	for i := range h.config.FilterRules {
		if h.config.FilterRules[i].Name == name {
			h.config.FilterRules[i].Enabled = true
			return
		}
	}
}

// DisableRule disables a rule by name
func (h *NotificationHandler) DisableRule(name string) {
	for i := range h.config.FilterRules {
		if h.config.FilterRules[i].Name == name {
			h.config.FilterRules[i].Enabled = false
			return
		}
	}
}

// GetStats returns notification handling statistics
func (h *NotificationHandler) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"rules_count":      len(h.config.FilterRules),
		"enabled_rules":    h.countEnabledRules(),
		"output_format":    h.config.OutputFormat,
		"log_enabled":      h.config.LogNotifications,
		"timestamp_format": h.config.TimestampFormat,
	}
}

// countEnabledRules counts the number of enabled rules
func (h *NotificationHandler) countEnabledRules() int {
	count := 0
	for _, rule := range h.config.FilterRules {
		if rule.Enabled {
			count++
		}
	}
	return count
}
