package codesitter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

func TestNewServer(t *testing.T) {
	server := NewServer()

	if server == nil {
		t.Fatal("NewServer() returned nil")
	}

	if server.parsers == nil {
		t.Error("parsers map not initialized")
	}

	if server.trees == nil {
		t.Error("trees map not initialized")
	}

	if server.fileWatchers == nil {
		t.Error("fileWatchers map not initialized")
	}

	if server.symbolWatchers == nil {
		t.Error("symbolWatchers map not initialized")
	}

	if server.diagnosticWatchers == nil {
		t.Error("diagnosticWatchers map not initialized")
	}

	if server.metrics == nil {
		t.Error("metrics not initialized")
	}

	if server.maxFileSize != 10*1024*1024 {
		t.Errorf("Expected maxFileSize to be 10MB, got %d", server.maxFileSize)
	}
}

func TestInitializeWorkspace(t *testing.T) {
	server := NewServer()

	// Create a temp workspace
	tmpDir := t.TempDir()

	// Create some test files
	goFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(goFile, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	req := &gismov1.InitializeWorkspaceRequest{
		WorkspaceRoot:            tmpDir,
		FilePatterns:             []string{"*.go"},
		ExcludePatterns:          []string{"vendor/**"},
		Languages:                []string{"go"},
		MaxFileSize:              1024 * 1024,
		EnableFileWatching:       true,
		EnableIncrementalParsing: true,
	}

	ctx := context.Background()
	resp, err := server.InitializeWorkspace(ctx, req)

	if err != nil {
		t.Fatalf("InitializeWorkspace() error = %v", err)
	}

	if resp == nil {
		t.Fatal("InitializeWorkspace() returned nil response")
	}

	if !resp.Success {
		t.Errorf("InitializeWorkspace() success = false, want true")
	}

	// Check that at least one file was parsed
	if resp.FilesParsed == 0 {
		t.Error("No files were parsed")
	}

	if resp.SessionId == "" {
		t.Error("SessionId is empty")
	}

	// Check that server state was updated
	server.mu.RLock()
	defer server.mu.RUnlock()

	if server.workspaceRoot != tmpDir {
		t.Errorf("workspaceRoot = %v, want %v", server.workspaceRoot, tmpDir)
	}

	// sessionID is managed internally, no need to check
}

func TestInitializeParsers(t *testing.T) {
	server := NewServer()

	tests := []struct {
		name          string
		languages     []string
		wantErr       bool
		expectParsers []string
	}{
		{
			name:          "supported languages",
			languages:     []string{"go", "javascript", "python"},
			wantErr:       false,
			expectParsers: []string{"go", "javascript", "python"},
		},
		{
			name:          "unsupported language",
			languages:     []string{"cobol"},
			wantErr:       true, // should error when no parsers initialized
			expectParsers: []string{},
		},
		{
			name:          "mixed languages",
			languages:     []string{"go", "unsupported"},
			wantErr:       false, // at least one parser initialized
			expectParsers: []string{"go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset parsers for each test
			server.parsers = make(map[string]*sitter.Parser)

			err := server.initializeParsers(tt.languages)
			if (err != nil) != tt.wantErr {
				t.Errorf("initializeParsers() error = %v, wantErr %v", err, tt.wantErr)
			}

			for _, lang := range tt.expectParsers {
				if _, ok := server.parsers[lang]; !ok {
					t.Errorf("Expected parser for %s not found", lang)
				}
			}
		})
	}
}

func TestDetectLanguage(t *testing.T) {
	server := NewServer()

	tests := []struct {
		path     string
		expected string
	}{
		{"test.go", "go"},
		{"test.js", "javascript"},
		{"test.jsx", ""}, // .jsx is not mapped in detectLanguage
		{"test.ts", "typescript"},
		{"test.tsx", "tsx"},
		{"test.py", "python"},
		{"test.unknown", ""},
		{"/path/to/file.go", "go"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := server.detectLanguage(tt.path)
			if result != tt.expected {
				t.Errorf("detectLanguage(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestMatchesPatterns(t *testing.T) {
	server := NewServer()

	tests := []struct {
		name     string
		path     string
		patterns []string
		expected bool
	}{
		{
			name:     "matches go files",
			path:     "main.go",
			patterns: []string{"*.go"},
			expected: true,
		},
		{
			name:     "matches nested go files",
			path:     "pkg/test/file.go",
			patterns: []string{"*.go"}, // filepath.Match doesn't support **
			expected: true,
		},
		{
			name:     "no match",
			path:     "main.py",
			patterns: []string{"*.go"},
			expected: false,
		},
		{
			name:     "empty patterns",
			path:     "main.go",
			patterns: []string{},
			expected: true,
		},
		{
			name:     "multiple patterns",
			path:     "main.go",
			patterns: []string{"*.py", "*.go"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.matchesPatterns(tt.path, tt.patterns)
			if result != tt.expected {
				t.Errorf("matchesPatterns(%q, %v) = %v, want %v", tt.path, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestIsExcluded(t *testing.T) {
	server := NewServer()
	server.excludePaths = []string{"*.test"}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/vendor/github.com/test/file.go", true}, // vendor is always excluded (needs / separator)
		{"/node_modules/package/index.js", true},  // node_modules is always excluded (needs / separator)
		{"file.test", true},                       // matches *.test pattern
		{"src/main.go", false},
		{"test/file.go", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := server.isExcluded(tt.path)
			if result != tt.expected {
				t.Errorf("isExcluded(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	server := NewServer()
	server.symbolIndex = NewSymbolIndex() // Initialize symbol index

	// Initialize Go parser
	if err := server.initializeParsers([]string{"go"}); err != nil {
		t.Fatal(err)
	}

	// Create a temp Go file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

func helper() int {
	return 42
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse the file
	fileTree, err := server.parseFile(goFile)
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if fileTree == nil {
		t.Fatal("parseFile() returned nil")
	}

	if fileTree.Path != goFile {
		t.Errorf("Path = %v, want %v", fileTree.Path, goFile)
	}

	if fileTree.Language != "go" {
		t.Errorf("Language = %v, want go", fileTree.Language)
	}

	if fileTree.Tree == nil {
		t.Error("Tree is nil")
	}

	if string(fileTree.Content) != goContent {
		t.Error("Content doesn't match")
	}

	// Check that symbols were extracted
	if len(fileTree.Symbols) == 0 {
		t.Error("No symbols extracted")
	}

	// Verify we found the main and helper functions
	foundMain := false
	foundHelper := false
	for _, sym := range fileTree.Symbols {
		if sym.Name == "main" {
			foundMain = true
		}
		if sym.Name == "helper" {
			foundHelper = true
		}
	}

	if !foundMain {
		t.Error("main function not found in symbols")
	}

	if !foundHelper {
		t.Error("helper function not found in symbols")
	}
}

func TestParseFileErrors(t *testing.T) {
	server := NewServer()

	// Test parsing non-existent file
	_, err := server.parseFile("/non/existent/file.go")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Test parsing file with no parser
	tmpDir := t.TempDir()
	unknownFile := filepath.Join(tmpDir, "test.unknown")
	if err := os.WriteFile(unknownFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = server.parseFile(unknownFile)
	if err == nil {
		t.Error("Expected error for unsupported language")
	}

	// Test parsing file that's too large
	server.maxFileSize = 10 // 10 bytes
	largeFile := filepath.Join(tmpDir, "large.go")
	if err := os.WriteFile(largeFile, []byte(strings.Repeat("a", 100)), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = server.parseFile(largeFile)
	if err == nil {
		t.Error("Expected error for file too large")
	}
}

func TestResolveFilePath(t *testing.T) {
	server := NewServer()
	server.workspaceRoot = "/workspace"

	tests := []struct {
		path     string
		expected string
	}{
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "/workspace/relative/path"},
		{"", "/workspace"}, // filepath.Join("/workspace", "") returns "/workspace"
		{"./file.go", "/workspace/file.go"},
		{"../file.go", "/file.go"}, // filepath.Join cleans the path
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := server.resolveFilePath(tt.path)
			if result != tt.expected {
				t.Errorf("resolveFilePath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFileTreeConcurrency(t *testing.T) {
	server := NewServer()
	server.symbolIndex = NewSymbolIndex() // Initialize symbol index

	// Initialize parser
	if err := server.initializeParsers([]string{"go"}); err != nil {
		t.Fatal(err)
	}

	// Create a test file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(goFile, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse file
	ft, err := server.parseFile(goFile)
	if err != nil {
		t.Fatal(err)
	}

	// Store in server
	server.mu.Lock()
	server.trees[goFile] = ft
	server.mu.Unlock()

	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Read operation
			server.mu.RLock()
			tree := server.trees[goFile]
			server.mu.RUnlock()

			if tree == nil {
				t.Errorf("Goroutine %d: tree is nil", id)
			}

			// Write operation
			if id%2 == 0 {
				server.mu.Lock()
				server.trees[goFile].LastModified = time.Now()
				server.mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
}

func TestExtractSymbols(t *testing.T) {
	server := NewServer()

	// Initialize parser
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	server.parsers["go"] = parser

	goContent := []byte(`package main

type MyStruct struct {
	Field1 string
	Field2 int
}

func (m *MyStruct) Method() {
	// method body
}

func Function() {
	// function body
}

var GlobalVar = "test"

const GlobalConst = 42
`)

	tree, err := parser.ParseCtx(context.Background(), nil, goContent)
	if err != nil {
		t.Fatal(err)
	}

	ft := &FileTree{
		Path:     "test.go",
		Language: "go",
		Content:  goContent,
		Tree:     tree,
	}

	symbols := server.extractSymbols(ft)

	// Just verify that extractSymbols was called and returned something
	// The actual extraction is tested in extractors_test.go
	if symbols == nil {
		t.Error("extractSymbols returned nil")
	}

	// If there are symbols, they should have basic fields set
	for _, sym := range symbols {
		if sym.Name == "" {
			t.Error("Symbol has empty name")
		}
		if sym.Kind == gismov1.SymbolKind_SYMBOL_KIND_UNSPECIFIED {
			t.Error("Symbol has unspecified kind")
		}
	}
}

func TestNotifyFileChange(t *testing.T) {
	server := NewServer()

	// Create a watcher channel
	watcherID := "test-watcher"
	watcherChan := make(chan *gismov1.FileChangeEvent, 10)
	server.fileWatchers[watcherID] = watcherChan

	// Send notification
	go server.notifyFileChange("/test/file.go", gismov1.FileChangeKind_FILE_CHANGE_KIND_CREATED)

	// Wait for event
	select {
	case event := <-watcherChan:
		if event.FilePath != "/test/file.go" {
			t.Errorf("FilePath = %v, want /test/file.go", event.FilePath)
		}
		if event.Kind != gismov1.FileChangeKind_FILE_CHANGE_KIND_CREATED {
			t.Errorf("Kind = %v, want FILE_CHANGE_KIND_CREATED", event.Kind)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for file change event")
	}
}

func TestServerMetrics(t *testing.T) {
	server := NewServer()

	// Update metrics
	server.metrics.FilesIndexed = 10
	server.metrics.SymbolsIndexed = 100
	server.metrics.ParseTime = 5 * time.Second
	server.metrics.QueryTime = 2 * time.Second

	// Verify metrics
	if server.metrics.FilesIndexed != 10 {
		t.Errorf("FilesIndexed = %v, want 10", server.metrics.FilesIndexed)
	}

	if server.metrics.SymbolsIndexed != 100 {
		t.Errorf("SymbolsIndexed = %v, want 100", server.metrics.SymbolsIndexed)
	}

	if server.metrics.ParseTime != 5*time.Second {
		t.Errorf("ParseTime = %v, want 5s", server.metrics.ParseTime)
	}

	if server.metrics.QueryTime != 2*time.Second {
		t.Errorf("QueryTime = %v, want 2s", server.metrics.QueryTime)
	}
}

// Benchmark tests
func BenchmarkParseFile(b *testing.B) {
	server := NewServer()

	// Initialize parser
	if err := server.initializeParsers([]string{"go"}); err != nil {
		b.Fatal(err)
	}

	// Create a test file
	tmpDir := b.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

import "fmt"

func main() {
	for i := 0; i < 100; i++ {
		fmt.Println(i)
	}
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := server.parseFile(goFile)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtractSymbols(b *testing.B) {
	server := NewServer()

	// Initialize parser
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	server.parsers["go"] = parser

	goContent := []byte(`package main

type A struct{ Field string }
type B struct{ Field int }
type C struct{ Field bool }

func Function1() {}
func Function2() {}
func Function3() {}

var Var1 = "test"
var Var2 = 42
var Var3 = true
`)

	tree, err := parser.ParseCtx(context.Background(), nil, goContent)
	if err != nil {
		b.Fatal(err)
	}

	ft := &FileTree{
		Path:     "test.go",
		Language: "go",
		Content:  goContent,
		Tree:     tree,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.extractSymbols(ft)
	}
}
