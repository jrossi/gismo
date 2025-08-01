package handlers

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo"
	"github.com/jrossi/gismo/linters/secrets"
)

// SecretDetectionHandler handles secret detection for user prompts and file operations
type SecretDetectionHandler struct {
	*gismo.BaseActionHandler
	secretLinter *secrets.SecretLinter
	config       *SecretDetectionConfig
}

// SecretDetectionConfig provides configuration for secret detection
type SecretDetectionConfig struct {
	// EnablePromptScanning enables scanning of user prompts
	EnablePromptScanning bool `json:"enable_prompt_scanning"`

	// EnableFileScanning enables scanning of file contents
	EnableFileScanning bool `json:"enable_file_scanning"`

	// AllowOverride enables the secret_detect=override pattern for prompts
	AllowOverride bool `json:"allow_override"`

	// BlockOnDetection determines if secrets should block operations
	BlockOnDetection bool `json:"block_on_detection"`

	// DisabledRules lists rule IDs to disable
	DisabledRules []string `json:"disabled_rules"`

	// MaxFileSize sets the maximum file size to scan (in bytes)
	MaxFileSize int `json:"max_file_size"`
}

// DefaultSecretDetectionConfig returns default configuration
func DefaultSecretDetectionConfig() *SecretDetectionConfig {
	return &SecretDetectionConfig{
		EnablePromptScanning: true,
		EnableFileScanning:   false, // Opt-in for file scanning
		AllowOverride:        true,
		BlockOnDetection:     true,
		DisabledRules:        []string{},
		MaxFileSize:          1024 * 1024, // 1MB
	}
}

// NewSecretDetectionHandler creates a new secret detection handler
func NewSecretDetectionHandler() *SecretDetectionHandler {
	return NewSecretDetectionHandlerWithConfig(DefaultSecretDetectionConfig())
}

// NewSecretDetectionHandlerWithConfig creates a new secret detection handler with custom configuration
func NewSecretDetectionHandlerWithConfig(config *SecretDetectionConfig) *SecretDetectionHandler {
	if config == nil {
		config = DefaultSecretDetectionConfig()
	}

	// Configure the secrets linter based on our config
	secretsConfig := &secrets.SecretsConfig{
		EnableFileScanning: config.EnableFileScanning,
		MaxFileSize:        config.MaxFileSize,
		DisabledRules:      config.DisabledRules,
		AllowOverride:      config.AllowOverride,
	}

	return &SecretDetectionHandler{
		BaseActionHandler: gismo.NewBaseActionHandler("secret-detection", 200), // Higher priority than linting
		secretLinter:      secrets.NewSecretLinterWithConfig(secretsConfig),
		config:            config,
	}
}

// ShouldHandle determines if this handler should process the event
func (h *SecretDetectionHandler) ShouldHandle(ctx context.Context, event gismo.HookMessage) bool {
	switch event.(type) {
	case *gismo.UserPromptSubmitMessage:
		return h.config.EnablePromptScanning
	case *gismo.PreToolUseMessage:
		return h.config.EnableFileScanning
	default:
		return false
	}
}

// HandleUserPromptSubmit scans user prompts for secrets
func (h *SecretDetectionHandler) HandleUserPromptSubmit(ctx context.Context, msg *gismo.UserPromptSubmitMessage) (*gismo.HookResponse, error) {
	if !h.config.EnablePromptScanning {
		return &gismo.HookResponse{Decision: "approve"}, nil
	}

	// Check for override pattern if enabled
	if h.config.AllowOverride {
		if matched, _ := regexp.MatchString(`secret_detect=override`, msg.UserPrompt); matched {
			fmt.Fprintf(os.Stderr, "\n> Prompt security check:\n  - [secret-detection]: Override detected, skipping secret scan\n")
			return &gismo.HookResponse{Decision: "approve"}, nil
		}
	}

	// Scan the prompt using the secret linter
	result, err := h.secretLinter.ScanPrompt(msg.UserPrompt)
	if err != nil {
		// If scanning fails, log but don't block unless configured to do so
		fmt.Fprintf(os.Stderr, "\n> Secret detection warning: %v\n", err)
		if h.config.BlockOnDetection {
			return &gismo.HookResponse{
				Decision: "block",
				Reason:   fmt.Sprintf("Secret detection failed: %v", err),
			}, nil
		}
		return &gismo.HookResponse{Decision: "approve"}, nil
	}

	if len(result.Issues) == 0 {
		return &gismo.HookResponse{Decision: "approve"}, nil
	}

	// Format findings for user
	var details strings.Builder
	details.WriteString("Found potential secrets in your prompt:\n")
	for i, issue := range result.Issues {
		if i > 0 {
			details.WriteString("\n")
		}
		details.WriteString(fmt.Sprintf("  - %s: %s (line %d)",
			issue.Rule, issue.Message, issue.Line))
	}
	if h.config.AllowOverride {
		details.WriteString("\n\nTo override this check, include 'secret_detect=override' in your prompt.")
	}

	// Write detailed feedback to stderr
	fmt.Fprintf(os.Stderr, "\n> Prompt security check:\n%s\n", details.String())

	if h.config.BlockOnDetection {
		return &gismo.HookResponse{
			Decision: "block",
			Reason:   fmt.Sprintf("Found %d potential secret(s) in prompt", len(result.Issues)),
		}, nil
	}

	// Don't block, but warn
	return &gismo.HookResponse{
		Decision: "approve",
		Message:  fmt.Sprintf("Warning: Found %d potential secret(s) in prompt", len(result.Issues)),
	}, nil
}

// HandlePreToolUse scans file content for secrets before writing
func (h *SecretDetectionHandler) HandlePreToolUse(ctx context.Context, msg *gismo.PreToolUseMessage) (*gismo.HookResponse, error) {
	if !h.config.EnableFileScanning {
		return &gismo.HookResponse{Decision: "approve"}, nil
	}

	// Only check Write operations
	if msg.ToolName != "Write" {
		return &gismo.HookResponse{Decision: "approve"}, nil
	}

	// Extract file path and content
	filePath, content, err := h.extractFileContent(msg)
	if err != nil {
		// If we can't extract content, approve (let other handlers deal with it)
		return &gismo.HookResponse{Decision: "approve"}, nil
	}

	// Use the secret linter to scan the content
	result, err := h.secretLinter.Lint(ctx, filePath, []byte(content))
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n> Secret detection error for %s: %v\n", filePath, err)
		if h.config.BlockOnDetection {
			return &gismo.HookResponse{
				Decision: "block",
				Reason:   fmt.Sprintf("Secret detection failed: %v", err),
			}, nil
		}
		return &gismo.HookResponse{Decision: "approve"}, nil
	}

	if len(result.Issues) == 0 {
		return &gismo.HookResponse{Decision: "approve"}, nil
	}

	// Format findings for user
	var details strings.Builder
	details.WriteString(fmt.Sprintf("Found potential secrets in %s:\n", filePath))
	for i, issue := range result.Issues {
		if i > 0 {
			details.WriteString("\n")
		}
		details.WriteString(fmt.Sprintf("  - %s: %s (line %d, col %d)",
			issue.Rule, issue.Message, issue.Line, issue.Column))
	}

	// Write detailed feedback to stderr
	fmt.Fprintf(os.Stderr, "\n> File security check:\n%s\n", details.String())

	if h.config.BlockOnDetection {
		return &gismo.HookResponse{
			Decision: "block",
			Reason:   fmt.Sprintf("Found %d potential secret(s) in %s", len(result.Issues), filePath),
		}, nil
	}

	// Don't block, but warn
	return &gismo.HookResponse{
		Decision: "approve",
		Message:  fmt.Sprintf("Warning: Found %d potential secret(s) in %s", len(result.Issues), filePath),
	}, nil
}

// extractFileContent extracts file path and content from PreToolUse message
func (h *SecretDetectionHandler) extractFileContent(msg *gismo.PreToolUseMessage) (string, string, error) {
	// Extract file path
	filePathRaw, exists := msg.ToolInput["file_path"]
	if !exists {
		return "", "", fmt.Errorf("no file_path in tool input")
	}
	var filePath string
	if err := json.Unmarshal(filePathRaw, &filePath); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal file_path: %w", err)
	}

	// Extract content
	contentRaw, exists := msg.ToolInput["content"]
	if !exists {
		return "", "", fmt.Errorf("no content in tool input")
	}
	var content string
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal content: %w", err)
	}

	return filePath, content, nil
}

// SetConfig updates the handler configuration
func (h *SecretDetectionHandler) SetConfig(config *SecretDetectionConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	h.config = config

	// Update the secret linter configuration
	secretsConfig := &secrets.SecretsConfig{
		EnableFileScanning: config.EnableFileScanning,
		MaxFileSize:        config.MaxFileSize,
		DisabledRules:      config.DisabledRules,
		AllowOverride:      config.AllowOverride,
	}

	return h.secretLinter.SetConfig(secretsConfig)
}
