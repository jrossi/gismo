package linters

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

// MockBatchingLinter simulates a linter that can batch multiple files
type MockBatchingLinter struct {
	MockLinter
	batchCallCount int32
	batchFunc      func(ctx context.Context, files map[string][]byte) (map[string]*LintResult, error)
}

func (m *MockBatchingLinter) LintBatch(ctx context.Context, files map[string][]byte) (map[string]*LintResult, error) {
	atomic.AddInt32(&m.batchCallCount, 1)

	if m.batchFunc != nil {
		return m.batchFunc(ctx, files)
	}

	results := make(map[string]*LintResult)
	for filePath := range files {
		results[filePath] = &LintResult{
			Success: true,
			Issues: []Issue{
				{
					File:     filePath,
					Message:  fmt.Sprintf("batch issue for %s", filePath),
					Severity: "warning",
				},
			},
		}
	}
	return results, nil
}

func TestBatchExecutor_ExecuteLintersBatched(t *testing.T) {
	// Create batch executor
	executor := NewBatchExecutor(4)

	// Create a batching linter
	batchLinter := &MockBatchingLinter{
		MockLinter: MockLinter{
			name: "batch-go",
			canHandle: func(path string) bool {
				return path[len(path)-3:] == ".go"
			},
		},
	}

	// Create a regular linter
	regularLinter := &MockLinter{
		name: "regular-md",
		canHandle: func(path string) bool {
			return path[len(path)-3:] == ".md"
		},
		lintResult: &LintResult{
			Success: true,
			Issues: []Issue{
				{Message: "regular issue"},
			},
		},
	}

	// Test files
	files := map[string][]byte{
		"file1.go": []byte("package main"),
		"file2.go": []byte("package main"),
		"file3.go": []byte("package main"),
		"doc.md":   []byte("# Doc"),
	}

	// Execute batched linting
	results := executor.ExecuteLintersBatched(
		context.Background(),
		[]Linter{batchLinter, regularLinter},
		files,
	)

	// Verify results
	if len(results) != 4 {
		t.Errorf("expected results for 4 files, got %d", len(results))
	}

	// Check that batch linter was called once for all Go files
	if atomic.LoadInt32(&batchLinter.batchCallCount) != 1 {
		t.Errorf("expected batch linter to be called once, got %d", batchLinter.batchCallCount)
	}

	// Verify Go files have batch results
	for _, goFile := range []string{"file1.go", "file2.go", "file3.go"} {
		fileResults, exists := results[goFile]
		if !exists {
			t.Errorf("missing results for %s", goFile)
			continue
		}
		if len(fileResults) != 1 {
			t.Errorf("expected 1 result for %s, got %d", goFile, len(fileResults))
		}
		if fileResults[0].LinterName != "batch-go" {
			t.Errorf("expected batch-go linter for %s, got %s", goFile, fileResults[0].LinterName)
		}
	}

	// Verify Markdown file has regular linter result
	mdResults, exists := results["doc.md"]
	if !exists {
		t.Error("missing results for doc.md")
	} else {
		if len(mdResults) != 1 {
			t.Errorf("expected 1 result for doc.md, got %d", len(mdResults))
		}
		if mdResults[0].LinterName != "regular-md" {
			t.Errorf("expected regular-md linter for doc.md, got %s", mdResults[0].LinterName)
		}
	}
}

func TestBatchExecutor_MixedLinters(t *testing.T) {
	executor := NewBatchExecutor(2)

	// Create multiple linters
	batchLinter1 := &MockBatchingLinter{
		MockLinter: MockLinter{
			name:      "batch1",
			canHandle: func(path string) bool { return true },
		},
	}

	batchLinter2 := &MockBatchingLinter{
		MockLinter: MockLinter{
			name:      "batch2",
			canHandle: func(path string) bool { return true },
		},
	}

	regularLinter := &MockLinter{
		name:      "regular",
		canHandle: func(path string) bool { return true },
		lintResult: &LintResult{
			Success: true,
			Issues:  []Issue{{Message: "regular issue"}},
		},
	}

	files := map[string][]byte{
		"test.go": []byte("content"),
	}

	results := executor.ExecuteLintersBatched(
		context.Background(),
		[]Linter{batchLinter1, batchLinter2, regularLinter},
		files,
	)

	// Should have results from all three linters
	if len(results["test.go"]) != 3 {
		t.Errorf("expected 3 results for test.go, got %d", len(results["test.go"]))
	}

	// Both batch linters should have been called
	if atomic.LoadInt32(&batchLinter1.batchCallCount) != 1 {
		t.Error("batch linter 1 not called")
	}
	if atomic.LoadInt32(&batchLinter2.batchCallCount) != 1 {
		t.Error("batch linter 2 not called")
	}
}

func TestBatchExecutor_EmptyFiles(t *testing.T) {
	executor := NewBatchExecutor(4)

	results := executor.ExecuteLintersBatched(
		context.Background(),
		[]Linter{&MockLinter{name: "test"}},
		map[string][]byte{},
	)

	if results != nil {
		t.Error("expected nil results for empty files")
	}
}

func TestBatchExecutor_ErrorHandling(t *testing.T) {
	executor := NewBatchExecutor(2)

	// Create a batching linter that returns an error
	errorLinter := &MockBatchingLinter{
		MockLinter: MockLinter{
			name:      "error-linter",
			canHandle: func(path string) bool { return true },
		},
		batchFunc: func(ctx context.Context, files map[string][]byte) (map[string]*LintResult, error) {
			return nil, fmt.Errorf("batch linting failed")
		},
	}

	files := map[string][]byte{
		"test.go": []byte("content"),
	}

	results := executor.ExecuteLintersBatched(
		context.Background(),
		[]Linter{errorLinter},
		files,
	)

	// Should have error result
	if len(results["test.go"]) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results["test.go"]))
	}

	if results["test.go"][0].Error == nil {
		t.Error("expected error in result")
	}
}
