package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	json "github.com/goccy/go-json"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jrossi/gismo/pkg/engine"
	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

// Converter handles conversion between JSON hook messages and Protobuf
type Converter struct {
	// Unmarshaler for JSON to protobuf conversion
	unmarshaler *protojson.UnmarshalOptions
	// Marshaler for protobuf to JSON conversion
	marshaler *protojson.MarshalOptions
}

// NewConverter creates a new JSON/Protobuf converter
func NewConverter() *Converter {
	return &Converter{
		unmarshaler: &protojson.UnmarshalOptions{
			DiscardUnknown: false, // Keep unknown fields for forward compatibility
		},
		marshaler: &protojson.MarshalOptions{
			UseProtoNames:   true,
			EmitUnpopulated: false,
			UseEnumNumbers:  false,
		},
	}
}

// JSONToProto converts a JSON hook message to protobuf
func (c *Converter) JSONToProto(jsonData []byte) (*gismov1.HookRequest, error) {
	// First, parse to determine hook type
	var rawMsg map[string]json.RawMessage
	if err := json.Unmarshal(jsonData, &rawMsg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract hook event name
	var eventName string
	if eventData, ok := rawMsg["hook_event_name"]; ok {
		if err := json.Unmarshal(eventData, &eventName); err != nil {
			return nil, fmt.Errorf("failed to parse hook_event_name: %w", err)
		}
	}

	// Create base message
	base := &gismov1.BaseHookMessage{
		EventType: c.mapEventType(eventName),
		Timestamp: timestamppb.Now(),
	}

	// Extract common fields
	if sessionID, ok := rawMsg["session_id"]; ok {
		json.Unmarshal(sessionID, &base.SessionId)
	}
	if transcriptPath, ok := rawMsg["transcript_path"]; ok {
		json.Unmarshal(transcriptPath, &base.TranscriptPath)
	}

	// Populate environment metadata
	base.Environment = c.extractEnvironmentMetadata(rawMsg)

	// Create the appropriate payload based on event type
	req := &gismov1.HookRequest{Base: base}

	switch eventName {
	case "PreToolUse":
		payload, err := c.parsePreToolUse(rawMsg)
		if err != nil {
			return nil, err
		}
		req.Payload = &gismov1.HookRequest_PreToolUse{PreToolUse: payload}

	case "PostToolUse":
		payload, err := c.parsePostToolUse(rawMsg)
		if err != nil {
			return nil, err
		}
		req.Payload = &gismov1.HookRequest_PostToolUse{PostToolUse: payload}

	case "UserPromptSubmit":
		payload, err := c.parseUserPromptSubmit(rawMsg)
		if err != nil {
			return nil, err
		}
		req.Payload = &gismov1.HookRequest_UserPromptSubmit{UserPromptSubmit: payload}

	case "Notification":
		payload, err := c.parseNotification(rawMsg)
		if err != nil {
			return nil, err
		}
		req.Payload = &gismov1.HookRequest_Notification{Notification: payload}

	case "Stop":
		payload, err := c.parseStop(rawMsg)
		if err != nil {
			return nil, err
		}
		req.Payload = &gismov1.HookRequest_Stop{Stop: payload}

	case "SubagentStop":
		payload, err := c.parseSubagentStop(rawMsg)
		if err != nil {
			return nil, err
		}
		req.Payload = &gismov1.HookRequest_SubagentStop{SubagentStop: payload}

	case "PreCompact":
		payload, err := c.parsePreCompact(rawMsg)
		if err != nil {
			return nil, err
		}
		req.Payload = &gismov1.HookRequest_PreCompact{PreCompact: payload}

	case "SessionStart":
		payload := &gismov1.SessionStartPayload{
			SessionType: "interactive",
			Metadata:    make(map[string]string),
		}
		req.Payload = &gismov1.HookRequest_SessionStart{SessionStart: payload}

	default:
		return nil, fmt.Errorf("unknown hook event type: %s", eventName)
	}

	return req, nil
}

// ProtoToJSON converts a protobuf hook response to JSON
func (c *Converter) ProtoToJSON(resp *gismov1.HookResponse) ([]byte, error) {
	// Create engine.HookResponse from protobuf
	engineResp := &engine.HookResponse{}

	if resp.Continue != nil {
		engineResp.Continue = resp.Continue
	}
	if resp.StopReason != "" {
		engineResp.StopReason = resp.StopReason
	}
	if resp.SuppressOutput != nil {
		engineResp.SuppressOutput = resp.SuppressOutput
	}

	// Map decision
	switch resp.Decision {
	case gismov1.HookResponse_DECISION_APPROVE:
		engineResp.Decision = "approve"
	case gismov1.HookResponse_DECISION_BLOCK:
		engineResp.Decision = "block"
	case gismov1.HookResponse_DECISION_MODIFY:
		engineResp.Decision = "modify"
	}

	engineResp.Reason = resp.Reason
	engineResp.Message = resp.Message

	// Convert to JSON
	return json.Marshal(engineResp)
}

// parsePreToolUse parses PreToolUse payload
func (c *Converter) parsePreToolUse(rawMsg map[string]json.RawMessage) (*gismov1.PreToolUsePayload, error) {
	payload := &gismov1.PreToolUsePayload{}

	// Extract tool name
	if toolName, ok := rawMsg["tool_name"]; ok {
		if err := json.Unmarshal(toolName, &payload.ToolName); err != nil {
			return nil, fmt.Errorf("failed to parse tool_name: %w", err)
		}
	}

	// Extract tool input and convert to appropriate parameter type
	if toolInput, ok := rawMsg["tool_input"]; ok {
		var inputMap map[string]json.RawMessage
		if err := json.Unmarshal(toolInput, &inputMap); err != nil {
			return nil, fmt.Errorf("failed to parse tool_input: %w", err)
		}

		// Route to specific parameter type based on tool name
		switch payload.ToolName {
		case "Bash":
			params, err := c.parseBashParams(inputMap)
			if err != nil {
				return nil, err
			}
			payload.Parameters = &gismov1.PreToolUsePayload_Bash{Bash: params}

		case "Edit", "MultiEdit":
			params, err := c.parseEditParams(inputMap)
			if err != nil {
				return nil, err
			}
			payload.Parameters = &gismov1.PreToolUsePayload_Edit{Edit: params}

		case "Write":
			params, err := c.parseWriteParams(inputMap)
			if err != nil {
				return nil, err
			}
			payload.Parameters = &gismov1.PreToolUsePayload_Write{Write: params}

		case "Read":
			params, err := c.parseReadParams(inputMap)
			if err != nil {
				return nil, err
			}
			payload.Parameters = &gismov1.PreToolUsePayload_Read{Read: params}

		case "Task":
			params, err := c.parseTaskParams(inputMap)
			if err != nil {
				return nil, err
			}
			payload.Parameters = &gismov1.PreToolUsePayload_Task{Task: params}

		case "Grep":
			params, err := c.parseGrepParams(inputMap)
			if err != nil {
				return nil, err
			}
			payload.Parameters = &gismov1.PreToolUsePayload_Grep{Grep: params}

		case "Glob":
			params, err := c.parseGlobParams(inputMap)
			if err != nil {
				return nil, err
			}
			payload.Parameters = &gismov1.PreToolUsePayload_Glob{Glob: params}

		default:
			// Use generic struct for unknown tools
			structParams, err := c.mapToStruct(inputMap)
			if err != nil {
				return nil, err
			}
			payload.Parameters = &gismov1.PreToolUsePayload_Generic{Generic: structParams}
		}
	}

	return payload, nil
}

// parseBashParams parses Bash tool parameters
func (c *Converter) parseBashParams(input map[string]json.RawMessage) (*gismov1.BashParameters, error) {
	params := &gismov1.BashParameters{}

	if cmd, ok := input["command"]; ok {
		json.Unmarshal(cmd, &params.Command)
	}
	if desc, ok := input["description"]; ok {
		json.Unmarshal(desc, &params.Description)
	}
	if timeout, ok := input["timeout"]; ok {
		var t int32
		if err := json.Unmarshal(timeout, &t); err == nil {
			params.Timeout = &t
		}
	}

	return params, nil
}

// parseEditParams parses Edit tool parameters
func (c *Converter) parseEditParams(input map[string]json.RawMessage) (*gismov1.EditParameters, error) {
	params := &gismov1.EditParameters{}

	if fp, ok := input["file_path"]; ok {
		json.Unmarshal(fp, &params.FilePath)
	}
	if old, ok := input["old_string"]; ok {
		json.Unmarshal(old, &params.OldString)
	}
	if new, ok := input["new_string"]; ok {
		json.Unmarshal(new, &params.NewString)
	}
	if replaceAll, ok := input["replace_all"]; ok {
		var ra bool
		if err := json.Unmarshal(replaceAll, &ra); err == nil {
			params.ReplaceAll = &ra
		}
	}

	return params, nil
}

// parseWriteParams parses Write tool parameters
func (c *Converter) parseWriteParams(input map[string]json.RawMessage) (*gismov1.WriteParameters, error) {
	params := &gismov1.WriteParameters{}

	if fp, ok := input["file_path"]; ok {
		json.Unmarshal(fp, &params.FilePath)
	}
	if content, ok := input["content"]; ok {
		json.Unmarshal(content, &params.Content)
	}

	return params, nil
}

// parseReadParams parses Read tool parameters
func (c *Converter) parseReadParams(input map[string]json.RawMessage) (*gismov1.ReadParameters, error) {
	params := &gismov1.ReadParameters{}

	if fp, ok := input["file_path"]; ok {
		json.Unmarshal(fp, &params.FilePath)
	}
	if limit, ok := input["limit"]; ok {
		var l int32
		if err := json.Unmarshal(limit, &l); err == nil {
			params.Limit = &l
		}
	}
	if offset, ok := input["offset"]; ok {
		var o int32
		if err := json.Unmarshal(offset, &o); err == nil {
			params.Offset = &o
		}
	}

	return params, nil
}

// parseTaskParams parses Task tool parameters
func (c *Converter) parseTaskParams(input map[string]json.RawMessage) (*gismov1.TaskParameters, error) {
	params := &gismov1.TaskParameters{}

	if desc, ok := input["description"]; ok {
		json.Unmarshal(desc, &params.Description)
	}
	if prompt, ok := input["prompt"]; ok {
		json.Unmarshal(prompt, &params.Prompt)
	}
	if subagent, ok := input["subagent_type"]; ok {
		json.Unmarshal(subagent, &params.SubagentType)
	}

	return params, nil
}

// parseGrepParams parses Grep tool parameters
func (c *Converter) parseGrepParams(input map[string]json.RawMessage) (*gismov1.GrepParameters, error) {
	params := &gismov1.GrepParameters{}

	if pattern, ok := input["pattern"]; ok {
		json.Unmarshal(pattern, &params.Pattern)
	}
	if path, ok := input["path"]; ok {
		var p string
		if err := json.Unmarshal(path, &p); err == nil && p != "" {
			params.Path = &p
		}
	}
	if glob, ok := input["glob"]; ok {
		var g string
		if err := json.Unmarshal(glob, &g); err == nil && g != "" {
			params.Glob = &g
		}
	}
	if ci, ok := input["-i"]; ok {
		var b bool
		if err := json.Unmarshal(ci, &b); err == nil {
			params.CaseInsensitive = &b
		}
	}
	if ml, ok := input["multiline"]; ok {
		var b bool
		if err := json.Unmarshal(ml, &b); err == nil {
			params.Multiline = &b
		}
	}
	if ctx, ok := input["-C"]; ok {
		var c int32
		if err := json.Unmarshal(ctx, &c); err == nil {
			params.ContextLines = &c
		}
	}

	return params, nil
}

// parseGlobParams parses Glob tool parameters
func (c *Converter) parseGlobParams(input map[string]json.RawMessage) (*gismov1.GlobParameters, error) {
	params := &gismov1.GlobParameters{}

	if pattern, ok := input["pattern"]; ok {
		json.Unmarshal(pattern, &params.Pattern)
	}
	if path, ok := input["path"]; ok {
		var p string
		if err := json.Unmarshal(path, &p); err == nil && p != "" {
			params.Path = &p
		}
	}

	return params, nil
}

// parsePostToolUse parses PostToolUse payload
func (c *Converter) parsePostToolUse(rawMsg map[string]json.RawMessage) (*gismov1.PostToolUsePayload, error) {
	payload := &gismov1.PostToolUsePayload{}

	if toolName, ok := rawMsg["tool_name"]; ok {
		json.Unmarshal(toolName, &payload.ToolName)
	}

	if toolInput, ok := rawMsg["tool_input"]; ok {
		var inputMap map[string]json.RawMessage
		if err := json.Unmarshal(toolInput, &inputMap); err == nil {
			if s, err := c.mapToStruct(inputMap); err == nil {
				payload.ToolInput = s
			}
		}
	}

	if toolOutput, ok := rawMsg["tool_output"]; ok {
		val, err := c.jsonToValue(toolOutput)
		if err == nil {
			payload.ToolOutput = val
		}
	}

	if toolError, ok := rawMsg["tool_error"]; ok {
		json.Unmarshal(toolError, &payload.ToolError)
	}

	return payload, nil
}

// parseUserPromptSubmit parses UserPromptSubmit payload
func (c *Converter) parseUserPromptSubmit(rawMsg map[string]json.RawMessage) (*gismov1.UserPromptSubmitPayload, error) {
	payload := &gismov1.UserPromptSubmitPayload{}

	if prompt, ok := rawMsg["user_prompt"]; ok {
		json.Unmarshal(prompt, &payload.UserPrompt)
	}

	if ts, ok := rawMsg["timestamp"]; ok {
		var timestamp int64
		if err := json.Unmarshal(ts, &timestamp); err == nil {
			payload.Timestamp = timestamppb.New(time.Unix(timestamp, 0))
		}
	} else {
		payload.Timestamp = timestamppb.Now()
	}

	return payload, nil
}

// parseNotification parses Notification payload
func (c *Converter) parseNotification(rawMsg map[string]json.RawMessage) (*gismov1.NotificationPayload, error) {
	payload := &gismov1.NotificationPayload{}

	if nt, ok := rawMsg["notification_type"]; ok {
		json.Unmarshal(nt, &payload.NotificationType)
	}
	if msg, ok := rawMsg["message"]; ok {
		json.Unmarshal(msg, &payload.Message)
	}

	return payload, nil
}

// parseStop parses Stop payload
func (c *Converter) parseStop(rawMsg map[string]json.RawMessage) (*gismov1.StopPayload, error) {
	payload := &gismov1.StopPayload{}

	if reason, ok := rawMsg["reason"]; ok {
		json.Unmarshal(reason, &payload.Reason)
	}
	if final, ok := rawMsg["final_message"]; ok {
		json.Unmarshal(final, &payload.FinalMessage)
	}

	return payload, nil
}

// parseSubagentStop parses SubagentStop payload
func (c *Converter) parseSubagentStop(rawMsg map[string]json.RawMessage) (*gismov1.SubagentStopPayload, error) {
	payload := &gismov1.SubagentStopPayload{}

	if id, ok := rawMsg["subagent_id"]; ok {
		json.Unmarshal(id, &payload.SubagentId)
	}
	if name, ok := rawMsg["subagent_name"]; ok {
		json.Unmarshal(name, &payload.SubagentName)
	}
	if result, ok := rawMsg["result"]; ok {
		json.Unmarshal(result, &payload.Result)
	}

	return payload, nil
}

// parsePreCompact parses PreCompact payload
func (c *Converter) parsePreCompact(rawMsg map[string]json.RawMessage) (*gismov1.PreCompactPayload, error) {
	payload := &gismov1.PreCompactPayload{}

	if current, ok := rawMsg["current_tokens"]; ok {
		json.Unmarshal(current, &payload.CurrentTokens)
	}
	if target, ok := rawMsg["target_tokens"]; ok {
		json.Unmarshal(target, &payload.TargetTokens)
	}

	return payload, nil
}

// mapEventType maps string event name to protobuf enum
func (c *Converter) mapEventType(eventName string) gismov1.HookEventType {
	switch eventName {
	case "PreToolUse":
		return gismov1.HookEventType_HOOK_EVENT_TYPE_PRE_TOOL_USE
	case "PostToolUse":
		return gismov1.HookEventType_HOOK_EVENT_TYPE_POST_TOOL_USE
	case "Notification":
		return gismov1.HookEventType_HOOK_EVENT_TYPE_NOTIFICATION
	case "Stop":
		return gismov1.HookEventType_HOOK_EVENT_TYPE_STOP
	case "SubagentStop":
		return gismov1.HookEventType_HOOK_EVENT_TYPE_SUBAGENT_STOP
	case "PreCompact":
		return gismov1.HookEventType_HOOK_EVENT_TYPE_PRE_COMPACT
	case "UserPromptSubmit":
		return gismov1.HookEventType_HOOK_EVENT_TYPE_USER_PROMPT_SUBMIT
	case "SessionStart":
		return gismov1.HookEventType_HOOK_EVENT_TYPE_SESSION_START
	default:
		return gismov1.HookEventType_HOOK_EVENT_TYPE_UNSPECIFIED
	}
}

// mapToStruct converts a map of JSON raw messages to protobuf Struct
func (c *Converter) mapToStruct(m map[string]json.RawMessage) (*structpb.Struct, error) {
	fields := make(map[string]*structpb.Value)
	for k, v := range m {
		val, err := c.jsonToValue(v)
		if err != nil {
			return nil, fmt.Errorf("failed to convert field %s: %w", k, err)
		}
		fields[k] = val
	}
	return &structpb.Struct{Fields: fields}, nil
}

// jsonToValue converts JSON raw message to protobuf Value
func (c *Converter) jsonToValue(raw json.RawMessage) (*structpb.Value, error) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return structpb.NewValue(v)
}

// extractEnvironmentMetadata populates environment metadata from the current system
func (c *Converter) extractEnvironmentMetadata(rawMsg map[string]json.RawMessage) *gismov1.EnvironmentMetadata {
	env := &gismov1.EnvironmentMetadata{
		EnvVars: make(map[string]string),
	}

	// Get working directory
	if wd, err := os.Getwd(); err == nil {
		env.WorkingDirectory = wd
	}

	// Get hostname
	if hostname, err := os.Hostname(); err == nil {
		env.Hostname = hostname
	}

	// Get OS info
	env.OsInfo = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	// Get runtime version
	env.RuntimeVersion = "gismo/1.0.0"

	// Extract project context from JSON if available
	if projectContext, ok := rawMsg["project_context"]; ok {
		json.Unmarshal(projectContext, &env.ProjectContext)
	}

	// Try to detect git repository
	env.GitContext = c.detectGitContext(env.WorkingDirectory)

	// Add selected environment variables
	c.addRelevantEnvVars(env)

	return env
}

// detectGitContext attempts to detect git repository information
func (c *Converter) detectGitContext(workingDir string) *gismov1.GitContext {
	if workingDir == "" {
		return nil
	}

	// Walk up directory tree looking for .git directory
	dir := workingDir
	for {
		gitDir := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			// Found git repository
			return &gismov1.GitContext{
				// These would be populated by actual git commands
				// For now, just mark that we're in a git repo
				RepoName: filepath.Base(dir),
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached root
		}
		dir = parent
	}

	return nil
}

// addRelevantEnvVars adds relevant environment variables to the metadata
func (c *Converter) addRelevantEnvVars(env *gismov1.EnvironmentMetadata) {
	// Add relevant environment variables (filtered for security)
	relevantVars := []string{
		"USER",
		"HOME",
		"PATH",
		"SHELL",
		"TERM",
		"LANG",
		"LC_ALL",
		"EDITOR",
		"VISUAL",
		"GISMO_DEBUG",
		"GISMO_CONFIG",
		"CI",
		"CI_COMMIT_SHA",
		"CI_BRANCH",
		"GITHUB_ACTIONS",
		"GITHUB_SHA",
		"GITHUB_REF",
	}

	for _, key := range relevantVars {
		if value := os.Getenv(key); value != "" {
			// Sanitize values to avoid exposing sensitive data
			if key == "PATH" {
				// Just indicate PATH is set, don't expose full value
				env.EnvVars[key] = "<set>"
			} else if len(value) > 100 {
				// Truncate long values
				env.EnvVars[key] = value[:100] + "..."
			} else {
				env.EnvVars[key] = value
			}
		}
	}
}
