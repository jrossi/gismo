# Gismo Package Management System - Usage Examples

This document provides comprehensive examples of how to use the gismo package management system.

## Overview

The gismo package management system provides a complete solution for distributing and managing Claude Code tools, prompts, and configurations. It consists of three main components:

- **Registry Management** (`gismo registry`) - Manage package repositories
- **Package Management** (`gismo package`) - Install, remove, and manage packages
- **Comprehensive CLI** - Full-featured command-line interface

## Quick Start

### 1. Add a Package Registry

```bash
# Add a GitHub repository as a package registry
gismo registry add --global github.com/user/claude-tools

# Add with custom name
gismo registry add --global github.com/company/internal-tools --name company-tools

# List configured registries
gismo registry list
```

### 2. Search for Packages

```bash
# Search all registries
gismo package search

# Search for specific pattern
gismo package search linter
gismo package search "code review"

# Search by author
gismo package search "author:gismo-team"
```

### 3. Install Packages

```bash
# Install a package globally
gismo package install --global claude-dev-tools

# Install for current project only
gismo package install --project markdown-tools

# Install with dependency resolution
gismo package install advanced-linters
# Output:
# 🔍 Resolving dependencies for 'advanced-linters'...
# 📋 Install plan (3 packages):
#   📦 advanced-linters (main package)
#     └─ base-linters (dependency)
#       └─ common-utils (dependency)
```

### 4. Manage Installed Packages

```bash
# List installed packages
gismo package list

# Update all packages
gismo package update

# Update specific package
gismo package update claude-dev-tools

# Remove package
gismo package remove --global old-package
```

## Sample Registry: Claude Dev Tools

We've created a comprehensive sample registry (`examples/sample-registry`) that demonstrates all package types and features:

### Package Contents

**📦 claude-dev-tools v1.0**
- **Description**: Essential development tools and prompts for Claude Code users
- **Author**: Gismo Team
- **Components**: 12 total components across 6 categories

#### Prompts (4 components)
1. **code-review** - Comprehensive code review prompt with best practices
2. **refactor** - Smart refactoring prompt for code improvements  
3. **debug** - Systematic debugging prompt for issue resolution
4. **test-gen** - Intelligent test generation for comprehensive coverage

#### Tools (2 Go binaries)
1. **json-formatter** - Fast JSON formatting and validation tool
2. **git-stats** - Git repository statistics and analysis

#### Scripts (2 shell scripts)  
1. **setup-dev** - Development environment setup automation
2. **quick-commit** - Smart git commit with automatic message generation

#### Linters (1 custom linter)
1. **markdown-lint** - Markdown linting with style and link checking

#### Configs (1 configuration file)
1. **dev-settings** - Recommended development settings for Claude Code

#### Schemas (1 JSON schema)
1. **project-config** - JSON schema for project configuration files

### Package Features Demonstrated

- **Version Requirements**: Requires gismo v0.3.0+
- **Component Types**: All 6 supported component types
- **Build Commands**: Go binary compilation instructions
- **Executable Permissions**: Proper script and binary permissions
- **File Extensions**: Linter file extension specifications
- **Hooks**: Post-install and pre-remove lifecycle hooks
- **Configuration Schema**: JSON schema validation for configs

## Component Types Reference

### 1. Commands/Prompts (`command`)
- **Purpose**: Claude Code prompt files
- **Target**: `prompts/` directory
- **Use Case**: Reusable prompts for code review, debugging, etc.

```json
{
  "source": "prompts/code-review.md",
  "target": "prompts/code-review.md", 
  "type": "command",
  "description": "Comprehensive code review prompt"
}
```

### 2. Go Binaries (`go-binary`)
- **Purpose**: Compiled Go tools and utilities
- **Target**: `bin/` directory
- **Build**: Automatic compilation from source
- **Executable**: True (automatically set)

```json
{
  "source": "tools/json-formatter.go",
  "target": "bin/json-formatter",
  "type": "go-binary", 
  "build": "go build -o json-formatter tools/json-formatter.go",
  "executable": true
}
```

### 3. Shell Scripts (`shell-script`)
- **Purpose**: Automation and utility scripts
- **Target**: `scripts/` directory
- **Executable**: Should be true for scripts

```json
{
  "source": "scripts/setup-dev.sh",
  "target": "scripts/setup-dev.sh",
  "type": "shell-script",
  "executable": true
}
```

### 4. Custom Linters (`go-linter`)
- **Purpose**: Code quality and style checking tools
- **Target**: `linters/` directory
- **Extensions**: File types the linter handles
- **Priority**: Execution order (lower = earlier)

```json
{
  "source": "linters/markdown-lint.go", 
  "target": "linters/markdown-lint",
  "type": "go-linter",
  "build": "go build -o markdown-lint linters/markdown-lint.go",
  "extensions": [".md", ".markdown"],
  "priority": 10
}
```

### 5. Configuration Files (`config`)
- **Purpose**: Settings and configuration templates
- **Target**: Root or `configs/` directory
- **Schema**: Optional JSON schema validation

```json
{
  "source": "configs/dev-settings.json",
  "target": "dev-settings.json", 
  "type": "config"
}
```

### 6. JSON Schemas (`schema`)
- **Purpose**: Validation schemas for configuration files
- **Target**: `schemas/` directory
- **Validation**: Used by other components

```json
{
  "source": "schemas/project-config.schema.json",
  "target": "schemas/project-config.schema.json",
  "type": "schema"
}
```

## Advanced Features

### Dependency Management

Packages can specify dependencies that are automatically resolved:

```json
{
  "dependencies": [
    "github.com/user/base-tools@v1.0.0",
    "github.com/user/common-utils@v2.1.0"
  ]
}
```

Features:
- **Automatic Resolution**: Dependencies installed automatically
- **Version Constraints**: Semantic versioning support
- **Circular Detection**: Prevents dependency loops
- **Conflict Resolution**: Handles version conflicts gracefully

### Version Management

Full semantic versioning support:

```bash
# Install specific version
gismo package install tool@v1.2.3

# Version constraints in manifests
"dependencies": [
  "github.com/user/lib@>=v1.0.0,<v2.0.0"
]
```

### Package Validation

Comprehensive validation during installation:
- **Manifest Structure**: JSON schema validation
- **File Existence**: All referenced files must exist
- **Component Types**: Valid type and configuration
- **Dependencies**: Proper dependency format
- **Integrity**: SHA256 checksums for security

### Lifecycle Hooks

Packages can define installation and removal hooks:

```json
{
  "hooks": {
    "postInstall": "echo 'Package installed! Run setup command.'",
    "preRemove": "echo 'Cleaning up package data...'"
  }
}
```

## Configuration Management

### Scopes

Packages can be installed at different scopes:

- **Global** (`--global`): Available to all projects (`~/.claude/`)
- **Project** (`--project`): Available to current project only (`./.claude/`)
- **Both** (default): Available at both levels

### Configuration Files

The system uses hierarchical configuration:

1. **Global**: `~/.claude/gismo.json`
2. **Project**: `./.claude/gismo.json` 
3. **Local**: `./.claude/gismo.local.json`

Later files override earlier ones for maximum flexibility.

## Best Practices

### Package Development

1. **Clear Naming**: Use descriptive, unique package names
2. **Semantic Versioning**: Follow semver for version numbers
3. **Complete Manifests**: Include all metadata (description, author, etc.)
4. **File Organization**: Organize components logically
5. **Documentation**: Include clear component descriptions

### Component Design

1. **Single Responsibility**: Each component should do one thing well
2. **Reusability**: Design for reuse across projects
3. **Dependencies**: Minimize external dependencies
4. **Error Handling**: Provide clear error messages
5. **Performance**: Optimize for fast execution

### Registry Management

1. **Organization**: Group related packages in registries
2. **Access Control**: Use appropriate repository permissions
3. **Versioning**: Tag releases for stable versions
4. **Documentation**: Maintain package documentation
5. **Testing**: Test packages before publishing

## Troubleshooting

### Common Issues

**Registry Not Found**
```bash
Error: failed to clone registry: repository not found
```
- Check repository URL and permissions
- Ensure git credentials are configured
- Verify repository exists and is accessible

**Package Installation Failed**
```bash
Error: package validation failed: component file not found
```
- Check manifest.json for correct file paths
- Ensure all referenced files exist in repository
- Verify component types and configurations

**Dependency Conflicts**
```bash
Error: dependency conflicts detected: package A requires v1.0.0 but v2.0.0 is required
```
- Review dependency versions
- Update packages to compatible versions
- Use `--force` to override (not recommended)

### Debug Mode

Use `--debug` flag for detailed operation logs:

```bash
gismo registry add --debug --global github.com/user/repo
gismo package install --debug my-package
```

### Getting Help

```bash
# Command help
gismo --help
gismo registry --help
gismo package --help

# Show configuration
gismo show config

# List available commands
gismo show setup
```

## Summary

The gismo package management system provides:

✅ **Complete Package Lifecycle** - From discovery to installation to removal
✅ **Dependency Resolution** - Automatic dependency management with conflict detection  
✅ **Multiple Component Types** - Support for prompts, tools, scripts, configs, and more
✅ **Version Management** - Full semantic versioning with constraints
✅ **Validation & Security** - Comprehensive validation and integrity checking
✅ **Flexible Configuration** - Global, project, and local configuration scopes
✅ **Rich CLI Interface** - Intuitive commands with comprehensive help

The system is production-ready and provides enterprise-grade package management for Claude Code environments.