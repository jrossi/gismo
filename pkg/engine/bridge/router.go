package bridge

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	json "github.com/goccy/go-json"
	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

// HandlerFunc is a function that handles a specific hook request
type HandlerFunc func(context.Context, *gismov1.HookRequest) (*gismov1.HookResponse, error)

// Router handles routing of protobuf hook messages to appropriate handlers
type Router struct {
	converter  *Converter
	validator  *Validator
	handlers   map[gismov1.HookEventType]HandlerFunc
	middleware []MiddlewareFunc
	debugMode  bool
	mu         sync.RWMutex

	// Metrics
	requestCount  int64
	errorCount    int64
	validationErr int64
}

// MiddlewareFunc is a function that can intercept and process requests
type MiddlewareFunc func(context.Context, *gismov1.HookRequest, HandlerFunc) (*gismov1.HookResponse, error)

// RouterOptions configures the router
type RouterOptions struct {
	DebugMode        bool
	EnableValidation bool
	EnableMetrics    bool
}

// NewRouter creates a new hook router
func NewRouter(opts RouterOptions) (*Router, error) {
	validator, err := NewValidator()
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	return &Router{
		converter:  NewConverter(),
		validator:  validator,
		handlers:   make(map[gismov1.HookEventType]HandlerFunc),
		middleware: make([]MiddlewareFunc, 0),
		debugMode:  opts.DebugMode,
	}, nil
}

// RegisterHandler registers a handler for a specific hook event type
func (r *Router) RegisterHandler(eventType gismov1.HookEventType, handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[eventType] = handler
}

// RegisterMiddleware adds middleware to the processing chain
func (r *Router) RegisterMiddleware(mw MiddlewareFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware = append(r.middleware, mw)
}

// ProcessJSONRequest processes a JSON hook request from stdin
func (r *Router) ProcessJSONRequest(ctx context.Context, input io.Reader, output io.Writer) error {
	// Read JSON from input
	jsonData, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	if r.debugMode {
		log.Printf("[DEBUG] Received JSON: %s", string(jsonData))
	}

	// Convert JSON to protobuf
	req, err := r.converter.JSONToProto(jsonData)
	if err != nil {
		r.errorCount++
		return r.writeErrorResponse(output, fmt.Sprintf("Failed to parse request: %v", err))
	}

	// Process the protobuf request
	resp, err := r.ProcessRequest(ctx, req)
	if err != nil {
		return r.writeErrorResponse(output, fmt.Sprintf("Failed to process request: %v", err))
	}

	// Convert response to JSON
	jsonResp, err := r.converter.ProtoToJSON(resp)
	if err != nil {
		return fmt.Errorf("failed to convert response: %w", err)
	}

	if r.debugMode {
		log.Printf("[DEBUG] Response JSON: %s", string(jsonResp))
	}

	// Write response
	_, err = output.Write(jsonResp)
	return err
}

// ProcessRequest processes a protobuf hook request
func (r *Router) ProcessRequest(ctx context.Context, req *gismov1.HookRequest) (*gismov1.HookResponse, error) {
	r.requestCount++

	// Validate request
	if err := r.validator.ValidateRequest(req); err != nil {
		r.validationErr++
		if r.debugMode {
			log.Printf("[DEBUG] Validation failed: %v", err)
		}
		return &gismov1.HookResponse{
			Decision:   gismov1.HookResponse_DECISION_BLOCK,
			Reason:     "Request validation failed",
			Message:    fmt.Sprintf("Validation error: %v", err),
			ExitCode:   2,
			Continue:   boolPtr(false),
			StopReason: "validation_failure",
		}, nil
	}

	// Get handler for event type
	r.mu.RLock()
	handler, exists := r.handlers[req.Base.EventType]
	r.mu.RUnlock()

	if !exists {
		// Default handler - approve by default
		handler = r.defaultHandler
	}

	// Apply middleware chain
	for i := len(r.middleware) - 1; i >= 0; i-- {
		mw := r.middleware[i]
		next := handler
		handler = func(ctx context.Context, req *gismov1.HookRequest) (*gismov1.HookResponse, error) {
			return mw(ctx, req, next)
		}
	}

	// Execute handler
	resp, err := handler(ctx, req)
	if err != nil {
		r.errorCount++
		return nil, err
	}

	// Validate response
	if err := r.validator.ValidateResponse(resp); err != nil {
		return nil, fmt.Errorf("response validation failed: %w", err)
	}

	return resp, nil
}

// defaultHandler is the fallback handler when no specific handler is registered
func (r *Router) defaultHandler(ctx context.Context, req *gismov1.HookRequest) (*gismov1.HookResponse, error) {
	if r.debugMode {
		log.Printf("[DEBUG] Using default handler for event type: %v", req.Base.EventType)
	}

	// Default behavior: approve everything except dangerous operations
	response := &gismov1.HookResponse{
		Decision: gismov1.HookResponse_DECISION_APPROVE,
		Continue: boolPtr(true),
	}

	// Special handling for PreToolUse
	if req.Base.EventType == gismov1.HookEventType_HOOK_EVENT_TYPE_PRE_TOOL_USE {
		if preToolUse := req.GetPreToolUse(); preToolUse != nil {
			// Check for dangerous tools
			switch preToolUse.ToolName {
			case "Bash":
				// Validate bash commands more strictly
				if bash := preToolUse.GetBash(); bash != nil {
					if err := r.validateBashSafety(bash); err != nil {
						response.Decision = gismov1.HookResponse_DECISION_BLOCK
						response.Reason = fmt.Sprintf("Unsafe command: %v", err)
						response.Continue = boolPtr(false)
					}
				}
			}
		}
	}

	return response, nil
}

// validateBashSafety performs additional safety checks on bash commands
func (r *Router) validateBashSafety(params *gismov1.BashParameters) error {
	// This would use the validator's existing command validation
	// Just a simple check here for demonstration
	dangerousCommands := []string{"rm -rf /", "dd if=/dev/zero"}
	for _, danger := range dangerousCommands {
		if contains(params.Command, danger) {
			return fmt.Errorf("dangerous command pattern: %s", danger)
		}
	}
	return nil
}

// writeErrorResponse writes an error response to output
func (r *Router) writeErrorResponse(output io.Writer, message string) error {
	resp := map[string]interface{}{
		"continue":   false,
		"stopReason": "error",
		"message":    message,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = output.Write(data)
	return err
}

// GetMetrics returns router metrics
func (r *Router) GetMetrics() map[string]int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return map[string]int64{
		"requests":         r.requestCount,
		"errors":           r.errorCount,
		"validation_fails": r.validationErr,
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}
