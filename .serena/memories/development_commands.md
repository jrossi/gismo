# Gismo Development Commands

## Build System (Mage)
The project uses mage as its build tool. All commands use `go tool mage -v`:

### Essential Commands
```bash
go tool mage -v all        # Format, lint, test, and build everything
go tool mage -v build      # Build all binaries (smart dependency tracking)
go tool mage -v test       # Run all tests with race detection and coverage
go tool mage -v fmt        # Format code with gofmt
go tool mage -v lint       # Run golangci-lint
go tool mage -v install    # Install binaries to GOPATH
go tool mage -v clean      # Clean build artifacts
go tool mage -v deps       # Download and tidy dependencies
go tool mage -v coverage   # Generate HTML coverage report
go tool mage -v check      # Run all quality checks (fmt, lint, test)
go tool mage -v info       # Show build information
go tool mage -l            # List all available targets
```

### Build Artifacts
Build outputs are organized in `./build/`:
- `./build/bin/` - Compiled binaries
- `./build/coverage/` - Coverage reports
- `./build/dist/` - Distribution artifacts

### Testing Commands
```bash
# All tests
go tool mage -v test

# Specific test types
go test ./pkg/...                    # Unit tests only (fast)
go test ./tests/integration/...      # Integration tests
go test ./tests/e2e/...             # End-to-end tests (slow)
go test -short ./...                # Skip slow tests
```

### Database Tests
```bash
go test ./pkg/database/...           # Database package tests
go test ./pkg/database/project/...   # Project management tests
go test ./pkg/database/search/...    # Search engine tests
```

## Development Workflow
1. `go tool mage -v fmt` - Format code
2. `go tool mage -v lint` - Check for linting issues  
3. `go tool mage -v test` - Run full test suite
4. `go tool mage -v check` - Run all quality checks
5. `go tool mage -v build` - Build binaries

## Critical Success Criteria
- **ALL** linting must pass with ZERO warnings
- **ALL** tests must pass (exit code 0)
- **ALL** formatting must be correct (gofmt makes no changes)
- **NO** forbidden patterns (interface{}, time.Sleep, etc.)
- **ALL** hooks must show ✅ GREEN status