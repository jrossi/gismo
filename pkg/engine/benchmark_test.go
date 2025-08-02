package engine

import (
	"context"
	"testing"
	"time"
)

// Benchmark the entire message processing pipeline
func BenchmarkHandler_ProcessMessage(b *testing.B) {
	engine := NewBaseRuleEngine()
	handler := NewHandler(engine)
	ctx := context.Background()

	msg := &PreToolUseMessage{
		BaseHookMessage: BaseHookMessage{
			SessionID:      "bench-session",
			TranscriptPath: "/path/to/transcript.json",
			HookEventName:  PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "/test/file.txt",
			"content":   "benchmark content",
		}),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := handler.ProcessMessage(ctx, msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark rule engine evaluation
func BenchmarkRuleEngine_EvaluatePreToolUse(b *testing.B) {
	engine := NewBaseRuleEngine()
	ctx := context.Background()

	msg := &PreToolUseMessage{
		BaseHookMessage: BaseHookMessage{
			SessionID:      "bench-session",
			TranscriptPath: "/path/to/transcript.json",
			HookEventName:  PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "/test/file.txt",
			"content":   "benchmark content",
		}),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark composite rule engine with multiple engines
func BenchmarkCompositeRuleEngine(b *testing.B) {
	// Create multiple engines
	engines := make([]RuleEngine, 5)
	for i := range engines {
		engines[i] = NewBaseRuleEngine()
	}

	composite := NewCompositeRuleEngine(engines...)
	ctx := context.Background()

	msg := &PreToolUseMessage{
		BaseHookMessage: BaseHookMessage{
			SessionID:      "bench-session",
			TranscriptPath: "/path/to/transcript.json",
			HookEventName:  PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "/test/file.txt",
			"content":   "benchmark content",
		}),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := composite.EvaluatePreToolUse(ctx, msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark registry operations
func BenchmarkRegistry_Register(b *testing.B) {
	registry := NewRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := HookConfig{
			Name:        "bench-hook",
			EventType:   PreToolUseEvent,
			ToolPattern: "Write.*",
			Priority:    1,
			Timeout:     30 * time.Second,
		}
		registry.Register(config)
	}
}

func BenchmarkRegistry_GetHooks(b *testing.B) {
	registry := NewRegistry()

	// Pre-populate with hooks
	for i := 0; i < 100; i++ {
		config := HookConfig{
			Name:        "hook",
			EventType:   PreToolUseEvent,
			ToolPattern: ".*",
			Priority:    i,
			Timeout:     30 * time.Second,
		}
		registry.Register(config)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hooks := registry.GetHooks(PreToolUseEvent)
		if len(hooks) == 0 {
			b.Fatal("expected hooks")
		}
	}
}

// Benchmark JSON marshaling of responses
func BenchmarkMarshalHookResponse(b *testing.B) {
	parser := NewParser()
	response := &HookResponse{
		Continue:       boolPtr(false),
		StopReason:     "Performance test",
		SuppressOutput: boolPtr(true),
		Decision:       "block",
		Reason:         "Benchmark reason",
		Message:        "This is a benchmark message",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.MarshalHookResponse(response)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark complete executor execution (without I/O)
func BenchmarkExecutor_ProcessMessage(b *testing.B) {
	engine := NewBaseRuleEngine()
	executor := NewExecutor(engine)
	ctx := context.Background()

	// Pre-create message data
	msgData := []byte(`{
		"session_id": "bench-123",
		"transcript_path": "/path/to/transcript.json",
		"hook_event_name": "PreToolUse",
		"tool_name": "Write",
		"tool_input": {
			"file_path": "/test.txt",
			"content": "benchmark content"
		}
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Parse the message
		msg, err := executor.handler.parser.ParseHookMessage(msgData)
		if err != nil {
			b.Fatal(err)
		}

		// Process it
		_, err = executor.handler.ProcessMessage(ctx, msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark parallel message processing
func BenchmarkHandler_ProcessMessage_Parallel(b *testing.B) {
	engine := NewBaseRuleEngine()
	handler := NewHandler(engine)
	ctx := context.Background()

	msg := &PreToolUseMessage{
		BaseHookMessage: BaseHookMessage{
			SessionID:      "bench-session",
			TranscriptPath: "/path/to/transcript.json",
			HookEventName:  PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: testConvertToRawMessage(map[string]interface{}{
			"file_path": "/test/file.txt",
			"content":   "benchmark content",
		}),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := handler.ProcessMessage(ctx, msg)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
