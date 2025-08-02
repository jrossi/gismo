package engine

import (
	"context"
	"fmt"
)

// ActionHandlerEngine implements RuleEngine using the action handler system
type ActionHandlerEngine struct {
	registry *ActionHandlerRegistry
	config   *AppConfig
}

// NewActionHandlerEngine creates a new action handler engine
func NewActionHandlerEngine() *ActionHandlerEngine {
	return &ActionHandlerEngine{
		registry: NewActionHandlerRegistry(),
		config:   NewAppConfig(),
	}
}

// SetAppConfig sets the application configuration
func (e *ActionHandlerEngine) SetAppConfig(config *AppConfig) {
	e.config = config
}

// GetAppConfig returns the application configuration
func (e *ActionHandlerEngine) GetAppConfig() *AppConfig {
	return e.config
}

// RegisterHandler registers an action handler for specific event types
func (e *ActionHandlerEngine) RegisterHandler(eventType HookEventName, handler ActionHandler) {
	e.registry.Register(eventType, handler)
}

// GetRegistry returns the action handler registry
func (e *ActionHandlerEngine) GetRegistry() *ActionHandlerRegistry {
	return e.registry
}

// EvaluatePreToolUse evaluates whether a tool should be allowed to run
func (e *ActionHandlerEngine) EvaluatePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error) {
	handlers := e.registry.GetApplicableHandlers(ctx, msg)

	for _, handler := range handlers {
		if preHandler, ok := handler.(PreToolUseHandler); ok {
			response, err := preHandler.HandlePreToolUse(ctx, msg)
			if err != nil {
				return nil, fmt.Errorf("handler %s failed: %w", handler.Name(), err)
			}
			if response != nil && response.Decision == "block" {
				return response, nil
			}
		}
	}

	return &HookResponse{Decision: "approve"}, nil
}

// EvaluatePostToolUse processes the output of a tool after execution
func (e *ActionHandlerEngine) EvaluatePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error) {
	handlers := e.registry.GetApplicableHandlers(ctx, msg)

	for _, handler := range handlers {
		if postHandler, ok := handler.(PostToolUseHandler); ok {
			response, err := postHandler.HandlePostToolUse(ctx, msg)
			if err != nil {
				return nil, fmt.Errorf("handler %s failed: %w", handler.Name(), err)
			}
			if response != nil {
				return response, nil
			}
		}
	}

	return nil, nil
}

// EvaluateNotification processes system notifications
func (e *ActionHandlerEngine) EvaluateNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error) {
	handlers := e.registry.GetApplicableHandlers(ctx, msg)

	for _, handler := range handlers {
		if notificationHandler, ok := handler.(NotificationHandler); ok {
			response, err := notificationHandler.HandleNotification(ctx, msg)
			if err != nil {
				return nil, fmt.Errorf("handler %s failed: %w", handler.Name(), err)
			}
			if response != nil {
				return response, nil
			}
		}
	}

	return nil, nil
}

// EvaluateStop processes when the main agent finishes
func (e *ActionHandlerEngine) EvaluateStop(ctx context.Context, msg *StopMessage) (*HookResponse, error) {
	handlers := e.registry.GetApplicableHandlers(ctx, msg)

	for _, handler := range handlers {
		if stopHandler, ok := handler.(StopHandler); ok {
			response, err := stopHandler.HandleStop(ctx, msg)
			if err != nil {
				return nil, fmt.Errorf("handler %s failed: %w", handler.Name(), err)
			}
			if response != nil {
				return response, nil
			}
		}
	}

	return nil, nil
}

// EvaluateSubagentStop processes when a subagent completes
func (e *ActionHandlerEngine) EvaluateSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error) {
	handlers := e.registry.GetApplicableHandlers(ctx, msg)

	for _, handler := range handlers {
		if subagentStopHandler, ok := handler.(SubagentStopHandler); ok {
			response, err := subagentStopHandler.HandleSubagentStop(ctx, msg)
			if err != nil {
				return nil, fmt.Errorf("handler %s failed: %w", handler.Name(), err)
			}
			if response != nil {
				return response, nil
			}
		}
	}

	return nil, nil
}

// EvaluatePreCompact processes before context compression
func (e *ActionHandlerEngine) EvaluatePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error) {
	handlers := e.registry.GetApplicableHandlers(ctx, msg)

	for _, handler := range handlers {
		if preCompactHandler, ok := handler.(PreCompactHandler); ok {
			response, err := preCompactHandler.HandlePreCompact(ctx, msg)
			if err != nil {
				return nil, fmt.Errorf("handler %s failed: %w", handler.Name(), err)
			}
			if response != nil {
				return response, nil
			}
		}
	}

	return nil, nil
}

// EvaluateUserPromptSubmit processes when user submits a prompt
func (e *ActionHandlerEngine) EvaluateUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error) {
	handlers := e.registry.GetApplicableHandlers(ctx, msg)

	for _, handler := range handlers {
		if promptHandler, ok := handler.(UserPromptSubmitHandler); ok {
			response, err := promptHandler.HandleUserPromptSubmit(ctx, msg)
			if err != nil {
				return nil, fmt.Errorf("handler %s failed: %w", handler.Name(), err)
			}
			if response != nil {
				return response, nil
			}
		}
	}

	return nil, nil
}

// EvaluateSessionStart processes when a new Claude session begins
func (e *ActionHandlerEngine) EvaluateSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error) {
	handlers := e.registry.GetApplicableHandlers(ctx, msg)

	for _, handler := range handlers {
		if sessionStartHandler, ok := handler.(SessionStartHandler); ok {
			response, err := sessionStartHandler.HandleSessionStart(ctx, msg)
			if err != nil {
				return nil, fmt.Errorf("handler %s failed: %w", handler.Name(), err)
			}
			if response != nil {
				return response, nil
			}
		}
	}

	return nil, nil
}
