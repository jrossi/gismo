# Gismo - Extensible Claude Code Hooks Library

A high-performance Go library and CLI tool providing an extensible action handler architecture for Claude Code hooks.
Features security-first policy enforcement, configurable rule engines, and comprehensive protection across all interaction points.
Built with [go-json](https://github.com/goccy/go-json) for optimal JSON parsing performance.

## Features

### 🏗️ **Extensible Action Handler Architecture**
- **Security-First Design**: Priority-based handler execution with security handlers running first
- **Pluggable Handlers**: Easy to add custom handlers for any hook type
- **Configurable Policies**: Rule-based pattern matching and conditional execution
- **All Hook Types**: Supports PreToolUse, PostToolUse, UserPromptSubmit, Notification, and more

### 🔒 **Built-in Security Handlers**
- **File Access Control**: Block reading PEM certificates, restrict system directory writes
- **Secret Detection**: Prevent secrets in prompts and files using Gitleaks integration
- **Regex Pattern Matching**: Configurable content filtering with multiple actions
- **Path-based Restrictions**: Granular control over file system access

### ⚡ **Performance & Quality**
- **Enhanced Go Linting**: golangci-lint integration with 30+ fast linters and intelligent fallback
- **High Performance**: Uses go-json for 2-3x faster JSON parsing
- **Parallel Execution**: Concurrent handler processing for optimal speed
- **Smart Caching**: Efficient rule evaluation and pattern matching

### 🛠️ **Developer Experience**
- **Comprehensive Analysis**: Runs gosimple, ineffassign, gofmt, goimports, and many more linters
- **Module-Aware**: Correctly detects Go module roots and runs tests from proper directory
- **Configuration Support**: Hierarchical config with pattern-based overrides
- **Fully Typed**: Strong typing for all hook message types
- **Well Tested**: Comprehensive test coverage and benchmarks

## Installation

### Install with Homebrew (macOS/Linux)

```bash
brew tap jrossi/gismo https://github.com/jrossi/gismo
brew install jrossi/gismo/gismo
```

### Download Pre-built Binary

Download the latest release for your platform from the [releases page](https://github.com/jrossi/gismo/releases).

```bash
# Linux x86_64
curl -L https://github.com/jrossi/gismo/releases/latest/download/gismo_Linux_x86_64.tar.gz | tar xz

# macOS x86_64
curl -L https://github.com/jrossi/gismo/releases/latest/download/gismo_Darwin_x86_64.tar.gz | tar xz

# macOS arm64 (M1/M2)
curl -L https://github.com/jrossi/gismo/releases/latest/download/gismo_Darwin_arm64.tar.gz | tar xz

# Windows x86_64
# Download gismo_Windows_x86_64.zip from releases page
```

### Install with Go

```bash
# Install the CLI tool
go install github.com/jrossi/gismo/cmd/gismo@latest

# Use as a library
go get github.com/jrossi/gismo
```

### Build from Source

```bash
git clone https://github.com/jrossi/gismo.git
cd gismo
make install
```

## Usage

### As a Library

#### Using the New Action Handler Architecture

```go
package main

import (
    "context"
    "github.com/jrossi/gismo"
    "github.com/jrossi/gismo/handlers"
)

func main() {
    // Create the new action handler engine
    engine := gismo.NewActionHandlerEngine()

    // Register handlers with priorities (higher = runs first)
    fileHandler := handlers.NewFileAccessHandler()
    engine.RegisterHandler(gismo.PreToolUseEvent, fileHandler)

    secretHandler := handlers.NewSecretDetectionHandler()
    engine.RegisterHandler(gismo.UserPromptSubmitEvent, secretHandler)

    // Create API with action handler engine
    api := gismo.NewWithRuleEngine(engine)

    // Process stdin (for use as a hook)
    ctx := context.Background()
    if err := api.ProcessStdin(ctx); err != nil {
        // Handle error
    }
}
```

#### Legacy Library Usage

```go
package main

import (
    "context"
    "github.com/jrossi/gismo"
)

func main() {
    // Create API with default linting rule engine
    api := gismo.New()

    // Or with a custom rule engine
    api := gismo.NewWithRuleEngine(myRuleEngine)

    // Process stdin (for use as a hook)
    ctx := context.Background()
    if err := api.ProcessStdin(ctx); err != nil {
        // Handle error
    }
}
```

### Custom Rule Engine

```go
type MyRuleEngine struct{}

func (e *MyRuleEngine) EvaluatePreToolUse(ctx context.Context, msg *gismo.PreToolUseMessage) (
    *gismo.HookResponse, error) {
    // Block dangerous tools
    if msg.ToolName == "Bash" {
        return &gismo.HookResponse{
            Decision: "block",
            Reason:   "Bash commands are not allowed",
        }, nil
    }
    return &gismo.HookResponse{Decision: "approve"}, nil
}

// Implement other methods...
```

### Composite Rule Engines

```go
// Combine multiple rule engines
composite := gismo.NewCompositeRuleEngine(
    securityEngine,
    loggingEngine,
    customEngine,
)

api := gismo.NewWithRuleEngine(composite)
```

### Builder Pattern

```go
api := gismo.NewBuilder().
    WithTimeout(30 * time.Second).
    WithRuleEngine(myEngine).
    RegisterHook(gismo.HookConfig{
        Name:        "security-check",
        EventType:   gismo.PreToolUseEvent,
        ToolPattern: "Write|Edit",
        Priority:    1,
        Timeout:     30 * time.Second,
    }).
    Build()
```

### CLI Tool

The CLI tool can be used as a hook processor or to analyze configuration:

#### Hook Processing Mode (Default)

Reads hook messages from stdin and writes responses to stdout:

```bash
# Basic usage
echo '{"session_id":"123","hook_event_name":"PreToolUse",...}' | gismo

# With custom timeout
gismo -timeout 30s

# Debug mode
gismo -debug

# With custom configuration
gismo -config my-config.json
```

#### Init Command

Set up gismo in Claude Code settings:

```bash
# Initialize gismo hooks in Claude Code settings
gismo init

# Only update global settings (~/.claude/settings.json)
gismo init --global

# Only update project settings (.claude/settings.json)
gismo init --project

# Preview changes without applying them
gismo init --dry-run

# Apply changes without confirmation prompt
gismo init --force

# Configure for specific tools only (e.g., Write, Edit, Bash)
gismo init --matcher "Write"
gismo init --matcher "Bash"

# Empty matcher (default) matches all tools
gismo init --matcher ""
```

The init command:
- Adds gismo as a PostToolUse hook in Claude Code settings
- Shows proposed changes in diff format before applying
- Creates timestamped backups of existing settings
- Preserves all existing configuration and custom fields
- Detects when gismo is already configured

#### Show Command

The show command provides comprehensive visibility into gismo's configuration and behavior:

```bash
# Show current configuration
gismo show config

# Show which rules and linters apply to a specific file
gismo show filter internal/api.go

# Show setup status and configuration paths
gismo show setup

# Show available linters and their status
gismo show linters

# With custom configuration file
gismo show --config team-config.json filter pkg/public/api.go

# Debug mode for more detailed output
gismo show --debug setup

# Backward compatibility: show-actions still works
gismo show-actions internal/api.go  # Same as: gismo show filter internal/api.go
```

**Show Subcommands:**

- **`show config`**: Displays the current merged configuration in JSON format
  - Shows all linters, rules, timeouts, and parallel settings
  - Indicates configuration file loading order in debug mode

- **`show filter <file>`**: Analyzes which rules apply to a specific file
  - Shows applicable linters for the file type
  - Displays base configuration for each linter
  - Shows rule hierarchy with pattern matching
  - Displays final merged configuration after all overrides

- **`show setup`**: Checks gismo setup status
  - Binary availability in PATH
  - Configuration file locations and status
  - Claude integration status (hooks configured)
  - Summary of enabled linters and rules

- **`show linters`**: Lists all available linters
  - Supported file extensions
  - Required tools and their availability
  - Current enabled/disabled status
  - Custom configuration for each linter

### Go Linting Integration

Gismo provides comprehensive Go file linting with enhanced golangci-lint integration:

**Enhanced Linting with golangci-lint:**
- Automatically detects and uses golangci-lint for comprehensive analysis
- Runs golangci-lint in `--fast` mode for optimal performance on individual files
- Supports custom `.golangci.yml` configuration files
- Provides detailed issue reporting with line/column information
- Includes 30+ fast linters (gosimple, ineffassign, gofmt, goimports, etc.)

**Intelligent Fallback:**
- Gracefully falls back to basic `go/format` checking if golangci-lint is unavailable
- Maintains functionality even without golangci-lint installed
- Ensures consistent behavior across different development environments

**Pre-Write Validation:**
- Blocks writes of Go files with severe syntax errors
- Warns about linting issues (but allows the write)
- Skips generated files and testdata directories
- Module-aware operation for proper import resolution

**Performance Characteristics:**
- Enhanced linting: ~100ms per file (comprehensive analysis)
- Basic fallback: ~4μs per file (syntax/format only)
- Optimized for real-time development feedback

**Post-Write Actions:**
- Currently limited due to hook message structure - PostToolUse messages don't include file paths
- Test running is available during PreToolUse validation for immediate feedback
- All operations are module-aware and respect Go project structure

**Example Hook Configuration:**
```json
{
  "PreToolUse": [
    {
      "command": "/path/to/gismo",
      "tool_patterns": ["Write", "Edit", "MultiEdit"]
    }
  ]
}
```

**Behavior Examples:**
```bash
# Clean Go code → Approved
{"decision": "approve"}

# Code with linting issues → Approved with detailed warnings
{
  "decision": "approve",
  "message": "Found 2 linting issues: Line 9: S1021 (gosimple), Line 13: needs gofmt"
}

# Syntax error → Blocked
{"decision": "block", "reason": "syntax: Go syntax error: missing ',' before newline"}

# golangci-lint unavailable → Basic linting fallback
{"decision": "approve", "message": "File test.go is not properly formatted. Consider running gofmt."}
```

## Hook Message Types

The library supports all Claude Code hook types:

- `PreToolUse`: Before tool execution
- `PostToolUse`: After tool execution
- `Notification`: System notifications
- `Stop`: Main agent completion
- `SubagentStop`: Subagent completion
- `PreCompact`: Before context compression

## Environment Variable Support

Gismo supports environment variable expansion in hook command paths, following Claude Code conventions:

**Supported Variables:**
- `$CLAUDE_PROJECT_DIR`: Expands to the current working directory (project root)
- Standard environment variables: `$HOME`, `$USER`, `$PATH`, etc.

**Example Configurations:**
```json
{
  "PreToolUse": [
    {
      "command": "$CLAUDE_PROJECT_DIR/scripts/pre-commit.sh",
      "tool_patterns": ["Write", "Edit"]
    }
  ],
  "PostToolUse": [
    {
      "command": "$HOME/.local/bin/gismo",
      "tool_patterns": ["Write", "Edit", "MultiEdit"]
    }
  ]
}
```

**Benefits:**
- **Portable configurations**: Works across different machines and users
- **Project-relative paths**: `$CLAUDE_PROJECT_DIR` ensures hooks run from project context
- **Flexible deployment**: Same configuration works in different environments

## Performance

Benchmarks show excellent performance across all components:

**Core System:**
- Message parsing: ~700ns per message
- Rule evaluation: <1ns for simple rules
- Full pipeline: ~22ns for handler processing

**Go Linting Performance:**
- Enhanced linting (golangci-lint --fast): ~100ms per file
- Basic fallback (go/format): ~4μs per file
- Performance optimized for real-time development feedback

## API Documentation

### Core Types

- `API`: Main interface for the library
- `RuleEngine`: Interface for custom rule implementations
- `Handler`: Processes hook messages
- `Parser`: High-performance JSON parser
- `Registry`: Manages hook configurations

### Response Format

Hook responses can use either exit codes or JSON:

**Exit Codes:**
- 0: Success (stdout shown)
- 2: Blocking error (stderr processed)
- Other: Non-blocking error

**JSON Response:**
```json
{
  "continue": false,
  "stopReason": "Security violation",
  "decision": "block",
  "reason": "Tool access denied"
}
```

## Examples

Working examples are included in the usage section above. For production use, ensure your hooks.json
configuration points to the installed gismo binary location.

## Contributing

Contributions are welcome! Please ensure:
- All tests pass
- Linting passes with no warnings
- Benchmarks show no performance regression

## License

[License to be determined]