package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo/pkg/engine"
	"github.com/jrossi/gismo/pkg/reflection"
)

// ReflectionHandler manages context tracking and reflection prompts
type ReflectionHandler struct {
	*engine.BaseActionHandler
	accumulator    *reflection.ContextAccumulator
	contextManager *reflection.ContextManager
	patternLearner *reflection.PatternLearner
	sessionManager *reflection.SessionManager
	storage        *reflection.Storage

	mu             sync.RWMutex
	operationCount int
	lastReflection time.Time

	// Configuration
	config ReflectionConfig
}

// ReflectionConfig provides configuration for the reflection handler
type ReflectionConfig struct {
	// Trigger thresholds
	OperationThreshold  int           `json:"operation_threshold"`  // Trigger after N operations
	TimeThreshold       time.Duration `json:"time_threshold"`       // Trigger after duration
	ConfidenceThreshold float64       `json:"confidence_threshold"` // Trigger below confidence

	// Context management
	MaxContextSize     int           `json:"max_context_size"`    // Max operations to track
	AutoCheckpoint     bool          `json:"auto_checkpoint"`     // Auto-save context
	CheckpointInterval time.Duration `json:"checkpoint_interval"` // How often to checkpoint

	// Learning
	EnableLearning       bool `json:"enable_learning"`        // Learn from patterns
	MinPatternOccurrence int  `json:"min_pattern_occurrence"` // Min times to see pattern
}

// DefaultReflectionConfig returns default configuration
func DefaultReflectionConfig() ReflectionConfig {
	return ReflectionConfig{
		OperationThreshold:   10,
		TimeThreshold:        5 * time.Minute,
		ConfidenceThreshold:  0.5,
		MaxContextSize:       100,
		AutoCheckpoint:       true,
		CheckpointInterval:   10 * time.Minute,
		EnableLearning:       true,
		MinPatternOccurrence: 3,
	}
}

// NewReflectionHandler creates a new reflection handler
func NewReflectionHandler() *ReflectionHandler {
	return NewReflectionHandlerWithConfig(DefaultReflectionConfig())
}

// NewReflectionHandlerWithConfig creates a new reflection handler with custom config
func NewReflectionHandlerWithConfig(config ReflectionConfig) *ReflectionHandler {
	handler := &ReflectionHandler{
		BaseActionHandler: engine.NewBaseActionHandler("reflection", 50), // Medium priority
		accumulator:       reflection.NewContextAccumulator(),
		contextManager:    reflection.NewContextManager(),
		patternLearner:    reflection.NewPatternLearner(),
		sessionManager:    reflection.NewSessionManager(),
		config:            config,
		lastReflection:    time.Now(),
	}

	// Configure session manager
	if config.AutoCheckpoint {
		handler.sessionManager.EnableAutoCheckpoint(config.CheckpointInterval)
	}

	return handler
}

// SetStorage sets the database storage for persistence
func (h *ReflectionHandler) SetStorage(storage *reflection.Storage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.storage = storage

	// Try to restore previous session if available
	if h.storage != nil {
		h.restoreFromStorage()
	}
}

// restoreFromStorage attempts to restore state from database
func (h *ReflectionHandler) restoreFromStorage() {
	sessionID := h.sessionManager.GetSessionID()

	// Try to restore session state
	state, err := h.storage.RestoreSession(sessionID)
	if err != nil {
		// No previous session, create new one
		if err := h.storage.SaveSession(sessionID, h.contextManager.GetProjectPath(),
			h.contextManager.GetProjectName(), nil); err != nil {
			fmt.Printf("Failed to save session: %v\n", err)
		}
		return
	}

	// Restore working memory
	if state.WorkingMemory != nil {
		h.contextManager.SetWorkingMemory(state.WorkingMemory)
	}

	// Restore operations to accumulator
	for _, op := range state.Operations {
		h.accumulator.RecordOperation(op)
		if op.Result != nil {
			h.accumulator.UpdateLastOperation(*op.Result)
		}
	}

	// Load learned patterns
	if h.config.EnableLearning {
		successPatterns, _ := h.storage.LoadPatterns("success", h.config.MinPatternOccurrence)
		failurePatterns, _ := h.storage.LoadPatterns("failure", h.config.MinPatternOccurrence)
		h.patternLearner.RestorePatterns(successPatterns, failurePatterns)
	}
}

// ShouldHandle determines if this handler should process the event
func (h *ReflectionHandler) ShouldHandle(ctx context.Context, event engine.HookMessage) bool {
	// Handle all tool use events for tracking
	switch event.(type) {
	case *engine.PreToolUseMessage, *engine.PostToolUseMessage:
		return true
	case *engine.UserPromptSubmitMessage:
		return true
	default:
		return false
	}
}

// HandlePreToolUse tracks tool usage before execution
func (h *ReflectionHandler) HandlePreToolUse(ctx context.Context, msg *engine.PreToolUseMessage) (*engine.HookResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Track the operation intent
	operation := reflection.Operation{
		ToolName:  msg.ToolName,
		Arguments: msg.ToolInput,
		Timestamp: time.Now(),
		Type:      h.classifyOperation(msg.ToolName),
		Intent:    "pre-execution",
	}

	h.accumulator.RecordOperation(operation)
	h.operationCount++

	// Save to database if available
	if h.storage != nil {
		if err := h.storage.SaveOperation(operation, h.sessionManager.GetSessionID()); err != nil {
			// Log but don't fail
			fmt.Printf("Failed to save operation: %v\n", err)
		}
	}

	// Check if reflection is needed
	if h.shouldTriggerReflection() {
		// Add reflection prompt to the message
		prompt := h.generateReflectionPrompt()

		// Return a result that includes the reflection prompt
		return &engine.HookResponse{
			Decision: "approve",
			Message:  fmt.Sprintf("\n🤔 Reflection suggested: %s\n", prompt),
		}, nil
	}

	return &engine.HookResponse{Decision: "approve"}, nil
}

// HandlePostToolUse tracks tool results after execution
func (h *ReflectionHandler) HandlePostToolUse(ctx context.Context, msg *engine.PostToolUseMessage) (*engine.HookResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Determine success based on whether there was an error
	success := msg.ToolError == ""

	// Update the operation with results
	h.accumulator.UpdateLastOperation(reflection.OperationResult{
		Success: success,
		Output:  msg.ToolOutput,
		Error:   msg.ToolError,
	})

	// Update context manager
	h.contextManager.ProcessToolResult(msg.ToolName, msg.ToolOutput, success)

	// Learn from the pattern if enabled
	if h.config.EnableLearning {
		h.patternLearner.ObserveOperation(msg.ToolName, success)
	}

	// Check for auto-checkpoint
	if h.config.AutoCheckpoint && h.sessionManager.ShouldCheckpoint() {
		if err := h.createCheckpoint("auto"); err != nil {
			// Log but don't fail
			fmt.Printf("Failed to create checkpoint: %v\n", err)
		}
	}

	return &engine.HookResponse{}, nil
}

// HandleUserPromptSubmit handles new user prompts
func (h *ReflectionHandler) HandleUserPromptSubmit(ctx context.Context, msg *engine.UserPromptSubmitMessage) (*engine.HookResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Update current task in context
	h.contextManager.SetCurrentTask(msg.UserPrompt)

	// Check if this is a task switch
	if h.contextManager.IsTaskSwitch(msg.UserPrompt) {
		// Create checkpoint for previous task
		if err := h.createCheckpoint("task_switch"); err != nil {
			fmt.Printf("Failed to create task switch checkpoint: %v\n", err)
		}

		// Suggest reflection on task switch
		return &engine.HookResponse{
			Message: "📋 Task switch detected. Consider reviewing progress on the previous task before proceeding.",
		}, nil
	}

	return &engine.HookResponse{}, nil
}

// Handle implements the ActionHandler interface
func (h *ReflectionHandler) Handle(ctx context.Context, event engine.HookMessage) (*engine.HookResponse, error) {
	switch msg := event.(type) {
	case *engine.PreToolUseMessage:
		return h.HandlePreToolUse(ctx, msg)
	case *engine.PostToolUseMessage:
		return h.HandlePostToolUse(ctx, msg)
	case *engine.UserPromptSubmitMessage:
		return h.HandleUserPromptSubmit(ctx, msg)
	default:
		return &engine.HookResponse{}, nil
	}
}

// shouldTriggerReflection determines if reflection should be triggered
func (h *ReflectionHandler) shouldTriggerReflection() bool {
	// Check operation count threshold
	if h.operationCount >= h.config.OperationThreshold {
		return true
	}

	// Check time threshold
	if time.Since(h.lastReflection) >= h.config.TimeThreshold {
		return true
	}

	// Check confidence threshold
	score := h.accumulator.CalculateContextScore()
	if score.Confidence < h.config.ConfidenceThreshold {
		return true
	}

	// Check for pattern-based triggers
	if h.patternLearner.SuggestsReflection() {
		return true
	}

	return false
}

// generateReflectionPrompt creates a context-aware reflection prompt
func (h *ReflectionHandler) generateReflectionPrompt() string {
	score := h.accumulator.CalculateContextScore()
	summary := h.contextManager.GenerateSummary()

	prompt := fmt.Sprintf(`
Based on your recent operations:
- Completeness: %.0f%%
- Confidence: %.0f%%
- Operations performed: %d

Current understanding:
%s

Consider:
1. Do you have sufficient information to proceed?
2. Are there any gaps in your understanding?
3. Should you explore additional areas?
`,
		score.Completeness*100,
		score.Confidence*100,
		h.operationCount,
		summary,
	)

	// Save reflection event to database
	if h.storage != nil {
		if err := h.storage.SaveReflectionEvent(h.sessionManager.GetSessionID(),
			"threshold_trigger", prompt, score, h.operationCount); err != nil {
			fmt.Printf("Failed to save reflection event: %v\n", err)
		}

		// Also save context score
		if err := h.storage.SaveContextScore(h.sessionManager.GetSessionID(), score); err != nil {
			fmt.Printf("Failed to save context score: %v\n", err)
		}
	}

	// Reset counters after reflection
	h.operationCount = 0
	h.lastReflection = time.Now()
	h.sessionManager.IncrementReflectionCount()

	return prompt
}

// classifyOperation determines the operation type from tool name
func (h *ReflectionHandler) classifyOperation(toolName string) reflection.OperationType {
	switch toolName {
	case "Search", "Grep", "Find":
		return reflection.OperationTypeSearch
	case "Read", "Get", "Fetch":
		return reflection.OperationTypeRead
	case "Write", "Edit", "Create":
		return reflection.OperationTypeWrite
	case "FindSymbol", "GetSymbolsOverview":
		return reflection.OperationTypeSymbolLookup
	case "FindReferences", "FindReferencingSymbols":
		return reflection.OperationTypeReferenceSearch
	case "ExaSearch", "WebSearch":
		return reflection.OperationTypeExternalSearch
	default:
		return reflection.OperationTypeAnalyze
	}
}

// createCheckpoint saves the current context state
func (h *ReflectionHandler) createCheckpoint(reason string) error {
	checkpoint := h.contextManager.CreateSnapshot()
	err := h.sessionManager.SaveCheckpoint(checkpoint, reason)

	// Also save to database if available
	if h.storage != nil && checkpoint != nil {
		dbCheckpoint := reflection.Checkpoint{
			ID:        fmt.Sprintf("checkpoint_%d", time.Now().UnixNano()),
			Timestamp: time.Now(),
			Reason:    reason,
			Snapshot:  checkpoint,
		}
		if dbErr := h.storage.SaveCheckpoint(dbCheckpoint, h.sessionManager.GetSessionID()); dbErr != nil {
			fmt.Printf("Failed to save checkpoint to database: %v\n", dbErr)
		}

		// Save working memory state
		if dbErr := h.storage.SaveWorkingMemory(h.sessionManager.GetSessionID(),
			h.contextManager.GetWorkingMemory()); dbErr != nil {
			fmt.Printf("Failed to save working memory: %v\n", dbErr)
		}
	}

	return err
}

// GetContextSummary returns a summary of the current context
func (h *ReflectionHandler) GetContextSummary() *reflection.ContextSummary {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.contextManager.GenerateContextSummary()
}

// RestoreFromCheckpoint restores context from a previous checkpoint
func (h *ReflectionHandler) RestoreFromCheckpoint(checkpointID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	checkpoint, err := h.sessionManager.LoadCheckpoint(checkpointID)
	if err != nil {
		return fmt.Errorf("failed to load checkpoint: %w", err)
	}

	return h.contextManager.RestoreFromSnapshot(checkpoint)
}

// UpdateWorkingMemory updates the working memory with new information
func (h *ReflectionHandler) UpdateWorkingMemory(update reflection.WorkingMemoryUpdate) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.contextManager.UpdateWorkingMemory(update)
}

// GetReflectionStats returns statistics about reflection patterns
func (h *ReflectionHandler) GetReflectionStats() *reflection.ReflectionStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return &reflection.ReflectionStats{
		TotalOperations:     h.accumulator.TotalOperations(),
		ReflectionCount:     h.sessionManager.ReflectionCount(),
		AverageContextScore: h.accumulator.AverageContextScore(),
		SuccessfulPatterns:  h.patternLearner.GetSuccessfulPatterns(),
		FailurePatterns:     h.patternLearner.GetFailurePatterns(),
	}
}

// ExportHandler exports the handler for use in the engine
func (h *ReflectionHandler) ExportHandler() engine.ActionHandler {
	return h
}

// SetConfig updates the handler configuration
func (h *ReflectionHandler) SetConfig(config interface{}) error {
	configData, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var reflectionConfig ReflectionConfig
	if err := json.Unmarshal(configData, &reflectionConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	h.mu.Lock()
	h.config = reflectionConfig
	h.mu.Unlock()

	// Update session manager
	if reflectionConfig.AutoCheckpoint {
		h.sessionManager.EnableAutoCheckpoint(reflectionConfig.CheckpointInterval)
	} else {
		h.sessionManager.DisableAutoCheckpoint()
	}

	return nil
}
