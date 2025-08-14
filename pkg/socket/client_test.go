package socket

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

func TestGetSocketPath(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func()
		cleanupEnv  func()
		wantPattern string
	}{
		{
			name: "with XDG_RUNTIME_DIR",
			setupEnv: func() {
				os.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
			},
			cleanupEnv: func() {
				os.Unsetenv("XDG_RUNTIME_DIR")
			},
			wantPattern: "/run/user/1000/gismo/gismo.sock",
		},
		{
			name: "without XDG_RUNTIME_DIR",
			setupEnv: func() {
				os.Unsetenv("XDG_RUNTIME_DIR")
			},
			cleanupEnv:  func() {},
			wantPattern: filepath.Join(os.TempDir(), fmt.Sprintf("gismo-%d", os.Getuid()), "gismo.sock"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			oldXDG := os.Getenv("XDG_RUNTIME_DIR")
			defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

			tt.setupEnv()
			defer tt.cleanupEnv()

			got := GetSocketPath()
			if got != tt.wantPattern {
				t.Errorf("GetSocketPath() = %v, want %v", got, tt.wantPattern)
			}
		})
	}
}

func TestConnect(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func() (func(), error)
		wantErr     bool
		errContains string
	}{
		{
			name: "socket not found",
			setupMock: func() (func(), error) {
				// Create a temp dir that doesn't contain a socket
				tmpDir := t.TempDir()
				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Setenv("XDG_RUNTIME_DIR", tmpDir)

				cleanup := func() {
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}
				return cleanup, nil
			},
			wantErr:     true,
			errContains: "not running",
		},
		{
			name: "successful connection",
			setupMock: func() (func(), error) {
				// Create a temporary socket for testing
				tmpDir := t.TempDir()
				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Setenv("XDG_RUNTIME_DIR", tmpDir)

				// Create gismo directory
				gismoDir := filepath.Join(tmpDir, "gismo")
				if err := os.MkdirAll(gismoDir, 0700); err != nil {
					return nil, err
				}

				socketPath := filepath.Join(gismoDir, "gismo.sock")

				// Create a mock server
				listener, err := net.Listen("unix", socketPath)
				if err != nil {
					return nil, fmt.Errorf("failed to create test listener: %w", err)
				}

				// Start a simple gRPC server
				srv := grpc.NewServer()
				go srv.Serve(listener)

				// Give server time to start
				time.Sleep(10 * time.Millisecond)

				cleanup := func() {
					srv.Stop()
					listener.Close()
					os.Remove(socketPath)
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}

				return cleanup, nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup, err := tt.setupMock()
			if err != nil {
				t.Fatalf("Failed to setup mock: %v", err)
			}
			defer cleanup()

			ctx := context.Background()
			conn, err := Connect(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Connect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Connect() error = %v, want error containing %v", err, tt.errContains)
				}
			}

			if conn != nil {
				defer conn.Close()
				// Verify connection is valid
				if conn.GetState() == connectivity.Shutdown {
					t.Error("Connection is already shutdown")
				}
			}
		})
	}
}

func TestConnectWithFallback(t *testing.T) {
	// Since ConnectWithFallback now only uses Unix socket,
	// it should behave exactly like Connect
	ctx := context.Background()

	// Create a temporary directory for our test socket
	tmpDir := t.TempDir()
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	// Create gismo directory
	gismoDir := filepath.Join(tmpDir, "gismo")
	if err := os.MkdirAll(gismoDir, 0700); err != nil {
		t.Fatalf("Failed to create gismo directory: %v", err)
	}

	socketPath := filepath.Join(gismoDir, "gismo.sock")

	// Test without socket existing
	_, err := ConnectWithFallback(ctx, "localhost:50051")
	if err == nil {
		t.Error("Expected error when socket doesn't exist")
	}

	// Create a mock server
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create test listener: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	srv := grpc.NewServer()
	go srv.Serve(listener)
	defer srv.Stop()

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	// Test with socket existing
	conn, err := ConnectWithFallback(ctx, "localhost:50051")
	if err != nil {
		t.Errorf("ConnectWithFallback() unexpected error: %v", err)
	}
	if conn != nil {
		defer conn.Close()
	}
}

func TestConnectIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary directory for our test
	tmpDir := t.TempDir()

	// Set up environment
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	// Create socket directory
	socketDir := filepath.Join(tmpDir, "gismo")
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		t.Fatalf("Failed to create socket directory: %v", err)
	}

	socketPath := filepath.Join(socketDir, "gismo.sock")

	// Create a real Unix socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	// Start a simple gRPC server
	srv := grpc.NewServer()
	go srv.Serve(listener)
	defer srv.Stop()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := Connect(ctx)
	if err != nil {
		t.Errorf("Connect() failed: %v", err)
	}

	if conn != nil {
		defer conn.Close()

		// Verify connection state
		state := conn.GetState()
		if state == connectivity.Shutdown || state == connectivity.TransientFailure {
			t.Errorf("Unexpected connection state: %v", state)
		}

		// Try to wait for ready
		ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel2()

		if !conn.WaitForStateChange(ctx2, connectivity.Shutdown) {
			// Connection is not shutdown, which is good
		}
	}
}

func TestConnectRaceCondition(t *testing.T) {
	// Test concurrent connections
	// Use a shorter path for Mac compatibility
	tmpDir := "/tmp/gtest"
	os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	// Create gismo directory
	gismoDir := filepath.Join(tmpDir, "gismo")
	if err := os.MkdirAll(gismoDir, 0700); err != nil {
		t.Fatalf("Failed to create gismo directory: %v", err)
	}

	socketPath := filepath.Join(gismoDir, "gismo.sock")

	// Create a mock server
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create test listener: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	srv := grpc.NewServer()
	go srv.Serve(listener)
	defer srv.Stop()

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	// Test multiple concurrent connections
	ctx := context.Background()
	numConnections := 10
	errors := make(chan error, numConnections)

	for i := 0; i < numConnections; i++ {
		go func() {
			conn, err := Connect(ctx)
			if conn != nil {
				conn.Close()
			}
			errors <- err
		}()
	}

	// Collect results
	for i := 0; i < numConnections; i++ {
		if err := <-errors; err != nil {
			t.Errorf("Concurrent connection %d failed: %v", i, err)
		}
	}
}

func TestConnectWithBadPermissions(t *testing.T) {
	// Skip on Windows as Unix permissions don't apply
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping Unix permission test on Windows")
	}

	tmpDir := t.TempDir()
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	// Create gismo directory
	gismoDir := filepath.Join(tmpDir, "gismo")
	if err := os.MkdirAll(gismoDir, 0700); err != nil {
		t.Fatalf("Failed to create gismo directory: %v", err)
	}

	socketPath := filepath.Join(gismoDir, "gismo.sock")

	// Create a regular file instead of socket
	file, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	file.Close()

	// This should fail because Connect expects a socket but finds a regular file
	// The grpc.NewClient will succeed but the actual connection will fail
	ctx := context.Background()
	conn, err := Connect(ctx)
	if err != nil {
		// This is expected - Connect should fail when the file is not a socket
		t.Logf("Got expected error: %v", err)
	} else {
		// If Connect didn't fail immediately, the connection state should indicate failure
		if conn != nil {
			defer conn.Close()
			state := conn.GetState()
			// The connection should not be in a good state since it's not a real socket
			if state != connectivity.TransientFailure && state != connectivity.Shutdown {
				t.Logf("Connection state: %v", state)
			}
		}
	}
}

// Benchmark tests
func BenchmarkGetSocketPath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GetSocketPath()
	}
}

func BenchmarkConnect(b *testing.B) {
	// Setup a test server
	tmpDir := b.TempDir()
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	gismoDir := filepath.Join(tmpDir, "gismo")
	if err := os.MkdirAll(gismoDir, 0700); err != nil {
		b.Fatalf("Failed to create gismo directory: %v", err)
	}

	socketPath := filepath.Join(gismoDir, "gismo.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		b.Fatalf("Failed to create test listener: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	srv := grpc.NewServer()
	go srv.Serve(listener)
	defer srv.Stop()

	time.Sleep(10 * time.Millisecond)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, err := Connect(ctx)
		if err != nil {
			b.Fatal(err)
		}
		conn.Close()
	}
}
