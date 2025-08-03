# Go Import Restrictions

The Go linter in Gismo supports AST-based import restrictions that allow you to block specific packages and suggest replacements. This feature is completely configurable and does nothing unless explicitly configured.

## Features

- **AST-based analysis** - Uses Go's native AST parsing for accurate import detection
- **Zero overhead when unconfigured** - No performance impact when import restrictions are not configured
- **Detailed error messages** - Provides clear reasons and suggested replacements
- **Batch processing support** - Works with both single file and batch linting
- **Integration with existing linter** - Seamlessly integrated with the existing golang linter

## Configuration

Add import restrictions to your gismo configuration file:

```json
{
  "golang": {
    "importRestrictions": {
      "encoding/json": {
        "blocked": true,
        "replacement": "github.com/goccy/go-json",
        "reason": "Use go-json for 2-3x better performance"
      },
      "io/ioutil": {
        "blocked": true,
        "replacement": "io and os packages",
        "reason": "deprecated since Go 1.16"
      },
      "github.com/pkg/errors": {
        "blocked": true,
        "replacement": "fmt.Errorf with %w verb",
        "reason": "Use standard library error wrapping"
      }
    }
  }
}
```

## Configuration Options

Each import restriction supports the following options:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `blocked` | boolean | Yes | Set to `true` to block the import |
| `replacement` | string | No | Suggested replacement package |
| `reason` | string | No | Explanation for why the import is restricted |

## Example Output

When a restricted import is detected, the linter will report an error like:

```
examples/demo_restricted_imports.go:4:2: Import 'encoding/json' is not allowed: Use go-json for 2-3x better performance. Use 'github.com/goccy/go-json' instead (import-restriction)
```

## Use Cases

### Performance Optimization
Block slower packages in favor of faster alternatives:
```json
{
  "encoding/json": {
    "blocked": true,
    "replacement": "github.com/goccy/go-json",
    "reason": "2-3x better performance"
  }
}
```

### Deprecation Management
Block deprecated packages:
```json
{
  "io/ioutil": {
    "blocked": true,
    "replacement": "io and os packages",
    "reason": "deprecated since Go 1.16"
  }
}
```

### Architecture Enforcement
Enforce architectural decisions:
```json
{
  "log": {
    "blocked": true,
    "replacement": "structured logging library like slog",
    "reason": "Use structured logging for better observability"
  }
}
```

### Security Compliance
Block packages with known security issues:
```json
{
  "crypto/md5": {
    "blocked": true,
    "replacement": "crypto/sha256",
    "reason": "MD5 is cryptographically broken"
  }
}
```

## Integration with Existing Tools

This feature complements existing tools like:

- **depguard** - While depguard is excellent for golangci-lint integration, this feature provides:
  - Native integration with gismo's linter architecture
  - Unified configuration and error reporting
  - Zero overhead when not configured
  - Extensible for future AST-based checks

- **golangci-lint** - Can be used alongside golangci-lint for comprehensive linting

## Technical Details

### AST Parsing
The import restriction checker:
1. Parses Go source files using `go/parser`
2. Walks through all import declarations
3. Matches import paths against configured restrictions
4. Reports violations with precise file positions

### Performance
- **Minimal overhead** - Reuses AST parsing already done for other checks
- **Efficient matching** - Uses map lookups for O(1) restriction checking
- **Batch processing** - Processes multiple files efficiently

### Error Handling
- **Graceful degradation** - Skips import checking if file has syntax errors
- **Precise positioning** - Reports exact line and column of restricted imports
- **Consistent severity** - All import restrictions are reported as errors

## Example Demo

See `examples/demo_restricted_imports.go` for a demonstration file that will trigger multiple import restriction errors when linted with the example configuration.

Run the demo:
```bash
# Create a config file with import restrictions
cp examples/golang_import_restrictions.json .gismo.json

# Lint the demo file to see import restrictions in action
gismo lint examples/demo_restricted_imports.go
```

This will show how the linter detects and reports restricted imports with helpful messages and suggestions.