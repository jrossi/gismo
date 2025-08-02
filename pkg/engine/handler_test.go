package engine

import (
	"context"
	"testing"
	"time"
)

// mockInvalidMessage is a test type that implements HookMessage for testing unknown message types
type mockInvalidMessage struct {
	BaseHookMessage
}

func (m mockInvalidMessage) GetBaseMessage() BaseHookMessage { return m.BaseHookMessage }
func (m mockInvalidMessage) EventName() HookEventName        { return "InvalidEvent" }

// MockRuleEngine implements RuleEngine for testing
type MockRuleEngine struct {
	preToolUseResponse   *HookResponse
	postToolUseResponse  *HookResponse
	notificationResponse *HookResponse
	stopResponse         *HookResponse
	subagentStopResponse *HookResponse
	preCompactResponse   *HookResponse

	preToolUseCalled   bool
	postToolUseCalled  bool
	notificationCalled bool
	stopCalled         bool
	subagentStopCalled bool
	preCompactCalled   bool
}

func (m *MockRuleEngine) EvaluatePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error) {
	m.preToolUseCalled = true
	return m.preToolUseResponse, nil
}

func (m *MockRuleEngine) EvaluatePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error) {
	m.postToolUseCalled = true
	return m.postToolUseResponse, nil
}

func (m *MockRuleEngine) EvaluateNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error) {
	m.notificationCalled = true
	return m.notificationResponse, nil
}

func (m *MockRuleEngine) EvaluateStop(ctx context.Context, msg *StopMessage) (*HookResponse, error) {
	m.stopCalled = true
	return m.stopResponse, nil
}

func (m *MockRuleEngine) EvaluateSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error) {
	m.subagentStopCalled = true
	return m.subagentStopResponse, nil
}

func (m *MockRuleEngine) EvaluatePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error) {
	m.preCompactCalled = true
	return m.preCompactResponse, nil
}

func (m *MockRuleEngine) EvaluateUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error) {
	return nil, nil
}

func (m *MockRuleEngine) EvaluateSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error) {
	return nil, nil
}

func TestHandler_ProcessMessage(t *testing.T) {
	tests := []struct {
		name        string
		message     HookMessage
		setupMock   func(*MockRuleEngine)
		checkCalled func(*testing.T, *MockRuleEngine)
		wantErr     bool
	}{
		{
			name: "PreToolUse message",
			message: &PreToolUseMessage{
				BaseHookMessage: BaseHookMessage{
					SessionID:      "test-123",
					TranscriptPath: "/path/to/transcript",
					HookEventName:  PreToolUseEvent,
				},
				ToolName: "Write",
			},
			setupMock: func(m *MockRuleEngine) {
				m.preToolUseResponse = &HookResponse{
					Decision: "approve",
				}
			},
			checkCalled: func(t *testing.T, m *MockRuleEngine) {
				if !m.preToolUseCalled {
					t.Error("EvaluatePreToolUse was not called")
				}
			},
			wantErr: false,
		},
		{
			name: "PostToolUse message",
			message: &PostToolUseMessage{
				BaseHookMessage: BaseHookMessage{
					SessionID:      "test-456",
					TranscriptPath: "/path/to/transcript",
					HookEventName:  PostToolUseEvent,
				},
				ToolName: "Read",
			},
			setupMock: func(m *MockRuleEngine) {
				// No response needed
			},
			checkCalled: func(t *testing.T, m *MockRuleEngine) {
				if !m.postToolUseCalled {
					t.Error("EvaluatePostToolUse was not called")
				}
			},
			wantErr: false,
		},
		{
			name:    "Unknown message type",
			message: &mockInvalidMessage{},
			setupMock: func(m *MockRuleEngine) {
				// No setup needed
			},
			checkCalled: func(t *testing.T, m *MockRuleEngine) {
				// Nothing should be called
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEngine := &MockRuleEngine{}
			tt.setupMock(mockEngine)

			handler := NewHandler(mockEngine)
			ctx := context.Background()

			_, err := handler.ProcessMessage(ctx, tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessMessage() error = %v, wantErr %v", err, tt.wantErr)
			}

			tt.checkCalled(t, mockEngine)
		})
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test registering hooks
	config1 := HookConfig{
		Name:        "test-hook-1",
		EventType:   PreToolUseEvent,
		ToolPattern: "Write.*",
		Priority:    1,
		Timeout:     30 * time.Second,
	}

	config2 := HookConfig{
		Name:        "test-hook-2",
		EventType:   PreToolUseEvent,
		ToolPattern: "Read.*",
		Priority:    2,
		Timeout:     30 * time.Second,
	}

	config3 := HookConfig{
		Name:      "test-hook-3",
		EventType: PostToolUseEvent,
		Priority:  1,
		Timeout:   30 * time.Second,
	}

	registry.Register(config1)
	registry.Register(config2)
	registry.Register(config3)

	// Test getting hooks
	preToolHooks := registry.GetHooks(PreToolUseEvent)
	if len(preToolHooks) != 2 {
		t.Errorf("expected 2 PreToolUse hooks, got %d", len(preToolHooks))
	}

	postToolHooks := registry.GetHooks(PostToolUseEvent)
	if len(postToolHooks) != 1 {
		t.Errorf("expected 1 PostToolUse hook, got %d", len(postToolHooks))
	}

	// Test clear
	registry.Clear()
	hooks := registry.GetHooks(PreToolUseEvent)
	if len(hooks) != 0 {
		t.Errorf("expected 0 hooks after clear, got %d", len(hooks))
	}
}

func TestHandler_SetRuleEngine(t *testing.T) {
	// Create handler with initial engine
	engine1 := &MockRuleEngine{
		preToolUseResponse: &HookResponse{Decision: "approve"},
	}
	handler := NewHandler(engine1)

	// Process a message with first engine
	ctx := context.Background()
	msg := &PreToolUseMessage{
		BaseHookMessage: BaseHookMessage{
			HookEventName: PreToolUseEvent,
		},
	}

	resp1, err := handler.ProcessMessage(ctx, msg)
	if err != nil {
		t.Fatalf("ProcessMessage() error = %v", err)
	}
	if resp1.Decision != "approve" {
		t.Errorf("expected approve decision, got %v", resp1.Decision)
	}

	// Change engine
	engine2 := &MockRuleEngine{
		preToolUseResponse: &HookResponse{Decision: "block"},
	}
	handler.SetRuleEngine(engine2)

	// Process again with new engine
	resp2, err := handler.ProcessMessage(ctx, msg)
	if err != nil {
		t.Fatalf("ProcessMessage() error = %v", err)
	}
	if resp2.Decision != "block" {
		t.Errorf("expected block decision, got %v", resp2.Decision)
	}
}
