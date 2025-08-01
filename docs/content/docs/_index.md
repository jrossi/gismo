---
title: "Gismo"
linkTitle: "Home"
type: "docs"
---

# Gismo

High-performance Go library and CLI tool providing an extensible action handler architecture for Claude Code hooks.
Features security-first policy enforcement, configurable rule engines, and comprehensive protection across all interaction points.

## Overview

Gismo provides an extensible action handler system that processes Claude Code hooks with configurable policies
and security-first design. The new architecture supports custom handlers for all hook types with priority-based
execution, enabling fine-grained control over tool calling at every interaction point.

## Key Features

- **Extensible Action Handlers**: Pluggable handler architecture for all hook types
- **Security-First Design**: Priority-based execution with security handlers running first
- **Multi-language linting**: Go, Python, JavaScript, Markdown, and JSON
- **File Access Control**: Block reading certificates, restrict system directory writes
- **Secret Detection**: Prevent secrets in prompts and files using Gitleaks integration
- **High performance**: Sub-microsecond message parsing with optimized execution
- **Flexible configuration**: Hierarchical configuration with pattern-based overrides
- **Hook integration**: Full Claude Code hook lifecycle support

## Quick Start

Get started with Gismo in minutes:

1. **[Install Gismo](installation/)** - Multiple installation options
2. **[Quick Start Guide](quickstart/)** - Basic usage examples
3. **[Configuration](configuration/)** - Set up your linting rules

## Use Cases

- **Security Policy Enforcement**: Block dangerous file access, detect secrets in prompts
- **Claude Code Hook Processing**: Validate code changes before and after tool execution
- **Custom Rule Implementation**: Create handlers for specific organizational policies
- **Multi-layered Protection**: Combine file access control, secret detection, and content filtering
- **CI/CD Integration**: Automated code quality checks in build pipelines
- **Development Workflows**: Real-time linting and security checks during development
- **Team Standards**: Enforce consistent code quality and security across teams

## Performance

Gismo is built for speed:
- Message parsing: ~700ns per message
- Rule evaluation: <1ns for simple rules
- Go linting: ~100ms enhanced / ~4μs fallback
- Full pipeline: ~22ns handler processing

[Get Started](installation/){.btn .btn-primary .btn-lg}
[View on GitHub](https://github.com/jrossi/gismo){.btn .btn-secondary .btn-lg}
<!-- Re-trigger deployment -->