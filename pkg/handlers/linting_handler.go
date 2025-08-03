package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jrossi/gismo/pkg/engine"

	linters2 "github.com/jrossi/gismo/pkg/linters"
	"github.com/jrossi/gismo/pkg/linters/golang"
	"github.com/jrossi/gismo/pkg/linters/javascript"
	jsonlinter "github.com/jrossi/gismo/pkg/linters/json"
	"github.com/jrossi/gismo/pkg/linters/markdown"
	"github.com/jrossi/gismo/pkg/linters/protobuf"
	"github.com/jrossi/gismo/pkg/linters/python"
	"github.com/jrossi/gismo/pkg/linters/rust"

	json "github.com/goccy/go-json"
)

// LintingHandler handles linting for file operations
type LintingHandler struct {
	*engine.BaseActionHandler
	linters  []linters2.Linter
	executor *linters2.ParallelExecutor
	config   *engine.AppConfig
}

// LintingConfig provides configuration options for the linting handler
type LintingConfig struct {
	MaxWorkers      int  `json:"max_workers"`
	DisableParallel bool `json:"disable_parallel"`
}

// NewLintingHandler creates a new linting handler
func NewLintingHandler() *LintingHandler {
	return NewLintingHandlerWithConfig(LintingConfig{})
}

// NewLintingHandlerWithConfig creates a new linting handler with custom configuration
func NewLintingHandlerWithConfig(config LintingConfig) *LintingHandler {
	maxWorkers := config.MaxWorkers
	if config.DisableParallel {
		maxWorkers = 1
	}

	handler := &LintingHandler{
		BaseActionHandler: engine.NewBaseActionHandler("linting", 100),
		linters:           []linters2.Linter{},
		executor:          linters2.NewParallelExecutor(maxWorkers),
		config:            engine.NewAppConfig(),
	}

	// Initialize default linters
	handler.linters = append(handler.linters, golang.NewGoLinter())
	handler.linters = append(handler.linters, javascript.NewJavaScriptLinter())
	handler.linters = append(handler.linters, jsonlinter.NewJSONLinter())
	handler.linters = append(handler.linters, markdown.NewMarkdownLinter())
	handler.linters = append(handler.linters, protobuf.NewProtobufLinter())
	handler.linters = append(handler.linters, python.NewPythonLinter())
	handler.linters = append(handler.linters, rust.NewRustLinter())

	return handler
}

// SetConfig sets the application configuration
func (h *LintingHandler) SetConfig(config *engine.AppConfig) {
	h.config = config

	// Update linter configurations
	if config != nil {
		for _, linter := range h.linters {
			// Check if this linter is disabled
			if !config.IsLinterEnabled(linter.Name()) {
				continue
			}

			// Get linter-specific configuration
			if linterConfig, ok := config.GetLinterConfig(linter.Name()); ok {
				// Try to cast to configurable linter
				if configurable, ok := linter.(engine.ConfigurableLinter); ok {
					if err := configurable.SetConfig(linterConfig); err != nil {
						// Log error but continue
						fmt.Fprintf(os.Stderr, "Warning: Failed to configure %s linter: %v\n", linter.Name(), err)
					}
				}
			}
		}
	}
}

// ShouldHandle determines if this handler should process file-related tool events
func (h *LintingHandler) ShouldHandle(ctx context.Context, event engine.HookMessage) bool {
	switch msg := event.(type) {
	case *engine.PreToolUseMessage:
		return msg.ToolName == "Write" || msg.ToolName == "Edit" || msg.ToolName == "MultiEdit"
	case *engine.PostToolUseMessage:
		return msg.ToolName == "Write" || msg.ToolName == "Edit" || msg.ToolName == "MultiEdit"
	default:
		return false
	}
}

// HandlePreToolUse checks files before they're written
func (h *LintingHandler) HandlePreToolUse(ctx context.Context, msg *engine.PreToolUseMessage) (*engine.HookResponse, error) {
	// Only check Write operations (Edit/MultiEdit can't be linted until after)
	if msg.ToolName != "Write" {
		return &engine.HookResponse{Decision: "approve"}, nil
	}

	// Extract file path from tool input
	filePathRaw, exists := msg.ToolInput["file_path"]
	if !exists {
		return &engine.HookResponse{Decision: "approve"}, nil
	}
	var filePath string
	if err := json.Unmarshal(filePathRaw, &filePath); err != nil {
		return &engine.HookResponse{Decision: "approve"}, nil
	}

	// Skip temporary test files
	if h.isTemporaryTestFile(filePath) {
		return &engine.HookResponse{Decision: "approve"}, nil
	}

	// Extract content from tool input
	contentRaw, exists := msg.ToolInput["content"]
	if !exists {
		return &engine.HookResponse{Decision: "approve"}, nil
	}
	var content string
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		return &engine.HookResponse{Decision: "approve"}, nil
	}

	// Apply rule overrides for this file
	h.applyRuleOverrides(filePath)

	// Run all applicable linters in parallel
	results := h.executor.ExecuteLinters(ctx, h.linters, filePath, []byte(content))

	// Aggregate results
	aggregatedResult, errs := linters2.AggregateResults(results)

	// Handle any linting errors
	if len(errs) > 0 {
		return &engine.HookResponse{
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
		output := h.formatLintOutput(filePath, errorIssues, true)
		fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n%s\n", output)
		return &engine.HookResponse{
			Decision: "block",
			Reason:   fmt.Sprintf("Found %d error(s) in %s", len(errorIssues), filePath),
		}, nil
	}

	// If formatting is needed, inform but don't block
	if len(warningIssues) > 0 {
		output := h.formatLintOutput(filePath, warningIssues, false)
		fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n%s\n", output)
		return &engine.HookResponse{
			Decision: "approve",
			Message:  fmt.Sprintf("Found %d warning(s) in %s", len(warningIssues), filePath),
		}, nil
	}

	// Write success message to stderr
	fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n  - [gismo]: ✅ Style clean. Continue with your task.\n")
	return &engine.HookResponse{Decision: "approve"}, nil
}

// HandlePostToolUse runs linters and tests after file operations
func (h *LintingHandler) HandlePostToolUse(ctx context.Context, msg *engine.PostToolUseMessage) (*engine.HookResponse, error) {
	// Skip if there was an error
	if msg.ToolError != "" {
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

	// Skip temporary test files
	if h.isTemporaryTestFile(filePath) {
		return nil, nil
	}

	// Read the actual file from disk
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n  - [gismo]: ⚠️  File not found: %s\n", filePath)
		} else {
			fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n  - [gismo]: ⚠️  Cannot read file: %v\n", err)
		}
		return nil, nil
	}

	// Apply rule overrides for this file
	h.applyRuleOverrides(filePath)

	// Run all applicable linters in parallel
	results := h.executor.ExecuteLinters(ctx, h.linters, filePath, content)

	// Aggregate results
	aggregatedResult, errs := linters2.AggregateResults(results)

	// Handle any linting errors
	for _, err := range errs {
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
		output := h.formatLintOutput(filePath, errorIssues, true)
		fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n%s\n", output)
	} else if len(warningIssues) > 0 {
		output := h.formatLintOutput(filePath, warningIssues, false)
		fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n%s\n", output)
	} else if len(errs) == 0 {
		fmt.Fprintf(os.Stderr, "\n> Write operation feedback:\n  - [gismo]: ✅ Style clean. Continue with your task.\n")
	}

	// Check for associated test files if it's a Go file
	if strings.HasSuffix(filePath, ".go") && !strings.HasSuffix(filePath, "_test.go") {
		h.checkTestFile(ctx, filePath)
	}

	return nil, nil
}

// applyRuleOverrides applies any rule overrides for the given file path
func (h *LintingHandler) applyRuleOverrides(filePath string) {
	if h.config == nil {
		return
	}

	// Apply overrides for each linter
	for _, linter := range h.linters {
		// Get any rule overrides for this file and linter
		overrides := h.config.GetRuleOverrides(filePath, linter.Name())
		if len(overrides) == 0 {
			continue
		}

		// Try to cast to configurable linter
		if configurable, ok := linter.(engine.ConfigurableLinter); ok {
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
					fmt.Fprintf(os.Stderr, "Warning: Failed to apply rule override for %s linter: %v\n", linter.Name(), err)
				}
			}
		}
	}
}

// formatLintOutput formats linting issues in a style similar to smart-lint.sh
func (h *LintingHandler) formatLintOutput(filePath string, issues []linters2.Issue, isBlocking bool) string {
	var output strings.Builder

	// Header similar to smart-lint.sh
	output.WriteString(fmt.Sprintf("- [gismo:%s]: ", filePath))

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
func (h *LintingHandler) checkTestFile(ctx context.Context, filePath string) {
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
	results := h.executor.ExecuteLinters(ctx, h.linters, testPath, content)

	// Aggregate results
	aggregatedResult, errs := linters2.AggregateResults(results)

	// Handle any linting errors
	for _, err := range errs {
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

		if len(errorIssues) > 0 {
			output := h.formatLintOutput(testPath, errorIssues, true)
			fmt.Fprintf(os.Stderr, "\n> Test file feedback:\n%s\n", output)
		} else if len(warningIssues) > 0 {
			output := h.formatLintOutput(testPath, warningIssues, false)
			fmt.Fprintf(os.Stderr, "\n> Test file feedback:\n%s\n", output)
		}
	}
}

// isTemporaryTestFile checks if a file path represents a temporary test file
func (h *LintingHandler) isTemporaryTestFile(filePath string) bool {
	baseName := filepath.Base(filePath)

	// Only exclude very specific patterns that are clearly test fixtures
	if filePath == baseName {
		verySpecificPatterns := []string{
			"success_test.go",
			"success_test.md",
			"large_test.go",
			"large_test.md",
			"concurrent_test.go",
			"concurrent_test.md",
		}

		for _, pattern := range verySpecificPatterns {
			if pattern == baseName {
				return true
			}
		}
	}

	// Special case: Exclude README.md files that are clearly test fixtures
	if baseName == "README.md" {
		if strings.Contains(filePath, "/testdata/") ||
			strings.Contains(filePath, "/test/") ||
			strings.Contains(filePath, "_test/") ||
			filePath == "README.md" {
			return true
		}
	}

	return false
}
