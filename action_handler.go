package gismo

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"sync"
)

// ActionHandler defines the interface for handling specific hook events
type ActionHandler interface {
	// Name returns the unique name of this handler
	Name() string

	// Priority returns the execution priority (higher numbers execute first)
	Priority() int

	// ShouldHandle determines if this handler should process the given event
	ShouldHandle(ctx context.Context, event HookMessage) bool

	// Handle processes the event and returns a response
	Handle(ctx context.Context, event HookMessage) (*HookResponse, error)
}

// PreToolUseHandler handles PreToolUse events
type PreToolUseHandler interface {
	ActionHandler
	HandlePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error)
}

// PostToolUseHandler handles PostToolUse events
type PostToolUseHandler interface {
	ActionHandler
	HandlePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error)
}

// UserPromptSubmitHandler handles UserPromptSubmit events
type UserPromptSubmitHandler interface {
	ActionHandler
	HandleUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error)
}

// NotificationHandler handles Notification events
type NotificationHandler interface {
	ActionHandler
	HandleNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error)
}

// SessionStartHandler handles SessionStart events
type SessionStartHandler interface {
	ActionHandler
	HandleSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error)
}

// StopHandler handles Stop events
type StopHandler interface {
	ActionHandler
	HandleStop(ctx context.Context, msg *StopMessage) (*HookResponse, error)
}

// SubagentStopHandler handles SubagentStop events
type SubagentStopHandler interface {
	ActionHandler
	HandleSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error)
}

// PreCompactHandler handles PreCompact events
type PreCompactHandler interface {
	ActionHandler
	HandlePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error)
}

// HandlerConfig represents configuration for an action handler
type HandlerConfig struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Priority int             `json:"priority"`
	Enabled  bool            `json:"enabled"`
	Rules    []HandlerRule   `json:"rules"`
	Config   json.RawMessage `json:"config,omitempty"`
}

// HandlerRule defines conditions for when a handler should execute
type HandlerRule struct {
	// Event types this rule applies to
	Events []HookEventName `json:"events"`

	// Tool name pattern (for PreToolUse/PostToolUse)
	ToolPattern string `json:"tool_pattern,omitempty"`

	// File path pattern
	FilePathPattern string `json:"file_path_pattern,omitempty"`

	// Notification type pattern
	NotificationTypePattern string `json:"notification_type_pattern,omitempty"`

	// Action to take: "allow", "deny", "continue"
	Action string `json:"action"`

	// Message to include in response
	Message string `json:"message,omitempty"`
}

// ActionHandlerRegistry manages action handlers for all hook types
type ActionHandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[HookEventName][]ActionHandler
	config   *ActionHandlerConfig
}

// ActionHandlerConfig represents the configuration for the action handler system
type ActionHandlerConfig struct {
	Handlers []HandlerConfig `json:"handlers"`
}

// NewActionHandlerRegistry creates a new action handler registry
func NewActionHandlerRegistry() *ActionHandlerRegistry {
	return &ActionHandlerRegistry{
		handlers: make(map[HookEventName][]ActionHandler),
		config:   &ActionHandlerConfig{},
	}
}

// Register adds an action handler for specific event types
func (r *ActionHandlerRegistry) Register(eventType HookEventName, handler ActionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[eventType] = append(r.handlers[eventType], handler)

	// Sort by priority (higher priority first)
	sort.Slice(r.handlers[eventType], func(i, j int) bool {
		return r.handlers[eventType][i].Priority() > r.handlers[eventType][j].Priority()
	})
}

// GetHandlers returns all handlers for a specific event type
func (r *ActionHandlerRegistry) GetHandlers(eventType HookEventName) []ActionHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := make([]ActionHandler, len(r.handlers[eventType]))
	copy(handlers, r.handlers[eventType])
	return handlers
}

// GetApplicableHandlers returns handlers that should process the given event
func (r *ActionHandlerRegistry) GetApplicableHandlers(ctx context.Context, event HookMessage) []ActionHandler {
	eventType := event.EventName()
	allHandlers := r.GetHandlers(eventType)

	var applicable []ActionHandler
	for _, handler := range allHandlers {
		if handler.ShouldHandle(ctx, event) {
			applicable = append(applicable, handler)
		}
	}

	return applicable
}

// Clear removes all registered handlers
func (r *ActionHandlerRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers = make(map[HookEventName][]ActionHandler)
}

// SetConfig updates the registry configuration
func (r *ActionHandlerRegistry) SetConfig(config *ActionHandlerConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.config = config
}

// GetConfig returns the current configuration
func (r *ActionHandlerRegistry) GetConfig() *ActionHandlerConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.config
}

// BaseActionHandler provides common functionality for action handlers
type BaseActionHandler struct {
	name     string
	priority int
	rules    []HandlerRule
}

// NewBaseActionHandler creates a new base action handler
func NewBaseActionHandler(name string, priority int) *BaseActionHandler {
	return &BaseActionHandler{
		name:     name,
		priority: priority,
		rules:    []HandlerRule{},
	}
}

// Name returns the handler name
func (h *BaseActionHandler) Name() string {
	return h.name
}

// Priority returns the handler priority
func (h *BaseActionHandler) Priority() int {
	return h.priority
}

// SetRules sets the handler rules
func (h *BaseActionHandler) SetRules(rules []HandlerRule) {
	h.rules = rules
}

// ShouldHandle determines if this handler should process the event based on rules
func (h *BaseActionHandler) ShouldHandle(ctx context.Context, event HookMessage) bool {
	if len(h.rules) == 0 {
		// No rules means handle all events of the registered type
		return true
	}

	eventType := event.EventName()

	for _, rule := range h.rules {
		if h.ruleMatches(rule, event, eventType) {
			return rule.Action == "allow" || rule.Action == "continue"
		}
	}

	// Default to not handling if no rules match
	return false
}

// ruleMatches checks if a rule matches the given event
func (h *BaseActionHandler) ruleMatches(rule HandlerRule, event HookMessage, eventType HookEventName) bool {
	// Check event type
	if len(rule.Events) > 0 {
		eventMatches := false
		for _, ruleEvent := range rule.Events {
			if ruleEvent == eventType {
				eventMatches = true
				break
			}
		}
		if !eventMatches {
			return false
		}
	}

	// Check tool pattern for PreToolUse/PostToolUse events
	if rule.ToolPattern != "" {
		var toolName string
		switch msg := event.(type) {
		case *PreToolUseMessage:
			toolName = msg.ToolName
		case *PostToolUseMessage:
			toolName = msg.ToolName
		default:
			return false
		}

		if matched, _ := regexp.MatchString(rule.ToolPattern, toolName); !matched {
			return false
		}
	}

	// Check file path pattern
	if rule.FilePathPattern != "" {
		filePath := h.extractFilePath(event)
		if filePath == "" {
			return false
		}

		if matched, _ := regexp.MatchString(rule.FilePathPattern, filePath); !matched {
			return false
		}
	}

	// Check notification type pattern
	if rule.NotificationTypePattern != "" {
		if msg, ok := event.(*NotificationMessage); ok {
			if matched, _ := regexp.MatchString(rule.NotificationTypePattern, msg.NotificationType); !matched {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

// extractFilePath extracts file path from tool events
func (h *BaseActionHandler) extractFilePath(event HookMessage) string {
	switch msg := event.(type) {
	case *PreToolUseMessage:
		if filePathRaw, exists := msg.ToolInput["file_path"]; exists {
			var filePath string
			if err := json.Unmarshal(filePathRaw, &filePath); err == nil {
				return filePath
			}
		}
	case *PostToolUseMessage:
		if filePathRaw, exists := msg.ToolInput["file_path"]; exists {
			var filePath string
			if err := json.Unmarshal(filePathRaw, &filePath); err == nil {
				return filePath
			}
		}
	}
	return ""
}

// Handle provides a default implementation that delegates to type-specific handlers
func (h *BaseActionHandler) Handle(ctx context.Context, event HookMessage) (*HookResponse, error) {
	return nil, fmt.Errorf("handle method not implemented for handler %s", h.name)
}
