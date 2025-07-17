package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/jrossi/ccfeedback/internal/daemon"
)

// Build variables injected via ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = ""
)

func main() {
	var (
		idleTimeout = flag.Duration("idle-timeout", 30*time.Minute, "Idle timeout before daemon shuts down")
		showVersion = flag.Bool("version", false, "Show version information")
		detach      = flag.Bool("detach", false, "Detach from parent process (daemonize)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CCFeedback Daemon - Background service for ccfeedback\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("ccfeedback-daemon version %s\n", version)
		if commit != "none" {
			fmt.Printf("  commit: %s\n", commit)
		}
		if date != "unknown" {
			fmt.Printf("  built at: %s\n", date)
		}
		if builtBy != "" {
			fmt.Printf("  built by: %s\n", builtBy)
		}
		os.Exit(0)
	}

	// Check if we should daemonize
	if *detach && os.Getenv("CCFEEDBACK_DAEMON_DETACHED") != "1" {
		// Re-execute ourselves with detached flag
		cmd := exec.Command(os.Args[0], os.Args[1:]...)
		cmd.Env = append(os.Environ(), "CCFEEDBACK_DAEMON_DETACHED=1")

		// Detach from parent
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}

		// Redirect to /dev/null
		devNull, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open /dev/null: %v\n", err)
			os.Exit(1)
		}
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull

		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start detached daemon: %v\n", err)
			os.Exit(1)
		}

		// Parent exits
		fmt.Printf("Daemon started with PID %d\n", cmd.Process.Pid)
		os.Exit(0)
	}

	// Check for existing daemon using lock file
	lockPath, err := daemon.GetLockPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get lock path: %v\n", err)
		os.Exit(1)
	}

	// Try to acquire lock
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open lock file: %v\n", err)
		os.Exit(1)
	}
	defer lockFile.Close()

	// Try to lock the file
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Another daemon is running
		fmt.Fprintf(os.Stderr, "Another daemon instance is already running\n")
		os.Exit(1)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	// Write PID to lock file
	fmt.Fprintf(lockFile, "%d\n", os.Getpid())

	// Create and start server
	server, err := daemon.NewServer(*idleTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	fmt.Fprintf(os.Stderr, "Starting ccfeedback daemon...\n")
	if err := server.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}

	// Clean up lock file
	os.Remove(lockPath)
}
