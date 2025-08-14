package socket

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCreateSocketDir(t *testing.T) {
	tests := []struct {
		name       string
		setupEnv   func() (cleanup func())
		wantErr    bool
		checkPerms bool
	}{
		{
			name: "creates directory with XDG_RUNTIME_DIR",
			setupEnv: func() func() {
				tmpDir := t.TempDir()
				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Setenv("XDG_RUNTIME_DIR", tmpDir)
				return func() {
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}
			},
			wantErr:    false,
			checkPerms: true,
		},
		{
			name: "creates directory without XDG_RUNTIME_DIR",
			setupEnv: func() func() {
				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Unsetenv("XDG_RUNTIME_DIR")
				return func() {
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}
			},
			wantErr:    false,
			checkPerms: true,
		},
		{
			name: "handles existing directory",
			setupEnv: func() func() {
				tmpDir := t.TempDir()
				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Setenv("XDG_RUNTIME_DIR", tmpDir)

				// Pre-create the gismo directory
				gismoDir := filepath.Join(tmpDir, "gismo")
				os.MkdirAll(gismoDir, 0700)

				return func() {
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}
			},
			wantErr:    false,
			checkPerms: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupEnv()
			defer cleanup()

			socketPath, err := CreateSocketDir()
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSocketDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// Verify the directory was created
				socketDir := filepath.Dir(socketPath)
				info, err := os.Stat(socketDir)
				if err != nil {
					t.Errorf("Failed to stat socket directory: %v", err)
					return
				}

				if !info.IsDir() {
					t.Error("Socket path parent is not a directory")
				}

				// Check permissions on Unix systems
				if tt.checkPerms && runtime.GOOS != "windows" {
					mode := info.Mode().Perm()
					// Should be 0700 (owner read/write/execute only)
					if mode != 0700 {
						t.Errorf("Socket directory has wrong permissions: %o, want 0700", mode)
					}
				}

				// Verify the path ends with gismo.sock
				if !strings.HasSuffix(socketPath, "gismo.sock") {
					t.Errorf("Socket path doesn't end with gismo.sock: %s", socketPath)
				}
			}
		})
	}
}

func TestListen(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func() (cleanup func())
		preSetup    func() error
		wantErr     bool
		errContains string
	}{
		{
			name: "successful listen",
			setupEnv: func() func() {
				tmpDir := t.TempDir()
				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Setenv("XDG_RUNTIME_DIR", tmpDir)
				return func() {
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}
			},
			wantErr: false,
		},
		{
			name: "removes existing socket",
			setupEnv: func() func() {
				tmpDir := t.TempDir()
				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Setenv("XDG_RUNTIME_DIR", tmpDir)
				return func() {
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}
			},
			preSetup: func() error {
				// Create a stale socket file
				socketPath := GetSocketPath()
				socketDir := filepath.Dir(socketPath)
				os.MkdirAll(socketDir, 0700)
				// Create a regular file where the socket should be
				file, err := os.Create(socketPath)
				if err != nil {
					return err
				}
				file.Close()
				return nil
			},
			wantErr: false,
		},
		{
			name: "handles permission denied on socket creation",
			setupEnv: func() func() {
				// Use a directory we can't write to
				tmpDir := t.TempDir()
				readOnlyDir := filepath.Join(tmpDir, "readonly")
				os.MkdirAll(readOnlyDir, 0500) // Read-only directory

				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Setenv("XDG_RUNTIME_DIR", readOnlyDir)
				return func() {
					os.Chmod(readOnlyDir, 0700) // Restore permissions for cleanup
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}
			},
			wantErr:     true,
			errContains: "failed to create socket directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupEnv()
			defer cleanup()

			if tt.preSetup != nil {
				if err := tt.preSetup(); err != nil {
					t.Fatalf("Failed to run preSetup: %v", err)
				}
			}

			listener, err := Listen()
			if (err != nil) != tt.wantErr {
				t.Errorf("Listen() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Listen() error = %v, want error containing %v", err, tt.errContains)
				}
			}

			if listener != nil {
				defer listener.Close()

				// Verify it's a Unix listener
				addr := listener.Addr()
				if addr.Network() != "unix" {
					t.Errorf("Listener network = %v, want unix", addr.Network())
				}

				// Verify socket file exists with correct permissions
				socketPath := GetSocketPath()
				info, err := os.Stat(socketPath)
				if err != nil {
					t.Errorf("Failed to stat socket file: %v", err)
				} else if runtime.GOOS != "windows" {
					mode := info.Mode().Perm()
					// Should be 0600 (owner read/write only)
					if mode != 0600 {
						t.Errorf("Socket file has wrong permissions: %o, want 0600", mode)
					}
				}

				// Test that we can accept connections
				go func() {
					conn, err := net.Dial("unix", socketPath)
					if err == nil {
						conn.Close()
					}
				}()

				conn, err := listener.Accept()
				if err == nil {
					conn.Close()
				}
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	tests := []struct {
		name     string
		setupEnv func() (cleanup func())
		preSetup func() error
		wantErr  bool
	}{
		{
			name: "removes existing socket",
			setupEnv: func() func() {
				tmpDir := t.TempDir()
				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Setenv("XDG_RUNTIME_DIR", tmpDir)
				return func() {
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}
			},
			preSetup: func() error {
				// Create a socket to clean up
				listener, err := Listen()
				if err != nil {
					return err
				}
				listener.Close()
				return nil
			},
			wantErr: false,
		},
		{
			name: "handles non-existent socket",
			setupEnv: func() func() {
				tmpDir := t.TempDir()
				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Setenv("XDG_RUNTIME_DIR", tmpDir)
				return func() {
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}
			},
			wantErr: false, // Should not error if socket doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupEnv()
			defer cleanup()

			if tt.preSetup != nil {
				if err := tt.preSetup(); err != nil {
					t.Fatalf("Failed to run preSetup: %v", err)
				}
			}

			err := Cleanup()
			if (err != nil) != tt.wantErr {
				t.Errorf("Cleanup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify socket is removed
			socketPath := GetSocketPath()
			if _, err := os.Stat(socketPath); err == nil {
				t.Error("Socket file still exists after cleanup")
			}
		})
	}
}

func TestListenIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup environment
	tmpDir := t.TempDir()
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	// Create first listener
	listener1, err := Listen()
	if err != nil {
		t.Fatalf("Failed to create first listener: %v", err)
	}
	defer listener1.Close()

	// Try to create second listener (should succeed by removing old socket)
	listener2, err := Listen()
	if err != nil {
		t.Fatalf("Failed to create second listener: %v", err)
	}
	defer listener2.Close()

	// First listener should be invalid now
	// Try to accept on first listener - it should fail
	done := make(chan bool)
	go func() {
		_, err := listener1.Accept()
		// We expect an error here since the socket was removed
		if err == nil {
			t.Error("Expected error when accepting on old listener")
		}
		done <- true
	}()

	// Give it a moment to fail
	select {
	case <-done:
		// Good, it failed as expected
	case <-time.After(100 * time.Millisecond):
		// Also acceptable - Accept is blocking
	}

	// Second listener should work
	socketPath := GetSocketPath()
	go func() {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
		}
	}()

	conn, err := listener2.Accept()
	if err != nil {
		t.Errorf("Failed to accept on second listener: %v", err)
	} else {
		conn.Close()
	}

	// Test cleanup
	if err := Cleanup(); err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	// Socket should be gone
	if _, err := os.Stat(socketPath); err == nil {
		t.Error("Socket still exists after cleanup")
	}
}

func TestListenConcurrent(t *testing.T) {
	// Test concurrent Listen calls
	tmpDir := t.TempDir()
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	numGoroutines := 5
	results := make(chan error, numGoroutines)
	listeners := make(chan net.Listener, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			listener, err := Listen()
			if err != nil {
				results <- err
			} else {
				listeners <- listener
				results <- nil
			}
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err == nil {
			successCount++
		}
	}

	// At least one should succeed
	if successCount == 0 {
		t.Error("No Listen calls succeeded")
	}

	// Close all successful listeners
	close(listeners)
	for listener := range listeners {
		listener.Close()
	}
}

func TestSocketDirPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix permission test on Windows")
	}

	tmpDir := t.TempDir()
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	// Create socket directory
	socketPath, err := CreateSocketDir()
	if err != nil {
		t.Fatalf("CreateSocketDir failed: %v", err)
	}

	// Check directory permissions
	socketDir := filepath.Dir(socketPath)
	info, err := os.Stat(socketDir)
	if err != nil {
		t.Fatalf("Failed to stat socket directory: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0700 {
		t.Errorf("Socket directory has wrong permissions: %o, want 0700", mode)
	}

	// Create socket and check its permissions
	listener, err := Listen()
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer listener.Close()

	socketInfo, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("Failed to stat socket file: %v", err)
	}

	socketMode := socketInfo.Mode().Perm()
	if socketMode != 0600 {
		t.Errorf("Socket file has wrong permissions: %o, want 0600", socketMode)
	}
}

// Benchmark tests
func BenchmarkCreateSocketDir(b *testing.B) {
	tmpDir := b.TempDir()
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	for i := 0; i < b.N; i++ {
		_, err := CreateSocketDir()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListen(b *testing.B) {
	tmpDir := b.TempDir()
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	for i := 0; i < b.N; i++ {
		listener, err := Listen()
		if err != nil {
			b.Fatal(err)
		}
		listener.Close()
	}
}

func BenchmarkCleanup(b *testing.B) {
	tmpDir := b.TempDir()
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	for i := 0; i < b.N; i++ {
		// Create a socket to clean up
		listener, err := Listen()
		if err != nil {
			b.Fatal(err)
		}
		listener.Close()

		// Benchmark the cleanup
		err = Cleanup()
		if err != nil {
			b.Fatal(err)
		}
	}
}
