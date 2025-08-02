package protobuf

import (
	"encoding/json"
	"time"
)

// ProtobufConfig represents protobuf linter specific configuration
type ProtobufConfig struct {
	// PreferredTools is the list of tools to try in order
	PreferredTools []string `json:"preferredTools,omitempty"`
	// ForceTool forces a specific tool to be used
	ForceTool *string `json:"forceTool,omitempty"`
	// BufPath is the path to the buf binary
	BufPath *string `json:"bufPath,omitempty"`
	// ProtocPath is the path to the protoc binary
	ProtocPath *string `json:"protocPath,omitempty"`
	// ProtolintPath is the path to the protolint binary
	ProtolintPath *string `json:"protolintPath,omitempty"`
	// BufConfigPath is the path to buf.yaml configuration
	BufConfigPath *string `json:"bufConfigPath,omitempty"`
	// BufWorkPath is the path to buf.work.yaml for workspaces
	BufWorkPath *string `json:"bufWorkPath,omitempty"`
	// DisabledChecks is a list of buf lint checks to disable
	DisabledChecks []string `json:"disabledChecks,omitempty"`
	// Categories is a list of buf lint categories to check
	Categories []string `json:"categories,omitempty"`
	// ProtolintConfig is the path to .protolint.yaml configuration
	ProtolintConfig *string `json:"protolintConfig,omitempty"`
	// MaxFileSize is the maximum file size in bytes to lint
	MaxFileSize *int64 `json:"maxFileSize,omitempty"`
	// TestTimeout is the timeout for running tests
	TestTimeout *Duration `json:"testTimeout,omitempty"`
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

// DefaultProtobufConfig returns the default configuration for Protobuf linting
func DefaultProtobufConfig() *ProtobufConfig {
	return &ProtobufConfig{
		PreferredTools: []string{"buf", "protolint", "protoc"},
		TestTimeout:    &Duration{Duration: 2 * time.Minute},
		MaxFileSize:    intPtr(10 * 1024 * 1024), // 10MB
		Verbose:        false,
	}
}

func intPtr(i int64) *int64 {
	return &i
}
