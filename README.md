# Gismo - Real-Time Claude Code Feedback Engine

Gismo is a high-performance parsing library and hook system designed to optimize Claude Code's performance through
immediate feedback loops. By providing real-time validation, linting, correction, and intelligent knowledge management
as Claude Code operates, Gismo prevents the AI from veering off course, eliminates failure loops, and ensures valid
output with contextual awareness from the moment files are created.

## The Problem Gismo Solves

When Claude Code operates without immediate feedback, it can:
- Write invalid code that fails later in the development cycle
- Go down unproductive paths due to lack of real-time validation
- Create cascading failures that require extensive backtracking
- Waste time on syntactically or stylistically incorrect implementations

## The Gismo Solution: Software-Defined Feedback Loops

Gismo creates **software-defined feedback loops** that provide instantaneous analysis and correction:

🔄 **Real-Time Validation**: Files are analyzed the moment Claude Code attempts to write them
⚡ **Immediate Correction**: Syntax errors, style violations, and policy breaches are caught instantly
🎯 **Course Correction**: Prevents Claude Code from pursuing invalid implementation paths
🛡️ **Security Enforcement**: Blocks dangerous operations and secret exposure before they happen
📊 **Context Management**: Maintains awareness of project standards and coding patterns

This approach makes Claude Code dramatically more performant by eliminating the traditional "write-test-fix" cycle
in favor of "validate-while-writing" for immediate course correction.

## Architecture: Extensible Action Handlers for Claude Code

Built on an extensible action handler architecture with security-first policy enforcement, configurable rule engines,
and comprehensive protection across all Claude Code interaction points. Optimized with
[go-json](https://github.com/goccy/go-json) for maximum JSON parsing performance.

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
- **Knowledge Database**: DuckDB-powered knowledge management with vector search capabilities

### 🛠️ **Developer Experience**
- **Comprehensive Analysis**: Runs gosimple, ineffassign, gofmt, goimports, and many more linters
- **Module-Aware**: Correctly detects Go module roots and runs tests from proper directory
- **Configuration Support**: Hierarchical config with pattern-based overrides
- **Fully Typed**: Strong typing for all hook message types
- **Well Tested**: Comprehensive test coverage and benchmarks
- **Knowledge Management**: Import and search docsets, documentation, and code knowledge
- **SQL Query Interface**: Direct SQL access to knowledge database for advanced queries

## Project Structure

```text
cmd/                    # CLI applications
├── gismo/             # Main binary (hook processor)
├── gismo-server/      # Server binary (main component)
├── gismo-knowledge/   # Knowledge base management
└── gismo-query/       # SQL query interface for knowledge DB

pkg/                   # Core library code
├── engine/           # Rule engines and core logic
├── linters/          # Language-specific linters
│   ├── golang/      # Go language linting
│   ├── javascript/  # JavaScript/TypeScript linting
│   ├── python/      # Python linting
│   ├── json/        # JSON validation
│   ├── markdown/    # Markdown linting
│   ├── secrets/     # Secret detection
│   ├── rust/        # Rust linting
│   └── protobuf/    # Protocol Buffer linting
├── handlers/         # Action handlers for hooks
├── toolcache/        # Caching utilities
├── server/          # Server implementation
├── client/          # Client implementation
├── database/        # DuckDB-based knowledge management
├── knowledge/       # Knowledge system core logic
└── version/         # Version parsing

tests/                # Centralized test organization
├── integration/      # Cross-package integration tests
├── e2e/             # End-to-end binary tests
├── fixtures/        # Shared test data
├── examples/        # Example test utilities
└── utils/           # Test utility functions

examples/            # Example configurations and demos
docs/               # Documentation site
magefiles/          # Build system (mage-based)
```

## Build Commands

The project uses mage (a make-like build tool) with these key commands:

```bash
go tool mage -v all        # Format, lint, test, and build everything
go tool mage -v build      # Build all binaries (smart dependency tracking)
go tool mage -v test       # Run all tests with race detection and coverage
go tool mage -v fmt        # Format code with gofmt
go tool mage -v lint       # Run golangci-lint
go tool mage -v install    # Install binaries to GOPATH
go tool mage -v clean      # Clean build artifacts
go tool mage -v deps       # Download and tidy dependencies
go tool mage -v coverage   # Generate HTML coverage report
go tool mage -v check      # Run all quality checks (fmt, lint, test)
go tool mage -v info       # Show build information
go tool mage -l            # List all available targets
```

Build artifacts are organized in `./build/`:
- `./build/bin/` - Compiled binaries
- `./build/coverage/` - Coverage reports
- `./build/dist/` - Distribution artifacts

## Knowledge System Features

### 🗄️ **Docset Management**
- **Import from Kapeli Dash feeds**: Direct integration with popular documentation sources
- **Local docset support**: Import .docset directories and archives
- **Version tracking**: Maintain multiple versions of documentation
- **Automatic indexing**: Full-text and semantic search capabilities

### 🔍 **Advanced Search**
- **Keyword search**: Fast text-based search across all content
- **Semantic search**: Vector-based similarity search (when available)
- **Hybrid search**: Combine keyword and semantic approaches
- **Docset filtering**: Limit searches to specific documentation sets
- **Content preview**: See relevant snippets before diving deep

### 🏗️ **Database Architecture**
- **DuckDB backend**: Cross-platform embedded OLAP database
- **Vector search**: HNSW indexing for similarity search
- **Project isolation**: Separate knowledge contexts per Claude Code project
- **SQL interface**: Direct database access for advanced queries
- **Performance optimized**: Efficient storage and retrieval for large docsets

### 📊 **Query Interface**
- **Interactive SQL shell**: Explore the knowledge database interactively
- **Multiple output formats**: Table, JSON, CSV output options
- **Streaming results**: Handle large query results efficiently
- **Pipe support**: Integrate with shell scripts and workflows
- **Timeout controls**: Configurable query execution limits

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
# Install all CLI tools
go install github.com/jrossi/gismo/cmd/gismo@latest
go install github.com/jrossi/gismo/cmd/gismo-knowledge@latest
go install github.com/jrossi/gismo/cmd/gismo-query@latest

# Use as a library
go get github.com/jrossi/gismo
```

### Build from Source

```bash
git clone https://github.com/jrossi/gismo.git
cd gismo
go tool mage install
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

### CLI Tools

#### Hook Processing with `gismo`

The main CLI tool is `gismo` which can be used as a hook processor:

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

#### Knowledge Management with `gismo-knowledge`

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

#### Direct SQL Queries with `gismo-query`

Execute SQL queries against the knowledge database:

```bash
# List all docsets
gismo-query "SELECT * FROM docsets"

# Search for specific content
gismo-query "SELECT name, type, path FROM docset_content WHERE name LIKE '%http%' LIMIT 10"

# Get docset statistics
gismo-query "SELECT docset_id, COUNT(*) as count FROM docset_content GROUP BY docset_id"

# Interactive mode
gismo-query
# gismo> SELECT * FROM docsets;
# gismo> .tables
# gismo> .quit

# Stream large results
gismo-query --stream "SELECT * FROM docset_content"

# Output as JSON or CSV
gismo-query --format json "SELECT id, name, version FROM docsets"
gismo-query --format csv "SELECT * FROM docsets"

# Query from pipe
echo "SELECT COUNT(*) FROM docsets" | gismo-query
```

#### Knowledge Management Tools

Gismo includes comprehensive knowledge management capabilities:

- **`gismo-knowledge`**: Manage docsets, import documentation, and search knowledge base
- **`gismo-query`**: Execute SQL queries directly against the knowledge database
- **`gismo-server`**: Backend server component with gRPC API

#### Additional Command-Line Tools (Planned)

The project architecture supports additional command-line utilities in development:

- **`gismo-init`**: Set up gismo in Claude Code settings
- **`gismo-show`**: Configuration inspector and analysis tool
- **`gismo-registry`**: Package registry manager
- **`gismo-package`**: Package manager for gismo components

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