#!/bin/bash
set -e

echo "=== Testing Gismo Knowledge System ==="
echo

# Kill any existing server
echo "1. Stopping any existing gismo-server..."
pkill -f gismo-server || true
sleep 1

# Remove lock file if exists
RUNTIME_DIR="/var/folders/_q/z5_d6l7d6zb8zt2fpg1xxvz80000gp/T/gismo-502"
if [ -f "$RUNTIME_DIR/gismo.lock" ]; then
    echo "   Removing stale lock file..."
    rm -f "$RUNTIME_DIR/gismo.lock"
fi

# Start the server with knowledge DB
echo "2. Starting gismo-server with knowledge database..."
echo "   Creating knowledge database at ~/.gismo/knowledge.db"

# Create a simple Go program to start server with knowledge DB
cat > /tmp/start-knowledge-server.go << 'EOF'
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/jrossi/gismo/pkg/knowledge"
    "github.com/jrossi/gismo/pkg/server"
)

func main() {
    ctx := context.Background()
    
    // Create knowledge store
    store, err := knowledge.New(ctx)
    if err != nil {
        log.Fatalf("Failed to create knowledge store: %v", err)
    }
    defer store.Close()
    
    // Create server with knowledge DB
    srv, err := server.NewWithKnowledgeDB(store.DB())
    if err != nil {
        log.Fatalf("Failed to create server: %v", err)
    }
    
    // Start server
    if err := srv.Start(); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
    
    fmt.Println("Server started with knowledge database support")
    
    // Wait for interrupt
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    <-sigCh
    
    fmt.Println("\nShutting down...")
    srv.Close()
}
EOF

# Build and run the server
echo "   Building custom server starter..."
go build -o /tmp/gismo-knowledge-server /tmp/start-knowledge-server.go

echo "   Starting server..."
/tmp/gismo-knowledge-server &
SERVER_PID=$!
sleep 2

# Test the connection
echo
echo "3. Testing knowledge database connection..."
echo "   Running test query..."

if ./build/bin/gismo-query "SELECT 'Hello from DuckDB!' as greeting, current_timestamp as time"; then
    echo
    echo "✅ Success! Knowledge database is working!"
    echo
    echo "4. Testing table creation..."
    ./build/bin/gismo-query "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name" || true
    
    echo
    echo "5. Interactive mode test (type .exit to quit):"
    echo "   You can now run SQL queries interactively..."
    echo
    
    # Give user option to test interactively
    read -p "Press Enter to start interactive mode or Ctrl+C to skip: "
    ./build/bin/gismo-query
else
    echo "❌ Failed to connect to knowledge database"
fi

# Cleanup
echo
echo "Cleaning up..."
kill $SERVER_PID 2>/dev/null || true
rm -f /tmp/start-knowledge-server.go /tmp/gismo-knowledge-server

echo "Done!"