package engine

import (
	"context"
	"errors"
	"testing"
)

// Mock engine that returns errors
type errorRuleEngine struct{}

func (e *errorRuleEngine) EvaluatePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error) {
	return nil, errors.New("pre tool use error")
}

func (e *errorRuleEngine) EvaluatePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error) {
	return nil, errors.New("post tool use error")
}

func (e *errorRuleEngine) EvaluateNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error) {
	return nil, errors.New("notification error")
}

func (e *errorRuleEngine) EvaluateStop(ctx context.Context, msg *StopMessage) (*HookResponse, error) {
	return nil, errors.New("stop error")
}

func (e *errorRuleEngine) EvaluateSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error) {
	return nil, errors.New("subagent stop error")
}

func (e *errorRuleEngine) EvaluatePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error) {
	return nil, errors.New("pre compact error")
}

func (e *errorRuleEngine) EvaluateUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error) {
	return nil, errors.New("user prompt submit error")
}

func (e *errorRuleEngine) EvaluateSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error) {
	return nil, errors.New("session start error")
}

func TestCompositeRuleEngine_ErrorHandling(t *testing.T) {
	composite := NewCompositeRuleEngine()
	composite.AddEngine(&errorRuleEngine{})

	ctx := context.Background()

	t.Run("PreToolUse_error", func(t *testing.T) {
		msg := &PreToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: PreToolUseEvent,
			},
			ToolName: "Write",
		}

		resp, err := composite.EvaluatePreToolUse(ctx, msg)
		if err == nil {
			t.Error("Expected error, got none")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("PostToolUse_error", func(t *testing.T) {
		msg := &PostToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: PostToolUseEvent,
			},
			ToolName: "Write",
		}

		resp, err := composite.EvaluatePostToolUse(ctx, msg)
		if err == nil {
			t.Error("Expected error, got none")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("Notification_error", func(t *testing.T) {
		msg := &NotificationMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: NotificationEvent,
			},
		}

		resp, err := composite.EvaluateNotification(ctx, msg)
		if err == nil {
			t.Error("Expected error, got none")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("Stop_error", func(t *testing.T) {
		msg := &StopMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: StopEvent,
			},
		}

		resp, err := composite.EvaluateStop(ctx, msg)
		if err == nil {
			t.Error("Expected error, got none")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("SubagentStop_error", func(t *testing.T) {
		msg := &SubagentStopMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: SubagentStopEvent,
			},
		}

		resp, err := composite.EvaluateSubagentStop(ctx, msg)
		if err == nil {
			t.Error("Expected error, got none")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("PreCompact_error", func(t *testing.T) {
		msg := &PreCompactMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: PreCompactEvent,
			},
		}

		resp, err := composite.EvaluatePreCompact(ctx, msg)
		if err == nil {
			t.Error("Expected error, got none")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})
}

// Mock engine that returns specific responses
type customRuleEngine struct {
	preResponse      *HookResponse
	postResponse     *HookResponse
	notifResponse    *HookResponse
	stopResponse     *HookResponse
	subagentResponse *HookResponse
	compactResponse  *HookResponse
}

func (e *customRuleEngine) EvaluatePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error) {
	return e.preResponse, nil
}

func (e *customRuleEngine) EvaluatePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error) {
	return e.postResponse, nil
}

func (e *customRuleEngine) EvaluateNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error) {
	return e.notifResponse, nil
}

func (e *customRuleEngine) EvaluateStop(ctx context.Context, msg *StopMessage) (*HookResponse, error) {
	return e.stopResponse, nil
}

func (e *customRuleEngine) EvaluateSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error) {
	return e.subagentResponse, nil
}

func (e *customRuleEngine) EvaluatePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error) {
	return e.compactResponse, nil
}

func (e *customRuleEngine) EvaluateUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error) {
	return nil, nil
}

func (e *customRuleEngine) EvaluateSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error) {
	return nil, nil
}

func TestCompositeRuleEngine_MultipleEngines(t *testing.T) {
	composite := NewCompositeRuleEngine()

	// Add multiple engines with different responses
	engine1 := &customRuleEngine{
		preResponse:      nil, // No response
		postResponse:     &HookResponse{Decision: "approve"},
		notifResponse:    nil,
		stopResponse:     &HookResponse{Decision: "logged"},
		subagentResponse: nil,
		compactResponse:  &HookResponse{Decision: "proceed"},
	}

	engine2 := &customRuleEngine{
		preResponse:      &HookResponse{Decision: "approve"},
		postResponse:     nil, // No response
		notifResponse:    &HookResponse{Decision: "acknowledged"},
		stopResponse:     nil,
		subagentResponse: &HookResponse{Decision: "handled"},
		compactResponse:  nil,
	}

	composite.AddEngine(engine1)
	composite.AddEngine(engine2)

	ctx := context.Background()

	t.Run("PreToolUse_second_engine_responds", func(t *testing.T) {
		msg := &PreToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: PreToolUseEvent,
			},
		}

		resp, err := composite.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if resp == nil || resp.Decision != "approve" {
			t.Errorf("Expected approve from second engine, got %v", resp)
		}
	})

	t.Run("PostToolUse_first_engine_responds", func(t *testing.T) {
		msg := &PostToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: PostToolUseEvent,
			},
		}

		resp, err := composite.EvaluatePostToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if resp == nil || resp.Decision != "approve" {
			t.Errorf("Expected approve from first engine, got %v", resp)
		}
	})

	t.Run("All_engines_return_nil", func(t *testing.T) {
		// Create composite with engines that all return nil
		composite2 := NewCompositeRuleEngine()
		composite2.AddEngine(&customRuleEngine{})
		composite2.AddEngine(&customRuleEngine{})

		msg := &PreToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: PreToolUseEvent,
			},
		}

		resp, err := composite2.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if resp == nil || resp.Decision != "approve" {
			t.Errorf("Expected approve when all engines return nil, got %v", resp)
		}
	})
}

func TestCompositeRuleEngine_ContextCancellation(t *testing.T) {
	composite := NewCompositeRuleEngine()

	// Add a slow engine
	slowEngine := &BaseRuleEngine{}
	composite.AddEngine(slowEngine)

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// All methods should handle canceled context gracefully
	t.Run("PreToolUse_canceled", func(t *testing.T) {
		msg := &PreToolUseMessage{}
		resp, err := composite.EvaluatePreToolUse(ctx, msg)
		// Should not error on canceled context for these simple operations
		if err != nil {
			t.Logf("Got error with canceled context: %v", err)
		}
		if resp == nil {
			t.Log("Got nil response as expected")
		}
	})
}

func TestBaseRuleEngine_UnusedMethods(t *testing.T) {
	engine := NewBaseRuleEngine()
	ctx := context.Background()

	// Test Stop
	stopResp, err := engine.EvaluateStop(ctx, &StopMessage{})
	if err != nil {
		t.Errorf("EvaluateStop() error = %v", err)
	}
	if stopResp != nil {
		t.Errorf("Expected nil response for Stop, got %v", stopResp)
	}

	// Test SubagentStop
	subagentResp, err := engine.EvaluateSubagentStop(ctx, &SubagentStopMessage{})
	if err != nil {
		t.Errorf("EvaluateSubagentStop() error = %v", err)
	}
	if subagentResp != nil {
		t.Errorf("Expected nil response for SubagentStop, got %v", subagentResp)
	}

	// Test PreCompact
	compactResp, err := engine.EvaluatePreCompact(ctx, &PreCompactMessage{})
	if err != nil {
		t.Errorf("EvaluatePreCompact() error = %v", err)
	}
	if compactResp != nil {
		t.Errorf("Expected nil response for PreCompact, got %v", compactResp)
	}
}

func TestCompositeRuleEngine_EmptyEngines(t *testing.T) {
	composite := &CompositeRuleEngine{
		engines: []RuleEngine{}, // Empty engines slice
	}

	ctx := context.Background()

	// All methods should handle empty engines gracefully
	tests := []struct {
		name          string
		fn            func() (*HookResponse, error)
		expectApprove bool
	}{
		{
			"PreToolUse",
			func() (*HookResponse, error) {
				return composite.EvaluatePreToolUse(ctx, &PreToolUseMessage{})
			},
			true, // PreToolUse returns approve by default
		},
		{
			"PostToolUse",
			func() (*HookResponse, error) {
				return composite.EvaluatePostToolUse(ctx, &PostToolUseMessage{})
			},
			false, // Other events return nil
		},
		{
			"Notification",
			func() (*HookResponse, error) {
				return composite.EvaluateNotification(ctx, &NotificationMessage{})
			},
			false,
		},
		{
			"Stop",
			func() (*HookResponse, error) {
				return composite.EvaluateStop(ctx, &StopMessage{})
			},
			false,
		},
		{
			"SubagentStop",
			func() (*HookResponse, error) {
				return composite.EvaluateSubagentStop(ctx, &SubagentStopMessage{})
			},
			false,
		},
		{
			"PreCompact",
			func() (*HookResponse, error) {
				return composite.EvaluatePreCompact(ctx, &PreCompactMessage{})
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.fn()
			if err != nil {
				t.Errorf("Unexpected error with empty engines: %v", err)
			}
			if tt.expectApprove {
				if resp == nil || resp.Decision != "approve" {
					t.Errorf("Expected approve response with empty engines, got %v", resp)
				}
			} else {
				if resp != nil {
					t.Errorf("Expected nil response with empty engines, got %v", resp)
				}
			}
		})
	}
}
