package socket

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// CreateSocketDir ensures the socket directory exists with proper permissions
func CreateSocketDir() (string, error) {
	socketPath := GetSocketPath()
	socketDir := filepath.Dir(socketPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create socket directory: %w", err)
	}

	return socketPath, nil
}

// Listen creates a Unix domain socket listener at the standard location
func Listen() (net.Listener, error) {
	socketPath, err := CreateSocketDir()
	if err != nil {
		return nil, err
	}

	// Remove existing socket if it exists
	if err := os.RemoveAll(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on Unix socket %s: %w", socketPath, err)
	}

	// Set socket permissions
	if err := os.Chmod(socketPath, 0600); err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	return listener, nil
}

// Cleanup removes the Unix domain socket file
func Cleanup() error {
	socketPath := GetSocketPath()
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove socket: %w", err)
	}
	return nil
}
