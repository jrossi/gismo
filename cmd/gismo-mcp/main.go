package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	json "github.com/goccy/go-json"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
	"github.com/jrossi/gismo/pkg/socket"
)

// JSON-RPC 2.0 message types
type JSONRPCMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *JSONRPCError    `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// MCP Protocol messages
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ClientCapabilities     `json:"capabilities"`
	ClientInfo      map[string]interface{} `json:"clientInfo,omitempty"`
}

type ClientCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ServerCapabilities     `json:"capabilities"`
	ServerInfo      map[string]interface{} `json:"serverInfo"`
}

type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// Tool definitions
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolCallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPServer handles MCP protocol over stdin/stdout
type MCPServer struct {
	reader          *bufio.Reader
	writer          *bufio.Writer
	grpcConn        *grpc.ClientConn
	csClient        gismov1.CodeSitterClient
	knowledgeClient gismov1.KnowledgeServiceClient
	workspace       string
	projectContext  *gismov1.ProjectContext
	tools           map[string]ToolHandler
}

// ToolHandler is a function that handles a tool call
type ToolHandler func(ctx context.Context, args json.RawMessage) ToolCallResult

func NewMCPServer() (*MCPServer, error) {
	// Connect to gismo-server
	conn, err := connectToGismoServer(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gismo-server: %w", err)
	}

	// Get current working directory
	cwd, _ := os.Getwd()

	// Find project root by looking for .claude directory
	projectRoot, err := findProjectRoot(cwd)
	if err != nil {
		// Fall back to current directory if no .claude found
		projectRoot = cwd
		log.Printf("Warning: Could not find .claude directory, using cwd: %s", cwd)
	} else {
		log.Printf("Found project root: %s", projectRoot)
	}

	// Create project context
	projectName := filepath.Base(projectRoot)

	s := &MCPServer{
		reader:          bufio.NewReader(os.Stdin),
		writer:          bufio.NewWriter(os.Stdout),
		grpcConn:        conn,
		csClient:        gismov1.NewCodeSitterClient(conn),
		knowledgeClient: gismov1.NewKnowledgeServiceClient(conn),
		workspace:       projectRoot,
		projectContext: &gismov1.ProjectContext{
			ProjectPath: projectRoot,
			ProjectName: projectName,
		},
		tools: make(map[string]ToolHandler),
	}

	// Register all tools
	s.registerTools()

	return s, nil
}

func connectToGismoServer(ctx context.Context) (*grpc.ClientConn, error) {
	return socket.Connect(ctx)
}

func (s *MCPServer) registerTools() {
	// Register CodeSitter tools
	s.tools["SearchForPattern"] = s.wrapGRPCCall(s.csClient.SearchForPattern)
	s.tools["GetSymbolsOverview"] = s.wrapGRPCCall(s.csClient.GetSymbolsOverview)
	s.tools["FindSymbol"] = s.wrapGRPCCall(s.csClient.FindSymbol)
	s.tools["FindReferencingSymbols"] = s.wrapGRPCCall(s.csClient.FindReferencingSymbols)
	s.tools["QuerySymbols"] = s.wrapGRPCCall(s.csClient.QuerySymbols)
	s.tools["FindReferences"] = s.wrapGRPCCall(s.csClient.FindReferences)
	s.tools["GetSymbolDefinition"] = s.wrapGRPCCall(s.csClient.GetSymbolDefinition)
	s.tools["AnalyzeSecurity"] = s.wrapGRPCCall(s.csClient.AnalyzeSecurity)
	s.tools["GetCodeMetrics"] = s.wrapGRPCCall(s.csClient.GetCodeMetrics)

	// Register Knowledge service tools
	s.tools["Search"] = s.wrapGRPCCall(s.knowledgeClient.Search)
	s.tools["GetContent"] = s.wrapGRPCCall(s.knowledgeClient.GetContent)
	s.tools["ExaSearch"] = s.wrapGRPCCall(s.knowledgeClient.ExaSearch)
	s.tools["ExecuteQuery"] = s.wrapGRPCCall(s.knowledgeClient.ExecuteQuery)
	s.tools["ListDocsets"] = s.wrapGRPCCall(s.knowledgeClient.ListDocsets)
	s.tools["ImportDocset"] = s.wrapGRPCCall(s.knowledgeClient.ImportDocset)
	s.tools["RemoveDocset"] = s.wrapGRPCCall(s.knowledgeClient.RemoveDocset)
	s.tools["CreateResearchTask"] = s.wrapGRPCCall(s.knowledgeClient.CreateResearchTask)
	s.tools["GetResearchTaskStatus"] = s.wrapGRPCCall(s.knowledgeClient.GetResearchTaskStatus)
}

// wrapGRPCCall wraps a gRPC method call to work as a tool handler
func (s *MCPServer) wrapGRPCCall(grpcMethod interface{}) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) ToolCallResult {
		// Use reflection to call the gRPC method
		methodValue := reflect.ValueOf(grpcMethod)
		methodType := methodValue.Type()

		// Get the request type (second parameter after context)
		// gRPC methods have signature: func(ctx, request, ...CallOption) (response, error)
		if methodType.NumIn() < 2 {
			return ToolCallResult{
				IsError: true,
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Invalid method signature: expected at least 2 params, got %d", methodType.NumIn())}},
			}
		}

		requestType := methodType.In(1)
		// Create a new instance of the request type
		requestValue := reflect.New(requestType.Elem())

		// Handle special cases where we need to add project context
		// Check if this is a Knowledge service request that needs project context
		// (We'll handle this in addProjectContext method below)

		// Convert the arguments to snake_case for protobuf
		// The MCP client sends PascalCase but protobuf expects snake_case
		var argsMap map[string]interface{}
		if err := json.Unmarshal(args, &argsMap); err == nil {
			// Convert keys to snake_case
			snakeCaseArgs := make(map[string]interface{})
			for k, v := range argsMap {
				snakeCaseArgs[camelToSnake(k)] = v
			}
			args, _ = json.Marshal(snakeCaseArgs)
		}

		// Unmarshal the arguments into the request
		if err := json.Unmarshal(args, requestValue.Interface()); err != nil {
			return ToolCallResult{
				IsError: true,
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Invalid parameters: %v", err)}},
			}
		}

		// Add project context for Knowledge service requests if needed
		s.addProjectContext(requestValue.Interface())

		// Call the gRPC method
		results := methodValue.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			requestValue,
		})

		// Check for error (second return value)
		if len(results) > 1 && !results[1].IsNil() {
			err := results[1].Interface().(error)
			return ToolCallResult{
				IsError: true,
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("RPC error: %v", err)}},
			}
		}

		// Format the response
		if len(results) > 0 && !results[0].IsNil() {
			response := results[0].Interface().(proto.Message)
			responseJSON, _ := json.MarshalIndent(response, "", "  ")
			return ToolCallResult{
				Content: []ContentItem{{Type: "text", Text: string(responseJSON)}},
			}
		}

		return ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: "Success"}},
		}
	}
}

// addProjectContext adds project context to requests that need it
func (s *MCPServer) addProjectContext(req interface{}) {
	// Use reflection to check if the request has a Context field
	v := reflect.ValueOf(req).Elem()
	contextField := v.FieldByName("Context")
	if contextField.IsValid() && contextField.CanSet() {
		contextField.Set(reflect.ValueOf(s.projectContext))
	}

	// Also handle file paths - convert relative to absolute
	s.normalizeFilePaths(req)
}

// normalizeFilePaths converts relative file paths to absolute paths
func (s *MCPServer) normalizeFilePaths(req interface{}) {
	v := reflect.ValueOf(req).Elem()

	// Check for FilePath field
	filePathField := v.FieldByName("FilePath")
	if filePathField.IsValid() && filePathField.CanSet() && filePathField.Kind() == reflect.String {
		path := filePathField.String()
		if path != "" && !filepath.IsAbs(path) {
			absPath := filepath.Join(s.workspace, path)
			filePathField.SetString(absPath)
		}
	}

	// Check for Path field
	pathField := v.FieldByName("Path")
	if pathField.IsValid() && pathField.CanSet() && pathField.Kind() == reflect.String {
		path := pathField.String()
		if path != "" && !filepath.IsAbs(path) {
			absPath := filepath.Join(s.workspace, path)
			pathField.SetString(absPath)
		}
	}

	// Check for WorkspaceRoot field
	rootField := v.FieldByName("WorkspaceRoot")
	if rootField.IsValid() && rootField.CanSet() && rootField.Kind() == reflect.String {
		path := rootField.String()
		if path != "" && !filepath.IsAbs(path) {
			absPath := filepath.Join(s.workspace, path)
			rootField.SetString(absPath)
		}
	}
}

// findProjectRoot finds the project root by looking for .claude directory
func findProjectRoot(startPath string) (string, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Walk up the directory tree looking for .claude
	current := absPath
	for {
		claudeDir := filepath.Join(current, ".claude")
		if stat, err := os.Stat(claudeDir); err == nil && stat.IsDir() {
			// Found .claude directory, this is the project root
			return current, nil
		}

		// Check if we're at the root
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding .claude
			break
		}
		current = parent
	}

	// Also check for .git as a fallback
	current = absPath
	for {
		gitDir := filepath.Join(current, ".git")
		if stat, err := os.Stat(gitDir); err == nil && (stat.IsDir() || stat.Mode().IsRegular()) {
			// Found .git directory or file (for submodules), this is a project root
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("could not find project root (.claude or .git directory)")
}

func (s *MCPServer) Run() error {
	// Log to stderr (MCP protocol allows this)
	log.SetOutput(os.Stderr)
	log.Println("MCP Server starting...")

	for {
		// Read line from stdin
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// Parse JSON-RPC message
		var msg JSONRPCMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// Handle the message
		response, err := s.handleMessage(&msg)
		if err != nil {
			log.Printf("Error handling message: %v", err)
			if msg.ID != nil {
				// Send error response
				s.sendError(msg.ID, -32603, err.Error())
			}
			continue
		}

		// Send response if it's a request (has ID)
		if response != nil && msg.ID != nil {
			response.ID = msg.ID
			if err := s.sendMessage(response); err != nil {
				log.Printf("Failed to send response: %v", err)
			}
		}
	}
}

func (s *MCPServer) handleMessage(msg *JSONRPCMessage) (*JSONRPCMessage, error) {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "tools/list":
		return s.handleToolsList(msg)
	case "tools/call":
		return s.handleToolCall(msg)
	default:
		return nil, fmt.Errorf("unknown method: %s", msg.Method)
	}
}

func (s *MCPServer) handleInitialize(msg *JSONRPCMessage) (*JSONRPCMessage, error) {
	var params InitializeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, err
	}

	// Initialize workspace with CodeSitter
	ctx := context.Background()
	_, err := s.csClient.InitializeWorkspace(ctx, &gismov1.InitializeWorkspaceRequest{
		WorkspaceRoot:            s.workspace,
		EnableFileWatching:       true,
		EnableIncrementalParsing: true,
	})
	if err != nil {
		log.Printf("Failed to initialize workspace: %v", err)
	}

	// Build response
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{},
		},
		ServerInfo: map[string]interface{}{
			"name":    "gismo-mcp",
			"version": "1.0.0",
		},
	}

	resultJSON, _ := json.Marshal(result)
	return &JSONRPCMessage{
		JSONRPC: "2.0",
		Result:  resultJSON,
	}, nil
}

func (s *MCPServer) handleToolsList(msg *JSONRPCMessage) (*JSONRPCMessage, error) {
	tools := []Tool{}

	// Generate tool definitions from registered handlers
	for name := range s.tools {
		// Convert CamelCase to snake_case for MCP
		mcpName := camelToSnake(name)

		// Generate a simple schema based on the method name
		schema := generateToolSchema(name)

		tools = append(tools, Tool{
			Name:        mcpName,
			Description: fmt.Sprintf("Call %s gRPC method", name),
			InputSchema: schema,
		})
	}

	result := map[string]interface{}{
		"tools": tools,
	}

	resultJSON, _ := json.Marshal(result)
	return &JSONRPCMessage{
		JSONRPC: "2.0",
		Result:  resultJSON,
	}, nil
}

func (s *MCPServer) handleToolCall(msg *JSONRPCMessage) (*JSONRPCMessage, error) {
	var params ToolCallParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, err
	}

	// Convert snake_case back to CamelCase
	methodName := snakeToCamel(params.Name)

	// Find the handler
	handler, ok := s.tools[methodName]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", params.Name)
	}

	// Call the handler
	ctx := context.Background()
	result := handler(ctx, params.Arguments)

	resultJSON, _ := json.Marshal(result)
	return &JSONRPCMessage{
		JSONRPC: "2.0",
		Result:  resultJSON,
	}, nil
}

func (s *MCPServer) sendMessage(msg *JSONRPCMessage) error {
	msg.JSONRPC = "2.0"
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = s.writer.WriteString(string(data) + "\n")
	if err != nil {
		return err
	}

	return s.writer.Flush()
}

func (s *MCPServer) sendError(id *json.RawMessage, code int, message string) {
	msg := &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	_ = s.sendMessage(msg)
}

// Helper functions for name conversion
func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// generateToolSchema generates a basic JSON schema for a tool
func generateToolSchema(methodName string) json.RawMessage {
	// For now, return a generic schema
	// In a real implementation, we could use protoreflect to generate this from the proto definition
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": true
	}`)
}

func main() {
	server, err := NewMCPServer()
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}
	defer server.grpcConn.Close()

	if err := server.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
