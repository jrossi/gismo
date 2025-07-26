package protobuf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jrossi/gismo/linters"
)

// ProtobufLinter handles Protocol Buffer file linting using buf, protolint, or protoc
type ProtobufLinter struct {
	// Cache for proto workspace locations
	workspaceCache map[string]*ProtoWorkspaceInfo
	// Cache tool binary paths for performance
	toolPaths struct {
		buf       string
		protolint string
		protoc    string
		hasBuf    bool
		hasProto  bool
		checked   bool
	}
	toolOnce sync.Once
	mu       sync.RWMutex
	config   *ProtobufConfig
}

// ProtoWorkspaceInfo contains information about a protobuf workspace
type ProtoWorkspaceInfo struct {
	Root        string   // Directory containing buf.yaml or buf.work.yaml
	ConfigPath  string   // Full path to buf.yaml or buf.work.yaml
	IsWorkspace bool     // Whether this is a buf workspace (buf.work.yaml)
	ProtoRoots  []string // Proto import paths
}

// BufMessage represents a single message from buf's JSON output
type BufMessage struct {
	Path        string `json:"path"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	StartColumn int    `json:"start_column"`
	EndColumn   int    `json:"end_column"`
	Type        string `json:"type"`
	Message     string `json:"message"`
}

// ProtolintMessage represents a single message from protolint's JSON output
type ProtolintMessage struct {
	Filename string `json:"filename"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
	Rule     string `json:"rule"`
}

// NewProtobufLinter creates a new Protobuf linter with default configuration
func NewProtobufLinter() *ProtobufLinter {
	return NewProtobufLinterWithConfig(nil)
}

// NewProtobufLinterWithConfig creates a new Protobuf linter with the given configuration
func NewProtobufLinterWithConfig(config *ProtobufConfig) *ProtobufLinter {
	if config == nil {
		config = DefaultProtobufConfig()
	}

	return &ProtobufLinter{
		workspaceCache: make(map[string]*ProtoWorkspaceInfo),
		config:         config,
	}
}

// SetConfig updates the linter configuration
func (l *ProtobufLinter) SetConfig(configData json.RawMessage) error {
	var config ProtobufConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse protobuf config: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Update config
	l.config = &config

	// Set defaults if not provided
	if len(l.config.PreferredTools) == 0 {
		l.config.PreferredTools = []string{"buf", "protolint", "protoc"}
	}
	if l.config.TestTimeout == nil {
		defaultTimeout := &Duration{Duration: 2 * time.Minute}
		l.config.TestTimeout = defaultTimeout
	}
	if l.config.MaxFileSize == nil {
		l.config.MaxFileSize = intPtr(10 * 1024 * 1024) // 10MB
	}

	return nil
}

// Name returns the linter name
func (l *ProtobufLinter) Name() string {
	return "protobuf"
}

// CanHandle returns true for Protocol Buffer files
func (l *ProtobufLinter) CanHandle(filePath string) bool {
	return strings.HasSuffix(filePath, ".proto")
}

// findProtoTools locates the protobuf tools and caches the paths
func (l *ProtobufLinter) findProtoTools() {
	l.toolOnce.Do(func() {
		// Check for buf
		if l.config.BufPath != nil && *l.config.BufPath != "" {
			l.toolPaths.buf = *l.config.BufPath
			l.toolPaths.hasBuf = true
		} else if path, err := exec.LookPath("buf"); err == nil {
			l.toolPaths.buf = path
			l.toolPaths.hasBuf = true
		}

		// Check for protolint
		if l.config.ProtolintPath != nil && *l.config.ProtolintPath != "" {
			l.toolPaths.protolint = *l.config.ProtolintPath
		} else if path, err := exec.LookPath("protolint"); err == nil {
			l.toolPaths.protolint = path
		}

		// Check for protoc
		if l.config.ProtocPath != nil && *l.config.ProtocPath != "" {
			l.toolPaths.protoc = *l.config.ProtocPath
			l.toolPaths.hasProto = true
		} else if path, err := exec.LookPath("protoc"); err == nil {
			l.toolPaths.protoc = path
			l.toolPaths.hasProto = true
		}

		l.toolPaths.checked = true
	})
}

// FindProtoWorkspace walks up the directory tree to find buf.yaml or buf.work.yaml
func (l *ProtobufLinter) FindProtoWorkspace(startPath string) (*ProtoWorkspaceInfo, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check cache first
	l.mu.RLock()
	if workspaceInfo, exists := l.workspaceCache[absPath]; exists {
		l.mu.RUnlock()
		return workspaceInfo, nil
	}
	l.mu.RUnlock()

	// Walk up the directory tree
	currentPath := absPath
	if info, err := os.Stat(currentPath); err == nil && !info.IsDir() {
		currentPath = filepath.Dir(currentPath)
	}

	for {
		// Check for buf.work.yaml first (workspace)
		bufWorkPath := filepath.Join(currentPath, "buf.work.yaml")
		if _, err := os.Stat(bufWorkPath); err == nil {
			workspaceInfo := &ProtoWorkspaceInfo{
				Root:        currentPath,
				ConfigPath:  bufWorkPath,
				IsWorkspace: true,
			}

			// Cache the result
			l.mu.Lock()
			l.workspaceCache[absPath] = workspaceInfo
			l.mu.Unlock()

			return workspaceInfo, nil
		}

		// Check for buf.yaml
		bufConfigPath := filepath.Join(currentPath, "buf.yaml")
		if _, err := os.Stat(bufConfigPath); err == nil {
			workspaceInfo := &ProtoWorkspaceInfo{
				Root:        currentPath,
				ConfigPath:  bufConfigPath,
				IsWorkspace: false,
			}

			// Cache the result
			l.mu.Lock()
			l.workspaceCache[absPath] = workspaceInfo
			l.mu.Unlock()

			return workspaceInfo, nil
		}

		// Get parent directory
		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			// Reached root of filesystem
			return nil, fmt.Errorf("buf.yaml or buf.work.yaml not found")
		}
		currentPath = parent
	}
}

// runBuf executes buf lint on the specified file
func (l *ProtobufLinter) runBuf(ctx context.Context, filePath string) ([]BufMessage, error) {
	l.findProtoTools()
	if !l.toolPaths.hasBuf {
		return nil, fmt.Errorf("buf not found")
	}

	// Find workspace root for proper context
	workspaceInfo, err := l.FindProtoWorkspace(filePath)
	if err != nil {
		// Try running buf without workspace context
		workspaceInfo = &ProtoWorkspaceInfo{
			Root: filepath.Dir(filePath),
		}
	}

	// Build buf lint arguments
	args := []string{"lint", "--format=json"}

	// Add config path if specified
	if l.config.BufConfigPath != nil && *l.config.BufConfigPath != "" {
		args = append(args, "--config", *l.config.BufConfigPath)
	} else if workspaceInfo.ConfigPath != "" && !workspaceInfo.IsWorkspace {
		args = append(args, "--config", workspaceInfo.ConfigPath)
	}

	// Add disabled checks
	for _, check := range l.config.DisabledChecks {
		args = append(args, "--except", check)
	}

	// Add the file path
	args = append(args, filePath)

	// Execute buf
	// #nosec G204 - toolPaths.buf is validated through findProtoTools()
	cmd := exec.CommandContext(ctx, l.toolPaths.buf, args...)
	cmd.Dir = workspaceInfo.Root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// buf returns non-zero exit code when lint issues are found, which is expected
	err = cmd.Run()

	// Parse JSON output line by line
	var messages []BufMessage
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line == "" {
			continue
		}

		var msg BufMessage
		if err := json.Unmarshal([]byte(line), &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	// Check if the error is due to actual failure (not just lint issues)
	if err != nil && len(messages) == 0 && stderr.Len() > 0 {
		return nil, fmt.Errorf("buf lint failed: %v\nstderr: %s", err, stderr.String())
	}

	return messages, nil
}

// runProtolint executes protolint on the specified file
func (l *ProtobufLinter) runProtolint(ctx context.Context, filePath string) ([]ProtolintMessage, error) {
	l.findProtoTools()
	if l.toolPaths.protolint == "" {
		return nil, fmt.Errorf("protolint not found")
	}

	// Build protolint arguments
	args := []string{"lint", "-reporter", "json"}

	// Add config path if specified
	if l.config.ProtolintConfig != nil && *l.config.ProtolintConfig != "" {
		args = append(args, "-config_path", *l.config.ProtolintConfig)
	}

	args = append(args, filePath)

	// Execute protolint
	// #nosec G204 - toolPaths.protolint is validated through findProtoTools()
	cmd := exec.CommandContext(ctx, l.toolPaths.protolint, args...)
	cmd.Dir = filepath.Dir(filePath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// protolint returns non-zero exit code when lint issues are found
	cmdErr := cmd.Run()

	// Parse JSON output
	var result struct {
		Lints []ProtolintMessage `json:"lints"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		// If JSON parsing fails and there was an execution error
		if cmdErr != nil && stderr.Len() > 0 {
			return nil, fmt.Errorf("protolint failed: %v\nstderr: %s", cmdErr, stderr.String())
		}
		return nil, fmt.Errorf("failed to parse protolint output: %w", err)
	}

	return result.Lints, nil
}

// runProtoc executes protoc for basic syntax validation
func (l *ProtobufLinter) runProtoc(ctx context.Context, filePath string) error {
	l.findProtoTools()
	if !l.toolPaths.hasProto {
		return fmt.Errorf("protoc not found")
	}

	// Build protoc arguments for syntax checking only
	// Use --descriptor_set_out to /dev/null or temp file for syntax validation
	// This prevents the "Missing output directives" error
	tempFile := filepath.Join(os.TempDir(), "protoc_check.pb")
	defer os.Remove(tempFile) // Clean up temp file

	args := []string{
		"--proto_path=" + filepath.Dir(filePath),
		"--descriptor_set_out=" + tempFile,
		filePath,
	}

	// Execute protoc
	// #nosec G204 - toolPaths.protoc is validated through findProtoTools()
	cmd := exec.CommandContext(ctx, l.toolPaths.protoc, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("protoc validation failed: %v\nstderr: %s", err, stderr.String())
	}

	return nil
}

// convertBufMessages converts buf messages to our internal Issue format
func (l *ProtobufLinter) convertBufMessages(messages []BufMessage, filePath string) []linters.Issue {
	var issues []linters.Issue

	for _, msg := range messages {
		// Only include messages for the current file
		if msg.Path != filePath && !strings.HasSuffix(filePath, msg.Path) {
			continue
		}

		severity := "warning"
		if strings.Contains(strings.ToLower(msg.Type), "error") {
			severity = "error"
		}

		issue := linters.Issue{
			File:     filePath,
			Line:     msg.StartLine,
			Column:   msg.StartColumn,
			Severity: severity,
			Message:  msg.Message,
			Rule:     msg.Type,
		}
		issues = append(issues, issue)
	}

	return issues
}

// convertProtolintMessages converts protolint messages to our internal Issue format
func (l *ProtobufLinter) convertProtolintMessages(messages []ProtolintMessage, filePath string) []linters.Issue {
	var issues []linters.Issue

	for _, msg := range messages {
		// Only include messages for the current file
		if msg.Filename != filePath && !strings.HasSuffix(filePath, msg.Filename) {
			continue
		}

		issue := linters.Issue{
			File:     filePath,
			Line:     msg.Line,
			Column:   msg.Column,
			Severity: "warning",
			Message:  msg.Message,
			Rule:     msg.Rule,
		}
		issues = append(issues, issue)
	}

	return issues
}

// Lint performs linting on a Protocol Buffer file
func (l *ProtobufLinter) Lint(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Check file size limit
	if l.config.MaxFileSize != nil && int64(len(content)) > *l.config.MaxFileSize {
		return result, nil // Skip large files
	}

	// Determine which tool to use
	var toolsToTry []string
	if l.config.ForceTool != nil && *l.config.ForceTool != "" {
		toolsToTry = []string{*l.config.ForceTool}
	} else {
		toolsToTry = l.config.PreferredTools
	}

	// Try tools in order of preference
	for _, tool := range toolsToTry {
		switch tool {
		case "buf":
			if messages, err := l.runBuf(ctx, filePath); err == nil {
				issues := l.convertBufMessages(messages, filePath)
				result.Issues = append(result.Issues, issues...)

				// Check if any issues are errors
				for _, issue := range issues {
					if issue.Severity == "error" {
						result.Success = false
					}
				}
				return result, nil
			}

		case "protolint":
			if messages, err := l.runProtolint(ctx, filePath); err == nil {
				issues := l.convertProtolintMessages(messages, filePath)
				result.Issues = append(result.Issues, issues...)
				return result, nil
			}

		case "protoc":
			if err := l.runProtoc(ctx, filePath); err != nil {
				// Only mark as failure if it's a syntax error, not a tool availability issue
				if !strings.Contains(err.Error(), "not found") {
					result.Success = false
				}
				result.Issues = append(result.Issues, linters.Issue{
					File:     filePath,
					Line:     1,
					Column:   1,
					Severity: "error",
					Message:  err.Error(),
					Rule:     "syntax",
				})
			}
			return result, nil
		}
	}

	// If no tools are available, report a warning
	result.Issues = append(result.Issues, linters.Issue{
		File:     filePath,
		Line:     1,
		Column:   1,
		Severity: "warning",
		Message:  "No protobuf linting tools available (buf, protolint, or protoc)",
		Rule:     "tool-availability",
	})

	return result, nil
}
