package secrets

import (
	"context"
	"testing"
)

func TestSecretLinter_Name(t *testing.T) {
	linter := NewSecretLinter()
	if linter.Name() != "secrets" {
		t.Errorf("Expected name 'secrets', got %s", linter.Name())
	}
}

func TestSecretLinter_CanHandle(t *testing.T) {
	tests := []struct {
		name               string
		enableFileScanning bool
		filePath           string
		expectedCanHandle  bool
	}{
		{
			name:               "File scanning enabled",
			enableFileScanning: true,
			filePath:           "test.go",
			expectedCanHandle:  true,
		},
		{
			name:               "File scanning disabled",
			enableFileScanning: false,
			filePath:           "test.go",
			expectedCanHandle:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultSecretsConfig()
			config.EnableFileScanning = tt.enableFileScanning
			linter := NewSecretLinterWithConfig(config)

			if linter.CanHandle(tt.filePath) != tt.expectedCanHandle {
				t.Errorf("Expected CanHandle to return %v, got %v", tt.expectedCanHandle, linter.CanHandle(tt.filePath))
			}
		})
	}
}

func TestSecretLinter_Lint(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectIssues bool
		expectRuleID string
	}{
		{
			name:         "Clean content",
			content:      "This is a normal file with no secrets",
			expectIssues: false,
		},
		{
			name:         "AWS-like key",
			content:      "aws_access_key = AKIA1234567890123456",
			expectIssues: true,
			expectRuleID: "generic-api-key",
		},
		{
			name:         "Another AWS key pattern",
			content:      "const apiKey = \"AKIA0987654321098765\"",
			expectIssues: true,
			expectRuleID: "generic-api-key",
		},
	}

	linter := NewSecretLinter()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := linter.Lint(ctx, "test.txt", []byte(tt.content))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			hasIssues := len(result.Issues) > 0
			if hasIssues != tt.expectIssues {
				t.Errorf("Expected issues: %v, got issues: %v (%d issues found)", tt.expectIssues, hasIssues, len(result.Issues))
			}

			if tt.expectIssues && len(result.Issues) > 0 {
				found := false
				for _, issue := range result.Issues {
					if issue.Rule == tt.expectRuleID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find rule %s in issues", tt.expectRuleID)
				}
			}
		})
	}
}

func TestSecretLinter_ScanPrompt(t *testing.T) {
	tests := []struct {
		name          string
		prompt        string
		expectBlocked bool
	}{
		{
			name:          "Clean prompt",
			prompt:        "Help me write a Go function",
			expectBlocked: false,
		},
		{
			name:          "Prompt with secret",
			prompt:        "Use this key: AKIA1234567890123456 for AWS",
			expectBlocked: true,
		},
		{
			name:          "Prompt with override",
			prompt:        "Use this key: AKIA1234567890123456 secret_detect=override",
			expectBlocked: false,
		},
		{
			name:          "Override at beginning",
			prompt:        "secret_detect=override Use this key: AKIA1234567890123456",
			expectBlocked: false,
		},
	}

	linter := NewSecretLinter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := linter.ScanPrompt(tt.prompt)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			hasIssues := len(result.Issues) > 0
			if hasIssues != tt.expectBlocked {
				t.Errorf("Expected blocked: %v, got issues: %v (%d issues found)", tt.expectBlocked, hasIssues, len(result.Issues))
			}
		})
	}
}

func TestSecretLinter_FileSizeLimit(t *testing.T) {
	config := DefaultSecretsConfig()
	config.MaxFileSize = 10 // Very small limit for testing
	linter := NewSecretLinterWithConfig(config)

	largeContent := make([]byte, 20) // Larger than limit
	for i := range largeContent {
		largeContent[i] = 'a'
	}

	result, err := linter.Lint(context.Background(), "test.txt", largeContent)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("Expected 1 issue for file size, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.Rule != "file-too-large" {
		t.Errorf("Expected rule 'file-too-large', got %s", issue.Rule)
	}
	if issue.Severity != "warning" {
		t.Errorf("Expected severity 'warning', got %s", issue.Severity)
	}
}

func TestSecretLinter_DisabledRules(t *testing.T) {
	config := DefaultSecretsConfig()
	config.DisabledRules = []string{"generic-api-key"}
	linter := NewSecretLinterWithConfig(config)

	// This content would normally trigger generic-api-key rule
	content := "aws_access_key = AKIA1234567890123456"
	result, err := linter.Lint(context.Background(), "test.txt", []byte(content))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have no issues since the rule is disabled
	if len(result.Issues) != 0 {
		t.Errorf("Expected no issues with disabled rule, got %d issues", len(result.Issues))
	}
}

func TestSecretLinter_OverrideDisabled(t *testing.T) {
	config := DefaultSecretsConfig()
	config.AllowOverride = false
	linter := NewSecretLinterWithConfig(config)

	// Prompt with secret and override - should still be blocked
	prompt := "Use this key: AKIA1234567890123456 secret_detect=override"
	result, err := linter.ScanPrompt(prompt)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have issues since override is disabled
	if len(result.Issues) == 0 {
		t.Error("Expected issues when override is disabled, got none")
	}
}

func TestDefaultSecretsConfig(t *testing.T) {
	config := DefaultSecretsConfig()

	if !config.EnableFileScanning {
		t.Error("Expected EnableFileScanning to be true by default")
	}
	if config.MaxFileSize != 1024*1024 {
		t.Errorf("Expected MaxFileSize to be 1MB, got %d", config.MaxFileSize)
	}
	if !config.AllowOverride {
		t.Error("Expected AllowOverride to be true by default")
	}
	if len(config.DisabledRules) != 0 {
		t.Errorf("Expected no disabled rules by default, got %d", len(config.DisabledRules))
	}
}

func TestSecretLinter_SetConfig(t *testing.T) {
	linter := NewSecretLinter()

	// Test setting nil config
	err := linter.SetConfig(nil)
	if err == nil {
		t.Error("Expected error when setting nil config")
	}

	// Test setting valid config
	newConfig := &SecretsConfig{
		EnableFileScanning: false,
		MaxFileSize:        512,
		AllowOverride:      false,
	}
	err = linter.SetConfig(newConfig)
	if err != nil {
		t.Errorf("Unexpected error setting config: %v", err)
	}

	if linter.config.EnableFileScanning {
		t.Error("Expected EnableFileScanning to be false after config update")
	}
	if linter.config.MaxFileSize != 512 {
		t.Errorf("Expected MaxFileSize to be 512 after config update, got %d", linter.config.MaxFileSize)
	}
}
