package main

import (
	"context"
	"flag"
	"fmt"
	"os"
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
	gitOps       *gismo.GitOperations
	debug        bool
}

// NewRegistryManager creates a new registry manager
func NewRegistryManager(appConfig *gismo.AppConfig, configLoader *gismo.ConfigLoader, debug bool) *RegistryManager {
	return &RegistryManager{
		appConfig:    appConfig,
		configLoader: configLoader,
		gitOps:       gismo.NewGitOperations(),
		debug:        debug,
	}
}

// AddRegistry adds a new registry
func (rm *RegistryManager) AddRegistry(ctx context.Context, gitURL string, scope Scope, dryRun, force bool) error {
	if rm.debug {
		fmt.Printf("Adding registry: %s (scope: %s, dry-run: %t, force: %t)\n", gitURL, scope, dryRun, force)
	}

	// Verify git is available
	if err := rm.gitOps.VerifyGitAvailable(ctx); err != nil {
		return fmt.Errorf("git is required but not available: %w", err)
	}

	// Normalize URL
	normalizedURL := gismo.NormalizeGitURL(gitURL)

	// Extract repository name from URL
	name := gismo.ExtractRepoName(normalizedURL)
	if name == "" {
		return fmt.Errorf("failed to extract repository name from URL: %s", gitURL)
	}

	// Check if registry already exists
	if existing, ok := rm.appConfig.GetRegistry(name); ok && !force {
		return fmt.Errorf("registry '%s' already exists (use --force to overwrite)\nExisting: %s", name, existing.URL)
	}

	if dryRun {
		fmt.Printf("Would add registry '%s' with URL: %s\n", name, normalizedURL)
		return nil
	}

	// Clone repository to temporary location to validate manifest
	fmt.Printf("📥 Fetching registry from %s...\n", normalizedURL)
	tempDir, err := rm.createTempRegistryDir(name)
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup temp directory

	// Clone the repository
	repoInfo, err := rm.gitOps.CloneRepository(ctx, normalizedURL, tempDir)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Validate manifest exists and is valid
	manifest, err := rm.validateRepositoryManifest(tempDir)
	if err != nil {
		return fmt.Errorf("repository validation failed: %w", err)
	}

	// Display manifest information
	fmt.Printf("✅ Found valid manifest.json:\n")
	fmt.Printf("   Name: %s\n", manifest.Name)
	fmt.Printf("   Version: %s\n", manifest.Version)
	if manifest.Description != "" {
		fmt.Printf("   Description: %s\n", manifest.Description)
	}
	if manifest.Author != "" {
		fmt.Printf("   Author: %s\n", manifest.Author)
	}

	// Count components
	componentCount := 0
	for _, group := range manifest.Components {
		componentCount += len(group)
	}
	fmt.Printf("   Components: %d\n", componentCount)

	// Create registry entry
	entry := &gismo.RegistryEntry{
		URL:         normalizedURL,
		GitSHA:      repoInfo.CommitSHA,
		Version:     manifest.Version,
		Scope:       string(scope),
		InstallDate: time.Now(),
		UpdatedDate: time.Now(),
	}

	// Add to configuration
	rm.appConfig.AddRegistry(name, entry)

	// Save configuration based on scope
	if err := rm.saveConfiguration(scope); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Added registry '%s' (SHA: %s)\n", name, repoInfo.CommitSHA[:8])
	fmt.Printf("💡 Run 'gismo package install %s' to install components\n", manifest.Name)
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

	// Verify git is available
	if err := rm.gitOps.VerifyGitAvailable(ctx); err != nil {
		return fmt.Errorf("git is required but not available: %w", err)
	}

	if registryName != "" {
		// Update specific registry
		return rm.updateSingleRegistry(ctx, registryName, scope, dryRun, force)
	}

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

	updateCount := 0
	for _, name := range registryNames {
		fmt.Printf("  📦 Updating %s...\n", name)
		if err := rm.updateSingleRegistry(ctx, name, scope, dryRun, force); err != nil {
			fmt.Printf("     ❌ Failed to update %s: %v\n", name, err)
		} else {
			fmt.Printf("     ✅ Updated %s\n", name)
			updateCount++
		}
	}

	fmt.Printf("✅ Updated %d/%d registries\n", updateCount, len(registryNames))
	return nil
}

// saveConfiguration saves the configuration to the appropriate files
func (rm *RegistryManager) saveConfiguration(scope Scope) error {
	if rm.debug {
		fmt.Printf("Saving configuration with scope: %s\n", scope)
	}

	switch scope {
	case ScopeGlobal:
		return rm.configLoader.SaveToGlobalConfig(rm.appConfig)
	case ScopeProject:
		return rm.configLoader.SaveToProjectConfig(rm.appConfig)
	case ScopeBoth:
		// Save to both global and project configs
		if err := rm.configLoader.SaveToGlobalConfig(rm.appConfig); err != nil {
			return fmt.Errorf("failed to save global config: %w", err)
		}
		if err := rm.configLoader.SaveToProjectConfig(rm.appConfig); err != nil {
			return fmt.Errorf("failed to save project config: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown scope: %s", scope)
	}
}

// createTempRegistryDir creates a temporary directory for registry operations
func (rm *RegistryManager) createTempRegistryDir(name string) (string, error) {
	tempDir := filepath.Join(os.TempDir(), "gismo-registry-"+name)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	return tempDir, nil
}

// validateRepositoryManifest validates that a repository contains a valid manifest
func (rm *RegistryManager) validateRepositoryManifest(repoPath string) (*gismo.ManifestData, error) {
	// Check if manifest.json exists
	manifestPath := filepath.Join(repoPath, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("manifest.json not found in repository root")
	}

	// Create manifest parser
	parser, err := gismo.NewManifestParser()
	if err != nil {
		return nil, fmt.Errorf("failed to create manifest parser: %w", err)
	}

	// Parse and validate manifest
	manifest, err := parser.ParseManifestFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Perform integrity validation
	if err := parser.ValidateManifestIntegrity(manifest, "dev"); err != nil {
		return nil, fmt.Errorf("manifest integrity validation failed: %w", err)
	}

	return manifest, nil
}

// updateSingleRegistry updates a single registry
func (rm *RegistryManager) updateSingleRegistry(ctx context.Context, registryName string, scope Scope, dryRun, force bool) error {
	// Get registry entry
	entry, ok := rm.appConfig.GetRegistry(registryName)
	if !ok {
		return fmt.Errorf("registry '%s' not found", registryName)
	}

	if dryRun {
		fmt.Printf("Would update registry '%s' from %s\n", registryName, entry.URL)
		return nil
	}

	// Clone to temporary directory to check for updates
	tempDir, err := rm.createTempRegistryDir(registryName + "-update")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone repository
	repoInfo, err := rm.gitOps.CloneRepository(ctx, entry.URL, tempDir)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Check if there are updates
	if repoInfo.CommitSHA == entry.GitSHA {
		if rm.debug {
			fmt.Printf("Registry '%s' is already up to date (SHA: %s)\n", registryName, entry.GitSHA[:8])
		}
		return nil
	}

	// Validate manifest in updated repository
	manifest, err := rm.validateRepositoryManifest(tempDir)
	if err != nil {
		return fmt.Errorf("updated repository validation failed: %w", err)
	}

	// Update registry entry
	entry.GitSHA = repoInfo.CommitSHA
	entry.Version = manifest.Version
	entry.UpdatedDate = time.Now()

	// Save updated configuration
	if err := rm.saveConfiguration(scope); err != nil {
		return fmt.Errorf("failed to save updated configuration: %w", err)
	}

	if rm.debug {
		fmt.Printf("Updated registry '%s' from SHA %s to %s\n", registryName, entry.GitSHA[:8], repoInfo.CommitSHA[:8])
	}

	return nil
}
