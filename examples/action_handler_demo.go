package main

import (
	"context"
	"fmt"
	"log"

	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo"
	"github.com/jrossi/gismo/handlers"
)

// This demo shows how the new action handler architecture works
// and demonstrates all 5 use cases specified in the requirements

func main() {
	fmt.Println("🚀 Action Handler Architecture Demo")
	fmt.Println("===================================")

	// Create the new action handler engine
	engine := gismo.NewActionHandlerEngine()

	// Register handlers with priorities (higher priority = runs first)
	fmt.Println("\n📝 Registering action handlers...")

	// 1. File Access Handler (highest priority for security)
	fileHandler := handlers.NewFileAccessHandler()
	engine.RegisterHandler(gismo.PreToolUseEvent, fileHandler)
	fmt.Println("✅ File Access Handler registered (priority 300)")

	// 2. Secret Detection Handler (high priority)
	secretHandler := handlers.NewSecretDetectionHandler()
	engine.RegisterHandler(gismo.UserPromptSubmitEvent, secretHandler)
	engine.RegisterHandler(gismo.PreToolUseEvent, secretHandler)
	fmt.Println("✅ Secret Detection Handler registered (priority 200)")

	// 3. Regex Handler (medium priority)
	regexHandler := handlers.NewRegexHandler()
	engine.RegisterHandler(gismo.UserPromptSubmitEvent, regexHandler)
	engine.RegisterHandler(gismo.PreToolUseEvent, regexHandler)
	fmt.Println("✅ Regex Handler registered (priority 150)")

	// 4. Linting Handler (lower priority)
	lintingHandler := handlers.NewLintingHandler()
	engine.RegisterHandler(gismo.PreToolUseEvent, lintingHandler)
	engine.RegisterHandler(gismo.PostToolUseEvent, lintingHandler)
	fmt.Println("✅ Linting Handler registered (priority 100)")

	// 5. Notification Handler (lowest priority)
	notificationHandler := handlers.NewNotificationHandler()
	engine.RegisterHandler(gismo.NotificationEvent, notificationHandler)
	fmt.Println("✅ Notification Handler registered (priority 50)")

	// Demonstrate the 5 use cases
	fmt.Println("\n🎯 Demonstrating the 5 specified use cases:")

	// Use Case 1: Secret detection in user prompt
	fmt.Println("\n1️⃣  Use Case 1: Secret detection in UserPromptSubmit")
	testSecretDetection(engine)

	// Use Case 2: Regex matching in prompt (non-blocking)
	fmt.Println("\n2️⃣  Use Case 2: Regex matching in UserPromptSubmit")
	testRegexMatching(engine)

	// Use Case 3: PreToolUse Read(*) blocks PEM cert files
	fmt.Println("\n3️⃣  Use Case 3: PreToolUse Read(*) blocks PEM cert files")
	testFileReadRestriction(engine)

	// Use Case 4: PreToolUse Write(*) blocks predefined paths
	fmt.Println("\n4️⃣  Use Case 4: PreToolUse Write(*) blocks system paths")
	testFileWriteRestriction(engine)

	// Use Case 5: Notification handling
	fmt.Println("\n5️⃣  Use Case 5: Notification handling")
	testNotificationHandling(engine)

	fmt.Println("\n🎉 Demo completed successfully!")
	fmt.Println("The new action handler architecture provides:")
	fmt.Println("  ✨ Extensible handler system")
	fmt.Println("  🔒 Security-first priority ordering")
	fmt.Println("  ⚙️  Configurable policy rules")
	fmt.Println("  🔌 Pluggable handler architecture")
	fmt.Println("  📊 Support for all Claude Code hook types")
}

func testSecretDetection(engine gismo.RuleEngine) {
	ctx := context.Background()

	// Create a UserPromptSubmit message with a potential secret
	msg := &gismo.UserPromptSubmitMessage{
		BaseHookMessage: gismo.BaseHookMessage{
			SessionID:     "demo-session",
			HookEventName: gismo.UserPromptSubmitEvent,
		},
		UserPrompt: "Please help me with api_key=sk-1234567890abcdef1234567890abcdef",
		Timestamp:  1234567890,
	}

	response, err := engine.EvaluateUserPromptSubmit(ctx, msg)
	if err != nil {
		log.Printf("❌ Error: %v", err)
		return
	}

	if response != nil && response.Decision == "block" {
		fmt.Printf("🔒 SUCCESS: Secret detected and blocked: %s\n", response.Reason)
	} else {
		fmt.Printf("⚠️  Response: %+v\n", response)
	}
}

func testRegexMatching(engine gismo.RuleEngine) {
	ctx := context.Background()

	// Create a UserPromptSubmit message with regex patterns
	msg := &gismo.UserPromptSubmitMessage{
		BaseHookMessage: gismo.BaseHookMessage{
			SessionID:     "demo-session",
			HookEventName: gismo.UserPromptSubmitEvent,
		},
		UserPrompt: "This is a demo test example for the system",
		Timestamp:  1234567890,
	}

	response, err := engine.EvaluateUserPromptSubmit(ctx, msg)
	if err != nil {
		log.Printf("❌ Error: %v", err)
		return
	}

	fmt.Printf("✅ Regex matching completed (non-blocking): %+v\n", response)
}

func testFileReadRestriction(engine gismo.RuleEngine) {
	ctx := context.Background()

	// Create a PreToolUse message for reading a PEM file
	toolInput := map[string]json.RawMessage{
		"file_path": json.RawMessage(`"/etc/ssl/private/server.pem"`),
	}

	msg := &gismo.PreToolUseMessage{
		BaseHookMessage: gismo.BaseHookMessage{
			SessionID:     "demo-session",
			HookEventName: gismo.PreToolUseEvent,
		},
		ToolName:  "Read",
		ToolInput: toolInput,
	}

	response, err := engine.EvaluatePreToolUse(ctx, msg)
	if err != nil {
		log.Printf("❌ Error: %v", err)
		return
	}

	if response != nil && response.Decision == "block" {
		fmt.Printf("🔒 SUCCESS: PEM file read blocked: %s\n", response.Reason)
	} else {
		fmt.Printf("⚠️  Unexpected response: %+v\n", response)
	}
}

func testFileWriteRestriction(engine gismo.RuleEngine) {
	ctx := context.Background()

	// Create a PreToolUse message for writing to /etc/
	toolInput := map[string]json.RawMessage{
		"file_path": json.RawMessage(`"/etc/passwd"`),
		"content":   json.RawMessage(`"malicious content"`),
	}

	msg := &gismo.PreToolUseMessage{
		BaseHookMessage: gismo.BaseHookMessage{
			SessionID:     "demo-session",
			HookEventName: gismo.PreToolUseEvent,
		},
		ToolName:  "Write",
		ToolInput: toolInput,
	}

	response, err := engine.EvaluatePreToolUse(ctx, msg)
	if err != nil {
		log.Printf("❌ Error: %v", err)
		return
	}

	if response != nil && response.Decision == "block" {
		fmt.Printf("🔒 SUCCESS: System file write blocked: %s\n", response.Reason)
	} else {
		fmt.Printf("⚠️  Unexpected response: %+v\n", response)
	}
}

func testNotificationHandling(engine gismo.RuleEngine) {
	ctx := context.Background()

	// Create a Notification message
	msg := &gismo.NotificationMessage{
		BaseHookMessage: gismo.BaseHookMessage{
			SessionID:     "demo-session",
			HookEventName: gismo.NotificationEvent,
		},
		NotificationType: "tool_permission",
		Message:          "User requested permission to execute high-risk operation",
	}

	response, err := engine.EvaluateNotification(ctx, msg)
	if err != nil {
		log.Printf("❌ Error: %v", err)
		return
	}

	fmt.Printf("✅ Notification handled successfully: %+v\n", response)
}
