# Gismo Code Style and Conventions

## Go Standards
- Follow standard Go formatting (gofmt)
- Use meaningful variable names (`userID` not `id`)
- Early returns to reduce nesting
- Concrete types from constructors: `func NewServer() *Server`
- Simple error handling: `return fmt.Errorf("context: %w", err)`

## Forbidden Patterns (Enforced by smart-lint hook)
- **NO interface{}** or **any{}** - use concrete types!
- **NO time.Sleep()** or busy waits - use channels for synchronization!
- **NO** keeping old and new code together
- **NO** migration functions or compatibility layers
- **NO** versioned function names (processV2, handleNew)
- **NO** custom error struct hierarchies
- **NO** TODOs in final code

## Required Standards
- **Delete** old code when replacing it
- **Channels for synchronization**: Use channels to signal readiness, not sleep
- **Select for timeouts**: Use `select` with timeout channels, not sleep loops
- **Table-driven tests** for complex logic
- **Godoc on all exported symbols**

## Project Structure
```
cmd/        # Application entrypoints
internal/   # Private code (the majority goes here)  
pkg/        # Public libraries (only if truly reusable)
linters/    # Language-specific linter implementations
```

## Testing Strategy
- Complex business logic → Write tests first
- Simple CRUD → Write tests after
- Hot paths → Add benchmarks
- Skip tests for main() and simple CLI parsing