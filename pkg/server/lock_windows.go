//go:build windows

package server

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const (
	LOCKFILE_EXCLUSIVE_LOCK   = 0x00000002
	LOCKFILE_FAIL_IMMEDIATELY = 0x00000001
)

// acquireLock tries to acquire an exclusive lock using Windows file locking
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

	// Try to acquire exclusive lock using Windows API
	handle := windows.Handle(file.Fd())
	overlapped := &windows.Overlapped{}

	err = windows.LockFileEx(
		handle,
		LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		overlapped,
	)

	if err != nil {
		file.Close()
		if err == windows.ERROR_LOCK_VIOLATION {
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

// releaseLock releases the lock using Windows file locking
func (s *Server) releaseLock() {
	if s.lockFile != nil {
		handle := windows.Handle(s.lockFile.Fd())
		overlapped := &windows.Overlapped{}

		// Unlock the file
		_ = windows.UnlockFileEx(
			handle,
			0,
			1,
			0,
			overlapped,
		)

		s.lockFile.Close()
		os.Remove(s.lockPath)
		s.lockFile = nil
	}
}
