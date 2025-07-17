package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Client handles communication with the daemon
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates a new daemon client
func NewClient() (*Client, error) {
	socketPath, err := GetSocketPath()
	if err != nil {
		return nil, fmt.Errorf("getting socket path: %w", err)
	}

	return &Client{
		socketPath: socketPath,
		timeout:    5 * time.Second,
	}, nil
}

// Connect attempts to connect to the daemon, starting it if necessary
func (c *Client) Connect(ctx context.Context) (net.Conn, error) {
	// First try to connect
	conn, err := c.tryConnect()
	if err == nil {
		return conn, nil
	}

	// If connection failed, try to start the daemon
	if err := c.startDaemon(ctx); err != nil {
		return nil, fmt.Errorf("starting daemon: %w", err)
	}

	// Wait for daemon to be ready
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout.C:
			return nil, fmt.Errorf("timeout waiting for daemon to start")
		case <-ticker.C:
			conn, err := c.tryConnect()
			if err == nil {
				return conn, nil
			}
		}
	}
}

// tryConnect attempts to connect to the daemon
func (c *Client) tryConnect() (net.Conn, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// startDaemon starts the daemon process
func (c *Client) startDaemon(ctx context.Context) error {
	// Try to find the daemon binary
	daemonPath := "ccfeedback-daemon"

	// Check if it exists in the same directory as the current executable
	if execPath, err := os.Executable(); err == nil {
		dir := filepath.Dir(execPath)
		localDaemon := filepath.Join(dir, "ccfeedback-daemon")
		if _, err := os.Stat(localDaemon); err == nil {
			daemonPath = localDaemon
		}
	}

	cmd := exec.CommandContext(ctx, daemonPath)
	cmd.Env = append(os.Environ(), "CCFEEDBACK_DAEMON=1")

	// Daemon should detach itself, so we just start it
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting daemon process: %w", err)
	}

	// Don't wait for it - daemon will detach
	return nil
}

// SendHookRequest sends a hook processing request to the daemon
func (c *Client) SendHookRequest(ctx context.Context, stdinData []byte) (*HookResponse, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connecting to daemon: %w", err)
	}
	defer conn.Close()

	// Create request
	hookReq := &HookRequest{
		StdinData: stdinData,
		Env:       make(map[string]string),
	}

	// Add relevant environment variables
	for _, key := range []string{"CLAUDE_PROJECT_ROOT", "CLAUDE_TOOL_ID"} {
		if val := os.Getenv(key); val != "" {
			hookReq.Env[key] = val
		}
	}

	reqData, err := json.Marshal(hookReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling hook request: %w", err)
	}

	req := &Request{
		ID:      uuid.New().String(),
		Type:    "hook",
		Payload: reqData,
	}

	// Send request
	if err := c.sendRequest(conn, req); err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	// Read response
	resp, err := c.readResponse(conn)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("daemon error: %s", resp.Error)
	}

	// Unmarshal hook response
	var hookResp HookResponse
	if err := json.Unmarshal(resp.Payload, &hookResp); err != nil {
		return nil, fmt.Errorf("unmarshaling hook response: %w", err)
	}

	return &hookResp, nil
}

// Ping checks if the daemon is running
func (c *Client) Ping(ctx context.Context) (*PingResponse, error) {
	conn, err := c.tryConnect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req := &Request{
		ID:   uuid.New().String(),
		Type: "ping",
	}

	if err := c.sendRequest(conn, req); err != nil {
		return nil, fmt.Errorf("sending ping request: %w", err)
	}

	resp, err := c.readResponse(conn)
	if err != nil {
		return nil, fmt.Errorf("reading ping response: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("ping failed: %s", resp.Error)
	}

	var pingResp PingResponse
	if err := json.Unmarshal(resp.Payload, &pingResp); err != nil {
		return nil, fmt.Errorf("unmarshaling ping response: %w", err)
	}

	return &pingResp, nil
}

// sendRequest sends a request to the daemon
func (c *Client) sendRequest(conn net.Conn, req *Request) error {
	encoder := json.NewEncoder(conn)
	return encoder.Encode(req)
}

// readResponse reads a response from the daemon
func (c *Client) readResponse(conn net.Conn) (*Response, error) {
	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
