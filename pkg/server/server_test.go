package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	srv, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if srv.socketPath == "" {
		t.Error("Socket path is empty")
	}

	if srv.lockPath == "" {
		t.Error("Lock path is empty")
	}

	// Verify paths contain expected filenames
	if filepath.Base(srv.socketPath) != SocketName {
		t.Errorf("Socket path doesn't contain expected filename: got %s, want %s", filepath.Base(srv.socketPath), SocketName)
	}

	if filepath.Base(srv.lockPath) != LockFileName {
		t.Errorf("Lock path doesn't contain expected filename: got %s, want %s", filepath.Base(srv.lockPath), LockFileName)
	}
}

func TestServerStartStop(t *testing.T) {
	srv, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Verify socket exists
	if _, err := os.Stat(srv.socketPath); err != nil {
		t.Errorf("Socket file not created: %v", err)
	}

	// Verify lock file exists
	if _, err := os.Stat(srv.lockPath); err != nil {
		t.Errorf("Lock file not created: %v", err)
	}

	// Try to start another server (should fail)
	srv2, err := New()
	if err != nil {
		t.Fatalf("Failed to create second server: %v", err)
	}

	if err := srv2.Start(); err == nil {
		t.Error("Expected error when starting second server, got nil")
		srv2.Close()
	}

	// Close first server
	if err := srv.Close(); err != nil {
		t.Errorf("Failed to close server: %v", err)
	}

	// Verify socket is removed
	if _, err := os.Stat(srv.socketPath); !os.IsNotExist(err) {
		t.Error("Socket file not removed after close")
	}

	// Verify lock file is removed
	if _, err := os.Stat(srv.lockPath); !os.IsNotExist(err) {
		t.Error("Lock file not removed after close")
	}
}

func TestIsRunning(t *testing.T) {
	// Initially should not be running
	if IsRunning() {
		t.Error("IsRunning returned true when no server is running")
	}

	// Start a server
	srv, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	// Now should be running
	if !IsRunning() {
		t.Error("IsRunning returned false when server is running")
	}

	// Close and verify not running
	srv.Close()

	// Give a moment for cleanup
	time.Sleep(10 * time.Millisecond)

	if IsRunning() {
		t.Error("IsRunning returned true after server closed")
	}
}

func TestCheckLockFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	// Test with non-existent file
	if checkLockFile(lockPath) {
		t.Error("checkLockFile returned true for non-existent file")
	}

	// Test with valid PID (our own process)
	pid := os.Getpid()
	if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("%d\n", pid)), 0600); err != nil {
		t.Fatalf("Failed to write lock file: %v", err)
	}

	if !checkLockFile(lockPath) {
		t.Error("checkLockFile returned false for valid PID")
	}

	// Test with invalid PID
	if err := os.WriteFile(lockPath, []byte("99999999\n"), 0600); err != nil {
		t.Fatalf("Failed to write lock file: %v", err)
	}

	if checkLockFile(lockPath) {
		t.Error("checkLockFile returned true for invalid PID")
	}

	// Test with malformed content
	if err := os.WriteFile(lockPath, []byte("not-a-number\n"), 0600); err != nil {
		t.Fatalf("Failed to write lock file: %v", err)
	}

	if checkLockFile(lockPath) {
		t.Error("checkLockFile returned true for malformed content")
	}
}

func TestGetRuntimeDir(t *testing.T) {
	// Save original env
	origXDG := os.Getenv("XDG_RUNTIME_DIR")
	defer os.Setenv("XDG_RUNTIME_DIR", origXDG)

	// Test with XDG_RUNTIME_DIR set
	testDir := "/tmp/test-xdg"
	os.Setenv("XDG_RUNTIME_DIR", testDir)

	dir, err := getRuntimeDir()
	if err != nil {
		t.Fatalf("getRuntimeDir failed: %v", err)
	}

	expected := filepath.Join(testDir, "gismo")
	if dir != expected {
		t.Errorf("Wrong runtime dir with XDG set: got %s, want %s", dir, expected)
	}

	// Test without XDG_RUNTIME_DIR
	os.Unsetenv("XDG_RUNTIME_DIR")

	dir, err = getRuntimeDir()
	if err != nil {
		t.Fatalf("getRuntimeDir failed without XDG: %v", err)
	}

	// Should use temp directory with UID
	if !filepath.IsAbs(dir) {
		t.Error("Runtime dir is not absolute path")
	}

	expectedBase := fmt.Sprintf("gismo-%d", os.Getuid())
	if filepath.Base(dir) != expectedBase {
		t.Errorf("Runtime dir doesn't have expected format: got %s, want %s", filepath.Base(dir), expectedBase)
	}
}

func TestSocketPermissions(t *testing.T) {
	srv, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	// Check socket permissions
	info, err := os.Stat(srv.socketPath)
	if err != nil {
		t.Fatalf("Failed to stat socket: %v", err)
	}

	// Should be 0600 (owner read/write only)
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Wrong socket permissions: got %o, want %o", perm, 0600)
	}
}

func TestConcurrentServerStart(t *testing.T) {
	// Test that multiple goroutines trying to start servers
	// results in only one successful start
	done := make(chan bool, 3)
	started := make(chan *Server, 3)

	for i := 0; i < 3; i++ {
		go func() {
			srv, err := New()
			if err != nil {
				done <- false
				return
			}

			if err := srv.Start(); err == nil {
				started <- srv
				done <- true
			} else {
				done <- false
			}
		}()
	}

	// Wait for all goroutines
	successCount := 0
	for i := 0; i < 3; i++ {
		if <-done {
			successCount++
		}
	}

	// Only one should succeed
	if successCount != 1 {
		t.Errorf("Expected 1 successful server start, got %d", successCount)
	}

	// Clean up the started server
	if successCount > 0 {
		srv := <-started
		srv.Close()
	}
}
