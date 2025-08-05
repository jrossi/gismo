# Gismo Project Overview

## Purpose
Gismo is an extensible Claude Code hooks system with security-first action handlers and comprehensive linting capabilities. It provides intelligent code validation, formatting checks, and quality assurance for various programming languages through a pluggable handler architecture.

## Key Features
- **Extensible action handler system** with priority-based execution
- **Security handlers run first** (highest priority)
- **Multi-language linting** with intelligent fallback strategies
- **Code search with vector embeddings** using DuckDB (cross-platform)
- **Rule-based pattern matching** with hierarchical configuration
- **Caching system** for performance optimization
- **Integration with Claude Code** through all hook types
- **Package registry support** for sharing linters and tools

## Tech Stack
- **Language**: Go 1.23+
- **Build System**: Mage (make-like build tool in Go)
- **Database**: DuckDB with VSS extension for vector search
- **JSON**: github.com/goccy/go-json for 2-3x faster parsing
- **Testing**: Go's built-in testing with race detection
- **Linting**: golangci-lint with 30+ linters enabled

## Architecture
```
cmd/                    # CLI applications
├── gismo/             # Main binary
├── gismo-init/        # Setup tool
├── gismo-show/        # Configuration inspector
├── gismo-registry/    # Package registry manager
└── gismo-package/     # Package manager

pkg/                   # Core library code
├── engine/           # Rule engines and core logic
├── linters/          # Language-specific linters
├── handlers/         # Action handlers for hooks
├── database/         # DuckDB-based code search
├── toolcache/        # Caching utilities
└── version/          # Version parsing

tests/                # Centralized test organization
├── integration/      # Cross-package integration tests
├── e2e/             # End-to-end binary tests
├── fixtures/        # Shared test data
└── utils/           # Test utility functions
```

## Database Features
- **Cross-platform support**: Windows, macOS, Linux via DuckDB
- **Vector search**: Similarity search using embeddings
- **Project isolation**: Separate code contexts per project
- **No external dependencies**: Embedded database approach