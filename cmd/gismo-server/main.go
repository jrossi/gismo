package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jrossi/gismo/pkg/server"
)

func main() {
	// Create server
	srv, err := server.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}

	// Start server
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan

	// Clean up
	if err := srv.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close server: %v\n", err)
		os.Exit(1)
	}
}
