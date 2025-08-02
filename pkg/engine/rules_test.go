package engine

import (
	"context"
	"testing"
)

func TestBaseRuleEngine(t *testing.T) {
	engine := NewBaseRuleEngine()
	ctx := context.Background()

	t.Run("PreToolUse", func(t *testing.T) {
		msg := &PreToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
			ToolName: "Write",
		}
		resp, err := engine.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluatePreToolUse() error = %v", err)
		}
		if resp.Decision != "approve" {
			t.Errorf("expected approve decision, got %v", resp.Decision)
		}
	})

	t.Run("PostToolUse", func(t *testing.T) {
		msg := &PostToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
			ToolName: "Read",
		}
		resp, err := engine.EvaluatePostToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluatePostToolUse() error = %v", err)
		}
		if resp != nil {
			t.Errorf("expected nil response, got %v", resp)
		}
	})

	t.Run("Notification", func(t *testing.T) {
		msg := &NotificationMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
			NotificationType: "info",
		}
		resp, err := engine.EvaluateNotification(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluateNotification() error = %v", err)
		}
		if resp != nil {
			t.Errorf("expected nil response, got %v", resp)
		}
	})
}

func TestCompositeRuleEngine(t *testing.T) {
	ctx := context.Background()

	t.Run("Multiple engines with blocking", func(t *testing.T) {
		// Create engines with different responses
		engine1 := &MockRuleEngine{
			preToolUseResponse: &HookResponse{Decision: "approve"},
		}
		engine2 := &MockRuleEngine{
			preToolUseResponse: &HookResponse{Decision: "block", Reason: "security policy"},
		}
		engine3 := &MockRuleEngine{
			preToolUseResponse: &HookResponse{Decision: "approve"},
		}

		composite := NewCompositeRuleEngine(engine1, engine2, engine3)

		msg := &PreToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
			ToolName: "Write",
		}

		resp, err := composite.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluatePreToolUse() error = %v", err)
		}

		// Should return the first blocking response
		if resp.Decision != "block" {
			t.Errorf("expected block decision, got %v", resp.Decision)
		}
		if resp.Reason != "security policy" {
			t.Errorf("expected 'security policy' reason, got %v", resp.Reason)
		}

		// Verify all engines before the blocking one were called
		if !engine1.preToolUseCalled {
			t.Error("engine1 should have been called")
		}
		if !engine2.preToolUseCalled {
			t.Error("engine2 should have been called")
		}
		// engine3 might not be called since engine2 blocked
	})

	t.Run("All engines approve", func(t *testing.T) {
		engine1 := &MockRuleEngine{
			preToolUseResponse: &HookResponse{Decision: "approve"},
		}
		engine2 := &MockRuleEngine{
			preToolUseResponse: &HookResponse{Decision: "approve"},
		}

		composite := NewCompositeRuleEngine(engine1, engine2)

		msg := &PreToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
			ToolName: "Read",
		}

		resp, err := composite.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluatePreToolUse() error = %v", err)
		}

		if resp.Decision != "approve" {
			t.Errorf("expected approve decision, got %v", resp.Decision)
		}
	})

	t.Run("PostToolUse first non-nil wins", func(t *testing.T) {
		engine1 := &MockRuleEngine{
			postToolUseResponse: nil,
		}
		engine2 := &MockRuleEngine{
			postToolUseResponse: &HookResponse{
				Message: "processed by engine2",
			},
		}
		engine3 := &MockRuleEngine{
			postToolUseResponse: &HookResponse{
				Message: "processed by engine3",
			},
		}

		composite := NewCompositeRuleEngine(engine1, engine2, engine3)

		msg := &PostToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
			ToolName: "Read",
		}

		resp, err := composite.EvaluatePostToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluatePostToolUse() error = %v", err)
		}

		// Should return engine2's response (first non-nil)
		if resp.Message != "processed by engine2" {
			t.Errorf("expected engine2's response, got %v", resp.Message)
		}
	})

	t.Run("AddEngine", func(t *testing.T) {
		composite := NewCompositeRuleEngine()

		// Initially no engines
		msg := &PreToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
			ToolName: "Write",
		}

		resp, err := composite.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluatePreToolUse() error = %v", err)
		}
		if resp.Decision != "approve" {
			t.Errorf("expected approve by default, got %v", resp.Decision)
		}

		// Add a blocking engine
		blockingEngine := &MockRuleEngine{
			preToolUseResponse: &HookResponse{Decision: "block"},
		}
		composite.AddEngine(blockingEngine)

		resp, err = composite.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluatePreToolUse() error = %v", err)
		}
		if resp.Decision != "block" {
			t.Errorf("expected block after adding engine, got %v", resp.Decision)
		}
	})
}

func TestCompositeRuleEngine_AllMethods(t *testing.T) {
	// Test that all methods properly aggregate responses
	ctx := context.Background()

	engine1 := &MockRuleEngine{}
	engine2 := &MockRuleEngine{
		notificationResponse: &HookResponse{Message: "notification handled"},
		stopResponse:         &HookResponse{Message: "stop handled"},
		subagentStopResponse: &HookResponse{Message: "subagent stop handled"},
		preCompactResponse:   &HookResponse{Message: "precompact handled"},
	}

	composite := NewCompositeRuleEngine(engine1, engine2)

	t.Run("Notification", func(t *testing.T) {
		msg := &NotificationMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
		}
		resp, err := composite.EvaluateNotification(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluateNotification() error = %v", err)
		}
		if resp.Message != "notification handled" {
			t.Errorf("expected engine2's response, got %v", resp.Message)
		}
	})

	t.Run("Stop", func(t *testing.T) {
		msg := &StopMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
		}
		resp, err := composite.EvaluateStop(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluateStop() error = %v", err)
		}
		if resp.Message != "stop handled" {
			t.Errorf("expected engine2's response, got %v", resp.Message)
		}
	})

	t.Run("SubagentStop", func(t *testing.T) {
		msg := &SubagentStopMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
		}
		resp, err := composite.EvaluateSubagentStop(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluateSubagentStop() error = %v", err)
		}
		if resp.Message != "subagent stop handled" {
			t.Errorf("expected engine2's response, got %v", resp.Message)
		}
	})

	t.Run("PreCompact", func(t *testing.T) {
		msg := &PreCompactMessage{
			BaseHookMessage: BaseHookMessage{
				SessionID: "test",
			},
		}
		resp, err := composite.EvaluatePreCompact(ctx, msg)
		if err != nil {
			t.Fatalf("EvaluatePreCompact() error = %v", err)
		}
		if resp.Message != "precompact handled" {
			t.Errorf("expected engine2's response, got %v", resp.Message)
		}
	})
}
