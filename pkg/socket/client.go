package socket

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GetSocketPath returns the standard Unix domain socket path for gismo-server
func GetSocketPath() string {
	var socketDir string
	if runtime := os.Getenv("XDG_RUNTIME_DIR"); runtime != "" {
		socketDir = filepath.Join(runtime, "gismo")
	} else {
		// Use temp dir with UID suffix like the server does
		socketDir = filepath.Join(os.TempDir(), fmt.Sprintf("gismo-%d", os.Getuid()))
	}
	return filepath.Join(socketDir, "gismo.sock")
}

// Connect establishes a gRPC connection to gismo-server via Unix domain socket
func Connect(ctx context.Context) (*grpc.ClientConn, error) {
	socketPath := GetSocketPath()

	// Check if socket exists
	if _, err := os.Stat(socketPath); err != nil {
		return nil, fmt.Errorf("gismo-server not running (socket not found at %s)", socketPath)
	}

	// Connect via Unix socket
	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gismo-server: %w", err)
	}

	return conn, nil
}

// ConnectWithFallback tries Unix socket first, then falls back to TCP
func ConnectWithFallback(ctx context.Context, tcpAddr string) (*grpc.ClientConn, error) {
	// Only use Unix socket - no TCP fallback for security
	return Connect(ctx)
}
