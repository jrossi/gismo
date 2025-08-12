package codesitter

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	gismov1 "github.com/jrossi/gismo/pkg/generated/gismo/v1"
)

// Server implements the CodeSitter gRPC service
type Server struct {
	gismov1.UnimplementedCodeSitterServer

	mu sync.RWMutex

	// Configuration
	workspaceRoot string
	sessionID     string
	filePatterns  []string
	excludePaths  []string
	maxFileSize   int32

	// Parsers by language
	parsers map[string]*sitter.Parser

	// Cached parse trees
	trees map[string]*FileTree

	// Symbol index for fast queries
	symbolIndex *SymbolIndex

	// File watcher
	watcher *fsnotify.Watcher

	// Streaming subscribers
	fileWatchers       map[string]chan *gismov1.FileChangeEvent
	symbolWatchers     map[string]chan *gismov1.SymbolChangeEvent
	diagnosticWatchers map[string]chan *gismov1.DiagnosticEvent

	// Metrics
	metrics *ServerMetrics
}

// FileTree represents a parsed file with its AST
type FileTree struct {
	Path         string
	Language     string
	Content      []byte
	Tree         *sitter.Tree
	LastModified time.Time
	Symbols      []*gismov1.Symbol
	Diagnostics  []*gismov1.Diagnostic
	mu           sync.RWMutex
}

// ServerMetrics tracks server performance
type ServerMetrics struct {
	FilesIndexed   int64
	SymbolsIndexed int64
	ParseTime      time.Duration
	QueryTime      time.Duration
}

// NewServer creates a new CodeSitter server
func NewServer() *Server {
	return &Server{
		parsers:            make(map[string]*sitter.Parser),
		trees:              make(map[string]*FileTree),
		fileWatchers:       make(map[string]chan *gismov1.FileChangeEvent),
		symbolWatchers:     make(map[string]chan *gismov1.SymbolChangeEvent),
		diagnosticWatchers: make(map[string]chan *gismov1.DiagnosticEvent),
		metrics:            &ServerMetrics{},
		maxFileSize:        10 * 1024 * 1024, // 10MB default
	}
}

// InitializeWorkspace sets up the workspace for analysis
func (s *Server) InitializeWorkspace(ctx context.Context, req *gismov1.InitializeWorkspaceRequest) (*gismov1.InitializeWorkspaceResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store configuration
	s.workspaceRoot = req.WorkspaceRoot
	s.filePatterns = req.FilePatterns
	s.excludePaths = req.ExcludePatterns
	s.sessionID = generateSessionID()

	if req.MaxFileSize > 0 {
		s.maxFileSize = req.MaxFileSize
	}

	// Initialize parsers for requested languages
	if err := s.initializeParsers(req.Languages); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to initialize parsers: %v", err)
	}

	// Create symbol index
	s.symbolIndex = NewSymbolIndex()

	// Parse all matching files
	filesByLang := make(map[string]int32)
	err := filepath.Walk(s.workspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check file size
		if info.Size() > int64(s.maxFileSize) {
			return nil
		}

		// Check if file matches patterns
		if !s.matchesPatterns(path, s.filePatterns) || s.isExcluded(path) {
			return nil
		}

		// Parse the file
		if tree, err := s.parseFile(path); err == nil {
			lang := tree.Language
			filesByLang[lang]++
			s.metrics.FilesIndexed++
		}

		return nil
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to walk workspace: %v", err)
	}

	// Start file watcher if requested
	if req.EnableFileWatching {
		if err := s.startFileWatcher(); err != nil {
			// Non-fatal: continue without watching
			fmt.Printf("Warning: failed to start file watcher: %v\n", err)
		}
	}

	// Get supported languages
	supportedLangs := make([]string, 0, len(s.parsers))
	for lang := range s.parsers {
		supportedLangs = append(supportedLangs, lang)
	}

	return &gismov1.InitializeWorkspaceResponse{
		Success:              true,
		FilesParsed:          int32(s.metrics.FilesIndexed), //nolint:gosec // safe metric conversion
		TotalSymbols:         int32(s.symbolIndex.Count()),  //nolint:gosec
		SupportedLanguages:   supportedLangs,
		SessionId:            s.sessionID,
		FileCountsByLanguage: filesByLang,
	}, nil
}

// initializeParsers sets up language parsers
func (s *Server) initializeParsers(languages []string) error {
	// If no languages specified, initialize all supported
	if len(languages) == 0 {
		languages = []string{"go", "javascript", "typescript", "python"}
	}

	for _, lang := range languages {
		parser := sitter.NewParser()

		switch strings.ToLower(lang) {
		case "go", "golang":
			parser.SetLanguage(golang.GetLanguage())
			s.parsers["go"] = parser
		case "javascript", "js":
			parser.SetLanguage(javascript.GetLanguage())
			s.parsers["javascript"] = parser
		case "typescript", "ts":
			parser.SetLanguage(typescript.GetLanguage())
			s.parsers["typescript"] = parser
		case "tsx":
			parser.SetLanguage(tsx.GetLanguage())
			s.parsers["tsx"] = parser
		case "python", "py":
			parser.SetLanguage(python.GetLanguage())
			s.parsers["python"] = parser
		default:
			// Skip unsupported languages
			fmt.Printf("Warning: language %s not supported\n", lang)
		}
	}

	if len(s.parsers) == 0 {
		return fmt.Errorf("no parsers initialized")
	}

	return nil
}

// parseFile parses a single file and caches the result
func (s *Server) parseFile(path string) (*FileTree, error) {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Detect language from extension
	lang := s.detectLanguage(path)
	if lang == "" {
		return nil, fmt.Errorf("unsupported file type: %s", path)
	}

	// Get parser for language
	parser, ok := s.parsers[lang]
	if !ok {
		return nil, fmt.Errorf("no parser for language: %s", lang)
	}

	// Parse the file
	startTime := time.Now()
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	s.metrics.ParseTime += time.Since(startTime)

	// Create file tree
	fileTree := &FileTree{
		Path:         path,
		Language:     lang,
		Content:      content,
		Tree:         tree,
		LastModified: time.Now(),
		Symbols:      make([]*gismov1.Symbol, 0),
		Diagnostics:  make([]*gismov1.Diagnostic, 0),
	}

	// Extract symbols
	fileTree.Symbols = s.extractSymbols(fileTree)

	// Add to symbol index
	for _, symbol := range fileTree.Symbols {
		s.symbolIndex.Add(symbol)
	}

	// Cache the tree
	s.trees[path] = fileTree

	return fileTree, nil
}

// detectLanguage determines the language from file extension
func (s *Server) detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".py":
		return "python"
	default:
		return ""
	}
}

// extractSymbols extracts symbols from a parsed tree
func (s *Server) extractSymbols(ft *FileTree) []*gismov1.Symbol {
	symbols := make([]*gismov1.Symbol, 0)

	// Language-specific symbol extraction
	switch ft.Language {
	case "go":
		symbols = s.extractGoSymbols(ft)
	case "javascript", "typescript", "tsx":
		symbols = s.extractJSSymbols(ft)
	case "python":
		symbols = s.extractPythonSymbols(ft)
	}

	return symbols
}

// matchesPatterns checks if a path matches any of the given patterns
func (s *Server) matchesPatterns(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return true // No patterns means match all
	}

	relPath, err := filepath.Rel(s.workspaceRoot, path)
	if err != nil {
		return false
	}

	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, relPath)
		if err == nil && matched {
			return true
		}
		// Also check against the base name
		matched, err = filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
	}
	return false
}

// isExcluded checks if a path should be excluded
func (s *Server) isExcluded(path string) bool {
	// Always exclude common non-source directories
	excludeDirs := []string{".git", "node_modules", ".venv", "vendor", "build", "dist"}
	for _, dir := range excludeDirs {
		if strings.Contains(path, string(os.PathSeparator)+dir+string(os.PathSeparator)) {
			return true
		}
	}

	// Check custom exclude patterns
	for _, pattern := range s.excludePaths {
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// startFileWatcher starts watching for file changes
func (s *Server) startFileWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	s.watcher = watcher

	// Start the watcher goroutine
	go s.watchFiles()

	// Add workspace root to watcher
	return filepath.Walk(s.workspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && !s.isExcluded(path) {
			if err := watcher.Add(path); err != nil {
				log.Printf("Failed to add watcher for %s: %v", path, err)
			}
		}
		return nil
	})
}

// watchFiles handles file system events
func (s *Server) watchFiles() {
	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			s.handleFileEvent(event)
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Watcher error: %v\n", err)
		}
	}
}

// handleFileEvent processes a file system event
func (s *Server) handleFileEvent(event fsnotify.Event) {
	// Check if file matches our patterns
	if !s.matchesPatterns(event.Name, s.filePatterns) || s.isExcluded(event.Name) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		// File modified - reparse
		if oldTree, exists := s.trees[event.Name]; exists {
			if newTree, err := s.reparseFile(event.Name, oldTree); err == nil {
				s.notifyFileChange(event.Name, gismov1.FileChangeKind_FILE_CHANGE_KIND_MODIFIED)
				s.detectSymbolChanges(oldTree, newTree)
			}
		}
	case event.Op&fsnotify.Create == fsnotify.Create:
		// New file - parse it
		if _, err := s.parseFile(event.Name); err == nil {
			s.notifyFileChange(event.Name, gismov1.FileChangeKind_FILE_CHANGE_KIND_CREATED)
		}
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		// File deleted - remove from cache
		if tree, exists := s.trees[event.Name]; exists {
			for _, symbol := range tree.Symbols {
				s.symbolIndex.Remove(symbol)
			}
			delete(s.trees, event.Name)
			s.notifyFileChange(event.Name, gismov1.FileChangeKind_FILE_CHANGE_KIND_DELETED)
		}
	}
}

// reparseFile incrementally reparses a modified file
func (s *Server) reparseFile(path string, oldTree *FileTree) (*FileTree, error) {
	// Read new content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Get parser
	parser, ok := s.parsers[oldTree.Language]
	if !ok {
		return nil, fmt.Errorf("no parser for language: %s", oldTree.Language)
	}

	// Parse with old tree for incremental parsing
	newTree, err := parser.ParseCtx(context.Background(), oldTree.Tree, content)
	if err != nil {
		return nil, err
	}

	// Create new file tree
	fileTree := &FileTree{
		Path:         path,
		Language:     oldTree.Language,
		Content:      content,
		Tree:         newTree,
		LastModified: time.Now(),
	}

	// Extract symbols
	fileTree.Symbols = s.extractSymbols(fileTree)

	// Update symbol index
	for _, oldSymbol := range oldTree.Symbols {
		s.symbolIndex.Remove(oldSymbol)
	}
	for _, newSymbol := range fileTree.Symbols {
		s.symbolIndex.Add(newSymbol)
	}

	// Update cache
	s.trees[path] = fileTree

	// Clean up old tree
	oldTree.Tree.Close()

	return fileTree, nil
}

// notifyFileChange sends file change events to watchers
func (s *Server) notifyFileChange(path string, kind gismov1.FileChangeKind) {
	event := &gismov1.FileChangeEvent{
		FilePath:  path,
		Kind:      kind,
		Timestamp: timestamppb.Now(),
	}

	for _, ch := range s.fileWatchers {
		select {
		case ch <- event:
		default:
			// Don't block if subscriber is slow
		}
	}
}

// detectSymbolChanges compares symbols between old and new trees
func (s *Server) detectSymbolChanges(oldTree, newTree *FileTree) {
	// Create maps for comparison
	oldSymbols := make(map[string]*gismov1.Symbol)
	for _, sym := range oldTree.Symbols {
		oldSymbols[sym.Name] = sym
	}

	newSymbols := make(map[string]*gismov1.Symbol)
	for _, sym := range newTree.Symbols {
		newSymbols[sym.Name] = sym
	}

	// Find added and modified symbols
	for name, newSym := range newSymbols {
		if oldSym, exists := oldSymbols[name]; exists {
			// Check if modified
			if !symbolsEqual(oldSym, newSym) {
				s.notifySymbolChange(newSym, gismov1.SymbolChangeEvent_CHANGE_KIND_MODIFIED, "")
			}
		} else {
			// New symbol
			s.notifySymbolChange(newSym, gismov1.SymbolChangeEvent_CHANGE_KIND_ADDED, "")
		}
	}

	// Find deleted symbols
	for name, oldSym := range oldSymbols {
		if _, exists := newSymbols[name]; !exists {
			s.notifySymbolChange(oldSym, gismov1.SymbolChangeEvent_CHANGE_KIND_DELETED, "")
		}
	}
}

// notifySymbolChange sends symbol change events to watchers
func (s *Server) notifySymbolChange(symbol *gismov1.Symbol, kind gismov1.SymbolChangeEvent_ChangeKind, oldName string) {
	event := &gismov1.SymbolChangeEvent{
		Symbol:    symbol,
		Kind:      kind,
		OldName:   oldName,
		Timestamp: timestamppb.Now(),
	}

	for _, ch := range s.symbolWatchers {
		select {
		case ch <- event:
		default:
			// Don't block
		}
	}
}

// Helper functions

func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}

func symbolsEqual(a, b *gismov1.Symbol) bool {
	if a.Name != b.Name || a.Kind != b.Kind {
		return false
	}
	if a.Location.StartLine != b.Location.StartLine || a.Location.EndLine != b.Location.EndLine {
		return false
	}
	if a.Signature != b.Signature {
		return false
	}
	return true
}
