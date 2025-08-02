package golang

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoLinter_runTests_SelectiveExecution tests that when we have tests with different prefixes,
// only tests matching the pattern are executed
func TestGoLinter_runTests_SelectiveExecution(t *testing.T) {
	linter := NewGoLinter()

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "go_test_selective_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	goModContent := `module testmodule

go 1.21`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create a test file with tests from different features
	// In a real scenario, this would be split into different files
	testContent := `package main

import "testing"

func TestAPI_Create(t *testing.T) {
	t.Log("api create test")
}

func TestAPI_Delete(t *testing.T) {
	t.Log("api delete test")
}

func TestHandler_Process(t *testing.T) {
	t.Log("handler process test")
}

func TestHandler_Validate(t *testing.T) {
	t.Log("handler validate test")
}`

	// Create multiple test files to demonstrate the pattern matching
	apiTestFile := filepath.Join(tmpDir, "api_test.go")
	if err := os.WriteFile(apiTestFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Now create a separate handler test file
	handlerContent := `package main

import "testing"

func TestHandler_Init(t *testing.T) {
	t.Log("handler init test")
}

func TestHandler_Close(t *testing.T) {
	t.Log("handler close test")
}`

	handlerTestFile := filepath.Join(tmpDir, "handler_test.go")
	if err := os.WriteFile(handlerTestFile, []byte(handlerContent), 0644); err != nil {
		t.Fatalf("Failed to create handler test file: %v", err)
	}

	// Test 1: Run tests from api_test.go - should include both API and Handler tests from that file
	t.Run("api_test_file", func(t *testing.T) {
		output, err := linter.runTests(context.Background(), apiTestFile)
		if err != nil {
			t.Logf("runTests returned error: %v", err)
		}

		// The api_test.go file contains both API and Handler tests
		// With extraction, it will run ALL tests from the file
		if !strings.Contains(output, "TestAPI_Create") {
			t.Errorf("Should run TestAPI_Create from api_test.go")
		}
		if !strings.Contains(output, "TestAPI_Delete") {
			t.Errorf("Should run TestAPI_Delete from api_test.go")
		}
		if !strings.Contains(output, "TestHandler_Process") {
			t.Errorf("Should run TestHandler_Process from api_test.go")
		}
		if !strings.Contains(output, "TestHandler_Validate") {
			t.Errorf("Should run TestHandler_Validate from api_test.go")
		}
	})

	// Test 2: Run tests from handler_test.go - should only include Handler tests from that file
	t.Run("handler_test_file", func(t *testing.T) {
		output, err := linter.runTests(context.Background(), handlerTestFile)
		if err != nil {
			t.Logf("runTests returned error: %v", err)
		}

		// Should run Handler tests from handler_test.go
		if !strings.Contains(output, "TestHandler_Init") {
			t.Errorf("Should run TestHandler_Init from handler_test.go")
		}
		if !strings.Contains(output, "TestHandler_Close") {
			t.Errorf("Should run TestHandler_Close from handler_test.go")
		}

		// Should NOT run API tests (they're in a different file)
		if strings.Contains(output, "TestAPI_") {
			t.Errorf("Should NOT run API tests when testing handler_test.go")
		}
	})
}

// TestGoLinter_runTests_CommonPrefixOptimization tests that when tests share a common prefix,
// the pattern is optimized to use that prefix
func TestGoLinter_runTests_CommonPrefixOptimization(t *testing.T) {
	linter := NewGoLinter()

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "go_test_prefix_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	goModContent := `module testmodule

go 1.21`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create a test file with tests that share a common prefix
	testContent := `package main

import "testing"

func TestDatabaseConnection_Open(t *testing.T) {
	t.Log("db open test")
}

func TestDatabaseConnection_Close(t *testing.T) {
	t.Log("db close test")
}

func TestDatabaseConnection_Query(t *testing.T) {
	t.Log("db query test")
}`

	testFile := filepath.Join(tmpDir, "database_test.go")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Extract tests to verify the common prefix
	tests, err := linter.extractTestFunctions(testFile)
	if err != nil {
		t.Fatalf("Failed to extract tests: %v", err)
	}

	// Check common prefix
	prefix := findCommonPrefix(tests)
	if prefix != "TestDatabaseConnection_" {
		t.Errorf("Expected common prefix 'TestDatabaseConnection_', got %q", prefix)
	}

	// Run tests and verify they all execute
	output, err := linter.runTests(context.Background(), testFile)
	if err != nil {
		t.Logf("runTests returned error: %v", err)
	}

	// All tests should run with the optimized pattern
	for _, test := range tests {
		if !strings.Contains(output, test) {
			t.Errorf("Test %s should have been executed", test)
		}
	}
}
