package engine

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAppConfig_Merge(t *testing.T) {
	tests := []struct {
		name     string
		base     *AppConfig
		other    *AppConfig
		expected *AppConfig
	}{
		{
			name: "merge_parallel_config",
			base: &AppConfig{
				Parallel: &ParallelConfig{
					MaxWorkers: intPtr(4),
				},
			},
			other: &AppConfig{
				Parallel: &ParallelConfig{
					DisableParallel: boolPtr(true),
				},
			},
			expected: &AppConfig{
				Parallel: &ParallelConfig{
					MaxWorkers:      intPtr(4),
					DisableParallel: boolPtr(true),
				},
				Linters: map[string]LinterConfig{},
				Rules:   []RuleOverride{},
			},
		},
		{
			name: "override_timeout",
			base: &AppConfig{
				Timeout: &Duration{Duration: 30 * time.Second},
			},
			other: &AppConfig{
				Timeout: &Duration{Duration: 60 * time.Second},
			},
			expected: &AppConfig{
				Timeout: &Duration{Duration: 60 * time.Second},
				Linters: map[string]LinterConfig{},
				Rules:   []RuleOverride{},
			},
		},
		{
			name: "merge_linters",
			base: &AppConfig{
				Linters: map[string]LinterConfig{
					"golang": {
						Enabled: boolPtr(true),
					},
				},
			},
			other: &AppConfig{
				Linters: map[string]LinterConfig{
					"golang": {
						Config: json.RawMessage(`{"maxLineLength": 120}`),
					},
					"markdown": {
						Enabled: boolPtr(false),
					},
				},
			},
			expected: &AppConfig{
				Linters: map[string]LinterConfig{
					"golang": {
						Enabled: boolPtr(true),
						Config:  json.RawMessage(`{"maxLineLength": 120}`),
					},
					"markdown": {
						Enabled: boolPtr(false),
					},
				},
				Rules: []RuleOverride{},
			},
		},
		{
			name: "append_rules",
			base: &AppConfig{
				Rules: []RuleOverride{
					{Pattern: "*.go", Linter: "golang", Rules: json.RawMessage(`{}`)},
				},
			},
			other: &AppConfig{
				Rules: []RuleOverride{
					{Pattern: "*.md", Linter: "markdown", Rules: json.RawMessage(`{}`)},
				},
			},
			expected: &AppConfig{
				Linters: map[string]LinterConfig{},
				Rules: []RuleOverride{
					{Pattern: "*.go", Linter: "golang", Rules: json.RawMessage(`{}`)},
					{Pattern: "*.md", Linter: "markdown", Rules: json.RawMessage(`{}`)},
				},
			},
		},
		{
			name:  "merge_with_nil",
			base:  NewAppConfig(),
			other: nil,
			expected: &AppConfig{
				Linters: map[string]LinterConfig{},
				Rules:   []RuleOverride{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)

			// Compare the result
			if !configsEqual(tt.base, tt.expected) {
				t.Errorf("Config merge mismatch\nGot:      %+v\nExpected: %+v", tt.base, tt.expected)
			}
		})
	}
}

func TestAppConfig_GetLinterConfig(t *testing.T) {
	config := &AppConfig{
		Linters: map[string]LinterConfig{
			"golang": {
				Enabled: boolPtr(true),
				Config:  json.RawMessage(`{"test": true}`),
			},
			"markdown": {
				Enabled: boolPtr(false),
			},
		},
	}

	tests := []struct {
		name       string
		linterName string
		wantConfig bool
		wantOk     bool
	}{
		{
			name:       "existing_linter_with_config",
			linterName: "golang",
			wantConfig: true,
			wantOk:     true,
		},
		{
			name:       "existing_linter_no_config",
			linterName: "markdown",
			wantConfig: false,
			wantOk:     false,
		},
		{
			name:       "non_existent_linter",
			linterName: "python",
			wantConfig: false,
			wantOk:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, ok := config.GetLinterConfig(tt.linterName)
			if ok != tt.wantOk {
				t.Errorf("GetLinterConfig() ok = %v, want %v", ok, tt.wantOk)
			}
			if tt.wantConfig && config == nil {
				t.Error("Expected config but got nil")
			}
			if !tt.wantConfig && config != nil {
				t.Error("Expected no config but got one")
			}
		})
	}
}

func TestAppConfig_IsLinterEnabled(t *testing.T) {
	config := &AppConfig{
		Linters: map[string]LinterConfig{
			"golang": {
				Enabled: boolPtr(true),
			},
			"markdown": {
				Enabled: boolPtr(false),
			},
			"python": {
				Config: json.RawMessage(`{}`),
			},
		},
	}

	tests := []struct {
		name       string
		linterName string
		want       bool
	}{
		{
			name:       "explicitly_enabled",
			linterName: "golang",
			want:       true,
		},
		{
			name:       "explicitly_disabled",
			linterName: "markdown",
			want:       false,
		},
		{
			name:       "no_enabled_field",
			linterName: "python",
			want:       true, // defaults to enabled
		},
		{
			name:       "non_existent_linter",
			linterName: "rust",
			want:       true, // defaults to enabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := config.IsLinterEnabled(tt.linterName); got != tt.want {
				t.Errorf("IsLinterEnabled(%q) = %v, want %v", tt.linterName, got, tt.want)
			}
		})
	}
}

func TestDuration_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:  "valid_duration",
			input: `"5m"`,
			want:  5 * time.Minute,
		},
		{
			name:  "seconds",
			input: `"30s"`,
			want:  30 * time.Second,
		},
		{
			name:  "complex_duration",
			input: `"1h30m45s"`,
			want:  time.Hour + 30*time.Minute + 45*time.Second,
		},
		{
			name:    "invalid_format",
			input:   `"invalid"`,
			wantErr: true,
		},
		{
			name:    "not_string",
			input:   `123`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tt.input), &d)
			if (err != nil) != tt.wantErr {
				t.Errorf("Duration.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && d.Duration != tt.want {
				t.Errorf("Duration.UnmarshalJSON() = %v, want %v", d.Duration, tt.want)
			}
		})
	}
}

func TestDuration_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		d    Duration
		want string
	}{
		{
			name: "minutes",
			d:    Duration{Duration: 5 * time.Minute},
			want: `"5m0s"`,
		},
		{
			name: "complex",
			d:    Duration{Duration: time.Hour + 30*time.Minute + 45*time.Second},
			want: `"1h30m45s"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.d)
			if err != nil {
				t.Errorf("Duration.MarshalJSON() error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("Duration.MarshalJSON() = %s, want %s", got, tt.want)
			}
		})
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func configsEqual(a, b *AppConfig) bool {
	// This is a simplified comparison for testing
	// In production, you might want to use reflect.DeepEqual or a more thorough comparison

	// Compare Parallel
	if (a.Parallel == nil) != (b.Parallel == nil) {
		return false
	}
	if a.Parallel != nil && b.Parallel != nil {
		if !intPtrsEqual(a.Parallel.MaxWorkers, b.Parallel.MaxWorkers) {
			return false
		}
		if !boolPtrsEqual(a.Parallel.DisableParallel, b.Parallel.DisableParallel) {
			return false
		}
	}

	// Compare Timeout
	if (a.Timeout == nil) != (b.Timeout == nil) {
		return false
	}
	if a.Timeout != nil && b.Timeout != nil {
		if a.Timeout.Duration != b.Timeout.Duration {
			return false
		}
	}

	// Compare Linters
	if len(a.Linters) != len(b.Linters) {
		return false
	}
	for k, v1 := range a.Linters {
		v2, ok := b.Linters[k]
		if !ok {
			return false
		}
		if !boolPtrsEqual(v1.Enabled, v2.Enabled) {
			return false
		}
		if string(v1.Config) != string(v2.Config) {
			return false
		}
	}

	// Compare Rules
	if len(a.Rules) != len(b.Rules) {
		return false
	}
	for i := range a.Rules {
		if a.Rules[i].Pattern != b.Rules[i].Pattern ||
			a.Rules[i].Linter != b.Rules[i].Linter ||
			string(a.Rules[i].Rules) != string(b.Rules[i].Rules) {
			return false
		}
	}

	return true
}

func intPtrsEqual(a, b *int) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a != nil && b != nil {
		return *a == *b
	}
	return true
}

func boolPtrsEqual(a, b *bool) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a != nil && b != nil {
		return *a == *b
	}
	return true
}

func TestAppConfig_GetRuleOverrides(t *testing.T) {
	config := &AppConfig{
		Rules: []RuleOverride{
			{
				Pattern: "*.go",
				Linter:  "golang",
				Rules:   json.RawMessage(`{"disabledChecks": ["gofmt"]}`),
			},
			{
				Pattern: "*_test.go",
				Linter:  "golang",
				Rules:   json.RawMessage(`{"testTimeout": "5m"}`),
			},
			{
				Pattern: "*.md",
				Linter:  "markdown",
				Rules:   json.RawMessage(`{"maxLineLength": 80}`),
			},
			{
				Pattern: "docs/*.md",
				Linter:  "markdown",
				Rules:   json.RawMessage(`{"requireFrontmatter": true}`),
			},
			{
				Pattern: "*.go",
				Linter:  "*",
				Rules:   json.RawMessage(`{"verbose": true}`),
			},
		},
	}

	tests := []struct {
		name       string
		filePath   string
		linterName string
		wantCount  int
	}{
		{
			name:       "golang_file_matches_two_rules",
			filePath:   "main.go",
			linterName: "golang",
			wantCount:  2, // *.go for golang and *.go for *
		},
		{
			name:       "test_file_matches_three_rules",
			filePath:   "main_test.go",
			linterName: "golang",
			wantCount:  3, // *.go, *_test.go for golang, and *.go for *
		},
		{
			name:       "markdown_file_basic",
			filePath:   "README.md",
			linterName: "markdown",
			wantCount:  1, // *.md
		},
		{
			name:       "markdown_file_in_docs",
			filePath:   "docs/guide.md",
			linterName: "markdown",
			wantCount:  2, // *.md and docs/*.md
		},
		{
			name:       "no_match",
			filePath:   "script.py",
			linterName: "python",
			wantCount:  0,
		},
		{
			name:       "wildcard_linter_match",
			filePath:   "main.go",
			linterName: "anylinter",
			wantCount:  1, // *.go for *
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overrides := config.GetRuleOverrides(tt.filePath, tt.linterName)
			if len(overrides) != tt.wantCount {
				t.Errorf("GetRuleOverrides(%q, %q) returned %d overrides, want %d",
					tt.filePath, tt.linterName, len(overrides), tt.wantCount)
			}
		})
	}
}
