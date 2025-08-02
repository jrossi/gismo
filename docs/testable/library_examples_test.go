package testable

import (
	"context"
	"testing"

	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo/pkg/engine"
	"github.com/jrossi/gismo/pkg/handlers"
)

// TestLibraryDocumentationExamples tests all examples from the library documentation
func TestLibraryDocumentationExamples(t *testing.T) {
	dt := NewDocumentationTester(t)
	defer dt.Cleanup()

	// Load examples from library documentation
	err := dt.LoadExamplesFromMarkdown("../content/docs/library/_index.md")
	if err != nil {
		t.Fatalf("Failed to load examples: %v", err)
	}

	// Test that imports are correct
	if err := dt.ValidateAPIImports(); err != nil {
		t.Errorf("API import validation failed: %v", err)
	}

	// Test that API functions are correct
	if err := dt.ValidateAPIFunctions(); err != nil {
		t.Errorf("API function validation failed: %v", err)
	}

	// Test all examples compile and work
	if err := dt.TestAllExamples(); err != nil {
		t.Errorf("Example testing failed: %v", err)
	}
}

// TestBasicAPIUsage validates the basic API usage examples work
func TestBasicAPIUsage(t *testing.T) {
	// Test basic API creation (this should work)
	api := engine.New()
	if api == nil {
		t.Fatal("engine.New() returned nil")
	}

	// Test with rule engine
	ruleEngine := engine.NewBaseRuleEngine()
	apiWithEngine := engine.NewWithRuleEngine(ruleEngine)
	if apiWithEngine == nil {
		t.Fatal("engine.NewWithRuleEngine() returned nil")
	}
}

// TestActionHandlerExample validates action handler examples
func TestActionHandlerExample(t *testing.T) {
	// Create action handler engine
	actionEngine := engine.NewActionHandlerEngine()
	if actionEngine == nil {
		t.Fatal("NewActionHandlerEngine() returned nil")
	}

	// Register handlers
	fileHandler := handlers.NewFileAccessHandler()
	actionEngine.RegisterHandler(engine.PreToolUseEvent, fileHandler)

	secretHandler := handlers.NewSecretDetectionHandler()
	actionEngine.RegisterHandler(engine.UserPromptSubmitEvent, secretHandler)

	// Test that registry works
	registry := actionEngine.GetRegistry()
	if registry == nil {
		t.Fatal("GetRegistry() returned nil")
	}
}

// TestConfigurationExample validates configuration examples
func TestConfigurationExample(t *testing.T) {
	// Test configuration creation
	config := engine.NewAppConfig()
	if config == nil {
		t.Fatal("NewAppConfig() returned nil")
	}

	// Test that we can check if linters are enabled
	enabled := config.IsLinterEnabled("golang")
	_ = enabled // We don't care about the result, just that it doesn't panic

	// Test getting linter config
	linterConfig, ok := config.GetLinterConfig("golang")
	if !ok {
		t.Log("No golang linter config found (expected for default config)")
	}
	_ = linterConfig // Avoid unused variable
}

// TestHookMessageExample validates hook message examples
func TestHookMessageExample(t *testing.T) {
	ctx := context.Background()

	// Create a PreToolUse message like in the docs
	msg := &engine.PreToolUseMessage{
		BaseHookMessage: engine.BaseHookMessage{
			SessionID:     "test-session",
			HookEventName: engine.PreToolUseEvent,
		},
		ToolName: "Write",
		ToolInput: map[string]json.RawMessage{
			"file_path": json.RawMessage(`"test.go"`),
			"content":   json.RawMessage(`"package main\n"`),
		},
	}

	// Test that the message structure is correct
	if msg.EventName() != engine.PreToolUseEvent {
		t.Errorf("Expected event name %s, got %s", engine.PreToolUseEvent, msg.EventName())
	}

	// Test with a rule engine
	ruleEngine := engine.NewBaseRuleEngine()
	response, err := ruleEngine.EvaluatePreToolUse(ctx, msg)
	if err != nil {
		t.Fatalf("EvaluatePreToolUse failed: %v", err)
	}

	// Base rule engine should allow everything
	if response != nil && response.Decision == "block" {
		t.Errorf("Base rule engine should not block, got: %+v", response)
	}
}

// TestLintingEngineExample validates linting engine examples
func TestLintingEngineExample(t *testing.T) {
	// Create linting engine
	lintingEngine := engine.NewLintingRuleEngine()
	if lintingEngine == nil {
		t.Fatal("NewLintingRuleEngine() returned nil")
	}

	// Test that we can set app config
	config := engine.NewAppConfig()
	lintingEngine.SetAppConfig(config)

	// Verify we can get it back
	retrievedConfig := lintingEngine.GetAppConfig()
	if retrievedConfig == nil {
		t.Fatal("GetAppConfig() returned nil after SetAppConfig()")
	}
}

// TestAPIBuilderExample validates API builder examples
func TestAPIBuilderExample(t *testing.T) {
	// Test builder pattern
	builder := engine.NewBuilder()
	if builder == nil {
		t.Fatal("NewBuilder() returned nil")
	}

	// Test builder methods
	ruleEngine := engine.NewBaseRuleEngine()
	builder = builder.WithRuleEngine(ruleEngine)
	if builder == nil {
		t.Fatal("WithRuleEngine() returned nil")
	}

	// Build the API
	api := builder.Build()
	if api == nil {
		t.Fatal("Builder.Build() returned nil")
	}
}
