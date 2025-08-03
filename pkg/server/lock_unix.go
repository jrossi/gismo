//go:build !windows

package server

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// acquireLock tries to acquire an exclusive lock using Unix flock
func (s *Server) acquireLock() error {
	// Create runtime directory if it doesn't exist
	runtimeDir := filepath.Dir(s.lockPath)
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	// Open or create lock file
	file, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("server is already running")
		}
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Write PID to lock file
	if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
		file.Close()
		return fmt.Errorf("failed to write PID: %w", err)
	}

	s.lockFile = file
	return nil
}

// releaseLock releases the lock using Unix flock
func (s *Server) releaseLock() {
	if s.lockFile != nil {
		_ = syscall.Flock(int(s.lockFile.Fd()), syscall.LOCK_UN)
		s.lockFile.Close()
		os.Remove(s.lockPath)
		s.lockFile = nil
	}
}
