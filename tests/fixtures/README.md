# Test Fixtures

This directory contains test fixtures used across unit, integration, and end-to-end tests.

## Structure

```text
testdata/
├── markdown/
│   ├── good.md          # Well-formatted markdown
│   ├── bad_trailing.md  # Contains trailing whitespace
│   ├── bad_headings.md  # Skipped heading levels
│   ├── bad_mixed.md     # Multiple formatting issues
│   └── large.md         # Large file for performance testing
└── golang/
    ├── good.go          # Well-formatted Go code
    ├── bad_interface.go # Uses forbidden interface{}/any
    ├── bad_sleep.go     # Uses forbidden time.Sleep()
    ├── bad_mixed.go     # Multiple issues (interface{}, sleep, panic, TODO)
    └── large.go         # Large file for performance testing
```

## Usage

Test files should load these fixtures using `os.ReadFile()` instead of embedding test data inline:

```go
content, err := os.ReadFile("testdata/markdown/good.md")
if err != nil {
    t.Fatal(err)
}
```

## Purpose

These fixtures ensure consistent test data across:
- Unit tests (in `linters/*/`)
- Integration tests (in `integration_test/`)
- End-to-end tests (in `e2e_test/`)