package server

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// UpgradeCoordinator handles zero-downtime upgrades
type UpgradeCoordinator struct {
	server        *Server
	binaryPath    string
	upgrading     atomic.Bool
	handoffSocket string
	mu            sync.RWMutex

	// Metrics for monitoring
	upgradeCount    int64
	lastUpgradeTime time.Time
	failedUpgrades  int64
}

// NewUpgradeCoordinator creates a new upgrade coordinator
func NewUpgradeCoordinator(server *Server) (*UpgradeCoordinator, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get actual binary path
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve binary path: %w", err)
	}

	return &UpgradeCoordinator{
		server:        server,
		binaryPath:    binaryPath,
		handoffSocket: fmt.Sprintf("%s.handoff", server.socketPath),
	}, nil
}

// TriggerUpgrade initiates a zero-downtime upgrade
func (uc *UpgradeCoordinator) TriggerUpgrade(ctx context.Context) error {
	if !uc.upgrading.CompareAndSwap(false, true) {
		return errors.New("upgrade already in progress")
	}
	defer uc.upgrading.Store(false)

	log.Printf("Starting zero-downtime upgrade from %s", uc.binaryPath)

	// Step 1: Verify new binary exists and is executable
	info, err := os.Stat(uc.binaryPath)
	if err != nil {
		atomic.AddInt64(&uc.failedUpgrades, 1)
		return fmt.Errorf("binary not found: %w", err)
	}
	if info.Mode()&0111 == 0 {
		atomic.AddInt64(&uc.failedUpgrades, 1)
		return errors.New("binary is not executable")
	}

	// Step 2: Create handoff socket for coordination
	handoffListener, err := net.Listen("unix", uc.handoffSocket)
	if err != nil {
		atomic.AddInt64(&uc.failedUpgrades, 1)
		return fmt.Errorf("failed to create handoff socket: %w", err)
	}
	defer func() {
		handoffListener.Close()
		os.Remove(uc.handoffSocket)
	}()

	// Step 3: Extract listening socket file descriptor
	listener := uc.server.listener
	tcpListener, ok := listener.(*net.UnixListener)
	if !ok {
		atomic.AddInt64(&uc.failedUpgrades, 1)
		return errors.New("listener is not a Unix socket")
	}

	file, err := tcpListener.File()
	if err != nil {
		atomic.AddInt64(&uc.failedUpgrades, 1)
		return fmt.Errorf("failed to get listener file: %w", err)
	}
	defer file.Close()

	// Step 4: Fork new process with special flags
	//nolint:gosec // G204: binaryPath is from our own executable, not user input
	cmd := exec.CommandContext(ctx, uc.binaryPath,
		"--fd-handoff", uc.handoffSocket,
		"--socket", uc.server.socketPath,
	)

	// Inherit environment but add upgrade marker
	cmd.Env = append(os.Environ(), "GISMO_UPGRADE=1")

	// Set up process attributes for clean handoff
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Start new process
	if err := cmd.Start(); err != nil {
		atomic.AddInt64(&uc.failedUpgrades, 1)
		return fmt.Errorf("failed to start new process: %w", err)
	}

	log.Printf("Started new process with PID %d", cmd.Process.Pid)

	// Step 5: Wait for new process to connect
	handoffConn, err := uc.acceptHandoff(handoffListener, 10*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		atomic.AddInt64(&uc.failedUpgrades, 1)
		return fmt.Errorf("handoff connection failed: %w", err)
	}
	defer handoffConn.Close()

	// Step 6: Send file descriptor to new process
	if err := uc.sendFileDescriptor(handoffConn, file); err != nil {
		_ = cmd.Process.Kill()
		atomic.AddInt64(&uc.failedUpgrades, 1)
		return fmt.Errorf("failed to send file descriptor: %w", err)
	}

	// Step 7: Wait for health check from new process
	if err := uc.waitForHealthCheck(handoffConn, 30*time.Second); err != nil {
		_ = cmd.Process.Kill()
		atomic.AddInt64(&uc.failedUpgrades, 1)
		return fmt.Errorf("health check failed: %w", err)
	}

	// Step 8: Stop accepting new connections
	uc.server.grpcServer.GracefulStop()

	// Step 9: Signal new process to take over
	if err := uc.sendTakeoverSignal(handoffConn); err != nil {
		_ = cmd.Process.Kill()
		atomic.AddInt64(&uc.failedUpgrades, 1)
		return fmt.Errorf("takeover signal failed: %w", err)
	}

	// Update metrics
	atomic.AddInt64(&uc.upgradeCount, 1)
	uc.mu.Lock()
	uc.lastUpgradeTime = time.Now()
	uc.mu.Unlock()

	log.Printf("Upgrade successful, new process PID %d has taken over", cmd.Process.Pid)

	// Step 10: Exit gracefully
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()

	return nil
}

// acceptHandoff waits for new process to connect
func (uc *UpgradeCoordinator) acceptHandoff(listener net.Listener, timeout time.Duration) (net.Conn, error) {
	connChan := make(chan net.Conn, 1)
	errChan := make(chan error, 1)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			errChan <- err
		} else {
			connChan <- conn
		}
	}()

	select {
	case conn := <-connChan:
		return conn, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(timeout):
		return nil, errors.New("handoff connection timeout")
	}
}

// sendFileDescriptor sends the listening socket FD to new process
func (uc *UpgradeCoordinator) sendFileDescriptor(conn net.Conn, file *os.File) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return errors.New("not a Unix connection")
	}

	// Get the raw connection
	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return fmt.Errorf("failed to get raw connection: %w", err)
	}

	// Send the file descriptor
	var sendErr error
	err = rawConn.Control(func(fd uintptr) {
		rights := syscall.UnixRights(int(file.Fd()))
		sendErr = syscall.Sendmsg(
			int(fd),
			[]byte("FD"),
			rights,
			nil,
			0,
		)
	})

	if err != nil {
		return fmt.Errorf("control error: %w", err)
	}
	if sendErr != nil {
		return fmt.Errorf("sendmsg error: %w", sendErr)
	}

	return nil
}

// waitForHealthCheck waits for new process to report ready
func (uc *UpgradeCoordinator) waitForHealthCheck(conn net.Conn, timeout time.Duration) error {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))

	// Read health check message
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read health check: %w", err)
	}

	msg := string(buf[:n])
	if msg != "READY" {
		return fmt.Errorf("unexpected health check response: %s", msg)
	}

	log.Println("New process health check passed")
	return nil
}

// sendTakeoverSignal tells new process to start accepting connections
func (uc *UpgradeCoordinator) sendTakeoverSignal(conn net.Conn) error {
	_, err := conn.Write([]byte("TAKEOVER"))
	if err != nil {
		return fmt.Errorf("failed to send takeover signal: %w", err)
	}
	return nil
}

// ReceiveFileDescriptor receives a file descriptor from parent process
func ReceiveFileDescriptor(handoffSocket string) (*os.File, error) {
	// Connect to parent's handoff socket
	conn, err := net.Dial("unix", handoffSocket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to handoff socket: %w", err)
	}
	defer conn.Close()

	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, errors.New("not a Unix connection")
	}

	// Receive the file descriptor
	buf := make([]byte, 2)
	oob := make([]byte, 1024)

	n, oobn, _, _, err := unixConn.ReadMsgUnix(buf, oob)
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	if n != 2 || string(buf) != "FD" {
		return nil, fmt.Errorf("unexpected message: %s", string(buf[:n]))
	}

	// Parse the file descriptor
	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return nil, fmt.Errorf("failed to parse control message: %w", err)
	}

	if len(scms) != 1 {
		return nil, fmt.Errorf("expected 1 control message, got %d", len(scms))
	}

	fds, err := syscall.ParseUnixRights(&scms[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse Unix rights: %w", err)
	}

	if len(fds) != 1 {
		return nil, fmt.Errorf("expected 1 file descriptor, got %d", len(fds))
	}

	// Create file from descriptor
	file := os.NewFile(uintptr(fds[0]), "listener")
	if file == nil {
		return nil, errors.New("failed to create file from descriptor")
	}

	// Send health check
	if _, err := conn.Write([]byte("READY")); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to send health check: %w", err)
	}

	// Wait for takeover signal
	buf = make([]byte, 256)
	n, err = conn.Read(buf)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read takeover signal: %w", err)
	}

	if string(buf[:n]) != "TAKEOVER" {
		file.Close()
		return nil, fmt.Errorf("unexpected takeover signal: %s", string(buf[:n]))
	}

	log.Println("File descriptor handoff successful")
	return file, nil
}

// GetMetrics returns upgrade metrics
func (uc *UpgradeCoordinator) GetMetrics() (upgradeCount, failedCount int64, lastUpgrade time.Time) {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return atomic.LoadInt64(&uc.upgradeCount), atomic.LoadInt64(&uc.failedUpgrades), uc.lastUpgradeTime
}

// UpgradeMessage represents messages exchanged during upgrade
type UpgradeMessage struct {
	Type    string
	Payload []byte
}

// EncodeMessage encodes a message for transmission
func EncodeMessage(msg *UpgradeMessage) ([]byte, error) {
	// Ensure type is max 8 bytes, pad with nulls if shorter
	typeBuf := make([]byte, 8)
	copy(typeBuf, []byte(msg.Type))

	// Combine type and payload
	payload := append(typeBuf, msg.Payload...)

	// Create length prefix
	lenBuf := make([]byte, 4)
	//nolint:gosec // G115: payload length is controlled and won't overflow
	binary.BigEndian.PutUint32(lenBuf, uint32(len(payload)))

	return append(lenBuf, payload...), nil
}

// DecodeMessage decodes a received message
func DecodeMessage(r io.Reader) (*UpgradeMessage, error) {
	// Read length prefix
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lenBuf)
	if length > 1024*1024 { // 1MB max message size
		return nil, errors.New("message too large")
	}

	// Read payload
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	// Parse message type (first 8 bytes)
	if len(payload) < 8 {
		return nil, errors.New("message too short")
	}

	// Extract type (first 8 bytes), trim null padding
	typeBuf := payload[:8]
	typeEnd := 8
	for i := 0; i < 8; i++ {
		if typeBuf[i] == 0 {
			typeEnd = i
			break
		}
	}

	// Extract payload (everything after the 8-byte type field)
	var msgPayload []byte
	if len(payload) > 8 {
		msgPayload = payload[8:]
	}

	return &UpgradeMessage{
		Type:    string(typeBuf[:typeEnd]),
		Payload: msgPayload,
	}, nil
}
