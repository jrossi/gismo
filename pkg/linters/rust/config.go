package rust

import (
	"encoding/json"
	"time"
)

// RustConfig represents rust linter specific configuration
type RustConfig struct {
	// ClippyConfig is the path to clippy.toml configuration file
	ClippyConfig *string `json:"clippyConfig,omitempty"`
	// RustfmtConfig is the path to rustfmt.toml configuration file
	RustfmtConfig *string `json:"rustfmtConfig,omitempty"`
	// DisabledLints is a list of clippy lints to disable
	DisabledLints []string `json:"disabledLints,omitempty"`
	// EnabledLints is a list of additional clippy lints to enable
	EnabledLints []string `json:"enabledLints,omitempty"`
	// TestTimeout is the timeout for running cargo test
	TestTimeout *Duration `json:"testTimeout,omitempty"`
	// NoDeps runs clippy only on the given crate, without linting dependencies
	NoDeps bool `json:"noDeps,omitempty"`
	// AllTargets checks all targets (lib, bin, test, example, etc.)
	AllTargets bool `json:"allTargets,omitempty"`
	// AllFeatures activates all available features
	AllFeatures bool `json:"allFeatures,omitempty"`
	// Features is a list of features to activate
	Features []string `json:"features,omitempty"`
	// Verbose enables verbose output
	Verbose bool `json:"verbose,omitempty"`
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

// DefaultRustConfig returns the default configuration for Rust linting
func DefaultRustConfig() *RustConfig {
	return &RustConfig{
		TestTimeout: &Duration{Duration: 10 * time.Minute},
		NoDeps:      true,  // Default to checking only the current crate
		AllTargets:  true,  // Check all targets by default
		AllFeatures: false, // Don't enable all features by default
		Verbose:     false,
	}
}
