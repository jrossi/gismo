package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		fmt.Fprintf(os.Stderr, "  search [pattern]        Search for packages in registries\n")
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
	case "search":
		var pattern string
		if len(args) > 1 {
			pattern = args[1]
		}
		err = packageManager.SearchPackages(ctx, pattern)
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
	depResolver  *gismo.DependencyResolver
	debug        bool
}

// NewPackageManager creates a new package manager
func NewPackageManager(appConfig *gismo.AppConfig, configLoader *gismo.ConfigLoader, debug bool) *PackageManager {
	return &PackageManager{
		appConfig:    appConfig,
		configLoader: configLoader,
		gitOps:       gismo.NewGitOperations(),
		installer:    NewComponentInstaller(debug),
		depResolver:  gismo.NewDependencyResolver(appConfig, debug),
		debug:        debug,
	}
}

// InstallPackage installs a package and its dependencies
func (pm *PackageManager) InstallPackage(ctx context.Context, packageName string, scope Scope, dryRun, force bool) error {
	if pm.debug {
		fmt.Printf("Installing package: %s (scope: %s, dry-run: %t, force: %t)\n", packageName, scope, dryRun, force)
	}

	// Resolve dependencies
	fmt.Printf("🔍 Resolving dependencies for '%s'...\n", packageName)
	installPlan, err := pm.depResolver.ResolveDependencies(ctx, packageName)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Validate the install plan
	if err := pm.depResolver.ValidateInstallPlan(installPlan); err != nil {
		return fmt.Errorf("install plan validation failed: %w", err)
	}

	// Show dependency plan
	if len(installPlan.Dependencies) > 1 {
		fmt.Printf("📋 Install plan (%d packages):\n", len(installPlan.Dependencies))
		for _, dep := range pm.depResolver.GetInstallOrder(installPlan) {
			prefix := strings.Repeat("  ", dep.InstallLevel)
			if dep.InstallLevel == 0 {
				fmt.Printf("%s📦 %s (main package)\n", prefix, dep.Spec.Name)
			} else {
				fmt.Printf("%s└─ %s (dependency)\n", prefix, dep.Spec.Name)
			}
		}
	} else {
		fmt.Printf("📋 No dependencies required\n")
	}

	if dryRun {
		fmt.Printf("Would install %d package(s)\n", len(installPlan.Dependencies))
		return nil
	}

	// Check for existing installations if not forcing
	if !force {
		for _, dep := range installPlan.Dependencies {
			if existing, ok := pm.appConfig.GetPackage(dep.Spec.Name); ok {
				return fmt.Errorf("package '%s' is already installed (use --force to reinstall)\nInstalled from: %s",
					dep.Spec.Name, existing.RegistryName)
			}
		}
	}

	// Install packages in dependency order
	installedCount := 0
	for _, dep := range pm.depResolver.GetInstallOrder(installPlan) {
		if err := pm.installSinglePackage(ctx, dep, scope); err != nil {
			return fmt.Errorf("failed to install package '%s': %w", dep.Spec.Name, err)
		}
		installedCount++
	}

	// Save configuration
	if err := pm.saveConfiguration(scope); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	if installedCount == 1 {
		fmt.Printf("✅ Installed package '%s'\n", packageName)
	} else {
		fmt.Printf("✅ Installed %d packages ('%s' and %d dependencies)\n",
			installedCount, packageName, installedCount-1)
	}
	return nil
}

// installSinglePackage installs a single package from a dependency entry
func (pm *PackageManager) installSinglePackage(ctx context.Context, dep *gismo.DependencyEntry, scope Scope) error {
	packageName := dep.Spec.Name

	if pm.debug {
		fmt.Printf("Installing single package: %s\n", packageName)
	}

	// Clone registry to temporary location for installation
	prefix := strings.Repeat("  ", dep.InstallLevel)
	fmt.Printf("%s📦 Installing '%s'...\n", prefix, packageName)

	tempDir, err := pm.createTempPackageDir(packageName)
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone the registry repository
	repoInfo, err := pm.gitOps.CloneRepository(ctx, dep.RegistryURL, tempDir)
	if err != nil {
		return fmt.Errorf("failed to clone registry: %w", err)
	}

	// Checkout the specific commit SHA for reproducibility
	if dep.CommitSHA != "" && dep.CommitSHA != repoInfo.CommitSHA {
		if err := pm.gitOps.CheckoutCommit(ctx, tempDir, dep.CommitSHA); err != nil {
			return fmt.Errorf("failed to checkout commit %s: %w", dep.CommitSHA, err)
		}
	}

	// Install components
	installResults, err := pm.installer.InstallComponents(ctx, tempDir, dep.Manifest, pm.getClaudeDir())
	if err != nil {
		return fmt.Errorf("failed to install components: %w", err)
	}

	// Create package entry
	packageEntry := &gismo.PackageEntry{
		RegistryName: dep.RegistryURL,
		GitSHA:       dep.CommitSHA,
		Installed:    installResults,
		Manifest:     dep.Manifest,
	}

	// Add package to configuration
	pm.appConfig.AddPackage(packageName, packageEntry)

	if pm.debug {
		fmt.Printf("%s✅ Installed '%s' with %d components\n", prefix, packageName, len(installResults))
	}

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

// SearchPackages searches for packages in configured registries
func (pm *PackageManager) SearchPackages(ctx context.Context, pattern string) error {
	if pm.debug {
		fmt.Printf("Searching for packages with pattern: '%s'\n", pattern)
	}

	registryNames := pm.appConfig.ListRegistries()
	if len(registryNames) == 0 {
		fmt.Println("No registries configured. Run 'gismo registry add <url>' first")
		return nil
	}

	fmt.Printf("🔍 Searching in %d registr%s...\n", len(registryNames),
		map[bool]string{true: "y", false: "ies"}[len(registryNames) == 1])

	foundPackages := []*SearchResult{}

	for _, registryName := range registryNames {
		registryEntry, ok := pm.appConfig.GetRegistry(registryName)
		if !ok {
			continue
		}

		if pm.debug {
			fmt.Printf("  Searching registry '%s'...\n", registryName)
		}

		results, err := pm.searchInRegistry(ctx, registryEntry, pattern)
		if err != nil {
			if pm.debug {
				fmt.Printf("    Error searching registry '%s': %v\n", registryName, err)
			}
			continue
		}

		foundPackages = append(foundPackages, results...)
	}

	if len(foundPackages) == 0 {
		if pattern == "" {
			fmt.Println("No packages found in any registry")
		} else {
			fmt.Printf("No packages found matching '%s'\n", pattern)
		}
		return nil
	}

	// Display results
	fmt.Printf("\n📦 Found %d package%s:\n", len(foundPackages),
		map[bool]string{true: "", false: "s"}[len(foundPackages) == 1])

	for _, result := range foundPackages {
		fmt.Printf("\n  📦 %s@%s\n", result.Name, result.Version)
		if result.Description != "" {
			fmt.Printf("     %s\n", result.Description)
		}
		if result.Author != "" {
			fmt.Printf("     Author: %s\n", result.Author)
		}
		fmt.Printf("     Registry: %s\n", result.RegistryName)

		// Show component count
		componentCount := 0
		for _, group := range result.Manifest.Components {
			componentCount += len(group)
		}
		fmt.Printf("     Components: %d\n", componentCount)

		// Show if already installed
		if existing, ok := pm.appConfig.GetPackage(result.Name); ok {
			if existing.GitSHA == result.CommitSHA {
				fmt.Printf("     Status: ✅ Installed (current)\n")
			} else {
				fmt.Printf("     Status: 📦 Installed (different version)\n")
			}
		} else {
			fmt.Printf("     Status: ⬇️  Available for install\n")
		}
	}

	fmt.Printf("\nTo install a package: gismo package install <name>\n")
	return nil
}

// SearchResult represents a package found in search
type SearchResult struct {
	Name         string
	Version      string
	Description  string
	Author       string
	RegistryName string
	RegistryURL  string
	CommitSHA    string
	Manifest     *gismo.ManifestData
}

// searchInRegistry searches for packages in a specific registry
func (pm *PackageManager) searchInRegistry(ctx context.Context, registryEntry *gismo.RegistryEntry, pattern string) ([]*SearchResult, error) {
	// Create temporary directory for search
	tempDir, err := pm.createTempPackageDir("search-" + extractRegistryName(registryEntry.URL))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone the registry
	repoInfo, err := pm.gitOps.CloneRepository(ctx, registryEntry.URL, tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to clone registry: %w", err)
	}

	// Parse manifest
	manifestPath := filepath.Join(tempDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no manifest.json found in registry")
	}

	parser, err := gismo.NewManifestParser()
	if err != nil {
		return nil, fmt.Errorf("failed to create manifest parser: %w", err)
	}

	manifest, err := parser.ParseManifestFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Check if package matches search pattern
	if pattern == "" || pm.matchesPattern(manifest, pattern) {
		return []*SearchResult{{
			Name:         manifest.Name,
			Version:      manifest.Version,
			Description:  manifest.Description,
			Author:       manifest.Author,
			RegistryName: extractRegistryName(registryEntry.URL),
			RegistryURL:  registryEntry.URL,
			CommitSHA:    repoInfo.CommitSHA,
			Manifest:     manifest,
		}}, nil
	}

	return []*SearchResult{}, nil
}

// matchesPattern checks if a manifest matches the search pattern
func (pm *PackageManager) matchesPattern(manifest *gismo.ManifestData, pattern string) bool {
	pattern = strings.ToLower(pattern)

	// Check name
	if strings.Contains(strings.ToLower(manifest.Name), pattern) {
		return true
	}

	// Check description
	if strings.Contains(strings.ToLower(manifest.Description), pattern) {
		return true
	}

	// Check author
	if strings.Contains(strings.ToLower(manifest.Author), pattern) {
		return true
	}

	return false
}

// extractRegistryName extracts a display name from a registry URL
func extractRegistryName(url string) string {
	// Remove protocol and extract meaningful name
	name := strings.TrimPrefix(url, "https://")
	name = strings.TrimPrefix(name, "http://")
	name = strings.TrimPrefix(name, "git@")

	// Handle SSH format
	name = strings.ReplaceAll(name, ":", "/")

	// Extract last two parts (user/repo)
	parts := strings.Split(name, "/")
	if len(parts) >= 2 {
		return fmt.Sprintf("%s/%s", parts[len(parts)-2], parts[len(parts)-1])
	}

	return name
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
