package daemon

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestGetSocketPath(t *testing.T) {
	path, err := GetSocketPath()
	if err != nil {
		t.Fatalf("GetSocketPath failed: %v", err)
	}
	if path == "" {
		t.Fatal("GetSocketPath returned empty path")
	}
	t.Logf("Socket path: %s", path)
}

func TestGetLockPath(t *testing.T) {
	path, err := GetLockPath()
	if err != nil {
		t.Fatalf("GetLockPath failed: %v", err)
	}
	if path == "" {
		t.Fatal("GetLockPath returned empty path")
	}
	t.Logf("Lock path: %s", path)
}

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient returned nil client")
	}
}

func TestNewServer(t *testing.T) {
	server, err := NewServer(30 * time.Second)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if server == nil {
		t.Fatal("NewServer returned nil server")
	}
}

func TestClientPingWithoutDaemon(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err = client.Ping(ctx)
	if err == nil {
		t.Fatal("Expected ping to fail when no daemon is running")
	}
	t.Logf("Ping failed as expected: %v", err)
}

func TestServerStartStop(t *testing.T) {
	server, err := NewServer(30 * time.Second)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Start server in background
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test client connection
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer pingCancel()

	resp, err := client.Ping(pingCtx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if resp.Version == "" {
		t.Fatal("Ping response has empty version")
	}
	t.Logf("Ping response: %+v", resp)

	// Stop server
	server.Stop()

	// Wait for server to finish
	select {
	case err := <-serverDone:
		if err != nil {
			t.Logf("Server finished with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Server didn't stop in time")
	}
}

func TestDaemonIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test (set INTEGRATION_TEST=1 to run)")
	}

	server, err := NewServer(30 * time.Second)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Start server in background
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Test hook processing
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	testHook := []byte(`{
		"session_id": "test-session",
		"transcript_path": "",
		"hook_event_name": "PostToolUse",
		"tool_name": "Write",
		"tool_input": null,
		"tool_output": {
			"result": "success"
		}
	}`)

	hookCtx, hookCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer hookCancel()

	resp, err := client.SendHookRequest(hookCtx, testHook)
	if err != nil {
		t.Fatalf("SendHookRequest failed: %v", err)
	}

	if resp.ExitCode != 2 {
		t.Fatalf("Expected exit code 2 for PostToolUse, got %d", resp.ExitCode)
	}
	t.Logf("Hook response: exit code %d, stdout length %d", resp.ExitCode, len(resp.Stdout))

	// Stop server
	server.Stop()

	// Wait for server to finish
	select {
	case err := <-serverDone:
		if err != nil {
			t.Logf("Server finished with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Server didn't stop in time")
	}
}
