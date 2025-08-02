package python

import (
	"encoding/json"
	"fmt"
	"time"
)

// PythonConfig holds configuration for the Python linter
type PythonConfig struct {
	// Ruff configuration via uvx
	RuffArgs      []string `json:"ruffArgs,omitempty"`
	MaxLineLength *int     `json:"maxLineLength,omitempty"`

	// Type checking via uvx
	TypeChecker   string   `json:"typeChecker,omitempty"` // e.g., "mypy", "pyright"
	TypeCheckArgs []string `json:"typeCheckArgs,omitempty"`

	// Test runner configuration
	TestRunner  string    `json:"testRunner,omitempty"` // e.g., "pytest", "unittest"
	TestArgs    []string  `json:"testArgs,omitempty"`
	TestTimeout *Duration `json:"testTimeout,omitempty"`
	RunTests    bool      `json:"runTests,omitempty"`
}

// Duration wraps time.Duration for JSON marshaling
type Duration struct {
	time.Duration
}

// DefaultPythonConfig returns the default configuration for Python linting
func DefaultPythonConfig() *PythonConfig {
	defaultTimeout := &Duration{Duration: 2 * time.Minute}
	defaultLineLength := 88 // Ruff default

	return &PythonConfig{
		RuffArgs:      []string{},
		MaxLineLength: &defaultLineLength,
		TypeChecker:   "mypy",
		TypeCheckArgs: []string{"--strict"},
		TestRunner:    "pytest",
		TestArgs:      []string{"-v"},
		TestTimeout:   defaultTimeout,
		RunTests:      true,
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
