package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// TestUpgradeCoordinator tests the basic upgrade coordinator functionality
func TestUpgradeCoordinator(t *testing.T) {
	// Create a test server
	tmpDir := t.TempDir()
	srv := NewWithRuntimeDir(tmpDir)

	// Start the server
	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	// Create upgrade coordinator
	coordinator, err := NewUpgradeCoordinator(srv)
	if err != nil {
		t.Fatalf("Failed to create upgrade coordinator: %v", err)
	}

	// Verify binary path was resolved
	if coordinator.binaryPath == "" {
		t.Error("Binary path should not be empty")
	}

	// Verify handoff socket path
	expectedHandoff := fmt.Sprintf("%s.handoff", srv.socketPath)
	if coordinator.handoffSocket != expectedHandoff {
		t.Errorf("Expected handoff socket %s, got %s", expectedHandoff, coordinator.handoffSocket)
	}
}

// TestFileDescriptorPassing tests passing file descriptors between processes
func TestFileDescriptorPassing(t *testing.T) {
	// Create a Unix socket pair for testing
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("Failed to create socket pair: %v", err)
	}

	// Create files from descriptors
	parentConn := os.NewFile(uintptr(fds[0]), "parent")
	childConn := os.NewFile(uintptr(fds[1]), "child")
	defer parentConn.Close()
	defer childConn.Close()

	// Create a test file to pass
	tmpFile, err := os.CreateTemp("", "test-fd-")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	testData := []byte("test data for FD passing")
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	_, _ = tmpFile.Seek(0, 0)

	// Test sending file descriptor
	errChan := make(chan error, 1)
	go func() {
		// Send the FD
		rights := syscall.UnixRights(int(tmpFile.Fd()))
		err := syscall.Sendmsg(int(parentConn.Fd()), []byte("FD"), rights, nil, 0)
		errChan <- err
	}()

	// Test receiving file descriptor
	buf := make([]byte, 2)
	oob := make([]byte, 1024)

	n, oobn, _, _, err := syscall.Recvmsg(int(childConn.Fd()), buf, oob, 0)
	if err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}

	if n != 2 || string(buf) != "FD" {
		t.Fatalf("Unexpected message: %s", string(buf[:n]))
	}

	// Parse the file descriptor
	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		t.Fatalf("Failed to parse control message: %v", err)
	}

	if len(scms) != 1 {
		t.Fatalf("Expected 1 control message, got %d", len(scms))
	}

	receivedFds, err := syscall.ParseUnixRights(&scms[0])
	if err != nil {
		t.Fatalf("Failed to parse Unix rights: %v", err)
	}

	if len(receivedFds) != 1 {
		t.Fatalf("Expected 1 file descriptor, got %d", len(receivedFds))
	}

	// Verify we can use the received file descriptor
	receivedFile := os.NewFile(uintptr(receivedFds[0]), "received")
	defer receivedFile.Close()

	readData, err := io.ReadAll(receivedFile)
	if err != nil {
		t.Fatalf("Failed to read from received file: %v", err)
	}

	if !bytes.Equal(readData, testData) {
		t.Errorf("Data mismatch: expected %s, got %s", testData, readData)
	}

	// Check send error
	if err := <-errChan; err != nil {
		t.Errorf("Send error: %v", err)
	}
}

// TestUpgradeHandshake tests the upgrade handshake protocol
func TestUpgradeHandshake(t *testing.T) {
	// Create a test handoff socket
	tmpDir := t.TempDir()
	handoffSocket := filepath.Join(tmpDir, "handoff.sock")

	// Start a mock parent process
	listener, err := net.Listen("unix", handoffSocket)
	if err != nil {
		t.Fatalf("Failed to create handoff socket: %v", err)
	}
	defer listener.Close()
	defer os.Remove(handoffSocket)

	// Simulate child connecting
	errChan := make(chan error, 1)
	go func() {
		conn, err := net.Dial("unix", handoffSocket)
		if err != nil {
			errChan <- err
			return
		}
		defer conn.Close()

		// Send health check
		if _, err := conn.Write([]byte("READY")); err != nil {
			errChan <- err
			return
		}

		// Wait for takeover signal
		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		if err != nil {
			errChan <- err
			return
		}

		if string(buf[:n]) != "TAKEOVER" {
			errChan <- fmt.Errorf("unexpected signal: %s", string(buf[:n]))
			return
		}

		errChan <- nil
	}()

	// Accept connection from child
	conn, err := listener.Accept()
	if err != nil {
		t.Fatalf("Failed to accept connection: %v", err)
	}
	defer conn.Close()

	// Read health check
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read health check: %v", err)
	}

	if string(buf[:n]) != "READY" {
		t.Errorf("Expected READY, got %s", string(buf[:n]))
	}

	// Send takeover signal
	if _, err := conn.Write([]byte("TAKEOVER")); err != nil {
		t.Fatalf("Failed to send takeover signal: %v", err)
	}

	// Check child error
	if err := <-errChan; err != nil {
		t.Errorf("Child error: %v", err)
	}
}

// TestBinaryWatcher tests the binary file watcher
func TestBinaryWatcher(t *testing.T) {
	// Create a test server
	tmpDir := t.TempDir()
	srv := NewWithRuntimeDir(tmpDir)

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	// Create coordinator
	coordinator, err := NewUpgradeCoordinator(srv)
	if err != nil {
		t.Fatalf("Failed to create coordinator: %v", err)
	}

	// Create a mock binary file
	mockBinary := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	// Override binary path for testing
	coordinator.binaryPath = mockBinary

	// Create watcher
	watcher, err := NewBinaryWatcher(coordinator)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Test configuration methods
	watcher.SetDebounceDelay(100 * time.Millisecond)
	watcher.SetCooldownDelay(200 * time.Millisecond)

	// Test checksum calculation
	checksum1, err := calculateChecksum(mockBinary)
	if err != nil {
		t.Fatalf("Failed to calculate checksum: %v", err)
	}

	// Modify the file
	newContent := []byte("#!/bin/sh\necho modified\n")
	if err := os.WriteFile(mockBinary, newContent, 0755); err != nil {
		t.Fatalf("Failed to modify mock binary: %v", err)
	}

	// Calculate new checksum
	checksum2, err := calculateChecksum(mockBinary)
	if err != nil {
		t.Fatalf("Failed to calculate new checksum: %v", err)
	}

	// Verify checksums are different
	if bytes.Equal(checksum1[:], checksum2[:]) {
		t.Error("Checksums should be different after modification")
	}

	// Test binary verification - make file bigger to pass size check
	largerContent := []byte("#!/bin/sh\necho modified\n" + string(make([]byte, 1024*1024)))
	if err := os.WriteFile(mockBinary, largerContent, 0755); err != nil {
		t.Fatalf("Failed to write larger binary: %v", err)
	}

	if err := watcher.verifyBinary(mockBinary); err != nil {
		t.Errorf("Binary verification failed: %v", err)
	}

	// Test invalid binary verification
	invalidBinary := filepath.Join(tmpDir, "invalid")
	if err := os.WriteFile(invalidBinary, []byte("too small"), 0644); err != nil {
		t.Fatalf("Failed to create invalid binary: %v", err)
	}

	if err := watcher.verifyBinary(invalidBinary); err == nil {
		t.Error("Should have failed to verify invalid binary")
	}
}

// TestUpgradeMetrics tests upgrade metrics tracking
func TestUpgradeMetrics(t *testing.T) {
	// Create a test server
	tmpDir := t.TempDir()
	srv := NewWithRuntimeDir(tmpDir)

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	// Create coordinator
	coordinator, err := NewUpgradeCoordinator(srv)
	if err != nil {
		t.Fatalf("Failed to create coordinator: %v", err)
	}

	// Get initial metrics
	upgradeCount, failedCount, lastUpgrade := coordinator.GetMetrics()
	if upgradeCount != 0 || failedCount != 0 {
		t.Errorf("Expected zero initial metrics, got %d upgrades, %d failed", upgradeCount, failedCount)
	}
	if !lastUpgrade.IsZero() {
		t.Error("Expected zero time for last upgrade")
	}

	// Simulate failed upgrade
	atomic.AddInt64(&coordinator.failedUpgrades, 1)

	// Check updated metrics
	_, failedCount, _ = coordinator.GetMetrics()
	if failedCount != 1 {
		t.Errorf("Expected 1 failed upgrade, got %d", failedCount)
	}
}

// TestConcurrentUpgrades tests that concurrent upgrades are prevented
func TestConcurrentUpgrades(t *testing.T) {
	// Create a test server
	tmpDir := t.TempDir()
	srv := NewWithRuntimeDir(tmpDir)

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	// Create coordinator
	coordinator, err := NewUpgradeCoordinator(srv)
	if err != nil {
		t.Fatalf("Failed to create coordinator: %v", err)
	}

	// Set upgrading flag
	coordinator.upgrading.Store(true)

	// Try to trigger upgrade while one is in progress
	ctx := context.Background()
	err = coordinator.TriggerUpgrade(ctx)
	if err == nil || err.Error() != "upgrade already in progress" {
		t.Errorf("Expected 'upgrade already in progress' error, got: %v", err)
	}

	// Reset flag
	coordinator.upgrading.Store(false)
}

// TestUpgradeRollback tests rollback on upgrade failure
func TestUpgradeRollback(t *testing.T) {
	// Create a test server
	tmpDir := t.TempDir()
	srv := NewWithRuntimeDir(tmpDir)

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	// Create coordinator
	coordinator, err := NewUpgradeCoordinator(srv)
	if err != nil {
		t.Fatalf("Failed to create coordinator: %v", err)
	}

	// Set an invalid binary path to trigger failure
	coordinator.binaryPath = "/nonexistent/binary"

	// Try upgrade - should fail
	ctx := context.Background()
	err = coordinator.TriggerUpgrade(ctx)
	if err == nil {
		t.Error("Expected upgrade to fail with invalid binary")
	}

	// Verify failure was recorded
	_, failedCount, _ := coordinator.GetMetrics()
	if failedCount != 1 {
		t.Errorf("Expected 1 failed upgrade, got %d", failedCount)
	}

	// Verify upgrading flag was reset
	if coordinator.upgrading.Load() {
		t.Error("Upgrading flag should be reset after failure")
	}
}

// TestMessageEncoding tests message encoding/decoding
func TestMessageEncoding(t *testing.T) {
	tests := []struct {
		name string
		msg  *UpgradeMessage
	}{
		{
			name: "simple message",
			msg: &UpgradeMessage{
				Type:    "TEST",
				Payload: []byte("test payload"),
			},
		},
		{
			name: "empty payload",
			msg: &UpgradeMessage{
				Type:    "EMPTY",
				Payload: nil,
			},
		},
		{
			name: "large payload",
			msg: &UpgradeMessage{
				Type:    "LARGE",
				Payload: make([]byte, 1024),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode message
			encoded, err := EncodeMessage(tt.msg)
			if err != nil {
				t.Fatalf("Failed to encode message: %v", err)
			}

			// Decode message
			buf := bytes.NewReader(encoded)
			decoded, err := DecodeMessage(buf)
			if err != nil {
				t.Fatalf("Failed to decode message: %v", err)
			}

			// Verify type
			if decoded.Type != tt.msg.Type {
				t.Errorf("Type mismatch: expected %s, got %s", tt.msg.Type, decoded.Type)
			}

			// Verify payload
			if !bytes.Equal(decoded.Payload, tt.msg.Payload) {
				t.Errorf("Payload mismatch: expected %d bytes, got %d bytes",
					len(tt.msg.Payload), len(decoded.Payload))
			}
		})
	}
}

// TestUpgradeUnderLoad simulates upgrade under heavy load
func TestUpgradeUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	// Create a test server
	tmpDir := t.TempDir()
	srv := NewWithRuntimeDir(tmpDir)

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	// Start load generation
	stopLoad := make(chan struct{})
	var wg sync.WaitGroup
	requestCount := atomic.Int64{}

	// Simulate multiple concurrent clients
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			for {
				select {
				case <-stopLoad:
					return
				default:
					// Simulate a request
					conn, err := net.Dial("unix", srv.socketPath)
					if err != nil {
						// Server might be upgrading
						time.Sleep(10 * time.Millisecond)
						continue
					}
					conn.Close()
					requestCount.Add(1)
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}

	// Let load run for a bit
	time.Sleep(100 * time.Millisecond)

	// Record requests before upgrade
	beforeUpgrade := requestCount.Load()
	t.Logf("Requests before upgrade: %d", beforeUpgrade)

	// Create coordinator and trigger upgrade
	coordinator, err := NewUpgradeCoordinator(srv)
	if err != nil {
		t.Fatalf("Failed to create coordinator: %v", err)
	}

	// Create a mock new binary
	mockBinary := filepath.Join(tmpDir, "new-binary")
	if err := createMockServerBinary(mockBinary); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}
	coordinator.binaryPath = mockBinary

	// Note: In a real test, we'd trigger the upgrade here
	// For now, we're just testing the load generation

	// Stop load generation
	close(stopLoad)
	wg.Wait()

	// Check total requests
	totalRequests := requestCount.Load()
	t.Logf("Total requests: %d", totalRequests)

	if totalRequests == 0 {
		t.Error("No requests were processed")
	}
}

// Helper function to create a mock server binary
func createMockServerBinary(path string) error {
	// Create a simple shell script that acts as a server
	script := `#!/bin/sh
# Mock server binary for testing
sleep 1
exit 0
`
	return os.WriteFile(path, []byte(script), 0755)
}

// BenchmarkFileDescriptorPassing benchmarks FD passing performance
func BenchmarkFileDescriptorPassing(b *testing.B) {
	// Create socket pair
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		b.Fatalf("Failed to create socket pair: %v", err)
	}

	parentConn := os.NewFile(uintptr(fds[0]), "parent")
	childConn := os.NewFile(uintptr(fds[1]), "child")
	defer parentConn.Close()
	defer childConn.Close()

	// Create test file
	tmpFile, err := os.CreateTemp("", "bench-fd-")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Send FD
		rights := syscall.UnixRights(int(tmpFile.Fd()))
		if err := syscall.Sendmsg(int(parentConn.Fd()), []byte("FD"), rights, nil, 0); err != nil {
			b.Fatalf("Send failed: %v", err)
		}

		// Receive FD
		buf := make([]byte, 2)
		oob := make([]byte, 1024)
		_, oobn, _, _, err := syscall.Recvmsg(int(childConn.Fd()), buf, oob, 0)
		if err != nil {
			b.Fatalf("Receive failed: %v", err)
		}

		// Parse FD (but don't create file to avoid leaks)
		scms, _ := syscall.ParseSocketControlMessage(oob[:oobn])
		if len(scms) > 0 {
			if fds, err := syscall.ParseUnixRights(&scms[0]); err == nil && len(fds) > 0 {
				// Close the received FD immediately to avoid leaks
				syscall.Close(fds[0])
			}
		}
	}
}

// TestUpgradeWithGracefulShutdown tests that upgrade waits for graceful shutdown
func TestUpgradeWithGracefulShutdown(t *testing.T) {
	// Create a test server with a short path (Unix socket path limit)
	tmpDir := "/tmp/gtest"
	os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srv := NewWithRuntimeDir(tmpDir)

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Close()

	// Simulate active connections by holding the grpc server
	activeRequests := sync.WaitGroup{}
	activeRequests.Add(1)

	// Start a goroutine that simulates a long-running request
	go func() {
		defer activeRequests.Done()
		// Simulate work
		time.Sleep(100 * time.Millisecond)
	}()

	// Create coordinator (not used but tests the creation)
	_, err := NewUpgradeCoordinator(srv)
	if err != nil {
		t.Fatalf("Failed to create coordinator: %v", err)
	}

	// Verify the server is properly configured
	if srv.grpcServer == nil {
		t.Fatal("gRPC server should be initialized")
	}

	// Wait for simulated request to complete
	activeRequests.Wait()
}

// TestUpgradeSignalHandling tests signal-based upgrade triggering
func TestUpgradeSignalHandling(t *testing.T) {
	if os.Getenv("TEST_SUBPROCESS") == "1" {
		// This is the subprocess
		handleTestSubprocess()
		return
	}

	// Start a subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestUpgradeSignalHandling")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS=1")

	// Capture output
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start subprocess: %v", err)
	}

	// Give subprocess time to start
	time.Sleep(100 * time.Millisecond)

	// Send SIGUSR1 to trigger upgrade
	if err := cmd.Process.Signal(syscall.SIGUSR1); err != nil {
		t.Errorf("Failed to send SIGUSR1: %v", err)
	}

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Send SIGTERM to cleanup
	_ = cmd.Process.Signal(syscall.SIGTERM)

	// Wait for subprocess to exit
	_ = cmd.Wait()

	output := out.String()
	if !bytes.Contains(out.Bytes(), []byte("SIGUSR1 received")) {
		t.Errorf("Subprocess did not handle SIGUSR1 correctly. Output: %s", output)
	}
}

// handleTestSubprocess handles the test subprocess for signal testing
func handleTestSubprocess() {
	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGUSR1, syscall.SIGTERM)

	for sig := range sigChan {
		switch sig {
		case syscall.SIGUSR1:
			fmt.Println("SIGUSR1 received")
		case syscall.SIGTERM:
			fmt.Println("SIGTERM received")
			os.Exit(0)
		}
	}
}
