package bridge

import (
	"fmt"
	"strings"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/proto"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

// Validator handles protobuf message validation using protovalidate
type Validator struct {
	validator protovalidate.Validator
	// Custom validation functions
	customValidators map[string]ValidationFunc
}

// ValidationFunc is a custom validation function
type ValidationFunc func(proto.Message) error

// NewValidator creates a new message validator
func NewValidator() (*Validator, error) {
	v, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create protovalidate validator: %w", err)
	}

	return &Validator{
		validator:        v,
		customValidators: make(map[string]ValidationFunc),
	}, nil
}

// ValidateRequest validates a hook request message
func (v *Validator) ValidateRequest(req *gismov1.HookRequest) error {
	// First, run protovalidate validation
	if err := v.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Apply custom validations based on event type
	switch req.Base.EventType {
	case gismov1.HookEventType_HOOK_EVENT_TYPE_PRE_TOOL_USE:
		if payload := req.GetPreToolUse(); payload != nil {
			if err := v.validatePreToolUse(payload); err != nil {
				return err
			}
		}

	case gismov1.HookEventType_HOOK_EVENT_TYPE_USER_PROMPT_SUBMIT:
		if payload := req.GetUserPromptSubmit(); payload != nil {
			if err := v.validateUserPrompt(payload); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateResponse validates a hook response message
func (v *Validator) ValidateResponse(resp *gismov1.HookResponse) error {
	// Run protovalidate validation
	if err := v.validator.Validate(resp); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Custom validation: Check decision consistency
	if resp.Decision == gismov1.HookResponse_DECISION_BLOCK && resp.Reason == "" {
		return fmt.Errorf("blocking decision requires a reason")
	}

	if resp.Decision == gismov1.HookResponse_DECISION_MODIFY && resp.ModifiedParameters == nil {
		return fmt.Errorf("modify decision requires modified parameters")
	}

	return nil
}

// validatePreToolUse applies custom validation for PreToolUse payloads
func (v *Validator) validatePreToolUse(payload *gismov1.PreToolUsePayload) error {
	// Validate based on tool type
	switch payload.ToolName {
	case "Bash":
		if bash := payload.GetBash(); bash != nil {
			return v.validateBashCommand(bash)
		}

	case "Edit", "MultiEdit":
		if edit := payload.GetEdit(); edit != nil {
			return v.validateEditOperation(edit)
		}

	case "Write":
		if write := payload.GetWrite(); write != nil {
			return v.validateWriteOperation(write)
		}
	}

	return nil
}

// validateBashCommand validates bash commands for security
func (v *Validator) validateBashCommand(params *gismov1.BashParameters) error {
	cmd := strings.ToLower(params.Command)

	// Check for dangerous patterns
	dangerousPatterns := []string{
		"rm -rf /",
		"rm -rf /*",
		"dd if=/dev/zero",
		"dd if=/dev/urandom",
		":(){ :|:& };:",      // Fork bomb
		"> /dev/sda",         // Disk overwrite
		"chmod -R 777 /",     // Permission destruction
		"mv /* /dev/null",    // Move everything to null
		"mkfs",               // Format filesystem
		"kill -9 -1",         // Kill all processes
		"sudo rm",            // Sudo remove
		"curl | sh",          // Pipe curl to shell
		"wget | bash",        // Pipe wget to bash
		"eval(",              // Eval execution
		"exec(",              // Exec execution
		"__import__",         // Python import
		"os.system",          // Python system call
		"subprocess",         // Python subprocess
		"`rm",                // Backtick command execution
		"$(rm",               // Command substitution
		"../../../../../../", // Path traversal
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(cmd, pattern) {
			return fmt.Errorf("dangerous command pattern detected: %s", pattern)
		}
	}

	// Check for attempts to modify system files
	systemPaths := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/etc/sudoers",
		"/boot/",
		"/sys/",
		"/proc/",
		"/dev/",
		"~/.ssh/",
		"~/.bashrc",
		"~/.zshrc",
	}

	for _, path := range systemPaths {
		if strings.Contains(cmd, path) {
			return fmt.Errorf("command attempts to modify system path: %s", path)
		}
	}

	// Check command length
	if len(params.Command) > 10000 {
		return fmt.Errorf("command exceeds maximum length of 10000 characters")
	}

	// Validate timeout if specified
	if params.Timeout != nil {
		if *params.Timeout < 100 || *params.Timeout > 600000 {
			return fmt.Errorf("timeout must be between 100ms and 10 minutes")
		}
	}

	return nil
}

// validateEditOperation validates file edit operations
func (v *Validator) validateEditOperation(params *gismov1.EditParameters) error {
	// Check for path traversal
	if err := v.validatePath(params.FilePath); err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	// Check string lengths
	if len(params.OldString) == 0 {
		return fmt.Errorf("old_string cannot be empty")
	}

	if len(params.OldString) > 100000 || len(params.NewString) > 100000 {
		return fmt.Errorf("edit strings exceed maximum length of 100000 characters")
	}

	// Ensure old and new are different
	if params.OldString == params.NewString {
		return fmt.Errorf("old_string and new_string cannot be identical")
	}

	return nil
}

// validateWriteOperation validates file write operations
func (v *Validator) validateWriteOperation(params *gismov1.WriteParameters) error {
	// Check for path traversal
	if err := v.validatePath(params.FilePath); err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	// Check content size
	if len(params.Content) > 10*1024*1024 { // 10MB limit
		return fmt.Errorf("content exceeds maximum size of 10MB")
	}

	// Check for attempts to write to sensitive locations
	sensitivePatterns := []string{
		"/etc/",
		"/sys/",
		"/proc/",
		"/boot/",
		"/dev/",
		".ssh/",
		".git/hooks/",
	}

	lowerPath := strings.ToLower(params.FilePath)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerPath, pattern) {
			return fmt.Errorf("cannot write to sensitive location: %s", pattern)
		}
	}

	return nil
}

// validateUserPrompt validates user prompts for security
func (v *Validator) validateUserPrompt(payload *gismov1.UserPromptSubmitPayload) error {
	prompt := strings.ToLower(payload.UserPrompt)

	// Check for injection attempts
	injectionPatterns := []string{
		"__import__",
		"eval(",
		"exec(",
		"compile(",
		"globals(",
		"locals(",
		"vars(",
		"<script",
		"javascript:",
		"onerror=",
		"onclick=",
		"DROP TABLE",
		"DELETE FROM",
		"INSERT INTO",
		"UPDATE SET",
		"; --",
		"' OR '1'='1",
	}

	for _, pattern := range injectionPatterns {
		if strings.Contains(prompt, pattern) {
			return fmt.Errorf("potential injection pattern detected: %s", pattern)
		}
	}

	// Check prompt length
	if len(payload.UserPrompt) > 50000 {
		return fmt.Errorf("prompt exceeds maximum length of 50000 characters")
	}

	return nil
}

// validatePath validates file paths for security
func (v *Validator) validatePath(path string) error {
	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path contains null bytes")
	}

	// Check for path traversal
	if strings.Contains(path, "../") || strings.Contains(path, "..\\") {
		return fmt.Errorf("path traversal detected")
	}

	// Check path length
	if len(path) > 4096 {
		return fmt.Errorf("path exceeds maximum length of 4096 characters")
	}

	// Ensure path doesn't start with sensitive prefixes
	if strings.HasPrefix(path, "/etc/") ||
		strings.HasPrefix(path, "/sys/") ||
		strings.HasPrefix(path, "/proc/") ||
		strings.HasPrefix(path, "/boot/") ||
		strings.HasPrefix(path, "/dev/") {
		return fmt.Errorf("path points to system directory")
	}

	return nil
}

// AddCustomValidator adds a custom validation function for a specific message type
func (v *Validator) AddCustomValidator(messageType string, fn ValidationFunc) {
	v.customValidators[messageType] = fn
}

// GetValidationErrors returns detailed validation errors
func (v *Validator) GetValidationErrors(err error) []string {
	if err == nil {
		return nil
	}

	// Extract detailed validation errors from protovalidate
	var errors []string
	errStr := err.Error()

	// Split by newline to get individual violations
	lines := strings.Split(errStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "validation error") {
			errors = append(errors, line)
		}
	}

	if len(errors) == 0 {
		errors = append(errors, errStr)
	}

	return errors
}

// SafeCommandPatterns returns a list of safe command patterns for reference
func SafeCommandPatterns() []string {
	return []string{
		"ls", "pwd", "echo", "cat", "grep", "find", "wc",
		"head", "tail", "sort", "uniq", "cut", "awk", "sed",
		"git status", "git log", "git diff", "git branch",
		"npm list", "npm test", "npm run",
		"go test", "go build", "go fmt", "go vet",
		"python -m", "pip list", "pip show",
		"docker ps", "docker images", "docker logs",
		"curl -I", "wget --spider", "ping -c",
	}
}

// DangerousCommandKeywords returns keywords that indicate dangerous commands
func DangerousCommandKeywords() []string {
	return []string{
		"rm", "delete", "format", "mkfs", "dd", "shred",
		"kill", "killall", "shutdown", "reboot", "halt",
		"chmod 777", "chown", "sudo", "su -",
		"eval", "exec", "source", ".",
		"curl | sh", "wget | bash", "| sh", "| bash",
	}
}
