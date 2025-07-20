package gismo

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kaptinlin/jsonschema"
)

//go:embed schemas/*.json
var schemas embed.FS

// ManifestValidator handles validation of registry manifest files
type ManifestValidator struct {
	schema *jsonschema.Schema
}

// NewManifestValidator creates a new manifest validator
func NewManifestValidator() (*ManifestValidator, error) {
	schemaBytes, err := schemas.ReadFile("schemas/manifest.schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(schemaBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile manifest schema: %w", err)
	}

	return &ManifestValidator{schema: schema}, nil
}

// ValidateManifest validates a manifest data structure
func (v *ManifestValidator) ValidateManifest(manifest *ManifestData) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}

	if err := v.schema.Validate(manifest); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}

	return nil
}

// ValidateManifestBytes validates raw JSON bytes as a manifest
func (v *ManifestValidator) ValidateManifestBytes(data []byte) (*ManifestData, error) {
	// First, parse the JSON
	var manifest ManifestData
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	// Then validate against schema
	if err := v.ValidateManifest(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// ValidateComponentType validates that a component type is supported
func ValidateComponentType(componentType string) error {
	validTypes := map[string]bool{
		"command":      true,
		"go-binary":    true,
		"shell-script": true,
		"go-linter":    true,
		"config":       true,
		"schema":       true,
	}

	if !validTypes[componentType] {
		return fmt.Errorf("unsupported component type: %s", componentType)
	}

	return nil
}

// GetComponentTargetPath returns the target path for a component type
func GetComponentTargetPath(componentType string) string {
	switch componentType {
	case "command":
		return "commands"
	case "go-binary":
		return "tools"
	case "shell-script":
		return "hooks"
	case "go-linter":
		return "linters"
	case "config":
		return "configs"
	case "schema":
		return "schemas"
	default:
		return "unknown"
	}
}

// ValidateGismoVersion checks if the current gismo version satisfies requirements
func ValidateGismoVersion(requirements *GismoRequirements, currentVersion string) error {
	if requirements == nil {
		return nil
	}

	// For now, just check that version is not empty
	// In a full implementation, this would use semantic versioning
	if requirements.MinVersion != "" && currentVersion == "" {
		return fmt.Errorf("gismo version required: %s", requirements.MinVersion)
	}

	return nil
}

// ManifestParser handles parsing and validation of manifest files
type ManifestParser struct {
	validator *ManifestValidator
}

// NewManifestParser creates a new manifest parser
func NewManifestParser() (*ManifestParser, error) {
	validator, err := NewManifestValidator()
	if err != nil {
		return nil, err
	}

	return &ManifestParser{
		validator: validator,
	}, nil
}

// ParseManifestFile parses a manifest file from disk
func (p *ManifestParser) ParseManifestFile(filePath string) (*ManifestData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file %s: %w", filePath, err)
	}

	return p.ParseManifestBytes(data)
}

// ParseManifestBytes parses manifest data from bytes
func (p *ManifestParser) ParseManifestBytes(data []byte) (*ManifestData, error) {
	return p.validator.ValidateManifestBytes(data)
}

// ValidateManifestIntegrity performs additional integrity checks on a manifest
func (p *ManifestParser) ValidateManifestIntegrity(manifest *ManifestData, currentVersion string) error {
	// Check gismo version compatibility
	if err := ValidateGismoVersion(manifest.Gismo, currentVersion); err != nil {
		return fmt.Errorf("version compatibility check failed: %w", err)
	}

	// Validate component types
	for groupName, group := range manifest.Components {
		for componentName, component := range group {
			if err := ValidateComponentType(component.Type); err != nil {
				return fmt.Errorf("invalid component %s.%s: %w", groupName, componentName, err)
			}

			// Check type-specific requirements
			if err := p.validateComponentRequirements(component); err != nil {
				return fmt.Errorf("component %s.%s validation failed: %w", groupName, componentName, err)
			}
		}
	}

	// Validate dependencies format
	for _, dep := range manifest.Dependencies {
		if err := p.validateDependency(dep); err != nil {
			return fmt.Errorf("invalid dependency '%s': %w", dep, err)
		}
	}

	return nil
}

// validateComponentRequirements validates type-specific component requirements
func (p *ManifestParser) validateComponentRequirements(component *Component) error {
	switch component.Type {
	case "go-binary":
		if component.Build == "" {
			return fmt.Errorf("go-binary components must specify a build command")
		}
	case "go-linter":
		if len(component.Extensions) == 0 {
			return fmt.Errorf("go-linter components must specify file extensions")
		}
		// Validate extension format
		for _, ext := range component.Extensions {
			if !strings.HasPrefix(ext, ".") {
				return fmt.Errorf("file extension '%s' must start with a dot", ext)
			}
		}
	case "shell-script":
		// Shell scripts should be executable (this is validated but not enforced as an error)
	}

	return nil
}

// validateDependency validates a dependency string format
func (p *ManifestParser) validateDependency(dependency string) error {
	// Expected format: repository@version
	// Example: github.com/user/repo@v1.0.0
	parts := strings.Split(dependency, "@")
	if len(parts) != 2 {
		return fmt.Errorf("dependency must be in format 'repository@version'")
	}

	repository := parts[0]
	version := parts[1]

	if repository == "" {
		return fmt.Errorf("repository cannot be empty")
	}

	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Basic version format check (should start with 'v')
	if !strings.HasPrefix(version, "v") {
		return fmt.Errorf("version must start with 'v' (e.g., v1.0.0)")
	}

	return nil
}

// GetComponentInstallPath returns the full installation path for a component
func (p *ManifestParser) GetComponentInstallPath(component *Component, claudeDir string, namespace string) string {
	basePath := GetComponentTargetPath(component.Type)
	return fmt.Sprintf("%s/%s/%s/%s", claudeDir, basePath, namespace, component.Target)
}

// ListAllComponents returns a flat list of all components in a manifest
func (p *ManifestParser) ListAllComponents(manifest *ManifestData) []*ComponentInfo {
	var components []*ComponentInfo

	for groupName, group := range manifest.Components {
		for componentName, component := range group {
			components = append(components, &ComponentInfo{
				Group:     groupName,
				Name:      componentName,
				Component: component,
			})
		}
	}

	return components
}

// ComponentInfo represents a component with its group and name
type ComponentInfo struct {
	Group     string
	Name      string
	Component *Component
}

// String returns a string representation of the component
func (ci *ComponentInfo) String() string {
	return fmt.Sprintf("%s.%s (%s)", ci.Group, ci.Name, ci.Component.Type)
}
