package client

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jrossi/gismo/pkg/server"
)

func TestNew(t *testing.T) {
	cli, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if cli.serverPath == "" {
		t.Error("Server path is empty")
	}

	// Server path should be either absolute or just the binary name
	if !filepath.IsAbs(cli.serverPath) && cli.serverPath != "gismo-server" {
		t.Errorf("Unexpected server path: %s", cli.serverPath)
	}
}

func TestNewWithExecutableInSameDir(t *testing.T) {
	// Create a temporary directory with a fake executable
	tmpDir := t.TempDir()
	fakeExec := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(fakeExec, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("Failed to create fake executable: %v", err)
	}

	// Create fake gismo-server in same directory
	fakeServer := filepath.Join(tmpDir, "gismo-server")
	if err := os.WriteFile(fakeServer, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("Failed to create fake server: %v", err)
	}

	// Override os.Executable for this test (can't actually do this)
	// So we'll test the logic indirectly by creating a client
	// and checking if it finds the server in PATH
	cli, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// The client should have found a server path
	if cli.serverPath == "" {
		t.Error("Client didn't find server path")
	}
}

func TestEnsureServerRunning(t *testing.T) {
	// Clean up any existing server state first by removing lock files
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(os.TempDir(), fmt.Sprintf("gismo-%d", os.Getuid()))
	} else {
		runtimeDir = filepath.Join(runtimeDir, "gismo")
	}
	lockPath := filepath.Join(runtimeDir, "gismo.lock")
	os.Remove(lockPath) // Ignore errors

	cli, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Since we can't actually start the server binary in tests,
	// we'll just verify the method exists and returns an error
	// when the binary doesn't exist or can't be started
	cli.serverPath = "/nonexistent/gismo-server"
	err = cli.EnsureServerRunning()
	if err == nil {
		t.Error("Expected error when server binary doesn't exist")
	}
}

func TestEnsureServerRunningWhenAlreadyRunning(t *testing.T) {
	// Use shorter temp directory for this test to avoid macOS socket path limits
	tmpDir := "/tmp/gtest_srv"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set XDG_RUNTIME_DIR so client and server use same directory
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	srv := server.NewWithRuntimeDir(filepath.Join(tmpDir, "gismo"))

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	// Create client and ensure server
	cli, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// This should succeed immediately since server is already running
	err = cli.EnsureServerRunning()
	if err != nil {
		t.Errorf("EnsureServerRunning failed when server was already running: %v", err)
	}
}

func TestClientServerIntegration(t *testing.T) {
	// This is more of an integration test
	// We can't fully test starting the server binary without
	// having the actual binary built
	cli, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// At minimum, the client should be created successfully
	if cli == nil {
		t.Error("Client is nil")
	}
}
