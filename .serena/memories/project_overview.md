# Gismo Project Overview

## Purpose
Gismo is a Claude Code Hook System that provides intelligent linting and code analysis for various programming languages and file types. It acts as a middleware layer between Claude Code and external linting tools, providing standardized feedback on code quality, formatting, and style issues.

## Key Features
- Multi-language support (Go, JavaScript, Python, Markdown, Protobuf, Rust, etc.)
- Intelligent tool detection and fallback strategies
- Rule-based configuration system with pattern matching
- Caching system for performance optimization
- Integration with Claude Code through hooks
- Pre-tool and post-tool validation workflows

## Tech Stack
- **Language**: Go 1.23.2
- **Build System**: Make
- **Testing**: Go's built-in testing framework with race detection
- **Dependencies**: Minimal external dependencies (see go.mod for details)
- **Linting Tools**: golangci-lint, gofmt, staticcheck, and various language-specific linters

## Architecture
- Command-line tools in `cmd/` directory
- Core logic in root package
- Language-specific linters in `linters/` subdirectories
- Shared configuration and utility code
- Integration tests in `e2e_test/` directory