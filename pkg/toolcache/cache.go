package toolcache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// UniversalToolCache represents the complete tool cache for a project
type UniversalToolCache struct {
	// Cache metadata
	Version     string    `json:"version"`     // Cache format version
	LastUpdated time.Time `json:"lastUpdated"` // When cache was last updated
	GitRoot     string    `json:"gitRoot"`     // Git repository root path
	Hostname    string    `json:"hostname"`    // Machine hostname for cache validity

	// Universal tool discovery
	Tools       AllToolsCache `json:"tools"`       // All discovered tools
	Projects    ProjectCache  `json:"projects"`    // Project-specific configurations
	Performance MetricsCache  `json:"performance"` // Performance data for optimization
}

// AllToolsCache contains all tool categories
type AllToolsCache struct {
	// Language-specific linting tools
	Go         GoToolsCache         `json:"go"`
	JavaScript JavaScriptToolsCache `json:"javascript"`
	Python     PythonToolsCache     `json:"python"`
	JSON       JSONToolsCache       `json:"json"`
	Markdown   MarkdownToolsCache   `json:"markdown"`

	// System tools used across linters
	System  SystemToolsCache  `json:"system"`
	Git     GitToolsCache     `json:"git"`
	Runtime RuntimeToolsCache `json:"runtime"`
}

// Go ecosystem tools
type GoToolsCache struct {
	Go           *ToolInfo `json:"go,omitempty"`
	Gofmt        *ToolInfo `json:"gofmt,omitempty"`
	GolangciLint *ToolInfo `json:"golangci-lint,omitempty"`
	GoTest       *ToolInfo `json:"gotest,omitempty"`
	GoMod        *ToolInfo `json:"gomod,omitempty"`
	GoVet        *ToolInfo `json:"govet,omitempty"`
	StaticCheck  *ToolInfo `json:"staticcheck,omitempty"`
}

// JavaScript/TypeScript ecosystem tools
type JavaScriptToolsCache struct {
	Node *ToolInfo `json:"node,omitempty"`
	NPM  *ToolInfo `json:"npm,omitempty"`
	Yarn *ToolInfo `json:"yarn,omitempty"`
	PNPM *ToolInfo `json:"pnpm,omitempty"`

	// Linting tools
	Biome     *ToolInfo `json:"biome,omitempty"`
	Oxlint    *ToolInfo `json:"oxlint,omitempty"`
	ESLint    *ToolInfo `json:"eslint,omitempty"`
	QuickLint *ToolInfo `json:"quick-lint-js,omitempty"`

	// Formatting tools
	Prettier *ToolInfo `json:"prettier,omitempty"`
	Dprint   *ToolInfo `json:"dprint,omitempty"`

	// TypeScript tools
	TSC      *ToolInfo `json:"tsc,omitempty"`
	TSServer *ToolInfo `json:"tsserver,omitempty"`
}

// Python ecosystem tools
type PythonToolsCache struct {
	Python  *ToolInfo `json:"python,omitempty"`
	Python3 *ToolInfo `json:"python3,omitempty"`
	Pip     *ToolInfo `json:"pip,omitempty"`
	UV      *ToolInfo `json:"uv,omitempty"`

	// Linting and formatting tools
	Ruff   *ToolInfo `json:"ruff,omitempty"`
	Black  *ToolInfo `json:"black,omitempty"`
	Isort  *ToolInfo `json:"isort,omitempty"`
	Pylint *ToolInfo `json:"pylint,omitempty"`
	Flake8 *ToolInfo `json:"flake8,omitempty"`
	Mypy   *ToolInfo `json:"mypy,omitempty"`

	// Testing tools
	Pytest *ToolInfo `json:"pytest,omitempty"`
}

// JSON ecosystem tools
type JSONToolsCache struct {
	JQ       *ToolInfo `json:"jq,omitempty"`
	JSONLint *ToolInfo `json:"jsonlint,omitempty"`
	Prettier *ToolInfo `json:"prettier,omitempty"`
}

// Markdown ecosystem tools
type MarkdownToolsCache struct {
	Markdownlint *ToolInfo `json:"markdownlint,omitempty"`
	Prettier     *ToolInfo `json:"prettier,omitempty"`
	Pandoc       *ToolInfo `json:"pandoc,omitempty"`
	Vale         *ToolInfo `json:"vale,omitempty"`
}

// System tools used across multiple linters
type SystemToolsCache struct {
	Grep    *ToolInfo `json:"grep,omitempty"`
	Sed     *ToolInfo `json:"sed,omitempty"`
	Awk     *ToolInfo `json:"awk,omitempty"`
	Find    *ToolInfo `json:"find,omitempty"`
	Xargs   *ToolInfo `json:"xargs,omitempty"`
	Timeout *ToolInfo `json:"timeout,omitempty"`
	Kill    *ToolInfo `json:"kill,omitempty"`
}

// Git tools
type GitToolsCache struct {
	Git       *ToolInfo `json:"git,omitempty"`
	GitLFS    *ToolInfo `json:"git-lfs,omitempty"`
	Hub       *ToolInfo `json:"hub,omitempty"`
	GH        *ToolInfo `json:"gh,omitempty"`
	PreCommit *ToolInfo `json:"pre-commit,omitempty"`
}

// Runtime environments and package managers
type RuntimeToolsCache struct {
	Docker *ToolInfo `json:"docker,omitempty"`
	Podman *ToolInfo `json:"podman,omitempty"`
	NVM    *ToolInfo `json:"nvm,omitempty"`
	ASDF   *ToolInfo `json:"asdf,omitempty"`
	Pyenv  *ToolInfo `json:"pyenv,omitempty"`
	GVM    *ToolInfo `json:"gvm,omitempty"`
	Make   *ToolInfo `json:"make,omitempty"`
	Ninja  *ToolInfo `json:"ninja,omitempty"`
	Bazel  *ToolInfo `json:"bazel,omitempty"`
}

// ToolInfo contains comprehensive tool metadata
type ToolInfo struct {
	Path         string    `json:"path"`                   // Full path to tool binary
	Version      string    `json:"version,omitempty"`      // Tool version
	Available    bool      `json:"available"`              // Whether tool was found
	LastCheck    time.Time `json:"lastCheck"`              // When tool was last verified
	Source       string    `json:"source"`                 // "global", "local", "forced", "system"
	InstallType  string    `json:"installType,omitempty"`  // "npm", "pip", "go-install", "system", "homebrew"
	ConfigPath   string    `json:"configPath,omitempty"`   // Path to tool's config file
	Capabilities []string  `json:"capabilities,omitempty"` // ["lint", "format", "test", "type-check"]
	BinaryHash   string    `json:"binaryHash,omitempty"`   // Hash of binary for change detection
	ModTime      time.Time `json:"modTime,omitempty"`      // Binary modification time

	// Performance metadata
	LastRunTime *time.Duration `json:"lastRunTime,omitempty"` // Last execution time
	AvgRunTime  *time.Duration `json:"avgRunTime,omitempty"`  // Average execution time
	SuccessRate *float64       `json:"successRate,omitempty"` // Success rate (0.0-1.0)
}

// ProjectCache holds project-specific configurations
type ProjectCache struct {
	Configs map[string]ProjectConfig `json:"configs"` // Keyed by relative path from git root
}

// ProjectConfig contains project-specific configuration data
type ProjectConfig struct {
	ConfigFiles    map[string]string `json:"configFiles"`             // tool -> config path mapping
	ProjectType    []string          `json:"projectType"`             // ["go", "javascript", "python", "mixed"]
	PackageFiles   map[string]string `json:"packageFiles"`            // "package.json", "go.mod", "pyproject.toml", etc.
	WorkspaceRoot  string            `json:"workspaceRoot,omitempty"` // Monorepo root
	SubProjects    []string          `json:"subProjects,omitempty"`   // Sub-project paths
	LastDiscovered time.Time         `json:"lastDiscovered"`          // When config was discovered
	GitCommit      string            `json:"gitCommit,omitempty"`     // Git commit when cached
}

// MetricsCache holds performance metrics for optimization
type MetricsCache struct {
	ToolPerformance map[string]ToolMetrics `json:"toolPerformance"` // tool name -> metrics
	LinterStats     map[string]LinterStats `json:"linterStats"`     // linter -> stats
	SystemInfo      SystemMetrics          `json:"systemInfo"`      // CPU, memory, etc.
	LastUpdated     time.Time              `json:"lastUpdated"`     // When metrics were updated
}

// ToolMetrics tracks tool performance over time
type ToolMetrics struct {
	TotalRuns      int64         `json:"totalRuns"`
	TotalTime      time.Duration `json:"totalTime"`
	AverageTime    time.Duration `json:"averageTime"`
	SuccessfulRuns int64         `json:"successfulRuns"`
	FailedRuns     int64         `json:"failedRuns"`
	LastUsed       time.Time     `json:"lastUsed"`
}

// LinterStats tracks linter usage patterns
type LinterStats struct {
	FilesProcessed  int64  `json:"filesProcessed"`
	IssuesFound     int64  `json:"issuesFound"`
	AverageFileSize int64  `json:"averageFileSize"`
	PreferredTool   string `json:"preferredTool"` // Most successful tool
}

// SystemMetrics tracks system information for optimization
type SystemMetrics struct {
	CPUCores     int    `json:"cpuCores"`
	TotalMemory  int64  `json:"totalMemory"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Shell        string `json:"shell"`
}

// CacheManager manages the universal tool cache
type CacheManager struct {
	gitRoot     string
	cachePath   string
	cache       *UniversalToolCache
	mu          sync.RWMutex
	initialized bool
}

// Global cache manager instance
var globalCacheManager *CacheManager
var cacheManagerOnce sync.Once

// GetCacheManager returns the global cache manager for the current project
func GetCacheManager(currentPath string) (*CacheManager, error) {
	// Find .claude directory using existing config pattern
	claudeDir, err := findClaudeDir(currentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find .claude directory: %w", err)
	}

	cacheManagerOnce.Do(func() {
		globalCacheManager = &CacheManager{
			gitRoot:   claudeDir,
			cachePath: filepath.Join(claudeDir, "gismo-tools.json"),
		}
	})

	if globalCacheManager.gitRoot != claudeDir {
		// Different .claude directory, create new cache manager
		return &CacheManager{
			gitRoot:   claudeDir,
			cachePath: filepath.Join(claudeDir, "gismo-tools.json"),
		}, nil
	}

	if err := globalCacheManager.ensureInitialized(); err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	return globalCacheManager, nil
}

// ensureInitialized loads or creates the cache file
func (c *CacheManager) ensureInitialized() error {
	if c.initialized && !c.shouldRefreshCache() {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Create .claude directory if it doesn't exist
	cacheDir := filepath.Dir(c.cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Try to load existing cache
	if err := c.loadCache(); err != nil {
		// Create new cache if loading fails
		c.createNewCache()
	}

	c.initialized = true
	return nil
}

// shouldRefreshCache implements daily cache validation
func (c *CacheManager) shouldRefreshCache() bool {
	if !c.initialized {
		return true
	}

	stat, err := os.Stat(c.cachePath)
	if err != nil {
		return true // File doesn't exist, need to refresh
	}

	// Check if file was modified since yesterday
	yesterday := time.Now().AddDate(0, 0, -1)
	return stat.ModTime().Before(yesterday)
}

// loadCache reads and validates the cache file
func (c *CacheManager) loadCache() error {
	data, err := os.ReadFile(c.cachePath)
	if err != nil {
		return fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache UniversalToolCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return fmt.Errorf("failed to parse cache file: %w", err)
	}

	// Validate cache is for this machine and git repo
	if cache.GitRoot != c.gitRoot {
		return fmt.Errorf("cache git root mismatch: got %s, expected %s", cache.GitRoot, c.gitRoot)
	}

	hostname, _ := os.Hostname()
	if cache.Hostname != hostname {
		return fmt.Errorf("cache hostname mismatch: got %s, expected %s", cache.Hostname, hostname)
	}

	c.cache = &cache
	return nil
}

// createNewCache initializes a fresh cache
func (c *CacheManager) createNewCache() {
	hostname, _ := os.Hostname()

	c.cache = &UniversalToolCache{
		Version:     "1.0.0",
		LastUpdated: time.Now(),
		GitRoot:     c.gitRoot,
		Hostname:    hostname,
		Tools:       AllToolsCache{},
		Projects: ProjectCache{
			Configs: make(map[string]ProjectConfig),
		},
		Performance: MetricsCache{
			ToolPerformance: make(map[string]ToolMetrics),
			LinterStats:     make(map[string]LinterStats),
			SystemInfo:      getSystemMetrics(),
			LastUpdated:     time.Now(),
		},
	}
}

// save persists the cache to disk
func (c *CacheManager) save() error {
	c.cache.LastUpdated = time.Now()

	data, err := json.MarshalIndent(c.cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(c.cachePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// GetTool retrieves cached tool information
func (c *CacheManager) GetTool(category, toolName string) *ToolInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cache == nil {
		return nil
	}

	return c.getToolByPath(category, toolName)
}

// getToolByPath navigates the cache structure to find the requested tool
func (c *CacheManager) getToolByPath(category, toolName string) *ToolInfo {
	tools := &c.cache.Tools

	switch category {
	case "go":
		return c.getGoTool(tools.Go, toolName)
	case "javascript", "typescript":
		return c.getJSTool(tools.JavaScript, toolName)
	case "python":
		return c.getPythonTool(tools.Python, toolName)
	case "json":
		return c.getJSONTool(tools.JSON, toolName)
	case "markdown":
		return c.getMarkdownTool(tools.Markdown, toolName)
	case "system":
		return c.getSystemTool(tools.System, toolName)
	case "git":
		return c.getGitTool(tools.Git, toolName)
	case "runtime":
		return c.getRuntimeTool(tools.Runtime, toolName)
	}

	return nil
}

// Helper methods to get tools from each category
func (c *CacheManager) getGoTool(tools GoToolsCache, toolName string) *ToolInfo {
	switch toolName {
	case "go":
		return tools.Go
	case "gofmt":
		return tools.Gofmt
	case "golangci-lint":
		return tools.GolangciLint
	case "gotest":
		return tools.GoTest
	case "gomod":
		return tools.GoMod
	case "govet":
		return tools.GoVet
	case "staticcheck":
		return tools.StaticCheck
	}
	return nil
}

func (c *CacheManager) getJSTool(tools JavaScriptToolsCache, toolName string) *ToolInfo {
	switch toolName {
	case "node":
		return tools.Node
	case "npm":
		return tools.NPM
	case "yarn":
		return tools.Yarn
	case "pnpm":
		return tools.PNPM
	case "biome":
		return tools.Biome
	case "oxlint":
		return tools.Oxlint
	case "eslint":
		return tools.ESLint
	case "quick-lint-js":
		return tools.QuickLint
	case "prettier":
		return tools.Prettier
	case "dprint":
		return tools.Dprint
	case "tsc":
		return tools.TSC
	case "tsserver":
		return tools.TSServer
	}
	return nil
}

func (c *CacheManager) getPythonTool(tools PythonToolsCache, toolName string) *ToolInfo {
	switch toolName {
	case "python":
		return tools.Python
	case "python3":
		return tools.Python3
	case "pip":
		return tools.Pip
	case "uv":
		return tools.UV
	case "ruff":
		return tools.Ruff
	case "black":
		return tools.Black
	case "isort":
		return tools.Isort
	case "pylint":
		return tools.Pylint
	case "flake8":
		return tools.Flake8
	case "mypy":
		return tools.Mypy
	case "pytest":
		return tools.Pytest
	}
	return nil
}

func (c *CacheManager) getJSONTool(tools JSONToolsCache, toolName string) *ToolInfo {
	switch toolName {
	case "jq":
		return tools.JQ
	case "jsonlint":
		return tools.JSONLint
	case "prettier":
		return tools.Prettier
	}
	return nil
}

func (c *CacheManager) getMarkdownTool(tools MarkdownToolsCache, toolName string) *ToolInfo {
	switch toolName {
	case "markdownlint":
		return tools.Markdownlint
	case "prettier":
		return tools.Prettier
	case "pandoc":
		return tools.Pandoc
	case "vale":
		return tools.Vale
	}
	return nil
}

func (c *CacheManager) getSystemTool(tools SystemToolsCache, toolName string) *ToolInfo {
	switch toolName {
	case "grep":
		return tools.Grep
	case "sed":
		return tools.Sed
	case "awk":
		return tools.Awk
	case "find":
		return tools.Find
	case "xargs":
		return tools.Xargs
	case "timeout":
		return tools.Timeout
	case "kill":
		return tools.Kill
	}
	return nil
}

func (c *CacheManager) getGitTool(tools GitToolsCache, toolName string) *ToolInfo {
	switch toolName {
	case "git":
		return tools.Git
	case "git-lfs":
		return tools.GitLFS
	case "hub":
		return tools.Hub
	case "gh":
		return tools.GH
	case "pre-commit":
		return tools.PreCommit
	}
	return nil
}

func (c *CacheManager) getRuntimeTool(tools RuntimeToolsCache, toolName string) *ToolInfo {
	switch toolName {
	case "docker":
		return tools.Docker
	case "podman":
		return tools.Podman
	case "nvm":
		return tools.NVM
	case "asdf":
		return tools.ASDF
	case "pyenv":
		return tools.Pyenv
	case "gvm":
		return tools.GVM
	case "make":
		return tools.Make
	case "ninja":
		return tools.Ninja
	case "bazel":
		return tools.Bazel
	}
	return nil
}

// UpdateTool updates cached tool information
func (c *CacheManager) UpdateTool(category, toolName string, info *ToolInfo) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache == nil {
		c.createNewCache()
	}

	c.setToolByPath(category, toolName, info)
	return c.save()
}

// setToolByPath updates tool information in the cache structure
func (c *CacheManager) setToolByPath(category, toolName string, info *ToolInfo) {
	tools := &c.cache.Tools

	switch category {
	case "go":
		c.setGoTool(&tools.Go, toolName, info)
	case "javascript", "typescript":
		c.setJSTool(&tools.JavaScript, toolName, info)
	case "python":
		c.setPythonTool(&tools.Python, toolName, info)
	case "json":
		c.setJSONTool(&tools.JSON, toolName, info)
	case "markdown":
		c.setMarkdownTool(&tools.Markdown, toolName, info)
	case "system":
		c.setSystemTool(&tools.System, toolName, info)
	case "git":
		c.setGitTool(&tools.Git, toolName, info)
	case "runtime":
		c.setRuntimeTool(&tools.Runtime, toolName, info)
	}
}

// Helper methods to set tools in each category
func (c *CacheManager) setGoTool(tools *GoToolsCache, toolName string, info *ToolInfo) {
	switch toolName {
	case "go":
		tools.Go = info
	case "gofmt":
		tools.Gofmt = info
	case "golangci-lint":
		tools.GolangciLint = info
	case "gotest":
		tools.GoTest = info
	case "gomod":
		tools.GoMod = info
	case "govet":
		tools.GoVet = info
	case "staticcheck":
		tools.StaticCheck = info
	}
}

func (c *CacheManager) setJSTool(tools *JavaScriptToolsCache, toolName string, info *ToolInfo) {
	switch toolName {
	case "node":
		tools.Node = info
	case "npm":
		tools.NPM = info
	case "yarn":
		tools.Yarn = info
	case "pnpm":
		tools.PNPM = info
	case "biome":
		tools.Biome = info
	case "oxlint":
		tools.Oxlint = info
	case "eslint":
		tools.ESLint = info
	case "quick-lint-js":
		tools.QuickLint = info
	case "prettier":
		tools.Prettier = info
	case "dprint":
		tools.Dprint = info
	case "tsc":
		tools.TSC = info
	case "tsserver":
		tools.TSServer = info
	}
}

func (c *CacheManager) setPythonTool(tools *PythonToolsCache, toolName string, info *ToolInfo) {
	switch toolName {
	case "python":
		tools.Python = info
	case "python3":
		tools.Python3 = info
	case "pip":
		tools.Pip = info
	case "uv":
		tools.UV = info
	case "ruff":
		tools.Ruff = info
	case "black":
		tools.Black = info
	case "isort":
		tools.Isort = info
	case "pylint":
		tools.Pylint = info
	case "flake8":
		tools.Flake8 = info
	case "mypy":
		tools.Mypy = info
	case "pytest":
		tools.Pytest = info
	}
}

func (c *CacheManager) setJSONTool(tools *JSONToolsCache, toolName string, info *ToolInfo) {
	switch toolName {
	case "jq":
		tools.JQ = info
	case "jsonlint":
		tools.JSONLint = info
	case "prettier":
		tools.Prettier = info
	}
}

func (c *CacheManager) setMarkdownTool(tools *MarkdownToolsCache, toolName string, info *ToolInfo) {
	switch toolName {
	case "markdownlint":
		tools.Markdownlint = info
	case "prettier":
		tools.Prettier = info
	case "pandoc":
		tools.Pandoc = info
	case "vale":
		tools.Vale = info
	}
}

func (c *CacheManager) setSystemTool(tools *SystemToolsCache, toolName string, info *ToolInfo) {
	switch toolName {
	case "grep":
		tools.Grep = info
	case "sed":
		tools.Sed = info
	case "awk":
		tools.Awk = info
	case "find":
		tools.Find = info
	case "xargs":
		tools.Xargs = info
	case "timeout":
		tools.Timeout = info
	case "kill":
		tools.Kill = info
	}
}

func (c *CacheManager) setGitTool(tools *GitToolsCache, toolName string, info *ToolInfo) {
	switch toolName {
	case "git":
		tools.Git = info
	case "git-lfs":
		tools.GitLFS = info
	case "hub":
		tools.Hub = info
	case "gh":
		tools.GH = info
	case "pre-commit":
		tools.PreCommit = info
	}
}

func (c *CacheManager) setRuntimeTool(tools *RuntimeToolsCache, toolName string, info *ToolInfo) {
	switch toolName {
	case "docker":
		tools.Docker = info
	case "podman":
		tools.Podman = info
	case "nvm":
		tools.NVM = info
	case "asdf":
		tools.ASDF = info
	case "pyenv":
		tools.Pyenv = info
	case "gvm":
		tools.GVM = info
	case "make":
		tools.Make = info
	case "ninja":
		tools.Ninja = info
	case "bazel":
		tools.Bazel = info
	}
}

// DiscoverTool performs tool discovery for a specific tool
func (c *CacheManager) DiscoverTool(category, toolName string) (*ToolInfo, error) {
	// Check if tool is cached and fresh
	if cachedTool := c.GetTool(category, toolName); cachedTool != nil {
		if c.isToolCacheFresh(cachedTool) {
			return cachedTool, nil
		}
	}

	// Perform fresh discovery
	tool := c.discoverSingleTool(toolName)

	// Update cache
	if err := c.UpdateTool(category, toolName, tool); err != nil {
		// Log warning but return discovered tool
		fmt.Printf("Warning: failed to update tool cache for %s: %v\n", toolName, err)
	}

	return tool, nil
}

// isToolCacheFresh checks if cached tool information is still valid
func (c *CacheManager) isToolCacheFresh(tool *ToolInfo) bool {
	// Check if last check was recent (within 24 hours)
	if time.Since(tool.LastCheck) > 24*time.Hour {
		return false
	}

	// Check if binary still exists and hasn't changed
	if tool.Path != "" {
		stat, err := os.Stat(tool.Path)
		if err != nil {
			return false // Binary no longer exists
		}

		// Check if modification time has changed
		if !stat.ModTime().Equal(tool.ModTime) {
			return false // Binary has been updated
		}
	}

	return true
}

// discoverSingleTool discovers a single tool and returns its information
func (c *CacheManager) discoverSingleTool(toolName string) *ToolInfo {
	path, err := exec.LookPath(toolName)
	if err != nil {
		return &ToolInfo{
			Available: false,
			LastCheck: time.Now(),
		}
	}

	tool := &ToolInfo{
		Path:      path,
		Available: true,
		LastCheck: time.Now(),
		Source:    "global",
	}

	// Get binary metadata
	if stat, err := os.Stat(path); err == nil {
		tool.ModTime = stat.ModTime()
		tool.BinaryHash = c.getBinaryHash(path)
	}

	// Get version if possible
	tool.Version = c.getToolVersion(toolName, path)

	return tool
}

// getBinaryHash computes SHA256 hash of the binary for change detection
func (c *CacheManager) getBinaryHash(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return ""
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// getToolVersion attempts to get the version of a tool
func (c *CacheManager) getToolVersion(toolName, path string) string {
	// Common version flags to try
	versionFlags := []string{"--version", "-V", "-v", "version"}

	for _, flag := range versionFlags {
		if version := c.tryGetVersion(path, flag); version != "" {
			return version
		}
	}

	return ""
}

// tryGetVersion attempts to get version using a specific flag
func (c *CacheManager) tryGetVersion(path, flag string) string {
	cmd := exec.Command(path, flag)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Extract version from output (simple approach)
	versionStr := strings.TrimSpace(string(output))
	lines := strings.Split(versionStr, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}

	return ""
}

// findClaudeDir walks up the directory tree to find the .claude directory
// following the same pattern as the existing config loader
func findClaudeDir(currentPath string) (string, error) {
	absPath, err := filepath.Abs(currentPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// First check if .claude exists in current directory or parents
	for {
		claudeDir := filepath.Join(absPath, ".claude")
		if stat, err := os.Stat(claudeDir); err == nil && stat.IsDir() {
			return claudeDir, nil
		}

		parent := filepath.Dir(absPath)
		if parent == absPath {
			break // Reached root
		}
		absPath = parent
	}

	// If no .claude directory found, create one in current working directory
	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	claudeDir := filepath.Join(workDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .claude directory: %w", err)
	}

	return claudeDir, nil
}

// getSystemMetrics collects system information for optimization
func getSystemMetrics() SystemMetrics {
	return SystemMetrics{
		CPUCores:     runtime.NumCPU(),
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Shell:        os.Getenv("SHELL"),
	}
}
