package linters

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// makeWorkChannel creates a channel that completes work after a brief delay
func makeWorkChannel() chan struct{} {
	ch := make(chan struct{})
	go func() {
		// Use a timer to simulate work without sleep
		timer := time.NewTimer(10 * time.Millisecond)
		<-timer.C
		close(ch)
	}()
	return ch
}

// makeImmediateWorkChannel creates a channel that completes work immediately
func makeImmediateWorkChannel() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// MockLinter simulates a linter with configurable behavior
type MockLinter struct {
	name       string
	canHandle  func(string) bool
	lintResult *LintResult
	lintErr    error
	execCount  int32
	workChan   chan struct{} // Channel to simulate work completion
}

func (m *MockLinter) Name() string { return m.name }

func (m *MockLinter) CanHandle(filePath string) bool {
	if m.canHandle != nil {
		return m.canHandle(filePath)
	}
	return true
}

func (m *MockLinter) Lint(ctx context.Context, filePath string, content []byte) (*LintResult, error) {
	atomic.AddInt32(&m.execCount, 1)

	// Simulate work using channel
	if m.workChan != nil {
		select {
		case <-m.workChan:
			// Work completed
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return m.lintResult, m.lintErr
}

func TestParallelExecutor_ExecuteTasks(t *testing.T) {
	tests := []struct {
		name       string
		maxWorkers int
		tasks      []LintTask
		wantErrors int
		wantIssues int
	}{
		{
			name:       "single task",
			maxWorkers: 4,
			tasks: []LintTask{
				{
					Linter: &MockLinter{
						name: "mock1",
						lintResult: &LintResult{
							Success: true,
							Issues:  []Issue{{Message: "test issue"}},
						},
					},
					FilePath: "test.go",
					Content:  []byte("test"),
				},
			},
			wantErrors: 0,
			wantIssues: 1,
		},
		{
			name:       "multiple tasks parallel",
			maxWorkers: 4,
			tasks: []LintTask{
				{
					Linter: &MockLinter{
						name: "mock1",
						lintResult: &LintResult{
							Success: true,
							Issues:  []Issue{{Message: "issue1"}},
						},
						workChan: makeWorkChannel(),
					},
					FilePath: "test1.go",
					Content:  []byte("test1"),
				},
				{
					Linter: &MockLinter{
						name: "mock2",
						lintResult: &LintResult{
							Success: true,
							Issues:  []Issue{{Message: "issue2"}},
						},
						workChan: makeWorkChannel(),
					},
					FilePath: "test2.go",
					Content:  []byte("test2"),
				},
				{
					Linter: &MockLinter{
						name: "mock3",
						lintResult: &LintResult{
							Success: true,
							Issues:  []Issue{{Message: "issue3"}},
						},
						workChan: makeWorkChannel(),
					},
					FilePath: "test3.go",
					Content:  []byte("test3"),
				},
			},
			wantErrors: 0,
			wantIssues: 3,
		},
		{
			name:       "task with error",
			maxWorkers: 2,
			tasks: []LintTask{
				{
					Linter: &MockLinter{
						name:       "mock1",
						lintResult: nil,
						lintErr:    fmt.Errorf("lint error"),
					},
					FilePath: "test.go",
					Content:  []byte("test"),
				},
				{
					Linter: &MockLinter{
						name: "mock2",
						lintResult: &LintResult{
							Success: true,
							Issues:  []Issue{{Message: "issue"}},
						},
					},
					FilePath: "test2.go",
					Content:  []byte("test2"),
				},
			},
			wantErrors: 1,
			wantIssues: 1,
		},
		{
			name:       "limited workers",
			maxWorkers: 1, // Force sequential execution
			tasks: []LintTask{
				{
					Linter: &MockLinter{
						name: "mock1",
						lintResult: &LintResult{
							Success: true,
						},
						workChan: makeWorkChannel(),
					},
					FilePath: "test1.go",
					Content:  []byte("test1"),
				},
				{
					Linter: &MockLinter{
						name: "mock2",
						lintResult: &LintResult{
							Success: true,
						},
						workChan: makeWorkChannel(),
					},
					FilePath: "test2.go",
					Content:  []byte("test2"),
				},
			},
			wantErrors: 0,
			wantIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewParallelExecutor(tt.maxWorkers)
			results := executor.ExecuteTasks(context.Background(), tt.tasks)

			// Count errors and issues
			var errorCount int
			var issueCount int
			for _, result := range results {
				if result.Error != nil {
					errorCount++
				}
				if result.Result != nil {
					issueCount += len(result.Result.Issues)
				}
			}

			if errorCount != tt.wantErrors {
				t.Errorf("got %d errors, want %d", errorCount, tt.wantErrors)
			}

			if issueCount != tt.wantIssues {
				t.Errorf("got %d issues, want %d", issueCount, tt.wantIssues)
			}

			// Verify all tasks were executed
			if len(results) != len(tt.tasks) {
				t.Errorf("got %d results, want %d", len(results), len(tt.tasks))
			}
		})
	}
}

func TestParallelExecutor_ExecuteLinters(t *testing.T) {
	goLinter := &MockLinter{
		name: "go",
		canHandle: func(path string) bool {
			return path == "test.go"
		},
		lintResult: &LintResult{
			Success: true,
			Issues:  []Issue{{Message: "go issue"}},
		},
	}

	mdLinter := &MockLinter{
		name: "markdown",
		canHandle: func(path string) bool {
			return path == "test.md"
		},
		lintResult: &LintResult{
			Success: true,
			Issues:  []Issue{{Message: "md issue"}},
		},
	}

	executor := NewParallelExecutor(2)

	// Test with Go file
	results := executor.ExecuteLinters(context.Background(), []Linter{goLinter, mdLinter}, "test.go", []byte("content"))
	if len(results) != 1 {
		t.Errorf("expected 1 result for .go file, got %d", len(results))
	}
	if results[0].LinterName != "go" {
		t.Errorf("expected go linter, got %s", results[0].LinterName)
	}

	// Test with Markdown file
	results = executor.ExecuteLinters(context.Background(), []Linter{goLinter, mdLinter}, "test.md", []byte("content"))
	if len(results) != 1 {
		t.Errorf("expected 1 result for .md file, got %d", len(results))
	}
	if results[0].LinterName != "markdown" {
		t.Errorf("expected markdown linter, got %s", results[0].LinterName)
	}
}

func TestParallelExecutor_ContextCancellation(t *testing.T) {
	// Create a work channel that won't complete immediately
	workChan := make(chan struct{})
	slowLinter := &MockLinter{
		name:     "slow",
		workChan: workChan, // Will block until channel is closed or context is canceled
		lintResult: &LintResult{
			Success: true,
		},
	}

	executor := NewParallelExecutor(2)
	ctx, cancel := context.WithCancel(context.Background())

	// Start execution in goroutine
	done := make(chan []LintTaskResult)
	started := make(chan struct{})
	go func() {
		close(started) // Signal that goroutine has started
		results := executor.ExecuteTasks(ctx, []LintTask{
			{
				Linter:   slowLinter,
				FilePath: "test.go",
				Content:  []byte("test"),
			},
		})
		done <- results
	}()

	// Wait for goroutine to start, then cancel context
	<-started
	cancel()

	// Wait for results
	results := <-done

	// Should have context cancellation error
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", results[0].Error)
	}

	// Clean up
	close(workChan)
}

func TestAggregateResults(t *testing.T) {
	results := []LintTaskResult{
		{
			LinterName: "linter1",
			Result: &LintResult{
				Success: true,
				Issues: []Issue{
					{Severity: "warning", Message: "warning1"},
					{Severity: "error", Message: "error1"},
				},
				TestOutput: "test output 1",
			},
		},
		{
			LinterName: "linter2",
			Result: &LintResult{
				Success: false,
				Issues: []Issue{
					{Severity: "error", Message: "error2"},
				},
				Formatted:  []byte("formatted"),
				TestOutput: "test output 2",
			},
		},
		{
			LinterName: "linter3",
			Error:      fmt.Errorf("linter3 failed"),
		},
	}

	aggregated, errors := AggregateResults(results)

	// Check aggregated issues
	if len(aggregated.Issues) != 3 {
		t.Errorf("expected 3 issues, got %d", len(aggregated.Issues))
	}

	// Check success status (should be false due to linter2)
	if aggregated.Success {
		t.Error("expected success to be false")
	}

	// Check formatted content (should be from linter2)
	if string(aggregated.Formatted) != "formatted" {
		t.Errorf("expected formatted content from linter2")
	}

	// Check test output aggregation
	expectedOutput := "test output 1\ntest output 2"
	if aggregated.TestOutput != expectedOutput {
		t.Errorf("expected test output %q, got %q", expectedOutput, aggregated.TestOutput)
	}

	// Check errors
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestParallelExecutor_DefaultWorkers(t *testing.T) {
	// Test with 0 workers (should default to NumCPU)
	executor := NewParallelExecutor(0)
	if executor.maxWorkers <= 0 {
		t.Errorf("expected positive maxWorkers, got %d", executor.maxWorkers)
	}

	// Test with negative workers (should default to NumCPU)
	executor = NewParallelExecutor(-1)
	if executor.maxWorkers <= 0 {
		t.Errorf("expected positive maxWorkers, got %d", executor.maxWorkers)
	}
}

func BenchmarkParallelExecutor_ExecuteTasks(b *testing.B) {
	// Create mock linters with some work
	linters := make([]*MockLinter, 4)
	for i := range linters {
		linters[i] = &MockLinter{
			name: fmt.Sprintf("linter%d", i),
			lintResult: &LintResult{
				Success: true,
				Issues: []Issue{
					{Message: fmt.Sprintf("issue from linter %d", i)},
				},
			},
			workChan: makeImmediateWorkChannel(), // Simulate immediate work completion
		}
	}

	tasks := make([]LintTask, len(linters))
	for i, linter := range linters {
		tasks[i] = LintTask{
			Linter:   linter,
			FilePath: fmt.Sprintf("test%d.go", i),
			Content:  []byte(fmt.Sprintf("content %d", i)),
		}
	}

	b.Run("Sequential", func(b *testing.B) {
		executor := NewParallelExecutor(1)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			executor.ExecuteTasks(context.Background(), tasks)
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		executor := NewParallelExecutor(4)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			executor.ExecuteTasks(context.Background(), tasks)
		}
	})
}
