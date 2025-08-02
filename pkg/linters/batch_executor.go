package linters

import (
	"context"
	"sync"
)

// BatchExecutor optimizes linting by batching files for linters that support it
type BatchExecutor struct {
	executor *ParallelExecutor
}

// NewBatchExecutor creates a new batch executor
func NewBatchExecutor(maxWorkers int) *BatchExecutor {
	return &BatchExecutor{
		executor: NewParallelExecutor(maxWorkers),
	}
}

// BatchingLinter is a linter that can process multiple files at once
type BatchingLinter interface {
	Linter
	// LintBatch processes multiple files in a single operation
	LintBatch(ctx context.Context, files map[string][]byte) (map[string]*LintResult, error)
}

// ExecuteLintersBatched runs linters with batching support for better performance
func (be *BatchExecutor) ExecuteLintersBatched(ctx context.Context, linters []Linter, files map[string][]byte) map[string][]LintTaskResult {
	if len(files) == 0 {
		return nil
	}

	results := make(map[string][]LintTaskResult)
	var mu sync.Mutex

	// Separate batching and non-batching linters
	var batchingLinters []BatchingLinter
	var regularLinters []Linter

	for _, linter := range linters {
		if bl, ok := linter.(BatchingLinter); ok {
			batchingLinters = append(batchingLinters, bl)
		} else {
			regularLinters = append(regularLinters, linter)
		}
	}

	var wg sync.WaitGroup

	// Process batching linters
	for _, batchLinter := range batchingLinters {
		wg.Add(1)
		go func(bl BatchingLinter) {
			defer wg.Done()

			// Filter files this linter can handle
			linterFiles := make(map[string][]byte)
			for path, content := range files {
				if bl.CanHandle(path) {
					linterFiles[path] = content
				}
			}

			if len(linterFiles) == 0 {
				return
			}

			// Run batch linting
			batchResults, err := bl.LintBatch(ctx, linterFiles)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				// Add error for all files
				for path := range linterFiles {
					results[path] = append(results[path], LintTaskResult{
						LinterName: bl.Name(),
						Error:      err,
					})
				}
			} else {
				// Add results for each file
				for path, result := range batchResults {
					results[path] = append(results[path], LintTaskResult{
						LinterName: bl.Name(),
						Result:     result,
					})
				}
			}
		}(batchLinter)
	}

	// Process regular linters using the existing parallel executor
	if len(regularLinters) > 0 {
		var tasks []LintTask
		for path, content := range files {
			for _, linter := range regularLinters {
				if linter.CanHandle(path) {
					tasks = append(tasks, LintTask{
						Linter:   linter,
						FilePath: path,
						Content:  content,
					})
				}
			}
		}

		if len(tasks) > 0 {
			taskResults := be.executor.ExecuteTasks(ctx, tasks)

			mu.Lock()
			for _, taskResult := range taskResults {
				// Find the file path from the task
				for _, task := range tasks {
					if task.Linter.Name() == taskResult.LinterName {
						results[task.FilePath] = append(results[task.FilePath], taskResult)
						break
					}
				}
			}
			mu.Unlock()
		}
	}

	wg.Wait()
	return results
}
