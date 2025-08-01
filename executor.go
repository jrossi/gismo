package gismo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Executor handles the execution of hooks and processing of responses
type Executor struct {
	handler  *Handler
	timeout  time.Duration
	registry *Registry
}

// NewExecutor creates a new hook executor
func NewExecutor(ruleEngine RuleEngine) *Executor {
	return &Executor{
		handler:  NewHandler(ruleEngine),
		timeout:  60 * time.Second, // Default 60 second timeout
		registry: NewRegistry(),
	}
}

// Execute runs the hook processing with the configured handler
func (e *Executor) Execute(ctx context.Context) error {
	_, err := e.ExecuteWithExitCode(ctx)
	return err
}

// ExecuteWithExitCode runs the hook processing and returns the appropriate exit code
func (e *Executor) ExecuteWithExitCode(ctx context.Context) (int, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Process the input and get the response
	response, err := e.handler.ProcessInputWithResponse(ctx)
	if err != nil {
		return 1, err
	}

	// Check if this is a PostToolUse hook by examining the handler's last processed message
	if e.handler.IsPostToolUseHook() {
		// For PostToolUse hooks, always return exit code 2 to ensure output is visible
		// This matches smart-lint.sh behavior
		return int(ExitBlocking), nil
	}

	// Determine exit code based on response
	if response != nil && response.Decision == "block" {
		return int(ExitBlocking), nil
	}

	return int(ExitSuccess), nil
}

// ExecuteWithReader processes hook messages from a custom reader
func (e *Executor) ExecuteWithReader(ctx context.Context, reader io.Reader) error {
	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Parse the message
	msg, err := e.handler.parser.ParseHookMessage(data)
	if err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	// Process the message
	response, err := e.handler.ProcessMessage(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to process message: %w", err)
	}

	// Write response if needed
	if response != nil {
		responseData, err := e.handler.parser.MarshalHookResponse(response)
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}

		_, err = os.Stdout.Write(responseData)
		if err != nil {
			return fmt.Errorf("failed to write response: %w", err)
		}
	}

	return nil
}

// SetTimeout updates the execution timeout
func (e *Executor) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

// SetRuleEngine updates the rule engine
func (e *Executor) SetRuleEngine(engine RuleEngine) {
	e.handler.SetRuleEngine(engine)
}

// GetRegistry returns the hook registry
func (e *Executor) GetRegistry() *Registry {
	return e.registry
}

// HookRunner provides utilities for running external hooks
type HookRunner struct {
	timeout time.Duration
}

// NewHookRunner creates a new hook runner
func NewHookRunner(timeout time.Duration) *HookRunner {
	return &HookRunner{
		timeout: timeout,
	}
}

// expandEnvVars expands environment variables in hook command paths
// Supports $CLAUDE_PROJECT_DIR and standard environment variables
func expandEnvVars(hookPath string) string {
	// Replace $CLAUDE_PROJECT_DIR with current working directory
	// This follows Claude Code's convention where hooks run in project context
	if wd, err := os.Getwd(); err == nil {
		hookPath = strings.ReplaceAll(hookPath, "$CLAUDE_PROJECT_DIR", wd)
	}

	// Expand other standard environment variables
	return os.ExpandEnv(hookPath)
}

// RunHook executes an external hook program
func (r *HookRunner) RunHook(ctx context.Context, hookPath string, input []byte) ([]byte, error) {
	// Expand environment variables in hook path
	expandedPath := expandEnvVars(hookPath)

	// Create command with timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, expandedPath)

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start hook: %w", err)
	}

	// Write input to stdin
	go func() {
		defer stdin.Close()
		_, _ = stdin.Write(input)
	}()

	// Wait for completion
	err = cmd.Wait()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Return stderr for exit code 2
			if exitErr.ExitCode() == int(ExitBlocking) {
				return stderr.Bytes(), nil
			}
		}
		return nil, fmt.Errorf("hook execution failed: %w", err)
	}

	return stdout.Bytes(), nil
}

// ChainExecutor allows chaining multiple rule engines in sequence
type ChainExecutor struct {
	executors []*Executor
}

// NewChainExecutor creates a new chain executor
func NewChainExecutor(executors ...*Executor) *ChainExecutor {
	return &ChainExecutor{
		executors: executors,
	}
}

// Execute runs all executors in sequence
func (c *ChainExecutor) Execute(ctx context.Context) error {
	for i, executor := range c.executors {
		if err := executor.Execute(ctx); err != nil {
			return fmt.Errorf("executor %d failed: %w", i, err)
		}
	}
	return nil
}

// hasResponseFeedback determines if a HookResponse contains meaningful feedback
func hasResponseFeedback(response *HookResponse) bool {
	if response == nil {
		return false
	}

	// Check if any feedback fields have content
	return response.Message != "" ||
		response.Reason != "" ||
		response.StopReason != "" ||
		response.Decision != "" ||
		response.Continue != nil ||
		response.SuppressOutput != nil
}
