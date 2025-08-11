package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/jrossi/gismo/pkg/codesitter"
	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

var (
	port          = flag.Int("port", 50051, "The server port")
	workspaceRoot = flag.String("workspace", "", "Default workspace root")
	enableDebug   = flag.Bool("debug", false, "Enable debug logging")
)

func main() {
	flag.Parse()

	// Create listener
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(100*1024*1024), // 100MB
		grpc.MaxSendMsgSize(100*1024*1024), // 100MB
	)

	// Create and register CodeSitter service
	codeSitterServer := codesitter.NewServer()
	gismov1.RegisterCodeSitterServer(grpcServer, codeSitterServer)

	// Register reflection service for grpcurl and other tools
	reflection.Register(grpcServer)

	// If workspace provided, initialize it
	if *workspaceRoot != "" {
		ctx := context.Background()
		resp, err := codeSitterServer.InitializeWorkspace(ctx, &gismov1.InitializeWorkspaceRequest{
			WorkspaceRoot:            *workspaceRoot,
			EnableFileWatching:       true,
			EnableIncrementalParsing: true,
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize workspace: %v", err)
		} else {
			log.Printf("Workspace initialized: %d files parsed, %d symbols indexed",
				resp.FilesParsed, resp.TotalSymbols)
		}
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down CodeSitter server...")
		grpcServer.GracefulStop()
	}()

	// Start server
	log.Printf("CodeSitter server listening on port %d", *port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
