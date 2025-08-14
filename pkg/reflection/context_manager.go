package reflection

import (
	"fmt"
	"sync"
	"time"
)

// ContextManager manages the overall context state
type ContextManager struct {
	mu sync.RWMutex

	accumulator   *ContextAccumulator
	workingMemory *WorkingMemory
	sessionID     string
	startTime     time.Time
	projectPath   string
	projectName   string

	// Task tracking
	currentTask  string
	previousTask string
	taskHistory  []TaskRecord
}

// WorkingMemory represents the current working state
type WorkingMemory struct {
	CurrentTask    string
	SubTasks       []string
	CompletedSteps []CompletedStep
	PendingSteps   []string
	Blockers       []Blocker
	Discoveries    map[string]string
	Assumptions    map[string]string
	Decisions      map[string]Decision
}

// TaskRecord represents a historical task
type TaskRecord struct {
	Task      string
	StartTime time.Time
	EndTime   time.Time
	Completed bool
}

// CompletedStep represents a completed task step
type CompletedStep struct {
	Description string
	CompletedAt time.Time
	Successful  bool
}

// Blocker represents something blocking progress
type Blocker struct {
	Description string
	Type        string // "missing_info", "error", "unclear_requirement"
	Resolved    bool
}

// Decision represents a decision made during the session
type Decision struct {
	Description string
	Rationale   string
	Timestamp   time.Time
}

// ContextSnapshot represents a point-in-time context state
type ContextSnapshot struct {
	SessionID     string
	Timestamp     time.Time
	WorkingMemory *WorkingMemory
	CurrentTask   string
	Operations    []Operation
	Score         ContextScore
}

// ContextSummary provides a high-level summary
type ContextSummary struct {
	CurrentTask     string
	Status          string
	ProgressPercent float64
	CompletedItems  []string
	InProgressItems []string
	KeyDiscoveries  []string
	NextSteps       []string
	SessionDuration time.Duration
}

// WorkingMemoryUpdate represents an update to working memory
type WorkingMemoryUpdate struct {
	Type  string
	Value interface{}
}

// NewContextManager creates a new context manager
func NewContextManager() *ContextManager {
	return &ContextManager{
		accumulator: NewContextAccumulator(),
		workingMemory: &WorkingMemory{
			Discoveries: make(map[string]string),
			Assumptions: make(map[string]string),
			Decisions:   make(map[string]Decision),
		},
		startTime: time.Now(),
	}
}

// SetCurrentTask sets the current task
func (c *ContextManager) SetCurrentTask(task string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.currentTask != "" && c.currentTask != task {
		// Save previous task to history
		c.taskHistory = append(c.taskHistory, TaskRecord{
			Task:      c.currentTask,
			StartTime: c.startTime,
			EndTime:   time.Now(),
			Completed: false,
		})
		c.previousTask = c.currentTask
	}

	c.currentTask = task
	c.workingMemory.CurrentTask = task
}

// IsTaskSwitch checks if the new task is different from current
func (c *ContextManager) IsTaskSwitch(newTask string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.currentTask != "" && c.currentTask != newTask
}

// ProcessToolResult processes the result of a tool execution
func (c *ContextManager) ProcessToolResult(toolName string, output interface{}, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update statistics based on tool results
	if success {
		// Track successful operations
		if toolName == "Search" || toolName == "FindSymbol" {
			// Record discoveries if relevant
			if output != nil {
				key := fmt.Sprintf("%s_result_%d", toolName, time.Now().Unix())
				c.workingMemory.Discoveries[key] = fmt.Sprintf("%v", output)
			}
		}
	}
}

// CreateSnapshot creates a snapshot of current context
func (c *ContextManager) CreateSnapshot() *ContextSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &ContextSnapshot{
		SessionID:     c.sessionID,
		Timestamp:     time.Now(),
		WorkingMemory: c.copyWorkingMemory(),
		CurrentTask:   c.currentTask,
		Operations:    c.accumulator.GetRecentOperations(50),
		Score:         c.accumulator.CalculateContextScore(),
	}
}

// RestoreFromSnapshot restores context from a snapshot
func (c *ContextManager) RestoreFromSnapshot(snapshot *ContextSnapshot) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	c.sessionID = snapshot.SessionID
	c.currentTask = snapshot.CurrentTask
	c.workingMemory = c.copyWorkingMemory()

	// Clear and repopulate accumulator
	c.accumulator.Clear()
	for _, op := range snapshot.Operations {
		c.accumulator.RecordOperation(op)
	}

	return nil
}

// GenerateSummary generates a human-readable summary
func (c *ContextManager) GenerateSummary() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summary := fmt.Sprintf("Task: %s\n", c.currentTask)

	if len(c.workingMemory.CompletedSteps) > 0 {
		summary += fmt.Sprintf("Completed: %d steps\n", len(c.workingMemory.CompletedSteps))
	}

	if len(c.workingMemory.PendingSteps) > 0 {
		summary += fmt.Sprintf("Pending: %d steps\n", len(c.workingMemory.PendingSteps))
	}

	if len(c.workingMemory.Blockers) > 0 {
		unresolvedCount := 0
		for _, b := range c.workingMemory.Blockers {
			if !b.Resolved {
				unresolvedCount++
			}
		}
		if unresolvedCount > 0 {
			summary += fmt.Sprintf("Blockers: %d unresolved\n", unresolvedCount)
		}
	}

	if len(c.workingMemory.Discoveries) > 0 {
		summary += fmt.Sprintf("Key discoveries: %d\n", len(c.workingMemory.Discoveries))
	}

	return summary
}

// GenerateContextSummary generates a detailed context summary
func (c *ContextManager) GenerateContextSummary() *ContextSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	completed := make([]string, 0, len(c.workingMemory.CompletedSteps))
	for _, step := range c.workingMemory.CompletedSteps {
		completed = append(completed, step.Description)
	}

	discoveries := make([]string, 0, len(c.workingMemory.Discoveries))
	for _, disc := range c.workingMemory.Discoveries {
		discoveries = append(discoveries, disc)
	}

	progress := 0.0
	total := len(c.workingMemory.CompletedSteps) + len(c.workingMemory.PendingSteps)
	if total > 0 {
		progress = float64(len(c.workingMemory.CompletedSteps)) / float64(total) * 100
	}

	status := "In Progress"
	if len(c.workingMemory.PendingSteps) == 0 && len(c.workingMemory.CompletedSteps) > 0 {
		status = "Complete"
	} else if len(c.workingMemory.Blockers) > 0 {
		status = "Blocked"
	}

	return &ContextSummary{
		CurrentTask:     c.currentTask,
		Status:          status,
		ProgressPercent: progress,
		CompletedItems:  completed,
		InProgressItems: c.workingMemory.SubTasks,
		KeyDiscoveries:  discoveries,
		NextSteps:       c.workingMemory.PendingSteps,
		SessionDuration: time.Since(c.startTime),
	}
}

// UpdateWorkingMemory updates the working memory
func (c *ContextManager) UpdateWorkingMemory(update WorkingMemoryUpdate) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch update.Type {
	case "add_subtask":
		if task, ok := update.Value.(string); ok {
			c.workingMemory.SubTasks = append(c.workingMemory.SubTasks, task)
		}
	case "complete_step":
		if step, ok := update.Value.(CompletedStep); ok {
			c.workingMemory.CompletedSteps = append(c.workingMemory.CompletedSteps, step)
		}
	case "add_pending":
		if step, ok := update.Value.(string); ok {
			c.workingMemory.PendingSteps = append(c.workingMemory.PendingSteps, step)
		}
	case "add_blocker":
		if blocker, ok := update.Value.(Blocker); ok {
			c.workingMemory.Blockers = append(c.workingMemory.Blockers, blocker)
		}
	case "add_discovery":
		if disc, ok := update.Value.(map[string]string); ok {
			for k, v := range disc {
				c.workingMemory.Discoveries[k] = v
			}
		}
	case "add_assumption":
		if assump, ok := update.Value.(map[string]string); ok {
			for k, v := range assump {
				c.workingMemory.Assumptions[k] = v
			}
		}
	case "add_decision":
		if dec, ok := update.Value.(Decision); ok {
			c.workingMemory.Decisions[dec.Description] = dec
		}
	default:
		return fmt.Errorf("unknown update type: %s", update.Type)
	}

	return nil
}

// copyWorkingMemory creates a deep copy of working memory
func (c *ContextManager) copyWorkingMemory() *WorkingMemory {
	wm := &WorkingMemory{
		CurrentTask:    c.workingMemory.CurrentTask,
		SubTasks:       make([]string, len(c.workingMemory.SubTasks)),
		CompletedSteps: make([]CompletedStep, len(c.workingMemory.CompletedSteps)),
		PendingSteps:   make([]string, len(c.workingMemory.PendingSteps)),
		Blockers:       make([]Blocker, len(c.workingMemory.Blockers)),
		Discoveries:    make(map[string]string),
		Assumptions:    make(map[string]string),
		Decisions:      make(map[string]Decision),
	}

	copy(wm.SubTasks, c.workingMemory.SubTasks)
	copy(wm.CompletedSteps, c.workingMemory.CompletedSteps)
	copy(wm.PendingSteps, c.workingMemory.PendingSteps)
	copy(wm.Blockers, c.workingMemory.Blockers)

	for k, v := range c.workingMemory.Discoveries {
		wm.Discoveries[k] = v
	}
	for k, v := range c.workingMemory.Assumptions {
		wm.Assumptions[k] = v
	}
	for k, v := range c.workingMemory.Decisions {
		wm.Decisions[k] = v
	}

	return wm
}

// GetProjectPath returns the project path
func (c *ContextManager) GetProjectPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.projectPath
}

// GetProjectName returns the project name
func (c *ContextManager) GetProjectName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.projectName
}

// SetProjectInfo sets the project information
func (c *ContextManager) SetProjectInfo(path, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.projectPath = path
	c.projectName = name
}

// SetWorkingMemory replaces the current working memory
func (c *ContextManager) SetWorkingMemory(wm *WorkingMemory) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workingMemory = wm
}

// GetWorkingMemory returns the current working memory
func (c *ContextManager) GetWorkingMemory() *WorkingMemory {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.copyWorkingMemory()
}
