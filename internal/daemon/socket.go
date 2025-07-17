package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// GetSocketPath returns the platform-specific socket path for ccfeedback daemon
func GetSocketPath() (string, error) {
	var socketDir string

	switch runtime.GOOS {
	case "darwin":
		// macOS: Use user's Library directory
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		socketDir = filepath.Join(home, "Library", "Application Support", "ccfeedback")
	case "linux":
		// Linux: Use XDG_RUNTIME_DIR if available, otherwise fallback
		if xdgRuntime := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntime != "" {
			socketDir = filepath.Join(xdgRuntime, "ccfeedback")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("getting home directory: %w", err)
			}
			socketDir = filepath.Join(home, ".ccfeedback", "run")
		}
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Ensure directory exists with proper permissions
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		return "", fmt.Errorf("creating socket directory: %w", err)
	}

	return filepath.Join(socketDir, "daemon.sock"), nil
}

// GetLockPath returns the path for the daemon lock file
func GetLockPath() (string, error) {
	socketPath, err := GetSocketPath()
	if err != nil {
		return "", err
	}
	return socketPath + ".lock", nil
}
