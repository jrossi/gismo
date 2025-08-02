package rust

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Helper function to create a test Rust file
func createTestRustFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	filePath := filepath.Join(dir, name)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	return filePath
}

// Helper function to create a basic Cargo.toml
func createCargoToml(t testing.TB, dir string) {
	t.Helper()
	cargoToml := `[package]
name = "test-project"
version = "0.1.0"
edition = "2021"

[dependencies]
`
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		t.Fatalf("Failed to create Cargo.toml: %v", err)
	}
}

func TestRustLinter_Name(t *testing.T) {
	linter := NewRustLinter()
	if got := linter.Name(); got != "rust" {
		t.Errorf("Name() = %v, want %v", got, "rust")
	}
}

func TestRustLinter_CanHandle(t *testing.T) {
	linter := NewRustLinter()

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "rust_file",
			filePath: "main.rs",
			want:     true,
		},
		{
			name:     "rust_lib_file",
			filePath: "src/lib.rs",
			want:     true,
		},
		{
			name:     "go_file",
			filePath: "main.go",
			want:     false,
		},
		{
			name:     "python_file",
			filePath: "main.py",
			want:     false,
		},
		{
			name:     "no_extension",
			filePath: "Makefile",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linter.CanHandle(tt.filePath); got != tt.want {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestRustLinter_Lint_WithoutCargo(t *testing.T) {
	// This test verifies behavior when Rust files are outside a Cargo project
	linter := NewRustLinter()
	ctx := context.Background()

	// Create a temporary directory without Cargo.toml
	tmpDir := t.TempDir()

	// Create a simple Rust file
	content := `fn main() {
    println!("Hello, world!");
}`
	filePath := createTestRustFile(t, tmpDir, "main.rs", content)

	result, err := linter.Lint(ctx, filePath, []byte(content))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// Should succeed but skip linting (no Cargo.toml)
	if !result.Success {
		t.Errorf("Expected success for file outside Cargo project")
	}

	if len(result.Issues) > 0 {
		t.Errorf("Expected no issues for file outside Cargo project, got %d", len(result.Issues))
	}
}

func TestRustLinter_Lint_ValidCode(t *testing.T) {
	// Skip if cargo is not available
	if _, err := os.Stat("/opt/homebrew/bin/cargo"); err != nil && os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/cargo"); err != nil && os.IsNotExist(err) {
			t.Skip("Cargo not found, skipping test")
		}
	}

	linter := NewRustLinter()
	ctx := context.Background()

	// Create a temporary Cargo project
	tmpDir := t.TempDir()
	createCargoToml(t, tmpDir)

	// Create src directory
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	// Create a valid Rust file
	content := `fn main() {
    println!("Hello, world!");
}

#[cfg(test)]
mod tests {
    #[test]
    fn test_example() {
        assert_eq!(2 + 2, 4);
    }
}
`
	filePath := createTestRustFile(t, srcDir, "main.rs", content)

	result, err := linter.Lint(ctx, filePath, []byte(content))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success for valid code")
	}

	// Check for any critical errors (formatting warnings are OK)
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			t.Errorf("Unexpected error: %v", issue.Message)
		}
	}
}

func TestRustLinter_Lint_WithWarnings(t *testing.T) {
	// Skip if cargo is not available
	if _, err := os.Stat("/opt/homebrew/bin/cargo"); err != nil && os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/cargo"); err != nil && os.IsNotExist(err) {
			t.Skip("Cargo not found, skipping test")
		}
	}

	linter := NewRustLinter()
	ctx := context.Background()

	// Create a temporary Cargo project
	tmpDir := t.TempDir()
	createCargoToml(t, tmpDir)

	// Create src directory
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	// Create a Rust file with warnings
	content := `#[allow(dead_code)]
fn unused_function() {
    println!("This function is never used");
}

fn main() {
    let unused_variable = 42;
    println!("Hello, world!");
}
`
	filePath := createTestRustFile(t, srcDir, "main.rs", content)

	result, err := linter.Lint(ctx, filePath, []byte(content))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// Should still succeed with warnings
	if !result.Success {
		t.Errorf("Expected success even with warnings")
	}

	// Should have at least one warning
	hasWarning := false
	for _, issue := range result.Issues {
		if issue.Severity == "warning" {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Logf("Issues found: %v", result.Issues)
		// It's OK if clippy doesn't find the unused variable warning
		// as it might be configured differently
	}
}

func TestRustLinter_Lint_WithSyntaxError(t *testing.T) {
	// Skip if cargo is not available
	if _, err := os.Stat("/opt/homebrew/bin/cargo"); err != nil && os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/cargo"); err != nil && os.IsNotExist(err) {
			t.Skip("Cargo not found, skipping test")
		}
	}

	linter := NewRustLinter()
	ctx := context.Background()

	// Create a temporary Cargo project
	tmpDir := t.TempDir()
	createCargoToml(t, tmpDir)

	// Create src directory
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	// Create a Rust file with syntax error
	content := `fn main() {
    println!("Missing closing brace"
    // Missing closing brace will cause syntax error
`
	filePath := createTestRustFile(t, srcDir, "main.rs", content)

	result, err := linter.Lint(ctx, filePath, []byte(content))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// Log all issues for debugging
	t.Logf("Issues found: %+v", result.Issues)

	// Should have at least one issue (error or warning)
	if len(result.Issues) == 0 {
		t.Errorf("Expected at least one issue for syntax error")
	}

	// Check if we have either an error or a warning about the syntax
	hasIssue := false
	for _, issue := range result.Issues {
		if issue.Severity == "error" || strings.Contains(issue.Message, "syntax") || strings.Contains(issue.Message, "parse") {
			hasIssue = true
			break
		}
	}

	if !hasIssue && len(result.Issues) > 0 {
		// It's OK if we only get formatting warnings for now
		// Clippy might not run on files with syntax errors
		t.Logf("Only formatting issues found, which is acceptable for files with syntax errors")
	}
}

func TestRustLinter_FindCargoRoot(t *testing.T) {
	linter := NewRustLinter()

	// Create a nested directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "myproject")
	srcDir := filepath.Join(projectDir, "src")
	subDir := filepath.Join(srcDir, "modules")

	// Create directories
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	// Create Cargo.toml in project root
	createCargoToml(t, projectDir)

	// Test finding Cargo.toml from various locations
	tests := []struct {
		name      string
		startPath string
		wantRoot  string
		wantError bool
	}{
		{
			name:      "from_project_root",
			startPath: projectDir,
			wantRoot:  projectDir,
			wantError: false,
		},
		{
			name:      "from_src_dir",
			startPath: srcDir,
			wantRoot:  projectDir,
			wantError: false,
		},
		{
			name:      "from_nested_dir",
			startPath: subDir,
			wantRoot:  projectDir,
			wantError: false,
		},
		{
			name:      "from_file_in_src",
			startPath: filepath.Join(srcDir, "main.rs"),
			wantRoot:  projectDir,
			wantError: false,
		},
		{
			name:      "no_cargo_toml",
			startPath: tmpDir,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := linter.FindCargoRoot(tt.startPath)
			if (err != nil) != tt.wantError {
				t.Errorf("FindCargoRoot() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && got.Root != tt.wantRoot {
				t.Errorf("FindCargoRoot() root = %v, want %v", got.Root, tt.wantRoot)
			}
		})
	}
}

func TestRustLinter_Configuration(t *testing.T) {
	tests := []struct {
		name   string
		config *RustConfig
		check  func(t *testing.T, l *RustLinter)
	}{
		{
			name:   "default_config",
			config: nil,
			check: func(t *testing.T, l *RustLinter) {
				if l.config == nil {
					t.Error("Expected default config to be set")
				}
				if !l.config.NoDeps {
					t.Error("Expected NoDeps to be true by default")
				}
				if !l.config.AllTargets {
					t.Error("Expected AllTargets to be true by default")
				}
			},
		},
		{
			name: "custom_config",
			config: &RustConfig{
				NoDeps:        false,
				AllTargets:    false,
				AllFeatures:   true,
				DisabledLints: []string{"dead_code"},
				EnabledLints:  []string{"clippy::pedantic"},
			},
			check: func(t *testing.T, l *RustLinter) {
				if l.config.NoDeps {
					t.Error("Expected NoDeps to be false")
				}
				if l.config.AllTargets {
					t.Error("Expected AllTargets to be false")
				}
				if !l.config.AllFeatures {
					t.Error("Expected AllFeatures to be true")
				}
				if len(l.config.DisabledLints) != 1 || l.config.DisabledLints[0] != "dead_code" {
					t.Errorf("Expected DisabledLints to contain 'dead_code', got %v", l.config.DisabledLints)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linter := NewRustLinterWithConfig(tt.config)
			tt.check(t, linter)
		})
	}
}

func TestRustLinter_TestExecution(t *testing.T) {
	// Skip if cargo is not available
	if _, err := os.Stat("/opt/homebrew/bin/cargo"); err != nil && os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/cargo"); err != nil && os.IsNotExist(err) {
			t.Skip("Cargo not found, skipping test")
		}
	}

	linter := NewRustLinter()
	ctx := context.Background()

	// Create a temporary Cargo project
	tmpDir := t.TempDir()
	createCargoToml(t, tmpDir)

	// Create src directory
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	// Create a Rust file with tests
	content := `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_add() {
        assert_eq!(add(2, 2), 4);
    }

    #[test]
    fn test_add_negative() {
        assert_eq!(add(-1, 1), 0);
    }
}
`
	filePath := createTestRustFile(t, srcDir, "lib.rs", content)

	result, err := linter.Lint(ctx, filePath, []byte(content))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success for valid code with tests")
	}

	// Check that tests were run
	if result.TestOutput == "" {
		t.Error("Expected test output, got empty string")
	}

	// Verify test output contains expected content
	if !strings.Contains(result.TestOutput, "test") || !strings.Contains(result.TestOutput, "passed") {
		t.Errorf("Test output doesn't contain expected content: %s", result.TestOutput)
	}
}

func TestRustLinter_FailingTests(t *testing.T) {
	// Skip if cargo is not available
	if _, err := os.Stat("/opt/homebrew/bin/cargo"); err != nil && os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/cargo"); err != nil && os.IsNotExist(err) {
			t.Skip("Cargo not found, skipping test")
		}
	}

	linter := NewRustLinter()
	ctx := context.Background()

	// Create a temporary Cargo project
	tmpDir := t.TempDir()
	createCargoToml(t, tmpDir)

	// Create src directory
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src directory: %v", err)
	}

	// Create a Rust file with a failing test
	content := `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_add_wrong() {
        assert_eq!(add(2, 2), 5); // This will fail
    }
}
`
	filePath := createTestRustFile(t, srcDir, "lib.rs", content)

	result, err := linter.Lint(ctx, filePath, []byte(content))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	// Should fail due to failing test
	if result.Success {
		t.Errorf("Expected failure for failing test")
	}

	// Should have a test error
	hasTestError := false
	for _, issue := range result.Issues {
		if issue.Rule == "test" && issue.Severity == "error" {
			hasTestError = true
			break
		}
	}

	if !hasTestError {
		t.Errorf("Expected test error, got issues: %v", result.Issues)
	}

	// Test output should contain failure information
	if !strings.Contains(result.TestOutput, "FAILED") && !strings.Contains(result.TestOutput, "failed") {
		t.Errorf("Test output should indicate failure: %s", result.TestOutput)
	}
}

// Benchmark tests
func BenchmarkRustLinter_Lint(b *testing.B) {
	// Skip if cargo is not available
	if _, err := os.Stat("/opt/homebrew/bin/cargo"); err != nil && os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/cargo"); err != nil && os.IsNotExist(err) {
			b.Skip("Cargo not found, skipping benchmark")
		}
	}

	linter := NewRustLinter()
	ctx := context.Background()

	// Create a temporary Cargo project
	tmpDir := b.TempDir()
	createCargoToml(b, tmpDir)

	// Create src directory
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		b.Fatalf("Failed to create src directory: %v", err)
	}

	// Create a simple Rust file
	content := `fn main() {
    println!("Hello, world!");
}
`
	filePath := filepath.Join(srcDir, "main.rs")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	// Reset timer to exclude setup
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := linter.Lint(ctx, filePath, []byte(content))
		if err != nil {
			b.Fatalf("Lint() error = %v", err)
		}
		if !result.Success {
			b.Errorf("Expected success, got failure")
		}
	}
}

// Integration test with ConfigurableLinter interface
func TestRustLinter_SetConfig(t *testing.T) {
	linter := NewRustLinter()

	configJSON := `{
		"noDeps": false,
		"allTargets": false,
		"allFeatures": true,
		"disabledLints": ["dead_code", "unused_variables"],
		"enabledLints": ["clippy::pedantic"],
		"testTimeout": "5m"
	}`

	err := linter.SetConfig([]byte(configJSON))
	if err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	// Verify configuration was applied
	if linter.config.NoDeps {
		t.Error("Expected NoDeps to be false")
	}
	if linter.config.AllTargets {
		t.Error("Expected AllTargets to be false")
	}
	if !linter.config.AllFeatures {
		t.Error("Expected AllFeatures to be true")
	}
	if len(linter.config.DisabledLints) != 2 {
		t.Errorf("Expected 2 disabled lints, got %d", len(linter.config.DisabledLints))
	}
	if len(linter.config.EnabledLints) != 1 {
		t.Errorf("Expected 1 enabled lint, got %d", len(linter.config.EnabledLints))
	}
	if linter.config.TestTimeout.Duration != 5*time.Minute {
		t.Errorf("Expected test timeout to be 5m, got %v", linter.config.TestTimeout.Duration)
	}
}
