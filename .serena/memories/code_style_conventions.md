# Gismo Code Style and Conventions

## Go-Specific Rules ⚠️ STRICTLY ENFORCED

### FORBIDDEN PATTERNS (automated blocking)
- ❌ `interface{}` or `any{}` - use concrete types
- ❌ `time.Sleep()` or busy waits - use channels for synchronization
- ❌ Custom error struct hierarchies
- ❌ Migration functions or compatibility layers
- ❌ Versioned function names (processV2, handleNew)
- ❌ TODOs in final code
- ❌ Commented-out code blocks

### REQUIRED STANDARDS
- ✅ **Delete old code** when replacing it
- ✅ **Meaningful names**: `userID` not `id`
- ✅ **Early returns** to reduce nesting
- ✅ **Concrete types** from constructors: `func NewServer() *Server`
- ✅ **Simple errors**: `return fmt.Errorf("context: %w", err)`
- ✅ **Table-driven tests** for complex logic
- ✅ **Use go-json**: `json "github.com/goccy/go-json"` for performance
- ✅ **Channels for synchronization**: Use channels to signal readiness
- ✅ **Select for timeouts**: Use `select` with timeout channels
- ✅ **Godoc on all exported symbols**

## Project Structure
```
cmd/                    # CLI applications
├── gismo/             # Main binary
├── gismo-init/        # Setup tool
├── gismo-show/        # Configuration inspector
├── gismo-registry/    # Package registry manager
└── gismo-package/     # Package manager

pkg/                   # Core library code
├── engine/           # Rule engines and core logic
├── linters/          # Language-specific linters
├── handlers/         # Action handlers for hooks
├── database/         # DuckDB-based code search
├── toolcache/        # Caching utilities
└── version/          # Version parsing

tests/                # Centralized test organization
├── integration/      # Cross-package integration tests
├── e2e/             # End-to-end binary tests
├── fixtures/        # Shared test data
└── utils/           # Test utility functions
```

## Performance Standards
- Use `github.com/goccy/go-json` instead of `encoding/json`
- Implement parallel execution where appropriate
- Use smart caching for rule evaluation
- Profile before optimizing (use pprof for real bottlenecks)

## Security Standards
- Validate all inputs
- Use crypto/rand for randomness
- Prepared statements for SQL (never concatenate)
- Never log or expose secrets and keys
- Implement security-first handler execution

## Testing Strategy
- **Unit Tests**: Co-located with source in `pkg/`
- **Integration Tests**: `tests/integration/`
- **End-to-End Tests**: `tests/e2e/`
- **Test Data**: `tests/fixtures/`
- Complex logic → Table-driven tests
- Hot paths → Add benchmarks
- Skip tests for main() and simple CLI parsing

## Database Conventions
- Use DuckDB for cross-platform compatibility
- Direct SQL queries (no ORM/sqlc)
- Proper array handling for embeddings
- Sequences for auto-increment (not SERIAL)
- Handle DuckDB-specific behaviors in tests