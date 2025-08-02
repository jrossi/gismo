package handlers

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jrossi/gismo/pkg/engine"

	json "github.com/goccy/go-json"
)

// RegexHandler handles regex-based pattern matching for prompts and content
type RegexHandler struct {
	*engine.BaseActionHandler
	config *RegexConfig
}

// RegexConfig provides configuration for regex pattern matching
type RegexConfig struct {
	// PromptRules defines regex rules for user prompts
	PromptRules []RegexRule `json:"prompt_rules"`

	// ContentRules defines regex rules for file content
	ContentRules []RegexRule `json:"content_rules"`

	// CaseSensitive determines if pattern matching is case sensitive
	CaseSensitive bool `json:"case_sensitive"`
}

// RegexRule defines a regex pattern and associated action
type RegexRule struct {
	// Name is a descriptive name for this rule
	Name string `json:"name"`

	// Pattern is the regular expression to match
	Pattern string `json:"pattern"`

	// Action defines what to do when pattern matches: "block", "warn", "log", "modify"
	Action string `json:"action"`

	// Message to show when rule is triggered
	Message string `json:"message"`

	// Replacement text (only used for "modify" action)
	Replacement string `json:"replacement,omitempty"`

	// Description explains what this rule does
	Description string `json:"description"`

	// Enabled determines if this rule is active
	Enabled bool `json:"enabled"`
}

// DefaultRegexConfig returns default configuration
func DefaultRegexConfig() *RegexConfig {
	return &RegexConfig{
		PromptRules: []RegexRule{
			{
				Name:        "no-passwords",
				Pattern:     `password\s*[:=]\s*["\']?([^"\'\s]+)`,
				Action:      "warn",
				Message:     "Detected potential password in prompt - please verify this is safe to share",
				Description: "Warns when password-like patterns are detected in prompts",
				Enabled:     true,
			},
			{
				Name:        "no-api-keys",
				Pattern:     `(?i)(api[_-]?key|secret[_-]?key)\s*[:=]\s*["\']?([a-zA-Z0-9_-]{20,})`,
				Action:      "warn",
				Message:     "Detected potential API key in prompt - please verify this is safe to share",
				Description: "Warns when API key patterns are detected in prompts",
				Enabled:     true,
			},
		},
		ContentRules:  []RegexRule{},
		CaseSensitive: false,
	}
}

// NewRegexHandler creates a new regex handler
func NewRegexHandler() *RegexHandler {
	return NewRegexHandlerWithConfig(DefaultRegexConfig())
}

// NewRegexHandlerWithConfig creates a new regex handler with custom configuration
func NewRegexHandlerWithConfig(config *RegexConfig) *RegexHandler {
	if config == nil {
		config = DefaultRegexConfig()
	}

	return &RegexHandler{
		BaseActionHandler: engine.NewBaseActionHandler("regex", 150), // Medium priority
		config:            config,
	}
}

// ShouldHandle determines if this handler should process the event
func (h *RegexHandler) ShouldHandle(ctx context.Context, event engine.HookMessage) bool {
	switch event.(type) {
	case *engine.UserPromptSubmitMessage:
		return len(h.config.PromptRules) > 0
	case *engine.PreToolUseMessage:
		return len(h.config.ContentRules) > 0
	default:
		return false
	}
}

// HandleUserPromptSubmit processes user prompts with regex rules
func (h *RegexHandler) HandleUserPromptSubmit(ctx context.Context, msg *engine.UserPromptSubmitMessage) (*engine.HookResponse, error) {
	if len(h.config.PromptRules) == 0 {
		return &engine.HookResponse{Decision: "approve"}, nil
	}

	var warnings []string
	modifiedPrompt := msg.UserPrompt

	for _, rule := range h.config.PromptRules {
		if !rule.Enabled {
			continue
		}

		// Compile regex with case sensitivity option
		pattern := rule.Pattern
		if !h.config.CaseSensitive {
			pattern = "(?i)" + pattern
		}

		regex, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n> Regex handler warning: Invalid pattern in rule '%s': %v\n", rule.Name, err)
			continue
		}

		// Check if pattern matches
		if regex.MatchString(modifiedPrompt) {
			switch rule.Action {
			case "block":
				fmt.Fprintf(os.Stderr, "\n> Prompt pattern check:\n  - [regex]: Rule '%s' blocked prompt\n  - Pattern: %s\n  - Reason: %s\n",
					rule.Name, rule.Pattern, rule.Description)
				return &engine.HookResponse{
					Decision: "block",
					Reason:   rule.Message,
				}, nil

			case "warn":
				warning := fmt.Sprintf("Rule '%s': %s", rule.Name, rule.Message)
				warnings = append(warnings, warning)
				fmt.Fprintf(os.Stderr, "\n> Prompt pattern check:\n  - [regex]: Warning from rule '%s'\n  - Pattern: %s\n  - Message: %s\n",
					rule.Name, rule.Pattern, rule.Message)

			case "log":
				fmt.Fprintf(os.Stderr, "\n> Prompt pattern check:\n  - [regex]: Rule '%s' matched\n  - Pattern: %s\n  - Description: %s\n",
					rule.Name, rule.Pattern, rule.Description)

			case "modify":
				if rule.Replacement != "" {
					modifiedPrompt = regex.ReplaceAllString(modifiedPrompt, rule.Replacement)
					fmt.Fprintf(os.Stderr, "\n> Prompt pattern check:\n  - [regex]: Rule '%s' modified prompt\n  - Pattern: %s\n  - Applied replacement\n",
						rule.Name, rule.Pattern)
				}
			}
		}
	}

	// If we have warnings, include them in the response
	if len(warnings) > 0 {
		warningMsg := "Pattern warnings: " + strings.Join(warnings, "; ")
		return &engine.HookResponse{
			Decision: "approve",
			Message:  warningMsg,
		}, nil
	}

	return &engine.HookResponse{Decision: "approve"}, nil
}

// HandlePreToolUse processes file content with regex rules
func (h *RegexHandler) HandlePreToolUse(ctx context.Context, msg *engine.PreToolUseMessage) (*engine.HookResponse, error) {
	if len(h.config.ContentRules) == 0 {
		return &engine.HookResponse{Decision: "approve"}, nil
	}

	// Only process Write operations for now
	if msg.ToolName != "Write" {
		return &engine.HookResponse{Decision: "approve"}, nil
	}

	// Extract content
	content, err := h.extractContent(msg)
	if err != nil {
		// If we can't extract content, let other handlers deal with it
		return &engine.HookResponse{Decision: "approve"}, nil
	}

	var warnings []string

	for _, rule := range h.config.ContentRules {
		if !rule.Enabled {
			continue
		}

		// Compile regex with case sensitivity option
		pattern := rule.Pattern
		if !h.config.CaseSensitive {
			pattern = "(?i)" + pattern
		}

		regex, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n> Regex handler warning: Invalid pattern in rule '%s': %v\n", rule.Name, err)
			continue
		}

		// Check if pattern matches
		if regex.MatchString(content) {
			switch rule.Action {
			case "block":
				fmt.Fprintf(os.Stderr, "\n> Content pattern check:\n  - [regex]: Rule '%s' blocked content\n  - Pattern: %s\n  - Reason: %s\n",
					rule.Name, rule.Pattern, rule.Description)
				return &engine.HookResponse{
					Decision: "block",
					Reason:   rule.Message,
				}, nil

			case "warn":
				warning := fmt.Sprintf("Rule '%s': %s", rule.Name, rule.Message)
				warnings = append(warnings, warning)
				fmt.Fprintf(os.Stderr, "\n> Content pattern check:\n  - [regex]: Warning from rule '%s'\n  - Pattern: %s\n  - Message: %s\n",
					rule.Name, rule.Pattern, rule.Message)

			case "log":
				fmt.Fprintf(os.Stderr, "\n> Content pattern check:\n  - [regex]: Rule '%s' matched\n  - Pattern: %s\n  - Description: %s\n",
					rule.Name, rule.Pattern, rule.Description)
			}
		}
	}

	// If we have warnings, include them in the response
	if len(warnings) > 0 {
		warningMsg := "Content pattern warnings: " + strings.Join(warnings, "; ")
		return &engine.HookResponse{
			Decision: "approve",
			Message:  warningMsg,
		}, nil
	}

	return &engine.HookResponse{Decision: "approve"}, nil
}

// extractContent extracts content from PreToolUse message
func (h *RegexHandler) extractContent(msg *engine.PreToolUseMessage) (string, error) {
	contentRaw, exists := msg.ToolInput["content"]
	if !exists {
		return "", fmt.Errorf("no content in tool input")
	}

	var content string
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		return "", fmt.Errorf("failed to unmarshal content: %w", err)
	}

	return content, nil
}

// SetConfig updates the handler configuration
func (h *RegexHandler) SetConfig(config *RegexConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	h.config = config
	return nil
}

// AddPromptRule adds a new prompt regex rule
func (h *RegexHandler) AddPromptRule(rule RegexRule) {
	h.config.PromptRules = append(h.config.PromptRules, rule)
}

// AddContentRule adds a new content regex rule
func (h *RegexHandler) AddContentRule(rule RegexRule) {
	h.config.ContentRules = append(h.config.ContentRules, rule)
}

// RemovePromptRule removes a prompt rule by name
func (h *RegexHandler) RemovePromptRule(name string) {
	var filtered []RegexRule
	for _, rule := range h.config.PromptRules {
		if rule.Name != name {
			filtered = append(filtered, rule)
		}
	}
	h.config.PromptRules = filtered
}

// RemoveContentRule removes a content rule by name
func (h *RegexHandler) RemoveContentRule(name string) {
	var filtered []RegexRule
	for _, rule := range h.config.ContentRules {
		if rule.Name != name {
			filtered = append(filtered, rule)
		}
	}
	h.config.ContentRules = filtered
}

// EnableRule enables a rule by name
func (h *RegexHandler) EnableRule(name string) {
	for i := range h.config.PromptRules {
		if h.config.PromptRules[i].Name == name {
			h.config.PromptRules[i].Enabled = true
			return
		}
	}
	for i := range h.config.ContentRules {
		if h.config.ContentRules[i].Name == name {
			h.config.ContentRules[i].Enabled = true
			return
		}
	}
}

// DisableRule disables a rule by name
func (h *RegexHandler) DisableRule(name string) {
	for i := range h.config.PromptRules {
		if h.config.PromptRules[i].Name == name {
			h.config.PromptRules[i].Enabled = false
			return
		}
	}
	for i := range h.config.ContentRules {
		if h.config.ContentRules[i].Name == name {
			h.config.ContentRules[i].Enabled = false
			return
		}
	}
}
