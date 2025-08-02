package linters

import (
	"context"
	"runtime"
	"sync"
)

// ParallelExecutor runs multiple linters concurrently for improved performance
type ParallelExecutor struct {
	maxWorkers int
}

// NewParallelExecutor creates a new parallel executor with the specified number of workers
// If maxWorkers is 0 or negative, it defaults to runtime.NumCPU()
func NewParallelExecutor(maxWorkers int) *ParallelExecutor {
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}
	return &ParallelExecutor{
		maxWorkers: maxWorkers,
	}
}

// LintTask represents a single linting task
type LintTask struct {
	Linter   Linter
	FilePath string
	Content  []byte
}

// LintTaskResult represents the result of a linting task
type LintTaskResult struct {
	LinterName string
	Result     *LintResult
	Error      error
}

// ExecuteTasks runs multiple linting tasks in parallel
func (pe *ParallelExecutor) ExecuteTasks(ctx context.Context, tasks []LintTask) []LintTaskResult {
	if len(tasks) == 0 {
		return nil
	}

	// For single task, run directly without goroutines
	if len(tasks) == 1 {
		task := tasks[0]
		result, err := task.Linter.Lint(ctx, task.FilePath, task.Content)
		return []LintTaskResult{{
			LinterName: task.Linter.Name(),
			Result:     result,
			Error:      err,
		}}
	}

	// Create channels for task distribution and result collection
	taskChan := make(chan LintTask, len(tasks))
	resultChan := make(chan LintTaskResult, len(tasks))

	// Create wait group for workers
	var wg sync.WaitGroup

	// Determine number of workers (don't create more than tasks)
	numWorkers := pe.maxWorkers
	if len(tasks) < numWorkers {
		numWorkers = len(tasks)
	}

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				// Check context cancellation
				select {
				case <-ctx.Done():
					resultChan <- LintTaskResult{
						LinterName: task.Linter.Name(),
						Error:      ctx.Err(),
					}
					continue
				default:
				}

				// Execute linting task
				result, err := task.Linter.Lint(ctx, task.FilePath, task.Content)
				resultChan <- LintTaskResult{
					LinterName: task.Linter.Name(),
					Result:     result,
					Error:      err,
				}
			}
		}()
	}

	// Send tasks to workers
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	results := make([]LintTaskResult, 0, len(tasks))
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// ExecuteLinters runs multiple linters on a single file in parallel
func (pe *ParallelExecutor) ExecuteLinters(ctx context.Context, linters []Linter, filePath string, content []byte) []LintTaskResult {
	tasks := make([]LintTask, 0, len(linters))
	for _, linter := range linters {
		if linter.CanHandle(filePath) {
			tasks = append(tasks, LintTask{
				Linter:   linter,
				FilePath: filePath,
				Content:  content,
			})
		}
	}
	return pe.ExecuteTasks(ctx, tasks)
}

// AggregateResults combines multiple lint results into a single result
func AggregateResults(results []LintTaskResult) (*LintResult, []error) {
	aggregated := &LintResult{
		Success: true,
		Issues:  []Issue{},
	}

	var errors []error

	for _, taskResult := range results {
		if taskResult.Error != nil {
			errors = append(errors, taskResult.Error)
			continue
		}

		if taskResult.Result != nil {
			// Merge issues
			aggregated.Issues = append(aggregated.Issues, taskResult.Result.Issues...)

			// Update success status
			if !taskResult.Result.Success {
				aggregated.Success = false
			}

			// Keep first non-nil formatted content
			if aggregated.Formatted == nil && taskResult.Result.Formatted != nil {
				aggregated.Formatted = taskResult.Result.Formatted
			}

			// Append test output
			if taskResult.Result.TestOutput != "" {
				if aggregated.TestOutput != "" {
					aggregated.TestOutput += "\n"
				}
				aggregated.TestOutput += taskResult.Result.TestOutput
			}
		}
	}

	return aggregated, errors
}
