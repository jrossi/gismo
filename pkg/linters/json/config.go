package json

import (
	"encoding/json"
	"fmt"
	"time"
)

// JSONFormat represents the JSON format type
type JSONFormat int

const (
	FormatJSON JSONFormat = iota
	FormatJSONLines
)

// ValidationLevel represents the level of JSON validation
type ValidationLevel int

const (
	ValidationSyntax ValidationLevel = iota
	ValidationStructure
	ValidationSchema
)

// JSONConfig holds configuration for the JSON linter
type JSONConfig struct {
	// MaxFileSize sets the maximum file size to process (default 1MB)
	MaxFileSize *int64 `json:"maxFileSize,omitempty"`

	// ValidationLevel sets the validation strictness level
	ValidationLevel *ValidationLevel `json:"validationLevel,omitempty"`

	// JSONSchema allows both file paths (string) and inline schemas (object)
	JSONSchema *json.RawMessage `json:"jsonSchema,omitempty"`

	// FormatDetection enables auto-detection of JSON vs JSON-L
	FormatDetection *bool `json:"formatDetection,omitempty"`

	// DisabledChecks list of checks to skip
	DisabledChecks []string `json:"disabledChecks,omitempty"`

	// StrictMode enables RFC 7159 strict compliance
	StrictMode *bool `json:"strictMode,omitempty"`

	// PrettyPrint enables formatted JSON output
	PrettyPrint *bool `json:"prettyPrint,omitempty"`

	// AllowComments enables parsing of JSON with comments (non-standard)
	AllowComments *bool `json:"allowComments,omitempty"`
}

// Duration wraps time.Duration for JSON marshaling
type Duration struct {
	time.Duration
}

// DefaultJSONConfig returns the default configuration for JSON linting
func DefaultJSONConfig() *JSONConfig {
	defaultMaxSize := int64(1024 * 1024) // 1MB
	defaultValidationLevel := ValidationSyntax
	defaultFormatDetection := true
	defaultStrictMode := false
	defaultPrettyPrint := false
	defaultAllowComments := false

	return &JSONConfig{
		MaxFileSize:     &defaultMaxSize,
		ValidationLevel: &defaultValidationLevel,
		FormatDetection: &defaultFormatDetection,
		DisabledChecks:  []string{},
		StrictMode:      &defaultStrictMode,
		PrettyPrint:     &defaultPrettyPrint,
		AllowComments:   &defaultAllowComments,
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

// UnmarshalJSON implements json.Unmarshaler for ValidationLevel
func (v *ValidationLevel) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch s {
	case "syntax":
		*v = ValidationSyntax
	case "structure":
		*v = ValidationStructure
	case "schema":
		*v = ValidationSchema
	default:
		return fmt.Errorf("invalid validation level: %s", s)
	}

	return nil
}

// MarshalJSON implements json.Marshaler for ValidationLevel
func (v ValidationLevel) MarshalJSON() ([]byte, error) {
	var s string
	switch v {
	case ValidationSyntax:
		s = "syntax"
	case ValidationStructure:
		s = "structure"
	case ValidationSchema:
		s = "schema"
	default:
		s = "syntax"
	}
	return json.Marshal(s)
}

// String returns the string representation of ValidationLevel
func (v ValidationLevel) String() string {
	switch v {
	case ValidationSyntax:
		return "syntax"
	case ValidationStructure:
		return "structure"
	case ValidationSchema:
		return "schema"
	default:
		return "syntax"
	}
}

// String returns the string representation of JSONFormat
func (f JSONFormat) String() string {
	switch f {
	case FormatJSON:
		return "json"
	case FormatJSONLines:
		return "jsonl"
	default:
		return "json"
	}
}
