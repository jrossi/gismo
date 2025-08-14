package knowledge

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

// mockKnowledgeServer implements a mock gRPC server for testing
type mockKnowledgeServer struct {
	gismov1.UnimplementedKnowledgeServiceServer

	// Control behavior for testing
	shouldError      bool
	errorCode        codes.Code
	importProgress   []*gismov1.ImportProgress
	docsets          []*gismov1.Docset
	searchResults    []*gismov1.SearchResult
	queryResponse    *gismov1.QueryResponse
	contentResponse  *gismov1.GetContentResponse
	exaResponse      *gismov1.ExaSearchResponse
	researchTaskResp *gismov1.CreateResearchTaskResponse
	taskStatusResp   *gismov1.GetResearchTaskStatusResponse
}

func (m *mockKnowledgeServer) ImportDocset(req *gismov1.ImportDocsetRequest, stream gismov1.KnowledgeService_ImportDocsetServer) error {
	if m.shouldError {
		return status.Error(m.errorCode, "mock error")
	}

	for _, progress := range m.importProgress {
		if err := stream.Send(progress); err != nil {
			return err
		}
	}

	return nil
}

func (m *mockKnowledgeServer) ListDocsets(ctx context.Context, req *gismov1.ListDocsetsRequest) (*gismov1.ListDocsetsResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	return &gismov1.ListDocsetsResponse{
		Docsets: m.docsets,
	}, nil
}

func (m *mockKnowledgeServer) RemoveDocset(ctx context.Context, req *gismov1.RemoveDocsetRequest) (*gismov1.RemoveDocsetResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	if req.DocsetId == "invalid" {
		return &gismov1.RemoveDocsetResponse{
			Success: false,
			Message: "docset not found",
		}, nil
	}

	return &gismov1.RemoveDocsetResponse{
		Success: true,
		Message: "docset removed",
	}, nil
}

func (m *mockKnowledgeServer) Search(ctx context.Context, req *gismov1.SearchRequest) (*gismov1.SearchResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	return &gismov1.SearchResponse{
		Results: m.searchResults,
	}, nil
}

func (m *mockKnowledgeServer) GetContent(ctx context.Context, req *gismov1.GetContentRequest) (*gismov1.GetContentResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	if m.contentResponse != nil {
		return m.contentResponse, nil
	}

	return &gismov1.GetContentResponse{
		DocsetId: "test-docset",
		Name:     fmt.Sprintf("Content %d", req.ContentId),
		Type:     "test",
		Path:     "/test/path",
		Content:  fmt.Sprintf("Content for ID %d", req.ContentId),
		Metadata: map[string]string{
			"type": "test",
		},
	}, nil
}

func (m *mockKnowledgeServer) ExecuteQuery(ctx context.Context, req *gismov1.QueryRequest) (*gismov1.QueryResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	if m.queryResponse != nil {
		return m.queryResponse, nil
	}

	return &gismov1.QueryResponse{
		Columns: []string{"id", "name"},
		Rows: []*structpb.Struct{
			mustStruct(map[string]interface{}{
				"id":   1,
				"name": "test",
			}),
		},
	}, nil
}

func (m *mockKnowledgeServer) ExecuteQueryStream(req *gismov1.QueryRequest, stream gismov1.KnowledgeService_ExecuteQueryStreamServer) error {
	if m.shouldError {
		return status.Error(m.errorCode, "mock error")
	}

	// Send a few results
	for i := 0; i < 3; i++ {
		result := &gismov1.QueryResult{
			Result: &gismov1.QueryResult_Row{
				Row: mustStruct(map[string]interface{}{
					"id":   i,
					"name": fmt.Sprintf("item_%d", i),
				}),
			},
		}
		if err := stream.Send(result); err != nil {
			return err
		}
	}

	return nil
}

func (m *mockKnowledgeServer) ExaSearch(ctx context.Context, req *gismov1.ExaSearchRequest) (*gismov1.ExaSearchResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	if m.exaResponse != nil {
		return m.exaResponse, nil
	}

	return &gismov1.ExaSearchResponse{
		SearchId: "test-search-id",
		Results: []*gismov1.ExaResult{
			{
				Url:   "https://example.com",
				Title: "Example",
				Score: 0.95,
			},
		},
		FromCache: false,
	}, nil
}

func (m *mockKnowledgeServer) ProvideFeedback(ctx context.Context, req *gismov1.SearchFeedbackRequest) (*gismov1.SearchFeedbackResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	return &gismov1.SearchFeedbackResponse{
		Success: true,
		Message: "Feedback recorded",
	}, nil
}

func (m *mockKnowledgeServer) GetCachedSearches(ctx context.Context, req *gismov1.GetCachedSearchesRequest) (*gismov1.GetCachedSearchesResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	return &gismov1.GetCachedSearchesResponse{
		Searches: []*gismov1.CachedSearch{
			{
				Id:         "cached-1",
				Query:      "test query",
				SearchType: "neural",
			},
		},
	}, nil
}

func (m *mockKnowledgeServer) CreateResearchTask(ctx context.Context, req *gismov1.CreateResearchTaskRequest) (*gismov1.CreateResearchTaskResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	if m.researchTaskResp != nil {
		return m.researchTaskResp, nil
	}

	return &gismov1.CreateResearchTaskResponse{
		TaskId:  "task-123",
		Message: "Research task created",
	}, nil
}

func (m *mockKnowledgeServer) GetResearchTaskStatus(ctx context.Context, req *gismov1.GetResearchTaskStatusRequest) (*gismov1.GetResearchTaskStatusResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	if m.taskStatusResp != nil {
		return m.taskStatusResp, nil
	}

	return &gismov1.GetResearchTaskStatusResponse{
		TaskId:          req.TaskId,
		Status:          "running",
		ProgressMessage: "Processing...",
		ElapsedSeconds:  300,
	}, nil
}

func (m *mockKnowledgeServer) CancelResearchTask(ctx context.Context, req *gismov1.CancelResearchTaskRequest) (*gismov1.CancelResearchTaskResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	return &gismov1.CancelResearchTaskResponse{
		Success: true,
		Message: "Task cancelled",
	}, nil
}

func (m *mockKnowledgeServer) ListActiveResearchTasks(ctx context.Context, req *gismov1.ListActiveResearchTasksRequest) (*gismov1.ListActiveResearchTasksResponse, error) {
	if m.shouldError {
		return nil, status.Error(m.errorCode, "mock error")
	}

	return &gismov1.ListActiveResearchTasksResponse{
		Tasks: []*gismov1.ResearchTaskSummary{
			{
				TaskId:       "task-1",
				Instructions: "Research task",
			},
		},
		TotalCount: 1,
	}, nil
}

// Helper function to create structpb.Struct
func mustStruct(m map[string]interface{}) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	if err != nil {
		panic(err)
	}
	return s
}

// Test helpers
func setupTestServer(t *testing.T) (*mockKnowledgeServer, string, func()) {
	// Use a shorter path for Mac compatibility
	tmpDir := "/tmp/gkt" + fmt.Sprintf("%d", time.Now().UnixNano()%100000)
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0700)

	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", tmpDir)

	socketPath := filepath.Join(tmpDir, "gismo", "gismo.sock")
	os.MkdirAll(filepath.Dir(socketPath), 0700)

	// Create Unix socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// Create gRPC server
	srv := grpc.NewServer()
	mockServer := &mockKnowledgeServer{}
	gismov1.RegisterKnowledgeServiceServer(srv, mockServer)

	// Start server
	go srv.Serve(listener)

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	cleanup := func() {
		srv.Stop()
		listener.Close()
		os.Remove(socketPath)
		os.RemoveAll(tmpDir)
		os.Setenv("XDG_RUNTIME_DIR", oldXDG)
	}

	return mockServer, socketPath, cleanup
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() func()
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful connection",
			setup: func() func() {
				mock, _, cleanup := setupTestServer(t)
				_ = mock // Server is running
				return cleanup
			},
			wantErr: false,
		},
		{
			name: "socket not found",
			setup: func() func() {
				tmpDir := t.TempDir()
				oldXDG := os.Getenv("XDG_RUNTIME_DIR")
				os.Setenv("XDG_RUNTIME_DIR", tmpDir)
				return func() {
					os.Setenv("XDG_RUNTIME_DIR", oldXDG)
				}
			},
			wantErr: true,
			errMsg:  "gismo server not running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setup()
			defer cleanup()

			client, err := New()
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("New() error = %v, want error containing %v", err, tt.errMsg)
				}
			}

			if client != nil {
				defer client.Close()
			}
		})
	}
}

func TestGetProjectContext(t *testing.T) {
	// Save original env vars
	oldCwd, _ := os.Getwd()
	oldProjectDir := os.Getenv("CLAUDE_PROJECT_DIR")
	oldProjectID := os.Getenv("CLAUDE_PROJECT_ID")
	defer func() {
		os.Chdir(oldCwd)
		os.Setenv("CLAUDE_PROJECT_DIR", oldProjectDir)
		os.Setenv("CLAUDE_PROJECT_ID", oldProjectID)
	}()

	tests := []struct {
		name      string
		setupEnv  func()
		wantEmpty bool
	}{
		{
			name:      "default context",
			setupEnv:  func() {},
			wantEmpty: false,
		},
		{
			name: "with Claude env vars",
			setupEnv: func() {
				os.Setenv("CLAUDE_PROJECT_DIR", "/test/project")
				os.Setenv("CLAUDE_PROJECT_ID", "test-id")
			},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()

			ctx := getProjectContext()
			if ctx == nil {
				t.Error("getProjectContext() returned nil")
				return
			}

			if tt.wantEmpty && ctx.ProjectPath != "" {
				t.Errorf("Expected empty context, got path: %s", ctx.ProjectPath)
			}
		})
	}
}

func TestClient_ImportDocset(t *testing.T) {
	mock, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Set up mock progress responses
	mock.importProgress = []*gismov1.ImportProgress{
		{
			Stage:           gismov1.ImportProgress_DOWNLOADING,
			Message:         "Downloading...",
			ProgressPercent: 25,
		},
		{
			Stage:           gismov1.ImportProgress_PARSING,
			Message:         "Parsing...",
			ProgressPercent: 50,
		},
		{
			Stage:           gismov1.ImportProgress_COMPLETE,
			Message:         "Done",
			ProgressPercent: 100,
		},
	}

	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	var progressCalls []string
	err = client.ImportDocset(context.Background(), "https://example.com/docs.xml",
		func(stage, message string, percent int) {
			progressCalls = append(progressCalls, fmt.Sprintf("%s:%s:%d", stage, message, percent))
		})

	if err != nil {
		t.Errorf("ImportDocset() error = %v", err)
	}

	if len(progressCalls) != 3 {
		t.Errorf("Expected 3 progress calls, got %d", len(progressCalls))
	}
}

func TestClient_ListDocsets(t *testing.T) {
	mock, _, cleanup := setupTestServer(t)
	defer cleanup()

	mock.docsets = []*gismov1.Docset{
		{
			Id:         "docset-1",
			Name:       "Test Docset",
			Version:    "1.0",
			SourceType: "official",
		},
	}

	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	docsets, err := client.ListDocsets(context.Background())
	if err != nil {
		t.Errorf("ListDocsets() error = %v", err)
	}

	if len(docsets) != 1 {
		t.Errorf("Expected 1 docset, got %d", len(docsets))
	}

	if docsets[0].Name != "Test Docset" {
		t.Errorf("Expected docset name 'Test Docset', got %s", docsets[0].Name)
	}
}

func TestClient_RemoveDocset(t *testing.T) {
	_, _, cleanup := setupTestServer(t)
	defer cleanup()

	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test successful removal
	err = client.RemoveDocset(context.Background(), "valid-id")
	if err != nil {
		t.Errorf("RemoveDocset() error = %v", err)
	}

	// Test failed removal
	err = client.RemoveDocset(context.Background(), "invalid")
	if err == nil {
		t.Error("Expected error for invalid docset ID")
	}
}

func TestClient_Search(t *testing.T) {
	mock, _, cleanup := setupTestServer(t)
	defer cleanup()

	mock.searchResults = []*gismov1.SearchResult{
		{
			DocsetId:       "docset-1",
			DocsetName:     "Test Docset",
			ItemName:       "Test Result",
			ItemType:       "Function",
			ContentPreview: "Test content",
			RelevanceScore: 0.95,
			ContentId:      1,
		},
	}

	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	results, err := client.Search(context.Background(), "test query", SearchOptions{
		Type:  SearchTypeHybrid,
		Limit: 10,
	})

	if err != nil {
		t.Errorf("Search() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestClient_ExecuteQueryStream(t *testing.T) {
	_, _, cleanup := setupTestServer(t)
	defer cleanup()

	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	var results []string
	err = client.ExecuteQueryStream(context.Background(), "SELECT * FROM test",
		func(result *gismov1.QueryResult) error {
			if row := result.GetRow(); row != nil {
				if nameVal, ok := row.Fields["name"]; ok {
					results = append(results, nameVal.GetStringValue())
				}
			}
			return nil
		})

	if err != nil {
		t.Errorf("ExecuteQueryStream() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}

func TestClient_ExaSearch(t *testing.T) {
	_, _, cleanup := setupTestServer(t)
	defer cleanup()

	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	useCache := true
	resp, err := client.ExaSearch(context.Background(), "test query", &ExaSearchOptions{
		NumResults:    5,
		SearchType:    "neural",
		UseAutoprompt: true,
		UseCache:      &useCache,
	})

	if err != nil {
		t.Errorf("ExaSearch() error = %v", err)
	}

	if resp.SearchId != "test-search-id" {
		t.Errorf("Expected search ID 'test-search-id', got %s", resp.SearchId)
	}

	// Test with nil options
	resp, err = client.ExaSearch(context.Background(), "test query", nil)
	if err != nil {
		t.Errorf("ExaSearch() with nil options error = %v", err)
	}
}

func TestClient_ResearchTasks(t *testing.T) {
	_, _, cleanup := setupTestServer(t)
	defer cleanup()

	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test CreateResearchTask
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"result": map[string]interface{}{
				"type": "string",
			},
		},
	}

	createResp, err := client.CreateResearchTask(context.Background(),
		"Research instructions", "gpt-4", schema, true)
	if err != nil {
		t.Errorf("CreateResearchTask() error = %v", err)
	}
	if createResp.TaskId != "task-123" {
		t.Errorf("Expected task ID 'task-123', got %s", createResp.TaskId)
	}

	// Test GetResearchTaskStatus
	statusResp, err := client.GetResearchTaskStatus(context.Background(), "task-123")
	if err != nil {
		t.Errorf("GetResearchTaskStatus() error = %v", err)
	}
	if statusResp.Status != "running" {
		t.Errorf("Expected status 'running', got %s", statusResp.Status)
	}

	// Test CancelResearchTask
	cancelResp, err := client.CancelResearchTask(context.Background(), "task-123")
	if err != nil {
		t.Errorf("CancelResearchTask() error = %v", err)
	}
	if !cancelResp.Success {
		t.Error("Expected successful cancellation")
	}

	// Test ListActiveResearchTasks
	listResp, err := client.ListActiveResearchTasks(context.Background(), false, 10)
	if err != nil {
		t.Errorf("ListActiveResearchTasks() error = %v", err)
	}
	if len(listResp.Tasks) != 1 {
		t.Errorf("Expected 1 active task, got %d", len(listResp.Tasks))
	}
}

func TestConvertSearchType(t *testing.T) {
	tests := []struct {
		input    SearchType
		expected gismov1.SearchRequest_SearchType
	}{
		{SearchTypeKeyword, gismov1.SearchRequest_KEYWORD},
		{SearchTypeSemantic, gismov1.SearchRequest_SEMANTIC},
		{SearchTypeHybrid, gismov1.SearchRequest_HYBRID},
		{"unknown", gismov1.SearchRequest_KEYWORD}, // Default case
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := convertSearchType(tt.input)
			if result != tt.expected {
				t.Errorf("convertSearchType(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetRuntimeDir(t *testing.T) {
	// Save original env
	oldXDG := os.Getenv("XDG_RUNTIME_DIR")
	defer os.Setenv("XDG_RUNTIME_DIR", oldXDG)

	// Test with XDG_RUNTIME_DIR
	os.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	dir, err := getRuntimeDir()
	if err != nil {
		t.Errorf("getRuntimeDir() error = %v", err)
	}
	if dir != "/run/user/1000/gismo" {
		t.Errorf("Expected '/run/user/1000/gismo', got %s", dir)
	}

	// Test without XDG_RUNTIME_DIR
	os.Unsetenv("XDG_RUNTIME_DIR")
	dir, err = getRuntimeDir()
	if err != nil {
		t.Errorf("getRuntimeDir() error = %v", err)
	}
	expected := filepath.Join(os.TempDir(), fmt.Sprintf("gismo-%d", os.Getuid()))
	if dir != expected {
		t.Errorf("Expected %s, got %s", expected, dir)
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	mock, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Configure mock to return errors
	mock.shouldError = true
	mock.errorCode = codes.Internal

	client, err := New()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test all methods handle errors properly
	_, err = client.ListDocsets(context.Background())
	if err == nil {
		t.Error("Expected error from ListDocsets")
	}

	err = client.RemoveDocset(context.Background(), "test")
	if err == nil {
		t.Error("Expected error from RemoveDocset")
	}

	_, err = client.Search(context.Background(), "test", SearchOptions{})
	if err == nil {
		t.Error("Expected error from Search")
	}

	_, err = client.GetContent(context.Background(), 1)
	if err == nil {
		t.Error("Expected error from GetContent")
	}

	_, err = client.ExecuteQuery(context.Background(), "SELECT 1", 10)
	if err == nil {
		t.Error("Expected error from ExecuteQuery")
	}

	err = client.ExecuteQueryStream(context.Background(), "SELECT 1",
		func(*gismov1.QueryResult) error { return nil })
	if err == nil {
		t.Error("Expected error from ExecuteQueryStream")
	}
}

// Benchmark tests
func BenchmarkNew(b *testing.B) {
	_, _, cleanup := setupTestServer(&testing.T{})
	defer cleanup()

	for i := 0; i < b.N; i++ {
		client, err := New()
		if err != nil {
			b.Fatal(err)
		}
		client.Close()
	}
}

func BenchmarkSearch(b *testing.B) {
	mock, _, cleanup := setupTestServer(&testing.T{})
	defer cleanup()

	mock.searchResults = []*gismov1.SearchResult{
		{
			DocsetId:       "docset-1",
			DocsetName:     "Test",
			ItemName:       "Result",
			ItemType:       "Function",
			ContentPreview: "Content",
			RelevanceScore: 0.9,
			ContentId:      1,
		},
	}

	client, err := New()
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	opts := SearchOptions{Type: SearchTypeKeyword, Limit: 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Search(ctx, "test", opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}
