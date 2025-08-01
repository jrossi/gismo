package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo"
)

// FileAccessHandler handles file access control for read/write operations
type FileAccessHandler struct {
	*gismo.BaseActionHandler
	config *FileAccessConfig
}

// FileAccessConfig provides configuration for file access control
type FileAccessConfig struct {
	// ReadRestictions defines patterns for files that cannot be read
	ReadRestrictions []FileRestriction `json:"read_restrictions"`

	// WriteRestrictions defines patterns for files that cannot be written
	WriteRestrictions []FileRestriction `json:"write_restrictions"`

	// BlockOnViolation determines if violations should block operations
	BlockOnViolation bool `json:"block_on_violation"`
}

// FileRestriction defines a file access restriction
type FileRestriction struct {
	// Pattern is a regex pattern for file paths to restrict
	Pattern string `json:"pattern"`

	// Description explains why this restriction exists
	Description string `json:"description"`

	// Action defines what to do: "block", "warn", "log"
	Action string `json:"action"`

	// Message to show when restriction is triggered
	Message string `json:"message"`
}

// DefaultFileAccessConfig returns default configuration
func DefaultFileAccessConfig() *FileAccessConfig {
	return &FileAccessConfig{
		ReadRestrictions: []FileRestriction{
			{
				Pattern:     `\.pem$`,
				Description: "Block reading of PEM certificate files to protect secrets",
				Action:      "block",
				Message:     "Reading PEM certificate files is not allowed for security reasons",
			},
			{
				Pattern:     `\.key$`,
				Description: "Block reading of private key files",
				Action:      "block",
				Message:     "Reading private key files is not allowed for security reasons",
			},
			{
				Pattern:     `\.p12$|\.pfx$`,
				Description: "Block reading of certificate bundle files",
				Action:      "block",
				Message:     "Reading certificate bundle files is not allowed for security reasons",
			},
		},
		WriteRestrictions: []FileRestriction{
			{
				Pattern:     `^/etc/`,
				Description: "Block writing to system configuration directories",
				Action:      "block",
				Message:     "Writing to system directories is not allowed",
			},
			{
				Pattern:     `^/usr/`,
				Description: "Block writing to system directories",
				Action:      "block",
				Message:     "Writing to system directories is not allowed",
			},
		},
		BlockOnViolation: true,
	}
}

// NewFileAccessHandler creates a new file access handler
func NewFileAccessHandler() *FileAccessHandler {
	return NewFileAccessHandlerWithConfig(DefaultFileAccessConfig())
}

// NewFileAccessHandlerWithConfig creates a new file access handler with custom configuration
func NewFileAccessHandlerWithConfig(config *FileAccessConfig) *FileAccessHandler {
	if config == nil {
		config = DefaultFileAccessConfig()
	}

	return &FileAccessHandler{
		BaseActionHandler: gismo.NewBaseActionHandler("file-access", 300), // Highest priority for security
		config:            config,
	}
}

// ShouldHandle determines if this handler should process file access events
func (h *FileAccessHandler) ShouldHandle(ctx context.Context, event gismo.HookMessage) bool {
	switch msg := event.(type) {
	case *gismo.PreToolUseMessage:
		return msg.ToolName == "Read" || msg.ToolName == "Write" || msg.ToolName == "Edit" || msg.ToolName == "MultiEdit"
	default:
		return false
	}
}

// HandlePreToolUse checks file access permissions before tool execution
func (h *FileAccessHandler) HandlePreToolUse(ctx context.Context, msg *gismo.PreToolUseMessage) (*gismo.HookResponse, error) {
	// Extract file path from tool input
	filePath, err := h.extractFilePath(msg)
	if err != nil {
		// If we can't extract file path, let other handlers deal with it
		return &gismo.HookResponse{Decision: "approve"}, nil
	}

	// Normalize the file path
	filePath = h.normalizePath(filePath)

	// Check restrictions based on tool type
	switch msg.ToolName {
	case "Read":
		return h.checkReadRestrictions(filePath)
	case "Write", "Edit", "MultiEdit":
		return h.checkWriteRestrictions(filePath)
	default:
		return &gismo.HookResponse{Decision: "approve"}, nil
	}
}

// checkReadRestrictions checks if a file can be read
func (h *FileAccessHandler) checkReadRestrictions(filePath string) (*gismo.HookResponse, error) {
	for _, restriction := range h.config.ReadRestrictions {
		if matched, _ := regexp.MatchString(restriction.Pattern, filePath); matched {
			message := restriction.Message
			if message == "" {
				message = fmt.Sprintf("File access restricted: %s", restriction.Description)
			}

			// Log the restriction
			fmt.Fprintf(os.Stderr, "\n> File access control:\n  - [file-access]: Read blocked for %s\n  - Reason: %s\n", filePath, restriction.Description)

			switch restriction.Action {
			case "block":
				if h.config.BlockOnViolation {
					return &gismo.HookResponse{
						Decision: "block",
						Reason:   message,
					}, nil
				}
			case "warn":
				return &gismo.HookResponse{
					Decision: "approve",
					Message:  fmt.Sprintf("Warning: %s", message),
				}, nil
			case "log":
				// Just log, continue
				break
			}
		}
	}

	return &gismo.HookResponse{Decision: "approve"}, nil
}

// checkWriteRestrictions checks if a file can be written
func (h *FileAccessHandler) checkWriteRestrictions(filePath string) (*gismo.HookResponse, error) {
	for _, restriction := range h.config.WriteRestrictions {
		if matched, _ := regexp.MatchString(restriction.Pattern, filePath); matched {
			message := restriction.Message
			if message == "" {
				message = fmt.Sprintf("File access restricted: %s", restriction.Description)
			}

			// Log the restriction
			fmt.Fprintf(os.Stderr, "\n> File access control:\n  - [file-access]: Write blocked for %s\n  - Reason: %s\n", filePath, restriction.Description)

			switch restriction.Action {
			case "block":
				if h.config.BlockOnViolation {
					return &gismo.HookResponse{
						Decision: "block",
						Reason:   message,
					}, nil
				}
			case "warn":
				return &gismo.HookResponse{
					Decision: "approve",
					Message:  fmt.Sprintf("Warning: %s", message),
				}, nil
			case "log":
				// Just log, continue
				break
			}
		}
	}

	return &gismo.HookResponse{Decision: "approve"}, nil
}

// extractFilePath extracts file path from PreToolUse message
func (h *FileAccessHandler) extractFilePath(msg *gismo.PreToolUseMessage) (string, error) {
	filePathRaw, exists := msg.ToolInput["file_path"]
	if !exists {
		return "", fmt.Errorf("no file_path in tool input")
	}

	var filePath string
	if err := json.Unmarshal(filePathRaw, &filePath); err != nil {
		return "", fmt.Errorf("failed to unmarshal file_path: %w", err)
	}

	return filePath, nil
}

// normalizePath normalizes a file path for consistent matching
func (h *FileAccessHandler) normalizePath(path string) string {
	// Convert to absolute path if possible
	if absPath, err := filepath.Abs(path); err == nil {
		path = absPath
	}

	// Clean the path
	path = filepath.Clean(path)

	// Convert to forward slashes for consistent regex matching
	path = strings.ReplaceAll(path, "\\", "/")

	return path
}

// SetConfig updates the handler configuration
func (h *FileAccessHandler) SetConfig(config *FileAccessConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	h.config = config
	return nil
}

// AddReadRestriction adds a new read restriction
func (h *FileAccessHandler) AddReadRestriction(restriction FileRestriction) {
	h.config.ReadRestrictions = append(h.config.ReadRestrictions, restriction)
}

// AddWriteRestriction adds a new write restriction
func (h *FileAccessHandler) AddWriteRestriction(restriction FileRestriction) {
	h.config.WriteRestrictions = append(h.config.WriteRestrictions, restriction)
}

// RemoveReadRestriction removes a read restriction by pattern
func (h *FileAccessHandler) RemoveReadRestriction(pattern string) {
	var filtered []FileRestriction
	for _, restriction := range h.config.ReadRestrictions {
		if restriction.Pattern != pattern {
			filtered = append(filtered, restriction)
		}
	}
	h.config.ReadRestrictions = filtered
}

// RemoveWriteRestriction removes a write restriction by pattern
func (h *FileAccessHandler) RemoveWriteRestriction(pattern string) {
	var filtered []FileRestriction
	for _, restriction := range h.config.WriteRestrictions {
		if restriction.Pattern != pattern {
			filtered = append(filtered, restriction)
		}
	}
	h.config.WriteRestrictions = filtered
}
