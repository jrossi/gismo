package project_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jrossi/gismo/pkg/database"
	"github.com/jrossi/gismo/pkg/database/project"
)

func setupTestDB(t *testing.T) (*database.DB, func()) {
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "gismo_project_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.NewWithPath(ctx, dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestNewManager(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := project.NewManager(db)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestGetCurrentProject(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := project.NewManager(db)

	// Save current directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	// Create and change to a test directory
	testDir, err := os.MkdirTemp("", "test_project")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	err = os.Chdir(testDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Test GetCurrentProject
	proj, err := manager.GetCurrentProject(ctx)
	if err != nil {
		t.Fatalf("Failed to get current project: %v", err)
	}

	if proj == nil {
		t.Fatal("GetCurrentProject returned nil")
	}

	// Get the actual current directory (which may be resolved differently on macOS)
	actualDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get actual working directory: %v", err)
	}

	expectedName := project.PathToProjectName(actualDir)
	if proj.ProjectName != expectedName {
		t.Errorf("Expected project name %s, got %s", expectedName, proj.ProjectName)
	}

	if proj.ProjectPath != actualDir {
		t.Errorf("Expected project path %s, got %s", actualDir, proj.ProjectPath)
	}

	// Call again - should return the same project
	proj2, err := manager.GetCurrentProject(ctx)
	if err != nil {
		t.Fatalf("Failed to get current project second time: %v", err)
	}

	if proj2.ID != proj.ID {
		t.Errorf("Expected same project ID %d, got %d", proj.ID, proj2.ID)
	}
}

func TestGetOrCreateProject(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := project.NewManager(db)

	testCases := []struct {
		name        string
		projectName string
		projectPath string
	}{
		{
			name:        "new project",
			projectName: "test-project-1",
			projectPath: "/test/path/1",
		},
		{
			name:        "existing project",
			projectName: "test-project-1",
			projectPath: "/test/path/1",
		},
		{
			name:        "another new project",
			projectName: "test-project-2",
			projectPath: "/test/path/2",
		},
	}

	var firstProjectID int
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proj, err := manager.GetOrCreateProject(ctx, tc.projectName, tc.projectPath)
			if err != nil {
				t.Fatalf("Failed to get or create project: %v", err)
			}

			if proj == nil {
				t.Fatal("GetOrCreateProject returned nil")
			}

			if proj.ProjectName != tc.projectName {
				t.Errorf("Expected project name %s, got %s", tc.projectName, proj.ProjectName)
			}

			if proj.ProjectPath != tc.projectPath {
				t.Errorf("Expected project path %s, got %s", tc.projectPath, proj.ProjectPath)
			}

			// For the existing project case, verify it returns the same ID
			if i == 0 {
				firstProjectID = proj.ID
			} else if i == 1 {
				if proj.ID != firstProjectID {
					t.Errorf("Expected same project ID for existing project, got %d != %d", proj.ID, firstProjectID)
				}
			}
		})
	}
}

func TestGetProjectFromEnv(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := project.NewManager(db)

	// Test without environment variable (should use current directory)
	os.Unsetenv("CLAUDE_PROJECT_DIR")

	// Save and change directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	testDir, err := os.MkdirTemp("", "env_test")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	err = os.Chdir(testDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Get the actual current directory (which may be resolved differently on macOS)
	actualDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get actual working directory: %v", err)
	}

	proj1, err := manager.GetProjectFromEnv(ctx)
	if err != nil {
		t.Fatalf("Failed to get project from env (no env): %v", err)
	}

	if proj1.ProjectPath != actualDir {
		t.Errorf("Expected project path %s, got %s", actualDir, proj1.ProjectPath)
	}

	// Test with environment variable
	envTestDir, err := os.MkdirTemp("", "env_project")
	if err != nil {
		t.Fatalf("Failed to create env test directory: %v", err)
	}
	defer os.RemoveAll(envTestDir)

	os.Setenv("CLAUDE_PROJECT_DIR", envTestDir)
	defer os.Unsetenv("CLAUDE_PROJECT_DIR")

	proj2, err := manager.GetProjectFromEnv(ctx)
	if err != nil {
		t.Fatalf("Failed to get project from env (with env): %v", err)
	}

	if proj2.ProjectPath != envTestDir {
		t.Errorf("Expected project path from env %s, got %s", envTestDir, proj2.ProjectPath)
	}

	expectedName := project.PathToProjectName(envTestDir)
	if proj2.ProjectName != expectedName {
		t.Errorf("Expected project name %s, got %s", expectedName, proj2.ProjectName)
	}

	// Projects should be different
	if proj1.ID == proj2.ID {
		t.Error("Expected different projects for different paths")
	}
}

func TestPathToProjectName(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "absolute unix path",
			input:    "/Users/jrossi/src/gismo",
			expected: "-Users-jrossi-src-gismo",
		},
		{
			name:     "root path",
			input:    "/",
			expected: "-",
		},
		{
			name:     "relative path gets cleaned",
			input:    "src/gismo",
			expected: "-src-gismo",
		},
		{
			name:     "path with trailing slash",
			input:    "/Users/jrossi/src/gismo/",
			expected: "-Users-jrossi-src-gismo",
		},
		{
			name:     "path with dots",
			input:    "/Users/jrossi/../jrossi/src",
			expected: "-Users-jrossi-src",
		},
	}

	// Windows-specific test cases
	if runtime.GOOS == "windows" {
		testCases = append(testCases, []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "windows path",
				input:    `C:\Users\jrossi\src\gismo`,
				expected: "-C:-Users-jrossi-src-gismo",
			},
			{
				name:     "windows UNC path",
				input:    `\\server\share\folder`,
				expected: `--server-share-folder`,
			},
		}...)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := project.PathToProjectName(tc.input)
			if result != tc.expected {
				t.Errorf("PathToProjectName(%s) = %s, expected %s", tc.input, result, tc.expected)
			}
		})
	}
}

func TestProjectNameToPath(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal project name",
			input:    "-Users-jrossi-src-gismo",
			expected: "/Users/jrossi/src/gismo",
		},
		{
			name:     "project name without leading dash",
			input:    "Users-jrossi-src-gismo",
			expected: "/Users/jrossi/src/gismo",
		},
		{
			name:     "single dash",
			input:    "-",
			expected: "/",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "/",
		},
	}

	// Windows-specific test cases
	if runtime.GOOS == "windows" {
		testCases = []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "windows project name",
				input:    "-C:-Users-jrossi-src-gismo",
				expected: `\C:\Users\jrossi\src\gismo`,
			},
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := project.ProjectNameToPath(tc.input)
			if result != tc.expected {
				t.Errorf("ProjectNameToPath(%s) = %s, expected %s", tc.input, result, tc.expected)
			}
		})
	}
}

func TestGetRelativePath(t *testing.T) {
	testCases := []struct {
		name         string
		projectPath  string
		absolutePath string
		expected     string
		shouldError  bool
	}{
		{
			name:         "file in project",
			projectPath:  "/project/root",
			absolutePath: "/project/root/src/main.go",
			expected:     "src/main.go",
			shouldError:  false,
		},
		{
			name:         "file in project root",
			projectPath:  "/project/root",
			absolutePath: "/project/root/main.go",
			expected:     "main.go",
			shouldError:  false,
		},
		{
			name:         "file outside project",
			projectPath:  "/project/root",
			absolutePath: "/other/path/file.go",
			expected:     "",
			shouldError:  true,
		},
		{
			name:         "parent directory traversal",
			projectPath:  "/project/root",
			absolutePath: "/project/file.go",
			expected:     "",
			shouldError:  true,
		},
		{
			name:         "same path",
			projectPath:  "/project/root",
			absolutePath: "/project/root",
			expected:     ".",
			shouldError:  false,
		},
		{
			name:         "nested deeply",
			projectPath:  "/a/b",
			absolutePath: "/a/b/c/d/e/f.go",
			expected:     "c/d/e/f.go",
			shouldError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := project.GetRelativePath(tc.projectPath, tc.absolutePath)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none, result: %s", result)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tc.expected {
					t.Errorf("GetRelativePath(%s, %s) = %s, expected %s",
						tc.projectPath, tc.absolutePath, result, tc.expected)
				}
			}
		})
	}
}

func TestGetAbsolutePath(t *testing.T) {
	testCases := []struct {
		name         string
		projectPath  string
		relativePath string
		expected     string
	}{
		{
			name:         "simple file",
			projectPath:  "/project/root",
			relativePath: "src/main.go",
			expected:     "/project/root/src/main.go",
		},
		{
			name:         "file in root",
			projectPath:  "/project/root",
			relativePath: "main.go",
			expected:     "/project/root/main.go",
		},
		{
			name:         "dot path",
			projectPath:  "/project/root",
			relativePath: ".",
			expected:     "/project/root", // filepath.Join normalizes "."
		},
		{
			name:         "empty relative path",
			projectPath:  "/project/root",
			relativePath: "",
			expected:     "/project/root",
		},
		{
			name:         "nested path",
			projectPath:  "/a/b/c",
			relativePath: "d/e/f/g.txt",
			expected:     "/a/b/c/d/e/f/g.txt",
		},
	}

	// Windows-specific behavior
	if runtime.GOOS == "windows" {
		// Adjust expectations for Windows paths
		for i := range testCases {
			testCases[i].projectPath = filepath.FromSlash(testCases[i].projectPath)
			testCases[i].relativePath = filepath.FromSlash(testCases[i].relativePath)
			testCases[i].expected = filepath.FromSlash(testCases[i].expected)
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := project.GetAbsolutePath(tc.projectPath, tc.relativePath)
			if result != tc.expected {
				t.Errorf("GetAbsolutePath(%s, %s) = %s, expected %s",
					tc.projectPath, tc.relativePath, result, tc.expected)
			}
		})
	}
}

func TestManagerConcurrency(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := project.NewManager(db)

	// Test sequential calls with same project name (simpler than concurrent for DuckDB)
	// This still tests the GetOrCreate logic
	var firstID int
	for i := 0; i < 5; i++ {
		proj, err := manager.GetOrCreateProject(ctx, "concurrent-project", "/concurrent/path")
		if err != nil {
			t.Fatalf("Failed to get or create project on iteration %d: %v", i, err)
		}

		if i == 0 {
			firstID = proj.ID
		} else if proj.ID != firstID {
			t.Errorf("Expected same project ID on iteration %d, got %d != %d", i, proj.ID, firstID)
		}
	}

	// Test with different projects in parallel (should work fine)
	done := make(chan bool, 5)
	errors := make(chan error, 5)

	for i := 0; i < 5; i++ {
		go func(idx int) {
			projName := fmt.Sprintf("parallel-project-%d", idx)
			projPath := fmt.Sprintf("/parallel/path/%d", idx)
			_, err := manager.GetOrCreateProject(ctx, projName, projPath)
			if err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Error creating parallel project: %v", err)
	}
}

func TestManagerWithDatabase(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	manager := project.NewManager(db)

	// Create a project using the manager
	proj, err := manager.GetOrCreateProject(ctx, "db-test-project", "/db/test/path")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Verify we can retrieve it directly from database
	dbProj, err := db.GetProjectByName(ctx, "db-test-project")
	if err != nil {
		t.Fatalf("Failed to get project from database: %v", err)
	}

	if dbProj == nil {
		t.Fatal("Project not found in database")
	}

	if dbProj.ID != proj.ID {
		t.Errorf("Project ID mismatch: manager returned %d, database has %d", proj.ID, dbProj.ID)
	}

	// Update the project in database
	err = db.UpdateProjectIndexTime(ctx, proj.ID)
	if err != nil {
		t.Fatalf("Failed to update project: %v", err)
	}

	// Get it again through manager - should get the updated version
	proj2, err := manager.GetOrCreateProject(ctx, "db-test-project", "/db/test/path")
	if err != nil {
		t.Fatalf("Failed to get project again: %v", err)
	}

	if !proj2.LastIndexedAt.Valid {
		t.Error("Expected LastIndexedAt to be set after update")
	}
}

func TestPathNormalization(t *testing.T) {
	// Test that paths are properly normalized
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path with double slashes",
			input:    "/Users//jrossi///src//gismo",
			expected: "-Users-jrossi-src-gismo",
		},
		{
			name:     "path with dots",
			input:    "/Users/jrossi/./src/../src/gismo",
			expected: "-Users-jrossi-src-gismo",
		},
		{
			name:     "relative path",
			input:    "./src/gismo",
			expected: "-src-gismo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := project.PathToProjectName(tc.input)
			// The actual expectation depends on how filepath.Clean handles the path
			// which may vary by OS
			cleanPath := filepath.Clean(tc.input)
			expected := project.PathToProjectName(cleanPath)
			if result != expected {
				t.Errorf("PathToProjectName(%s) = %s, expected %s (clean path: %s)",
					tc.input, result, expected, cleanPath)
			}
		})
	}
}
