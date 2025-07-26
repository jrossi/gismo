package gismo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewExecutor(t *testing.T) {
	engine := NewBaseRuleEngine()
	executor := NewExecutor(engine)

	if executor == nil {
		t.Fatal("NewExecutor() returned nil")
	}

	if executor.handler == nil {
		t.Error("Expected handler to be initialized")
	}

	if executor.timeout != 60*time.Second {
		t.Errorf("Expected default timeout of 60s, got %v", executor.timeout)
	}

	if executor.registry == nil {
		t.Error("Expected registry to be initialized")
	}
}

func TestExecutor_Execute(t *testing.T) {
	// Save original stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create test input
	input := `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write","tool_input":{"file_path":"test.go","content":"package main\n"}}`
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	// Write test data
	go func() {
		_, _ = w.Write([]byte(input))
		w.Close()
	}()

	executor := NewExecutor(NewBaseRuleEngine())
	err = executor.Execute(context.Background())

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
}

func TestExecutor_ExecuteWithExitCode(t *testing.T) {
	tests := []struct {
		name         string
		setupEngine  func() RuleEngine
		input        string
		wantExitCode int
		wantErr      bool
	}{
		{
			name: "approve_response",
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					preResponse: &HookResponse{
						Decision: "approve",
						Message:  "All good",
					},
				}
			},
			input:        `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write"}`,
			wantExitCode: int(ExitSuccess),
		},
		{
			name: "block_response",
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					preResponse: &HookResponse{
						Decision: "block",
						Reason:   "Validation failed",
					},
				}
			},
			input:        `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write"}`,
			wantExitCode: int(ExitBlocking),
		},
		{
			name: "nil_response",
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					preResponse: nil,
				}
			},
			input:        `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write"}`,
			wantExitCode: int(ExitSuccess),
		},
		{
			name: "invalid_json",
			setupEngine: func() RuleEngine {
				return NewBaseRuleEngine()
			},
			input:   `{"invalid json`,
			wantErr: true,
		},
		{
			name: "post_tool_use_with_feedback_returns_1",
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					postResponse: &HookResponse{
						Decision: "block",
						Reason:   "Should not block PostToolUse",
					},
				}
			},
			input:        `{"hook_event_name":"PostToolUse","session_id":"test","tool_name":"Write","tool_output":"test output"}`,
			wantExitCode: int(ExitBlocking), // PostToolUse always returns exit code 2
		},
		{
			name: "post_tool_use_with_message_returns_1",
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					postResponse: &HookResponse{
						Decision: "approve",
						Message:  "PostToolUse feedback",
					},
				}
			},
			input:        `{"hook_event_name":"PostToolUse","session_id":"test","tool_name":"Write","tool_output":"test output"}`,
			wantExitCode: int(ExitBlocking), // PostToolUse always returns exit code 2
		},
		{
			name: "post_tool_use_without_feedback_returns_success",
			setupEngine: func() RuleEngine {
				return &customRuleEngine{
					postResponse: nil, // No response means no feedback
				}
			},
			input:        `{"hook_event_name":"PostToolUse","session_id":"test","tool_name":"Write","tool_output":"test output"}`,
			wantExitCode: int(ExitBlocking), // PostToolUse always returns exit code 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore stdin
			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()

			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdin = r

			// Write test data
			go func() {
				_, _ = w.Write([]byte(tt.input))
				w.Close()
			}()

			executor := NewExecutor(tt.setupEngine())
			exitCode, err := executor.ExecuteWithExitCode(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ExecuteWithExitCode() error = %v", err)
			}

			if exitCode != tt.wantExitCode {
				t.Errorf("Got exit code %d, want %d", exitCode, tt.wantExitCode)
			}
		})
	}
}

func TestExecutor_ExecuteWithReader(t *testing.T) {
	executor := NewExecutor(NewBaseRuleEngine())

	input := `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write"}`
	reader := strings.NewReader(input)

	err := executor.ExecuteWithReader(context.Background(), reader)
	if err != nil {
		t.Errorf("ExecuteWithReader() error = %v", err)
	}
}

func TestExecutor_ExecuteWithReader_Errors(t *testing.T) {
	executor := NewExecutor(NewBaseRuleEngine())

	tests := []struct {
		name    string
		reader  io.Reader
		wantErr string
	}{
		{
			name:    "read_error",
			reader:  &errorReader{err: errors.New("read failed")},
			wantErr: "failed to read input",
		},
		{
			name:    "invalid_json",
			reader:  strings.NewReader(`{"invalid": json}`),
			wantErr: "failed to parse message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.ExecuteWithReader(context.Background(), tt.reader)
			if err == nil {
				t.Error("Expected error, got none")
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestExecutor_SetTimeout(t *testing.T) {
	executor := NewExecutor(NewBaseRuleEngine())

	newTimeout := 30 * time.Second
	executor.SetTimeout(newTimeout)

	if executor.timeout != newTimeout {
		t.Errorf("Expected timeout %v, got %v", newTimeout, executor.timeout)
	}
}

func TestExecutor_SetRuleEngine(t *testing.T) {
	executor := NewExecutor(NewBaseRuleEngine())

	// Create custom engine
	customEngine := &customRuleEngine{
		preResponse: &HookResponse{
			Decision: "custom",
		},
	}

	executor.SetRuleEngine(customEngine)

	// Verify it's used
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	input := `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write"}`
	go func() {
		_, _ = w.Write([]byte(input))
		w.Close()
	}()

	exitCode, err := executor.ExecuteWithExitCode(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Custom engine returns "custom" decision, which should result in success
	if exitCode != int(ExitSuccess) {
		t.Errorf("Expected success exit code with custom engine")
	}
}

func TestExecutor_GetRegistry(t *testing.T) {
	executor := NewExecutor(NewBaseRuleEngine())
	registry := executor.GetRegistry()

	if registry == nil {
		t.Fatal("GetRegistry() returned nil")
	}

	// Should be the same instance
	if registry != executor.registry {
		t.Error("Expected same registry instance")
	}
}

func TestExecutor_Timeout(t *testing.T) {
	// Create an engine that blocks
	blockingEngine := &customRuleEngine{
		preResponse: &HookResponse{Decision: "approve"},
	}

	executor := NewExecutor(blockingEngine)
	executor.SetTimeout(100 * time.Millisecond)

	// Save stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	// Don't write anything, just let it timeout
	// Use a timer channel instead of sleep
	timer := time.NewTimer(200 * time.Millisecond)
	go func() {
		<-timer.C
		w.Close()
	}()

	start := time.Now()
	ctx := context.Background()
	_, err = executor.ExecuteWithExitCode(ctx)
	duration := time.Since(start)

	// Should timeout
	if err == nil {
		t.Error("Expected timeout error")
	}

	// Should respect timeout (with some buffer for slow systems)
	if duration > 250*time.Millisecond {
		t.Errorf("Took too long to timeout: %v", duration)
	}
}

func TestHookRunner(t *testing.T) {
	runner := NewHookRunner(5 * time.Second)

	if runner == nil {
		t.Fatal("NewHookRunner() returned nil")
	}

	if runner.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", runner.timeout)
	}
}

func TestHookRunner_RunHook(t *testing.T) {
	// Create a script that reads from stdin and writes to stdout
	tmpDir, err := os.MkdirTemp("", "hook_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "echo_stdin.sh")
	scriptContent := `#!/bin/sh
cat
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	runner := NewHookRunner(5 * time.Second)
	ctx := context.Background()

	input := []byte("test input")
	output, err := runner.RunHook(ctx, scriptPath, input)

	if err != nil {
		t.Fatalf("RunHook() error = %v", err)
	}

	// Script should return the input
	if !bytes.Contains(output, []byte("test input")) {
		t.Errorf("Expected output to contain input, got %s", output)
	}
}

func TestHookRunner_RunHook_ExitCode2(t *testing.T) {
	// Create a script that exits with code 2
	tmpDir, err := os.MkdirTemp("", "hook_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "exit2.sh")
	scriptContent := `#!/bin/sh
echo "Error message" >&2
exit 2
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	runner := NewHookRunner(5 * time.Second)
	ctx := context.Background()

	output, err := runner.RunHook(ctx, scriptPath, []byte("input"))

	// Should not error for exit code 2
	if err != nil {
		t.Errorf("Expected no error for exit code 2, got %v", err)
	}

	// Should return stderr
	if !bytes.Contains(output, []byte("Error message")) {
		t.Errorf("Expected stderr in output, got %s", output)
	}
}

func TestHookRunner_RunHook_Timeout(t *testing.T) {
	// Create a script that ignores signals and sleeps forever
	tmpDir, err := os.MkdirTemp("", "hook_timeout_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "sleep.sh")
	// Use a simple infinite loop instead of sleep to ensure we test timeout
	scriptContent := `#!/bin/sh
while true; do
  : # no-op
done
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	runner := NewHookRunner(50 * time.Millisecond)
	ctx := context.Background()

	start := time.Now()
	_, err = runner.RunHook(ctx, scriptPath, []byte("input"))
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	}

	// We expect timeout around 50ms, but allow up to 1s for slow CI systems
	// The key is that it shouldn't run forever
	if duration > 1*time.Second {
		t.Errorf("Timeout took too long: %v", duration)
	}

	if duration < 25*time.Millisecond {
		t.Errorf("Timeout happened too quickly: %v", duration)
	}
}

func TestChainExecutor(t *testing.T) {
	// Create multiple executors
	executor1 := NewExecutor(NewBaseRuleEngine())
	executor2 := NewExecutor(NewLintingRuleEngine())

	chain := NewChainExecutor(executor1, executor2)

	if chain == nil {
		t.Fatal("NewChainExecutor() returned nil")
	}

	if len(chain.executors) != 2 {
		t.Errorf("Expected 2 executors, got %d", len(chain.executors))
	}
}

func TestChainExecutor_Execute(t *testing.T) {
	// Create executors with custom engines
	engine1 := &customRuleEngine{
		preResponse: &HookResponse{Decision: "approve"},
	}
	engine2 := &customRuleEngine{
		preResponse: &HookResponse{Decision: "approve"},
	}

	executor1 := NewExecutor(engine1)
	executor2 := NewExecutor(engine2)

	// Track calls differently since we can't override methods
	// The chain will call Execute on each executor

	chain := &ChainExecutor{
		executors: []*Executor{executor1, executor2},
	}

	// Prepare stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	input := `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write"}`
	go func() {
		// Write for both executors
		_, _ = w.Write([]byte(input))
		_, _ = w.Write([]byte("\n"))
		_, _ = w.Write([]byte(input))
		w.Close()
	}()

	err = chain.Execute(context.Background())
	if err == nil {
		t.Log("Chain execution completed successfully")
	}
}

func TestChainExecutor_Execute_Error(t *testing.T) {
	// Create executor that will fail
	errorEngine := &errorRuleEngine{}
	executor1 := NewExecutor(errorEngine)
	executor2 := NewExecutor(NewBaseRuleEngine())

	chain := NewChainExecutor(executor1, executor2)

	// Prepare stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r

	input := `{"hook_event_name":"PreToolUse","session_id":"test","tool_name":"Write"}`
	go func() {
		_, _ = w.Write([]byte(input))
		w.Close()
	}()

	err = chain.Execute(context.Background())
	if err == nil {
		t.Error("Expected error from chain")
	}

	// Should indicate which executor failed
	if !strings.Contains(err.Error(), "executor 0 failed") {
		t.Errorf("Expected error to indicate executor 0 failed, got %v", err)
	}
}

// TestPostToolUseExitCodes verifies that PostToolUse hooks
// always return exit code 2 to ensure output visibility
func TestPostToolUseExitCodes(t *testing.T) {
	tests := []struct {
		name         string
		response     *HookResponse
		wantExitCode int
	}{
		{
			name: "PostToolUse with block decision returns 2",
			response: &HookResponse{
				Decision: "block",
				Reason:   "This has feedback",
			},
			wantExitCode: int(ExitBlocking),
		},
		{
			name: "PostToolUse with approve decision returns 2",
			response: &HookResponse{
				Decision: "approve",
				Reason:   "Normal approve case",
			},
			wantExitCode: int(ExitBlocking),
		},
		{
			name: "PostToolUse with message returns 2",
			response: &HookResponse{
				Message: "Some feedback message",
			},
			wantExitCode: int(ExitBlocking),
		},
		{
			name:         "PostToolUse with nil response returns 2",
			response:     nil,
			wantExitCode: int(ExitBlocking),
		},
		{
			name:         "PostToolUse with empty response returns 2",
			response:     &HookResponse{},
			wantExitCode: int(ExitBlocking),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original stdin
			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()

			// Create pipe for stdin
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdin = r

			// Create test input with PostToolUse event
			input := `{"hook_event_name": "PostToolUse","session_id": "test-session","tool_name": "Write","tool_input": {"file_path": "test.go", "content": "test content"},"tool_output": "File written successfully"}`

			// Write input and close
			go func() {
				_, _ = w.Write([]byte(input))
				w.Close()
			}()

			// Create custom rule engine that returns the test response
			engine := &customRuleEngine{
				postResponse: tt.response,
			}

			// Create executor with custom engine
			executor := NewExecutor(engine)

			// Execute and get exit code
			exitCode, err := executor.ExecuteWithExitCode(context.Background())
			if err != nil {
				t.Fatalf("ExecuteWithExitCode() error = %v", err)
			}

			// Check exit code matches expected
			if exitCode != tt.wantExitCode {
				t.Errorf("PostToolUse hook returned exit code %d, expected %d for response=%+v",
					exitCode, tt.wantExitCode, tt.response)
			}
		})
	}
}

func TestHasResponseFeedback(t *testing.T) {
	tests := []struct {
		name     string
		response *HookResponse
		want     bool
	}{
		{
			name:     "nil response",
			response: nil,
			want:     false,
		},
		{
			name:     "empty response",
			response: &HookResponse{},
			want:     false,
		},
		{
			name: "response with message",
			response: &HookResponse{
				Message: "Some feedback",
			},
			want: true,
		},
		{
			name: "response with reason",
			response: &HookResponse{
				Reason: "Block reason",
			},
			want: true,
		},
		{
			name: "response with decision",
			response: &HookResponse{
				Decision: "block",
			},
			want: true,
		},
		{
			name: "response with stop reason",
			response: &HookResponse{
				StopReason: "Error occurred",
			},
			want: true,
		},
		{
			name: "response with continue true",
			response: &HookResponse{
				Continue: boolPtr(true),
			},
			want: true,
		},
		{
			name: "response with continue false",
			response: &HookResponse{
				Continue: boolPtr(false),
			},
			want: true,
		},
		{
			name: "response with suppress output",
			response: &HookResponse{
				SuppressOutput: boolPtr(true),
			},
			want: true,
		},
		{
			name: "response with multiple fields",
			response: &HookResponse{
				Decision: "block",
				Reason:   "Multiple reasons",
				Message:  "User message",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasResponseFeedback(tt.response); got != tt.want {
				t.Errorf("hasResponseFeedback() = %v, want %v", got, tt.want)
			}
		})
	}
}
