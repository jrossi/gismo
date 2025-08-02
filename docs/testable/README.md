# Documentation Testing Framework

This directory contains a comprehensive testing framework for validating documentation examples and ensuring they stay in sync with the actual codebase.

## Problem Solved

The documentation testing framework addresses several critical issues:

1. **Out-of-date Examples**: Documentation examples often become stale and contain outdated API calls
2. **Broken Code**: Examples in documentation may not compile or work as expected
3. **Import Path Mismatches**: Documentation may reference non-existent import paths
4. **API Drift**: As the codebase evolves, documentation examples lag behind

## Framework Components

### Core Framework (`doc_test_framework.go`)

The main testing framework provides:

- **Code Extraction**: Automatically extracts Go code blocks from Markdown files
- **Compilation Testing**: Validates that code examples compile successfully
- **API Validation**: Checks that import paths and function calls are correct
- **Import Verification**: Ensures all imports reference actual packages

### Test Suites

#### `library_examples_test.go`
- Tests all examples from the library documentation
- Validates API usage patterns
- Checks message structure correctness

#### `documentation_validation_test.go`
- Demonstrates framework capabilities
- Shows how outdated documentation is detected
- Provides examples of correct vs incorrect documentation

## Usage

### Running Documentation Tests

```bash
# Run all documentation tests
make test-docs

# Run specific test suites
go test -v ./docs/testable/... -run TestDocumentationValidation
go test -v ./docs/testable/... -run TestWorkingExamples
```

### Adding New Documentation Tests

1. **Create Test File**: Add a new `*_test.go` file for your documentation section
2. **Extract Examples**: Use `LoadExamplesFromMarkdown()` to extract code blocks
3. **Validate Examples**: Call `TestAllExamples()` or `TestExample()` for specific examples

Example:

```go
func TestMyDocumentationSection(t *testing.T) {
    dt := NewDocumentationTester(t)
    defer dt.Cleanup()

    // Load examples from your markdown file
    err := dt.LoadExamplesFromMarkdown("../content/docs/my-section/_index.md")
    if err != nil {
        t.Fatalf("Failed to load examples: %v", err)
    }

    // Test all examples
    if err := dt.TestAllExamples(); err != nil {
        t.Errorf("Documentation validation failed: %v", err)
    }
}
```

## CI Integration

The documentation tests are integrated into the CI pipeline via the Makefile:

- **`make all`**: Now includes `test-docs` target
- **`make test-docs`**: Runs documentation-specific tests
- **Automated Detection**: CI will fail if documentation examples are broken

## Framework Features

### Automatic Code Preparation

The framework automatically adds necessary boilerplate to make documentation examples compilable:

```go
// Original documentation example:
api := engine.New()

// Framework prepares it as:
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    json "github.com/goccy/go-json"
    "github.com/jrossi/gismo/pkg/engine"
    "github.com/jrossi/gismo/pkg/handlers"
)

func main() {
    api := engine.New()
}
```

### Import Path Validation

The framework validates that documentation uses correct import paths:

- ✅ **Valid**: `"github.com/jrossi/gismo/pkg/engine"`
- ❌ **Invalid**: `"github.com/jrossi/gismo"` (doesn't exist at root)

### API Function Mapping

The framework maps deprecated documentation functions to current API:

```go
// Documentation shows:     // Should be:
gismo.NewAPI()          →   engine.New()
gismo.LoadConfig()      →   loader.LoadConfig()
gismo.ProcessHook()     →   api.ProcessMessage()
```

## Example Output

When documentation is out of date, the framework provides clear feedback:

```
=== RUN   TestDocumentationValidation/DetectInvalidImports
✅ EXPECTED: Found invalid imports in documentation:
example http_server_373 contains invalid import "github.com/jrossi/gismo"

=== RUN   TestDocumentationValidation/DetectInvalidAPIFunctions
✅ EXPECTED: Found invalid API functions in documentation:
example basic_usage_24 uses deprecated gismo.NewAPI(), should use engine.New()
example api_creation_96 uses deprecated gismo.NewAPI(), should use engine.New()
```

## Best Practices

### Writing Testable Documentation

1. **Complete Examples**: Provide full, runnable code examples
2. **Correct Imports**: Use actual import paths from the codebase
3. **Real API Calls**: Use current API functions, not deprecated ones
4. **Compilable Code**: Ensure examples can be compiled without errors

### Maintaining Documentation

1. **Run Tests Regularly**: Use `make test-docs` during development
2. **Fix Issues Immediately**: When tests fail, update documentation promptly
3. **Add Tests for New Docs**: Create tests for new documentation sections
4. **Review CI Failures**: Documentation test failures block builds

## Architecture Benefits

### Automatic Validation
- Documentation examples are validated on every CI run
- Broken examples are caught before they reach users
- API changes automatically flag documentation that needs updates

### Developer Experience
- Clear error messages when documentation is out of date
- Automated detection of common documentation issues
- Framework handles boilerplate code generation

### Maintenance Efficiency
- Reduces manual documentation review burden
- Catches regressions in documentation quality
- Ensures examples stay current with codebase evolution

## Future Enhancements

The framework can be extended to support:

- **Multiple Languages**: Testing examples in other languages
- **Integration Testing**: Running examples in containers
- **Performance Testing**: Benchmarking documentation examples
- **Link Validation**: Checking that referenced links work
- **Schema Validation**: Validating JSON/YAML examples against schemas

## Related Files

- `../../content/docs/library/_index.md`: Updated library documentation with correct examples
- `../../../Makefile`: CI integration with `test-docs` target
- `../../../examples/`: Working examples that complement documentation