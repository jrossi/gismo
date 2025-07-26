package gismo

import (
	"context"
	"os"
	"strings"
	"testing"
)

// mockUnknownMessage is a test type that implements HookMessage but isn't recognized
type mockUnknownMessage struct {
	BaseHookMessage
}

func (m mockUnknownMessage) GetBaseMessage() BaseHookMessage { return m.BaseHookMessage }
func (m mockUnknownMessage) EventName() HookEventName        { return "UnknownEvent" }

func TestHandler_ProcessInput(t *testing.T) {
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

	handler := NewHandler(NewBaseRuleEngine())
	err = handler.ProcessInput(context.Background())

	if err != nil {
		t.Errorf("ProcessInput() error = %v", err)
	}
}

func TestHandler_ProcessInputWithResponse(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		setupEngine  func() RuleEngine
		wantResponse bool
		wantDecision string
	}{
		{
			name:  "pre_tool_use_approve",
			input: `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write"}`,
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					preResponse: &HookResponse{
						Decision: "approve",
						Message:  "All good",
					},
				}
			},
			wantResponse: true,
			wantDecision: "approve",
		},
		{
			name:  "post_tool_use",
			input: `{"hook_event_name":"PostToolUse","session_id":"test","tool_name":"Write","tool_output":"success"}`,
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					postResponse: &HookResponse{
						Decision: "logged",
					},
				}
			},
			wantResponse: true,
			wantDecision: "logged",
		},
		{
			name:  "notification",
			input: `{"hook_event_name":"Notification","session_id":"test","notification_type":"info","message":"Test"}`,
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					notifResponse: &HookResponse{
						Decision: "acknowledged",
					},
				}
			},
			wantResponse: true,
			wantDecision: "acknowledged",
		},
		{
			name:  "stop_event",
			input: `{"hook_event_name":"Stop","session_id":"test","reason":"User requested"}`,
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					stopResponse: &HookResponse{
						Decision: "handled",
					},
				}
			},
			wantResponse: true,
			wantDecision: "handled",
		},
		{
			name:  "subagent_stop",
			input: `{"hook_event_name":"SubagentStop","session_id":"test","subagent_id":"sub-123"}`,
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					subagentResponse: &HookResponse{
						Decision: "processed",
					},
				}
			},
			wantResponse: true,
			wantDecision: "processed",
		},
		{
			name:  "pre_compact",
			input: `{"hook_event_name":"PreCompact","session_id":"test","size_before":1000}`,
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					compactResponse: &HookResponse{
						Decision: "proceed",
					},
				}
			},
			wantResponse: true,
			wantDecision: "proceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore stdin
			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()

			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdin = r

			// Write test data
			go func() {
				_, _ = w.Write([]byte(tt.input))
				w.Close()
			}()

			handler := NewHandler(tt.setupEngine())
			resp, err := handler.ProcessInputWithResponse(context.Background())

			if err != nil {
				t.Fatalf("ProcessInputWithResponse() error = %v", err)
			}

			if tt.wantResponse && resp == nil {
				t.Error("Expected response, got nil")
			}

			if tt.wantResponse && resp != nil && resp.Decision != tt.wantDecision {
				t.Errorf("Expected decision %q, got %q", tt.wantDecision, resp.Decision)
			}
		})
	}
}

func TestHandler_handleNotification(t *testing.T) {
	handler := NewHandler(&customRuleEngine{
		notifResponse: &HookResponse{
			Decision: "acknowledged",
			Message:  "Notification received",
		},
	})

	msg := &NotificationMessage{
		BaseHookMessage: BaseHookMessage{
			SessionID:     "test",
			HookEventName: NotificationEvent,
		},
		NotificationType: "info",
		Message:          "Test notification",
	}

	resp, err := handler.handleNotification(context.Background(), msg)
	if err != nil {
		t.Fatalf("handleNotification() error = %v", err)
	}

	if resp == nil || resp.Decision != "acknowledged" {
		t.Errorf("Expected acknowledged response, got %v", resp)
	}
}

func TestHandler_handleStop(t *testing.T) {
	handler := NewHandler(&customRuleEngine{
		stopResponse: &HookResponse{
			Decision: "handled",
			Message:  "Stop processed",
		},
	})

	msg := &StopMessage{
		BaseHookMessage: BaseHookMessage{
			SessionID:     "test",
			HookEventName: StopEvent,
		},
		Reason: "User requested stop",
	}

	resp, err := handler.handleStop(context.Background(), msg)
	if err != nil {
		t.Fatalf("handleStop() error = %v", err)
	}

	if resp == nil || resp.Decision != "handled" {
		t.Errorf("Expected handled response, got %v", resp)
	}
}

func TestHandler_handleSubagentStop(t *testing.T) {
	handler := NewHandler(&customRuleEngine{
		subagentResponse: &HookResponse{
			Decision: "processed",
			Message:  "Subagent stop processed",
		},
	})

	msg := &SubagentStopMessage{
		BaseHookMessage: BaseHookMessage{
			SessionID:     "test",
			HookEventName: SubagentStopEvent,
		},
		SubagentID: "subagent-123",
		Result:     "Task completed",
	}

	resp, err := handler.handleSubagentStop(context.Background(), msg)
	if err != nil {
		t.Fatalf("handleSubagentStop() error = %v", err)
	}

	if resp == nil || resp.Decision != "processed" {
		t.Errorf("Expected processed response, got %v", resp)
	}
}

func TestHandler_handlePreCompact(t *testing.T) {
	handler := NewHandler(&customRuleEngine{
		compactResponse: &HookResponse{
			Decision: "proceed",
			Message:  "Compaction approved",
		},
	})

	msg := &PreCompactMessage{
		BaseHookMessage: BaseHookMessage{
			SessionID:     "test",
			HookEventName: PreCompactEvent,
		},
		CurrentTokens: 1000000,
		TargetTokens:  500000,
	}

	resp, err := handler.handlePreCompact(context.Background(), msg)
	if err != nil {
		t.Fatalf("handlePreCompact() error = %v", err)
	}

	if resp == nil || resp.Decision != "proceed" {
		t.Errorf("Expected proceed response, got %v", resp)
	}
}

func TestHandler_ProcessMessage_Errors(t *testing.T) {
	handler := NewHandler(&errorRuleEngine{})
	ctx := context.Background()

	tests := []struct {
		name    string
		msg     HookMessage
		wantErr string
	}{
		{
			name:    "pre_tool_use_error",
			msg:     &PreToolUseMessage{},
			wantErr: "rule evaluation failed",
		},
		{
			name:    "post_tool_use_error",
			msg:     &PostToolUseMessage{},
			wantErr: "rule evaluation failed",
		},
		{
			name:    "notification_error",
			msg:     &NotificationMessage{},
			wantErr: "notification error",
		},
		{
			name:    "stop_error",
			msg:     &StopMessage{},
			wantErr: "stop error",
		},
		{
			name:    "subagent_stop_error",
			msg:     &SubagentStopMessage{},
			wantErr: "subagent stop error",
		},
		{
			name:    "pre_compact_error",
			msg:     &PreCompactMessage{},
			wantErr: "pre compact error",
		},
		{
			name:    "unknown_message_type",
			msg:     &mockUnknownMessage{},
			wantErr: "unknown message type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.ProcessMessage(ctx, tt.msg)
			if err == nil {
				t.Error("Expected error, got none")
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestHandler_NoRuleEngine(t *testing.T) {
	handler := &Handler{
		parser:   NewParser(),
		registry: NewRegistry(),
		// ruleEngine is nil
	}

	ctx := context.Background()
	msg := &PreToolUseMessage{}

	_, err := handler.ProcessMessage(ctx, msg)
	if err == nil {
		t.Error("Expected error for nil rule engine")
	}

	if !strings.Contains(err.Error(), "no rule engine configured") {
		t.Errorf("Expected 'no rule engine' error, got %v", err)
	}
}

func TestHandler_ProcessInput_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "invalid_json",
			input:   `{"invalid": json}`,
			wantErr: "failed to parse hook message",
		},
		{
			name:    "unknown_event",
			input:   `{"hook_event_name":"UnknownEvent","session_id":"test"}`,
			wantErr: "failed to parse hook message",
		},
		{
			name:    "empty_input",
			input:   "",
			wantErr: "failed to parse hook message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore stdin
			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()

			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdin = r

			// Write test data
			go func() {
				_, _ = w.Write([]byte(tt.input))
				w.Close()
			}()

			handler := NewHandler(NewBaseRuleEngine())
			err = handler.ProcessInput(context.Background())

			if err == nil {
				t.Error("Expected error, got none")
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestRegistry_Operations(t *testing.T) {
	registry := NewRegistry()

	// Register some hooks
	hook1 := HookConfig{
		Name:      "hook1",
		EventType: PreToolUseEvent,
		Priority:  1,
	}

	hook2 := HookConfig{
		Name:      "hook2",
		EventType: PreToolUseEvent,
		Priority:  2,
	}

	hook3 := HookConfig{
		Name:      "hook3",
		EventType: PostToolUseEvent,
		Priority:  1,
	}

	registry.Register(hook1)
	registry.Register(hook2)
	registry.Register(hook3)

	// Get hooks for PreToolUse
	preHooks := registry.GetHooks(PreToolUseEvent)
	if len(preHooks) != 2 {
		t.Errorf("Expected 2 PreToolUse hooks, got %d", len(preHooks))
	}

	// Get hooks for PostToolUse
	postHooks := registry.GetHooks(PostToolUseEvent)
	if len(postHooks) != 1 {
		t.Errorf("Expected 1 PostToolUse hook, got %d", len(postHooks))
	}

	// Get hooks for non-existent event
	notifHooks := registry.GetHooks(NotificationEvent)
	if len(notifHooks) != 0 {
		t.Errorf("Expected 0 Notification hooks, got %d", len(notifHooks))
	}

	// Clear registry
	registry.Clear()

	// Should be empty now
	preHooks = registry.GetHooks(PreToolUseEvent)
	if len(preHooks) != 0 {
		t.Errorf("Expected 0 hooks after clear, got %d", len(preHooks))
	}
}

func TestHandler_ConcurrentAccess(t *testing.T) {
	handler := NewHandler(NewLintingRuleEngine())

	// Test concurrent SetRuleEngine and ProcessMessage
	done := make(chan bool, 10)

	// Multiple goroutines setting rule engine
	for i := 0; i < 5; i++ {
		go func() {
			engine := NewBaseRuleEngine()
			handler.SetRuleEngine(engine)
			done <- true
		}()
	}

	// Multiple goroutines processing messages
	for i := 0; i < 5; i++ {
		go func() {
			msg := &PreToolUseMessage{
				BaseHookMessage: BaseHookMessage{
					HookEventName: PreToolUseEvent,
				},
				ToolName: "Write",
			}
			_, _ = handler.ProcessMessage(context.Background(), msg)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
