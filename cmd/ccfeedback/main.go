package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jrossi/ccfeedback"
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
		timeout     = flag.Duration("timeout", 60*time.Second, "Hook execution timeout")
		showVersion = flag.Bool("version", false, "Show version information")
		debug       = flag.Bool("debug", false, "Enable debug output")
		configFile  = flag.String("config", "", "Path to configuration file")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CCFeedback - Claude Code Hooks Feedback System\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [command] [arguments]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  init                    Set up ccfeedback in Claude Code settings\n")
		fmt.Fprintf(os.Stderr, "  show <command>          Show various information (config, filter, setup, linters)\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nDefault behavior (no command):\n")
		fmt.Fprintf(os.Stderr, "  The tool reads hook messages from stdin and writes responses to stdout.\n")
		fmt.Fprintf(os.Stderr, "\nExit codes:\n")
		fmt.Fprintf(os.Stderr, "  0 - Success (stdout shown in transcript)\n")
		fmt.Fprintf(os.Stderr, "  2 - Blocking error (stderr processed by Claude)\n")
		fmt.Fprintf(os.Stderr, "  Other - Non-blocking error\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("ccfeedback version %s\n", version)
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

	// Load configuration
	configLoader, err := ccfeedback.NewConfigLoader()
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Failed to create config loader: %v\n", err)
		}
		// Continue without config
		configLoader = nil
	}

	var appConfig *ccfeedback.AppConfig
	if configLoader != nil {
		if *configFile != "" {
			// Load specific config file
			appConfig, err = configLoader.LoadConfigWithPaths([]string{*configFile})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load config file %s: %v\n", *configFile, err)
				os.Exit(1)
			}
		} else {
			// Load default config files
			appConfig, err = configLoader.LoadConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
				os.Exit(1)
			}
		}
	}

	// Create linting config from app config
	lintingConfig := ccfeedback.LintingConfig{}
	if appConfig != nil {
		if appConfig.Parallel != nil {
			if appConfig.Parallel.MaxWorkers != nil {
				lintingConfig.MaxWorkers = *appConfig.Parallel.MaxWorkers
			}
			if appConfig.Parallel.DisableParallel != nil {
				lintingConfig.DisableParallel = *appConfig.Parallel.DisableParallel
			}
		}
		// Override timeout if specified in config
		if appConfig.Timeout != nil {
			*timeout = appConfig.Timeout.Duration
		}
	}

	// Create rule engine with linting capabilities
	ruleEngine := ccfeedback.NewLintingRuleEngineWithConfig(lintingConfig)

	// Set the app config if available
	if appConfig != nil {
		ruleEngine.SetAppConfig(appConfig)
	}

	// Check for subcommands
	args := flag.Args()
	if len(args) > 0 && args[0] == "init" {
		// Dispatch to ccfeedback-init binary
		subcommand := "ccfeedback-init"

		// Try to find the subcommand in the same directory as the main binary
		execPath, err := os.Executable()
		if err == nil {
			dir := filepath.Dir(execPath)
			localSubcommand := filepath.Join(dir, subcommand)
			if _, err := os.Stat(localSubcommand); err == nil {
				subcommand = localSubcommand
			}
		}

		cmd := exec.Command(subcommand, args[1:]...) // #nosec G204 - subcommand is controlled
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()

		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Fprintf(os.Stderr, "Error: failed to execute %s: %v\n", subcommand, err)
			os.Exit(1)
		}
		os.Exit(0)
	} else if len(args) > 0 && (args[0] == "show" || args[0] == "show-actions") {
		// Dispatch to ccfeedback-show binary
		subcommand := "ccfeedback-show"

		// Try to find the subcommand in the same directory as the main binary
		execPath, err := os.Executable()
		if err == nil {
			dir := filepath.Dir(execPath)
			localSubcommand := filepath.Join(dir, subcommand)
			if _, err := os.Stat(localSubcommand); err == nil {
				subcommand = localSubcommand
			}
		}

		// Handle backward compatibility for show-actions
		showArgs := args[1:]
		if args[0] == "show-actions" {
			// Convert "show-actions <file>" to "show filter <file>"
			// Note: show filter only supports one file at a time
			if len(args) > 1 {
				showArgs = []string{"filter", args[1]}
			} else {
				showArgs = []string{"filter"}
			}
		}

		// Pass through flags if specified
		if *debug {
			showArgs = append([]string{"--debug"}, showArgs...)
		}
		if *configFile != "" {
			showArgs = append([]string{"--config", *configFile}, showArgs...)
		}

		// Debug output
		if *debug {
			fmt.Fprintf(os.Stderr, "Debug: configFile=%s\n", *configFile)
			fmt.Fprintf(os.Stderr, "Debug: showArgs=%v\n", showArgs)
		}

		cmd := exec.Command(subcommand, showArgs...) // #nosec G204 - subcommand is controlled
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()

		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Fprintf(os.Stderr, "Error: failed to execute %s: %v\n", subcommand, err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Default behavior: process hook from stdin
	ctx := context.Background()

	// Try to use daemon first
	if tryDaemonMode(ctx, *debug) {
		// Successfully processed via daemon
		return
	}

	// Fallback to direct execution
	// Create executor
	executor := ccfeedback.NewExecutor(ruleEngine)
	executor.SetTimeout(*timeout)

	// Execute
	exitCode, err := executor.ExecuteWithExitCode(ctx)

	// Always flush both stdout and stderr before exiting
	os.Stdout.Sync()
	os.Stderr.Sync()

	if err != nil {
		// Errors are non-blocking (exit 1) and shown on stderr
		fmt.Fprintf(os.Stderr, "\n> Hook execution error:\n")
		fmt.Fprintf(os.Stderr, "  - [ccfeedback]: ❌ %v\n", err)
		if *debug {
			fmt.Fprintf(os.Stderr, "  - Debug: Full error: %v\n", err)
		}
		// Default to non-blocking error
		os.Exit(1)
	}

	// Show status for successful exit codes in debug mode
	if exitCode == 0 && *debug {
		// Success messages go to stdout for exit code 0
		fmt.Fprintf(os.Stdout, "\n> Hook execution completed:\n")
		fmt.Fprintf(os.Stdout, "  - [ccfeedback]: ✅ Success (exit code 0)\n")
	}

	// Exit with the proper code
	os.Exit(exitCode)
}

// tryDaemonMode attempts to process the hook via the daemon
func tryDaemonMode(ctx context.Context, debug bool) bool {
	// Only use daemon mode if explicitly enabled
	if os.Getenv("CCFEEDBACK_DAEMON") != "1" {
		if debug {
			fmt.Fprintf(os.Stderr, "Debug: Daemon mode disabled (CCFEEDBACK_DAEMON != 1)\n")
		}
		return false
	}
	if debug {
		fmt.Fprintf(os.Stderr, "Debug: Daemon mode enabled\n")
	}

	// Read stdin data
	stdinData, err := io.ReadAll(os.Stdin)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "Failed to read stdin: %v\n", err)
		}
		return false
	}
	if debug {
		fmt.Fprintf(os.Stderr, "Debug: Read %d bytes from stdin\n", len(stdinData))
		fmt.Fprintf(os.Stderr, "Debug: stdin data: %q\n", string(stdinData))
	}

	// Create daemon client
	client, err := daemon.NewClient()
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "Failed to create daemon client: %v\n", err)
		}
		return false
	}

	// Try to send request to daemon
	resp, err := client.SendHookRequest(ctx, stdinData)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "Failed to process via daemon: %v\n", err)
		}
		return false
	}

	// Write response
	if len(resp.Stdout) > 0 {
		os.Stdout.Write(resp.Stdout)
	}
	if len(resp.Stderr) > 0 {
		os.Stderr.Write(resp.Stderr)
	}

	// Always flush both stdout and stderr before exiting
	os.Stdout.Sync()
	os.Stderr.Sync()

	// Exit with daemon's exit code
	os.Exit(resp.ExitCode)
	return true // This line won't be reached due to os.Exit
}
