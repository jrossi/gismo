package gismo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/goccy/go-json"
)

func TestAPI_New(t *testing.T) {
	api := New()
	if api == nil {
		t.Fatal("New() returned nil")
	}

	// Should have default components
	if api.executor == nil {
		t.Error("Expected executor to be initialized")
	}
	if api.parser == nil {
		t.Error("Expected parser to be initialized")
	}
}

func TestAPI_NewWithRuleEngine(t *testing.T) {
	engine := NewBaseRuleEngine()
	api := NewWithRuleEngine(engine)

	if api == nil {
		t.Fatal("NewWithRuleEngine() returned nil")
	}

	// Should use provided engine
	if api.executor == nil {
		t.Error("Expected executor to be initialized")
	}
}

func TestAPI_ProcessStdin(t *testing.T) {
	// Save original stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create test input
	input := `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write","tool_input":{"file_path":"test.go","content":"package main\n"}}`
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	// Write test data
	go func() {
		_, _ = w.Write([]byte(input))
		w.Close()
	}()

	api := New()
	ctx := context.Background()

	err = api.ProcessStdin(ctx)
	if err != nil {
		t.Errorf("ProcessStdin() error = %v", err)
	}
}

func TestAPI_ProcessMessage(t *testing.T) {
	api := New()
	ctx := context.Background()

	msg := &PreToolUseMessage{
		BaseHookMessage: BaseHookMessage{
			SessionID:     "test-session",
			HookEventName: PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "test.go",
			"content":   "package main\n",
		}),
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	resp, err := api.ProcessMessage(ctx, msgData)
	if err != nil {
		t.Fatalf("ProcessMessage() error = %v", err)
	}

	// Default engine should approve
	if resp == nil || resp.Decision != "approve" {
		t.Errorf("Expected approve response, got %v", resp)
	}
}

func TestAPI_SetRuleEngine(t *testing.T) {
	api := New()

	// Create custom engine
	customEngine := &customRuleEngine{
		preResponse: &HookResponse{
			Decision: "custom",
			Message:  "Custom engine response",
		},
	}

	api.SetRuleEngine(customEngine)

	// Test that custom engine is used
	ctx := context.Background()
	msg := &PreToolUseMessage{
		BaseHookMessage: BaseHookMessage{
			HookEventName: PreToolUseEvent,
		},
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	resp, err := api.ProcessMessage(ctx, msgData)
	if err != nil {
		t.Fatalf("ProcessMessage() error = %v", err)
	}

	if resp == nil || resp.Decision != "custom" {
		t.Errorf("Expected custom response, got %v", resp)
	}
}

func TestAPI_SetTimeout(t *testing.T) {
	api := New()
	api.SetTimeout(100 * time.Millisecond)

	// Verify timeout is set
	if api.executor.timeout != 100*time.Millisecond {
		t.Errorf("Expected timeout to be 100ms, got %v", api.executor.timeout)
	}
}

func TestAPI_GetRegistry(t *testing.T) {
	api := New()
	registry := api.GetRegistry()

	if registry == nil {
		t.Fatal("GetRegistry() returned nil")
	}

	// Should be able to register hooks
	registry.Register(HookConfig{
		Name:      "test-hook",
		EventType: PreToolUseEvent,
	})

	hooks := registry.GetHooks(PreToolUseEvent)
	if len(hooks) != 1 {
		t.Errorf("Expected 1 hook, got %d", len(hooks))
	}
}

func TestAPI_ParseHookMessage(t *testing.T) {
	api := New()

	input := `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write"}`
	msg, err := api.ParseHookMessage([]byte(input))

	if err != nil {
		t.Fatalf("ParseHookMessage() error = %v", err)
	}

	if _, ok := msg.(*PreToolUseMessage); !ok {
		t.Errorf("Expected *PreToolUseMessage, got %T", msg)
	}
}

func TestAPI_ParseHookResponse(t *testing.T) {
	api := New()

	input := `{"decision":"block","reason":"Test reason"}`
	resp, err := api.ParseHookResponse([]byte(input))

	if err != nil {
		t.Fatalf("ParseHookResponse() error = %v", err)
	}

	if resp.Decision != "block" {
		t.Errorf("Expected block decision, got %s", resp.Decision)
	}
}

func TestAPI_MarshalHookResponse(t *testing.T) {
	api := New()

	resp := &HookResponse{
		Decision: "approve",
		Message:  "All good",
	}

	data, err := api.MarshalHookResponse(resp)
	if err != nil {
		t.Fatalf("MarshalHookResponse() error = %v", err)
	}

	// Verify it's valid JSON
	var decoded HookResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}

	if decoded.Decision != "approve" {
		t.Errorf("Expected approve decision in marshaled data")
	}
}

func TestAPI_NewWithConfig(t *testing.T) {
	config := Config{
		RuleEngine: NewLintingRuleEngine(),
		Timeout:    30 * time.Second,
	}

	api := NewWithConfig(config)

	if api == nil {
		t.Fatal("NewWithConfig() returned nil")
	}

	// Verify config was applied
	if api.executor.timeout != 30*time.Second {
		t.Errorf("Expected timeout from config, got %v", api.executor.timeout)
	}
}

func TestBuilder(t *testing.T) {
	// Test builder pattern
	builder := NewBuilder()

	engine := NewLintingRuleEngine()
	timeout := 45 * time.Second

	api := builder.
		WithRuleEngine(engine).
		WithTimeout(timeout).
		RegisterHook(HookConfig{
			Name:      "test-hook",
			EventType: PreToolUseEvent,
		}).
		Build()

	if api == nil {
		t.Fatal("Build() returned nil")
	}

	// Verify configuration
	if api.executor.timeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, api.executor.timeout)
	}

	// Verify hook was registered
	hooks := api.GetRegistry().GetHooks(PreToolUseEvent)
	if len(hooks) != 1 {
		t.Errorf("Expected 1 hook, got %d", len(hooks))
	}
}

func TestCreateBlockingEngine(t *testing.T) {
	blockedTools := []string{"Execute", "Delete"}
	qs := QuickStart{}
	engine := qs.CreateBlockingEngine(blockedTools...)

	ctx := context.Background()

	t.Run("allowed_tool", func(t *testing.T) {
		msg := &PreToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: PreToolUseEvent,
			},
			ToolName: "Write",
		}

		resp, err := engine.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should approve non-blocked tool
		if resp == nil || resp.Decision != "approve" {
			t.Errorf("Expected approve for non-blocked tool, got %v", resp)
		}
	})

	t.Run("blocked_tool", func(t *testing.T) {
		msg := &PreToolUseMessage{
			BaseHookMessage: BaseHookMessage{
				HookEventName: PreToolUseEvent,
			},
			ToolName: "Execute",
		}

		resp, err := engine.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should block blocked tool
		if resp == nil || resp.Decision != "block" {
			t.Errorf("Expected block for blocked tool, got %v", resp)
		}
	})
}

func TestBlockingEngine_AllMethods(t *testing.T) {
	engine := &blockingEngine{
		blockedTools: []string{"Execute", "Delete"},
	}

	ctx := context.Background()

	// Test PostToolUse
	postResp, err := engine.EvaluatePostToolUse(ctx, &PostToolUseMessage{})
	if err != nil {
		t.Errorf("EvaluatePostToolUse() error = %v", err)
	}
	if postResp != nil {
		t.Errorf("Expected nil response for PostToolUse")
	}

	// Test Notification
	notifResp, err := engine.EvaluateNotification(ctx, &NotificationMessage{})
	if err != nil {
		t.Errorf("EvaluateNotification() error = %v", err)
	}
	if notifResp != nil {
		t.Errorf("Expected nil response for Notification")
	}

	// Test Stop
	stopResp, err := engine.EvaluateStop(ctx, &StopMessage{})
	if err != nil {
		t.Errorf("EvaluateStop() error = %v", err)
	}
	if stopResp != nil {
		t.Errorf("Expected nil response for Stop")
	}

	// Test SubagentStop
	subResp, err := engine.EvaluateSubagentStop(ctx, &SubagentStopMessage{})
	if err != nil {
		t.Errorf("EvaluateSubagentStop() error = %v", err)
	}
	if subResp != nil {
		t.Errorf("Expected nil response for SubagentStop")
	}

	// Test PreCompact
	compactResp, err := engine.EvaluatePreCompact(ctx, &PreCompactMessage{})
	if err != nil {
		t.Errorf("EvaluatePreCompact() error = %v", err)
	}
	if compactResp != nil {
		t.Errorf("Expected nil response for PreCompact")
	}
}

func TestAPI_CompleteWorkflow(t *testing.T) {
	// Test a complete workflow
	api := NewBuilder().
		WithRuleEngine(NewLintingRuleEngine()).
		WithTimeout(5 * time.Second).
		Build()

	ctx := context.Background()

	// Create a message
	msg := &PreToolUseMessage{
		BaseHookMessage: BaseHookMessage{
			SessionID:     "test-session",
			HookEventName: PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "test.go",
			"content":   "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n",
		}),
	}

	// Marshal message
	msgData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Process it
	resp, err := api.ProcessMessage(ctx, msgData)
	if err != nil {
		t.Fatalf("ProcessMessage() error = %v", err)
	}

	// Should get a response (approve or block based on linting)
	if resp == nil {
		t.Error("Expected response, got nil")
	}

	// Marshal the response
	respData, err := api.MarshalHookResponse(resp)
	if err != nil {
		t.Fatalf("MarshalHookResponse() error = %v", err)
	}

	// Parse it back
	parsedResp, err := api.ParseHookResponse(respData)
	if err != nil {
		t.Fatalf("ParseHookResponse() error = %v", err)
	}

	if parsedResp.Decision != resp.Decision {
		t.Errorf("Decision mismatch after round trip: %s != %s", parsedResp.Decision, resp.Decision)
	}
}
