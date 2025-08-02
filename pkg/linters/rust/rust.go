package rust

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jrossi/gismo/pkg/linters"

	json "github.com/goccy/go-json"
)

// RustLinter handles Rust file linting, formatting, and test running with cargo tools
type RustLinter struct {
	// Cache for Cargo.toml locations
	cargoCache map[string]*CargoInfo
	// Cache cargo binary paths for performance
	cargoPaths struct {
		cargo   string
		clippy  string
		fmt     string
		hasRust bool
	}
	cargoOnce sync.Once
	mu        sync.RWMutex
	config    *RustConfig
}

// CargoInfo contains information about a Cargo workspace or package
type CargoInfo struct {
	Root          string // Directory containing Cargo.toml
	CargoTomlPath string // Full path to Cargo.toml
	IsWorkspace   bool   // Whether this is a workspace root
}

// ClippyMessage represents a single message from clippy's JSON output
type ClippyMessage struct {
	Reason  string `json:"reason"`
	Message struct {
		Rendered string `json:"rendered"`
		Level    string `json:"level"`
		Spans    []struct {
			FileName    string `json:"file_name"`
			LineStart   int    `json:"line_start"`
			LineEnd     int    `json:"line_end"`
			ColumnStart int    `json:"column_start"`
			ColumnEnd   int    `json:"column_end"`
			Text        []struct {
				Text string `json:"text"`
			} `json:"text"`
		} `json:"spans"`
		Code struct {
			Code        string `json:"code"`
			Explanation string `json:"explanation"`
		} `json:"code"`
	} `json:"message"`
}

// NewRustLinter creates a new Rust linter with default configuration
func NewRustLinter() *RustLinter {
	return NewRustLinterWithConfig(nil)
}

// NewRustLinterWithConfig creates a new Rust linter with the given configuration
func NewRustLinterWithConfig(config *RustConfig) *RustLinter {
	if config == nil {
		config = DefaultRustConfig()
	}

	return &RustLinter{
		cargoCache: make(map[string]*CargoInfo),
		config:     config,
	}
}

// SetConfig updates the linter configuration
func (l *RustLinter) SetConfig(configData json.RawMessage) error {
	var config RustConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse rust config: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Update config
	l.config = &config

	// Set defaults if not provided
	if l.config.TestTimeout == nil {
		defaultTimeout := &Duration{Duration: 10 * time.Minute}
		l.config.TestTimeout = defaultTimeout
	}

	return nil
}

// Name returns the linter name
func (l *RustLinter) Name() string {
	return "rust"
}

// CanHandle returns true for Rust files
func (l *RustLinter) CanHandle(filePath string) bool {
	return strings.HasSuffix(filePath, ".rs")
}

// findCargoTools locates the cargo tools and caches the paths
func (l *RustLinter) findCargoTools() {
	l.cargoOnce.Do(func() {
		// Check for cargo
		if path, err := exec.LookPath("cargo"); err == nil {
			l.cargoPaths.cargo = path
			l.cargoPaths.hasRust = true

			// Check if clippy is available
			cmd := exec.Command(path, "clippy", "--version")
			if err := cmd.Run(); err == nil {
				l.cargoPaths.clippy = path
			}

			// Check if rustfmt is available
			cmd = exec.Command(path, "fmt", "--version")
			if err := cmd.Run(); err == nil {
				l.cargoPaths.fmt = path
			}
		}
	})
}

// FindCargoRoot walks up the directory tree to find Cargo.toml
func (l *RustLinter) FindCargoRoot(startPath string) (*CargoInfo, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check cache first
	l.mu.RLock()
	if cargoInfo, exists := l.cargoCache[absPath]; exists {
		l.mu.RUnlock()
		return cargoInfo, nil
	}
	l.mu.RUnlock()

	// Walk up the directory tree
	currentPath := absPath
	if info, err := os.Stat(currentPath); err == nil && !info.IsDir() {
		currentPath = filepath.Dir(currentPath)
	}

	for {
		cargoTomlPath := filepath.Join(currentPath, "Cargo.toml")
		if _, err := os.Stat(cargoTomlPath); err == nil {
			// Found Cargo.toml
			cargoInfo := &CargoInfo{
				Root:          currentPath,
				CargoTomlPath: cargoTomlPath,
			}

			// Check if this is a workspace
			if data, err := os.ReadFile(cargoTomlPath); err == nil {
				if bytes.Contains(data, []byte("[workspace]")) {
					cargoInfo.IsWorkspace = true
				}
			}

			// Cache the result
			l.mu.Lock()
			l.cargoCache[absPath] = cargoInfo
			l.mu.Unlock()

			return cargoInfo, nil
		}

		// Get parent directory
		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			// Reached root of filesystem
			return nil, fmt.Errorf("Cargo.toml not found")
		}
		currentPath = parent
	}
}

// runClippy executes cargo clippy on the specified file
func (l *RustLinter) runClippy(ctx context.Context, filePath string) ([]ClippyMessage, error) {
	l.findCargoTools()
	if !l.cargoPaths.hasRust || l.cargoPaths.clippy == "" {
		return nil, fmt.Errorf("cargo clippy not found")
	}

	// Find cargo root for proper context
	cargoInfo, err := l.FindCargoRoot(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to find Cargo.toml: %w", err)
	}

	// Build clippy arguments
	args := []string{"clippy", "--message-format=json"}

	// Add configuration options
	if l.config.NoDeps {
		args = append(args, "--no-deps")
	}
	if l.config.AllTargets {
		args = append(args, "--all-targets")
	}
	if l.config.AllFeatures {
		args = append(args, "--all-features")
	}
	if len(l.config.Features) > 0 {
		args = append(args, "--features", strings.Join(l.config.Features, ","))
	}

	// Add the separator before lint flags
	args = append(args, "--")

	// Add enabled lints
	for _, lint := range l.config.EnabledLints {
		args = append(args, "-W", lint)
	}

	// Add disabled lints
	for _, lint := range l.config.DisabledLints {
		args = append(args, "-A", lint)
	}

	// Execute clippy
	// #nosec G204 - cargoPaths.cargo is validated through findCargoTools()
	cmd := exec.CommandContext(ctx, l.cargoPaths.cargo, args...)
	cmd.Dir = cargoInfo.Root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// clippy returns non-zero exit code when warnings are found, which is expected
	err = cmd.Run()

	// Parse JSON output line by line
	var messages []ClippyMessage
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line == "" {
			continue
		}

		var msg ClippyMessage
		if err := json.Unmarshal([]byte(line), &msg); err == nil && msg.Reason == "compiler-message" {
			messages = append(messages, msg)
		}
	}

	// Check if the error is due to actual failure (not just warnings)
	if err != nil && len(messages) == 0 && stderr.Len() > 0 {
		return nil, fmt.Errorf("cargo clippy failed: %v\nstderr: %s", err, stderr.String())
	}

	return messages, nil
}

// runFmtCheck checks if the file needs formatting
func (l *RustLinter) runFmtCheck(ctx context.Context, filePath string) (bool, error) {
	l.findCargoTools()
	if !l.cargoPaths.hasRust || l.cargoPaths.fmt == "" {
		return true, nil // Skip fmt check if rustfmt is not available
	}

	// Find cargo root
	cargoInfo, err := l.FindCargoRoot(filePath)
	if err != nil {
		return true, nil // Skip fmt check if not in a cargo project
	}

	// Run cargo fmt in check mode
	args := []string{"fmt", "--check"}
	if l.config.Verbose {
		args = append(args, "--verbose")
	}

	// #nosec G204 - cargoPaths.cargo is validated through findCargoTools()
	cmd := exec.CommandContext(ctx, l.cargoPaths.cargo, args...)
	cmd.Dir = cargoInfo.Root

	err = cmd.Run()
	// If the command returns non-zero, formatting is needed
	return err == nil, nil
}

// convertClippyMessages converts clippy messages to our internal Issue format
func (l *RustLinter) convertClippyMessages(messages []ClippyMessage, filePath string) []linters.Issue {
	var issues []linters.Issue

	for _, msg := range messages {
		// Skip messages not related to the current file
		relevant := false
		for _, span := range msg.Message.Spans {
			if span.FileName == filePath {
				relevant = true
				break
			}
		}
		if !relevant {
			continue
		}

		// Map clippy levels to our severity
		severity := "warning"
		switch msg.Message.Level {
		case "error":
			severity = "error"
		case "warning":
			severity = "warning"
		case "note", "help":
			severity = "info"
		}

		// Find the primary span for this file
		for _, span := range msg.Message.Spans {
			if span.FileName == filePath {
				issue := linters.Issue{
					File:     filePath,
					Line:     span.LineStart,
					Column:   span.ColumnStart,
					Severity: severity,
					Message:  strings.TrimSpace(msg.Message.Rendered),
					Rule:     msg.Message.Code.Code,
				}
				issues = append(issues, issue)
				break // Only add one issue per message
			}
		}
	}

	return issues
}

// Lint performs linting on a Rust file
func (l *RustLinter) Lint(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Skip if not in a Cargo project
	if _, err := l.FindCargoRoot(filePath); err != nil {
		// Not in a Cargo project, just check basic syntax
		// For now, we'll skip files not in a Cargo project
		return result, nil
	}

	// Check formatting first
	needsFormatting, err := l.runFmtCheck(ctx, filePath)
	if err == nil && !needsFormatting {
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "warning",
			Message:  "File is not properly formatted with rustfmt",
			Rule:     "rustfmt",
		})
	}

	// Run clippy
	if messages, err := l.runClippy(ctx, filePath); err == nil {
		clippyIssues := l.convertClippyMessages(messages, filePath)
		result.Issues = append(result.Issues, clippyIssues...)

		// Check if any issues are errors
		for _, issue := range clippyIssues {
			if issue.Severity == "error" {
				result.Success = false
			}
		}
	}
	// If clippy fails, we continue with just formatting results

	// Run tests if this is a test file or has tests
	if strings.Contains(string(content), "#[test]") || strings.Contains(string(content), "#[cfg(test)]") {
		if output, err := l.runTests(ctx, filePath); err != nil {
			result.Success = false
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     1,
				Column:   1,
				Severity: "error",
				Message:  fmt.Sprintf("Tests failed: %v", err),
				Rule:     "test",
			})
			result.TestOutput = output
		} else {
			result.TestOutput = output
		}
	}

	return result, nil
}

// runTests runs tests for a specific Rust file
func (l *RustLinter) runTests(ctx context.Context, filePath string) (string, error) {
	l.findCargoTools()
	if !l.cargoPaths.hasRust {
		return "", fmt.Errorf("cargo not found")
	}

	// Find cargo root
	cargoInfo, err := l.FindCargoRoot(filePath)
	if err != nil {
		return "", nil // Skip tests if not in a cargo project
	}

	// Extract the module path from the file path
	relPath, err := filepath.Rel(cargoInfo.Root, filePath)
	if err != nil {
		return "", nil
	}

	// Convert file path to module path (e.g., src/lib/foo.rs -> lib::foo)
	modulePath := strings.TrimSuffix(relPath, ".rs")
	modulePath = strings.TrimPrefix(modulePath, "src/")
	modulePath = strings.ReplaceAll(modulePath, "/", "::")

	// Build test command
	args := []string{"test"}

	// Add timeout if configured
	if l.config.TestTimeout != nil {
		// Cargo doesn't have a direct timeout flag, so we'll rely on context timeout
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, l.config.TestTimeout.Duration)
		defer cancel()
	}

	// Add test filter to run only tests in this module
	if modulePath != "" && modulePath != "main" && modulePath != "lib" {
		args = append(args, modulePath)
	}

	// Add feature flags
	if l.config.AllFeatures {
		args = append(args, "--all-features")
	} else if len(l.config.Features) > 0 {
		args = append(args, "--features", strings.Join(l.config.Features, ","))
	}

	// Run tests
	// #nosec G204 - cargoPaths.cargo is validated through findCargoTools()
	cmd := exec.CommandContext(ctx, l.cargoPaths.cargo, args...)
	cmd.Dir = cargoInfo.Root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	if err != nil {
		return output, fmt.Errorf("cargo test failed: %w", err)
	}

	return output, nil
}
