package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jrossi/gismo/pkg/client"
	"github.com/jrossi/gismo/pkg/engine"
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

	// Create subcommand registry
	registry, err := engine.NewSubcommandRegistry()
	if err != nil {
		// Fallback to basic usage if we can't create registry
		registry = nil
		if *debug {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create subcommand registry: %v\n", err)
		}
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CCFeedback - Claude Code Hooks Feedback System\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [command] [arguments]\n\n", os.Args[0])

		// Use dynamic command list from registry if available
		if registry != nil {
			registry.PrintUsage()
		} else {
			// Fallback to static list
			fmt.Fprintf(os.Stderr, "Commands:\n")
			fmt.Fprintf(os.Stderr, "  init                    Set up gismo in Claude Code settings\n")
			fmt.Fprintf(os.Stderr, "  show <command>          Show various information (config, filter, setup, linters)\n")
			fmt.Fprintf(os.Stderr, "  registry <subcommand>   Manage package registries (add, remove, list, update)\n")
			fmt.Fprintf(os.Stderr, "  package <subcommand>    Manage packages (install, remove, list, update)\n")
			fmt.Fprintf(os.Stderr, "  query [SQL]             Execute SQL queries against the knowledge database\n")
		}

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

	// Ensure server is running
	cli, err := client.New()
	if err == nil {
		// Ignore error - server startup is optional
		_ = cli.EnsureServerRunning()
	}

	if *showVersion {
		fmt.Printf("gismo version %s\n", version)
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

	// Check for subcommands
	args := flag.Args()
	if len(args) > 0 && registry != nil {
		// Try to find and execute subcommand
		cmdName := args[0]

		// Special handling for "help" command to show available subcommands
		if cmdName == "help" || cmdName == "commands" {
			fmt.Fprintf(os.Stdout, "Available gismo subcommands:\n\n")
			registry.PrintUsage()
			os.Exit(0)
		}

		// Special handling for show-actions (backward compatibility)
		if cmdName == "show-actions" {
			cmdInfo, found := registry.GetCommand("show")
			if found {
				// Prepare flags to pass through
				flags := map[string]interface{}{
					"--config": configFile,
					"--debug":  *debug,
				}

				// Execute the show command with remaining args
				if err := registry.Execute(cmdInfo, args[1:], flags); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
		}

		// Look up the subcommand
		if cmdInfo, found := registry.GetCommand(cmdName); found {
			// Prepare flags to pass through
			flags := map[string]interface{}{
				"--config":  configFile,
				"--debug":   *debug,
				"--timeout": timeout.String(),
			}

			// Execute the subcommand
			if err := registry.Execute(cmdInfo, args[1:], flags); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		// Unknown subcommand - fall through to default behavior
		// This allows for future extensibility
	}

	// Load configuration for default hook processing
	configLoader, err := engine.NewConfigLoader()
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Failed to create config loader: %v\n", err)
		}
		// Continue without config
		configLoader = nil
	}

	var appConfig *engine.AppConfig
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
	lintingConfig := engine.LintingConfig{}
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
	ruleEngine := engine.NewLintingRuleEngineWithConfig(lintingConfig)

	// Set the app config if available
	if appConfig != nil {
		ruleEngine.SetAppConfig(appConfig)
	}

	// Default behavior: process hook from stdin
	// Create executor
	executor := engine.NewExecutor(ruleEngine)
	executor.SetTimeout(*timeout)

	// Create context
	ctx := context.Background()

	// Execute
	exitCode, err := executor.ExecuteWithExitCode(ctx)

	// Always flush both stdout and stderr before exiting
	os.Stdout.Sync()
	os.Stderr.Sync()

	if err != nil {
		// Errors are non-blocking (exit 1) and shown on stderr
		fmt.Fprintf(os.Stderr, "\n> Hook execution error:\n")
		fmt.Fprintf(os.Stderr, "  - [gismo]: ❌ %v\n", err)
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
		fmt.Fprintf(os.Stdout, "  - [gismo]: ✅ Success (exit code 0)\n")
	}

	// Exit with the proper code
	os.Exit(exitCode)
}
