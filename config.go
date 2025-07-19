package gismo

import (
	"encoding/json"
	"path/filepath"
	"time"
)

// AppConfig represents the complete configuration for gismo
type AppConfig struct {
	// Global settings
	Parallel *ParallelConfig `json:"parallel,omitempty"`
	Timeout  *Duration       `json:"timeout,omitempty"`

	// Linter configurations keyed by linter name
	Linters map[string]LinterConfig `json:"linters,omitempty"`

	// Rule overrides by file pattern
	Rules []RuleOverride `json:"rules,omitempty"`

	// Registry configuration
	Registry *RegistryConfig `json:"registry,omitempty"`
}

// ParallelConfig controls parallel execution settings
type ParallelConfig struct {
	MaxWorkers      *int  `json:"maxWorkers,omitempty"`
	DisableParallel *bool `json:"disableParallel,omitempty"`
}

// LinterConfig represents configuration for a specific linter
type LinterConfig struct {
	Enabled *bool           `json:"enabled,omitempty"`
	Config  json.RawMessage `json:"config,omitempty"`
}

// RuleOverride applies linter-specific rules based on file patterns
type RuleOverride struct {
	Pattern string          `json:"pattern"` // glob pattern for files
	Linter  string          `json:"linter"`  // which linter this applies to
	Rules   json.RawMessage `json:"rules"`   // linter-specific rule configuration
}

// Duration is a wrapper around time.Duration for JSON unmarshaling
type Duration struct {
	time.Duration
}

// UnmarshalJSON implements json.Unmarshaler for Duration
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = duration
	return nil
}

// MarshalJSON implements json.Marshaler for Duration
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

// MarkdownConfig represents markdown linter specific configuration
type MarkdownConfig struct {
	MaxLineLength      *int             `json:"maxLineLength,omitempty"`
	RequireFrontmatter *bool            `json:"requireFrontmatter,omitempty"`
	FrontmatterSchema  *json.RawMessage `json:"frontmatterSchema,omitempty"`
	DisabledRules      []string         `json:"disabledRules,omitempty"`
	MaxBlankLines      *int             `json:"maxBlankLines,omitempty"`
	ListIndentSize     *int             `json:"listIndentSize,omitempty"`
}

// GolangConfig represents golang linter specific configuration
type GolangConfig struct {
	GolangciConfig *string   `json:"golangciConfig,omitempty"` // path to golangci.yml
	DisabledChecks []string  `json:"disabledChecks,omitempty"`
	TestTimeout    *Duration `json:"testTimeout,omitempty"`
}

// RegistryConfig holds all registry and package configuration
type RegistryConfig struct {
	Registries map[string]*RegistryEntry `json:"registries,omitempty"`
	Packages   map[string]*PackageEntry  `json:"packages,omitempty"`
}

// RegistryEntry represents a configured registry source
type RegistryEntry struct {
	URL         string    `json:"url"`               // Git repository URL
	GitSHA      string    `json:"gitSHA"`            // Specific commit SHA (like go.sum)
	Version     string    `json:"version,omitempty"` // Semantic version if tagged
	Scope       string    `json:"scope"`             // "global" or "project"
	InstallDate time.Time `json:"installDate"`       // When installed
	UpdatedDate time.Time `json:"updatedDate"`       // When last updated
}

// PackageEntry represents an installed package from a registry
type PackageEntry struct {
	RegistryName string            `json:"registryName"` // Which registry this came from
	GitSHA       string            `json:"gitSHA"`       // SHA for integrity verification
	Installed    map[string]string `json:"installed"`    // component -> file path mapping
	Manifest     *ManifestData     `json:"manifest"`     // Cached manifest data
}

// ManifestData represents the parsed manifest.json from a registry
type ManifestData struct {
	Version      string                           `json:"version"`
	Name         string                           `json:"name"`
	Description  string                           `json:"description,omitempty"`
	Author       string                           `json:"author,omitempty"`
	Homepage     string                           `json:"homepage,omitempty"`
	License      string                           `json:"license,omitempty"`
	Gismo        *GismoRequirements               `json:"gismo,omitempty"`
	Components   map[string]map[string]*Component `json:"components,omitempty"`
	Dependencies []string                         `json:"dependencies,omitempty"`
	Hooks        *ManifestHooks                   `json:"hooks,omitempty"`
	Config       *ManifestConfig                  `json:"config,omitempty"`
}

// GismoRequirements specifies version requirements for gismo
type GismoRequirements struct {
	MinVersion string `json:"minVersion,omitempty"`
	MaxVersion string `json:"maxVersion,omitempty"`
}

// Component represents a single component in a manifest
type Component struct {
	Source      string   `json:"source"`               // Source path in repository
	Target      string   `json:"target"`               // Target path relative to .claude/
	Type        string   `json:"type"`                 // Component type
	Build       string   `json:"build,omitempty"`      // Build command for go-binary
	Executable  bool     `json:"executable,omitempty"` // Set executable permissions
	Extensions  []string `json:"extensions,omitempty"` // File extensions for linters
	Priority    int      `json:"priority,omitempty"`   // Priority for linters
	Description string   `json:"description,omitempty"`
}

// ManifestHooks defines lifecycle hooks for packages
type ManifestHooks struct {
	PostInstall string `json:"postInstall,omitempty"`
	PreRemove   string `json:"preRemove,omitempty"`
}

// ManifestConfig defines package-specific configuration
type ManifestConfig struct {
	Schema  string          `json:"schema,omitempty"`
	Default json.RawMessage `json:"default,omitempty"`
}

// NewAppConfig creates a new AppConfig with default values
func NewAppConfig() *AppConfig {
	return &AppConfig{
		Linters: make(map[string]LinterConfig),
		Rules:   []RuleOverride{},
	}
}

// Merge combines two configs, with other taking precedence
func (c *AppConfig) Merge(other *AppConfig) {
	if other == nil {
		return
	}

	// Merge parallel config
	if other.Parallel != nil {
		if c.Parallel == nil {
			c.Parallel = &ParallelConfig{}
		}
		if other.Parallel.MaxWorkers != nil {
			c.Parallel.MaxWorkers = other.Parallel.MaxWorkers
		}
		if other.Parallel.DisableParallel != nil {
			c.Parallel.DisableParallel = other.Parallel.DisableParallel
		}
	}

	// Merge timeout
	if other.Timeout != nil {
		c.Timeout = other.Timeout
	}

	// Merge linters
	if c.Linters == nil {
		c.Linters = make(map[string]LinterConfig)
	}
	for name, linterConfig := range other.Linters {
		existing, exists := c.Linters[name]
		if !exists {
			c.Linters[name] = linterConfig
		} else {
			// Merge linter config
			if linterConfig.Enabled != nil {
				existing.Enabled = linterConfig.Enabled
			}
			if linterConfig.Config != nil {
				existing.Config = linterConfig.Config
			}
			c.Linters[name] = existing
		}
	}

	// Append rules (don't merge, later rules take precedence)
	c.Rules = append(c.Rules, other.Rules...)

	// Merge registry config
	if other.Registry != nil {
		if c.Registry == nil {
			c.Registry = &RegistryConfig{
				Registries: make(map[string]*RegistryEntry),
				Packages:   make(map[string]*PackageEntry),
			}
		}
		// Merge registries
		if c.Registry.Registries == nil {
			c.Registry.Registries = make(map[string]*RegistryEntry)
		}
		for name, registry := range other.Registry.Registries {
			c.Registry.Registries[name] = registry
		}
		// Merge packages
		if c.Registry.Packages == nil {
			c.Registry.Packages = make(map[string]*PackageEntry)
		}
		for name, pkg := range other.Registry.Packages {
			c.Registry.Packages[name] = pkg
		}
	}
}

// GetLinterConfig returns the configuration for a specific linter
func (c *AppConfig) GetLinterConfig(name string) (json.RawMessage, bool) {
	if c.Linters == nil {
		return nil, false
	}
	linterConfig, ok := c.Linters[name]
	if !ok || linterConfig.Config == nil {
		return nil, false
	}
	return linterConfig.Config, true
}

// IsLinterEnabled checks if a linter is enabled
func (c *AppConfig) IsLinterEnabled(name string) bool {
	if c.Linters == nil {
		return true // default to enabled
	}
	linterConfig, ok := c.Linters[name]
	if !ok || linterConfig.Enabled == nil {
		return true // default to enabled
	}
	return *linterConfig.Enabled
}

// GetRuleOverrides returns all rule overrides that match the given file path for a specific linter
func (c *AppConfig) GetRuleOverrides(filePath, linterName string) []json.RawMessage {
	if len(c.Rules) == 0 {
		return nil
	}

	var overrides []json.RawMessage
	for _, rule := range c.Rules {
		// Check if this rule applies to the given linter
		if rule.Linter != linterName && rule.Linter != "*" {
			continue
		}

		// Check if the pattern matches the file path
		matched, err := filepath.Match(rule.Pattern, filePath)
		if err != nil {
			// Invalid pattern, skip
			continue
		}

		if !matched {
			// Also check against just the filename
			matched, _ = filepath.Match(rule.Pattern, filepath.Base(filePath))
		}

		if matched {
			overrides = append(overrides, rule.Rules)
		}
	}

	return overrides
}

// GetRegistryConfig returns the registry configuration, initializing if needed
func (c *AppConfig) GetRegistryConfig() *RegistryConfig {
	if c.Registry == nil {
		c.Registry = &RegistryConfig{
			Registries: make(map[string]*RegistryEntry),
			Packages:   make(map[string]*PackageEntry),
		}
	}
	return c.Registry
}

// GetRegistry returns a specific registry entry by name
func (c *AppConfig) GetRegistry(name string) (*RegistryEntry, bool) {
	if c.Registry == nil || c.Registry.Registries == nil {
		return nil, false
	}
	entry, ok := c.Registry.Registries[name]
	return entry, ok
}

// AddRegistry adds or updates a registry entry
func (c *AppConfig) AddRegistry(name string, entry *RegistryEntry) {
	registryConfig := c.GetRegistryConfig()
	if registryConfig.Registries == nil {
		registryConfig.Registries = make(map[string]*RegistryEntry)
	}
	registryConfig.Registries[name] = entry
}

// RemoveRegistry removes a registry entry by name
func (c *AppConfig) RemoveRegistry(name string) bool {
	if c.Registry == nil || c.Registry.Registries == nil {
		return false
	}
	_, exists := c.Registry.Registries[name]
	if exists {
		delete(c.Registry.Registries, name)
	}
	return exists
}

// GetPackage returns a specific package entry by name
func (c *AppConfig) GetPackage(name string) (*PackageEntry, bool) {
	if c.Registry == nil || c.Registry.Packages == nil {
		return nil, false
	}
	entry, ok := c.Registry.Packages[name]
	return entry, ok
}

// AddPackage adds or updates a package entry
func (c *AppConfig) AddPackage(name string, entry *PackageEntry) {
	registryConfig := c.GetRegistryConfig()
	if registryConfig.Packages == nil {
		registryConfig.Packages = make(map[string]*PackageEntry)
	}
	registryConfig.Packages[name] = entry
}

// RemovePackage removes a package entry by name
func (c *AppConfig) RemovePackage(name string) bool {
	if c.Registry == nil || c.Registry.Packages == nil {
		return false
	}
	_, exists := c.Registry.Packages[name]
	if exists {
		delete(c.Registry.Packages, name)
	}
	return exists
}

// ListRegistries returns all registry names
func (c *AppConfig) ListRegistries() []string {
	if c.Registry == nil || c.Registry.Registries == nil {
		return nil
	}
	names := make([]string, 0, len(c.Registry.Registries))
	for name := range c.Registry.Registries {
		names = append(names, name)
	}
	return names
}

// ListPackages returns all package names
func (c *AppConfig) ListPackages() []string {
	if c.Registry == nil || c.Registry.Packages == nil {
		return nil
	}
	names := make([]string, 0, len(c.Registry.Packages))
	for name := range c.Registry.Packages {
		names = append(names, name)
	}
	return names
}
