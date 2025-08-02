package protobuf

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProtobufLinter_Name(t *testing.T) {
	linter := NewProtobufLinter()
	if got := linter.Name(); got != "protobuf" {
		t.Errorf("Name() = %v, want %v", got, "protobuf")
	}
}

func TestProtobufLinter_CanHandle(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "proto file",
			filePath: "api/v1/service.proto",
			want:     true,
		},
		{
			name:     "proto file with path",
			filePath: "/home/user/project/protos/service.proto",
			want:     true,
		},
		{
			name:     "go file",
			filePath: "main.go",
			want:     false,
		},
		{
			name:     "python file",
			filePath: "script.py",
			want:     false,
		},
		{
			name:     "no extension",
			filePath: "README",
			want:     false,
		},
		{
			name:     "protobuf text format",
			filePath: "config.textproto",
			want:     false,
		},
	}

	linter := NewProtobufLinter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linter.CanHandle(tt.filePath); got != tt.want {
				t.Errorf("CanHandle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProtobufLinter_Lint_NoTools(t *testing.T) {
	// This test verifies behavior when no protobuf tools are available
	config := &ProtobufConfig{
		PreferredTools: []string{"nonexistent-tool"},
	}
	linter := NewProtobufLinterWithConfig(config)

	ctx := context.Background()
	content := []byte(`syntax = "proto3";`)
	result, err := linter.Lint(ctx, "test.proto", content)

	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	if !result.Success {
		t.Error("Expected success = true when no tools available")
	}

	if len(result.Issues) != 1 {
		t.Fatalf("Expected 1 issue for no tools available, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.Rule != "tool-availability" {
		t.Errorf("Expected rule = tool-availability, got %s", issue.Rule)
	}
}

func TestProtobufLinter_Lint_ValidProto(t *testing.T) {
	// Skip if buf is not available
	if _, err := exec.LookPath("buf"); err != nil {
		t.Skip("buf not found in PATH, skipping test")
	}

	linter := NewProtobufLinter()
	ctx := context.Background()

	testFile := filepath.Join("testdata", "valid.proto")
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	result, err := linter.Lint(ctx, testFile, content)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// If we only get a tool availability warning, that's still considered success
	if !result.Success && len(result.Issues) > 0 && result.Issues[0].Rule != "tool-availability" {
		t.Error("Expected success = true for valid proto file")
	}

	// Valid proto should have no issues (assuming default buf config)
	if len(result.Issues) > 0 {
		t.Logf("Found %d issues in valid proto:", len(result.Issues))
		for _, issue := range result.Issues {
			t.Logf("  - %s:%d:%d %s (%s)", issue.File, issue.Line, issue.Column, issue.Message, issue.Rule)
		}
	}
}

func TestProtobufLinter_Lint_SyntaxError(t *testing.T) {
	// This test should work with any tool (buf, protolint, or protoc)
	linter := NewProtobufLinter()
	ctx := context.Background()

	testFile := filepath.Join("testdata", "syntax_error.proto")
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	result, err := linter.Lint(ctx, testFile, content)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// Syntax errors should cause failure with protoc
	if len(result.Issues) == 0 {
		t.Error("Expected issues for proto with syntax errors")
	}

	// Should have at least one error-level issue
	hasError := false
	for _, issue := range result.Issues {
		if issue.Severity == "error" || strings.Contains(issue.Message, "syntax") {
			hasError = true
			break
		}
	}

	if !hasError && !result.Success {
		t.Error("Expected at least one error-level issue for syntax errors")
	}
}

func TestProtobufLinter_Lint_StyleViolations(t *testing.T) {
	// Skip if buf is not available (style checks require buf or protolint)
	if _, err := exec.LookPath("buf"); err != nil {
		if _, err := exec.LookPath("protolint"); err != nil {
			t.Skip("Neither buf nor protolint found in PATH, skipping test")
		}
	}

	linter := NewProtobufLinter()
	ctx := context.Background()

	testFile := filepath.Join("testdata", "style_violations.proto")
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	result, err := linter.Lint(ctx, testFile, content)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// Style violations should produce warnings
	if len(result.Issues) == 0 {
		t.Error("Expected issues for proto with style violations")
	}

	// Log the issues for debugging
	t.Logf("Found %d style issues:", len(result.Issues))
	for _, issue := range result.Issues {
		t.Logf("  - %s:%d:%d %s (%s) [%s]", issue.File, issue.Line, issue.Column, issue.Message, issue.Rule, issue.Severity)
	}
}

func TestProtobufLinter_FindProtoWorkspace(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create workspace structure
	workspaceDir := filepath.Join(tmpDir, "workspace")
	projectDir := filepath.Join(workspaceDir, "project")
	subDir := filepath.Join(projectDir, "api", "v1")

	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create buf.yaml in project directory
	bufYaml := filepath.Join(projectDir, "buf.yaml")
	if err := os.WriteFile(bufYaml, []byte("version: v1\n"), 0644); err != nil {
		t.Fatalf("Failed to create buf.yaml: %v", err)
	}

	// Create buf.work.yaml in workspace directory
	bufWork := filepath.Join(workspaceDir, "buf.work.yaml")
	if err := os.WriteFile(bufWork, []byte("version: v1\ndirectories:\n  - project\n"), 0644); err != nil {
		t.Fatalf("Failed to create buf.work.yaml: %v", err)
	}

	linter := NewProtobufLinter()

	tests := []struct {
		name        string
		startPath   string
		wantRoot    string
		wantConfig  string
		isWorkspace bool
		wantErr     bool
	}{
		{
			name:        "from project root",
			startPath:   projectDir,
			wantRoot:    projectDir,
			wantConfig:  bufYaml,
			isWorkspace: false,
			wantErr:     false,
		},
		{
			name:        "from subdirectory",
			startPath:   subDir,
			wantRoot:    projectDir,
			wantConfig:  bufYaml,
			isWorkspace: false,
			wantErr:     false,
		},
		{
			name:        "from workspace root",
			startPath:   workspaceDir,
			wantRoot:    workspaceDir,
			wantConfig:  bufWork,
			isWorkspace: true,
			wantErr:     false,
		},
		{
			name:      "no config found",
			startPath: tmpDir,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := linter.FindProtoWorkspace(tt.startPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindProtoWorkspace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if info.Root != tt.wantRoot {
				t.Errorf("Root = %v, want %v", info.Root, tt.wantRoot)
			}
			if info.ConfigPath != tt.wantConfig {
				t.Errorf("ConfigPath = %v, want %v", info.ConfigPath, tt.wantConfig)
			}
			if info.IsWorkspace != tt.isWorkspace {
				t.Errorf("IsWorkspace = %v, want %v", info.IsWorkspace, tt.isWorkspace)
			}
		})
	}
}

func TestProtobufLinter_Configuration(t *testing.T) {
	tests := []struct {
		name   string
		config *ProtobufConfig
		check  func(t *testing.T, l *ProtobufLinter)
	}{
		{
			name:   "default config",
			config: nil,
			check: func(t *testing.T, l *ProtobufLinter) {
				if len(l.config.PreferredTools) != 3 {
					t.Errorf("Expected 3 preferred tools, got %d", len(l.config.PreferredTools))
				}
				if l.config.PreferredTools[0] != "buf" {
					t.Errorf("Expected first tool to be buf, got %s", l.config.PreferredTools[0])
				}
			},
		},
		{
			name: "custom config",
			config: &ProtobufConfig{
				PreferredTools: []string{"protolint", "protoc"},
				DisabledChecks: []string{"PACKAGE_VERSION_SUFFIX"},
				TestTimeout:    &Duration{Duration: 1 * time.Minute},
			},
			check: func(t *testing.T, l *ProtobufLinter) {
				if len(l.config.PreferredTools) != 2 {
					t.Errorf("Expected 2 preferred tools, got %d", len(l.config.PreferredTools))
				}
				if l.config.PreferredTools[0] != "protolint" {
					t.Errorf("Expected first tool to be protolint, got %s", l.config.PreferredTools[0])
				}
				if len(l.config.DisabledChecks) != 1 {
					t.Errorf("Expected 1 disabled check, got %d", len(l.config.DisabledChecks))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linter := NewProtobufLinterWithConfig(tt.config)
			tt.check(t, linter)
		})
	}
}

func TestProtobufLinter_SetConfig(t *testing.T) {
	linter := NewProtobufLinter()

	configJSON := []byte(`{
		"preferredTools": ["protoc"],
		"disabledChecks": ["CHECK1", "CHECK2"],
		"verbose": true
	}`)

	err := linter.SetConfig(configJSON)
	if err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	if len(linter.config.PreferredTools) != 1 || linter.config.PreferredTools[0] != "protoc" {
		t.Error("SetConfig did not update preferredTools correctly")
	}

	if len(linter.config.DisabledChecks) != 2 {
		t.Error("SetConfig did not update disabledChecks correctly")
	}

	if !linter.config.Verbose {
		t.Error("SetConfig did not update verbose correctly")
	}
}

func TestProtobufLinter_MaxFileSize(t *testing.T) {
	config := &ProtobufConfig{
		MaxFileSize: intPtr(100), // 100 bytes limit
	}
	linter := NewProtobufLinterWithConfig(config)

	ctx := context.Background()
	largeContent := make([]byte, 200) // 200 bytes, over limit

	result, err := linter.Lint(ctx, "large.proto", largeContent)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	if !result.Success {
		t.Error("Expected success = true for skipped large file")
	}

	if len(result.Issues) != 0 {
		t.Error("Expected no issues for skipped large file")
	}
}
