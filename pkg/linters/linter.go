package linters

import (
	"context"
)

// Linter defines the interface for all file linters
type Linter interface {
	// Lint checks the file and returns any issues found
	Lint(ctx context.Context, filePath string, content []byte) (*LintResult, error)

	// CanHandle returns true if this linter can handle the given file
	CanHandle(filePath string) bool

	// Name returns the linter name for logging
	Name() string
}

// LintResult contains the results of linting a file
type LintResult struct {
	Success    bool
	Issues     []Issue
	Formatted  []byte // Formatted content if applicable
	TestOutput string // Output from running tests
}

// Issue represents a single linting issue
type Issue struct {
	File     string
	Line     int
	Column   int
	Severity string // "error", "warning", "info"
	Message  string
	Rule     string // Rule that was violated
}
