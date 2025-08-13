package server

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
	"github.com/jrossi/gismo/pkg/server/handlers"
)

// ErrNoUpgradeSupport is returned when upgrade is not supported
var ErrNoUpgradeSupport = errors.New("upgrade not supported in this mode")

// ServerOptions holds configuration for the server
type ServerOptions struct {
	SocketPath   string
	KnowledgeDB  *sql.DB
	AutoReload   bool
	FDHandoff    string
	Version      string
	Commit       string
	BuildDate    string
	OnUpgrade    func()
	ReloadConfig func() error
	RuntimeDir   string
}

// Option is a function that configures ServerOptions
type Option func(*ServerOptions)

// WithSocketPath sets a custom socket path
func WithSocketPath(path string) Option {
	return func(o *ServerOptions) {
		o.SocketPath = path
	}
}

// WithKnowledgeDB sets the knowledge database
func WithKnowledgeDB(db *sql.DB) Option {
	return func(o *ServerOptions) {
		o.KnowledgeDB = db
	}
}

// WithAutoReload enables automatic binary reloading
func WithAutoReload() Option {
	return func(o *ServerOptions) {
		o.AutoReload = true
	}
}

// WithFDHandoff sets the file descriptor handoff socket
func WithFDHandoff(socket string) Option {
	return func(o *ServerOptions) {
		o.FDHandoff = socket
	}
}

// WithVersion sets version information
func WithVersion(version, commit, buildDate string) Option {
	return func(o *ServerOptions) {
		o.Version = version
		o.Commit = commit
		o.BuildDate = buildDate
	}
}

// WithRuntimeDir sets a custom runtime directory
func WithRuntimeDir(dir string) Option {
	return func(o *ServerOptions) {
		o.RuntimeDir = dir
	}
}

// ExtendedServer represents the server with upgrade capabilities
type ExtendedServer struct {
	*Server
	options     *ServerOptions
	coordinator *UpgradeCoordinator
	watcher     *BinaryWatcher
	ctx         context.Context
	cancel      context.CancelFunc
}

// New creates a new extended server with options
func NewExtended(opts ...Option) (*ExtendedServer, error) {
	options := &ServerOptions{
		Version:   "dev",
		Commit:    "unknown",
		BuildDate: "unknown",
	}

	for _, opt := range opts {
		opt(options)
	}

	// Create base server
	var srv *Server
	var err error

	if options.RuntimeDir != "" {
		srv = NewWithRuntimeDir(options.RuntimeDir)
	} else if options.KnowledgeDB != nil {
		srv, err = NewWithKnowledgeDB(options.KnowledgeDB)
	} else {
		srv, err = New()
	}

	if err != nil {
		return nil, err
	}

	// Override socket path if specified
	if options.SocketPath != "" {
		srv.socketPath = options.SocketPath
	}

	ctx, cancel := context.WithCancel(context.Background())

	es := &ExtendedServer{
		Server:  srv,
		options: options,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Handle file descriptor handoff for upgrades
	if options.FDHandoff != "" {
		if err := es.handleFDHandoff(options.FDHandoff); err != nil {
			return nil, err
		}
	}

	return es, nil
}

// handleFDHandoff receives a file descriptor from parent process
func (es *ExtendedServer) handleFDHandoff(handoffSocket string) error {
	log.Println("Receiving file descriptor from parent process...")

	file, err := ReceiveFileDescriptor(handoffSocket)
	if err != nil {
		return err
	}

	// Create listener from file descriptor
	listener, err := net.FileListener(file)
	if err != nil {
		file.Close()
		return err
	}

	// Use the inherited listener
	es.listener = listener
	log.Println("Successfully inherited listener from parent process")
	return nil
}

// Start starts the extended server
func (es *ExtendedServer) Start() error {
	// If we inherited a listener, skip the normal startup
	if es.listener != nil && es.options.FDHandoff != "" {
		return es.startWithInheritedListener()
	}

	// Normal startup
	if err := es.Server.Start(); err != nil {
		return err
	}

	// Set up upgrade coordinator if not in handoff mode
	if es.options.FDHandoff == "" {
		coordinator, err := NewUpgradeCoordinator(es.Server)
		if err != nil {
			log.Printf("Warning: Failed to create upgrade coordinator: %v", err)
		} else {
			es.coordinator = coordinator

			// Set up binary watcher if auto-reload is enabled
			if es.options.AutoReload {
				watcher, err := NewBinaryWatcher(coordinator)
				if err != nil {
					log.Printf("Warning: Failed to create binary watcher: %v", err)
				} else {
					es.watcher = watcher
					es.watcher.Start(es.ctx)
				}
			}
		}
	}

	return nil
}

// startWithInheritedListener starts the server with an inherited listener
func (es *ExtendedServer) startWithInheritedListener() error {
	// We already have the listener from handleFDHandoff
	// Just need to set up the gRPC server and start serving

	// Try to acquire lock (parent will release it after handoff)
	time.Sleep(100 * time.Millisecond) // Give parent time to release
	if err := es.Server.acquireLock(); err != nil {
		// This might be okay if parent hasn't released yet
		log.Printf("Warning: Could not acquire lock immediately: %v", err)
	}

	// Initialize gRPC server (copied from Server.Start)
	es.Server.grpcServer = grpc.NewServer()

	// Register knowledge service if database is available
	if es.Server.knowledgeDB != nil {
		knowledgeHandler := handlers.NewKnowledgeHandlerFromDB(es.Server.knowledgeDB)
		gismov1.RegisterKnowledgeServiceServer(es.Server.grpcServer, knowledgeHandler)
	}

	// Register CodeSitter service
	codeSitterHandler := handlers.NewCodeSitterHandler()
	gismov1.RegisterCodeSitterServer(es.Server.grpcServer, codeSitterHandler)

	// Start serving in background
	go func() {
		if err := es.Server.grpcServer.Serve(es.listener); err != nil {
			// Server was likely stopped gracefully
			_ = err
		}
	}()

	log.Println("Server started with inherited listener")
	return nil
}

// TriggerUpgrade manually triggers an upgrade
func (es *ExtendedServer) TriggerUpgrade() error {
	if es.coordinator == nil {
		return ErrNoUpgradeSupport
	}
	return es.coordinator.TriggerUpgrade(es.ctx)
}

// ReloadConfig reloads the configuration
func (es *ExtendedServer) ReloadConfig() error {
	if es.options.ReloadConfig != nil {
		return es.options.ReloadConfig()
	}
	log.Println("Configuration reload requested but no handler configured")
	return nil
}

// Shutdown gracefully shuts down the server
func (es *ExtendedServer) Shutdown(ctx context.Context) error {
	es.cancel()

	if es.watcher != nil {
		es.watcher.Stop()
	}

	// Use base server's Close method
	return es.Server.Close()
}

// GetUpgradeMetrics returns upgrade metrics
func (es *ExtendedServer) GetUpgradeMetrics() (upgradeCount, failedCount int64, lastUpgrade time.Time) {
	if es.coordinator != nil {
		return es.coordinator.GetMetrics()
	}
	return 0, 0, time.Time{}
}
