package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// SubcommandInfo represents metadata about a subcommand
type SubcommandInfo struct {
	Name        string   // The subcommand name (e.g., "init", "show")
	Binary      string   // The binary name (e.g., "gismo-init", "gismo-show")
	Description string   // Human-readable description
	PassFlags   []string // Flags to pass through from main command
}

// SubcommandRegistry manages available subcommands
type SubcommandRegistry struct {
	commands map[string]*SubcommandInfo
	binDir   string // Directory containing gismo binaries
}

// NewSubcommandRegistry creates a new subcommand registry
func NewSubcommandRegistry() (*SubcommandRegistry, error) {
	// Find the directory containing the main gismo binary
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	binDir := filepath.Dir(execPath)

	registry := &SubcommandRegistry{
		commands: make(map[string]*SubcommandInfo),
		binDir:   binDir,
	}

	// Register known subcommands with their metadata
	registry.registerKnownCommands()

	// Discover additional subcommands dynamically
	if err := registry.discoverCommands(); err != nil {
		// Non-fatal: we can still use registered commands
		// Just log the error if debug is enabled
		_ = err
	}

	return registry, nil
}

// registerKnownCommands registers the known subcommands with their metadata
func (r *SubcommandRegistry) registerKnownCommands() {
	// Register known commands with descriptions
	knownCommands := []SubcommandInfo{
		{
			Name:        "init",
			Binary:      "gismo-init",
			Description: "Set up gismo in Claude Code settings",
			PassFlags:   []string{},
		},
		{
			Name:        "show",
			Binary:      "gismo-show",
			Description: "Show various information (config, filter, setup, linters)",
			PassFlags:   []string{"--config", "--debug"},
		},
		{
			Name:        "registry",
			Binary:      "gismo-registry",
			Description: "Manage package registries (add, remove, list, update)",
			PassFlags:   []string{"--config", "--debug"},
		},
		{
			Name:        "package",
			Binary:      "gismo-package",
			Description: "Manage packages (install, remove, list, update)",
			PassFlags:   []string{"--config", "--debug"},
		},
		{
			Name:        "query",
			Binary:      "gismo-query",
			Description: "Execute SQL queries against the knowledge database",
			PassFlags:   []string{"--debug", "--timeout"},
		},
		{
			Name:        "knowledge",
			Binary:      "gismo-knowledge",
			Description: "Manage knowledge database (import, search, analyze)",
			PassFlags:   []string{"--debug"},
		},
		{
			Name:        "server",
			Binary:      "gismo-server",
			Description: "Run the gismo gRPC server",
			PassFlags:   []string{"--debug"},
		},
		{
			Name:        "codesitter",
			Binary:      "gismo-codesitter",
			Description: "Code analysis using tree-sitter",
			PassFlags:   []string{"--debug"},
		},
		{
			Name:        "mcp",
			Binary:      "gismo-mcp",
			Description: "MCP server for Claude Code integration",
			PassFlags:   []string{},
		},
	}

	for _, cmd := range knownCommands {
		cmd := cmd // Capture loop variable
		r.commands[cmd.Name] = &cmd
	}
}

// discoverCommands dynamically discovers gismo-* binaries in the bin directory
func (r *SubcommandRegistry) discoverCommands() error {
	// List all files in the bin directory
	entries, err := os.ReadDir(r.binDir)
	if err != nil {
		return fmt.Errorf("failed to read bin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Look for gismo-* binaries
		if strings.HasPrefix(name, "gismo-") && name != "gismo" {
			// Extract subcommand name
			cmdName := strings.TrimPrefix(name, "gismo-")

			// Skip if already registered
			if _, exists := r.commands[cmdName]; exists {
				continue
			}

			// Check if it's executable
			fullPath := filepath.Join(r.binDir, name)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}

			// Check if file is executable (Unix)
			if info.Mode()&0111 == 0 {
				continue
			}

			// Register discovered command
			r.commands[cmdName] = &SubcommandInfo{
				Name:        cmdName,
				Binary:      name,
				Description: fmt.Sprintf("Run %s subcommand", cmdName),
				PassFlags:   []string{"--debug"}, // Default flags to pass
			}
		}
	}

	return nil
}

// GetCommand returns information about a subcommand
func (r *SubcommandRegistry) GetCommand(name string) (*SubcommandInfo, bool) {
	// Check for exact match
	if cmd, ok := r.commands[name]; ok {
		return cmd, true
	}

	// Handle special case for "show-actions" (backward compatibility)
	if name == "show-actions" {
		if cmd, ok := r.commands["show"]; ok {
			return cmd, true
		}
	}

	return nil, false
}

// ListCommands returns all available commands
func (r *SubcommandRegistry) ListCommands() []*SubcommandInfo {
	var commands []*SubcommandInfo
	for _, cmd := range r.commands {
		commands = append(commands, cmd)
	}

	// Sort commands alphabetically for consistent output
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	return commands
}

// Execute runs a subcommand with the given arguments
func (r *SubcommandRegistry) Execute(cmdInfo *SubcommandInfo, args []string, flags map[string]interface{}) error {
	// Build the full path to the binary
	binaryPath := filepath.Join(r.binDir, cmdInfo.Binary)

	// Check if the binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		// Try to find it in PATH as fallback
		if path, err := exec.LookPath(cmdInfo.Binary); err == nil {
			binaryPath = path
		} else {
			return fmt.Errorf("subcommand binary not found: %s", cmdInfo.Binary)
		}
	}

	// Build command arguments
	var cmdArgs []string

	// Add pass-through flags
	for _, flagName := range cmdInfo.PassFlags {
		if value, ok := flags[flagName]; ok && value != nil {
			// Handle different flag value types
			switch v := value.(type) {
			case bool:
				if v {
					cmdArgs = append(cmdArgs, flagName)
				}
			case string:
				if v != "" {
					cmdArgs = append(cmdArgs, flagName, v)
				}
			case *string:
				if v != nil && *v != "" {
					cmdArgs = append(cmdArgs, flagName, *v)
				}
			default:
				cmdArgs = append(cmdArgs, flagName, fmt.Sprintf("%v", v))
			}
		}
	}

	// Add subcommand arguments
	cmdArgs = append(cmdArgs, args...)

	// Create and run the command
	cmd := exec.Command(binaryPath, cmdArgs...) // #nosec G204 - binary path is controlled
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("failed to execute %s: %w", cmdInfo.Binary, err)
	}

	return nil
}

// PrintUsage prints usage information including all available subcommands
func (r *SubcommandRegistry) PrintUsage() {
	fmt.Fprintf(os.Stderr, "Commands:\n")

	// Sort commands for consistent output
	maxLen := 0
	for _, cmd := range r.commands {
		if len(cmd.Name) > maxLen {
			maxLen = len(cmd.Name)
		}
	}

	// Print each command with aligned descriptions
	for _, cmd := range r.ListCommands() {
		padding := strings.Repeat(" ", maxLen-len(cmd.Name)+2)
		fmt.Fprintf(os.Stderr, "  %s%s%s\n", cmd.Name, padding, cmd.Description)
	}
}
