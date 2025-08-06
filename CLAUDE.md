# Gismo Development Guide

This is a Go project implementing an extensible Claude Code hooks system with security-first action handlers,
comprehensive linting capabilities, and intelligent knowledge management using DuckDB for cross-platform code
search and documentation.

## ⚠️ CRITICAL: Never Commit Build Artifacts

**NEVER commit anything from the `build/` directory!** This includes:
- Binary executables (`build/bin/*`)
- Coverage reports (`build/coverage/*`)
- Distribution files (`build/dist/*`)
- Any other build artifacts

The `build/` directory is in `.gitignore` and MUST remain excluded from version control.
**Always check `git status` before committing to ensure no binaries are included.**

## Project Structure

```text
cmd/                    # CLI applications
├── gismo/             # Main binary (hook processor)
├── gismo-server/      # Server binary with gRPC API
├── gismo-knowledge/   # Knowledge base management
├── gismo-query/       # SQL query interface for knowledge DB
├── gismo-init/        # Setup tool (planned)
├── gismo-show/        # Configuration inspector (planned)
├── gismo-registry/    # Package registry manager (planned)
└── gismo-package/     # Package manager (planned)

pkg/                   # Core library code
├── engine/           # Rule engines and core logic
├── linters/          # Language-specific linters
├── handlers/         # Action handlers for hooks
├── database/         # DuckDB-based knowledge management
├── knowledge/        # Knowledge system core logic
├── server/           # gRPC server implementation
├── client/           # gRPC client implementation
├── toolcache/        # Caching utilities
└── version/          # Version parsing

tests/                # Centralized test organization
├── integration/      # Cross-package integration tests
├── e2e/             # End-to-end binary tests
├── fixtures/        # Shared test data
├── examples/        # Example test utilities
└── utils/           # Test utility functions

examples/            # Example configurations and demos
docs/               # Documentation site
```

## Build Commands

The project uses mage (a make-like build tool) with these key commands:

```bash
go tool mage -v all        # Format, lint, test, and build everything
go tool mage -v build      # Build all binaries (main tools)
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

Build artifacts are organized in `./build/`:
- `./build/bin/` - Compiled binaries
- `./build/coverage/` - Coverage reports
- `./build/dist/` - Distribution artifacts

## Code Standards

### Go-Specific Rules ⚠️ STRICTLY ENFORCED

**FORBIDDEN PATTERNS** (automated blocking):
- ❌ `interface{}` or `any{}` - use concrete types
- ❌ `time.Sleep()` or busy waits - use channels for synchronization
- ❌ Custom error struct hierarchies
- ❌ Migration functions or compatibility layers
- ❌ Versioned function names (processV2, handleNew)
- ❌ TODOs in final code

**REQUIRED STANDARDS**:
- ✅ **Delete old code** when replacing it (no commented-out code)
- ✅ **Meaningful names**: `userID` not `id`
- ✅ **Early returns** to reduce nesting
- ✅ **Concrete types** from constructors: `func NewServer() *Server`
- ✅ **Simple errors**: `return fmt.Errorf("context: %w", err)`
- ✅ **Table-driven tests** for complex logic
- ✅ **Use go-json**: `json "github.com/goccy/go-json"` for performance
- ✅ **Channels for synchronization**: Use channels to signal readiness
- ✅ **Select for timeouts**: Use `select` with timeout channels

### Performance Standards

- Use `github.com/goccy/go-json` instead of `encoding/json` for 2-3x faster JSON parsing
- Implement parallel execution where appropriate
- Use smart caching for rule evaluation
- Profile before optimizing (use pprof for real bottlenecks)

### Security Standards

- Validate all inputs
- Use crypto/rand for randomness
- Prepared statements for SQL (never concatenate)
- Never log or expose secrets and keys
- Implement security-first handler execution

## Test Organization

**Unit Tests**: Co-located with source code in `pkg/` directories
- Fast execution, no external dependencies
- Test individual functions and methods in isolation

**Integration Tests**: `tests/integration/`
- Test interaction between multiple packages
- Cross-package functionality validation

**End-to-End Tests**: `tests/e2e/`
- Test complete binary with real input/output
- Build and execute actual binaries
- Test CLI flags and full workflows

**Test Data**: `tests/fixtures/`
- Shared test data organized by language/type
- Sample code files with known good/bad patterns

### Running Tests

```bash
# All tests
go tool mage -v test

# Specific test types
go test ./pkg/...                    # Unit tests only (fast)
go test ./tests/integration/...      # Integration tests
go test ./tests/e2e/...             # End-to-end tests (slow)
go test -short ./...                # Skip slow tests
```

## Development Workflow

### Before Starting Work

1. **Research the codebase** to understand existing patterns
2. **Plan your implementation** - understand the extensible architecture
3. **Check existing handlers** in `pkg/handlers/` for patterns
4. **Review test structure** - understand unit vs integration vs e2e testing

### When Adding Features

1. **Follow the action handler architecture** - see `examples/action_handler_demo.go`
2. **Use existing linter patterns** - see `pkg/linters/` for language-specific implementations
3. **Add appropriate tests** at all levels (unit, integration, e2e)
4. **Update documentation** if adding new handler types or configs

### Code Quality Checks

Always run before committing:
```bash
go tool mage -v check
```

**All linting issues are BLOCKING** - zero tolerance for:
- Formatting violations
- Linting warnings
- Test failures
- Forbidden patterns (interface{}, time.Sleep, etc.)

## Architecture Notes

### Action Handler System

The project uses a priority-based, extensible action handler architecture:

- **Security handlers run first** (highest priority)
- **Pluggable handler system** - easy to add new handler types
- **Configurable policies** with rule-based pattern matching
- **Supports all hook types**: PreToolUse, PostToolUse, UserPromptSubmit, etc.

### Linting Engine

Multi-language linting with:
- **Go**: golangci-lint with 30+ linters, intelligent fallback
- **JavaScript/TypeScript**: ESLint integration
- **Python**: Multiple linter support
- **Markdown**: Content and formatting validation
- **JSON**: Syntax and structure validation
- **Rust**: cargo clippy and rustfmt
- **Protocol Buffers**: protoc and buf integration

### Configuration System

Hierarchical configuration with:
- Global defaults
- Per-project overrides
- Pattern-based rules
- Environment variable expansion
- Rule priority and inheritance

## Contributing Guidelines

1. **Security first** - all security handlers must run before other handlers
2. **Performance conscious** - use go-json, implement parallel execution
3. **Test thoroughly** - unit, integration, and e2e coverage
4. **Follow Go standards** - effective Go patterns, no forbidden constructs
5. **Document changes** - update relevant docs and examples
6. **Zero tolerance for quality issues** - all linting must pass

## Common Patterns

### Adding a New Linter

1. Create new directory in `pkg/linters/`
2. Implement the `Linter` interface
3. Add configuration struct
4. Add unit tests
5. Add integration tests in `tests/integration/`
6. Update documentation

### Adding a New Handler

1. Create handler in `pkg/handlers/`
2. Implement required hook methods
3. Register with appropriate priority
4. Add comprehensive tests
5. Update examples and documentation

### Performance Optimization

1. **Profile first** - use `go tool pprof`
2. **Measure improvements** - benchmark before/after
3. **Use go-json** for JSON operations
4. **Implement parallel execution** where safe
5. **Cache expensive operations** appropriately

This project maintains high standards for code quality, security, and performance. All contributions must meet
these standards before being accepted.