package golang

import (
	"context"
	"testing"

	"github.com/jrossi/gismo/pkg/linters"

	json "github.com/goccy/go-json"
)

func TestGoLinter_ImportRestrictions_NoConfig(t *testing.T) {
	linter := NewGoLinter()

	content := `package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	fmt.Println("test")
}
`

	result, err := linter.Lint(context.Background(), "test.go", []byte(content))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should not have any import restriction issues when no config is set
	for _, issue := range result.Issues {
		if issue.Rule == "import-restriction" {
			t.Errorf("Should not have import restriction issues without configuration, got: %v", issue)
		}
	}
}

func TestGoLinter_ImportRestrictions_BlockedImport(t *testing.T) {
	linter := NewGoLinter()

	// Configure import restrictions
	config := GolangConfig{
		ImportRestrictions: map[string]ImportRestriction{
			"encoding/json": {
				Blocked:     true,
				Replacement: "github.com/goccy/go-json",
				Reason:      "Use go-json for better performance",
			},
		},
	}

	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = linter.SetConfig(configData)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	content := `package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	data, _ := json.Marshal(map[string]string{"key": "value"})
	fmt.Println(string(data))
}
`

	result, err := linter.Lint(context.Background(), "test.go", []byte(content))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should fail due to blocked import
	if result.Success {
		t.Error("Expected linting to fail due to blocked import")
	}

	// Should have exactly one import restriction issue
	var importIssues []linters.Issue
	for _, issue := range result.Issues {
		if issue.Rule == "import-restriction" {
			importIssues = append(importIssues, issue)
		}
	}

	if len(importIssues) != 1 {
		t.Errorf("Expected 1 import restriction issue, got %d", len(importIssues))
	}

	if len(importIssues) > 0 {
		issue := importIssues[0]
		if issue.Severity != "error" {
			t.Errorf("Expected error severity, got %s", issue.Severity)
		}
		if issue.Line != 4 {
			t.Errorf("Expected line 4, got line %d", issue.Line)
		}
		expectedMessage := "Import 'encoding/json' is not allowed: Use go-json for better performance. Use 'github.com/goccy/go-json' instead"
		if issue.Message != expectedMessage {
			t.Errorf("Expected message '%s', got '%s'", expectedMessage, issue.Message)
		}
	}
}

func TestGoLinter_ImportRestrictions_AllowedImport(t *testing.T) {
	linter := NewGoLinter()

	// Configure import restrictions
	config := GolangConfig{
		ImportRestrictions: map[string]ImportRestriction{
			"encoding/json": {
				Blocked:     true,
				Replacement: "github.com/goccy/go-json",
				Reason:      "Use go-json for better performance",
			},
		},
	}

	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = linter.SetConfig(configData)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	content := `package main

import (
	"fmt"
	json "github.com/goccy/go-json"
)

func main() {
	data, _ := json.Marshal(map[string]string{"key": "value"})
	fmt.Println(string(data))
}
`

	result, err := linter.Lint(context.Background(), "test.go", []byte(content))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should not have any import restriction issues
	for _, issue := range result.Issues {
		if issue.Rule == "import-restriction" {
			t.Errorf("Should not have import restriction issues for allowed import, got: %v", issue)
		}
	}
}

func TestGoLinter_ImportRestrictions_MultipleImports(t *testing.T) {
	linter := NewGoLinter()

	// Configure multiple import restrictions
	config := GolangConfig{
		ImportRestrictions: map[string]ImportRestriction{
			"encoding/json": {
				Blocked:     true,
				Replacement: "github.com/goccy/go-json",
				Reason:      "Use go-json for better performance",
			},
			"io/ioutil": {
				Blocked:     true,
				Replacement: "io and os packages",
				Reason:      "deprecated since Go 1.16",
			},
		},
	}

	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = linter.SetConfig(configData)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	content := `package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

func main() {
	data, _ := json.Marshal(map[string]string{"key": "value"})
	content, _ := ioutil.ReadFile("test.txt")
	fmt.Println(string(data), string(content))
}
`

	result, err := linter.Lint(context.Background(), "test.go", []byte(content))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should fail due to blocked imports
	if result.Success {
		t.Error("Expected linting to fail due to blocked imports")
	}

	// Should have exactly two import restriction issues
	var importIssues []linters.Issue
	for _, issue := range result.Issues {
		if issue.Rule == "import-restriction" {
			importIssues = append(importIssues, issue)
		}
	}

	if len(importIssues) != 2 {
		t.Errorf("Expected 2 import restriction issues, got %d", len(importIssues))
	}

	// Check that both blocked imports are reported
	blockedImports := make(map[string]bool)
	for _, issue := range importIssues {
		if issue.Severity != "error" {
			t.Errorf("Expected error severity, got %s", issue.Severity)
		}
		// Extract import path from message
		if issue.Message == "Import 'encoding/json' is not allowed: Use go-json for better performance. Use 'github.com/goccy/go-json' instead" {
			blockedImports["encoding/json"] = true
		}
		if issue.Message == "Import 'io/ioutil' is not allowed: deprecated since Go 1.16. Use 'io and os packages' instead" {
			blockedImports["io/ioutil"] = true
		}
	}

	if !blockedImports["encoding/json"] {
		t.Error("Expected encoding/json to be reported as blocked")
	}
	if !blockedImports["io/ioutil"] {
		t.Error("Expected io/ioutil to be reported as blocked")
	}
}

func TestGoLinter_ImportRestrictions_BatchLinting(t *testing.T) {
	linter := NewGoLinter()

	// Configure import restrictions
	config := GolangConfig{
		ImportRestrictions: map[string]ImportRestriction{
			"encoding/json": {
				Blocked:     true,
				Replacement: "github.com/goccy/go-json",
				Reason:      "Use go-json for better performance",
			},
		},
	}

	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = linter.SetConfig(configData)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	files := map[string][]byte{
		"good.go": []byte(`package main

import (
	"fmt"
	json "github.com/goccy/go-json"
)

func main() {
	data, _ := json.Marshal(map[string]string{"key": "value"})
	fmt.Println(string(data))
}
`),
		"bad.go": []byte(`package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	data, _ := json.Marshal(map[string]string{"key": "value"})
	fmt.Println(string(data))
}
`),
	}

	results, err := linter.LintBatch(context.Background(), files)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// good.go should have no import restriction issues
	goodResult := results["good.go"]
	if goodResult == nil {
		t.Fatal("Expected result for good.go")
	}

	for _, issue := range goodResult.Issues {
		if issue.Rule == "import-restriction" {
			t.Errorf("good.go should not have import restriction issues, got: %v", issue)
		}
	}

	// bad.go should have import restriction issues
	badResult := results["bad.go"]
	if badResult == nil {
		t.Fatal("Expected result for bad.go")
	}

	if badResult.Success {
		t.Error("Expected bad.go to fail due to blocked import")
	}

	var importIssues []linters.Issue
	for _, issue := range badResult.Issues {
		if issue.Rule == "import-restriction" {
			importIssues = append(importIssues, issue)
		}
	}

	if len(importIssues) != 1 {
		t.Errorf("Expected 1 import restriction issue in bad.go, got %d", len(importIssues))
	}
}

func TestGoLinter_ImportRestrictions_SyntaxError(t *testing.T) {
	linter := NewGoLinter()

	// Configure import restrictions
	config := GolangConfig{
		ImportRestrictions: map[string]ImportRestriction{
			"encoding/json": {
				Blocked:     true,
				Replacement: "github.com/goccy/go-json",
			},
		},
	}

	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = linter.SetConfig(configData)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	// File with syntax error
	content := `package main

import (
	"encoding/json"
	"fmt"

func main() {  // Missing closing parenthesis in import block
	fmt.Println("test")
}
`

	result, err := linter.Lint(context.Background(), "test.go", []byte(content))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should fail due to syntax error, not import restrictions
	if result.Success {
		t.Error("Expected linting to fail due to syntax error")
	}

	// Should not have import restriction issues due to parse failure
	for _, issue := range result.Issues {
		if issue.Rule == "import-restriction" {
			t.Errorf("Should not check import restrictions when syntax is invalid, got: %v", issue)
		}
	}

	// Should have syntax error
	var syntaxErrors []linters.Issue
	for _, issue := range result.Issues {
		if issue.Rule == "syntax" {
			syntaxErrors = append(syntaxErrors, issue)
		}
	}

	if len(syntaxErrors) == 0 {
		t.Error("Expected syntax error to be reported")
	}
}

func TestImportRestriction_ConfigurationOptions(t *testing.T) {
	tests := []struct {
		name        string
		restriction ImportRestriction
		expectMsg   string
	}{
		{
			name: "blocked only",
			restriction: ImportRestriction{
				Blocked: true,
			},
			expectMsg: "Import 'test/package' is not allowed",
		},
		{
			name: "with reason",
			restriction: ImportRestriction{
				Blocked: true,
				Reason:  "deprecated",
			},
			expectMsg: "Import 'test/package' is not allowed: deprecated",
		},
		{
			name: "with replacement",
			restriction: ImportRestriction{
				Blocked:     true,
				Replacement: "new/package",
			},
			expectMsg: "Import 'test/package' is not allowed. Use 'new/package' instead",
		},
		{
			name: "with reason and replacement",
			restriction: ImportRestriction{
				Blocked:     true,
				Reason:      "better performance",
				Replacement: "new/package",
			},
			expectMsg: "Import 'test/package' is not allowed: better performance. Use 'new/package' instead",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linter := NewGoLinter()

			config := GolangConfig{
				ImportRestrictions: map[string]ImportRestriction{
					"test/package": tt.restriction,
				},
			}

			configData, err := json.Marshal(config)
			if err != nil {
				t.Fatalf("Failed to marshal config: %v", err)
			}

			err = linter.SetConfig(configData)
			if err != nil {
				t.Fatalf("Failed to set config: %v", err)
			}

			content := `package main

import (
	"test/package"
)

func main() {}
`

			result, err := linter.Lint(context.Background(), "test.go", []byte(content))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var importIssues []linters.Issue
			for _, issue := range result.Issues {
				if issue.Rule == "import-restriction" {
					importIssues = append(importIssues, issue)
				}
			}

			if len(importIssues) != 1 {
				t.Fatalf("Expected 1 import restriction issue, got %d", len(importIssues))
			}

			if importIssues[0].Message != tt.expectMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.expectMsg, importIssues[0].Message)
			}
		})
	}
}
