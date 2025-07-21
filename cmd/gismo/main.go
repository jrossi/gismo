package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jrossi/gismo"
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
		fmt.Fprintf(os.Stderr, "  init                    Set up gismo in Claude Code settings\n")
		fmt.Fprintf(os.Stderr, "  show <command>          Show various information (config, filter, setup, linters)\n")
		fmt.Fprintf(os.Stderr, "  registry <subcommand>   Manage package registries (add, remove, list, update)\n")
		fmt.Fprintf(os.Stderr, "  package <subcommand>    Manage packages (install, remove, list, update)\n")
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

	// Load configuration
	configLoader, err := gismo.NewConfigLoader()
	if err != nil {
		if *debug {
			fmt.Fprintf(os.Stderr, "Failed to create config loader: %v\n", err)
		}
		// Continue without config
		configLoader = nil
	}

	var appConfig *gismo.AppConfig
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
	lintingConfig := gismo.LintingConfig{}
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
	ruleEngine := gismo.NewLintingRuleEngineWithConfig(lintingConfig)

	// Set the app config if available
	if appConfig != nil {
		ruleEngine.SetAppConfig(appConfig)
	}

	// Check for subcommands
	args := flag.Args()
	if len(args) > 0 && args[0] == "init" {
		// Dispatch to gismo-init binary
		subcommand := "gismo-init"

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
		// Dispatch to gismo-show binary
		subcommand := "gismo-show"

		// Try to find the subcommand in the same directory as the main binary
		execPath, err := os.Executable()
		if err == nil {
			dir := filepath.Dir(execPath)
			localSubcommand := filepath.Join(dir, subcommand)
			if _, err := os.Stat(localSubcommand); err == nil {
				subcommand = localSubcommand
			}
		}

		// Build arguments for show command
		var showArgs []string

		// Add config flag if it was provided
		if *configFile != "" {
			showArgs = append(showArgs, "--config", *configFile)
		}
		// Add debug flag if it was provided
		if *debug {
			showArgs = append(showArgs, "--debug")
		}

		// Handle backward compatibility for show-actions
		if args[0] == "show-actions" {
			// For show-actions, just pass the files directly
			showArgs = append(showArgs, args[1:]...)
		} else {
			// Regular show command
			showArgs = append(showArgs, args[1:]...)
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
	} else if len(args) > 0 && args[0] == "registry" {
		// Dispatch to gismo-registry binary
		subcommand := "gismo-registry"

		// Try to find the subcommand in the same directory as the main binary
		execPath, err := os.Executable()
		if err == nil {
			dir := filepath.Dir(execPath)
			localSubcommand := filepath.Join(dir, subcommand)
			if _, err := os.Stat(localSubcommand); err == nil {
				subcommand = localSubcommand
			}
		}

		// Build arguments for registry command
		var registryArgs []string

		// Add config flag if it was provided
		if *configFile != "" {
			registryArgs = append(registryArgs, "--config", *configFile)
		}
		// Add debug flag if it was provided
		if *debug {
			registryArgs = append(registryArgs, "--debug")
		}

		// Add registry subcommand and its arguments
		registryArgs = append(registryArgs, args[1:]...)

		cmd := exec.Command(subcommand, registryArgs...) // #nosec G204 - subcommand is controlled
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
	} else if len(args) > 0 && args[0] == "package" {
		// Dispatch to gismo-package binary
		subcommand := "gismo-package"

		// Try to find the subcommand in the same directory as the main binary
		execPath, err := os.Executable()
		if err == nil {
			dir := filepath.Dir(execPath)
			localSubcommand := filepath.Join(dir, subcommand)
			if _, err := os.Stat(localSubcommand); err == nil {
				subcommand = localSubcommand
			}
		}

		// Build arguments for package command
		var packageArgs []string

		// Add config flag if it was provided
		if *configFile != "" {
			packageArgs = append(packageArgs, "--config", *configFile)
		}
		// Add debug flag if it was provided
		if *debug {
			packageArgs = append(packageArgs, "--debug")
		}

		// Add package subcommand and its arguments
		packageArgs = append(packageArgs, args[1:]...)

		cmd := exec.Command(subcommand, packageArgs...) // #nosec G204 - subcommand is controlled
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
	// Create executor
	executor := gismo.NewExecutor(ruleEngine)
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
