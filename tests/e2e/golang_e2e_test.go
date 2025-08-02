package e2e_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
)

func TestE2E_GolangHookIntegration(t *testing.T) {
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
			name:     "good_golang",
			toolName: "Write",
			filePath: "/tmp/good_test.go",
			content: `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`,
			expectBlocking: false,
			expectApproval: true,
		},
		{
			name:     "bad_golang_formatting",
			toolName: "Write",
			filePath: "/tmp/bad_test.go",
			content: `package main

import "fmt"

func main(){
fmt.Println("Bad formatting")
}`,
			expectBlocking: false, // Formatting issues are warnings
			expectApproval: true,
		},
		{
			name:     "golang_syntax_error",
			toolName: "Write",
			filePath: "/tmp/syntax_error.go",
			content: `package main

func main() {
	fmt.Println("Missing import"
}`,
			expectBlocking: true, // Syntax errors are caught and blocked
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

func TestE2E_GolangHookErrorHandling(t *testing.T) {
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

func TestE2E_GolangHookTimeout(t *testing.T) {
	binPath := buildTestBinary(t)
	defer os.Remove(binPath)

	// Create very large content to potentially trigger timeout
	largeContent := "package main\n\nimport \"fmt\"\n\n"
	for i := 0; i < 10000; i++ {
		largeContent += "func Function" + string(rune(i)) + "() {\n\tfmt.Println(\"Test\")\n}\n\n"
	}

	msg := map[string]interface{}{
		"session_id":      "test-session",
		"transcript_path": "/tmp/test-transcript",
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input": map[string]interface{}{
			"file_path": "/tmp/large_test.go",
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

func TestE2E_GolangHookDebugMode(t *testing.T) {
	binPath := buildTestBinary(t)
	defer os.Remove(binPath)

	content := `package main

import "fmt"

func main() {
	fmt.Println("Debug test")
}`

	msg := map[string]interface{}{
		"session_id":      "test-session",
		"transcript_path": "/tmp/test-transcript",
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input": map[string]interface{}{
			"file_path": "/tmp/debug_test.go",
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

func TestE2E_GolangHookVersionAndHelp(t *testing.T) {
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
