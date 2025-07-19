package gismo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-json"
)

// ConfigLoader handles loading and merging configuration files
type ConfigLoader struct {
	projectDir string
	homeDir    string
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader() (*ConfigLoader, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	return &ConfigLoader{
		projectDir: projectDir,
		homeDir:    homeDir,
	}, nil
}

// LoadConfig loads and merges configuration from multiple sources
func (cl *ConfigLoader) LoadConfig() (*AppConfig, error) {
	config := NewAppConfig()

	// Configuration files in order of precedence (lowest to highest)
	configPaths := []string{
		filepath.Join(cl.homeDir, ".claude", "gismo.json"),          // user global
		filepath.Join(cl.projectDir, ".claude", "gismo.json"),       // project-specific
		filepath.Join(cl.projectDir, ".claude", "gismo.local.json"), // local overrides
	}

	for _, path := range configPaths {
		if err := cl.loadAndMergeConfig(config, path); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// LoadConfigWithPaths loads configuration from specific paths
func (cl *ConfigLoader) LoadConfigWithPaths(paths []string) (*AppConfig, error) {
	config := NewAppConfig()

	for _, path := range paths {
		if err := cl.loadAndMergeConfig(config, path); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// loadAndMergeConfig loads a single config file and merges it
func (cl *ConfigLoader) loadAndMergeConfig(config *AppConfig, path string) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// File doesn't exist, skip silently
		return nil
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// Parse the JSON
	var fileConfig AppConfig
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Merge into main config
	config.Merge(&fileConfig)

	return nil
}

// FindProjectRoot finds the project root by looking for .git directory
func (cl *ConfigLoader) FindProjectRoot() (string, error) {
	dir := cl.projectDir
	for {
		// Check if .git exists in current directory
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
			return dir, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root of filesystem
			break
		}
		dir = parent
	}

	// No .git found, use current directory
	return cl.projectDir, nil
}

// GetConfigPaths returns the paths where config files will be searched
func (cl *ConfigLoader) GetConfigPaths() []string {
	return []string{
		filepath.Join(cl.homeDir, ".claude", "gismo.json"),
		filepath.Join(cl.projectDir, ".claude", "gismo.json"),
		filepath.Join(cl.projectDir, ".claude", "gismo.local.json"),
	}
}

// ConfigExists checks if any configuration files exist
func (cl *ConfigLoader) ConfigExists() bool {
	for _, path := range cl.GetConfigPaths() {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// SaveConfig saves configuration to a specific file
func (cl *ConfigLoader) SaveConfig(config *AppConfig, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file atomically
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up temp file on error
		return fmt.Errorf("failed to rename config file: %w", err)
	}

	return nil
}

// SaveToGlobalConfig saves configuration to the global config file
func (cl *ConfigLoader) SaveToGlobalConfig(config *AppConfig) error {
	path := filepath.Join(cl.homeDir, ".claude", "gismo.json")
	return cl.SaveConfig(config, path)
}

// SaveToProjectConfig saves configuration to the project config file
func (cl *ConfigLoader) SaveToProjectConfig(config *AppConfig) error {
	path := filepath.Join(cl.projectDir, ".claude", "gismo.json")
	return cl.SaveConfig(config, path)
}
