package javascript

import (
	"encoding/json"
	"fmt"
	"time"
)

// JavaScriptConfig holds configuration for the JavaScript/TypeScript linter
type JavaScriptConfig struct {
	// Tool Selection and Forcing
	ForceTool      *string  `json:"forceTool,omitempty"`      // "biome", "oxlint", "eslint", "node" - skip discovery
	PreferredTools []string `json:"preferredTools,omitempty"` // Priority order for discovery

	// Performance and Limits
	MaxFileSize *int64    `json:"maxFileSize,omitempty"` // Default 10MB (larger than other linters)
	TestTimeout *Duration `json:"testTimeout,omitempty"` // Tool execution timeout

	// Configuration Paths (skip discovery if specified)
	BiomeConfigPath  *string `json:"biomeConfigPath,omitempty"`  // Force specific biome.json
	ESLintConfigPath *string `json:"eslintConfigPath,omitempty"` // Force specific .eslintrc
	OxlintConfigPath *string `json:"oxlintConfigPath,omitempty"` // Force specific .oxlintrc.json
	TSConfigPath     *string `json:"tsconfigPath,omitempty"`     // Force specific tsconfig.json

	// Tool Paths (skip discovery if specified)
	BiomePath  *string `json:"biomePath,omitempty"`  // Force specific biome binary
	OxlintPath *string `json:"oxlintPath,omitempty"` // Force specific oxlint binary
	ESLintPath *string `json:"eslintPath,omitempty"` // Force specific eslint binary
	NodePath   *string `json:"nodePath,omitempty"`   // Force specific node binary

	// Rule Configuration
	DisabledChecks  []string `json:"disabledChecks,omitempty"`  // Tool-agnostic rule names
	IncludePatterns []string `json:"includePatterns,omitempty"` // File patterns to include
	ExcludePatterns []string `json:"excludePatterns,omitempty"` // File patterns to exclude

	// Project Context
	WorkspaceRoot   *string `json:"workspaceRoot,omitempty"`   // Monorepo root directory
	PackageJsonPath *string `json:"packageJsonPath,omitempty"` // Force specific package.json
}

// Duration wraps time.Duration for JSON marshaling
type Duration struct {
	time.Duration
}

// DefaultJavaScriptConfig returns the default configuration for JavaScript/TypeScript linting
func DefaultJavaScriptConfig() *JavaScriptConfig {
	defaultMaxSize := int64(10 * 1024 * 1024) // 10MB (larger than other linters for JS projects)
	defaultTimeout := Duration{30 * time.Second}
	defaultPreferredTools := []string{"biome", "oxlint", "eslint"}

	return &JavaScriptConfig{
		MaxFileSize:    &defaultMaxSize,
		TestTimeout:    &defaultTimeout,
		PreferredTools: defaultPreferredTools,
		DisabledChecks: []string{},
	}
}

// UnmarshalJSON implements json.Unmarshaler for Duration
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid duration type: %T", v)
	}
}

// MarshalJSON implements json.Marshaler for Duration
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}
