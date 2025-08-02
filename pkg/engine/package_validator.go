package engine

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jrossi/gismo/pkg/version"
)

// PackageValidator handles package validation and integrity checks
type PackageValidator struct {
	debug bool
}

// NewPackageValidator creates a new package validator
func NewPackageValidator(debug bool) *PackageValidator {
	return &PackageValidator{
		debug: debug,
	}
}

// ValidationResult represents the result of package validation
type ValidationResult struct {
	Valid     bool              // Overall validation result
	Errors    []string          // Validation errors
	Warnings  []string          // Validation warnings
	Checksums map[string]string // File checksums for integrity
}

// ValidatePackage performs comprehensive validation of a package
func (pv *PackageValidator) ValidatePackage(ctx context.Context, repoPath string, manifest *ManifestData) (*ValidationResult, error) {
	if pv.debug {
		fmt.Printf("Validating package: %s\n", manifest.Name)
	}

	result := &ValidationResult{
		Valid:     true,
		Errors:    []string{},
		Warnings:  []string{},
		Checksums: make(map[string]string),
	}

	// Validate manifest structure
	if err := pv.validateManifestStructure(manifest, result); err != nil {
		return nil, fmt.Errorf("manifest validation failed: %w", err)
	}

	// Validate component files exist and are accessible
	if err := pv.validateComponentFiles(repoPath, manifest, result); err != nil {
		return nil, fmt.Errorf("component file validation failed: %w", err)
	}

	// Validate component types and configurations
	if err := pv.validateComponentTypes(manifest, result); err != nil {
		return nil, fmt.Errorf("component type validation failed: %w", err)
	}

	// Generate file checksums for integrity verification
	if err := pv.generateChecksums(repoPath, manifest, result); err != nil {
		return nil, fmt.Errorf("checksum generation failed: %w", err)
	}

	// Validate dependencies if present
	if err := pv.validateDependencies(manifest, result); err != nil {
		return nil, fmt.Errorf("dependency validation failed: %w", err)
	}

	// Validate gismo version requirements
	if err := pv.validateGismoRequirements(manifest, result); err != nil {
		return nil, fmt.Errorf("gismo requirements validation failed: %w", err)
	}

	// Set overall validity
	result.Valid = len(result.Errors) == 0

	if pv.debug {
		fmt.Printf("Package validation completed: valid=%t, errors=%d, warnings=%d\n",
			result.Valid, len(result.Errors), len(result.Warnings))
	}

	return result, nil
}

// validateManifestStructure validates the basic structure of the manifest
func (pv *PackageValidator) validateManifestStructure(manifest *ManifestData, result *ValidationResult) error {
	// Check required fields
	if manifest.Name == "" {
		result.Errors = append(result.Errors, "package name is required")
	} else if !isValidPackageName(manifest.Name) {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid package name '%s': must be lowercase alphanumeric with hyphens", manifest.Name))
	}

	if manifest.Version == "" {
		result.Errors = append(result.Errors, "package version is required")
	} else if _, err := version.ParseVersion(manifest.Version); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid version format '%s': %v", manifest.Version, err))
	}

	if len(manifest.Components) == 0 {
		result.Errors = append(result.Errors, "at least one component is required")
	}

	// Check optional fields for validity
	if manifest.Description != "" && len(manifest.Description) > 200 {
		result.Warnings = append(result.Warnings, "description is longer than recommended 200 characters")
	}

	if manifest.Author != "" && len(manifest.Author) > 100 {
		result.Warnings = append(result.Warnings, "author field is longer than recommended 100 characters")
	}

	return nil
}

// validateComponentFiles checks that all referenced component files exist
func (pv *PackageValidator) validateComponentFiles(repoPath string, manifest *ManifestData, result *ValidationResult) error {
	for groupName, components := range manifest.Components {
		for componentName, component := range components {
			sourcePath := filepath.Join(repoPath, component.Source)

			// Check if source file exists
			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				result.Errors = append(result.Errors,
					fmt.Sprintf("component %s.%s: source file '%s' does not exist", groupName, componentName, component.Source))
				continue
			}

			// Check file permissions and accessibility
			file, err := os.Open(sourcePath)
			if err != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("component %s.%s: cannot read source file '%s': %v", groupName, componentName, component.Source, err))
				continue
			}
			file.Close()

			// Validate file size (warn if very large)
			if info, err := os.Stat(sourcePath); err == nil {
				if info.Size() > 10*1024*1024 { // 10MB
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("component %s.%s: source file '%s' is very large (%d bytes)",
							groupName, componentName, component.Source, info.Size()))
				}
			}
		}
	}

	return nil
}

// validateComponentTypes validates component type configurations
func (pv *PackageValidator) validateComponentTypes(manifest *ManifestData, result *ValidationResult) error {
	validTypes := map[string]bool{
		"command":      true,
		"go-binary":    true,
		"shell-script": true,
		"go-linter":    true,
		"config":       true,
		"schema":       true,
	}

	for groupName, components := range manifest.Components {
		for componentName, component := range components {
			// Check if type is valid
			if !validTypes[component.Type] {
				result.Errors = append(result.Errors,
					fmt.Sprintf("component %s.%s: invalid type '%s'", groupName, componentName, component.Type))
				continue
			}

			// Type-specific validation
			switch component.Type {
			case "go-binary":
				if component.Build == "" {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("component %s.%s: go-binary type should specify build command", groupName, componentName))
				}
			case "shell-script":
				if !component.Executable {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("component %s.%s: shell-script should be executable", groupName, componentName))
				}
			case "go-linter":
				if len(component.Extensions) == 0 {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("component %s.%s: go-linter should specify file extensions", groupName, componentName))
				}
			}

			// Validate target path
			if strings.Contains(component.Target, "..") {
				result.Errors = append(result.Errors,
					fmt.Sprintf("component %s.%s: target path '%s' contains '..' which is not allowed",
						groupName, componentName, component.Target))
			}
		}
	}

	return nil
}

// generateChecksums generates SHA256 checksums for all component files
func (pv *PackageValidator) generateChecksums(repoPath string, manifest *ManifestData, result *ValidationResult) error {
	for groupName, components := range manifest.Components {
		for componentName, component := range components {
			sourcePath := filepath.Join(repoPath, component.Source)

			// Skip if file doesn't exist (already reported as error)
			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				continue
			}

			checksum, err := pv.calculateFileChecksum(sourcePath)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("component %s.%s: failed to calculate checksum: %v", groupName, componentName, err))
				continue
			}

			key := fmt.Sprintf("%s.%s", groupName, componentName)
			result.Checksums[key] = checksum
		}
	}

	return nil
}

// calculateFileChecksum calculates SHA256 checksum of a file
func (pv *PackageValidator) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// validateDependencies validates dependency specifications
func (pv *PackageValidator) validateDependencies(manifest *ManifestData, result *ValidationResult) error {
	for _, dep := range manifest.Dependencies {
		spec, err := ParseDependency(dep)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid dependency '%s': %v", dep, err))
			continue
		}

		// Validate version format
		if _, err := version.ParseVersion(spec.Version); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("dependency '%s' has invalid version: %v", dep, err))
		}

		// Check for self-dependency
		if spec.Name == manifest.Name {
			result.Errors = append(result.Errors, fmt.Sprintf("package cannot depend on itself: %s", dep))
		}
	}

	return nil
}

// validateGismoRequirements validates gismo version requirements
func (pv *PackageValidator) validateGismoRequirements(manifest *ManifestData, result *ValidationResult) error {
	if manifest.Gismo == nil {
		return nil
	}

	if manifest.Gismo.MinVersion != "" {
		if _, err := version.ParseVersion(manifest.Gismo.MinVersion); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid gismo minVersion: %v", err))
		}
	}

	if manifest.Gismo.MaxVersion != "" {
		if _, err := version.ParseVersion(manifest.Gismo.MaxVersion); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid gismo maxVersion: %v", err))
		}
	}

	// Check that minVersion <= maxVersion
	if manifest.Gismo.MinVersion != "" && manifest.Gismo.MaxVersion != "" {
		minVer, err1 := version.ParseVersion(manifest.Gismo.MinVersion)
		maxVer, err2 := version.ParseVersion(manifest.Gismo.MaxVersion)

		if err1 == nil && err2 == nil && minVer.Compare(maxVer) > 0 {
			result.Errors = append(result.Errors, "gismo minVersion cannot be greater than maxVersion")
		}
	}

	return nil
}

// ValidateAgainstCurrentGismo validates that the package is compatible with current gismo version
func (pv *PackageValidator) ValidateAgainstCurrentGismo(manifest *ManifestData, currentVersion string) error {
	if manifest.Gismo == nil {
		return nil // No requirements specified
	}

	currentVer, err := version.ParseVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("invalid current gismo version: %w", err)
	}

	if manifest.Gismo.MinVersion != "" {
		minVer, err := version.ParseVersion(manifest.Gismo.MinVersion)
		if err != nil {
			return fmt.Errorf("invalid minVersion in manifest: %w", err)
		}

		if currentVer.Compare(minVer) < 0 {
			return fmt.Errorf("package requires gismo %s or later, but current version is %s",
				minVer.String(), currentVer.String())
		}
	}

	if manifest.Gismo.MaxVersion != "" {
		maxVer, err := version.ParseVersion(manifest.Gismo.MaxVersion)
		if err != nil {
			return fmt.Errorf("invalid maxVersion in manifest: %w", err)
		}

		if currentVer.Compare(maxVer) > 0 {
			return fmt.Errorf("package requires gismo %s or earlier, but current version is %s",
				maxVer.String(), currentVer.String())
		}
	}

	return nil
}

// isValidPackageName checks if a package name follows the required format
func isValidPackageName(name string) bool {
	if len(name) == 0 || len(name) > 50 {
		return false
	}

	// Must start and end with alphanumeric, can contain hyphens in between
	if len(name) == 1 {
		return name[0] >= 'a' && name[0] <= 'z' || name[0] >= '0' && name[0] <= '9'
	}

	// Check first and last characters
	first := name[0]
	last := name[len(name)-1]
	if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
		return false
	}
	if !((last >= 'a' && last <= 'z') || (last >= '0' && last <= '9')) {
		return false
	}

	// Check middle characters
	for i := 1; i < len(name)-1; i++ {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}

	return true
}
