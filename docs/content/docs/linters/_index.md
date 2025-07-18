---
title: "Linters"
linkTitle: "Linters"
weight: 40
description: >
  Language-specific linting capabilities and configuration
---

# Linters

Gismo provides comprehensive linting support for multiple programming languages and file formats.

## Supported Linters

| Language | Tool | Features |
|----------|------|----------|
| [Go](/docs/linters/golang/) | golangci-lint | 30+ linters, module-aware, fast mode |
| [Python](/docs/linters/python/) | UV/UVX + ruff | Modern Python tooling, fast execution |
| [JavaScript](/docs/linters/javascript/) | ESLint | Configurable rules, TypeScript support |
| [Rust](/docs/linters/rust/) | cargo clippy + rustfmt | Clippy lints, format checking, test execution |
| [Protocol Buffers](/docs/linters/protobuf/) | buf, protolint, protoc | Style enforcement, syntax validation |
| [Markdown](/docs/linters/markdown/) | Built-in | Frontmatter validation, line length |
| [JSON](/docs/linters/json/) | Built-in | Schema validation, syntax checking |

## Quick Configuration

Enable all linters with sensible defaults:

```json
{
  "linters": {
    "golang": { "enabled": true },
    "python": { "enabled": true },
    "javascript": { "enabled": true },
    "rust": { "enabled": true },
    "protobuf": { "enabled": true },
    "markdown": { "enabled": true },
    "json": { "enabled": true }
  }
}
```

## Performance Characteristics

| Linter | Typical Speed | Features |
|--------|---------------|----------|
| Go | ~100ms (enhanced) / ~4μs (fallback) | Module detection, parallel execution |
| Python | ~50ms | UV-based tooling, modern Python support |
| JavaScript | ~30ms | ESLint integration, configurable rules |
| Rust | ~500ms (no deps) / ~5s+ (with deps) | Cargo integration, clippy + rustfmt |
| Protocol Buffers | ~200ms | buf linting, protoc validation |
| Markdown | ~10ms | Built-in goldmark processing |
| JSON | ~1ms | High-performance go-json parsing |

## Common Use Cases

### Code Quality Enforcement

```json
{
  "linters": {
    "golang": {
      "config": {
        "disabledChecks": [],
        "enabledChecks": ["bodyclose", "errcheck", "gosec"]
      }
    }
  }
}
```

### Documentation Standards

```json
{
  "linters": {
    "markdown": {
      "config": {
        "maxLineLength": 80,
        "requireFrontmatter": true,
        "frontmatterSchema": "docs/schema.json"
      }
    }
  }
}
```

### Multi-Language Projects

```json
{
  "linters": {
    "golang": { "enabled": true },
    "python": { "enabled": true },
    "markdown": { "enabled": true }
  },
  "rules": [
    {
      "pattern": "*.go",
      "linter": "golang",
      "rules": { "testTimeout": "10m" }
    },
    {
      "pattern": "*.py",
      "linter": "python",
      "rules": { "tool": "uv" }
    },
    {
      "pattern": "docs/*.md",
      "linter": "markdown",
      "rules": { "requireFrontmatter": true }
    }
  ]
}
```

## Pattern-Based Configuration

Apply different rules to different file patterns:

```json
{
  "rules": [
    {
      "pattern": "*_test.go",
      "linter": "golang",
      "rules": {
        "disabledChecks": ["dupl", "gocyclo"]
      }
    },
    {
      "pattern": "*.generated.*",
      "linter": "*",
      "rules": {
        "enabled": false
      }
    },
    {
      "pattern": "docs/**",
      "linter": "markdown",
      "rules": {
        "maxLineLength": 80
      }
    }
  ]
}
```

## Integration Examples

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit
gismo --config .claude/gismo.json
```

### CI/CD Pipeline

```yaml
# GitHub Actions
- name: Lint Code
  run: |
    go install github.com/jrossi/gismo/cmd/gismo@latest
    gismo --config .claude/gismo.json
```

### Make Target

```makefile
lint:
	gismo --config .claude/gismo.json

lint-fix:
	gismo --config .claude/gismo.json --fix
```

## Language-Specific Guides

- **[Go Linting](/docs/linters/golang/)** - golangci-lint integration and configuration
- **[Python Linting](/docs/linters/python/)** - UV/UVX and ruff setup
- **[JavaScript Linting](/docs/linters/javascript/)** - ESLint configuration
- **[Rust Linting](/docs/linters/rust/)** - cargo clippy and rustfmt integration
- **[Protocol Buffer Linting](/docs/linters/protobuf/)** - buf, protolint, and protoc support
- **[Markdown Linting](/docs/linters/markdown/)** - Documentation standards
- **[JSON Validation](/docs/linters/json/)** - Schema validation and syntax checking

## Advanced Topics

### Custom Rule Engines

Gismo supports pluggable rule engines for custom linting logic:

```go
type CustomEngine struct{}

func (e *CustomEngine) ShouldProcess(ctx context.Context, msg *Message) (bool, error) {
    // Custom processing logic
    return true, nil
}

func (e *CustomEngine) ProcessMessage(ctx context.Context, msg *Message) (*Result, error) {
    // Custom linting implementation
    return &Result{}, nil
}
```

### Composite Rule Engines

Chain multiple rule engines for complex processing:

```go
engine := gismo.NewCompositeRuleEngine(
    &GoLintEngine{},
    &CustomSecurityEngine{},
    &DocumentationEngine{},
)
```

## Troubleshooting

### Common Issues

1. **Linter not found**: Ensure the underlying tool is installed (golangci-lint, ruff, etc.)
2. **Slow performance**: Use `"fastMode": true` for Go linting
3. **Memory usage**: Adjust `"maxWorkers"` in parallel configuration
4. **Configuration not loading**: Check file paths and JSON syntax

### Debug Mode

Enable verbose output for troubleshooting:

```bash
gismo --config .claude/gismo.json --verbose
```

### Performance Profiling

Profile linter performance:

```bash
gismo --config .claude/gismo.json --profile
```