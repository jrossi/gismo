package engine

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseHookMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, result interface{})
	}{
		{
			name: "PreToolUse message",
			input: `{
				"session_id": "test-123",
				"transcript_path": "/path/to/transcript.json",
				"hook_event_name": "PreToolUse",
				"tool_name": "Write",
				"tool_input": {
					"file_path": "/test.txt",
					"content": "test content"
				}
			}`,
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				msg, ok := result.(*PreToolUseMessage)
				if !ok {
					t.Fatalf("expected *PreToolUseMessage, got %T", result)
				}
				if msg.SessionID != "test-123" {
					t.Errorf("SessionID = %v, want %v", msg.SessionID, "test-123")
				}
				if msg.ToolName != "Write" {
					t.Errorf("ToolName = %v, want %v", msg.ToolName, "Write")
				}
			},
		},
		{
			name: "PostToolUse message",
			input: `{
				"session_id": "test-456",
				"transcript_path": "/path/to/transcript.json",
				"hook_event_name": "PostToolUse",
				"tool_name": "Read",
				"tool_output": "file contents",
				"tool_error": ""
			}`,
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				msg, ok := result.(*PostToolUseMessage)
				if !ok {
					t.Fatalf("expected *PostToolUseMessage, got %T", result)
				}
				if msg.ToolName != "Read" {
					t.Errorf("ToolName = %v, want %v", msg.ToolName, "Read")
				}
				var output string
				if err := json.Unmarshal(msg.ToolOutput, &output); err != nil {
					t.Errorf("Failed to unmarshal ToolOutput: %v", err)
				} else if output != "file contents" {
					t.Errorf("ToolOutput = %v, want %v", output, "file contents")
				}
			},
		},
		{
			name: "Unknown event type",
			input: `{
				"session_id": "test-789",
				"transcript_path": "/path/to/transcript.json",
				"hook_event_name": "UnknownEvent"
			}`,
			wantErr: true,
		},
		{
			name:    "Invalid JSON",
			input:   `{invalid json`,
			wantErr: true,
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseHookMessage([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHookMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestParseHookResponse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *HookResponse
		wantErr bool
	}{
		{
			name: "Full response",
			input: `{
				"continue": false,
				"stopReason": "Security violation",
				"suppressOutput": true,
				"decision": "block",
				"reason": "File access denied"
			}`,
			want: &HookResponse{
				Continue:       boolPtr(false),
				StopReason:     "Security violation",
				SuppressOutput: boolPtr(true),
				Decision:       "block",
				Reason:         "File access denied",
			},
			wantErr: false,
		},
		{
			name: "Minimal response",
			input: `{
				"decision": "approve"
			}`,
			want: &HookResponse{
				Decision: "approve",
			},
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseHookResponse([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHookResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Decision != tt.want.Decision {
					t.Errorf("Decision = %v, want %v", got.Decision, tt.want.Decision)
				}
				if got.StopReason != tt.want.StopReason {
					t.Errorf("StopReason = %v, want %v", got.StopReason, tt.want.StopReason)
				}
			}
		})
	}
}

func TestStreamParser(t *testing.T) {
	input := `{"session_id":"1","transcript_path":"/p1","hook_event_name":"PreToolUse","tool_name":"Write","tool_input":{}}
{"session_id":"2","transcript_path":"/p2","hook_event_name":"PostToolUse","tool_name":"Read","tool_output":"data"}`

	reader := strings.NewReader(input)
	parser := NewStreamParser(reader)

	// First message
	msg1, err := parser.ParseNext()
	if err != nil {
		t.Fatalf("ParseNext() error = %v", err)
	}
	preMsg, ok := msg1.(*PreToolUseMessage)
	if !ok {
		t.Fatalf("expected *PreToolUseMessage, got %T", msg1)
	}
	if preMsg.SessionID != "1" {
		t.Errorf("SessionID = %v, want %v", preMsg.SessionID, "1")
	}

	// Second message
	msg2, err := parser.ParseNext()
	if err != nil {
		t.Fatalf("ParseNext() error = %v", err)
	}
	postMsg, ok := msg2.(*PostToolUseMessage)
	if !ok {
		t.Fatalf("expected *PostToolUseMessage, got %T", msg2)
	}
	if postMsg.SessionID != "2" {
		t.Errorf("SessionID = %v, want %v", postMsg.SessionID, "2")
	}
}

func TestParseMultiple(t *testing.T) {
	input := `{"session_id":"1","transcript_path":"/p1","hook_event_name":"PreToolUse","tool_name":"Write","tool_input":{}}
{"session_id":"2","transcript_path":"/p2","hook_event_name":"PostToolUse","tool_name":"Read","tool_output":"data"}`

	parser := NewParser()
	messages, err := parser.ParseMultiple([]byte(input))
	if err != nil {
		t.Fatalf("ParseMultiple() error = %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// Check first message
	if _, ok := messages[0].(*PreToolUseMessage); !ok {
		t.Errorf("expected first message to be *PreToolUseMessage, got %T", messages[0])
	}

	// Check second message
	if _, ok := messages[1].(*PostToolUseMessage); !ok {
		t.Errorf("expected second message to be *PostToolUseMessage, got %T", messages[1])
	}
}

func TestMarshalHookResponse(t *testing.T) {
	parser := NewParser()
	response := &HookResponse{
		Continue:   boolPtr(false),
		StopReason: "Test reason",
		Decision:   "block",
	}

	data, err := parser.MarshalHookResponse(response)
	if err != nil {
		t.Fatalf("MarshalHookResponse() error = %v", err)
	}

	// Verify it can be parsed back
	parsed, err := parser.ParseHookResponse(data)
	if err != nil {
		t.Fatalf("ParseHookResponse() error = %v", err)
	}

	if parsed.Decision != response.Decision {
		t.Errorf("Decision = %v, want %v", parsed.Decision, response.Decision)
	}
}

// Benchmark tests
func BenchmarkParsePreToolUse(b *testing.B) {
	input := []byte(`{
		"session_id": "bench-123",
		"transcript_path": "/path/to/transcript.json",
		"hook_event_name": "PreToolUse",
		"tool_name": "Write",
		"tool_input": {
			"file_path": "/test.txt",
			"content": "benchmark content"
		}
	}`)

	parser := NewParser()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ParseHookMessage(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStreamParser(b *testing.B) {
	input := `{"session_id":"1","transcript_path":"/p","hook_event_name":"PreToolUse","tool_name":"Write","tool_input":{}}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader([]byte(input))
		parser := NewStreamParser(reader)
		_, err := parser.ParseNext()
		if err != nil {
			b.Fatal(err)
		}
	}
}
