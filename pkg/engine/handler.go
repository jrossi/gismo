package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Handler processes hook messages and generates responses
type Handler struct {
	parser          *Parser
	registry        *Registry
	ruleEngine      RuleEngine
	mu              sync.RWMutex
	lastMessageType HookEventName // Track the type of the last processed message
}

// NewHandler creates a new hook handler
func NewHandler(ruleEngine RuleEngine) *Handler {
	return &Handler{
		parser:     NewParser(),
		registry:   NewRegistry(),
		ruleEngine: ruleEngine,
	}
}

// ProcessInput reads hook message from stdin and processes it
func (h *Handler) ProcessInput(ctx context.Context) error {
	_, err := h.ProcessInputWithResponse(ctx)
	return err
}

// ProcessInputWithResponse reads hook message from stdin, processes it, and returns the response
func (h *Handler) ProcessInputWithResponse(ctx context.Context) (*HookResponse, error) {
	// Read from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdin: %w", err)
	}

	// Parse the message
	msg, err := h.parser.ParseHookMessage(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hook message: %w", err)
	}

	// Process the message
	response, err := h.ProcessMessage(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to process message: %w", err)
	}

	// Write response if needed
	if response != nil {
		responseData, err := h.parser.MarshalHookResponse(response)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		_, err = os.Stdout.Write(responseData)
		if err != nil {
			return nil, fmt.Errorf("failed to write response: %w", err)
		}
	}

	return response, nil
}

// ProcessMessage handles a specific hook message
func (h *Handler) ProcessMessage(ctx context.Context, msg HookMessage) (*HookResponse, error) {
	h.mu.Lock()
	// Store the message type before processing
	h.lastMessageType = msg.EventName()
	h.mu.Unlock()

	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.ruleEngine == nil {
		return nil, fmt.Errorf("no rule engine configured")
	}

	// Process based on message type
	switch m := msg.(type) {
	case *PreToolUseMessage:
		return h.handlePreToolUse(ctx, m)
	case *PostToolUseMessage:
		return h.handlePostToolUse(ctx, m)
	case *NotificationMessage:
		return h.handleNotification(ctx, m)
	case *StopMessage:
		return h.handleStop(ctx, m)
	case *SubagentStopMessage:
		return h.handleSubagentStop(ctx, m)
	case *PreCompactMessage:
		return h.handlePreCompact(ctx, m)
	case *UserPromptSubmitMessage:
		return h.handleUserPromptSubmit(ctx, m)
	case *SessionStartMessage:
		return h.handleSessionStart(ctx, m)
	default:
		return nil, fmt.Errorf("unknown message type: %T", msg)
	}
}

// SetRuleEngine updates the rule engine
func (h *Handler) SetRuleEngine(engine RuleEngine) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ruleEngine = engine
}

// IsPostToolUseHook returns true if the last processed message was a PostToolUse hook
func (h *Handler) IsPostToolUseHook() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastMessageType == PostToolUseEvent
}

func (h *Handler) handlePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error) {
	// Use rule engine to determine if tool use should be allowed
	decision, err := h.ruleEngine.EvaluatePreToolUse(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("rule evaluation failed: %w", err)
	}

	return decision, nil
}

func (h *Handler) handlePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error) {
	// Use rule engine to process tool output
	response, err := h.ruleEngine.EvaluatePostToolUse(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("rule evaluation failed: %w", err)
	}

	return response, nil
}

func (h *Handler) handleNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error) {
	// Process notification through rule engine
	return h.ruleEngine.EvaluateNotification(ctx, msg)
}

func (h *Handler) handleStop(ctx context.Context, msg *StopMessage) (*HookResponse, error) {
	// Process stop event through rule engine
	return h.ruleEngine.EvaluateStop(ctx, msg)
}

func (h *Handler) handleSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error) {
	// Process subagent stop through rule engine
	return h.ruleEngine.EvaluateSubagentStop(ctx, msg)
}

func (h *Handler) handlePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error) {
	// Process pre-compact event through rule engine
	return h.ruleEngine.EvaluatePreCompact(ctx, msg)
}

func (h *Handler) handleUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error) {
	// Process user prompt submit through rule engine
	return h.ruleEngine.EvaluateUserPromptSubmit(ctx, msg)
}

func (h *Handler) handleSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error) {
	// Process session start through rule engine
	return h.ruleEngine.EvaluateSessionStart(ctx, msg)
}

// Registry manages hook configurations
type Registry struct {
	mu    sync.RWMutex
	hooks map[HookEventName][]HookConfig
}

// HookConfig represents a hook configuration
type HookConfig struct {
	Name        string
	EventType   HookEventName
	ToolPattern string // Regex pattern for tool matching
	Priority    int
	Timeout     time.Duration
}

// NewRegistry creates a new hook registry
func NewRegistry() *Registry {
	return &Registry{
		hooks: make(map[HookEventName][]HookConfig),
	}
}

// Register adds a new hook configuration
func (r *Registry) Register(config HookConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.hooks[config.EventType] = append(r.hooks[config.EventType], config)
}

// GetHooks returns all hooks for a specific event type
func (r *Registry) GetHooks(eventType HookEventName) []HookConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.hooks[eventType]
}

// Clear removes all registered hooks
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.hooks = make(map[HookEventName][]HookConfig)
}
