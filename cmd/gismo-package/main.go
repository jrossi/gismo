package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
		fmt.Fprintf(os.Stderr, "gismo package - Manage packages\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <command> [arguments]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  install <name>          Install a package\n")
		fmt.Fprintf(os.Stderr, "  remove <name>           Remove a package\n")
		fmt.Fprintf(os.Stderr, "  list                    List installed packages\n")
		fmt.Fprintf(os.Stderr, "  update [name]           Update packages (or specific package)\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nScope flags:\n")
		fmt.Fprintf(os.Stderr, "  --global                Apply changes to global configuration (~/.claude/gismo.json)\n")
		fmt.Fprintf(os.Stderr, "  --project               Apply changes to project configuration (./.claude/gismo.json)\n")
		fmt.Fprintf(os.Stderr, "  (default behavior uses both global and project scopes)\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s install --global claude-prompts\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s list\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s remove --project my-package\n", os.Args[0])
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("gismo-package version %s\n", version)
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

	// Create package manager
	packageManager := NewPackageManager(appConfig, configLoader, *debug)

	// Determine scope
	scope := determineScope(*globalFlag, *projectFlag)

	// Execute command
	ctx := context.Background()
	command := args[0]

	switch command {
	case "install":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: 'install' command requires a package name\n")
			flag.Usage()
			os.Exit(1)
		}
		err = packageManager.InstallPackage(ctx, args[1], scope, *dryRunFlag, *forceFlag)
	case "remove", "rm":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: 'remove' command requires a package name\n")
			flag.Usage()
			os.Exit(1)
		}
		err = packageManager.RemovePackage(ctx, args[1], scope, *dryRunFlag, *forceFlag)
	case "list", "ls":
		err = packageManager.ListPackages(ctx, scope)
	case "update":
		var packageName string
		if len(args) > 1 {
			packageName = args[1]
		}
		err = packageManager.UpdatePackages(ctx, packageName, scope, *dryRunFlag, *forceFlag)
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

// PackageManager handles package operations
type PackageManager struct {
	appConfig    *gismo.AppConfig
	configLoader *gismo.ConfigLoader
	gitOps       *gismo.GitOperations
	installer    *ComponentInstaller
	debug        bool
}

// NewPackageManager creates a new package manager
func NewPackageManager(appConfig *gismo.AppConfig, configLoader *gismo.ConfigLoader, debug bool) *PackageManager {
	return &PackageManager{
		appConfig:    appConfig,
		configLoader: configLoader,
		gitOps:       gismo.NewGitOperations(),
		installer:    NewComponentInstaller(debug),
		debug:        debug,
	}
}

// InstallPackage installs a package
func (pm *PackageManager) InstallPackage(ctx context.Context, packageName string, scope Scope, dryRun, force bool) error {
	if pm.debug {
		fmt.Printf("Installing package: %s (scope: %s, dry-run: %t, force: %t)\n", packageName, scope, dryRun, force)
	}

	// Find the package in available registries
	registryEntry, manifest, err := pm.findPackageInRegistries(ctx, packageName)
	if err != nil {
		return fmt.Errorf("failed to find package '%s': %w", packageName, err)
	}

	// Check if package is already installed
	if existing, ok := pm.appConfig.GetPackage(packageName); ok && !force {
		return fmt.Errorf("package '%s' is already installed (use --force to reinstall)\nInstalled from: %s", packageName, existing.RegistryName)
	}

	if dryRun {
		fmt.Printf("Would install package '%s' from registry '%s'\n", packageName, registryEntry.URL)
		componentCount := 0
		for _, group := range manifest.Components {
			componentCount += len(group)
		}
		fmt.Printf("Would install %d components\n", componentCount)
		return nil
	}

	// Clone registry to temporary location for installation
	fmt.Printf("📦 Installing package '%s'...\n", packageName)
	tempDir, err := pm.createTempPackageDir(packageName)
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone the registry repository
	repoInfo, err := pm.gitOps.CloneRepository(ctx, registryEntry.URL, tempDir)
	if err != nil {
		return fmt.Errorf("failed to clone registry: %w", err)
	}

	// Checkout the specific commit SHA for reproducibility
	if registryEntry.GitSHA != "" && registryEntry.GitSHA != repoInfo.CommitSHA {
		if err := pm.gitOps.CheckoutCommit(ctx, tempDir, registryEntry.GitSHA); err != nil {
			return fmt.Errorf("failed to checkout commit %s: %w", registryEntry.GitSHA, err)
		}
	}

	// Install components
	installResults, err := pm.installer.InstallComponents(ctx, tempDir, manifest, pm.getClaudeDir())
	if err != nil {
		return fmt.Errorf("failed to install components: %w", err)
	}

	// Create package entry
	packageEntry := &gismo.PackageEntry{
		RegistryName: registryEntry.URL, // Store URL as registry identifier
		GitSHA:       repoInfo.CommitSHA,
		Installed:    installResults,
		Manifest:     manifest,
	}

	// Add package to configuration
	pm.appConfig.AddPackage(packageName, packageEntry)

	// Save configuration
	if err := pm.saveConfiguration(scope); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Installed package '%s' with %d components\n", packageName, len(installResults))
	return nil
}

// RemovePackage removes a package
func (pm *PackageManager) RemovePackage(ctx context.Context, packageName string, scope Scope, dryRun, force bool) error {
	if pm.debug {
		fmt.Printf("Removing package: %s (scope: %s, dry-run: %t, force: %t)\n", packageName, scope, dryRun, force)
	}

	// Check if package exists
	packageEntry, ok := pm.appConfig.GetPackage(packageName)
	if !ok {
		return fmt.Errorf("package '%s' is not installed", packageName)
	}

	if dryRun {
		fmt.Printf("Would remove package '%s' with %d components\n", packageName, len(packageEntry.Installed))
		return nil
	}

	// Remove installed components
	fmt.Printf("📦 Removing package '%s'...\n", packageName)
	if err := pm.installer.RemoveComponents(packageEntry.Installed); err != nil {
		return fmt.Errorf("failed to remove components: %w", err)
	}

	// Remove from configuration
	pm.appConfig.RemovePackage(packageName)

	// Save configuration
	if err := pm.saveConfiguration(scope); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Removed package '%s'\n", packageName)
	return nil
}

// ListPackages lists installed packages
func (pm *PackageManager) ListPackages(ctx context.Context, scope Scope) error {
	if pm.debug {
		fmt.Printf("Listing packages (scope: %s)\n", scope)
	}

	packageNames := pm.appConfig.ListPackages()
	if len(packageNames) == 0 {
		fmt.Println("No packages installed")
		return nil
	}

	fmt.Printf("Installed packages (%d):\n", len(packageNames))
	for _, name := range packageNames {
		if entry, ok := pm.appConfig.GetPackage(name); ok {
			fmt.Printf("  📦 %s\n", name)
			fmt.Printf("     Registry: %s\n", entry.RegistryName)
			fmt.Printf("     SHA: %s\n", entry.GitSHA[:8])
			if entry.Manifest != nil {
				fmt.Printf("     Version: %s\n", entry.Manifest.Version)
				if entry.Manifest.Description != "" {
					fmt.Printf("     Description: %s\n", entry.Manifest.Description)
				}
			}
			fmt.Printf("     Components: %d\n", len(entry.Installed))
			fmt.Println()
		}
	}

	return nil
}

// UpdatePackages updates packages
func (pm *PackageManager) UpdatePackages(ctx context.Context, packageName string, scope Scope, dryRun, force bool) error {
	if pm.debug {
		fmt.Printf("Updating packages: %s (scope: %s, dry-run: %t, force: %t)\n", packageName, scope, dryRun, force)
	}

	if packageName != "" {
		// Update specific package
		return pm.updateSinglePackage(ctx, packageName, scope, dryRun, force)
	}

	// Update all packages
	packageNames := pm.appConfig.ListPackages()
	if len(packageNames) == 0 {
		fmt.Println("No packages to update")
		return nil
	}

	if dryRun {
		fmt.Printf("Would update %d packages\n", len(packageNames))
		return nil
	}

	fmt.Printf("🔄 Updating %d packages...\n", len(packageNames))

	updateCount := 0
	for _, name := range packageNames {
		fmt.Printf("  📦 Updating %s...\n", name)
		if err := pm.updateSinglePackage(ctx, name, scope, dryRun, force); err != nil {
			fmt.Printf("     ❌ Failed to update %s: %v\n", name, err)
		} else {
			fmt.Printf("     ✅ Updated %s\n", name)
			updateCount++
		}
	}

	fmt.Printf("✅ Updated %d/%d packages\n", updateCount, len(packageNames))
	return nil
}

// Helper methods will be implemented next...

// createTempPackageDir creates a temporary directory for package operations
func (pm *PackageManager) createTempPackageDir(name string) (string, error) {
	tempDir := filepath.Join(os.TempDir(), "gismo-package-"+name)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	return tempDir, nil
}

// getClaudeDir returns the path to the Claude directory
func (pm *PackageManager) getClaudeDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".claude")
}

// findPackageInRegistries finds a package in available registries
func (pm *PackageManager) findPackageInRegistries(ctx context.Context, packageName string) (*gismo.RegistryEntry, *gismo.ManifestData, error) {
	registryNames := pm.appConfig.ListRegistries()
	if len(registryNames) == 0 {
		return nil, nil, fmt.Errorf("no registries configured. Run 'gismo registry add <url>' first")
	}

	for _, registryName := range registryNames {
		registryEntry, ok := pm.appConfig.GetRegistry(registryName)
		if !ok {
			continue
		}

		// Clone registry to check for package
		tempDir, err := pm.createTempPackageDir(registryName + "-search")
		if err != nil {
			continue
		}
		defer os.RemoveAll(tempDir)

		// Clone the registry
		_, err = pm.gitOps.CloneRepository(ctx, registryEntry.URL, tempDir)
		if err != nil {
			if pm.debug {
				fmt.Printf("Failed to clone registry %s: %v\n", registryName, err)
			}
			continue
		}

		// Parse manifest
		manifestPath := filepath.Join(tempDir, "manifest.json")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			continue
		}

		parser, err := gismo.NewManifestParser()
		if err != nil {
			continue
		}

		manifest, err := parser.ParseManifestFile(manifestPath)
		if err != nil {
			continue
		}

		// Check if this manifest contains the requested package
		if manifest.Name == packageName {
			return registryEntry, manifest, nil
		}
	}

	return nil, nil, fmt.Errorf("package '%s' not found in any configured registry", packageName)
}

// saveConfiguration saves the configuration
func (pm *PackageManager) saveConfiguration(scope Scope) error {
	if pm.debug {
		fmt.Printf("Saving configuration with scope: %s\n", scope)
	}

	switch scope {
	case ScopeGlobal:
		return pm.configLoader.SaveToGlobalConfig(pm.appConfig)
	case ScopeProject:
		return pm.configLoader.SaveToProjectConfig(pm.appConfig)
	case ScopeBoth:
		// Save to both global and project configs
		if err := pm.configLoader.SaveToGlobalConfig(pm.appConfig); err != nil {
			return fmt.Errorf("failed to save global config: %w", err)
		}
		if err := pm.configLoader.SaveToProjectConfig(pm.appConfig); err != nil {
			return fmt.Errorf("failed to save project config: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown scope: %s", scope)
	}
}

// updateSinglePackage updates a single package
func (pm *PackageManager) updateSinglePackage(ctx context.Context, packageName string, scope Scope, dryRun, force bool) error {
	// Get package entry
	_, ok := pm.appConfig.GetPackage(packageName)
	if !ok {
		return fmt.Errorf("package '%s' is not installed", packageName)
	}

	if dryRun {
		fmt.Printf("Would update package '%s'\n", packageName)
		return nil
	}

	// For now, just reinstall the package
	// In a full implementation, this would check for updates first
	if err := pm.RemovePackage(ctx, packageName, scope, false, true); err != nil {
		return fmt.Errorf("failed to remove old package version: %w", err)
	}

	if err := pm.InstallPackage(ctx, packageName, scope, false, force); err != nil {
		return fmt.Errorf("failed to install updated package: %w", err)
	}

	return nil
}

// ComponentInstaller handles the actual file installation
type ComponentInstaller struct {
	debug bool
}

// NewComponentInstaller creates a new component installer
func NewComponentInstaller(debug bool) *ComponentInstaller {
	return &ComponentInstaller{
		debug: debug,
	}
}

// InstallComponents installs components from a manifest
func (ci *ComponentInstaller) InstallComponents(ctx context.Context, repoPath string, manifest *gismo.ManifestData, claudeDir string) (map[string]string, error) {
	installed := make(map[string]string)

	// Parse components and install them
	parser, err := gismo.NewManifestParser()
	if err != nil {
		return nil, fmt.Errorf("failed to create manifest parser: %w", err)
	}

	components := parser.ListAllComponents(manifest)

	for _, componentInfo := range components {
		componentName := fmt.Sprintf("%s.%s", componentInfo.Group, componentInfo.Name)

		// Determine source and destination paths
		srcPath := filepath.Join(repoPath, componentInfo.Component.Source)
		dstPath := parser.GetComponentInstallPath(componentInfo.Component, claudeDir)

		if ci.debug {
			fmt.Printf("Installing component %s: %s -> %s\n", componentName, srcPath, dstPath)
		}

		// Install the component
		if err := ci.installSingleComponent(srcPath, dstPath, componentInfo.Component); err != nil {
			return nil, fmt.Errorf("failed to install component %s: %w", componentName, err)
		}

		installed[componentName] = dstPath
	}

	return installed, nil
}

// RemoveComponents removes installed components
func (ci *ComponentInstaller) RemoveComponents(installed map[string]string) error {
	for componentName, filePath := range installed {
		if ci.debug {
			fmt.Printf("Removing component %s: %s\n", componentName, filePath)
		}

		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove component %s at %s: %w", componentName, filePath, err)
		}
	}

	return nil
}

// installSingleComponent installs a single component file
func (ci *ComponentInstaller) installSingleComponent(srcPath, dstPath string, component *gismo.Component) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy file
	if err := ci.copyFile(srcPath, dstPath); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Set permissions if needed
	if component.Executable {
		if err := os.Chmod(dstPath, 0755); err != nil {
			return fmt.Errorf("failed to set executable permissions: %w", err)
		}
	} else {
		if err := os.Chmod(dstPath, 0644); err != nil {
			return fmt.Errorf("failed to set file permissions: %w", err)
		}
	}

	return nil
}

// copyFile copies a file from source to destination
func (ci *ComponentInstaller) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file atomically using temporary file
	tmpDst := dst + ".tmp"
	dstFile, err := os.Create(tmpDst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		dstFile.Close()
		if err != nil {
			os.Remove(tmpDst) // Cleanup on error
		}
	}()

	// Copy content
	if _, err = srcFile.WriteTo(dstFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Close destination file before rename
	if err = dstFile.Close(); err != nil {
		return fmt.Errorf("failed to close destination file: %w", err)
	}

	// Atomic rename
	if err = os.Rename(tmpDst, dst); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}
