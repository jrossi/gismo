package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jrossi/gismo/pkg/server"
)

// Client represents a client that ensures the server is running
type Client struct {
	serverPath string
}

// New creates a new client instance
func New() (*Client, error) {
	// Try to find gismo-server in the same directory as the current executable
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	dir := filepath.Dir(execPath)
	serverPath := filepath.Join(dir, "gismo-server")

	// Check if server binary exists
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		// Try to find it in PATH
		serverPath = "gismo-server"
	}

	return &Client{
		serverPath: serverPath,
	}, nil
}

// EnsureServerRunning checks if the server is running and starts it if not
func (c *Client) EnsureServerRunning() error {
	// Check if server is already running
	if server.IsRunning() {
		return nil
	}

	// Start the server
	cmd := exec.Command(c.serverPath) // #nosec G204 - serverPath is controlled
	cmd.Stdout = nil                  // Server runs in background
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Start the server process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Give the server a moment to start up
	time.Sleep(100 * time.Millisecond)

	// Verify the server is now running
	if !server.IsRunning() {
		return fmt.Errorf("server failed to start")
	}

	return nil
}
