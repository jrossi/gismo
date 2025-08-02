package javascript

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jrossi/gismo/pkg/toolcache"

	"github.com/jrossi/gismo/pkg/linters"
)

// JavaScriptLinter handles linting of JavaScript and TypeScript files
type JavaScriptLinter struct {
	config       *JavaScriptConfig
	cacheManager *toolcache.CacheManager

	// Tool selection cache (protected by mutex)
	mu           sync.RWMutex
	selectedTool string
	toolPath     string

	// Project context cache
	projectCache map[string]*ProjectInfo
}

// ProjectInfo contains cached project-specific information
type ProjectInfo struct {
	PackageJsonPath string            `json:"packageJsonPath"`
	TSConfigPath    string            `json:"tsconfigPath"`
	ConfigFiles     map[string]string `json:"configFiles"` // tool -> config path
	WorkspaceRoot   string            `json:"workspaceRoot"`
	LastDiscovered  time.Time         `json:"lastDiscovered"`
}

// ToolResult represents output from a JavaScript linting tool
type ToolResult struct {
	Issues    []ToolIssue `json:"issues,omitempty"`
	Formatted []byte      `json:"formatted,omitempty"`
	Success   bool        `json:"success"`
	Tool      string      `json:"tool"`
}

// ToolIssue represents a single linting issue from any tool
type ToolIssue struct {
	FilePath string `json:"filePath"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"` // "error", "warning", "info"
	Message  string `json:"message"`
	Rule     string `json:"rule"`
	Source   string `json:"source"` // Tool that generated this issue
}

// BiomeIssue represents a Biome linting issue
type BiomeIssue struct {
	Category string        `json:"category"`
	Severity string        `json:"severity"`
	Message  BiomeMessage  `json:"message"`
	Location BiomeLocation `json:"location"`
	Advices  []BiomeAdvice `json:"advices,omitempty"`
}

type BiomeMessage struct {
	Text string `json:"text"`
}

type BiomeLocation struct {
	Path BiomePath `json:"path"`
	Span BiomeSpan `json:"span"`
}

type BiomePath struct {
	File string `json:"file"`
}

type BiomeSpan struct {
	Start BiomePosition `json:"start"`
	End   BiomePosition `json:"end"`
}

type BiomePosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type BiomeAdvice struct {
	Log BiomeMessage `json:"log"`
}

// ESLintIssue represents an ESLint linting issue
type ESLintIssue struct {
	FilePath     string      `json:"filePath"`
	Messages     []ESLintMsg `json:"messages"`
	ErrorCount   int         `json:"errorCount"`
	WarningCount int         `json:"warningCount"`
	Output       string      `json:"output,omitempty"`
}

type ESLintMsg struct {
	RuleId   string `json:"ruleId"`
	Severity int    `json:"severity"` // 1 = warning, 2 = error
	Message  string `json:"message"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	NodeType string `json:"nodeType,omitempty"`
	Source   string `json:"source,omitempty"`
}

// OxlintIssue represents an Oxlint linting issue
type OxlintIssue struct {
	Type     string         `json:"type"`
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	Location OxlintLocation `json:"location"`
	Rule     string         `json:"rule,omitempty"`
}

type OxlintLocation struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// NewJavaScriptLinter creates a new JavaScript/TypeScript linter with default configuration
func NewJavaScriptLinter() *JavaScriptLinter {
	return &JavaScriptLinter{
		config:       DefaultJavaScriptConfig(),
		projectCache: make(map[string]*ProjectInfo),
	}
}

// NewJavaScriptLinterWithConfig creates a new JavaScript/TypeScript linter with custom configuration
func NewJavaScriptLinterWithConfig(config *JavaScriptConfig) *JavaScriptLinter {
	if config == nil {
		config = DefaultJavaScriptConfig()
	}

	return &JavaScriptLinter{
		config:       config,
		projectCache: make(map[string]*ProjectInfo),
	}
}

// Name returns the linter name
func (l *JavaScriptLinter) Name() string {
	return "javascript"
}

// CanHandle returns true if this linter can handle the given file
func (l *JavaScriptLinter) CanHandle(filePath string) bool {
	lowerPath := strings.ToLower(filePath)
	return strings.HasSuffix(lowerPath, ".js") ||
		strings.HasSuffix(lowerPath, ".jsx") ||
		strings.HasSuffix(lowerPath, ".ts") ||
		strings.HasSuffix(lowerPath, ".tsx") ||
		strings.HasSuffix(lowerPath, ".mjs") ||
		strings.HasSuffix(lowerPath, ".cjs") ||
		strings.HasSuffix(lowerPath, ".vue") ||
		strings.HasSuffix(lowerPath, ".svelte")
}

// SetConfig updates the linter configuration
func (l *JavaScriptLinter) SetConfig(config []byte) error {
	var jsConfig JavaScriptConfig
	if err := json.Unmarshal(config, &jsConfig); err != nil {
		return fmt.Errorf("failed to parse JavaScript config: %w", err)
	}

	l.config = &jsConfig
	return nil
}

// Lint performs linting on a single JavaScript/TypeScript file
func (l *JavaScriptLinter) Lint(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Initialize cache manager if not already done
	if l.cacheManager == nil {
		cache, err := toolcache.GetCacheManager(filePath)
		if err != nil {
			// Fallback to non-cached operation
			return l.lintWithoutCache(ctx, filePath, content)
		}
		l.cacheManager = cache
	}

	// Check file size limit
	if l.config.MaxFileSize != nil && int64(len(content)) > *l.config.MaxFileSize {
		result.Success = false
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("File size %d exceeds limit %d", len(content), *l.config.MaxFileSize),
			Rule:     "file-size",
		})
		return result, nil
	}

	// Ensure tool is discovered and ready
	if err := l.ensureToolReady(filePath); err != nil {
		return l.lintWithoutCache(ctx, filePath, content)
	}

	// Perform linting with selected tool
	return l.lintWithTool(ctx, filePath, content)
}

// LintBatch performs batch linting for performance optimization
func (l *JavaScriptLinter) LintBatch(ctx context.Context, files map[string][]byte) (map[string]*linters.LintResult, error) {
	results := make(map[string]*linters.LintResult)
	var mu sync.Mutex

	// Filter JavaScript/TypeScript files
	jsFiles := make(map[string][]byte)
	for path, content := range files {
		if l.CanHandle(path) {
			jsFiles[path] = content
		}
	}

	if len(jsFiles) == 0 {
		return results, nil
	}

	// Initialize cache manager
	if l.cacheManager == nil {
		for filePath := range jsFiles {
			cache, err := toolcache.GetCacheManager(filePath)
			if err == nil {
				l.cacheManager = cache
				break
			}
		}
	}

	// Process files in parallel
	var wg sync.WaitGroup
	for filePath, content := range jsFiles {
		wg.Add(1)
		go func(path string, data []byte) {
			defer wg.Done()

			result, err := l.Lint(ctx, path, data)
			if err != nil {
				result = &linters.LintResult{
					Success: false,
					Issues: []linters.Issue{
						{
							File:     path,
							Line:     1,
							Column:   1,
							Severity: "error",
							Message:  fmt.Sprintf("Linting failed: %v", err),
							Rule:     "internal",
						},
					},
				}
			}

			mu.Lock()
			results[path] = result
			mu.Unlock()
		}(filePath, content)
	}

	wg.Wait()
	return results, nil
}

// ensureToolReady ensures a linting tool is discovered and ready to use
func (l *JavaScriptLinter) ensureToolReady(filePath string) error {
	// Check if we already have a tool selected
	l.mu.RLock()
	hasToolSelected := l.selectedTool != "" && l.toolPath != ""
	l.mu.RUnlock()

	if hasToolSelected {
		return nil
	}

	// If tool is forced in config, use that
	if l.config.ForceTool != nil {
		return l.setupForcedTool(*l.config.ForceTool)
	}

	// Discover tools in preferred order
	return l.discoverAndSelectTool(filePath)
}

// setupForcedTool configures the linter to use a specific forced tool
func (l *JavaScriptLinter) setupForcedTool(toolName string) error {
	var toolPath string

	switch toolName {
	case "biome":
		if l.config.BiomePath != nil {
			toolPath = *l.config.BiomePath
		} else {
			tool, err := l.cacheManager.DiscoverTool("javascript", "biome")
			if err != nil || !tool.Available {
				return fmt.Errorf("forced tool 'biome' not available")
			}
			toolPath = tool.Path
		}
	case "oxlint":
		if l.config.OxlintPath != nil {
			toolPath = *l.config.OxlintPath
		} else {
			tool, err := l.cacheManager.DiscoverTool("javascript", "oxlint")
			if err != nil || !tool.Available {
				return fmt.Errorf("forced tool 'oxlint' not available")
			}
			toolPath = tool.Path
		}
	case "eslint":
		if l.config.ESLintPath != nil {
			toolPath = *l.config.ESLintPath
		} else {
			tool, err := l.cacheManager.DiscoverTool("javascript", "eslint")
			if err != nil || !tool.Available {
				return fmt.Errorf("forced tool 'eslint' not available")
			}
			toolPath = tool.Path
		}
	case "node":
		if l.config.NodePath != nil {
			toolPath = *l.config.NodePath
		} else {
			tool, err := l.cacheManager.DiscoverTool("javascript", "node")
			if err != nil || !tool.Available {
				return fmt.Errorf("forced tool 'node' not available")
			}
			toolPath = tool.Path
		}
	default:
		return fmt.Errorf("unknown forced tool: %s", toolName)
	}

	l.mu.Lock()
	l.selectedTool = toolName
	l.toolPath = toolPath
	l.mu.Unlock()

	return nil
}

// discoverAndSelectTool discovers available tools and selects the best one
func (l *JavaScriptLinter) discoverAndSelectTool(filePath string) error {
	// Use preferred tools from config, or default order
	preferredTools := l.config.PreferredTools
	if len(preferredTools) == 0 {
		preferredTools = []string{"biome", "oxlint", "eslint", "node"}
	}

	for _, toolName := range preferredTools {
		tool, err := l.cacheManager.DiscoverTool("javascript", toolName)
		if err != nil {
			continue
		}

		if tool.Available {
			l.mu.Lock()
			l.selectedTool = toolName
			l.toolPath = tool.Path
			l.mu.Unlock()
			return nil
		}
	}

	return fmt.Errorf("no JavaScript linting tools available")
}

// getToolPath safely returns the current tool path
func (l *JavaScriptLinter) getToolPath() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.toolPath
}

// lintWithTool performs linting using the selected tool
func (l *JavaScriptLinter) lintWithTool(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	l.mu.RLock()
	selectedTool := l.selectedTool
	l.mu.RUnlock()

	switch selectedTool {
	case "biome":
		return l.lintWithBiome(ctx, filePath, content)
	case "oxlint":
		return l.lintWithOxlint(ctx, filePath, content)
	case "eslint":
		return l.lintWithESLint(ctx, filePath, content)
	case "node":
		return l.lintWithNode(ctx, filePath, content)
	default:
		return l.lintWithoutCache(ctx, filePath, content)
	}
}

// lintWithBiome performs linting using Biome
func (l *JavaScriptLinter) lintWithBiome(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Create timeout context
	timeout := 30 * time.Second
	if l.config.TestTimeout != nil {
		timeout = l.config.TestTimeout.Duration
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run biome check
	// #nosec G204 - toolPath is validated through cache discovery
	cmd := exec.CommandContext(ctx, l.getToolPath(), "check", "--reporter=json", filePath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Biome returns non-zero exit code when issues are found
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		result.Success = false
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  "Biome execution timed out",
			Rule:     "timeout",
		})
		return result, nil
	}

	// Parse Biome JSON output
	if stdout.Len() > 0 {
		issues, parseErr := l.parseBiomeOutput(stdout.Bytes(), filePath)
		if parseErr != nil {
			// If we can't parse output, treat as error but continue
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     1,
				Column:   1,
				Severity: "warning",
				Message:  fmt.Sprintf("Failed to parse Biome output: %v", parseErr),
				Rule:     "parse-error",
			})
		} else {
			result.Issues = append(result.Issues, issues...)
		}
	}

	// Check if any errors were found
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			result.Success = false
			break
		}
	}

	return result, nil
}

// lintWithOxlint performs linting using Oxlint
func (l *JavaScriptLinter) lintWithOxlint(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Create timeout context
	timeout := 30 * time.Second
	if l.config.TestTimeout != nil {
		timeout = l.config.TestTimeout.Duration
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run oxlint
	// #nosec G204 - toolPath is validated through cache discovery
	cmd := exec.CommandContext(ctx, l.getToolPath(), "--format=json", filePath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Oxlint returns non-zero exit code when issues are found
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		result.Success = false
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  "Oxlint execution timed out",
			Rule:     "timeout",
		})
		return result, nil
	}

	// Parse Oxlint JSON output
	if stdout.Len() > 0 {
		issues, parseErr := l.parseOxlintOutput(stdout.Bytes(), filePath)
		if parseErr != nil {
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     1,
				Column:   1,
				Severity: "warning",
				Message:  fmt.Sprintf("Failed to parse Oxlint output: %v", parseErr),
				Rule:     "parse-error",
			})
		} else {
			result.Issues = append(result.Issues, issues...)
		}
	}

	// Check if any errors were found
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			result.Success = false
			break
		}
	}

	return result, nil
}

// lintWithESLint performs linting using ESLint
func (l *JavaScriptLinter) lintWithESLint(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Create timeout context
	timeout := 30 * time.Second
	if l.config.TestTimeout != nil {
		timeout = l.config.TestTimeout.Duration
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run ESLint
	// #nosec G204 - toolPath is validated through cache discovery
	cmd := exec.CommandContext(ctx, l.getToolPath(), "--format=json", filePath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// ESLint returns non-zero exit code when issues are found
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		result.Success = false
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  "ESLint execution timed out",
			Rule:     "timeout",
		})
		return result, nil
	}

	// Parse ESLint JSON output
	if stdout.Len() > 0 {
		issues, parseErr := l.parseESLintOutput(stdout.Bytes(), filePath)
		if parseErr != nil {
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     1,
				Column:   1,
				Severity: "warning",
				Message:  fmt.Sprintf("Failed to parse ESLint output: %v", parseErr),
				Rule:     "parse-error",
			})
		} else {
			result.Issues = append(result.Issues, issues...)
		}
	}

	// Check if any errors were found
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			result.Success = false
			break
		}
	}

	return result, nil
}

// lintWithNode performs basic syntax checking using Node.js
func (l *JavaScriptLinter) lintWithNode(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Create timeout context
	timeout := 10 * time.Second
	if l.config.TestTimeout != nil {
		timeout = l.config.TestTimeout.Duration
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use Node.js to check syntax
	// #nosec G204 - toolPath is validated through cache discovery
	cmd := exec.CommandContext(ctx, l.getToolPath(), "-c", string(content))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Success = false
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     1,
				Column:   1,
				Severity: "error",
				Message:  "Node.js syntax check timed out",
				Rule:     "timeout",
			})
		} else {
			// Parse syntax error from Node.js
			errorMsg := strings.TrimSpace(stderr.String())
			if errorMsg != "" {
				result.Success = false
				issue := l.parseNodeError(errorMsg, filePath)
				result.Issues = append(result.Issues, issue)
			}
		}
	}

	return result, nil
}

// lintWithoutCache performs linting without cache (fallback)
func (l *JavaScriptLinter) lintWithoutCache(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	// Try to find any available tool and use it
	tools := []string{"biome", "oxlint", "eslint", "node"}
	for _, tool := range tools {
		if path, err := exec.LookPath(tool); err == nil {
			l.mu.Lock()
			l.selectedTool = tool
			l.toolPath = path
			l.mu.Unlock()
			return l.lintWithTool(ctx, filePath, content)
		}
	}

	// No tools available, provide a basic syntax check
	return l.basicSyntaxCheck(filePath, content)
}

// basicSyntaxCheck performs a very basic syntax validation
func (l *JavaScriptLinter) basicSyntaxCheck(filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Basic checks for common syntax errors
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		lineNum := i + 1

		// Check for common issues
		trimmed := strings.TrimSpace(line)

		// Check for unmatched braces (very basic)
		openBraces := strings.Count(line, "{")
		closeBraces := strings.Count(line, "}")
		if openBraces != closeBraces && (openBraces > 0 || closeBraces > 0) {
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     lineNum,
				Column:   1,
				Severity: "warning",
				Message:  "Possible unmatched braces",
				Rule:     "basic-syntax",
			})
		}

		// Check for common typos
		if strings.Contains(trimmed, "function(") && !strings.Contains(trimmed, "function (") {
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     lineNum,
				Column:   strings.Index(line, "function(") + 1,
				Severity: "info",
				Message:  "Consider adding space after 'function'",
				Rule:     "basic-style",
			})
		}
	}

	return result, nil
}

// parseBiomeOutput parses Biome JSON output into linter issues
func (l *JavaScriptLinter) parseBiomeOutput(output []byte, filePath string) ([]linters.Issue, error) {
	var biomeResult struct {
		Diagnostics []BiomeIssue `json:"diagnostics"`
	}

	if err := json.Unmarshal(output, &biomeResult); err != nil {
		return nil, fmt.Errorf("failed to parse Biome JSON: %w", err)
	}

	var issues []linters.Issue
	for _, diag := range biomeResult.Diagnostics {
		severity := "warning"
		if diag.Severity == "error" {
			severity = "error"
		}

		issue := linters.Issue{
			File:     filePath,
			Line:     diag.Location.Span.Start.Line,
			Column:   diag.Location.Span.Start.Column,
			Severity: severity,
			Message:  diag.Message.Text,
			Rule:     diag.Category,
		}

		issues = append(issues, issue)
	}

	return issues, nil
}

// parseOxlintOutput parses Oxlint JSON output into linter issues
func (l *JavaScriptLinter) parseOxlintOutput(output []byte, filePath string) ([]linters.Issue, error) {
	var oxlintIssues []OxlintIssue

	if err := json.Unmarshal(output, &oxlintIssues); err != nil {
		return nil, fmt.Errorf("failed to parse Oxlint JSON: %w", err)
	}

	var issues []linters.Issue
	for _, oxIssue := range oxlintIssues {
		severity := "warning"
		if oxIssue.Severity == "error" {
			severity = "error"
		}

		issue := linters.Issue{
			File:     filePath,
			Line:     oxIssue.Location.Line,
			Column:   oxIssue.Location.Column,
			Severity: severity,
			Message:  oxIssue.Message,
			Rule:     oxIssue.Rule,
		}

		issues = append(issues, issue)
	}

	return issues, nil
}

// parseESLintOutput parses ESLint JSON output into linter issues
func (l *JavaScriptLinter) parseESLintOutput(output []byte, filePath string) ([]linters.Issue, error) {
	var eslintResults []ESLintIssue

	if err := json.Unmarshal(output, &eslintResults); err != nil {
		return nil, fmt.Errorf("failed to parse ESLint JSON: %w", err)
	}

	var issues []linters.Issue
	for _, result := range eslintResults {
		for _, msg := range result.Messages {
			severity := "warning"
			if msg.Severity == 2 {
				severity = "error"
			}

			issue := linters.Issue{
				File:     filePath,
				Line:     msg.Line,
				Column:   msg.Column,
				Severity: severity,
				Message:  msg.Message,
				Rule:     msg.RuleId,
			}

			issues = append(issues, issue)
		}
	}

	return issues, nil
}

// parseNodeError parses Node.js syntax error into a linter issue
func (l *JavaScriptLinter) parseNodeError(errorMsg, filePath string) linters.Issue {
	// Parse line number from Node.js error (if available)
	line := 1
	column := 1

	// Look for patterns like "SyntaxError: ... at line 5"
	scanner := bufio.NewScanner(strings.NewReader(errorMsg))
	for scanner.Scan() {
		text := scanner.Text()
		if strings.Contains(text, "SyntaxError") {
			// Extract line number if present
			if idx := strings.Index(text, " line "); idx != -1 {
				// Try to parse line number
				_, _ = fmt.Sscanf(text[idx+6:], "%d", &line)
			}
			break
		}
	}

	return linters.Issue{
		File:     filePath,
		Line:     line,
		Column:   column,
		Severity: "error",
		Message:  errorMsg,
		Rule:     "syntax",
	}
}
