# Test Organization

This directory contains all higher-level tests for the Gismo project. Unit tests remain co-located with their source files in the `pkg/` directory structure.

## Directory Structure

```
tests/
├── README.md                 # This documentation
├── integration/              # Integration tests (cross-package testing)
├── e2e/                      # End-to-end tests (full binary testing)
├── fixtures/                 # Shared test data and fixtures
├── examples/                 # Example test utilities and sample configs
└── utils/                    # Test utility functions
```

## Test Types

### Unit Tests
**Location**: `pkg/*/` directories (co-located with source code)
- Test individual functions and methods in isolation
- Fast execution, no external dependencies
- Examples: `pkg/engine/parser_test.go`, `pkg/linters/golang/golang_test.go`

### Integration Tests
**Location**: `tests/integration/`
- Test interaction between multiple packages
- Test the engine with different linters and handlers
- Validate cross-package functionality
- Examples: `golang_engine_test.go`, `markdown_engine_test.go`

### End-to-End Tests
**Location**: `tests/e2e/`
- Test the complete binary with real input/output
- Build and execute the actual `gismo` binary
- Test CLI flags, error handling, and full workflows
- Examples: `golang_e2e_test.go`, `markdown_e2e_test.go`

### Test Fixtures
**Location**: `tests/fixtures/`
- Shared test data used across multiple test types
- Sample code files with known good/bad patterns
- Organized by language/type: `golang/`, `markdown/`

### Example Tests
**Location**: `tests/examples/`
- Test utilities for validating example configurations
- Sample registry and manifest validation
- Utilities for testing external package functionality

### Test Utils
**Location**: `tests/utils/`
- Shared helper functions and utilities
- Common test setup and teardown functions
- Namespace extraction and URL parsing utilities

## Running Tests

### All Tests
```bash
make test
```

### Specific Test Types
```bash
# Unit tests only (fast)
go test ./pkg/...

# Integration tests
go test ./tests/integration/...

# End-to-end tests (slow)
go test ./tests/e2e/...

# Example validations
go test ./tests/examples/...
```

### Test Coverage
```bash
make coverage
```

## Best Practices

1. **Unit tests** should be fast and test one thing well
2. **Integration tests** should validate cross-package interactions
3. **E2E tests** should test real user scenarios but be kept minimal
4. **Fixtures** should be small and focused on specific test cases
5. **Test utilities** should be reusable across test types

## Adding New Tests

- **Unit tests**: Add `*_test.go` files next to source code in `pkg/`
- **Integration tests**: Add to `tests/integration/` for cross-package testing
- **E2E tests**: Add to `tests/e2e/` for full binary testing scenarios
- **Fixtures**: Add test data to `tests/fixtures/` organized by type
- **Examples**: Add example validations to `tests/examples/`

## Performance Considerations

- Unit tests run in CI on every commit (keep them fast)
- Integration tests run in CI but may be slower
- E2E tests are the slowest and build actual binaries
- Use `go test -short` to skip slow tests during development