---
title: "CLI Reference"
linkTitle: "CLI"
weight: 50
description: >
  Complete command-line interface documentation
---

# CLI Reference

Gismo provides a comprehensive command-line interface for processing Claude Code hooks, analyzing linting
configurations, and managing knowledge bases with documentation and code search capabilities.

## Installation

```bash
# Install all CLI tools
go install github.com/jrossi/gismo/cmd/gismo@latest
go install github.com/jrossi/gismo/cmd/gismo-knowledge@latest
go install github.com/jrossi/gismo/cmd/gismo-query@latest
```

## Commands

### gismo - Hook Processing

#### Default Mode (Hook Processing)

Process Claude Code hook messages from stdin:

```bash
# Basic usage - reads from stdin
gismo

# With custom timeout
gismo -timeout 30s

# With custom configuration
gismo -config my-config.json

# Debug mode
gismo -debug
```

#### init Command

Set up gismo in Claude Code settings:

```bash
# Initialize gismo hooks (updates both global and project settings)
gismo init

# Only update global settings (~/.claude/settings.json)
gismo init --global

# Only update project settings (.claude/settings.json)
gismo init --project

# Preview changes without applying them
gismo init --dry-run

# Apply changes without confirmation prompt
gismo init --force

# Configure for specific tools only
gismo init --matcher "Write"    # Only for Write tool
gismo init --matcher "Edit"     # Only for Edit tool
gismo init --matcher "Bash"     # Only for Bash tool
gismo init --matcher ""         # All tools (default)
```

The `init` command:
- Adds gismo as a PostToolUse hook in Claude Code settings
- Shows proposed changes in diff format before applying
- Creates timestamped backups of existing settings
- Preserves all existing configuration and custom fields
- Detects when gismo is already configured

#### show Command

The `show` command provides comprehensive visibility into gismo's configuration and behavior.
It includes several subcommands:

#### show config

Display the current merged configuration:

```bash
# Show current configuration
gismo show config

# With custom configuration file
gismo show --config team-config.json config

# Debug mode shows configuration sources
gismo show --debug config
```

#### show filter

Analyze which rules and linters apply to specific files:

```bash
# Show configuration for a single file
gismo show filter internal/api.go

# With custom configuration
gismo show --config team-config.json filter pkg/api.go

# Debug mode shows pattern matching details
gismo show --debug filter internal/test.go
```

#### show setup

Check gismo setup status:

```bash
# Show setup status
gismo show setup

# Debug mode includes environment details
gismo show --debug setup
```

#### show linters

List all available linters and their status:

```bash
# Show all linters
gismo show linters

# With custom configuration
gismo show --config team-config.json linters
```

#### Backward Compatibility

The old `show-actions` command still works and maps to `show filter`:

```bash
# These are equivalent:
gismo show-actions internal/api.go
gismo show filter internal/api.go
```

#### Show Command Features

- **`show config`**: Displays the complete merged configuration in JSON format
- **`show filter <file>`**: Shows which linters and rules apply to a specific file
- **`show setup`**: Checks binary availability, config files, and Claude integration
- **`show linters`**: Lists all linters with their supported files and tool requirements

### gismo-knowledge - Knowledge Base Management

Manage docsets, documentation, and searchable knowledge:

```bash
# Import Go documentation from Kapeli Dash feeds
gismo-knowledge import --url https://kapeli.com/feeds/Go.xml

# Import from local docset
gismo-knowledge import --path ~/Downloads/Python.docset

# List all installed docsets
gismo-knowledge list

# Search across all knowledge sources
gismo-knowledge search "http handler"

# Search with content preview
gismo-knowledge search -v "error handling"

# Get full content by ID
gismo-knowledge get 123

# Show knowledge base statistics
gismo-knowledge stats

# Remove a docset
gismo-knowledge remove Go
```

#### Knowledge Commands

- **`import`**: Import docsets from URLs or local paths
- **`list`**: List all installed docsets with statistics
- **`search`**: Search across all knowledge sources with various options
- **`get`**: Retrieve full content by ID
- **`remove`**: Remove docsets from the knowledge base
- **`stats`**: Show comprehensive knowledge base statistics
- **`push`**: Push files or content to knowledge base (planned)
- **`index`**: Manage search indexes (planned)

### gismo-query - SQL Query Interface

Execute SQL queries directly against the knowledge database:

```bash
# List all docsets
gismo-query "SELECT * FROM docsets"

# Search for content
gismo-query "SELECT name, type, path FROM docset_content WHERE name LIKE '%http%' LIMIT 10"

# Get statistics
gismo-query "SELECT docset_id, COUNT(*) as count FROM docset_content GROUP BY docset_id"

# Interactive mode
gismo-query
# gismo> SELECT * FROM docsets;
# gismo> .tables
# gismo> .quit

# Stream large results
gismo-query --stream "SELECT * FROM docset_content"

# Output formats
gismo-query --format json "SELECT id, name, version FROM docsets"
gismo-query --format csv "SELECT * FROM docsets"

# Query from pipe
echo "SELECT COUNT(*) FROM docsets" | gismo-query
```

#### Query Features

- **Interactive shell**: SQL REPL with `.tables`, `.schema`, `.help` commands
- **Multiple output formats**: table, JSON, CSV
- **Streaming results**: Handle large datasets efficiently
- **Pipe support**: Integrate with shell scripts and workflows
- **Special commands**: `.tables` to list tables, `.schema` for schemas

#### Database Schema

Key tables in the knowledge database:

```sql
-- Docsets table
CREATE TABLE docsets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT,
    language TEXT,
    source_url TEXT,
    imported_at TIMESTAMP,
    content_count INTEGER
);

-- Content table
CREATE TABLE docset_content (
    id INTEGER PRIMARY KEY,
    docset_id TEXT,
    name TEXT,
    type TEXT,
    path TEXT,
    content TEXT,
    summary TEXT
);
```

## Global Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-config` | Path to configuration file | Auto-detect |
| `-debug` | Enable debug output | false |
| `-timeout` | Hook execution timeout | 60s |
| `-version` | Show version information | - |

## Exit Codes

| Code | Description | Usage |
|------|-------------|-------|
| 0 | Success | Output shown in transcript (stdout) |
| 1 | Non-blocking error | General errors |
| 2 | Blocking error | Feedback processed by Claude (stderr) |

## Configuration

Gismo looks for configuration files in the following order:
1. `~/.claude/gismo.json` (user global)
2. `.claude/gismo.json` (project-specific)
3. `.claude/gismo.local.json` (local overrides, git-ignored)
4. File specified with `-config` flag

## Hook Processing

### Processing Flow

1. Read hook message from stdin
2. Parse message type (PreToolUse, PostToolUse, etc.)
3. Apply configured rules based on tool and file patterns
4. Run applicable linters for file operations
5. Return response with decision and feedback

### Example Hook Message

```json
{
  "session_id": "123",
  "hook_event_name": "PostToolUse",
  "tool_name": "Write",
  "tool_input": {
    "file_path": "src/main.go",
    "content": "package main\n\nfunc main() {\n\t// code\n}"
  }
}
```

### Example Response

```json
{
  "decision": "approve",
  "message": "✅ Style clean. Continue with your task."
}
```

## Integration Examples

### Claude Code Settings

After running `gismo init`, your Claude Code settings will include:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "gismo",
            "timeout": 60000,
            "continueOnError": false
          }
        ]
      }
    ]
  }
}
```

### Project Configuration

Example `.claude/gismo.json`:

```json
{
  "linters": {
    "golang": {
      "enabled": true,
      "config": {
        "golangciConfig": ".golangci.yml"
      }
    },
    "markdown": {
      "enabled": true,
      "config": {
        "maxLineLength": 120
      }
    }
  },
  "rules": [
    {
      "pattern": "**/*_test.go",
      "linter": "golang",
      "rules": {
        "disabledChecks": ["line-length"]
      }
    }
  ]
}
```

## Troubleshooting

### Common Issues

#### gismo not found

Ensure gismo is in your PATH:

```bash
# Check installation
which gismo

# Add Go bin to PATH if needed
export PATH=$PATH:$(go env GOPATH)/bin
```

#### Configuration not loaded

Check which configuration files are being loaded:

```bash
gismo -debug show-actions test.go
```

#### Hook not triggering

Verify Claude Code settings:

```bash
# Check if hook is configured
cat ~/.claude/settings.json | jq '.hooks.PostToolUse'

# Re-run init if needed
gismo init
```

### Getting Help

```bash
# Show usage
gismo -help

# Show version
gismo -version
```

## Related Documentation

- [Installation Guide](/docs/installation/) - Detailed installation instructions
- [Configuration Guide](/docs/configuration/) - Configuration options and examples
- [Quick Start](/docs/quickstart/) - Getting started guide
- [Linters](/docs/linters/) - Available linters and their options