package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrossi/gismo/pkg/engine"
)

// getCcfeedbackPath finds the gismo binary for testing
func getCcfeedbackPath(t *testing.T) string {
	// Try current directory first (for local builds)
	if _, err := os.Stat("../../gismo"); err == nil {
		return "../../gismo"
	}

	// Fall back to PATH
	gismoPath, err := exec.LookPath("gismo")
	if err != nil {
		t.Skip("gismo binary not found in current directory or PATH")
	}
	return gismoPath
}

func TestShowActionsCommand(t *testing.T) {
	gismoPath := getCcfeedbackPath(t)

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := `package test

import "fmt"

func HelloWorld() {
	fmt.Println("Hello, World!")
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Run gismo show-actions command
	cmd := exec.Command(gismoPath, "show-actions", testFile)
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify output contains expected sections
	outputStr := string(output)
	expectedSections := []string{
		"=== Configuration Analysis for:",
		"--- Applicable Linters ---",
		"--- Base Configuration for",
		"--- Rule Hierarchy ---",
		"--- Final Configuration for",
	}

	for _, section := range expectedSections {
		if !strings.Contains(outputStr, section) {
			t.Errorf("Output missing expected section: %s\nFull output:\n%s", section, outputStr)
		}
	}
}

func TestShowActionsCommand_MultipleFiles(t *testing.T) {
	gismoPath := getCcfeedbackPath(t)

	// Create temporary test files
	tmpDir := t.TempDir()

	// Go file
	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package test

func Add(a, b int) int {
	return a + b
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create go file: %v", err)
	}

	// Markdown file
	mdFile := filepath.Join(tmpDir, "README.md")
	mdContent := `# Test Project

This is a test project.
`
	if err := os.WriteFile(mdFile, []byte(mdContent), 0644); err != nil {
		t.Fatalf("Failed to create markdown file: %v", err)
	}

	// Run gismo show-actions command with multiple files
	// Note: show filter only processes the first file
	cmd := exec.Command(gismoPath, "show-actions", goFile, mdFile)
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify first file is analyzed (show filter only processes one file)
	outputStr := string(output)
	if !strings.Contains(outputStr, "test.go") {
		t.Error("Output should contain analysis for test.go")
	}
	// Second file is ignored by show filter
}

func TestShowActionsCommand_NoFiles(t *testing.T) {
	gismoPath := getCcfeedbackPath(t)

	// Run gismo show-actions without files
	cmd := exec.Command(gismoPath, "show-actions")
	output, err := cmd.CombinedOutput()

	// Should fail with exit code 1
	if err == nil {
		t.Error("Expected command to fail when no files provided")
	}

	// Should show error message
	outputStr := string(output)
	if !strings.Contains(outputStr, "show-actions requires at least one file path") {
		t.Errorf("Expected error message in output, got: %s", outputStr)
	}
}

func TestVersionFlag(t *testing.T) {
	gismoPath := getCcfeedbackPath(t)

	// Run gismo --version
	cmd := exec.Command(gismoPath, "--version")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Should contain version info
	outputStr := string(output)
	if !strings.Contains(outputStr, "gismo version") {
		t.Errorf("Expected version output, got: %s", outputStr)
	}
}

func TestUsageFlag(t *testing.T) {
	gismoPath := getCcfeedbackPath(t)

	// Run gismo --help (which triggers usage)
	cmd := exec.Command(gismoPath, "--help")
	output, err := cmd.CombinedOutput()

	// --help causes exit code 2 by default
	if err != nil && !strings.Contains(err.Error(), "exit status 2") {
		t.Fatalf("Unexpected error: %v\nOutput: %s", err, output)
	}

	// Should contain usage info
	outputStr := string(output)
	expectedStrings := []string{
		"CCFeedback - Claude Code Hooks Feedback System",
		"Usage:",
		"Commands:",
		"Flags:",
		"Exit codes:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("Usage output missing: %s\nFull output:\n%s", expected, outputStr)
		}
	}
}

func TestDefaultBehavior_ValidInput(t *testing.T) {
	gismoPath := getCcfeedbackPath(t)

	// Create valid hook message
	input := engine.PostToolUseMessage{
		BaseHookMessage: engine.BaseHookMessage{
			HookEventName: engine.PostToolUseEvent,
			SessionID:     "test-session",
		},
		ToolName:   "Write",
		ToolOutput: json.RawMessage(`{"result":"success"}`),
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	// Run gismo with input on stdin
	cmd := exec.Command(gismoPath)
	cmd.Stdin = bytes.NewReader(inputJSON)
	cmd.Env = append(os.Environ(), "CCFEEDBACK_NO_DAEMON=1")
	output, err := cmd.CombinedOutput()

	// PostToolUse always exits with code 2 (blocking) to ensure output is visible
	if err == nil {
		t.Fatal("Expected exit code 2 for PostToolUse, but command succeeded")
	}

	// Check for exit code 2
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 2 {
			t.Fatalf("Expected exit code 2, got %d\nOutput: %s", exitErr.ExitCode(), output)
		}
	} else {
		t.Fatalf("Unexpected error type: %v", err)
	}

	// Should have minimal output (no errors)
	outputStr := string(output)
	if strings.Contains(outputStr, "error") || strings.Contains(outputStr, "Error") {
		t.Errorf("Unexpected error in output: %s", outputStr)
	}
}

func TestDefaultBehavior_InvalidJSON(t *testing.T) {
	gismoPath := getCcfeedbackPath(t)

	// Create invalid JSON input
	input := []byte(`{invalid json}`)

	// Run gismo with invalid input
	cmd := exec.Command(gismoPath)
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.CombinedOutput()

	// Should fail with exit code 1
	if err == nil {
		t.Error("Expected command to fail for invalid JSON")
	}

	// Should contain error message
	outputStr := string(output)
	if !strings.Contains(outputStr, "Hook execution error") {
		t.Errorf("Expected error message in output, got: %s", outputStr)
	}
}

func TestConfigFileFlag(t *testing.T) {
	gismoPath := getCcfeedbackPath(t)

	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.json")
	configContent := `{
		"version": "1.0",
		"linters": {
			"golang": {
				"enabled": true,
				"config": {
					"args": ["--fast"]
				}
			}
		}
	}`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create a test Go file
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := `package test

func Test() {}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set env var to disable daemon mode for this test
	os.Setenv("CCFEEDBACK_NO_DAEMON", "1")
	defer os.Unsetenv("CCFEEDBACK_NO_DAEMON")

	// First build the show binary if needed
	showBinaryPath := filepath.Join(filepath.Dir(gismoPath), "gismo-show")
	if _, err := os.Stat(showBinaryPath); os.IsNotExist(err) {
		// Try to build it
		buildCmd := exec.Command("go", "build", "-o", showBinaryPath, "./cmd/gismo-show")
		if err := buildCmd.Run(); err != nil {
			t.Skip("Skipping test: gismo-show binary not available")
		}
	}

	// Verify config file exists and read its contents
	if _, err := os.Stat(configFile); err != nil {
		t.Fatalf("Config file doesn't exist: %v", err)
	}
	configData, _ := os.ReadFile(configFile)
	t.Logf("Config file contents: %s", string(configData))

	// Run with config flag - config flag should be before the subcommand
	cmd := exec.Command(gismoPath, "--debug", "--config", configFile, "show-actions", testFile)
	cmd.Env = append(os.Environ(), "CCFEEDBACK_DEBUG=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := stdout.Bytes()

	t.Logf("Command: %v", cmd.Args)
	t.Logf("Config file path: %s", configFile)
	if _, err := os.Stat(configFile); err == nil {
		t.Logf("Config file exists: true")
	} else {
		t.Logf("Config file exists: false (err: %v)", err)
	}
	t.Logf("Stdout: %s", output)
	t.Logf("Stderr: %s", stderr.String())

	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Should use the specified config
	outputStr := string(output)
	// Look for the args configuration in the output
	if !strings.Contains(outputStr, "args:") || !strings.Contains(outputStr, "--fast") {
		t.Errorf("Config not applied, expected 'args: [--fast]' in output:\n%s", outputStr)
	}
}

func TestTimeoutFlag(t *testing.T) {
	gismoPath := getCcfeedbackPath(t)

	// Test with custom timeout
	cmd := exec.Command(gismoPath, "--timeout", "5s", "--version")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Should still work with custom timeout
	if !strings.Contains(string(output), "gismo version") {
		t.Error("Version command should work with timeout flag")
	}
}

func TestDebugFlag(t *testing.T) {
	gismoPath := getCcfeedbackPath(t)

	// Create invalid JSON input to trigger error
	input := []byte(`{invalid json}`)

	// Run with debug flag
	cmd := exec.Command(gismoPath, "--debug")
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.CombinedOutput()

	// Should fail but with debug output
	if err == nil {
		t.Error("Expected command to fail for invalid JSON")
	}

	// Should contain debug information
	outputStr := string(output)
	if !strings.Contains(outputStr, "Debug: Full error:") {
		t.Errorf("Expected debug output, got: %s", outputStr)
	}
}
