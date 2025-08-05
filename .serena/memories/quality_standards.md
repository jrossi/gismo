# Gismo Quality Standards

## Zero Tolerance Policy
The project enforces ZERO tolerance for quality issues. All code must meet these standards before being accepted.

## Automated Enforcement
The `~/.claude/hooks/smart-lint.sh` hook automatically blocks commits that violate these rules.

## Quality Checklist

### Linting Standards
- ✅ **ZERO warnings** from golangci-lint (30+ linters enabled)
- ✅ **ZERO disabled linter rules** without documented justification
- ✅ **ZERO "nolint" comments** without explanation
- ✅ **ZERO formatting issues** (all code must be gofmt'd)

### Code Quality
- ✅ No `interface{}` or `any{}` - concrete types everywhere
- ✅ No `time.Sleep()` for synchronization - channels only
- ✅ Simple error handling - no custom error hierarchies
- ✅ Early returns to reduce nesting
- ✅ Meaningful variable names
- ✅ Proper context propagation
- ✅ No goroutine leaks
- ✅ Deferred cleanup where appropriate
- ✅ No race conditions (tested with -race flag)

### Testing Standards
- ✅ ALL tests pass without flakiness
- ✅ Meaningful test coverage (quality over quantity)
- ✅ Table-driven tests for complex logic
- ✅ No skipped tests without justification
- ✅ Tests actually test behavior, not implementation

### Documentation
- ✅ All exported symbols have godoc comments
- ✅ No commented-out code blocks
- ✅ No debugging print statements
- ✅ No placeholder implementations
- ✅ Clear commit messages with context

### Security Requirements
- ✅ Input validation on all external data
- ✅ SQL queries use prepared statements
- ✅ Crypto operations use crypto/rand
- ✅ No hardcoded secrets or credentials
- ✅ Proper permission checks

### Performance Standards
- ✅ Use github.com/goccy/go-json for JSON operations
- ✅ No obvious N+1 queries
- ✅ Appropriate use of pointers vs values
- ✅ Buffered channels where beneficial
- ✅ No unnecessary allocations in hot paths
- ✅ No busy-wait loops

## Verification Commands
```bash
# Run ALL checks - must pass with ZERO issues
go tool mage -v check

# Individual checks
go tool mage -v fmt    # Format code
go tool mage -v lint   # Run linters
go tool mage -v test   # Run tests

# Hook verification
~/.claude/hooks/smart-lint.sh
```

## Response to Issues
When issues are found:
1. **FIX IMMEDIATELY** - No exceptions
2. **NO EXCUSES** - "It's just formatting" → Fix it NOW
3. **VERIFY** - Re-run all checks after fixes
4. **REPEAT** - Until ALL checks show ✅ GREEN