package engine

import (
	"context"
)

// RuleEngine defines the interface for evaluating hook messages
type RuleEngine interface {
	// EvaluatePreToolUse evaluates whether a tool should be allowed to run
	EvaluatePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error)

	// EvaluatePostToolUse processes the output of a tool after execution
	EvaluatePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error)

	// EvaluateNotification processes system notifications
	EvaluateNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error)

	// EvaluateStop processes when the main agent finishes
	EvaluateStop(ctx context.Context, msg *StopMessage) (*HookResponse, error)

	// EvaluateSubagentStop processes when a subagent completes
	EvaluateSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error)

	// EvaluatePreCompact processes before context compression
	EvaluatePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error)

	// EvaluateUserPromptSubmit processes when user submits a prompt
	EvaluateUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error)

	// EvaluateSessionStart processes when a new Claude session begins
	EvaluateSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error)
}

// BaseRuleEngine provides a default implementation of RuleEngine
type BaseRuleEngine struct{}

// NewBaseRuleEngine creates a new base rule engine with default behavior
func NewBaseRuleEngine() *BaseRuleEngine {
	return &BaseRuleEngine{}
}

// EvaluatePreToolUse provides default pre-tool-use evaluation
func (e *BaseRuleEngine) EvaluatePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error) {
	// Default: approve all tool uses
	return &HookResponse{
		Decision: "approve",
	}, nil
}

// EvaluatePostToolUse provides default post-tool-use evaluation
func (e *BaseRuleEngine) EvaluatePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error) {
	// Default: continue without modification
	return nil, nil
}

// EvaluateNotification provides default notification evaluation
func (e *BaseRuleEngine) EvaluateNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error) {
	// Default: continue without modification
	return nil, nil
}

// EvaluateStop provides default stop evaluation
func (e *BaseRuleEngine) EvaluateStop(ctx context.Context, msg *StopMessage) (*HookResponse, error) {
	// Default: continue without modification
	return nil, nil
}

// EvaluateSubagentStop provides default subagent stop evaluation
func (e *BaseRuleEngine) EvaluateSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error) {
	// Default: continue without modification
	return nil, nil
}

// EvaluatePreCompact provides default pre-compact evaluation
func (e *BaseRuleEngine) EvaluatePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error) {
	// Default: continue without modification
	return nil, nil
}

// EvaluateUserPromptSubmit provides default user prompt submit evaluation
func (e *BaseRuleEngine) EvaluateUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error) {
	// Default: continue without modification
	return nil, nil
}

// EvaluateSessionStart provides default session start evaluation
func (e *BaseRuleEngine) EvaluateSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error) {
	// Default: continue without modification
	return nil, nil
}

// CompositeRuleEngine combines multiple rule engines
type CompositeRuleEngine struct {
	engines []RuleEngine
}

// NewCompositeRuleEngine creates a new composite rule engine
func NewCompositeRuleEngine(engines ...RuleEngine) *CompositeRuleEngine {
	return &CompositeRuleEngine{
		engines: engines,
	}
}

// AddEngine adds a rule engine to the composite
func (c *CompositeRuleEngine) AddEngine(engine RuleEngine) {
	c.engines = append(c.engines, engine)
}

// EvaluatePreToolUse runs all engines and returns the first blocking response
func (c *CompositeRuleEngine) EvaluatePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error) {
	for _, engine := range c.engines {
		response, err := engine.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			return nil, err
		}
		if response != nil && response.Decision == "block" {
			return response, nil
		}
	}
	return &HookResponse{Decision: "approve"}, nil
}

// EvaluatePostToolUse runs all engines and returns the first non-nil response
func (c *CompositeRuleEngine) EvaluatePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error) {
	for _, engine := range c.engines {
		response, err := engine.EvaluatePostToolUse(ctx, msg)
		if err != nil {
			return nil, err
		}
		if response != nil {
			return response, nil
		}
	}
	return nil, nil
}

// EvaluateNotification runs all engines and returns the first non-nil response
func (c *CompositeRuleEngine) EvaluateNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error) {
	for _, engine := range c.engines {
		response, err := engine.EvaluateNotification(ctx, msg)
		if err != nil {
			return nil, err
		}
		if response != nil {
			return response, nil
		}
	}
	return nil, nil
}

// EvaluateStop runs all engines and returns the first non-nil response
func (c *CompositeRuleEngine) EvaluateStop(ctx context.Context, msg *StopMessage) (*HookResponse, error) {
	for _, engine := range c.engines {
		response, err := engine.EvaluateStop(ctx, msg)
		if err != nil {
			return nil, err
		}
		if response != nil {
			return response, nil
		}
	}
	return nil, nil
}

// EvaluateSubagentStop runs all engines and returns the first non-nil response
func (c *CompositeRuleEngine) EvaluateSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error) {
	for _, engine := range c.engines {
		response, err := engine.EvaluateSubagentStop(ctx, msg)
		if err != nil {
			return nil, err
		}
		if response != nil {
			return response, nil
		}
	}
	return nil, nil
}

// EvaluatePreCompact runs all engines and returns the first non-nil response
func (c *CompositeRuleEngine) EvaluatePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error) {
	for _, engine := range c.engines {
		response, err := engine.EvaluatePreCompact(ctx, msg)
		if err != nil {
			return nil, err
		}
		if response != nil {
			return response, nil
		}
	}
	return nil, nil
}

// EvaluateUserPromptSubmit runs all engines and returns the first non-nil response
func (c *CompositeRuleEngine) EvaluateUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error) {
	for _, engine := range c.engines {
		response, err := engine.EvaluateUserPromptSubmit(ctx, msg)
		if err != nil {
			return nil, err
		}
		if response != nil {
			return response, nil
		}
	}
	return nil, nil
}

// EvaluateSessionStart runs all engines and returns the first non-nil response
func (c *CompositeRuleEngine) EvaluateSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error) {
	for _, engine := range c.engines {
		response, err := engine.EvaluateSessionStart(ctx, msg)
		if err != nil {
			return nil, err
		}
		if response != nil {
			return response, nil
		}
	}
	return nil, nil
}
