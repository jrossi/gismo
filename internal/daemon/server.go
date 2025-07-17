package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Version is the daemon version
const Version = "1.0.0"

// Server represents the daemon server
type Server struct {
	socketPath string
	listener   net.Listener

	// Shutdown management
	shutdown     chan struct{}
	shutdownOnce sync.Once
	wg           sync.WaitGroup

	// Statistics
	startTime time.Time
	processed atomic.Int64

	// Idle timeout
	idleTimeout time.Duration
	lastRequest atomic.Value // stores time.Time
}

// NewServer creates a new daemon server
func NewServer(idleTimeout time.Duration) (*Server, error) {
	socketPath, err := GetSocketPath()
	if err != nil {
		return nil, fmt.Errorf("getting socket path: %w", err)
	}

	// Remove existing socket if it exists
	os.Remove(socketPath)

	s := &Server{
		socketPath:  socketPath,
		shutdown:    make(chan struct{}),
		startTime:   time.Now(),
		idleTimeout: idleTimeout,
	}

	s.lastRequest.Store(time.Now())

	return s, nil
}

// Start starts the daemon server
func (s *Server) Start(ctx context.Context) error {
	// Create listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("creating unix listener: %w", err)
	}
	s.listener = listener

	// Set socket permissions
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		listener.Close()
		return fmt.Errorf("setting socket permissions: %w", err)
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start accept loop
	s.wg.Add(1)
	go s.acceptLoop()

	// Start idle timeout checker
	s.wg.Add(1)
	go s.idleTimeoutChecker(ctx)

	// Wait for shutdown
	select {
	case <-ctx.Done():
	case <-s.shutdown:
	case <-sigChan:
	}

	// Graceful shutdown
	s.Stop()
	return nil
}

// Stop stops the daemon server
func (s *Server) Stop() {
	s.shutdownOnce.Do(func() {
		close(s.shutdown)

		// Close listener
		if s.listener != nil {
			s.listener.Close()
		}

		// Wait for connections to finish
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(30 * time.Second):
			fmt.Fprintf(os.Stderr, "Warning: forced shutdown after timeout\n")
		}

		// Clean up socket
		os.Remove(s.socketPath)
	})
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return
			default:
				fmt.Fprintf(os.Stderr, "Accept error: %v\n", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Update last request time
	s.lastRequest.Store(time.Now())

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err != io.EOF {
				s.sendError(encoder, "", fmt.Sprintf("decoding request: %v", err))
			}
			return
		}

		start := time.Now()

		switch req.Type {
		case "ping":
			s.handlePing(encoder, &req)
		case "hook":
			s.handleHook(encoder, &req)
		default:
			s.sendError(encoder, req.ID, fmt.Sprintf("unknown request type: %s", req.Type))
		}

		// Update statistics
		s.processed.Add(1)
		s.lastRequest.Store(time.Now())

		// Log request for debugging
		duration := time.Since(start)
		fmt.Fprintf(os.Stderr, "Processed %s request in %v\n", req.Type, duration)
	}
}

// handlePing handles a ping request
func (s *Server) handlePing(encoder *json.Encoder, req *Request) {
	pingResp := PingResponse{
		Version:   Version,
		Uptime:    time.Since(s.startTime),
		Processed: s.processed.Load(),
	}

	respData, _ := json.Marshal(pingResp)

	resp := Response{
		ID:       req.ID,
		Success:  true,
		Payload:  respData,
		Duration: time.Since(s.startTime),
	}

	_ = encoder.Encode(resp)
}

// handleHook handles a hook processing request
func (s *Server) handleHook(encoder *json.Encoder, req *Request) {
	start := time.Now()

	// Unmarshal hook request
	var hookReq HookRequest
	if err := json.Unmarshal(req.Payload, &hookReq); err != nil {
		s.sendError(encoder, req.ID, fmt.Sprintf("unmarshaling hook request: %v", err))
		return
	}

	// Set environment variables
	for k, v := range hookReq.Env {
		os.Setenv(k, v)
	}

	// Process the hook
	stdout, stderr, exitCode := s.processHook(hookReq.StdinData)

	// Create response
	hookResp := HookResponse{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}

	respData, _ := json.Marshal(hookResp)

	resp := Response{
		ID:       req.ID,
		Success:  true,
		Payload:  respData,
		Duration: time.Since(start),
	}

	_ = encoder.Encode(resp)
}

// processHook processes a hook message
func (s *Server) processHook(stdinData []byte) ([]byte, []byte, int) {
	// The daemon should not process hooks itself
	// It should be a simple relay that returns the data as-is
	// The actual processing happens in the ccfeedback client
	return stdinData, nil, 0
}

// sendError sends an error response
func (s *Server) sendError(encoder *json.Encoder, id string, errMsg string) {
	resp := Response{
		ID:      id,
		Success: false,
		Error:   errMsg,
	}
	_ = encoder.Encode(resp)
}

// idleTimeoutChecker checks for idle timeout
func (s *Server) idleTimeoutChecker(ctx context.Context) {
	defer s.wg.Done()

	if s.idleTimeout <= 0 {
		return // No idle timeout
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			lastReq, ok := s.lastRequest.Load().(time.Time)
			if ok && time.Since(lastReq) > s.idleTimeout {
				fmt.Fprintf(os.Stderr, "Idle timeout reached, shutting down\n")
				s.Stop()
				return
			}
		}
	}
}
