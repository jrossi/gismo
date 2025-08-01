package secrets

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jrossi/gismo/linters"
	"github.com/zricethezav/gitleaks/v8/detect"
)

// SecretLinter implements the Linter interface for secret detection
type SecretLinter struct {
	detector *detect.Detector
	config   *SecretsConfig
}

// SecretsConfig provides configuration options for the secrets linter
type SecretsConfig struct {
	// EnableFileScanning enables scanning of file contents
	EnableFileScanning bool `json:"enable_file_scanning"`
	// MaxFileSize sets the maximum file size to scan (in bytes)
	MaxFileSize int `json:"max_file_size"`
	// DisabledRules lists rule IDs to disable
	DisabledRules []string `json:"disabled_rules"`
	// AllowOverride enables the secret_detect=override pattern for prompts
	AllowOverride bool `json:"allow_override"`
}

// DefaultSecretsConfig returns default configuration
func DefaultSecretsConfig() *SecretsConfig {
	return &SecretsConfig{
		EnableFileScanning: true,
		MaxFileSize:        1024 * 1024, // 1MB
		DisabledRules:      []string{},
		AllowOverride:      true,
	}
}

// NewSecretLinter creates a new secret linter with default configuration
func NewSecretLinter() *SecretLinter {
	return NewSecretLinterWithConfig(DefaultSecretsConfig())
}

// NewSecretLinterWithConfig creates a new secret linter with custom configuration
func NewSecretLinterWithConfig(config *SecretsConfig) *SecretLinter {
	if config == nil {
		config = DefaultSecretsConfig()
	}

	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		// If we can't initialize detector, create a stub that will return empty results
		detector = nil
	}

	return &SecretLinter{
		detector: detector,
		config:   config,
	}
}

// Name returns the linter name
func (s *SecretLinter) Name() string {
	return "secrets"
}

// CanHandle returns true for all file types if file scanning is enabled
func (s *SecretLinter) CanHandle(filePath string) bool {
	return s.config.EnableFileScanning
}

// Lint scans the content for secrets
func (s *SecretLinter) Lint(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	// Check file size limit
	if len(content) > s.config.MaxFileSize {
		return &linters.LintResult{
			Issues: []linters.Issue{
				{
					Line:     1,
					Column:   1,
					Rule:     "file-too-large",
					Severity: "warning",
					Message:  fmt.Sprintf("File too large for secret scanning (%d bytes, max %d)", len(content), s.config.MaxFileSize),
				},
			},
		}, nil
	}

	return s.scanContent(string(content))
}

// ScanPrompt scans a user prompt for secrets with override support
func (s *SecretLinter) ScanPrompt(prompt string) (*linters.LintResult, error) {
	// Check for override pattern if enabled
	if s.config.AllowOverride {
		if matched, _ := regexp.MatchString(`secret_detect=override`, prompt); matched {
			return &linters.LintResult{Issues: []linters.Issue{}}, nil
		}
	}

	return s.scanContent(prompt)
}

// scanContent performs the actual secret detection
func (s *SecretLinter) scanContent(content string) (*linters.LintResult, error) {
	if s.detector == nil {
		return &linters.LintResult{
			Issues: []linters.Issue{
				{
					Line:     1,
					Column:   1,
					Rule:     "detector-unavailable",
					Severity: "warning",
					Message:  "Secret detector unavailable - skipping scan",
				},
			},
		}, nil
	}

	// Scan for secrets
	findings := s.detector.DetectString(content)
	if len(findings) == 0 {
		return &linters.LintResult{Issues: []linters.Issue{}}, nil
	}

	// Convert gitleaks findings to linter issues
	var issues []linters.Issue
	for _, finding := range findings {
		// Skip disabled rules
		if s.isRuleDisabled(finding.RuleID) {
			continue
		}

		issues = append(issues, linters.Issue{
			Line:     finding.StartLine,
			Column:   finding.StartColumn,
			Rule:     finding.RuleID,
			Severity: "error", // Treat all secrets as errors
			Message:  fmt.Sprintf("Potential secret detected: %s", finding.Description),
		})
	}

	return &linters.LintResult{Issues: issues}, nil
}

// isRuleDisabled checks if a rule ID is in the disabled list
func (s *SecretLinter) isRuleDisabled(ruleID string) bool {
	for _, disabled := range s.config.DisabledRules {
		if disabled == ruleID {
			return true
		}
		// Support wildcard matching
		if strings.Contains(disabled, "*") {
			if matched, _ := regexp.MatchString(strings.ReplaceAll(disabled, "*", ".*"), ruleID); matched {
				return true
			}
		}
	}
	return false
}

// SetConfig updates the linter configuration
func (s *SecretLinter) SetConfig(config *SecretsConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	s.config = config
	return nil
}
