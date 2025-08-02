package toolcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestGetCacheManager(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Change to the temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Test getting cache manager
	manager, err := GetCacheManager(tmpDir)
	if err != nil {
		t.Fatalf("GetCacheManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("Expected non-nil cache manager")
	}

	if manager.gitRoot != claudeDir {
		t.Errorf("Expected gitRoot to be %s, got %s", claudeDir, manager.gitRoot)
	}

	expectedCachePath := filepath.Join(claudeDir, "gismo-tools.json")
	if manager.cachePath != expectedCachePath {
		t.Errorf("Expected cachePath to be %s, got %s", expectedCachePath, manager.cachePath)
	}
}

func TestCacheManager_CreateNewCache(t *testing.T) {
	manager := &CacheManager{
		gitRoot: "/test/git/root",
	}

	manager.createNewCache()

	if manager.cache == nil {
		t.Fatal("Expected cache to be created")
	}

	if manager.cache.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", manager.cache.Version)
	}

	if manager.cache.GitRoot != "/test/git/root" {
		t.Errorf("Expected git root to match, got %s", manager.cache.GitRoot)
	}

	hostname, _ := os.Hostname()
	if manager.cache.Hostname != hostname {
		t.Errorf("Expected hostname %s, got %s", hostname, manager.cache.Hostname)
	}

	if manager.cache.Projects.Configs == nil {
		t.Error("Expected Projects.Configs to be initialized")
	}

	if manager.cache.Performance.ToolPerformance == nil {
		t.Error("Expected Performance.ToolPerformance to be initialized")
	}

	if manager.cache.Performance.LinterStats == nil {
		t.Error("Expected Performance.LinterStats to be initialized")
	}
}

func TestCacheManager_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test-cache.json")

	manager := &CacheManager{
		gitRoot:   tmpDir,
		cachePath: cachePath,
	}

	// Create a cache with test data
	manager.createNewCache()

	// Add some test data
	testTool := &ToolInfo{
		Path:      "/usr/bin/go",
		Version:   "go1.21.0",
		Available: true,
		LastCheck: time.Now(),
		Source:    "global",
	}
	manager.cache.Tools.Go.Go = testTool

	// Save the cache
	if err := manager.save(); err != nil {
		t.Fatalf("Failed to save cache: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("Cache file not created: %v", err)
	}

	// Create a new manager and load the cache
	newManager := &CacheManager{
		gitRoot:   tmpDir,
		cachePath: cachePath,
	}

	if err := newManager.loadCache(); err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}

	// Verify the loaded data
	if newManager.cache.Tools.Go.Go == nil {
		t.Fatal("Expected Go tool to be loaded")
	}

	if newManager.cache.Tools.Go.Go.Path != "/usr/bin/go" {
		t.Errorf("Expected Go path to be /usr/bin/go, got %s", newManager.cache.Tools.Go.Go.Path)
	}

	if newManager.cache.Tools.Go.Go.Version != "go1.21.0" {
		t.Errorf("Expected Go version to be go1.21.0, got %s", newManager.cache.Tools.Go.Go.Version)
	}
}

func TestCacheManager_GetTool(t *testing.T) {
	manager := &CacheManager{}
	manager.createNewCache()

	// Add test tools
	manager.cache.Tools.Go.GolangciLint = &ToolInfo{
		Path:      "/usr/local/bin/golangci-lint",
		Available: true,
		Version:   "1.54.0",
	}

	manager.cache.Tools.JavaScript.ESLint = &ToolInfo{
		Path:      "/usr/local/bin/eslint",
		Available: true,
		Version:   "8.0.0",
	}

	tests := []struct {
		category string
		toolName string
		expected *ToolInfo
	}{
		{"go", "golangci-lint", manager.cache.Tools.Go.GolangciLint},
		{"javascript", "eslint", manager.cache.Tools.JavaScript.ESLint},
		{"go", "nonexistent", nil},
		{"invalid", "tool", nil},
	}

	for _, tt := range tests {
		t.Run(tt.category+"/"+tt.toolName, func(t *testing.T) {
			result := manager.GetTool(tt.category, tt.toolName)
			if result != tt.expected {
				t.Errorf("GetTool(%s, %s) = %v, want %v", tt.category, tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestCacheManager_UpdateTool(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test-cache.json")

	manager := &CacheManager{
		gitRoot:   tmpDir,
		cachePath: cachePath,
	}
	manager.createNewCache()

	// Update a tool
	newTool := &ToolInfo{
		Path:      "/opt/go/bin/go",
		Version:   "go1.22.0",
		Available: true,
		LastCheck: time.Now(),
	}

	if err := manager.UpdateTool("go", "go", newTool); err != nil {
		t.Fatalf("UpdateTool failed: %v", err)
	}

	// Verify the update
	if manager.cache.Tools.Go.Go == nil {
		t.Fatal("Expected Go tool to be set")
	}

	if manager.cache.Tools.Go.Go.Path != "/opt/go/bin/go" {
		t.Errorf("Expected updated path, got %s", manager.cache.Tools.Go.Go.Path)
	}

	// Verify it was saved to disk
	if _, err := os.Stat(cachePath); err != nil {
		t.Error("Expected cache file to be created")
	}
}

func TestCacheManager_DiscoverTool(t *testing.T) {
	tmpDir := t.TempDir()
	manager := &CacheManager{
		gitRoot:   tmpDir,
		cachePath: filepath.Join(tmpDir, "cache.json"),
	}
	manager.createNewCache()

	// Test discovering a tool that should exist on most systems
	tool, err := manager.DiscoverTool("system", "grep")
	if err != nil {
		t.Fatalf("DiscoverTool failed: %v", err)
	}

	if tool == nil {
		t.Fatal("Expected tool to be discovered")
	}

	// On most Unix systems, grep should be available
	if runtime.GOOS != "windows" {
		if !tool.Available {
			t.Error("Expected grep to be available on Unix systems")
		}
	}

	// Test discovering a non-existent tool
	tool, err = manager.DiscoverTool("go", "nonexistent-tool-xyz")
	if err != nil {
		t.Fatalf("DiscoverTool failed: %v", err)
	}

	if tool.Available {
		t.Error("Expected non-existent tool to not be available")
	}
}

func TestCacheManager_IsToolCacheFresh(t *testing.T) {
	manager := &CacheManager{}

	// Create a test file
	tmpFile, err := os.CreateTemp("", "test-tool-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	stat, _ := os.Stat(tmpFile.Name())

	tests := []struct {
		name     string
		tool     *ToolInfo
		expected bool
	}{
		{
			name: "fresh tool",
			tool: &ToolInfo{
				Path:      tmpFile.Name(),
				LastCheck: time.Now(),
				ModTime:   stat.ModTime(),
			},
			expected: true,
		},
		{
			name: "old check",
			tool: &ToolInfo{
				Path:      tmpFile.Name(),
				LastCheck: time.Now().Add(-48 * time.Hour),
				ModTime:   stat.ModTime(),
			},
			expected: false,
		},
		{
			name: "non-existent file",
			tool: &ToolInfo{
				Path:      "/non/existent/file",
				LastCheck: time.Now(),
			},
			expected: false,
		},
		{
			name: "modified file",
			tool: &ToolInfo{
				Path:      tmpFile.Name(),
				LastCheck: time.Now(),
				ModTime:   time.Now().Add(-1 * time.Hour),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.isToolCacheFresh(tt.tool)
			if result != tt.expected {
				t.Errorf("isToolCacheFresh() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSystemMetrics(t *testing.T) {
	metrics := getSystemMetrics()

	if metrics.CPUCores != runtime.NumCPU() {
		t.Errorf("Expected CPU cores to be %d, got %d", runtime.NumCPU(), metrics.CPUCores)
	}

	if metrics.OS != runtime.GOOS {
		t.Errorf("Expected OS to be %s, got %s", runtime.GOOS, metrics.OS)
	}

	if metrics.Architecture != runtime.GOARCH {
		t.Errorf("Expected architecture to be %s, got %s", runtime.GOARCH, metrics.Architecture)
	}
}

func TestCacheValidation(t *testing.T) {
	manager := &CacheManager{
		gitRoot: "/test/path",
	}

	// Test mismatched git root
	cache := UniversalToolCache{
		GitRoot:  "/different/path",
		Hostname: "test-host",
	}
	data, _ := json.Marshal(cache)

	tmpFile, err := os.CreateTemp("", "cache-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if err := os.WriteFile(tmpFile.Name(), data, 0600); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	manager.cachePath = tmpFile.Name()
	err = manager.loadCache()
	if err == nil {
		t.Error("Expected error for mismatched git root")
	}

	// Test mismatched hostname
	hostname, _ := os.Hostname()
	cache = UniversalToolCache{
		GitRoot:  "/test/path",
		Hostname: "different-host",
	}
	data, _ = json.Marshal(cache)

	if err := os.WriteFile(tmpFile.Name(), data, 0600); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	err = manager.loadCache()
	if err == nil && hostname != "different-host" {
		t.Error("Expected error for mismatched hostname")
	}
}

func TestFindClaudeDir(t *testing.T) {
	// Test finding existing .claude directory
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, "subdir", ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	workDir := filepath.Join(tmpDir, "subdir", "project")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("Failed to create work directory: %v", err)
	}

	found, err := findClaudeDir(workDir)
	if err != nil {
		t.Fatalf("findClaudeDir failed: %v", err)
	}

	if found != claudeDir {
		t.Errorf("Expected to find %s, got %s", claudeDir, found)
	}

	// Test creating new .claude directory
	newWorkDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(newWorkDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	found, err = findClaudeDir(newWorkDir)
	if err != nil {
		t.Fatalf("findClaudeDir failed: %v", err)
	}

	expectedDir := filepath.Join(newWorkDir, ".claude")
	// Clean both paths to handle symlinks (e.g., /var vs /private/var on macOS)
	foundClean, _ := filepath.EvalSymlinks(found)
	expectedClean, _ := filepath.EvalSymlinks(expectedDir)

	if foundClean != expectedClean {
		t.Errorf("Expected to create %s, got %s", expectedClean, foundClean)
	}

	// Verify directory was created
	if _, err := os.Stat(expectedDir); err != nil {
		t.Error("Expected .claude directory to be created")
	}
}

// Benchmark cache operations
func BenchmarkCacheManager_Save(b *testing.B) {
	tmpDir := b.TempDir()
	manager := &CacheManager{
		gitRoot:   tmpDir,
		cachePath: filepath.Join(tmpDir, "bench-cache.json"),
	}
	manager.createNewCache()

	// Add some data
	for i := 0; i < 10; i++ {
		tool := &ToolInfo{
			Path:      "/usr/bin/tool" + string(rune(i)),
			Available: true,
			Version:   "1.0.0",
		}
		manager.cache.Tools.Go.Go = tool
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.save()
	}
}

func BenchmarkCacheManager_Load(b *testing.B) {
	tmpDir := b.TempDir()
	cachePath := filepath.Join(tmpDir, "bench-cache.json")

	// Create a cache file
	manager := &CacheManager{
		gitRoot:   tmpDir,
		cachePath: cachePath,
	}
	manager.createNewCache()
	_ = manager.save()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newManager := &CacheManager{
			gitRoot:   tmpDir,
			cachePath: cachePath,
		}
		_ = newManager.loadCache()
	}
}
