---
title: "Library API"
linkTitle: "Library"
weight: 60
description: >
  Go library API documentation and integration examples
---

# Library API

Gismo provides a comprehensive Go library for integrating hook processing and linting capabilities
into your applications.

## Installation

```bash
go get github.com/jrossi/gismo
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    json "github.com/goccy/go-json"
    "github.com/jrossi/gismo/pkg/engine"
)

func main() {
    // Create API with default configuration
    api := engine.New()

    // Create a hook message
    msg := &engine.PreToolUseMessage{
        BaseHookMessage: engine.BaseHookMessage{
            SessionID:     "demo-session",
            HookEventName: engine.PreToolUseEvent,
        },
        ToolName: "Write",
        ToolInput: map[string]json.RawMessage{
            "file_path": json.RawMessage(`"main.go"`),
            "content":   json.RawMessage(`"package main\n"`),
        },
    }

    result, err := api.ProcessMessage(context.Background(), msg)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Hook processed: %+v\n", result)
}
```

### With Custom Rule Engine

```go
package main

import (
    "context"
    "log"

    json "github.com/goccy/go-json"
    "github.com/jrossi/gismo/pkg/engine"
)

func main() {
    // Create custom rule engine
    ruleEngine := engine.NewLintingRuleEngine()

    // Create API with custom rule engine
    api := engine.NewWithRuleEngine(ruleEngine)

    // Process hook message
    msg := &engine.PreToolUseMessage{
        BaseHookMessage: engine.BaseHookMessage{
            SessionID:     "demo-session",
            HookEventName: engine.PreToolUseEvent,
        },
        ToolName: "Edit",
        ToolInput: map[string]json.RawMessage{
            "file_path": json.RawMessage(`"main.go"`),
        },
    }

    result, err := api.ProcessMessage(context.Background(), msg)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Processing result: %+v", result)
}
```

## Core API

### API Creation

```go
// Create with default configuration
api := engine.New()

// Create with custom rule engine
ruleEngine := engine.NewLintingRuleEngine()
api := engine.NewWithRuleEngine(ruleEngine)

// Create with configuration
config := engine.NewAppConfig()
api := engine.NewWithConfig(config)

// Use builder pattern
api := engine.NewBuilder().
    WithRuleEngine(ruleEngine).
    Build()
```

### Hook Processing

```go
// Process hook message
result, err := api.ProcessMessage(ctx, message)

// Process from stdin
result, err := api.ProcessStdin(ctx)

// Parse hook messages
message, err := api.ParseHookMessage(messageJSON)

// Create hook response
response := &engine.HookResponse{
    Decision: "allow",
    Reason:   "No issues found",
}
responseJSON, err := api.MarshalHookResponse(response)
```

## Configuration API

### Loading Configuration

```go
// Create new configuration
config := engine.NewAppConfig()

// Load configuration from config loader
loader := engine.NewConfigLoader()
config, err := loader.LoadConfig("gismo.json")

// Load with search paths
config, err := loader.LoadConfigWithPaths([]string{
    ".claude/gismo.json",
    "~/.claude/gismo.json",
})
```

### Configuration Structure

```go
type AppConfig struct {
    Linters  map[string]json.RawMessage `json:"linters"`
    Parallel ParallelConfig             `json:"parallel"`
    Registry RegistryConfig             `json:"registry"`
}

type ParallelConfig struct {
    MaxWorkers      int  `json:"max_workers"`
    DisableParallel bool `json:"disable_parallel"`
}

type LinterConfig struct {
    Enabled *bool          `json:"enabled,omitempty"`
    Config  map[string]any `json:"config,omitempty"`
}
```

## Rule Engines

### Built-in Engines

```go
// Base rule engine (allows everything)
engine := engine.NewBaseRuleEngine()

// Linting rule engine
lintingEngine := engine.NewLintingRuleEngine()

// Action handler engine
actionEngine := engine.NewActionHandlerEngine()

// Composite engine (multiple engines)
composite := engine.NewCompositeRuleEngine()
composite.AddEngine(lintingEngine)
composite.AddEngine(actionEngine)
```

### Custom Rule Engine

```go
type MyRuleEngine struct {
    config MyConfig
}

func (e *MyRuleEngine) EvaluatePreToolUse(ctx context.Context, msg *engine.PreToolUseMessage) (*engine.HookResponse, error) {
    // Custom processing logic
    return &engine.HookResponse{
        Decision: "allow",
        Reason:   "Custom processing completed",
    }, nil
}

// Implement other evaluation methods...

// Use custom engine
api := engine.NewWithRuleEngine(&MyRuleEngine{})
```

## Message Types

### Hook Message Structure

```go
type BaseHookMessage struct {
    SessionID     string         `json:"session_id"`
    HookEventName HookEventName  `json:"hook_event_name"`
}

type PreToolUseMessage struct {
    BaseHookMessage
    ToolName  string                     `json:"tool_name"`
    ToolInput map[string]json.RawMessage `json:"tool_input"`
}

type PostToolUseMessage struct {
    BaseHookMessage
    ToolName   string                     `json:"tool_name"`
    ToolInput  map[string]json.RawMessage `json:"tool_input"`
    ToolOutput json.RawMessage            `json:"tool_output"`
}
```

### Supported Hook Types

```go
const (
    PreToolUseEvent      HookEventName = "PreToolUse"
    PostToolUseEvent     HookEventName = "PostToolUse"
    NotificationEvent    HookEventName = "Notification"
    StopEvent           HookEventName = "Stop"
    SubagentStopEvent   HookEventName = "SubagentStop"
    PreCompactEvent     HookEventName = "PreCompact"
    UserPromptSubmitEvent HookEventName = "UserPromptSubmit"
    SessionStartEvent   HookEventName = "SessionStart"
)
```

### Response Structure

```go
type HookResponse struct {
    Decision string                 `json:"decision"`
    Reason   string                 `json:"reason,omitempty"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}

const (
    ExitSuccess  ExitCode = 0
    ExitBlocking ExitCode = 2
)
```

## Action Handlers

### Built-in Handlers

```go
// File access restriction handler
fileHandler := handlers.NewFileAccessHandler()

// Secret detection handler
secretHandler := handlers.NewSecretDetectionHandler()

// Regex pattern matching handler
regexHandler := handlers.NewRegexHandler()

// Linting integration handler
lintingHandler := handlers.NewLintingHandler()

// Notification handler
notificationHandler := handlers.NewNotificationHandler()
```

### Registering Handlers

```go
// Create action handler engine
actionEngine := engine.NewActionHandlerEngine()

// Register handlers with different hook types
actionEngine.RegisterHandler(engine.PreToolUseEvent, fileHandler)
actionEngine.RegisterHandler(engine.UserPromptSubmitEvent, secretHandler)
actionEngine.RegisterHandler(engine.NotificationEvent, notificationHandler)
```

### Custom Action Handler

```go
type MyHandler struct {
    engine.BaseActionHandler
}

func (h *MyHandler) HandlePreToolUse(ctx context.Context, msg *engine.PreToolUseMessage) (*engine.HookResponse, error) {
    // Custom logic here
    return &engine.HookResponse{
        Decision: "allow",
        Reason:   "Custom handler approved",
    }, nil
}

// Register custom handler
actionEngine.RegisterHandler(engine.PreToolUseEvent, &MyHandler{})
```

## Advanced Usage

### Context and Cancellation

```go
// Process with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := api.ProcessMessage(ctx, message)

// Process with cancellation
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(10 * time.Second)
    cancel() // Cancel after 10 seconds
}()

result, err := api.ProcessMessage(ctx, message)
```

### Error Handling

```go
result, err := api.ProcessMessage(ctx, message)
if err != nil {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        log.Println("Processing timed out")
    case errors.Is(err, context.Canceled):
        log.Println("Processing was canceled")
    default:
        log.Printf("Unexpected error: %v", err)
    }
}

// Check response for blocking
if result != nil && result.ExitCode == engine.ExitBlocking {
    log.Printf("Operation blocked: %s", result.Reason)
}
```

## Integration Examples

### HTTP Server

```go
package main

import (
    "encoding/json"
    "net/http"
    "log"

    "github.com/jrossi/gismo/pkg/engine"
)

func main() {
    api := engine.New()

    http.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
        decoder := json.NewDecoder(r.Body)
        var rawMsg json.RawMessage
        if err := decoder.Decode(&rawMsg); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        msg, err := api.ParseHookMessage(rawMsg)
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        result, err := api.ProcessMessage(r.Context(), msg)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(result)
    })

    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### CLI Tool

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "os"

    "github.com/jrossi/gismo/pkg/engine"
)

func main() {
    var (
        configFile = flag.String("config", "", "Configuration file")
    )
    flag.Parse()

    var api *engine.API
    if *configFile != "" {
        loader := engine.NewConfigLoader()
        config, err := loader.LoadConfig(*configFile)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
            os.Exit(1)
        }
        api = engine.NewWithConfig(config)
    } else {
        api = engine.New()
    }

    result, err := api.ProcessStdin(context.Background())
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    if result.ExitCode == engine.ExitBlocking {
        os.Exit(int(result.ExitCode))
    }
}
```

## Testing

### Unit Tests

```go
package main

import (
    "context"
    "testing"

    json "github.com/goccy/go-json"
    "github.com/jrossi/gismo/pkg/engine"
)

func TestAPIProcessing(t *testing.T) {
    api := engine.New()

    msg := &engine.PreToolUseMessage{
        BaseHookMessage: engine.BaseHookMessage{
            SessionID:     "test-session",
            HookEventName: engine.PreToolUseEvent,
        },
        ToolName: "Write",
        ToolInput: map[string]json.RawMessage{
            "file_path": json.RawMessage(`"test.go"`),
        },
    }

    result, err := api.ProcessMessage(context.Background(), msg)
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }

    if result.ExitCode == engine.ExitBlocking {
        t.Errorf("Expected success, got blocking: %s", result.Reason)
    }
}
```

### Benchmarks

```go
func BenchmarkAPIProcessing(b *testing.B) {
    api := engine.New()
    msg := &engine.PreToolUseMessage{
        BaseHookMessage: engine.BaseHookMessage{
            SessionID:     "bench-session",
            HookEventName: engine.PreToolUseEvent,
        },
        ToolName: "Read",
        ToolInput: map[string]json.RawMessage{
            "file_path": json.RawMessage(`"test.go"`),
        },
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := api.ProcessMessage(context.Background(), msg)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Related Documentation

- [Configuration Guide](/docs/configuration/) - Configuration options
- [CLI Reference](/docs/cli/) - Command-line interface
- [Action Handlers](/docs/action-handlers/) - Security and validation handlers
- [Linter Documentation](/docs/linters/) - Language-specific linting