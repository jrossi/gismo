# Gismo Configuration Examples

This directory contains example configuration files for Gismo.

## Configuration Files

### basic-config.json
A simple configuration file showing basic settings for golang and markdown linters.

### advanced-config.json
A comprehensive example showing:
- Custom golangci-lint configuration paths
- Disabled checks for specific linters
- Pattern-based rule overrides for test files, generated files, and documentation
- Frontmatter schema validation for markdown files

### team-config.json
An example configuration for team use showing:
- Different rules for internal vs public packages
- Disabling all checks for generated files (protobuf, etc.)
- Special handling for test data directories
- Relaxed rules for changelog files

## Configuration Loading Order

Gismo loads configuration files in the following order (later files override earlier ones):

1. `~/.claude/gismo.json` - User's global configuration
2. `PROJECT_DIR/.claude/gismo.json` - Project-specific configuration
3. `PROJECT_DIR/.claude/gismo.local.json` - Local overrides (typically git-ignored)

You can also specify a custom configuration file using the `--config` flag.

## Configuration Structure

### Global Settings
```json
{
  "parallel": {
    "maxWorkers": 4,           // Number of parallel workers
    "disableParallel": false   // Disable parallel execution
  },
  "timeout": "5m"              // Timeout for hook execution
}
```

### Linter Configuration
```json
{
  "linters": {
    "golang": {
      "enabled": true,
      "config": {
        "golangciConfig": "path/to/.golangci.yml",
        "disabledChecks": ["gofmt", "gosec"],
        "testTimeout": "10m"
      }
    },
    "markdown": {
      "enabled": true,
      "config": {
        "maxLineLength": 120,
        "requireFrontmatter": false,
        "maxBlankLines": 2,
        "listIndentSize": 2,
        "disabledRules": ["rule-name"]
      }
    }
  }
}
```

### Pattern-Based Rule Overrides
```json
{
  "rules": [
    {
      "pattern": "*.go",        // Glob pattern for files
      "linter": "golang",       // Which linter (or "*" for all)
      "rules": {                // Override rules for matching files
        "disabledChecks": ["gofmt"]
      }
    }
  ]
}
```

## Common Patterns

### Disable Linting for Generated Files
```json
{
  "rules": [
    {
      "pattern": "*.generated.go",
      "linter": "golang",
      "rules": {
        "disabledChecks": ["all"]
      }
    }
  ]
}
```

### Different Rules for Tests
```json
{
  "rules": [
    {
      "pattern": "*_test.go",
      "linter": "golang",
      "rules": {
        "testTimeout": "15m",
        "disabledChecks": ["dupl", "gocyclo"]
      }
    }
  ]
}
```

### Strict Documentation Requirements
```json
{
  "rules": [
    {
      "pattern": "docs/*.md",
      "linter": "markdown",
      "rules": {
        "requireFrontmatter": true,
        "maxLineLength": 80
      }
    }
  ]
}
```

## Usage

1. Copy one of the example files to your project:
   ```bash
   mkdir -p .claude
   cp examples/basic-config.json .claude/gismo.json
   ```

2. Customize the configuration for your needs

3. Test the configuration:
   ```bash
   gismo --config .claude/gismo.json
   ```

## Tips

- Start with `basic-config.json` and add complexity as needed
- Use `.claude/gismo.local.json` for personal preferences that shouldn't be committed
- Use pattern-based rules to handle special cases without modifying global settings
- Disable specific checks rather than entire linters when possible