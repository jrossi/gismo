package gismo

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

func TestParser_ParseHookMessage_AllMessageTypes(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		msgType string
	}{
		{
			name: "notification_message",
			input: `{
				"hook_event_name": "Notification",
				"session_id": "test-session",
				"notification_type": "info",
				"message": "Test notification"
			}`,
			msgType: "Notification",
		},
		{
			name: "stop_message",
			input: `{
				"hook_event_name": "Stop",
				"session_id": "test-session",
				"reason": "User requested stop"
			}`,
			msgType: "Stop",
		},
		{
			name: "subagent_stop_message",
			input: `{
				"hook_event_name": "SubagentStop",
				"session_id": "test-session",
				"subagent_id": "subagent-123",
				"reason": "Task completed"
			}`,
			msgType: "SubagentStop",
		},
		{
			name: "pre_compact_message",
			input: `{
				"hook_event_name": "PreCompact",
				"session_id": "test-session",
				"size_before": 1000000,
				"target_size": 500000
			}`,
			msgType: "PreCompact",
		},
		{
			name:    "malformed_json",
			input:   `{"hook_event_name": "PreToolUse", invalid json`,
			wantErr: true,
		},
		{
			name:    "empty_hook_event",
			input:   `{"session_id": "test"}`,
			wantErr: true,
		},
		{
			name:    "unknown_hook_event",
			input:   `{"hook_event_name": "UnknownEvent", "session_id": "test"}`,
			wantErr: true,
		},
		{
			name: "nested_tool_input",
			input: `{
				"hook_event_name": "PreToolUse",
				"session_id": "test",
				"tool_name": "Write",
				"tool_input": {
					"file_path": "/test.go",
					"content": "package main\n",
					"nested": {
						"deeply": {
							"nested": "value"
						}
					}
				}
			}`,
			msgType: "PreToolUse",
		},
		{
			name: "tool_input_with_arrays",
			input: `{
				"hook_event_name": "PreToolUse",
				"session_id": "test",
				"tool_name": "MultiEdit",
				"tool_input": {
					"file_path": "/test.go",
					"edits": [
						{"old": "foo", "new": "bar"},
						{"old": "baz", "new": "qux"}
					]
				}
			}`,
			msgType: "PreToolUse",
		},
		{
			name: "post_tool_with_error",
			input: `{
				"hook_event_name": "PostToolUse",
				"session_id": "test",
				"tool_name": "Write",
				"tool_output": "Error: permission denied",
				"tool_error": "true"
			}`,
			msgType: "PostToolUse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := parser.ParseHookMessage([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseHookMessage() error = %v", err)
			}

			// Verify message type
			switch msg.(type) {
			case *PreToolUseMessage:
				if tt.msgType != "PreToolUse" {
					t.Errorf("Expected PreToolUse, got different type")
				}
			case *PostToolUseMessage:
				if tt.msgType != "PostToolUse" {
					t.Errorf("Expected PostToolUse, got different type")
				}
			case *NotificationMessage:
				if tt.msgType != "Notification" {
					t.Errorf("Expected Notification, got different type")
				}
			case *StopMessage:
				if tt.msgType != "Stop" {
					t.Errorf("Expected Stop, got different type")
				}
			case *SubagentStopMessage:
				if tt.msgType != "SubagentStop" {
					t.Errorf("Expected SubagentStop, got different type")
				}
			case *PreCompactMessage:
				if tt.msgType != "PreCompact" {
					t.Errorf("Expected PreCompact, got different type")
				}
			default:
				t.Errorf("Unknown message type returned")
			}
		})
	}
}

func TestParser_MarshalHookMessage(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name string
		msg  HookMessage
	}{
		{
			name: "pre_tool_use_message",
			msg: &PreToolUseMessage{
				BaseHookMessage: BaseHookMessage{
					SessionID:     "test-session",
					HookEventName: PreToolUseEvent,
				},
				ToolName: "Write",
				ToolInput: testConvertToRawMessage(map[string]interface{}{
					"file_path": "test.go",
					"content":   "package main\n",
				}),
			},
		},
		{
			name: "notification_message",
			msg: &NotificationMessage{
				BaseHookMessage: BaseHookMessage{
					SessionID:     "test-session",
					HookEventName: NotificationEvent,
				},
				NotificationType: "info",
				Message:          "Test notification",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := parser.MarshalHookMessage(tt.msg)
			if err != nil {
				t.Fatalf("MarshalHookMessage() error = %v", err)
			}

			// Verify it's valid JSON
			var decoded map[string]interface{}
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Errorf("Result is not valid JSON: %v", err)
			}

			// Should end with newline
			if !bytes.HasSuffix(data, []byte("\n")) {
				t.Error("Expected JSON to end with newline")
			}
		})
	}
}

func TestStreamParser_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		count   int
	}{
		{
			name:  "multiple_messages_no_separator",
			input: `{"hook_event_name":"PreToolUse","session_id":"1","tool_name":"Write"}{"hook_event_name":"PostToolUse","session_id":"2","tool_name":"Write"}`,
			count: 2,
		},
		{
			name: "messages_with_whitespace",
			input: `  {"hook_event_name":"PreToolUse","session_id":"1","tool_name":"Write"}  

  {"hook_event_name":"PostToolUse","session_id":"2","tool_name":"Write"}  `,
			count: 2,
		},
		{
			name:    "incomplete_json",
			input:   `{"hook_event_name":"PreToolUse","session_id":"1"`,
			wantErr: true,
		},
		{
			name:  "empty_lines_between",
			input: "{\n\"hook_event_name\":\"PreToolUse\",\n\"session_id\":\"1\",\n\"tool_name\":\"Write\"\n}\n\n\n{\n\"hook_event_name\":\"PostToolUse\",\n\"session_id\":\"2\",\n\"tool_name\":\"Write\"\n}",
			count: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			messages, err := parser.ParseMultiple([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseMultiple() error = %v", err)
			}

			if len(messages) != tt.count {
				t.Errorf("Expected %d messages, got %d", tt.count, len(messages))
			}
		})
	}
}

func TestStreamParser_ParseNext_Scenarios(t *testing.T) {
	t.Run("EOF_after_valid_message", func(t *testing.T) {
		input := `{"hook_event_name":"PreToolUse","session_id":"1","tool_name":"Write"}`
		parser := NewStreamParser(strings.NewReader(input))

		// First message should parse
		msg, err := parser.ParseNext()
		if err != nil {
			t.Fatalf("First ParseNext() error = %v", err)
		}
		if msg == nil {
			t.Error("Expected message, got nil")
		}

		// Second call should return EOF
		msg2, err2 := parser.ParseNext()
		if err2 == nil {
			t.Error("Expected EOF error")
		}
		if msg2 != nil {
			t.Error("Expected nil message on EOF")
		}
	})

	t.Run("multiple_sequential_reads", func(t *testing.T) {
		messages := []string{
			`{"hook_event_name":"PreToolUse","session_id":"1","tool_name":"Write"}`,
			`{"hook_event_name":"PostToolUse","session_id":"1","tool_name":"Write","tool_output":"success"}`,
			`{"hook_event_name":"Notification","session_id":"1","notification_type":"info","message":"Done"}`,
		}
		input := strings.Join(messages, "\n")
		parser := NewStreamParser(strings.NewReader(input))

		for i := range messages {
			msg, err := parser.ParseNext()
			if err != nil {
				t.Fatalf("ParseNext() #%d error = %v", i+1, err)
			}
			if msg == nil {
				t.Errorf("Expected message #%d, got nil", i+1)
			}
		}
	})
}

// Mock reader that returns an error
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

func TestStreamParser_ReadError(t *testing.T) {
	customErr := errors.New("custom read error")
	parser := NewStreamParser(&errorReader{err: customErr})

	_, err := parser.ParseNext()
	if err == nil {
		t.Error("Expected error from failed read")
	}
}

func TestParser_LargeMessages(t *testing.T) {
	parser := NewParser()

	// Create a large tool input
	largeContent := strings.Repeat("x", 1000000) // 1MB of content
	msg := map[string]interface{}{
		"hook_event_name": "PreToolUse",
		"session_id":      "test",
		"tool_name":       "Write",
		"tool_input": map[string]interface{}{
			"file_path": "large.txt",
			"content":   largeContent,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal large message: %v", err)
	}

	// Should handle large messages
	parsed, err := parser.ParseHookMessage(data)
	if err != nil {
		t.Fatalf("ParseHookMessage() with large content error = %v", err)
	}

	if parsed == nil {
		t.Error("Expected parsed message, got nil")
	}

	// Verify content was preserved
	if preMsg, ok := parsed.(*PreToolUseMessage); ok {
		if contentRaw, exists := preMsg.ToolInput["content"]; exists {
			var content string
			if err := json.Unmarshal(contentRaw, &content); err == nil {
				if len(content) != len(largeContent) {
					t.Errorf("Content length mismatch: got %d, want %d", len(content), len(largeContent))
				}
			}
		}
	}
}
