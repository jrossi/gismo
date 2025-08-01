---
title: "Action Handlers"
linkTitle: "Action Handlers"
weight: 25
description: >
  Extensible action handler architecture for Claude Code hooks
---

# Action Handlers

Gismo's action handler architecture provides an extensible, security-first approach to processing Claude Code hooks.
Instead of a monolithic linting system, handlers are specialized, configurable components that can be combined
to create powerful policy enforcement pipelines.

## Architecture Overview

The action handler system consists of three main components:

- **ActionHandler Interface**: Base interface implemented by all handlers
- **ActionHandlerRegistry**: Manages handler registration and execution ordering
- **ActionHandlerEngine**: Orchestrates handler execution while maintaining RuleEngine compatibility

### Priority-Based Execution

Handlers execute in priority order (higher numbers first) to ensure security-first processing:

1. **Priority 300**: File Access Control (Security)
2. **Priority 200**: Secret Detection (Security)
3. **Priority 150**: Regex Pattern Matching (Policy)
4. **Priority 100**: Code Linting (Quality)
5. **Priority 50**: Notifications (Logging)

## Built-in Handlers

### File Access Handler (Priority 300)

Controls file system access with configurable restrictions:

```go
fileHandler := handlers.NewFileAccessHandler()
engine.RegisterHandler(gismo.PreToolUseEvent, fileHandler)
```

**Default Restrictions:**
- **Read**: Blocks `.pem`, `.key`, `.p12`, `.pfx` certificate files
- **Write**: Blocks `/etc/`, `/usr/` system directories

**Configuration:**
```go
config := &handlers.FileAccessConfig{
    ReadRestrictions: []handlers.FileRestriction{
        {
            Pattern:     `\.pem$`,
            Description: "Block PEM certificate files",
            Action:      "block",
            Message:     "Reading PEM files is not allowed",
        },
    },
    WriteRestrictions: []handlers.FileRestriction{
        {
            Pattern:     `^/etc/`,
            Description: "Block system configuration",
            Action:      "block",
            Message:     "Writing to /etc/ is not allowed",
        },
    },
    BlockOnViolation: true,
}
handler := handlers.NewFileAccessHandlerWithConfig(config)
```

### Secret Detection Handler (Priority 200)

Prevents secrets in prompts and files using Gitleaks integration:

```go
secretHandler := handlers.NewSecretDetectionHandler()
engine.RegisterHandler(gismo.UserPromptSubmitEvent, secretHandler)
engine.RegisterHandler(gismo.PreToolUseEvent, secretHandler)
```

**Features:**
- Scans user prompts for API keys, passwords, tokens
- Blocks file writes containing secrets
- Override support with `secret_detect=override` pattern
- Configurable rule disabling

**Configuration:**
```go
config := &handlers.SecretDetectionConfig{
    EnablePromptScanning: true,
    EnableFileScanning:   true,
    AllowOverride:        true,
    BlockOnDetection:     true,
    DisabledRules:        []string{"generic-api-key"},
    MaxFileSize:          1024 * 1024, // 1MB
}
handler := handlers.NewSecretDetectionHandlerWithConfig(config)
```

### Regex Handler (Priority 150)

Pattern-based content filtering with configurable actions:

```go
regexHandler := handlers.NewRegexHandler()
engine.RegisterHandler(gismo.UserPromptSubmitEvent, regexHandler)
```

**Actions:**
- **block**: Prevent operation
- **warn**: Log warning but continue
- **log**: Silent logging
- **modify**: Replace matched content

**Configuration:**
```go
config := &handlers.RegexConfig{
    PromptRules: []handlers.RegexRule{
        {
            Name:        "no-passwords",
            Pattern:     `password\s*[:=]\s*["\']?([^"\'\s]+)`,
            Action:      "warn",
            Message:     "Potential password detected",
            Description: "Warns about password patterns",
            Enabled:     true,
        },
    },
    CaseSensitive: false,
}
handler := handlers.NewRegexHandlerWithConfig(config)
```

### Linting Handler (Priority 100)

Code quality enforcement with multi-language support:

```go
lintingHandler := handlers.NewLintingHandler()
engine.RegisterHandler(gismo.PreToolUseEvent, lintingHandler)
engine.RegisterHandler(gismo.PostToolUseEvent, lintingHandler)
```

**Features:**
- Go, Python, JavaScript, Markdown, JSON, Rust, Protobuf support
- Parallel linter execution
- Rule overrides per file
- Temporary file exclusion

### Notification Handler (Priority 50)

System notification processing with structured logging:

```go
notificationHandler := handlers.NewNotificationHandler()
engine.RegisterHandler(gismo.NotificationEvent, notificationHandler)
```

**Features:**
- Rule-based filtering
- Multiple output formats (structured, simple, JSON)
- Alert generation for critical events
- Custom message templates

## Creating Custom Handlers

### Basic Handler Implementation

```go
type MyHandler struct {
    *gismo.BaseActionHandler
    config *MyConfig
}

func NewMyHandler() *MyHandler {
    return &MyHandler{
        BaseActionHandler: gismo.NewBaseActionHandler("my-handler", 75),
        config:            &MyConfig{},
    }
}

func (h *MyHandler) ShouldHandle(ctx context.Context, event gismo.HookMessage) bool {
    // Determine if this handler should process the event
    return true
}

func (h *MyHandler) HandlePreToolUse(ctx context.Context, msg *gismo.PreToolUseMessage) (*gismo.HookResponse, error) {
    // Custom logic here
    return &gismo.HookResponse{Decision: "approve"}, nil
}
```

### Handler Registration

```go
engine := gismo.NewActionHandlerEngine()
myHandler := NewMyHandler()
engine.RegisterHandler(gismo.PreToolUseEvent, myHandler)
```

## Configuration Management

### Handler Rules

Handlers support configurable rules with pattern matching:

```go
type HandlerRule struct {
    Events              []gismo.HookEventName `json:"events"`
    ToolPattern         string                `json:"tool_pattern,omitempty"`
    FilePathPattern     string                `json:"file_path_pattern,omitempty"`
    NotificationTypePattern string            `json:"notification_type_pattern,omitempty"`
    Action              string                `json:"action"`
    Message             string                `json:"message,omitempty"`
}
```

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

## Use Case Examples

### 1. Secret Detection in Prompts

```go
engine := gismo.NewActionHandlerEngine()
secretHandler := handlers.NewSecretDetectionHandler()
engine.RegisterHandler(gismo.UserPromptSubmitEvent, secretHandler)
```

**Result**: User prompts containing API keys or passwords are blocked.

### 2. File Access Restrictions

```go
fileHandler := handlers.NewFileAccessHandler()
fileHandler.AddReadRestriction(handlers.FileRestriction{
    Pattern:     `\.key$`,
    Description: "Block private key files",
    Action:      "block",
    Message:     "Private key files cannot be read",
})
engine.RegisterHandler(gismo.PreToolUseEvent, fileHandler)
```

**Result**: Attempts to read `.key` files are blocked.

### 3. Content Pattern Filtering

```go
regexHandler := handlers.NewRegexHandler()
regexHandler.AddPromptRule(handlers.RegexRule{
    Name:        "social-security",
    Pattern:     `\b\d{3}-\d{2}-\d{4}\b`,
    Action:      "block",
    Message:     "Social Security Numbers are not allowed",
    Description: "Blocks SSN patterns",
    Enabled:     true,
})
engine.RegisterHandler(gismo.UserPromptSubmitEvent, regexHandler)
```

**Result**: Prompts containing SSN patterns are blocked.

## Best Practices

1. **Security First**: Register security handlers with highest priorities
2. **Specific Patterns**: Use precise regex patterns to avoid false positives
3. **Graceful Degradation**: Handle errors without breaking the pipeline
4. **Configuration**: Make handlers configurable for different environments
5. **Testing**: Test handlers independently and in combination
6. **Documentation**: Document custom handler behavior and configuration

## Performance Considerations

- Handlers execute in priority order, stopping at first blocking response
- Use efficient regex patterns to minimize performance impact
- Consider caching for expensive operations
- Monitor handler execution times in production

## Troubleshooting

### Common Issues

- **Handler Not Executing**: Check event type registration and ShouldHandle logic
- **Wrong Priority**: Verify handler priorities are set correctly
- **Pattern Not Matching**: Test regex patterns independently
- **Configuration Ignored**: Ensure config is applied after handler creation

### Debug Mode

Enable debug logging to trace handler execution:

```go
// Log handler execution details
fmt.Fprintf(os.Stderr, "Handler %s processing %s event\n", handler.Name(), event.EventName())
```