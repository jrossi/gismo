package project

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	db "github.com/jrossi/gismo/pkg/database/sqlc"
)

// Manager handles project-related operations
type Manager struct {
	queries *db.Queries
}

// NewManager creates a new project manager
func NewManager(queries *db.Queries) *Manager {
	return &Manager{
		queries: queries,
	}
}

// GetCurrentProject returns the current Claude Code project based on working directory
func (m *Manager) GetCurrentProject(ctx context.Context) (*db.Project, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Try to get project by path first
	project, err := m.queries.GetProjectByPath(ctx, cwd)
	if err == nil {
		return &project, nil
	}

	// If not found, try to create it
	projectName := PathToProjectName(cwd)
	return m.GetOrCreateProject(ctx, projectName, cwd)
}

// GetOrCreateProject gets an existing project or creates a new one
func (m *Manager) GetOrCreateProject(ctx context.Context, projectName, projectPath string) (*db.Project, error) {
	// Try to get existing project
	project, err := m.queries.GetProjectByName(ctx, projectName)
	if err == nil {
		return &project, nil
	}

	// Create new project
	newProject, err := m.queries.InsertProject(ctx, db.InsertProjectParams{
		ProjectName: projectName,
		ProjectPath: projectPath,
		Description: sql.NullString{
			String: fmt.Sprintf("Claude Code project for %s", projectPath),
			Valid:  true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return &newProject, nil
}

// GetProjectFromEnv gets the project based on CLAUDE_PROJECT_DIR environment variable
func (m *Manager) GetProjectFromEnv(ctx context.Context) (*db.Project, error) {
	projectDir := os.Getenv("CLAUDE_PROJECT_DIR")
	if projectDir == "" {
		return m.GetCurrentProject(ctx)
	}

	// Normalize the path
	projectDir = filepath.Clean(projectDir)

	// Try to get project by path
	project, err := m.queries.GetProjectByPath(ctx, projectDir)
	if err == nil {
		return &project, nil
	}

	// If not found, create it
	projectName := PathToProjectName(projectDir)
	return m.GetOrCreateProject(ctx, projectName, projectDir)
}

// PathToProjectName converts a file path to Claude's project name format
// e.g., "/Users/jrossi/src/gismo" -> "-Users-jrossi-src-gismo"
func PathToProjectName(path string) string {
	// Clean the path
	path = filepath.Clean(path)

	// Replace path separators with dashes
	projectName := strings.ReplaceAll(path, string(filepath.Separator), "-")

	// Ensure it starts with a dash
	if !strings.HasPrefix(projectName, "-") {
		projectName = "-" + projectName
	}

	return projectName
}

// ProjectNameToPath converts Claude's project name format back to a file path
// e.g., "-Users-jrossi-src-gismo" -> "/Users/jrossi/src/gismo"
func ProjectNameToPath(projectName string) string {
	// Remove leading dash if present
	projectName = strings.TrimPrefix(projectName, "-")

	// Replace dashes with path separators
	path := strings.ReplaceAll(projectName, "-", string(filepath.Separator))

	// Ensure it starts with separator (absolute path)
	if !strings.HasPrefix(path, string(filepath.Separator)) {
		path = string(filepath.Separator) + path
	}

	return path
}

// GetRelativePath gets the relative path of a file within a project
func GetRelativePath(projectPath, absolutePath string) (string, error) {
	// Clean both paths
	projectPath = filepath.Clean(projectPath)
	absolutePath = filepath.Clean(absolutePath)

	// Get relative path
	relPath, err := filepath.Rel(projectPath, absolutePath)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}

	// Ensure the file is within the project
	if strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("file %s is outside project %s", absolutePath, projectPath)
	}

	return relPath, nil
}

// GetAbsolutePath gets the absolute path of a file within a project
func GetAbsolutePath(projectPath, relativePath string) string {
	return filepath.Join(projectPath, relativePath)
}
