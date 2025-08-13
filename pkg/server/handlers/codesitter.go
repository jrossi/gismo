package handlers

import (
	"context"

	"github.com/jrossi/gismo/pkg/codesitter"
	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

// CodeSitterHandler wraps the codesitter.Server to provide the gRPC service
type CodeSitterHandler struct {
	*codesitter.Server
}

// NewCodeSitterHandler creates a new CodeSitter handler
func NewCodeSitterHandler() *CodeSitterHandler {
	return &CodeSitterHandler{
		Server: codesitter.NewServer(),
	}
}

// Ensure CodeSitterHandler implements the CodeSitterServer interface
var _ gismov1.CodeSitterServer = (*CodeSitterHandler)(nil)

// The actual implementation methods are embedded from codesitter.Server
// which already implements all the required gRPC methods:
// - InitializeWorkspace
// - ShutdownWorkspace
// - QuerySymbols
// - FindReferences
// - GetSymbolDefinition
// - GetCallGraph
// - AnalyzeSecurity
// - DetectPatterns
// - ValidateEdit
// - GetDiagnostics
// - GetTypeInfo
// - SuggestRefactoring
// - GetCodeMetrics
// - WatchFiles
// - WatchSymbols
// - WatchDiagnostics

// OptionalInitialize can be called to pre-initialize a workspace
func (h *CodeSitterHandler) OptionalInitialize(ctx context.Context, workspaceRoot string) error {
	if workspaceRoot == "" {
		return nil
	}

	_, err := h.Server.InitializeWorkspace(ctx, &gismov1.InitializeWorkspaceRequest{
		WorkspaceRoot:            workspaceRoot,
		EnableFileWatching:       true,
		EnableIncrementalParsing: true,
	})
	return err
}
