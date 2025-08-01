# Action Handler Architecture

## Overview

This document describes the new extensible action handler architecture that replaces the previous linter-focused design. The new system allows for tool calling at all stages of Claude Code interaction points with configurable policies and rules.

## Architecture Components

### Core Components

1. **ActionHandler Interface** (`action_handler.go`)
   - Base interface for all action handlers
   - Provides priority-based execution ordering
   - Supports conditional execution via `ShouldHandle()`

2. **ActionHandlerRegistry** (`action_handler.go`)
   - Manages registration and retrieval of handlers
   - Sorts handlers by priority (higher numbers execute first)
   - Provides filtered handler lists based on event criteria

3. **ActionHandlerEngine** (`action_handler_engine.go`)
   - Implements the `RuleEngine` interface
   - Orchestrates handler execution for all hook types
   - Maintains backward compatibility with existing code

### Handler Types

The system supports specialized handlers for each hook type:

- `PreToolUseHandler` - Controls tool execution permissions
- `PostToolUseHandler` - Processes tool output and side effects
- `UserPromptSubmitHandler` - Validates and processes user prompts
- `NotificationHandler` - Handles system notifications
- `SessionStartHandler` - Processes session initialization
- `StopHandler` - Handles main agent completion
- `SubagentStopHandler` - Processes subagent completion
- `PreCompactHandler` - Manages context compression events

## Handler Implementations

### 1. LintingHandler (`handlers/linting_handler.go`)
- **Priority**: 100 (medium)
- **Purpose**: Code quality and style enforcement
- **Events**: PreToolUse, PostToolUse
- **Features**:
  - Parallel linter execution
  - Rule overrides per file
  - Test file checking
  - Temporary file exclusion

### 2. SecretDetectionHandler (`handlers/secret_detection_handler.go`)
- **Priority**: 200 (high)
- **Purpose**: Prevent secret leakage in prompts and files
- **Events**: UserPromptSubmit, PreToolUse
- **Features**:
  - Gitleaks integration
  - Override patterns (`secret_detect=override`)
  - Configurable file size limits
  - Custom rule disabling

### 3. FileAccessHandler (`handlers/file_access_handler.go`)
- **Priority**: 300 (highest)
- **Purpose**: File system access control
- **Events**: PreToolUse
- **Features**:
  - Read/write restrictions
  - Regex pattern matching
  - Path normalization
  - Action-based responses (block/warn/log)

### 4. RegexHandler (`handlers/regex_handler.go`)
- **Priority**: 150 (medium)
- **Purpose**: Pattern-based content filtering
- **Events**: UserPromptSubmit, PreToolUse
- **Features**:
  - Configurable regex rules
  - Multiple actions (block/warn/log/modify)
  - Case sensitivity options
  - Rule management (enable/disable)

### 5. NotificationHandler (`handlers/notification_handler.go`)
- **Priority**: 50 (lowest)
- **Purpose**: System notification processing
- **Events**: Notification
- **Features**:
  - Structured logging
  - Alert formatting
  - Rule-based filtering
  - Multiple output formats

## Use Case Implementation

The architecture successfully implements all 5 specified use cases:

### Use Case 1: Secret Detection in UserPromptSubmit
- **Handler**: SecretDetectionHandler
- **Behavior**: Scans user prompts for secrets, blocks if detected
- **Override**: `secret_detect=override` pattern support

### Use Case 2: Regex Matching in UserPromptSubmit (Non-blocking)
- **Handler**: RegexHandler
- **Behavior**: Matches patterns in prompts, logs without blocking
- **Configuration**: Custom regex rules with log-only actions

### Use Case 3: PreToolUse Read(*) Blocks PEM Files
- **Handler**: FileAccessHandler
- **Behavior**: Blocks reading of certificate files (`.pem`, `.key`, `.p12`)
- **Security**: Protects against secret exposure

### Use Case 4: PreToolUse Write(*) Blocks Predefined Paths
- **Handler**: FileAccessHandler
- **Behavior**: Blocks writing to system directories (`/etc/`, `/usr/`)
- **Flexibility**: Configurable path patterns and actions

### Use Case 5: Notification Handling
- **Handler**: NotificationHandler
- **Behavior**: Processes and logs system notifications
- **Features**: Rule-based filtering and alert generation

## Priority System

Handlers execute in priority order (highest first) to ensure security-first processing:

1. **Priority 300**: File Access (Security)
2. **Priority 200**: Secret Detection (Security)
3. **Priority 150**: Regex Matching (Policy)
4. **Priority 100**: Linting (Quality)
5. **Priority 50**: Notifications (Logging)

## Configuration

### Handler Rules
Each handler supports configurable rules with:
- Pattern matching (regex)
- Action types (block/warn/log)
- Enable/disable flags
- Custom messages
- Event filtering

### Example Configuration
```json
{
  "handlers": [
    {
      "name": "file-access",
      "type": "file_access",
      "priority": 300,
      "enabled": true,
      "rules": [
        {
          "events": ["PreToolUse"],
          "tool_pattern": "Read",
          "file_path_pattern": "\\.pem$",
          "action": "deny",
          "message": "PEM files cannot be read"
        }
      ]
    }
  ]
}
```

## Benefits

### Extensibility
- Easy to add new handler types
- Pluggable architecture
- No core code changes required

### Security
- Security handlers run first
- Multiple layers of protection
- Configurable enforcement levels

### Flexibility
- Rule-based configuration
- Priority-based execution
- Event-specific handling

### Maintainability
- Separation of concerns
- Clear handler responsibilities
- Testable components

## Migration Path

The new architecture maintains backward compatibility:
1. Existing `LintingRuleEngine` continues to work
2. New `ActionHandlerEngine` provides enhanced functionality
3. Gradual migration supported
4. Drop-in replacement available

## Demo

See `examples/action_handler_demo.go` for a complete demonstration of all use cases and handler functionality.

## Future Enhancements

Potential extensions to the architecture:

1. **Configuration Loading**: JSON/YAML config file support
2. **Handler Plugins**: Dynamic handler loading
3. **Metrics Collection**: Handler performance monitoring
4. **Rule Templates**: Predefined rule sets for common scenarios
5. **External Integrations**: Webhook and API notifications