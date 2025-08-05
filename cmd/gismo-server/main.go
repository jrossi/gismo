package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jrossi/gismo/pkg/knowledge"
	"github.com/jrossi/gismo/pkg/server"
)

func main() {
	var enableKnowledge = flag.Bool("knowledge", true, "Enable knowledge database support")
	flag.Parse()

	var srv *server.Server
	var err error

	if *enableKnowledge {
		// Try to create server with knowledge database
		ctx := context.Background()
		store, err := knowledge.New(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create knowledge store: %v\n", err)
			// Fall back to regular server
			srv, err = server.New()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
				os.Exit(1)
			}
		} else {
			defer store.Close()
			srv, err = server.NewWithKnowledgeDB(store.DB())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create server with knowledge DB: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Server started with knowledge database support")
		}
	} else {
		// Create regular server
		srv, err = server.New()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
			os.Exit(1)
		}
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
