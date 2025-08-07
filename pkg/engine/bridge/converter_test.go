package bridge

import (
	"testing"

	json "github.com/goccy/go-json"
	"google.golang.org/protobuf/types/known/timestamppb"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

func TestJSONToProto_PreToolUse(t *testing.T) {
	converter := NewConverter()

	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name: "bash command",
			json: `{
				"hook_event_name": "PreToolUse",
				"session_id": "test-session-123",
				"transcript_path": "/tmp/transcript.json",
				"tool_name": "Bash",
				"tool_input": {
					"command": "ls -la",
					"description": "List files",
					"timeout": 5000
				}
			}`,
			wantErr: false,
		},
		{
			name: "edit operation",
			json: `{
				"hook_event_name": "PreToolUse",
				"session_id": "test-session-456",
				"tool_name": "Edit",
				"tool_input": {
					"file_path": "test.txt",
					"old_string": "foo",
					"new_string": "bar",
					"replace_all": true
				}
			}`,
			wantErr: false,
		},
		{
			name: "write operation",
			json: `{
				"hook_event_name": "PreToolUse",
				"session_id": "test-session-789",
				"tool_name": "Write",
				"tool_input": {
					"file_path": "output.txt",
					"content": "Hello, World!"
				}
			}`,
			wantErr: false,
		},
		{
			name: "unknown tool with generic params",
			json: `{
				"hook_event_name": "PreToolUse",
				"session_id": "test-session-unknown",
				"tool_name": "CustomTool",
				"tool_input": {
					"param1": "value1",
					"param2": 42,
					"param3": true
				}
			}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := converter.JSONToProto([]byte(tt.json))
			if (err != nil) != tt.wantErr {
				t.Errorf("JSONToProto() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify base message
				if req.Base == nil {
					t.Fatal("Expected base message to be set")
				}
				if req.Base.EventType != gismov1.HookEventType_HOOK_EVENT_TYPE_PRE_TOOL_USE {
					t.Errorf("Expected PRE_TOOL_USE event type, got %v", req.Base.EventType)
				}

				// Verify payload
				payload := req.GetPreToolUse()
				if payload == nil {
					t.Fatal("Expected PreToolUse payload")
				}

				// Check tool-specific parameters
				switch payload.ToolName {
				case "Bash":
					bash := payload.GetBash()
					if bash == nil {
						t.Fatal("Expected Bash parameters")
					}
					if bash.Command != "ls -la" {
						t.Errorf("Expected command 'ls -la', got %s", bash.Command)
					}
					if bash.Timeout == nil || *bash.Timeout != 5000 {
						t.Error("Expected timeout to be 5000")
					}

				case "Edit":
					edit := payload.GetEdit()
					if edit == nil {
						t.Fatal("Expected Edit parameters")
					}
					if edit.FilePath != "test.txt" {
						t.Errorf("Expected file_path 'test.txt', got %s", edit.FilePath)
					}
					if edit.ReplaceAll == nil || !*edit.ReplaceAll {
						t.Error("Expected replace_all to be true")
					}

				case "Write":
					write := payload.GetWrite()
					if write == nil {
						t.Fatal("Expected Write parameters")
					}
					if write.Content != "Hello, World!" {
						t.Errorf("Expected content 'Hello, World!', got %s", write.Content)
					}

				case "CustomTool":
					generic := payload.GetGeneric()
					if generic == nil {
						t.Fatal("Expected generic parameters for unknown tool")
					}
					if len(generic.Fields) != 3 {
						t.Errorf("Expected 3 fields, got %d", len(generic.Fields))
					}
				}
			}
		})
	}
}

func TestJSONToProto_PostToolUse(t *testing.T) {
	converter := NewConverter()

	jsonData := `{
		"hook_event_name": "PostToolUse",
		"session_id": "test-session-post",
		"tool_name": "Bash",
		"tool_input": {
			"command": "echo test"
		},
		"tool_output": "test\n",
		"tool_error": ""
	}`

	req, err := converter.JSONToProto([]byte(jsonData))
	if err != nil {
		t.Fatalf("JSONToProto() error = %v", err)
	}

	payload := req.GetPostToolUse()
	if payload == nil {
		t.Fatal("Expected PostToolUse payload")
	}

	if payload.ToolName != "Bash" {
		t.Errorf("Expected tool_name 'Bash', got %s", payload.ToolName)
	}

	if payload.ToolOutput == nil {
		t.Error("Expected tool_output to be set")
	}
}

func TestJSONToProto_UserPromptSubmit(t *testing.T) {
	converter := NewConverter()

	jsonData := `{
		"hook_event_name": "UserPromptSubmit",
		"session_id": "test-session-prompt",
		"user_prompt": "Help me write a function",
		"timestamp": 1704067200
	}`

	req, err := converter.JSONToProto([]byte(jsonData))
	if err != nil {
		t.Fatalf("JSONToProto() error = %v", err)
	}

	payload := req.GetUserPromptSubmit()
	if payload == nil {
		t.Fatal("Expected UserPromptSubmit payload")
	}

	if payload.UserPrompt != "Help me write a function" {
		t.Errorf("Expected prompt 'Help me write a function', got %s", payload.UserPrompt)
	}

	if payload.Timestamp == nil {
		t.Error("Expected timestamp to be set")
	}
}

func TestProtoToJSON_Response(t *testing.T) {
	converter := NewConverter()

	tests := []struct {
		name     string
		response *gismov1.HookResponse
		wantJSON map[string]interface{}
	}{
		{
			name: "approve decision",
			response: &gismov1.HookResponse{
				Decision: gismov1.HookResponse_DECISION_APPROVE,
				Reason:   "Command is safe",
				Message:  "Executing command",
			},
			wantJSON: map[string]interface{}{
				"decision": "approve",
				"reason":   "Command is safe",
				"message":  "Executing command",
			},
		},
		{
			name: "block decision",
			response: &gismov1.HookResponse{
				Decision:   gismov1.HookResponse_DECISION_BLOCK,
				Reason:     "Dangerous command detected",
				Message:    "Command blocked for security",
				ExitCode:   2,
				Continue:   boolPtr(false),
				StopReason: "Security violation",
			},
			wantJSON: map[string]interface{}{
				"decision":   "block",
				"reason":     "Dangerous command detected",
				"message":    "Command blocked for security",
				"continue":   false,
				"stopReason": "Security violation",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := converter.ProtoToJSON(tt.response)
			if err != nil {
				t.Fatalf("ProtoToJSON() error = %v", err)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(jsonData, &result); err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			// Check expected fields
			for key, expectedValue := range tt.wantJSON {
				if actualValue, ok := result[key]; !ok {
					t.Errorf("Missing field %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("Field %s: expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestJSONToProto_InvalidJSON(t *testing.T) {
	converter := NewConverter()

	tests := []struct {
		name string
		json string
	}{
		{
			name: "malformed JSON",
			json: `{"invalid": json}`,
		},
		{
			name: "missing event name",
			json: `{"session_id": "test"}`,
		},
		{
			name: "unknown event type",
			json: `{"hook_event_name": "UnknownEvent"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := converter.JSONToProto([]byte(tt.json))
			if err == nil {
				t.Error("Expected error for invalid JSON")
			}
		})
	}
}

func TestEventTypeMapping(t *testing.T) {
	converter := NewConverter()

	tests := []struct {
		eventName string
		expected  gismov1.HookEventType
	}{
		{"PreToolUse", gismov1.HookEventType_HOOK_EVENT_TYPE_PRE_TOOL_USE},
		{"PostToolUse", gismov1.HookEventType_HOOK_EVENT_TYPE_POST_TOOL_USE},
		{"Notification", gismov1.HookEventType_HOOK_EVENT_TYPE_NOTIFICATION},
		{"Stop", gismov1.HookEventType_HOOK_EVENT_TYPE_STOP},
		{"SubagentStop", gismov1.HookEventType_HOOK_EVENT_TYPE_SUBAGENT_STOP},
		{"PreCompact", gismov1.HookEventType_HOOK_EVENT_TYPE_PRE_COMPACT},
		{"UserPromptSubmit", gismov1.HookEventType_HOOK_EVENT_TYPE_USER_PROMPT_SUBMIT},
		{"SessionStart", gismov1.HookEventType_HOOK_EVENT_TYPE_SESSION_START},
		{"Unknown", gismov1.HookEventType_HOOK_EVENT_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.eventName, func(t *testing.T) {
			result := converter.mapEventType(tt.eventName)
			if result != tt.expected {
				t.Errorf("mapEventType(%s) = %v, want %v", tt.eventName, result, tt.expected)
			}
		})
	}
}

func TestComplexToolParameters(t *testing.T) {
	converter := NewConverter()

	// Test complex Grep parameters
	grepJSON := `{
		"hook_event_name": "PreToolUse",
		"session_id": "test-grep",
		"tool_name": "Grep",
		"tool_input": {
			"pattern": "TODO",
			"path": "/src",
			"glob": "*.go",
			"-i": true,
			"multiline": false,
			"-C": 3
		}
	}`

	req, err := converter.JSONToProto([]byte(grepJSON))
	if err != nil {
		t.Fatalf("Failed to convert Grep parameters: %v", err)
	}

	payload := req.GetPreToolUse()
	if payload == nil {
		t.Fatal("Expected PreToolUse payload")
	}

	grep := payload.GetGrep()
	if grep == nil {
		t.Fatal("Expected Grep parameters")
	}

	if grep.Pattern != "TODO" {
		t.Errorf("Expected pattern 'TODO', got %s", grep.Pattern)
	}

	if grep.Path == nil || *grep.Path != "/src" {
		t.Error("Expected path to be '/src'")
	}

	if grep.CaseInsensitive == nil || !*grep.CaseInsensitive {
		t.Error("Expected case_insensitive to be true")
	}

	if grep.ContextLines == nil || *grep.ContextLines != 3 {
		t.Error("Expected context_lines to be 3")
	}
}

// Test helper functions
func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int32) *int32 {
	return &i
}

func timestampFromUnix(seconds int64) *timestamppb.Timestamp {
	return &timestamppb.Timestamp{
		Seconds: seconds,
	}
}