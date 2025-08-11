package bridge

import (
	"context"
	"fmt"
	"log"
	"strings"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

// boolPtr returns a pointer to a bool
func boolPtr(b bool) *bool {
	return &b
}

// StandardHandlers provides default handlers for common hook events
type StandardHandlers struct {
	debugMode bool
}

// NewStandardHandlers creates standard hook handlers
func NewStandardHandlers(debug bool) *StandardHandlers {
	return &StandardHandlers{
		debugMode: debug,
	}
}

// PreToolUseHandler handles pre-tool-use hooks with security validation
func (h *StandardHandlers) PreToolUseHandler(ctx context.Context, req *gismov1.HookRequest) (*gismov1.HookResponse, error) {
	payload := req.GetPreToolUse()
	if payload == nil {
		return nil, fmt.Errorf("missing PreToolUse payload")
	}

	if h.debugMode {
		log.Printf("[PreToolUse] Tool: %s, Session: %s", payload.ToolName, req.Base.SessionId)
	}

	// Default response: approve
	response := &gismov1.HookResponse{
		Decision: gismov1.HookResponse_DECISION_APPROVE,
		Continue: boolPtr(true),
	}

	// Tool-specific validation
	switch payload.ToolName {
	case "Bash":
		if err := h.validateBashCommand(payload.GetBash()); err != nil {
			response.Decision = gismov1.HookResponse_DECISION_BLOCK
			response.Reason = fmt.Sprintf("Command blocked: %v", err)
			response.Message = "This command has been blocked for security reasons"
			response.Continue = boolPtr(false)
			response.StopReason = "security_block"
			response.ExitCode = 2
		}

	case "Write", "Edit":
		if err := h.validateFileOperation(payload); err != nil {
			response.Decision = gismov1.HookResponse_DECISION_BLOCK
			response.Reason = fmt.Sprintf("File operation blocked: %v", err)
			response.Continue = boolPtr(false)
		}

	case "Task":
		// Allow task spawning but log it
		if h.debugMode {
			if task := payload.GetTask(); task != nil {
				log.Printf("[Task] Spawning subagent: %s", task.SubagentType)
			}
		}
	}

	return response, nil
}

// PostToolUseHandler handles post-tool-use hooks for auditing
func (h *StandardHandlers) PostToolUseHandler(ctx context.Context, req *gismov1.HookRequest) (*gismov1.HookResponse, error) {
	payload := req.GetPostToolUse()
	if payload == nil {
		return nil, fmt.Errorf("missing PostToolUse payload")
	}

	if h.debugMode {
		log.Printf("[PostToolUse] Tool: %s, ExitCode: %d", payload.ToolName, payload.ExitCode)
		if payload.ToolError != "" {
			log.Printf("[PostToolUse] Error: %s", payload.ToolError)
		}
	}

	// Log failures for security tools
	if payload.ExitCode != 0 && (payload.ToolName == "Bash" || payload.ToolName == "Write") {
		log.Printf("SECURITY: Tool %s failed with exit code %d: %s",
			payload.ToolName, payload.ExitCode, payload.ToolError)
	}

	return &gismov1.HookResponse{
		Continue: boolPtr(true),
	}, nil
}

// UserPromptSubmitHandler handles user prompt submissions
func (h *StandardHandlers) UserPromptSubmitHandler(ctx context.Context, req *gismov1.HookRequest) (*gismov1.HookResponse, error) {
	payload := req.GetUserPromptSubmit()
	if payload == nil {
		return nil, fmt.Errorf("missing UserPromptSubmit payload")
	}

	// Check for suspicious patterns
	prompt := strings.ToLower(payload.UserPrompt)
	suspiciousPatterns := []string{
		"ignore previous instructions",
		"disregard all prior",
		"forget everything",
		"new instructions:",
		"system prompt",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(prompt, pattern) {
			log.Printf("SECURITY: Suspicious prompt pattern detected: %s", pattern)
			return &gismov1.HookResponse{
				Continue:   boolPtr(false),
				StopReason: "suspicious_prompt",
				Message:    "This prompt contains suspicious patterns and has been blocked",
				ExitCode:   2,
			}, nil
		}
	}

	if h.debugMode {
		log.Printf("[UserPrompt] Length: %d, Project: %s",
			len(payload.UserPrompt), req.Base.Environment.GetProjectContext())
	}

	return &gismov1.HookResponse{
		Continue: boolPtr(true),
	}, nil
}

// NotificationHandler handles system notifications
func (h *StandardHandlers) NotificationHandler(ctx context.Context, req *gismov1.HookRequest) (*gismov1.HookResponse, error) {
	payload := req.GetNotification()
	if payload == nil {
		return nil, fmt.Errorf("missing Notification payload")
	}

	if h.debugMode {
		log.Printf("[Notification] Type: %s, Message: %s",
			payload.NotificationType, payload.Message)
	}

	// Log important notifications
	if payload.NotificationType == "error" || payload.NotificationType == "security" {
		log.Printf("SYSTEM: %s notification: %s", payload.NotificationType, payload.Message)
	}

	return &gismov1.HookResponse{
		Continue: boolPtr(true),
	}, nil
}

// SessionStartHandler handles session start events
func (h *StandardHandlers) SessionStartHandler(ctx context.Context, req *gismov1.HookRequest) (*gismov1.HookResponse, error) {
	payload := req.GetSessionStart()
	if payload == nil {
		// Session start might not have payload
		payload = &gismov1.SessionStartPayload{
			SessionType: "interactive",
		}
	}

	env := req.Base.Environment
	if env != nil {
		log.Printf("Session started: Type=%s, Project=%s, Host=%s, OS=%s",
			payload.SessionType,
			env.ProjectContext,
			env.Hostname,
			env.OsInfo)

		// Log git context if available
		if git := env.GitContext; git != nil {
			log.Printf("Git context: Branch=%s, Repo=%s, Uncommitted=%v",
				git.Branch, git.RepoName, git.HasUncommittedChanges)
		}
	}

	return &gismov1.HookResponse{
		Continue: boolPtr(true),
	}, nil
}

// validateBashCommand validates bash commands for security
func (h *StandardHandlers) validateBashCommand(params *gismov1.BashParameters) error {
	if params == nil {
		return fmt.Errorf("missing bash parameters")
	}

	cmd := strings.ToLower(params.Command)

	// Check for obviously dangerous commands
	if strings.Contains(cmd, "rm -rf /") ||
		strings.Contains(cmd, ":(){ :|:& };:") ||
		strings.Contains(cmd, "dd if=/dev/zero") ||
		strings.Contains(cmd, "mkfs") {
		return fmt.Errorf("dangerous command detected")
	}

	// Check for sudo usage
	if strings.HasPrefix(cmd, "sudo ") {
		return fmt.Errorf("sudo commands are not allowed")
	}

	// Check for pipe to shell
	if strings.Contains(cmd, "| sh") || strings.Contains(cmd, "| bash") {
		return fmt.Errorf("piping to shell is not allowed")
	}

	return nil
}

// validateFileOperation validates file write/edit operations
func (h *StandardHandlers) validateFileOperation(payload *gismov1.PreToolUsePayload) error {
	var filePath string

	switch payload.ToolName {
	case "Write":
		if write := payload.GetWrite(); write != nil {
			filePath = write.FilePath
		}
	case "Edit":
		if edit := payload.GetEdit(); edit != nil {
			filePath = edit.FilePath
		}
	default:
		return nil
	}

	if filePath == "" {
		return fmt.Errorf("missing file path")
	}

	// Block system paths
	systemPaths := []string{"/etc/", "/sys/", "/proc/", "/boot/", "/dev/"}
	for _, syspath := range systemPaths {
		if strings.HasPrefix(filePath, syspath) {
			return fmt.Errorf("cannot modify system path: %s", syspath)
		}
	}

	// Block hidden system files
	if strings.Contains(filePath, "/.ssh/") ||
		strings.Contains(filePath, "/.git/hooks/") ||
		strings.HasSuffix(filePath, "/.bashrc") ||
		strings.HasSuffix(filePath, "/.zshrc") {
		return fmt.Errorf("cannot modify sensitive configuration file")
	}

	return nil
}

// RegisterStandardHandlers registers all standard handlers with the router
func RegisterStandardHandlers(router *Router, debug bool) {
	handlers := NewStandardHandlers(debug)

	router.RegisterHandler(gismov1.HookEventType_HOOK_EVENT_TYPE_PRE_TOOL_USE, handlers.PreToolUseHandler)
	router.RegisterHandler(gismov1.HookEventType_HOOK_EVENT_TYPE_POST_TOOL_USE, handlers.PostToolUseHandler)
	router.RegisterHandler(gismov1.HookEventType_HOOK_EVENT_TYPE_USER_PROMPT_SUBMIT, handlers.UserPromptSubmitHandler)
	router.RegisterHandler(gismov1.HookEventType_HOOK_EVENT_TYPE_NOTIFICATION, handlers.NotificationHandler)
	router.RegisterHandler(gismov1.HookEventType_HOOK_EVENT_TYPE_SESSION_START, handlers.SessionStartHandler)
}
