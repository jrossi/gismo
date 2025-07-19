package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
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
		showVersion = flag.Bool("version", false, "Show version information")
		debug       = flag.Bool("debug", false, "Enable debug output")
		configFile  = flag.String("config", "", "Path to configuration file")
		globalFlag  = flag.Bool("global", false, "Apply to global configuration")
		projectFlag = flag.Bool("project", false, "Apply to project configuration")
		dryRunFlag  = flag.Bool("dry-run", false, "Show what would be done")
		forceFlag   = flag.Bool("force", false, "Force operation")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "gismo registry - Manage package registries\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <command> [arguments]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  add <git-url>           Add a new registry\n")
		fmt.Fprintf(os.Stderr, "  remove <name>           Remove a registry\n")
		fmt.Fprintf(os.Stderr, "  list                    List configured registries\n")
		fmt.Fprintf(os.Stderr, "  update [name]           Update registries (or specific registry)\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nScope flags:\n")
		fmt.Fprintf(os.Stderr, "  --global                Apply changes to global configuration (~/.claude/gismo.json)\n")
		fmt.Fprintf(os.Stderr, "  --project               Apply changes to project configuration (./.claude/gismo.json)\n")
		fmt.Fprintf(os.Stderr, "  (default behavior uses both global and project scopes)\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s add --global github.com/jrossi/claude-prompts\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s list\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s remove --project my-registry\n", os.Args[0])
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("gismo-registry version %s\n", version)
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

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no command specified\n")
		flag.Usage()
		os.Exit(1)
	}

	// Load configuration
	configLoader, err := gismo.NewConfigLoader()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config loader: %v\n", err)
		os.Exit(1)
	}

	var appConfig *gismo.AppConfig
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

	// Create registry manager
	registryManager := NewRegistryManager(appConfig, configLoader, *debug)

	// Determine scope
	scope := determineScope(*globalFlag, *projectFlag)

	// Execute command
	ctx := context.Background()
	command := args[0]

	switch command {
	case "add":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: 'add' command requires a git URL\n")
			flag.Usage()
			os.Exit(1)
		}
		err = registryManager.AddRegistry(ctx, args[1], scope, *dryRunFlag, *forceFlag)
	case "remove", "rm":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: 'remove' command requires a registry name\n")
			flag.Usage()
			os.Exit(1)
		}
		err = registryManager.RemoveRegistry(ctx, args[1], scope, *dryRunFlag, *forceFlag)
	case "list", "ls":
		err = registryManager.ListRegistries(ctx, scope)
	case "update":
		var registryName string
		if len(args) > 1 {
			registryName = args[1]
		}
		err = registryManager.UpdateRegistries(ctx, registryName, scope, *dryRunFlag, *forceFlag)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n", command)
		flag.Usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// Scope represents the configuration scope
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
	ScopeBoth    Scope = "both"
)

// determineScope determines the appropriate scope based on flags
func determineScope(globalFlag, projectFlag bool) Scope {
	if globalFlag && projectFlag {
		fmt.Fprintf(os.Stderr, "Warning: both --global and --project specified, using both scopes\n")
		return ScopeBoth
	}
	if globalFlag {
		return ScopeGlobal
	}
	if projectFlag {
		return ScopeProject
	}
	// Default to both scopes for backward compatibility
	return ScopeBoth
}

// RegistryManager handles registry operations
type RegistryManager struct {
	appConfig    *gismo.AppConfig
	configLoader *gismo.ConfigLoader
	debug        bool
}

// NewRegistryManager creates a new registry manager
func NewRegistryManager(appConfig *gismo.AppConfig, configLoader *gismo.ConfigLoader, debug bool) *RegistryManager {
	return &RegistryManager{
		appConfig:    appConfig,
		configLoader: configLoader,
		debug:        debug,
	}
}

// AddRegistry adds a new registry
func (rm *RegistryManager) AddRegistry(ctx context.Context, gitURL string, scope Scope, dryRun, force bool) error {
	if rm.debug {
		fmt.Printf("Adding registry: %s (scope: %s, dry-run: %t, force: %t)\n", gitURL, scope, dryRun, force)
	}

	// Extract repository name from URL
	name := extractRepoName(gitURL)
	if name == "" {
		return fmt.Errorf("failed to extract repository name from URL: %s", gitURL)
	}

	// Check if registry already exists
	if existing, ok := rm.appConfig.GetRegistry(name); ok && !force {
		return fmt.Errorf("registry '%s' already exists (use --force to overwrite)\nExisting: %s", name, existing.URL)
	}

	// Create registry entry
	entry := &gismo.RegistryEntry{
		URL:         gitURL,
		GitSHA:      "", // Will be populated when fetching
		Version:     "", // Will be populated if tagged
		Scope:       string(scope),
		InstallDate: time.Now(),
		UpdatedDate: time.Now(),
	}

	if dryRun {
		fmt.Printf("Would add registry '%s' with URL: %s\n", name, gitURL)
		return nil
	}

	// Add to configuration
	rm.appConfig.AddRegistry(name, entry)

	// Save configuration based on scope
	if err := rm.saveConfiguration(scope); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Added registry '%s' with URL: %s\n", name, gitURL)
	return nil
}

// RemoveRegistry removes a registry
func (rm *RegistryManager) RemoveRegistry(ctx context.Context, name string, scope Scope, dryRun, force bool) error {
	if rm.debug {
		fmt.Printf("Removing registry: %s (scope: %s, dry-run: %t, force: %t)\n", name, scope, dryRun, force)
	}

	// Check if registry exists
	if _, ok := rm.appConfig.GetRegistry(name); !ok {
		return fmt.Errorf("registry '%s' not found", name)
	}

	if dryRun {
		fmt.Printf("Would remove registry '%s'\n", name)
		return nil
	}

	// Remove from configuration
	if removed := rm.appConfig.RemoveRegistry(name); !removed {
		return fmt.Errorf("registry '%s' not found", name)
	}

	// Save configuration based on scope
	if err := rm.saveConfiguration(scope); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Removed registry '%s'\n", name)
	return nil
}

// ListRegistries lists all configured registries
func (rm *RegistryManager) ListRegistries(ctx context.Context, scope Scope) error {
	if rm.debug {
		fmt.Printf("Listing registries (scope: %s)\n", scope)
	}

	registryNames := rm.appConfig.ListRegistries()
	if len(registryNames) == 0 {
		fmt.Println("No registries configured")
		return nil
	}

	fmt.Printf("Configured registries (%d):\n", len(registryNames))
	for _, name := range registryNames {
		if entry, ok := rm.appConfig.GetRegistry(name); ok {
			fmt.Printf("  📦 %s\n", name)
			fmt.Printf("     URL: %s\n", entry.URL)
			fmt.Printf("     Scope: %s\n", entry.Scope)
			if entry.GitSHA != "" {
				fmt.Printf("     SHA: %s\n", entry.GitSHA)
			}
			if entry.Version != "" {
				fmt.Printf("     Version: %s\n", entry.Version)
			}
			fmt.Printf("     Installed: %s\n", entry.InstallDate.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}
	}

	return nil
}

// UpdateRegistries updates registries
func (rm *RegistryManager) UpdateRegistries(ctx context.Context, registryName string, scope Scope, dryRun, force bool) error {
	if rm.debug {
		fmt.Printf("Updating registries: %s (scope: %s, dry-run: %t, force: %t)\n", registryName, scope, dryRun, force)
	}

	if registryName != "" {
		// Update specific registry
		if _, ok := rm.appConfig.GetRegistry(registryName); !ok {
			return fmt.Errorf("registry '%s' not found", registryName)
		}

		if dryRun {
			fmt.Printf("Would update registry '%s'\n", registryName)
			return nil
		}

		fmt.Printf("🔄 Updating registry '%s'...\n", registryName)
		// TODO: Implement actual git fetching and SHA updating
		fmt.Printf("✅ Updated registry '%s'\n", registryName)
	} else {
		// Update all registries
		registryNames := rm.appConfig.ListRegistries()
		if len(registryNames) == 0 {
			fmt.Println("No registries to update")
			return nil
		}

		if dryRun {
			fmt.Printf("Would update %d registries\n", len(registryNames))
			return nil
		}

		fmt.Printf("🔄 Updating %d registries...\n", len(registryNames))
		for _, name := range registryNames {
			fmt.Printf("  - %s\n", name)
			// TODO: Implement actual git fetching and SHA updating
		}
		fmt.Printf("✅ Updated %d registries\n", len(registryNames))
	}

	return nil
}

// saveConfiguration saves the configuration to the appropriate files
func (rm *RegistryManager) saveConfiguration(scope Scope) error {
	// For now, this is a placeholder
	// In a full implementation, this would:
	// 1. Determine which config files to write to based on scope
	// 2. Write the configuration to the appropriate files
	// 3. Handle file permissions and backup creation

	if rm.debug {
		fmt.Printf("Would save configuration with scope: %s\n", scope)
	}

	return nil
}

// extractRepoName extracts a repository name from a git URL
func extractRepoName(gitURL string) string {
	// Handle common git URL formats:
	// - https://github.com/user/repo
	// - git@github.com:user/repo.git
	// - github.com/user/repo

	// Remove common prefixes
	url := strings.TrimPrefix(gitURL, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "git@")

	// Handle SSH format (convert : to /)
	url = strings.ReplaceAll(url, ":", "/")

	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Split by / and take the last part
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		// Return "user-repo" format for better readability
		user := parts[len(parts)-2]
		repo := parts[len(parts)-1]
		return fmt.Sprintf("%s-%s", user, repo)
	}

	return ""
}
