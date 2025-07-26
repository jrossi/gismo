# Gismo Task Completion Checklist

## When a coding task is complete:

### Automated Checks (MANDATORY - ALL must be ✅ GREEN)
1. **Formatting**: `make fmt` produces no changes
2. **Linting**: `make lint` passes with zero issues  
3. **Tests**: `make test` passes with exit code 0
4. **Race Detection**: Tests pass with `-race` flag
5. **Coverage**: Coverage reports are generated successfully

### Manual Verification
- [ ] All linters pass with zero issues
- [ ] All tests pass including race detection
- [ ] Feature works end-to-end
- [ ] Old code is deleted (no migration functions)
- [ ] Godoc on all exported symbols
- [ ] No forbidden patterns (enforced by smart-lint)

### Claude Code Hook Integration
- The project has global Claude Code hooks configured
- Hook failures with exit code 2 are **BLOCKING**
- Format issues must be resolved immediately
- Syntax errors prevent any commits

### Recovery Protocol
When hooks fail:
1. **STOP IMMEDIATELY** - Do not continue with other tasks
2. **FIX ALL ISSUES** - Address every ❌ issue until everything is ✅ GREEN  
3. **VERIFY THE FIX** - Re-run the failed command to confirm it's fixed
4. **CONTINUE ORIGINAL TASK** - Return to what you were doing before

### System-Specific Notes
- Platform: Darwin (macOS)
- Go version: 1.23.2
- Uses golangci-lint for comprehensive linting
- Supports multiple linting backends per language