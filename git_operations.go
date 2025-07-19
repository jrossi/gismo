package gismo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitOperations handles git repository operations for the registry system
type GitOperations struct {
	timeout time.Duration
}

// NewGitOperations creates a new git operations handler
func NewGitOperations() *GitOperations {
	return &GitOperations{
		timeout: 60 * time.Second, // Default timeout for git operations
	}
}

// SetTimeout sets the timeout for git operations
func (g *GitOperations) SetTimeout(timeout time.Duration) {
	g.timeout = timeout
}

// GitRepoInfo contains information about a cloned repository
type GitRepoInfo struct {
	URL       string    // Original URL
	LocalPath string    // Local path where repo is cloned
	CommitSHA string    // Current commit SHA
	Branch    string    // Current branch
	CloneTime time.Time // When it was cloned
}

// CloneRepository clones a git repository to a target directory
func (g *GitOperations) CloneRepository(ctx context.Context, gitURL, targetDir string) (*GitRepoInfo, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// Ensure target directory's parent exists
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Remove target directory if it exists (for clean clone)
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return nil, fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	// Execute git clone
	cmd := exec.CommandContext(ctx, "git", "clone", gitURL, targetDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git clone failed for %s: %v\nstderr: %s", gitURL, err, stderr.String())
	}

	// Get repository information
	repoInfo, err := g.GetRepositoryInfo(ctx, targetDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	repoInfo.URL = gitURL
	repoInfo.LocalPath = targetDir
	repoInfo.CloneTime = time.Now()

	return repoInfo, nil
}

// GetRepositoryInfo gets information about a local git repository
func (g *GitOperations) GetRepositoryInfo(ctx context.Context, repoPath string) (*GitRepoInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info := &GitRepoInfo{
		LocalPath: repoPath,
	}

	// Get current commit SHA
	commitSHA, err := g.runGitCommand(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get commit SHA: %w", err)
	}
	info.CommitSHA = strings.TrimSpace(commitSHA)

	// Get current branch
	branch, err := g.runGitCommand(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		// If we can't get branch, it's not critical
		info.Branch = "unknown"
	} else {
		info.Branch = strings.TrimSpace(branch)
	}

	return info, nil
}

// UpdateRepository updates a local git repository to the latest version
func (g *GitOperations) UpdateRepository(ctx context.Context, repoPath string) (*GitRepoInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// Check if directory exists and is a git repository
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s is not a git repository", repoPath)
	}

	// Fetch latest changes
	if _, err := g.runGitCommand(ctx, repoPath, "fetch", "origin"); err != nil {
		return nil, fmt.Errorf("failed to fetch from origin: %w", err)
	}

	// Get current branch
	branch, err := g.runGitCommand(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}
	branch = strings.TrimSpace(branch)

	// Pull latest changes (only if not in detached HEAD state)
	if branch != "HEAD" {
		if _, err := g.runGitCommand(ctx, repoPath, "pull", "origin", branch); err != nil {
			return nil, fmt.Errorf("failed to pull latest changes: %w", err)
		}
	}

	// Get updated repository information
	return g.GetRepositoryInfo(ctx, repoPath)
}

// CheckoutCommit checks out a specific commit in the repository
func (g *GitOperations) CheckoutCommit(ctx context.Context, repoPath, commitSHA string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if _, err := g.runGitCommand(ctx, repoPath, "checkout", commitSHA); err != nil {
		return fmt.Errorf("failed to checkout commit %s: %w", commitSHA, err)
	}

	return nil
}

// GetFileFromRepo reads a file from a git repository
func (g *GitOperations) GetFileFromRepo(repoPath, filePath string) ([]byte, error) {
	fullPath := filepath.Join(repoPath, filePath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file %s not found in repository", filePath)
	}

	// Read file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return content, nil
}

// ListFiles lists all files in a directory within the repository
func (g *GitOperations) ListFiles(repoPath, dirPath string) ([]string, error) {
	fullPath := filepath.Join(repoPath, dirPath)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory %s not found in repository", dirPath)
		}
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// VerifyGitAvailable checks if git is available on the system
func (g *GitOperations) VerifyGitAvailable(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git is not available: %w", err)
	}

	return nil
}

// GetRemoteURL gets the remote origin URL of a repository
func (g *GitOperations) GetRemoteURL(ctx context.Context, repoPath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	url, err := g.runGitCommand(ctx, repoPath, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}

	return strings.TrimSpace(url), nil
}

// runGitCommand is a helper function to run git commands in a specific directory
func (g *GitOperations) runGitCommand(ctx context.Context, workDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git command failed: %v\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// NormalizeGitURL normalizes different git URL formats to a standard format
func NormalizeGitURL(gitURL string) string {
	// Remove common prefixes
	url := strings.TrimPrefix(gitURL, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "git@")

	// Handle SSH format (convert : to /)
	url = strings.ReplaceAll(url, ":", "/")

	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Ensure https:// prefix for cloning
	if !strings.HasPrefix(gitURL, "git@") {
		return "https://" + url
	}

	return gitURL
}

// ExtractRepoName extracts a repository name from a git URL for use as a directory name
func ExtractRepoName(gitURL string) string {
	// Normalize URL first
	url := NormalizeGitURL(gitURL)

	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Split by / and take the last part
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		// Return "user-repo" format for better readability
		user := parts[len(parts)-2]
		repo := parts[len(parts)-1]
		return fmt.Sprintf("%s-%s", user, repo)
	}

	return "unknown-repo"
}
