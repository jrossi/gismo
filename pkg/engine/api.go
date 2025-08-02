package engine

import (
	"context"
	"fmt"
	"time"
)

// API provides the main interface for the gismo library
type API struct {
	executor *Executor
	parser   *Parser
}

// New creates a new API instance with a default rule engine
func New() *API {
	return NewWithRuleEngine(NewBaseRuleEngine())
}

// NewWithRuleEngine creates a new API instance with a custom rule engine
func NewWithRuleEngine(engine RuleEngine) *API {
	return &API{
		executor: NewExecutor(engine),
		parser:   NewParser(),
	}
}

// ProcessStdin processes a hook message from stdin and writes response to stdout
func (a *API) ProcessStdin(ctx context.Context) error {
	return a.executor.Execute(ctx)
}

// ProcessMessage processes a hook message and returns the response
func (a *API) ProcessMessage(ctx context.Context, msgData []byte) (*HookResponse, error) {
	msg, err := a.parser.ParseHookMessage(msgData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	return a.executor.handler.ProcessMessage(ctx, msg)
}

// SetRuleEngine updates the rule engine
func (a *API) SetRuleEngine(engine RuleEngine) {
	a.executor.SetRuleEngine(engine)
}

// SetTimeout updates the execution timeout
func (a *API) SetTimeout(timeout time.Duration) {
	a.executor.SetTimeout(timeout)
}

// GetRegistry returns the hook registry for configuration
func (a *API) GetRegistry() *Registry {
	return a.executor.GetRegistry()
}

// ParseHookMessage parses a hook message from JSON
func (a *API) ParseHookMessage(data []byte) (HookMessage, error) {
	return a.parser.ParseHookMessage(data)
}

// ParseHookResponse parses a hook response from JSON
func (a *API) ParseHookResponse(data []byte) (*HookResponse, error) {
	return a.parser.ParseHookResponse(data)
}

// MarshalHookResponse serializes a hook response to JSON
func (a *API) MarshalHookResponse(response *HookResponse) ([]byte, error) {
	return a.parser.MarshalHookResponse(response)
}

// Config provides configuration options for the API
type Config struct {
	Timeout    time.Duration
	RuleEngine RuleEngine
}

// NewWithConfig creates a new API instance with configuration
func NewWithConfig(cfg Config) *API {
	api := New()

	if cfg.Timeout > 0 {
		api.SetTimeout(cfg.Timeout)
	}

	if cfg.RuleEngine != nil {
		api.SetRuleEngine(cfg.RuleEngine)
	}

	return api
}

// Builder provides a fluent interface for creating an API instance
type Builder struct {
	timeout    time.Duration
	ruleEngine RuleEngine
	registry   *Registry
}

// NewBuilder creates a new API builder
func NewBuilder() *Builder {
	return &Builder{
		timeout:  60 * time.Second,
		registry: NewRegistry(),
	}
}

// WithTimeout sets the execution timeout
func (b *Builder) WithTimeout(timeout time.Duration) *Builder {
	b.timeout = timeout
	return b
}

// WithRuleEngine sets the rule engine
func (b *Builder) WithRuleEngine(engine RuleEngine) *Builder {
	b.ruleEngine = engine
	return b
}

// RegisterHook adds a hook configuration
func (b *Builder) RegisterHook(config HookConfig) *Builder {
	b.registry.Register(config)
	return b
}

// Build creates the API instance
func (b *Builder) Build() *API {
	if b.ruleEngine == nil {
		b.ruleEngine = NewBaseRuleEngine()
	}

	api := NewWithRuleEngine(b.ruleEngine)
	api.SetTimeout(b.timeout)

	// Copy registry hooks
	for _, hooks := range b.registry.hooks {
		for _, hook := range hooks {
			api.GetRegistry().Register(hook)
		}
	}

	return api
}

// QuickStart provides common API usage patterns
type QuickStart struct{}

// CreateBlockingEngine creates a rule engine that blocks specific tools
func (qs QuickStart) CreateBlockingEngine(blockedTools ...string) RuleEngine {
	return &blockingEngine{blockedTools: blockedTools}
}

type blockingEngine struct {
	blockedTools []string
}

func (e *blockingEngine) EvaluatePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error) {
	for _, blocked := range e.blockedTools {
		if msg.ToolName == blocked {
			return &HookResponse{
				Decision: "block",
				Reason:   fmt.Sprintf("Tool %s is blocked by policy", msg.ToolName),
			}, nil
		}
	}
	return &HookResponse{Decision: "approve"}, nil
}

func (e *blockingEngine) EvaluatePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error) {
	return nil, nil
}

func (e *blockingEngine) EvaluateNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error) {
	return nil, nil
}

func (e *blockingEngine) EvaluateStop(ctx context.Context, msg *StopMessage) (*HookResponse, error) {
	return nil, nil
}

func (e *blockingEngine) EvaluateSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error) {
	return nil, nil
}

func (e *blockingEngine) EvaluatePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error) {
	return nil, nil
}

func (e *blockingEngine) EvaluateUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error) {
	return nil, nil
}

func (e *blockingEngine) EvaluateSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error) {
	return nil, nil
}
