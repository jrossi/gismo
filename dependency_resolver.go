package gismo

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// DependencyResolver handles package dependency resolution and validation
type DependencyResolver struct {
	gitOps    *GitOperations
	appConfig *AppConfig
	validator *PackageValidator
	debug     bool
}

// NewDependencyResolver creates a new dependency resolver
func NewDependencyResolver(appConfig *AppConfig, debug bool) *DependencyResolver {
	return &DependencyResolver{
		gitOps:    NewGitOperations(),
		appConfig: appConfig,
		validator: NewPackageValidator(debug),
		debug:     debug,
	}
}

// DependencySpec represents a parsed dependency specification
type DependencySpec struct {
	Repository string // e.g., "github.com/user/repo"
	Version    string // e.g., "v1.2.3"
	Name       string // extracted package name
}

// InstallPlan represents the complete installation plan for a package and its dependencies
type InstallPlan struct {
	RootPackage  string             // The originally requested package
	Dependencies []*DependencyEntry // Dependencies in installation order
	Conflicts    []string           // Any conflicts detected
}

// DependencyEntry represents a dependency in the install plan
type DependencyEntry struct {
	Spec         *DependencySpec // Dependency specification
	Manifest     *ManifestData   // The resolved manifest
	RegistryURL  string          // Registry URL where found
	CommitSHA    string          // Git commit SHA
	InstallLevel int             // Depth in dependency tree (0 = root)
}

// ParseDependency parses a dependency string like "github.com/user/repo@v1.2.3"
func ParseDependency(dep string) (*DependencySpec, error) {
	// Match pattern: repository@version
	re := regexp.MustCompile(`^([a-zA-Z0-9./:-]+)@(v[0-9]+\.[0-9]+\.[0-9]+)$`)
	matches := re.FindStringSubmatch(dep)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid dependency format '%s', expected 'repository@version'", dep)
	}

	repository := matches[1]
	version := matches[2]

	// Extract package name from repository path
	parts := strings.Split(repository, "/")
	name := parts[len(parts)-1]

	return &DependencySpec{
		Repository: repository,
		Version:    version,
		Name:       name,
	}, nil
}

// ResolveDependencies creates a complete installation plan for a package and its dependencies
func (dr *DependencyResolver) ResolveDependencies(ctx context.Context, packageName string) (*InstallPlan, error) {
	if dr.debug {
		fmt.Printf("Resolving dependencies for package: %s\n", packageName)
	}

	plan := &InstallPlan{
		RootPackage:  packageName,
		Dependencies: []*DependencyEntry{},
		Conflicts:    []string{},
	}

	// Keep track of visited packages to detect cycles
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	// Start with the root package
	if err := dr.resolveDependencyTree(ctx, packageName, "", 0, plan, visited, visiting); err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Sort dependencies by install level (dependencies first, then dependents)
	sort.Slice(plan.Dependencies, func(i, j int) bool {
		return plan.Dependencies[i].InstallLevel > plan.Dependencies[j].InstallLevel
	})

	if dr.debug {
		fmt.Printf("Resolved %d dependencies for %s\n", len(plan.Dependencies), packageName)
	}

	return plan, nil
}

// resolveDependencyTree recursively resolves dependencies
func (dr *DependencyResolver) resolveDependencyTree(
	ctx context.Context,
	packageName string,
	requestedBy string,
	level int,
	plan *InstallPlan,
	visited map[string]bool,
	visiting map[string]bool,
) error {
	// Check for circular dependencies
	if visiting[packageName] {
		cycle := dr.findCycle(packageName, visiting)
		return fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
	}

	// Skip if already processed
	if visited[packageName] {
		return nil
	}

	visiting[packageName] = true
	defer func() {
		visiting[packageName] = false
		visited[packageName] = true
	}()

	if dr.debug && level > 0 {
		fmt.Printf("  %sResolving dependency: %s (requested by %s)\n",
			strings.Repeat("  ", level), packageName, requestedBy)
	}

	// Find the package in registries
	manifest, registryURL, commitSHA, err := dr.findPackageInRegistries(ctx, packageName)
	if err != nil {
		return fmt.Errorf("dependency '%s' not found: %w", packageName, err)
	}

	// Create dependency entry
	entry := &DependencyEntry{
		Spec: &DependencySpec{
			Repository: registryURL,
			Version:    manifest.Version,
			Name:       packageName,
		},
		Manifest:     manifest,
		RegistryURL:  registryURL,
		CommitSHA:    commitSHA,
		InstallLevel: level,
	}

	// Add to plan
	plan.Dependencies = append(plan.Dependencies, entry)

	// Process this package's dependencies
	for _, dep := range manifest.Dependencies {
		spec, err := ParseDependency(dep)
		if err != nil {
			return fmt.Errorf("invalid dependency '%s' in package '%s': %w", dep, packageName, err)
		}

		// Check for version conflicts
		if existing := dr.findExistingDependency(plan, spec.Name); existing != nil {
			if existing.Spec.Version != spec.Version {
				conflict := fmt.Sprintf("%s requires %s@%s but %s@%s is already required",
					packageName, spec.Name, spec.Version,
					existing.Spec.Name, existing.Spec.Version)
				plan.Conflicts = append(plan.Conflicts, conflict)
				if dr.debug {
					fmt.Printf("  Version conflict detected: %s\n", conflict)
				}
			}
			continue // Skip if already processed
		}

		// Recursively resolve this dependency
		if err := dr.resolveDependencyTree(ctx, spec.Name, packageName, level+1, plan, visited, visiting); err != nil {
			return fmt.Errorf("failed to resolve dependency '%s' for package '%s': %w", spec.Name, packageName, err)
		}
	}

	return nil
}

// findPackageInRegistries searches for a package across all configured registries
func (dr *DependencyResolver) findPackageInRegistries(ctx context.Context, packageName string) (*ManifestData, string, string, error) {
	registryNames := dr.appConfig.ListRegistries()
	if len(registryNames) == 0 {
		return nil, "", "", fmt.Errorf("no registries configured")
	}

	for _, registryName := range registryNames {
		registryEntry, ok := dr.appConfig.GetRegistry(registryName)
		if !ok {
			continue
		}

		manifest, commitSHA, err := dr.searchRegistryForPackage(ctx, registryEntry.URL, packageName)
		if err != nil {
			if dr.debug {
				fmt.Printf("    Package '%s' not found in registry '%s': %v\n", packageName, registryName, err)
			}
			continue
		}

		if dr.debug {
			fmt.Printf("    Found package '%s' in registry '%s'\n", packageName, registryName)
		}

		return manifest, registryEntry.URL, commitSHA, nil
	}

	return nil, "", "", fmt.Errorf("package '%s' not found in any configured registry", packageName)
}

// searchRegistryForPackage searches a specific registry for a package
func (dr *DependencyResolver) searchRegistryForPackage(ctx context.Context, registryURL, packageName string) (*ManifestData, string, error) {
	// Create temporary directory for registry search
	tempDir := fmt.Sprintf("/tmp/gismo-dep-search-%s", strings.ReplaceAll(packageName, "/", "-"))
	defer func() {
		// Clean up temp directory
		if err := os.RemoveAll(tempDir); err != nil && dr.debug {
			fmt.Printf("Warning: failed to clean up temp directory %s: %v\n", tempDir, err)
		}
	}()

	// Clone the registry
	repoInfo, err := dr.gitOps.CloneRepository(ctx, registryURL, tempDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to clone registry: %w", err)
	}

	// Parse manifest
	manifestPath := tempDir + "/manifest.json"
	parser, err := NewManifestParser()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create manifest parser: %w", err)
	}

	manifest, err := parser.ParseManifestFile(manifestPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Check if this manifest contains the requested package
	if manifest.Name == packageName {
		// Validate the package before returning it
		if dr.debug {
			fmt.Printf("    Validating package '%s'...\n", packageName)
		}

		validationResult, err := dr.validator.ValidatePackage(ctx, tempDir, manifest)
		if err != nil {
			return nil, "", fmt.Errorf("package validation failed: %w", err)
		}

		if !validationResult.Valid {
			return nil, "", fmt.Errorf("package validation failed: %s", strings.Join(validationResult.Errors, "; "))
		}

		if len(validationResult.Warnings) > 0 && dr.debug {
			fmt.Printf("    Package warnings: %s\n", strings.Join(validationResult.Warnings, "; "))
		}

		return manifest, repoInfo.CommitSHA, nil
	}

	return nil, "", fmt.Errorf("package '%s' not found in this registry", packageName)
}

// findExistingDependency finds an existing dependency in the plan by name
func (dr *DependencyResolver) findExistingDependency(plan *InstallPlan, name string) *DependencyEntry {
	for _, dep := range plan.Dependencies {
		if dep.Spec.Name == name {
			return dep
		}
	}
	return nil
}

// findCycle detects and returns the cycle path
func (dr *DependencyResolver) findCycle(startPackage string, visiting map[string]bool) []string {
	var cycle []string
	found := false

	for pkg := range visiting {
		if pkg == startPackage {
			found = true
		}
		if found {
			cycle = append(cycle, pkg)
		}
	}

	cycle = append(cycle, startPackage) // Close the cycle
	return cycle
}

// ValidateInstallPlan validates that an install plan can be executed safely
func (dr *DependencyResolver) ValidateInstallPlan(plan *InstallPlan) error {
	if len(plan.Conflicts) > 0 {
		return fmt.Errorf("dependency conflicts detected:\n  - %s", strings.Join(plan.Conflicts, "\n  - "))
	}

	// Check for any already installed packages that might conflict
	for _, dep := range plan.Dependencies {
		if existing, ok := dr.appConfig.GetPackage(dep.Spec.Name); ok {
			if existing.GitSHA != dep.CommitSHA {
				return fmt.Errorf("package '%s' is already installed with different version (SHA: %s != %s)",
					dep.Spec.Name, existing.GitSHA[:8], dep.CommitSHA[:8])
			}
		}
	}

	return nil
}

// GetInstallOrder returns the dependencies in the correct installation order
func (dr *DependencyResolver) GetInstallOrder(plan *InstallPlan) []*DependencyEntry {
	// Dependencies are already sorted by install level in ResolveDependencies
	// Higher levels (deeper dependencies) come first
	return plan.Dependencies
}
