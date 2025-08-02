package javascript

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestJavaScriptLinter_CanHandle(t *testing.T) {
	linter := NewJavaScriptLinter()

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"JavaScript file", "test.js", true},
		{"JSX file", "component.jsx", true},
		{"TypeScript file", "module.ts", true},
		{"TSX file", "component.tsx", true},
		{"ES Module", "module.mjs", true},
		{"CommonJS", "script.cjs", true},
		{"Vue component", "App.vue", true},
		{"Svelte component", "Button.svelte", true},
		{"JavaScript with path", "/path/to/script.js", true},
		{"TypeScript with path", "/src/utils/helper.ts", true},
		{"Go file", "main.go", false},
		{"Python file", "script.py", false},
		{"Text file", "readme.txt", false},
		{"No extension", "Makefile", false},
		{"Hidden JS file", ".hidden.js", true},
		{"Case insensitive", "Test.JS", true},
		{"Case insensitive TS", "Module.TS", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linter.CanHandle(tt.filePath)
			if got != tt.want {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestJavaScriptLinter_Name(t *testing.T) {
	linter := NewJavaScriptLinter()
	if got := linter.Name(); got != "javascript" {
		t.Errorf("Name() = %v, want %v", got, "javascript")
	}
}

func TestJavaScriptLinter_SetConfig(t *testing.T) {
	linter := NewJavaScriptLinter()

	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{
			name:    "Valid config",
			config:  `{"forceTool": "biome", "maxFileSize": 5000000}`,
			wantErr: false,
		},
		{
			name:    "Valid config with preferred tools",
			config:  `{"preferredTools": ["oxlint", "eslint"], "testTimeout": "30s"}`,
			wantErr: false,
		},
		{
			name:    "Valid config with paths",
			config:  `{"biomePath": "/usr/bin/biome", "eslintPath": "/usr/bin/eslint"}`,
			wantErr: false,
		},
		{
			name:    "Empty config",
			config:  `{}`,
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			config:  `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "Invalid field type",
			config:  `{"maxFileSize": "not a number"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := linter.SetConfig([]byte(tt.config))
			if (err != nil) != tt.wantErr {
				t.Errorf("SetConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJavaScriptLinter_DefaultConfig(t *testing.T) {
	config := DefaultJavaScriptConfig()

	if config == nil {
		t.Fatal("DefaultJavaScriptConfig() returned nil")
	}

	if config.MaxFileSize == nil || *config.MaxFileSize != 10*1024*1024 {
		t.Errorf("Default MaxFileSize = %v, want %d", config.MaxFileSize, 10*1024*1024)
	}

	if config.TestTimeout == nil || config.TestTimeout.Duration != 30*time.Second {
		t.Errorf("Default TestTimeout = %v, want %v", config.TestTimeout, 30*time.Second)
	}

	expectedTools := []string{"biome", "oxlint", "eslint"}
	if len(config.PreferredTools) != len(expectedTools) {
		t.Errorf("Default PreferredTools length = %d, want %d", len(config.PreferredTools), len(expectedTools))
	}

	for i, tool := range expectedTools {
		if i >= len(config.PreferredTools) || config.PreferredTools[i] != tool {
			t.Errorf("Default PreferredTools[%d] = %s, want %s", i, config.PreferredTools[i], tool)
		}
	}
}

func TestJavaScriptLinter_Lint_FileSize(t *testing.T) {
	linter := NewJavaScriptLinter()

	// Set a very small file size limit for testing
	smallLimit := int64(10)
	linter.config.MaxFileSize = &smallLimit

	largeContent := []byte("console.log('This is a large file that exceeds the limit');")

	result, err := linter.Lint(context.Background(), "test.js", largeContent)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	if result.Success {
		t.Error("Lint() should fail for oversized file")
	}

	if len(result.Issues) == 0 {
		t.Error("Lint() should return file size issue")
	}

	issue := result.Issues[0]
	if issue.Severity != "error" || issue.Rule != "file-size" {
		t.Errorf("Expected file-size error, got severity=%s rule=%s", issue.Severity, issue.Rule)
	}
}

func TestJavaScriptLinter_BasicSyntaxCheck(t *testing.T) {
	linter := NewJavaScriptLinter()

	tests := []struct {
		name        string
		content     string
		expectIssue bool
		issueType   string
	}{
		{
			name:        "Valid JavaScript",
			content:     "console.log('Hello, World!');",
			expectIssue: false,
		},
		{
			name:        "Unmatched braces",
			content:     "function test() { console.log('test');",
			expectIssue: true,
			issueType:   "basic-syntax",
		},
		{
			name:        "Function spacing style",
			content:     "function(x) { return x; }",
			expectIssue: true,
			issueType:   "basic-style",
		},
		{
			name:        "Valid function spacing",
			content:     "function (x) { return x; }",
			expectIssue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the basic syntax check method directly
			result, err := linter.basicSyntaxCheck("test.js", []byte(tt.content))
			if err != nil {
				t.Fatalf("basicSyntaxCheck() error = %v", err)
			}

			hasIssue := len(result.Issues) > 0
			if hasIssue != tt.expectIssue {
				t.Errorf("basicSyntaxCheck() issues = %v, expectIssue %v", hasIssue, tt.expectIssue)
			}

			if tt.expectIssue && len(result.Issues) > 0 {
				found := false
				for _, issue := range result.Issues {
					if issue.Rule == tt.issueType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected issue type %s not found in issues", tt.issueType)
				}
			}
		})
	}
}

func TestJavaScriptLinter_LintBatch(t *testing.T) {
	linter := NewJavaScriptLinter()

	files := map[string][]byte{
		"app.js":   []byte("console.log('Hello from JS');"),
		"app.ts":   []byte("const x: number = 42;"),
		"main.go":  []byte("package main"), // Should be ignored
		"test.jsx": []byte("const Button = () => <button>Click</button>;"),
	}

	results, err := linter.LintBatch(context.Background(), files)
	if err != nil {
		t.Fatalf("LintBatch() error = %v", err)
	}

	// Should only process JS/TS files
	expectedFiles := []string{"app.js", "app.ts", "test.jsx"}
	if len(results) != len(expectedFiles) {
		t.Errorf("LintBatch() processed %d files, want %d", len(results), len(expectedFiles))
	}

	for _, file := range expectedFiles {
		if _, exists := results[file]; !exists {
			t.Errorf("LintBatch() missing result for %s", file)
		}
	}

	// Should not process non-JS files
	if _, exists := results["main.go"]; exists {
		t.Error("LintBatch() should not process Go files")
	}
}

func TestJavaScriptLinter_NewJavaScriptLinterWithConfig(t *testing.T) {
	// Test with nil config
	linter1 := NewJavaScriptLinterWithConfig(nil)
	if linter1 == nil {
		t.Fatal("NewJavaScriptLinterWithConfig(nil) returned nil")
	}
	if linter1.config == nil {
		t.Error("NewJavaScriptLinterWithConfig(nil) should use default config")
	}

	// Test with custom config
	customConfig := &JavaScriptConfig{
		ForceTool:      stringPtr("eslint"),
		PreferredTools: []string{"eslint", "biome"},
	}
	linter2 := NewJavaScriptLinterWithConfig(customConfig)
	if linter2.config != customConfig {
		t.Error("NewJavaScriptLinterWithConfig() should use provided config")
	}
}

func TestDuration_JSON(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		json     string
		wantErr  bool
	}{
		{
			name:     "30 seconds",
			duration: Duration{30 * time.Second},
			json:     `"30s"`,
			wantErr:  false,
		},
		{
			name:     "2 minutes",
			duration: Duration{2 * time.Minute},
			json:     `"2m0s"`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" marshal", func(t *testing.T) {
			data, err := json.Marshal(tt.duration)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(data) != tt.json {
				t.Errorf("Marshal() = %s, want %s", string(data), tt.json)
			}
		})

		t.Run(tt.name+" unmarshal", func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tt.json), &d)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && d.Duration != tt.duration.Duration {
				t.Errorf("Unmarshal() = %v, want %v", d.Duration, tt.duration.Duration)
			}
		})
	}

	// Test invalid JSON unmarshaling
	invalidTests := []string{
		`"invalid"`,
		`null`,
		`[]`,
	}

	for _, invalidJSON := range invalidTests {
		t.Run("invalid "+invalidJSON, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(invalidJSON), &d)
			if err == nil {
				t.Error("Unmarshal() should fail for invalid input")
			}
		})
	}
}

func TestJavaScriptLinter_ParseOutput(t *testing.T) {
	linter := NewJavaScriptLinter()

	// Test Biome output parsing
	biomeOutput := `{
		"diagnostics": [
			{
				"category": "lint/suspicious/noDoubleEquals",
				"severity": "error",
				"message": {"text": "Use === instead of =="},
				"location": {
					"path": {"file": "test.js"},
					"span": {
						"start": {"line": 1, "column": 5},
						"end": {"line": 1, "column": 7}
					}
				}
			}
		]
	}`

	issues, err := linter.parseBiomeOutput([]byte(biomeOutput), "test.js")
	if err != nil {
		t.Fatalf("parseBiomeOutput() error = %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("parseBiomeOutput() returned %d issues, want 1", len(issues))
	}

	issue := issues[0]
	if issue.File != "test.js" || issue.Line != 1 || issue.Column != 5 {
		t.Errorf("parseBiomeOutput() issue location = %s:%d:%d, want test.js:1:5", issue.File, issue.Line, issue.Column)
	}
	if issue.Severity != "error" || issue.Rule != "lint/suspicious/noDoubleEquals" {
		t.Errorf("parseBiomeOutput() issue = %s:%s, want error:lint/suspicious/noDoubleEquals", issue.Severity, issue.Rule)
	}

	// Test ESLint output parsing
	eslintOutput := `[
		{
			"filePath": "test.js",
			"messages": [
				{
					"ruleId": "eqeqeq",
					"severity": 2,
					"message": "Expected '===' and instead saw '=='.",
					"line": 1,
					"column": 5
				}
			],
			"errorCount": 1,
			"warningCount": 0
		}
	]`

	issues, err = linter.parseESLintOutput([]byte(eslintOutput), "test.js")
	if err != nil {
		t.Fatalf("parseESLintOutput() error = %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("parseESLintOutput() returned %d issues, want 1", len(issues))
	}

	issue = issues[0]
	if issue.File != "test.js" || issue.Line != 1 || issue.Column != 5 {
		t.Errorf("parseESLintOutput() issue location = %s:%d:%d, want test.js:1:5", issue.File, issue.Line, issue.Column)
	}
	if issue.Severity != "error" || issue.Rule != "eqeqeq" {
		t.Errorf("parseESLintOutput() issue = %s:%s, want error:eqeqeq", issue.Severity, issue.Rule)
	}
}

// Helper function for tests
func stringPtr(s string) *string {
	return &s
}
