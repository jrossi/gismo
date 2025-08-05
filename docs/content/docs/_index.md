---
title: "Gismo"
linkTitle: "Home"
type: "docs"
---

# Gismo

Real-time feedback engine that optimizes Claude Code performance through immediate validation, course correction,
and intelligent knowledge management. Prevents AI drift and failure loops by providing instant feedback as files
are written and contextual knowledge access during development.

## Overview

Gismo creates software-defined feedback loops that dramatically improve Claude Code's effectiveness. By providing
real-time validation, linting, security enforcement, and contextual knowledge access the moment Claude Code
attempts to write files, Gismo prevents the AI from pursuing invalid paths, eliminates cascading failures, and
ensures valid output from the first attempt. This immediate feedback approach combined with intelligent knowledge
management makes Claude Code significantly more performant and reliable.

## Key Features

- **Real-Time Validation**: Files analyzed instantly as Claude Code attempts to write them
- **Course Correction**: Prevents Claude Code from pursuing invalid implementation paths
- **Software-Defined Feedback Loops**: Immediate analysis and correction during development
- **Multi-language Linting**: Go, Python, JavaScript, Markdown, JSON with instant feedback
- **Security Enforcement**: Block dangerous operations and secret exposure before they happen
- **Context Management**: Maintains awareness of project standards and coding patterns
- **Knowledge Management**: Import and search docsets, documentation, and code knowledge
- **Vector Search**: Semantic similarity search using DuckDB with cross-platform support
- **SQL Query Interface**: Direct database access for advanced knowledge queries
- **High Performance**: Sub-microsecond processing for immediate feedback delivery
- **Extensible Architecture**: Pluggable handlers for custom validation and policy enforcement

## Quick Start

Get started with Gismo in minutes:

1. **[Install Gismo](installation/)** - Multiple installation options
2. **[Quick Start Guide](quickstart/)** - Basic usage examples
3. **[Configuration](configuration/)** - Set up your linting rules

## Use Cases

- **Claude Code Optimization**: Eliminate failure loops and AI drift through immediate feedback
- **Real-Time Course Correction**: Prevent Claude Code from pursuing invalid implementation paths
- **Instant Validation**: Catch syntax errors, style violations, and security issues immediately
- **Performance Enhancement**: Reduce development time by eliminating "write-test-fix" cycles
- **Context Preservation**: Maintain project standards and coding patterns throughout development
- **Security Policy Enforcement**: Block dangerous operations before they execute
- **Quality Assurance**: Ensure valid, high-quality code from the first attempt
- **Team Productivity**: Accelerate development with instant feedback and validation

## Performance

Gismo is built for speed:
- Message parsing: ~700ns per message
- Rule evaluation: <1ns for simple rules
- Go linting: ~100ms enhanced / ~4μs fallback
- Full pipeline: ~22ns handler processing

[Get Started](installation/){.btn .btn-primary .btn-lg}
[View on GitHub](https://github.com/jrossi/gismo){.btn .btn-secondary .btn-lg}
<!-- Re-trigger deployment -->