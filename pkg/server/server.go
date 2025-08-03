package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

const (
	// SocketName is the name of the Unix domain socket
	SocketName = "gismo.sock"
	// LockFileName is the name of the lock file
	LockFileName = "gismo.lock"
)

// Server represents the gismo background server
type Server struct {
	socketPath string
	lockPath   string
	listener   net.Listener
	lockFile   *os.File
}

// New creates a new server instance
func New() (*Server, error) {
	// Get runtime directory
	runtimeDir, err := getRuntimeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime directory: %w", err)
	}

	return &Server{
		socketPath: filepath.Join(runtimeDir, SocketName),
		lockPath:   filepath.Join(runtimeDir, LockFileName),
	}, nil
}

// NewWithRuntimeDir creates a new server instance with a custom runtime directory.
// This is primarily used for testing to avoid conflicts between parallel tests.
func NewWithRuntimeDir(runtimeDir string) *Server {
	return &Server{
		socketPath: filepath.Join(runtimeDir, SocketName),
		lockPath:   filepath.Join(runtimeDir, LockFileName),
	}
}

// Start starts the server if it's not already running
func (s *Server) Start() error {
	// Try to acquire lock
	if err := s.acquireLock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Remove any existing socket
	os.Remove(s.socketPath)

	// Create Unix domain socket
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		s.releaseLock()
		return fmt.Errorf("failed to create socket: %w", err)
	}
	s.listener = listener

	// Set socket permissions
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		s.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	return nil
}

// Close closes the server and releases resources
func (s *Server) Close() error {
	var errs []error

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close listener: %w", err))
		}
	}

	// Remove socket file
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("failed to remove socket: %w", err))
	}

	// Release lock
	s.releaseLock()

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// IsRunning checks if a server is already running
func IsRunning() bool {
	runtimeDir, err := getRuntimeDir()
	if err != nil {
		return false
	}

	lockPath := filepath.Join(runtimeDir, LockFileName)

	// Try to open lock file exclusively
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists, check if process is still running
			return checkLockFile(lockPath)
		}
		return false
	}

	// We were able to create the lock file, so no server is running
	file.Close()
	os.Remove(lockPath)
	return false
}

// checkLockFile checks if the process that created the lock file is still running
func checkLockFile(lockPath string) bool {
	// Try to read PID from lock file
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}

	// Check if process is still running
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// getRuntimeDir returns the appropriate runtime directory for the platform
func getRuntimeDir() (string, error) {
	// Try XDG_RUNTIME_DIR first
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "gismo"), nil
	}

	// Fall back to temp directory
	return filepath.Join(os.TempDir(), fmt.Sprintf("gismo-%d", os.Getuid())), nil
}
