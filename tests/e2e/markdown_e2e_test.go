package e2e_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
)

func TestE2E_MarkdownHookIntegration(t *testing.T) {
	// Build binary first
	binPath := buildTestBinary(t)
	defer os.Remove(binPath)

	tests := []struct {
		name           string
		toolName       string
		filePath       string
		content        string
		expectBlocking bool
		expectApproval bool
	}{
		{
			name:     "good_markdown",
			toolName: "Write",
			filePath: "/tmp/good_test.md",
			content: `# Good Document

This is well-formatted markdown.

## Section

- Item 1
  - Nested item
- Item 2

` + "```go" + `
fmt.Println("Hello")
` + "```",
			expectBlocking: false,
			expectApproval: true,
		},
		{
			name:     "bad_markdown",
			toolName: "Write",
			filePath: "/tmp/bad_test.md",
			content: `# Bad Document

This line has trailing whitespace.   

##### Skipped heading levels

- Item
   - Wrong indentation`,
			expectBlocking: true,
			expectApproval: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create hook message
			msg := map[string]interface{}{
				"session_id":      "test-session",
				"transcript_path": "/tmp/test-transcript",
				"hook_event_name": "PreToolUse",
				"tool_name":       tt.toolName,
				"tool_input": map[string]interface{}{
					"file_path": tt.filePath,
					"content":   tt.content,
				},
			}

			msgBytes, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("Failed to marshal message: %v", err)
			}

			// Execute hook binary
			cmd := exec.Command(binPath)
			cmd.Stdin = bytes.NewReader(msgBytes)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			_ = cmd.Run()
			exitCode := cmd.ProcessState.ExitCode()

			t.Logf("Exit code: %d", exitCode)
			t.Logf("Stdout: %s", stdout.String())
			t.Logf("Stderr: %s", stderr.String())

			if tt.expectBlocking {
				// Should exit with code 2 (blocking error)
				if exitCode != 2 {
					t.Errorf("Expected exit code 2 for blocking, got %d", exitCode)
				}

				// Should have response indicating block
				var response map[string]interface{}
				if err := json.Unmarshal(stdout.Bytes(), &response); err == nil {
					if decision, ok := response["decision"].(string); ok {
						if decision != "block" {
							t.Errorf("Expected block decision, got %s", decision)
						}
					}
				}

				// Should have stderr output with details
				if stderr.Len() == 0 {
					t.Error("Expected stderr output for blocking case")
				}
			} else if tt.expectApproval {
				// Should exit with code 0 (success)
				if exitCode != 0 {
					t.Errorf("Expected exit code 0 for approval, got %d", exitCode)
				}

				// Should have response indicating approval
				var response map[string]interface{}
				if err := json.Unmarshal(stdout.Bytes(), &response); err == nil {
					if decision, ok := response["decision"].(string); ok {
						if decision != "approve" {
							t.Errorf("Expected approve decision, got %s", decision)
						}
					}
				}
			}
		})
	}
}

func TestE2E_MarkdownHookErrorHandling(t *testing.T) {
	binPath := buildTestBinary(t)
	defer os.Remove(binPath)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "invalid_json",
			input:   `{"invalid": json}`,
			wantErr: true,
		},
		{
			name:    "missing_hook_event",
			input:   `{"tool_name": "Write"}`,
			wantErr: true,
		},
		{
			name:    "unknown_event_type",
			input:   `{"hook_event_name": "UnknownEvent", "tool_name": "Write"}`,
			wantErr: true,
		},
		{
			name:    "empty_input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binPath)
			cmd.Stdin = strings.NewReader(tt.input)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			_ = cmd.Run()
			exitCode := cmd.ProcessState.ExitCode()

			if tt.wantErr {
				if exitCode == 0 {
					t.Errorf("Expected non-zero exit code for error case, got 0")
				}
			}

			t.Logf("Exit code: %d", exitCode)
			t.Logf("Stderr: %s", stderr.String())
		})
	}
}

func TestE2E_MarkdownHookTimeout(t *testing.T) {
	binPath := buildTestBinary(t)
	defer os.Remove(binPath)

	// Create very large content to potentially trigger timeout
	largeContent := "# Large Document\n\n"
	for i := 0; i < 10000; i++ {
		largeContent += "## Section " + string(rune(i)) + "\n\nContent here.\n\n"
	}

	msg := map[string]interface{}{
		"session_id":      "test-session",
		"transcript_path": "/tmp/test-transcript",
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input": map[string]interface{}{
			"file_path": "/tmp/large_test.md",
			"content":   largeContent,
		},
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Execute with short timeout
	cmd := exec.Command(binPath, "--timeout", "100ms")
	cmd.Stdin = bytes.NewReader(msgBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	_ = cmd.Run()
	duration := time.Since(start)

	t.Logf("Command completed in %v", duration)
	t.Logf("Exit code: %d", cmd.ProcessState.ExitCode())

	// Should complete reasonably quickly even with short timeout
	// Allow more time in CI environments which can be slower
	if duration > 30*time.Second {
		t.Errorf("Command took too long: %v", duration)
	}
}

func TestE2E_MarkdownHookRealFiles(t *testing.T) {
	binPath := buildTestBinary(t)
	defer os.Remove(binPath)

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "e2e_markdown_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with real file paths
	testFile := filepath.Join(tmpDir, "test.md")
	content := `# Real File Test

This line has trailing spaces.   

##### Skipped levels`

	msg := map[string]interface{}{
		"session_id":      "test-session",
		"transcript_path": "/tmp/test-transcript",
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input": map[string]interface{}{
			"file_path": testFile,
			"content":   content,
		},
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(msgBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()
	exitCode := cmd.ProcessState.ExitCode()

	// Should block due to errors
	if exitCode != 2 {
		t.Errorf("Expected exit code 2, got %d", exitCode)
	}

	// Should have detailed error output
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "trailing whitespace") {
		t.Errorf("Expected trailing whitespace error in stderr: %s", stderrStr)
	}
	if !strings.Contains(stderrStr, "skips level") {
		t.Errorf("Expected heading hierarchy error in stderr: %s", stderrStr)
	}
}

func TestE2E_MarkdownHookDebugMode(t *testing.T) {
	binPath := buildTestBinary(t)
	defer os.Remove(binPath)

	content := `# Debug Test

Content with no issues.`

	msg := map[string]interface{}{
		"session_id":      "test-session",
		"transcript_path": "/tmp/test-transcript",
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input": map[string]interface{}{
			"file_path": "/tmp/debug_test.md",
			"content":   content,
		},
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Test with debug flag
	cmd := exec.Command(binPath, "--debug")
	cmd.Stdin = bytes.NewReader(msgBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()
	exitCode := cmd.ProcessState.ExitCode()

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	t.Logf("Debug output: %s", stderr.String())
}

func TestE2E_MarkdownHookVersionAndHelp(t *testing.T) {
	binPath := buildTestBinary(t)
	defer os.Remove(binPath)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "version_flag",
			args: []string{"--version"},
		},
		{
			name: "help_flag",
			args: []string{"--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binPath, tt.args...)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			_ = cmd.Run()

			// These should exit successfully
			if cmd.ProcessState.ExitCode() != 0 {
				t.Errorf("Expected exit code 0 for %s, got %d", tt.name, cmd.ProcessState.ExitCode())
			}

			// Should have some output
			if stdout.Len() == 0 && stderr.Len() == 0 {
				t.Errorf("Expected some output for %s", tt.name)
			}

			t.Logf("%s output: %s%s", tt.name, stdout.String(), stderr.String())
		})
	}
}
