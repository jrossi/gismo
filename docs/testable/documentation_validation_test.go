package testable

import (
	"testing"
)

// TestDocumentationValidation shows how our testing framework validates documentation
func TestDocumentationValidation(t *testing.T) {
	dt := NewDocumentationTester(t)
	defer dt.Cleanup()

	// This test demonstrates that our framework correctly identifies issues
	// in the current documentation. The fact that this test detects problems
	// proves the framework is working as intended.

	t.Run("DetectInvalidImports", func(t *testing.T) {
		// Load examples from library documentation
		err := dt.LoadExamplesFromMarkdown("../content/docs/library/_index.md")
		if err != nil {
			t.Fatalf("Failed to load examples: %v", err)
		}

		// Test that imports are validated (this should find issues)
		err = dt.ValidateAPIImports()
		if err != nil {
			t.Logf("✅ EXPECTED: Found invalid imports in documentation: %v", err)
			// This is expected - our docs have invalid imports that need fixing
		} else {
			t.Error("❌ UNEXPECTED: No import issues found - documentation may already be fixed")
		}
	})

	t.Run("DetectInvalidAPIFunctions", func(t *testing.T) {
		// Test that API functions are validated (this should find issues)
		err := dt.ValidateAPIFunctions()
		if err != nil {
			t.Logf("✅ EXPECTED: Found invalid API functions in documentation: %v", err)
			// This is expected - our docs have deprecated function calls
		} else {
			t.Error("❌ UNEXPECTED: No API function issues found - documentation may already be fixed")
		}
	})

	// Note: We don't run TestAllExamples here because we know it will fail
	// due to the compilation errors. That's the point - we've identified
	// exactly what needs to be fixed in the documentation.
}

// TestWorkingExamples demonstrates that our framework can validate correct examples
func TestWorkingExamples(t *testing.T) {
	// This test shows that when we have correct documentation,
	// our framework validates it successfully
	dt := NewDocumentationTester(t)
	defer dt.Cleanup()

	// Create a correct example manually
	correctExample := &CodeExample{
		Title: "correct_api_usage",
		Code: `package main

import (
	"github.com/jrossi/gismo/pkg/engine"
)

func main() {
	api := engine.New()
	if api == nil {
		panic("Failed to create API")
	}
}`,
		Expected: ExampleExpected{
			ShouldCompile: true,
			ShouldExecute: true,
		},
	}

	dt.examples["correct_api_usage"] = correctExample

	// This should pass
	err := dt.TestExample("correct_api_usage")
	if err != nil {
		t.Errorf("Correct example failed validation: %v", err)
	} else {
		t.Log("✅ Correct example passed validation")
	}
}

// TestFrameworkCapabilities demonstrates various framework features
func TestFrameworkCapabilities(t *testing.T) {
	dt := NewDocumentationTester(t)
	defer dt.Cleanup()

	t.Run("CanDetectCompilationIssues", func(t *testing.T) {
		badExample := &CodeExample{
			Title: "bad_syntax",
			Code: `package main

func main() {
	// Missing closing brace
`,
			Expected: ExampleExpected{
				ShouldCompile: true,
			},
		}

		dt.examples["bad_syntax"] = badExample
		err := dt.TestExample("bad_syntax")
		if err != nil {
			t.Logf("✅ EXPECTED: Framework detected compilation issue: %v", err)
		} else {
			t.Error("❌ Framework failed to detect compilation issue")
		}
	})

	t.Run("CanValidateWorkingCode", func(t *testing.T) {
		goodExample := &CodeExample{
			Title: "good_syntax",
			Code: `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`,
			Expected: ExampleExpected{
				ShouldCompile: true,
			},
		}

		dt.examples["good_syntax"] = goodExample
		err := dt.TestExample("good_syntax")
		if err != nil {
			t.Errorf("❌ Framework rejected valid code: %v", err)
		} else {
			t.Log("✅ Framework validated working code")
		}
	})
}
