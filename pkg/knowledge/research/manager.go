package research

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	"github.com/jrossi/gismo/pkg/external/exa"
)

// TaskStatus represents the status of a research task
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusPolling   TaskStatus = "polling"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
	StatusCanceled  TaskStatus = "canceled"
)

// ResearchManager manages research tasks with async processing
type ResearchManager struct {
	db             *sql.DB
	exaClient      *exa.Client
	tasks          sync.Map // taskID -> *TaskWorker
	pollInterval   time.Duration
	maxCost        float64 // Maximum cost per task
	requireConsent bool
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// TaskWorker handles polling for a specific task
type TaskWorker struct {
	taskID     string
	exaTaskID  string
	manager    *ResearchManager
	pollTicker *time.Ticker
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewResearchManager creates a new research manager
func NewResearchManager(db *sql.DB, apiKey string) (*ResearchManager, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required for research tasks")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ResearchManager{
		db:             db,
		exaClient:      exa.NewClient(apiKey),
		pollInterval:   10 * time.Second,
		maxCost:        15.0, // Default $15 max per task
		requireConsent: true, // Require explicit consent by default
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

// SetMaxCost sets the maximum allowed cost per task
func (m *ResearchManager) SetMaxCost(maxCost float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxCost = maxCost
}

// SetRequireConsent sets whether user consent is required
func (m *ResearchManager) SetRequireConsent(require bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requireConsent = require
}

// CreateTask creates a new research task with cost consent
func (m *ResearchManager) CreateTask(ctx context.Context, instructions string, model string, schema interface{}, projectContext string, userConsent bool) (string, error) {
	// Check consent requirement
	if m.requireConsent && !userConsent {
		return "", fmt.Errorf("user consent required for research tasks (potential cost up to $%.2f)", m.maxCost)
	}

	// Generate task ID
	var taskID string
	err := m.db.QueryRowContext(ctx,
		`INSERT INTO exa_research_tasks (instructions, model, output_schema, project_context, user_consent, consent_timestamp)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		instructions, model, schema, projectContext, userConsent, time.Now()).Scan(&taskID)
	if err != nil {
		return "", fmt.Errorf("failed to create task record: %w", err)
	}

	// Log event
	m.logEvent(taskID, "created", map[string]interface{}{
		"instructions": instructions,
		"model":        model,
		"has_schema":   schema != nil,
	})

	// Start the task asynchronously
	go m.startTask(taskID, instructions, model, schema)

	return taskID, nil
}

// startTask initiates the research task with Exa API
func (m *ResearchManager) startTask(taskID string, instructions string, model string, schema interface{}) {
	// Update status to running
	_, err := m.db.Exec(
		`UPDATE exa_research_tasks SET status = $1, started_at = $2 WHERE id = $3`,
		StatusRunning, time.Now(), taskID)
	if err != nil {
		log.Printf("Failed to update task status: %v", err)
		return
	}

	// Prepare research request
	req := &exa.ResearchRequest{
		Instructions: instructions,
		Model:        model,
	}

	if schema != nil {
		req.Output = &exa.ResearchOutputConfig{
			Schema: schema,
		}
	}

	// Create research task via API
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := m.exaClient.CreateResearchTask(ctx, req)
	if err != nil {
		m.markTaskFailed(taskID, fmt.Sprintf("Failed to create research task: %v", err))
		return
	}

	// Update task with Exa task ID
	_, err = m.db.Exec(
		`UPDATE exa_research_tasks SET exa_task_id = $1, status = $2 WHERE id = $3`,
		resp.ID, StatusPolling, taskID)
	if err != nil {
		log.Printf("Failed to update task with Exa ID: %v", err)
		return
	}

	// Log event
	m.logEvent(taskID, "started", map[string]interface{}{
		"exa_task_id": resp.ID,
	})

	// Start polling worker
	m.startPollingWorker(taskID, resp.ID)
}

// startPollingWorker starts a background worker to poll for task completion
func (m *ResearchManager) startPollingWorker(taskID, exaTaskID string) {
	ctx, cancel := context.WithCancel(m.ctx)

	worker := &TaskWorker{
		taskID:     taskID,
		exaTaskID:  exaTaskID,
		manager:    m,
		pollTicker: time.NewTicker(m.pollInterval),
		ctx:        ctx,
		cancel:     cancel,
	}

	m.tasks.Store(taskID, worker)
	m.wg.Add(1)

	go worker.run()
}

// run is the main polling loop for a task worker
func (w *TaskWorker) run() {
	defer w.manager.wg.Done()
	defer w.pollTicker.Stop()
	defer w.manager.tasks.Delete(w.taskID)

	maxPolls := 30 // Max 5 minutes of polling (30 * 10s)
	pollCount := 0

	for {
		select {
		case <-w.ctx.Done():
			w.manager.markTaskCanceled(w.taskID)
			return

		case <-w.pollTicker.C:
			pollCount++

			// Check task status
			status, err := w.checkStatus()
			if err != nil {
				log.Printf("Error checking task %s status: %v", w.taskID, err)
				if pollCount >= maxPolls {
					w.manager.markTaskFailed(w.taskID, "Polling timeout exceeded")
					return
				}
				continue
			}

			// Handle based on status
			switch status.Status {
			case "completed":
				w.handleCompletion(status)
				return

			case "failed":
				w.manager.markTaskFailed(w.taskID, status.Error)
				return

			case "running", "pending":
				// Update progress if available
				if status.ProgressMessage != "" {
					w.updateProgress(status.ProgressMessage, status.EstimatedCost)
				}

			default:
				log.Printf("Unknown task status: %s", status.Status)
			}

			// Check timeout
			if pollCount >= maxPolls {
				w.manager.markTaskFailed(w.taskID, "Polling timeout exceeded")
				return
			}
		}
	}
}

// checkStatus checks the current status of the research task
func (w *TaskWorker) checkStatus() (*exa.ResearchTaskStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return w.manager.exaClient.GetResearchTaskStatus(ctx, w.exaTaskID)
}

// updateProgress updates the task progress in the database
func (w *TaskWorker) updateProgress(message string, estimatedCost float64) {
	_, err := w.manager.db.Exec(
		`UPDATE exa_research_tasks SET progress_message = $1, estimated_cost = $2 WHERE id = $3`,
		message, estimatedCost, w.taskID)
	if err != nil {
		log.Printf("Failed to update progress for task %s: %v", w.taskID, err)
	}

	// Log event
	w.manager.logEvent(w.taskID, "progress", map[string]interface{}{
		"message":        message,
		"estimated_cost": estimatedCost,
	})
}

// handleCompletion processes a completed research task
func (w *TaskWorker) handleCompletion(status *exa.ResearchTaskStatus) {
	// Marshal results and citations
	resultJSON, _ := json.Marshal(status.Result)
	citationsJSON, _ := json.Marshal(status.Citations)

	// Update task as completed
	_, err := w.manager.db.Exec(
		`UPDATE exa_research_tasks 
		 SET status = $1, result = $2, citations = $3, actual_cost = $4, completed_at = $5 
		 WHERE id = $6`,
		StatusCompleted, resultJSON, citationsJSON, status.ActualCost, time.Now(), w.taskID)

	if err != nil {
		log.Printf("Failed to update completed task %s: %v", w.taskID, err)
		return
	}

	// Log completion event
	w.manager.logEvent(w.taskID, "completed", map[string]interface{}{
		"actual_cost":    status.ActualCost,
		"citation_count": len(status.Citations),
	})

	log.Printf("Research task %s completed successfully (cost: $%.2f)", w.taskID, status.ActualCost)
}

// markTaskFailed marks a task as failed
func (m *ResearchManager) markTaskFailed(taskID, errorMsg string) {
	_, err := m.db.Exec(
		`UPDATE exa_research_tasks SET status = $1, error_message = $2, completed_at = $3 WHERE id = $4`,
		StatusFailed, errorMsg, time.Now(), taskID)
	if err != nil {
		log.Printf("Failed to mark task %s as failed: %v", taskID, err)
	}

	m.logEvent(taskID, "failed", map[string]interface{}{
		"error": errorMsg,
	})
}

// markTaskCanceled marks a task as canceled
func (m *ResearchManager) markTaskCanceled(taskID string) {
	_, err := m.db.Exec(
		`UPDATE exa_research_tasks SET status = $1, completed_at = $2 WHERE id = $3`,
		StatusCanceled, time.Now(), taskID)
	if err != nil {
		log.Printf("Failed to mark task %s as canceled: %v", taskID, err)
	}

	m.logEvent(taskID, "canceled", nil)
}

// logEvent logs an event for a research task
func (m *ResearchManager) logEvent(taskID, eventType string, eventData interface{}) {
	dataJSON, _ := json.Marshal(eventData)
	_, err := m.db.Exec(
		`INSERT INTO exa_research_events (task_id, event_type, event_data) VALUES ($1, $2, $3)`,
		taskID, eventType, dataJSON)
	if err != nil {
		log.Printf("Failed to log event for task %s: %v", taskID, err)
	}
}

// GetTaskStatus retrieves the current status of a task
func (m *ResearchManager) GetTaskStatus(ctx context.Context, taskID string) (map[string]interface{}, error) {
	var status TaskStatus
	var progressMsg sql.NullString
	var estimatedCost, actualCost sql.NullFloat64
	var result, citations sql.NullString
	var errorMsg sql.NullString

	err := m.db.QueryRowContext(ctx,
		`SELECT status, progress_message, estimated_cost, actual_cost, result, citations, error_message
		 FROM exa_research_tasks WHERE id = $1`,
		taskID).Scan(&status, &progressMsg, &estimatedCost, &actualCost, &result, &citations, &errorMsg)

	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	response := map[string]interface{}{
		"task_id": taskID,
		"status":  status,
	}

	if progressMsg.Valid {
		response["progress_message"] = progressMsg.String
	}
	if estimatedCost.Valid {
		response["estimated_cost"] = estimatedCost.Float64
	}
	if actualCost.Valid {
		response["actual_cost"] = actualCost.Float64
	}
	if result.Valid {
		var resultData interface{}
		if err := json.Unmarshal([]byte(result.String), &resultData); err == nil {
			response["result"] = resultData
		}
	}
	if citations.Valid {
		var citationData []interface{}
		if err := json.Unmarshal([]byte(citations.String), &citationData); err == nil {
			response["citations"] = citationData
		}
	}
	if errorMsg.Valid {
		response["error"] = errorMsg.String
	}

	return response, nil
}

// CancelTask cancels a running research task
func (m *ResearchManager) CancelTask(ctx context.Context, taskID string) error {
	if worker, ok := m.tasks.Load(taskID); ok {
		w := worker.(*TaskWorker)
		w.cancel()
		return nil
	}
	return fmt.Errorf("task %s not found or already completed", taskID)
}

// Shutdown gracefully shuts down the research manager
func (m *ResearchManager) Shutdown() {
	m.cancel()
	m.wg.Wait()
}

// GetActiveTasks returns all active research tasks
func (m *ResearchManager) GetActiveTasks(ctx context.Context, projectContext string) ([]map[string]interface{}, error) {
	query := `
		SELECT id, instructions, model, status, progress_message, 
		       EXTRACT(EPOCH FROM (NOW() - created_at)) as elapsed_seconds,
		       estimated_cost, retry_count
		FROM exa_research_tasks
		WHERE status IN ('pending', 'running', 'polling')
	`
	args := []interface{}{}

	if projectContext != "" {
		query += " AND project_context = $1"
		args = append(args, projectContext)
	}

	query += " ORDER BY created_at DESC"

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query active tasks: %w", err)
	}
	defer rows.Close()

	var tasks []map[string]interface{}
	for rows.Next() {
		var id, instructions, model, status string
		var progressMsg sql.NullString
		var elapsedSeconds float64
		var estimatedCost sql.NullFloat64
		var retryCount int

		err := rows.Scan(&id, &instructions, &model, &status, &progressMsg, &elapsedSeconds, &estimatedCost, &retryCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		task := map[string]interface{}{
			"id":              id,
			"instructions":    instructions,
			"model":           model,
			"status":          status,
			"elapsed_seconds": elapsedSeconds,
			"retry_count":     retryCount,
		}

		if progressMsg.Valid {
			task["progress_message"] = progressMsg.String
		}
		if estimatedCost.Valid {
			task["estimated_cost"] = estimatedCost.Float64
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}
