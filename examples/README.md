# Gismo Configuration Examples

This directory contains example configuration files for Gismo, including setup for the hook system,
linting configurations, and knowledge management integration.

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

## Knowledge System Usage Examples

### Setting Up Documentation Import

```bash
# Import Go documentation for your project
gismo-knowledge import --url https://kapeli.com/feeds/Go.xml

# Import Python documentation
gismo-knowledge import --url https://kapeli.com/feeds/Python_3.xml

# Import from local docset
gismo-knowledge import --path ~/Downloads/React.docset
```

### Searching Your Knowledge Base

```bash
# Basic search for HTTP-related content
gismo-knowledge search "http handler"

# Search with content preview
gismo-knowledge search -v "error handling patterns"

# Search within specific docset
gismo-knowledge search --docset Go "context cancellation"

# Get detailed content by ID
gismo-knowledge get 42
```

### Advanced SQL Queries

```sql
-- Find all function-type entries across docsets
SELECT d.name as docset, c.name, c.path
FROM docsets d
JOIN docset_content c ON d.id = c.docset_id
WHERE c.type = 'Function'
LIMIT 20;

-- Get content statistics by docset
SELECT d.name, d.version, COUNT(c.id) as entries,
       AVG(LENGTH(c.content)) as avg_content_length
FROM docsets d
LEFT JOIN docset_content c ON d.id = c.docset_id
GROUP BY d.id, d.name, d.version;

-- Search for error handling patterns
SELECT c.name, c.type, c.path,
       SUBSTR(c.content, 1, 200) as preview
FROM docset_content c
WHERE c.content LIKE '%error%'
  AND c.content LIKE '%handle%'
LIMIT 10;
```

### Integration with Claude Code

Configure your `.claude/hooks.json` to include knowledge-aware validation:

```json
{
  "PreToolUse": [
    {
      "command": "gismo",
      "tool_patterns": ["Write", "Edit", "MultiEdit"]
    }
  ],
  "PostToolUse": [
    {
      "command": "gismo-knowledge search --limit 3",
      "tool_patterns": ["Write"],
      "description": "Find relevant documentation for new code"
    }
  ]
}
```

## Configuration Usage

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

4. Set up knowledge base for your project:
   ```bash
   # Import relevant documentation
   gismo-knowledge import --url https://kapeli.com/feeds/YourFramework.xml
   ```

## Tips

- Start with `basic-config.json` and add complexity as needed
- Use `.claude/gismo.local.json` for personal preferences that shouldn't be committed
- Use pattern-based rules to handle special cases without modifying global settings
- Disable specific checks rather than entire linters when possible