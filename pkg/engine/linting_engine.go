package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	linters2 "github.com/jrossi/gismo/pkg/linters"
	"github.com/jrossi/gismo/pkg/linters/golang"
	"github.com/jrossi/gismo/pkg/linters/javascript"
	jsonlinter "github.com/jrossi/gismo/pkg/linters/json"
	"github.com/jrossi/gismo/pkg/linters/markdown"
	"github.com/jrossi/gismo/pkg/linters/protobuf"
	"github.com/jrossi/gismo/pkg/linters/python"
	"github.com/jrossi/gismo/pkg/linters/rust"
	"github.com/jrossi/gismo/pkg/linters/secrets"
)

// LintingRuleEngine implements RuleEngine to provide linting functionality
type LintingRuleEngine struct {
	linters  []linters2.Linter
	executor *linters2.ParallelExecutor
	config   *AppConfig
}

// LintingConfig provides configuration options for the linting engine
type LintingConfig struct {
	// MaxWorkers sets the maximum number of concurrent workers
	// If 0 or negative, defaults to runtime.NumCPU()
	MaxWorkers int
	// DisableParallel disables parallel execution for debugging
	DisableParallel bool
}

// NewLintingRuleEngine creates a new linting rule engine with default linters
func NewLintingRuleEngine() *LintingRuleEngine {
	return NewLintingRuleEngineWithConfig(LintingConfig{})
}

// NewLintingRuleEngineWithConfig creates a new linting rule engine with custom configuration
func NewLintingRuleEngineWithConfig(config LintingConfig) *LintingRuleEngine {
	maxWorkers := config.MaxWorkers
	if config.DisableParallel {
		maxWorkers = 1
	}

	engine := &LintingRuleEngine{
		linters:  []linters2.Linter{},
		executor: linters2.NewParallelExecutor(maxWorkers),
		config:   NewAppConfig(),
	}

	// Initialize linters with empty configs for now
	// We'll update them when SetAppConfig is called
	engine.linters = append(engine.linters, golang.NewGoLinter())
	engine.linters = append(engine.linters, javascript.NewJavaScriptLinter())
	engine.linters = append(engine.linters, jsonlinter.NewJSONLinter())
	engine.linters = append(engine.linters, markdown.NewMarkdownLinter())
	engine.linters = append(engine.linters, protobuf.NewProtobufLinter())
	engine.linters = append(engine.linters, python.NewPythonLinter())
	engine.linters = append(engine.linters, rust.NewRustLinter())
	// Note: secrets linter is only added when explicitly enabled in config

	return engine
}

// AddLinter adds a custom linter to the engine
func (e *LintingRuleEngine) AddLinter(linter linters2.Linter) {
	e.linters = append(e.linters, linter)
}

// SetAppConfig sets the application configuration
func (e *LintingRuleEngine) SetAppConfig(config *AppConfig) {
	e.config = config

	// Add secrets linter only if explicitly enabled (opt-in only)
	if config != nil && e.isSecretsLinterExplicitlyEnabled(config) {
		// Check if secrets linter is already present
		hasSecretsLinter := false
		for _, linter := range e.linters {
			if linter.Name() == "secrets" {
				hasSecretsLinter = true
				break
			}
		}
		// Add secrets linter if not already present
		if !hasSecretsLinter {
			e.linters = append(e.linters, secrets.NewSecretLinter())
		}
	}

	// Update linter configurations
	if config != nil {
		for _, linter := range e.linters {
			// Check if this linter is disabled
			if !config.IsLinterEnabled(linter.Name()) {
				continue
			}

			// Get linter-specific configuration
			if linterConfig, ok := config.GetLinterConfig(linter.Name()); ok {
				// Try to cast to configurable linter
				if configurable, ok := linter.(ConfigurableLinter); ok {
					if err := configurable.SetConfig(linterConfig); err != nil {
						// Log error but continue
						fmt.Fprintf(os.Stderr, "Warning: Failed to configure %s linter: %v\n", linter.Name(), err)
					}
				}
			}
		}
	}
}

// GetAppConfig returns the application configuration
func (e *LintingRuleEngine) GetAppConfig() *AppConfig {
	return e.config
}

// isSecretsLinterExplicitlyEnabled checks if the secrets linter is explicitly enabled in config
// Unlike IsLinterEnabled, this method requires explicit opt-in and doesn't default to enabled
func (e *LintingRuleEngine) isSecretsLinterExplicitlyEnabled(config *AppConfig) bool {
	if config == nil || config.Linters == nil {
		return false // secrets linter is opt-in only
	}
	linterConfig, ok := config.Linters["secrets"]
	if !ok {
		return false // no config means not enabled
	}
	if linterConfig.Enabled == nil {
		return false // no explicit enabled flag means not enabled for secrets
	}
	return *linterConfig.Enabled
}

// ConfigurableLinter is an interface for linters that support runtime configuration
type ConfigurableLinter interface {
	linters2.Linter
	SetConfig(config json.RawMessage) error
}

// applyRuleOverrides applies any rule overrides for the given file path
func (e *LintingRuleEngine) applyRuleOverrides(filePath string) {
	if e.config == nil {
		return
	}

	// Apply overrides for each linter
	for _, linter := range e.linters {
		// Get any rule overrides for this file and linter
		overrides := e.config.GetRuleOverrides(filePath, linter.Name())
		if len(overrides) == 0 {
			continue
		}

		// Try to cast to configurable linter
		if configurable, ok := linter.(ConfigurableLinter); ok {
			// Merge all overrides into a single config
			mergedConfig := make(map[string]interface{})

			for _, override := range overrides {
				var overrideMap map[string]interface{}
				if err := json.Unmarshal(override, &overrideMap); err != nil {
					continue
				}

				// Merge this override into the merged config
				for k, v := range overrideMap {
					mergedConfig[k] = v
				}
			}

			// Convert back to JSON and apply
			if configData, err := json.Marshal(mergedConfig); err == nil {
				if err := configurable.SetConfig(configData); err != nil {
					// Log error but continue
					fmt.Fprintf(os.Stderr, "Warning: Failed to apply rule override for %s linter: %v\n", linter.Name(), err)
				}
			}
		}
	}
}

// EvaluatePreToolUse checks files before they're written
func (e *LintingRuleEngine) EvaluatePreToolUse(ctx context.Context, msg *PreToolUseMessage) (*HookResponse, error) {
	// Only check Write and Edit operations
	if msg.ToolName != "Write" && msg.ToolName != "Edit" && msg.ToolName != "MultiEdit" {
		return &HookResponse{Decision: "approve"}, nil
	}

	// Extract file path from tool input
	filePathRaw, exists := msg.ToolInput["file_path"]
	if !exists {
		return &HookResponse{Decision: "approve"}, nil
	}
	var filePath string
	if err := json.Unmarshal(filePathRaw, &filePath); err != nil {
		return &HookResponse{Decision: "approve"}, nil
	}

	// Skip temporary test files to avoid linting noise during tests
	if isTemporaryTestFile(filePath) {
		return &HookResponse{Decision: "approve"}, nil
	}

	// For Edit/MultiEdit, we can't lint until after the edit is done
	if msg.ToolName == "Edit" || msg.ToolName == "MultiEdit" {
		return &HookResponse{Decision: "approve"}, nil
	}

	// For Write operations, check the content
	contentRaw, exists := msg.ToolInput["content"]
	if !exists {
		return &HookResponse{Decision: "approve"}, nil
	}
	var content string
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		return &HookResponse{Decision: "approve"}, nil
	}

	// Apply rule overrides for this file
	e.applyRuleOverrides(filePath)

	// Run all applicable linters in parallel
	results := e.executor.ExecuteLinters(ctx, e.linters, filePath, []byte(content))

	// Aggregate results
	aggregatedResult, errs := linters2.AggregateResults(results)

	// Handle any linting errors
	if len(errs) > 0 {
		return &HookResponse{
			Decision: "block",
			Reason:   fmt.Sprintf("Linting error: %v", errs[0]),
		}, nil
	}

	// Check for issues and format detailed output
	var errorIssues, warningIssues []linters2.Issue
	for _, issue := range aggregatedResult.Issues {
		if issue.Severity == "error" {
			errorIssues = append(errorIssues, issue)
		} else {
			warningIssues = append(warningIssues, issue)
		}
	}

	// If there are syntax errors, block the write
	if len(errorIssues) > 0 {
		output := e.formatLintOutput(filePath, errorIssues, true)
		// Write detailed output to stderr for user visibility
		fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n%s\n", output)
		return &HookResponse{
			Decision: "block",
			Reason:   fmt.Sprintf("Found %d error(s) in %s", len(errorIssues), filePath),
		}, nil
	}

	// If formatting is needed, inform but don't block
	if len(warningIssues) > 0 {
		output := e.formatLintOutput(filePath, warningIssues, false)
		// Write detailed output to stderr for user visibility
		fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n%s\n", output)
		return &HookResponse{
			Decision: "approve",
			Message:  fmt.Sprintf("Found %d warning(s) in %s", len(warningIssues), filePath),
		}, nil
	}

	// Write success message to stderr (matching smart-lint.sh behavior)
	fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n  - [gismo]: ✅ Style clean. Continue with your task.\n")
	return &HookResponse{Decision: "approve"}, nil
}

// EvaluatePostToolUse runs linters and tests after file operations
func (e *LintingRuleEngine) EvaluatePostToolUse(ctx context.Context, msg *PostToolUseMessage) (*HookResponse, error) {
	// Only check Write and Edit operations
	if msg.ToolName != "Write" && msg.ToolName != "Edit" && msg.ToolName != "MultiEdit" {
		// Show status for non-file operations on stderr (matching smart-lint.sh behavior)
		fmt.Fprintf(os.Stderr, "\n> Tool execution feedback:\n  - [gismo]: ℹ️  %s operation completed (no linting required)\n", msg.ToolName)
		return nil, nil
	}

	// Skip if there was an error
	if msg.ToolError != "" {
		// Tool errors trigger exit code 1, shown on stderr
		fmt.Fprintf(os.Stderr, "\n> Tool execution feedback:\n  - [gismo]: ⚠️  Tool error: %s (skipping linting)\n", msg.ToolError)
		return nil, nil
	}

	// Extract file path from tool input
	filePathRaw, exists := msg.ToolInput["file_path"]
	if !exists {
		return nil, nil
	}
	var filePath string
	if err := json.Unmarshal(filePathRaw, &filePath); err != nil || filePath == "" {
		return nil, nil
	}

	// Skip temporary test files to avoid linting noise during tests
	if isTemporaryTestFile(filePath) {
		return nil, nil
	}

	// Read the actual file from disk
	content, err := os.ReadFile(filePath)
	if err != nil {
		// File errors shown on stderr (matching smart-lint.sh behavior)
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n  - [gismo]: ⚠️  File not found: %s\n", filePath)
		} else {
			fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n  - [gismo]: ⚠️  Cannot read file: %v\n", err)
		}
		return nil, nil
	}

	// Apply rule overrides for this file
	e.applyRuleOverrides(filePath)

	// Run all applicable linters in parallel
	results := e.executor.ExecuteLinters(ctx, e.linters, filePath, content)

	// Aggregate results
	aggregatedResult, errs := linters2.AggregateResults(results)

	// Handle any linting errors
	for _, err := range errs {
		// Linting errors trigger exit code 1, shown on stderr
		fmt.Fprintf(os.Stderr, "\n> Linting error for %s: %v\n", filePath, err)
	}

	// Check for issues and format detailed output
	var errorIssues, warningIssues []linters2.Issue
	for _, issue := range aggregatedResult.Issues {
		if issue.Severity == "error" {
			errorIssues = append(errorIssues, issue)
		} else {
			warningIssues = append(warningIssues, issue)
		}
	}

	// Issues trigger exit code 1, shown on stderr
	if len(errorIssues) > 0 {
		output := e.formatLintOutput(filePath, errorIssues, true)
		fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n%s\n", output)
	} else if len(warningIssues) > 0 {
		output := e.formatLintOutput(filePath, warningIssues, false)
		fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n%s\n", output)
	} else if len(errs) == 0 {
		// Success shown on stderr (matching smart-lint.sh behavior)
		fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n  - [gismo]: ✅ Style clean. Continue with your task.\n")
	}

	// Check for associated test files if it's a Go file
	if strings.HasSuffix(filePath, ".go") && !strings.HasSuffix(filePath, "_test.go") {
		e.checkTestFile(ctx, filePath)
	}

	// Always return nil for PostToolUse to avoid JSON output interfering with stderr
	// The exit code is controlled by executor.go based on IsPostToolUseHook()
	return nil, nil
}

// EvaluateNotification handles system notifications
func (e *LintingRuleEngine) EvaluateNotification(ctx context.Context, msg *NotificationMessage) (*HookResponse, error) {
	return nil, nil
}

// EvaluateStop handles main agent completion
func (e *LintingRuleEngine) EvaluateStop(ctx context.Context, msg *StopMessage) (*HookResponse, error) {
	return nil, nil
}

// EvaluateSubagentStop handles subagent completion
func (e *LintingRuleEngine) EvaluateSubagentStop(ctx context.Context, msg *SubagentStopMessage) (*HookResponse, error) {
	return nil, nil
}

// EvaluatePreCompact handles pre-compact events
func (e *LintingRuleEngine) EvaluatePreCompact(ctx context.Context, msg *PreCompactMessage) (*HookResponse, error) {
	return nil, nil
}

// EvaluateUserPromptSubmit handles user prompt submit events
func (e *LintingRuleEngine) EvaluateUserPromptSubmit(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error) {
	return e.detectSecretsInPrompt(ctx, msg)
}

// detectSecretsInPrompt scans user prompt for secrets using the secret linter
func (e *LintingRuleEngine) detectSecretsInPrompt(ctx context.Context, msg *UserPromptSubmitMessage) (*HookResponse, error) {
	// Find the secrets linter in our linters list
	var secretLinter *secrets.SecretLinter
	for _, linter := range e.linters {
		if sl, ok := linter.(*secrets.SecretLinter); ok {
			secretLinter = sl
			break
		}
	}

	if secretLinter == nil {
		// No secret linter configured, approve by default
		return &HookResponse{Decision: "approve"}, nil
	}

	// Scan the prompt using the secret linter
	result, err := secretLinter.ScanPrompt(msg.UserPrompt)
	if err != nil {
		// If scanning fails, log but don't block
		fmt.Fprintf(os.Stderr, "\n> Secret detection warning: %v\n", err)
		return &HookResponse{Decision: "approve"}, nil
	}

	if len(result.Issues) == 0 {
		return &HookResponse{Decision: "approve"}, nil
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
	details.WriteString("\n\nTo override this check, include 'secret_detect=override' in your prompt.")

	// Write detailed feedback to stderr
	fmt.Fprintf(os.Stderr, "\n> Prompt security check:\n%s\n", details.String())

	return &HookResponse{
		Decision: "block",
		Reason:   fmt.Sprintf("Found %d potential secret(s) in prompt", len(result.Issues)),
	}, nil
}

// EvaluateSessionStart handles session start events
func (e *LintingRuleEngine) EvaluateSessionStart(ctx context.Context, msg *SessionStartMessage) (*HookResponse, error) {
	return nil, nil
}

// formatLintOutput formats linting issues in a style similar to smart-lint.sh
func (e *LintingRuleEngine) formatLintOutput(filePath string, issues []linters2.Issue, isBlocking bool) string {
	var output strings.Builder

	// Header similar to smart-lint.sh
	output.WriteString(fmt.Sprintf("- [ccfeedback:%s]: ", filePath))

	// Add details for each issue
	for i, issue := range issues {
		if i > 0 {
			output.WriteString("\n  ")
		}

		// Format: file:line:column: message
		if issue.Line > 0 && issue.Column > 0 {
			output.WriteString(fmt.Sprintf("%s:%d:%d: %s",
				strings.TrimPrefix(filePath, "/Users/jrossi/src/gismo/"),
				issue.Line, issue.Column, issue.Message))
		} else {
			output.WriteString(issue.Message)
		}

		if issue.Rule != "" {
			output.WriteString(fmt.Sprintf(" (%s)", issue.Rule))
		}
	}

	output.WriteString("\n")

	// Add footer similar to smart-lint.sh
	if isBlocking {
		issueCount := len(issues)
		output.WriteString(fmt.Sprintf("\n❌ Found %d blocking issue(s) - fix all above\n", issueCount))
		output.WriteString("⛔ BLOCKING: Must fix ALL errors above before continuing")
	} else {
		issueCount := len(issues)
		output.WriteString(fmt.Sprintf("\n⚠️  Found %d warning(s) - consider fixing\n", issueCount))
		output.WriteString("📝 NON-BLOCKING: Issues detected but you can continue")
	}

	return output.String()
}

// checkTestFile checks for an associated _test.go file and runs linting on it
func (e *LintingRuleEngine) checkTestFile(ctx context.Context, filePath string) {
	// Construct test file path
	base := strings.TrimSuffix(filePath, ".go")
	testPath := base + "_test.go"

	// Check if test file exists
	content, err := os.ReadFile(testPath)
	if err != nil {
		// No test file, that's ok
		return
	}

	// Run all applicable linters on test file in parallel
	results := e.executor.ExecuteLinters(ctx, e.linters, testPath, content)

	// Aggregate results
	aggregatedResult, errs := linters2.AggregateResults(results)

	// Handle any linting errors
	for _, err := range errs {
		// Test file linting errors trigger exit code 1, shown on stderr
		fmt.Fprintf(os.Stderr, "\n> Test file linting error for %s: %v\n", testPath, err)
	}

	// Report any issues found in test file
	if len(aggregatedResult.Issues) > 0 {
		var errorIssues, warningIssues []linters2.Issue
		for _, issue := range aggregatedResult.Issues {
			if issue.Severity == "error" {
				errorIssues = append(errorIssues, issue)
			} else {
				warningIssues = append(warningIssues, issue)
			}
		}

		// Test file issues trigger exit code 1, shown on stderr
		if len(errorIssues) > 0 {
			output := e.formatLintOutput(testPath, errorIssues, true)
			fmt.Fprintf(os.Stderr, "\n> Test file feedback:\n%s\n", output)
		} else if len(warningIssues) > 0 {
			output := e.formatLintOutput(testPath, warningIssues, false)
			fmt.Fprintf(os.Stderr, "\n> Test file feedback:\n%s\n", output)
		}
	}
}

// isTemporaryTestFile checks if a file path represents a temporary test file
// that should be excluded from linting during test execution
func isTemporaryTestFile(filePath string) bool {
	baseName := filepath.Base(filePath)

	// Only exclude very specific patterns that are clearly test fixtures
	// Be very conservative to avoid interfering with legitimate testing

	// Only exclude files that are clearly just the base name without any directory context
	// This helps distinguish between test fixtures and legitimate test scenarios
	if filePath == baseName {
		verySpecificPatterns := []string{
			"success_test.go",    // Temporary success test files
			"success_test.md",    // Temporary success test markdown files
			"large_test.go",      // Temporary large test files
			"large_test.md",      // Temporary large test markdown files
			"concurrent_test.go", // Temporary concurrent test files
			"concurrent_test.md", // Temporary concurrent test markdown files
		}

		// Check specific test fixture patterns
		for _, pattern := range verySpecificPatterns {
			if pattern == baseName {
				return true
			}
		}
	}

	// Special case: Exclude README.md files that are clearly test fixtures
	// (not the actual project README.md)
	if baseName == "README.md" {
		// Allow actual project README.md files, but exclude ones in test directories
		// or ones that are likely test fixtures based on path context
		if strings.Contains(filePath, "/testdata/") ||
			strings.Contains(filePath, "/test/") ||
			strings.Contains(filePath, "_test/") ||
			// If it's just "README.md" with no path, it's likely a test fixture
			filePath == "README.md" {
			return true
		}
	}

	return false
}
