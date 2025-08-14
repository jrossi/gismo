package reflection

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

// Storage handles persistence of reflection data in DuckDB
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new reflection storage instance
func NewStorage(db *sql.DB) (*Storage, error) {
	s := &Storage{db: db}
	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	return s, nil
}

// initSchema is no longer needed as the schema is created via knowledge.AddReflectionSchema
// This method is kept for backward compatibility but does nothing
func (s *Storage) initSchema() error {
	// Schema is now initialized through knowledge.AddReflectionSchema
	// which integrates with the existing knowledge database
	return nil
}

// SaveSession creates or updates a session
func (s *Storage) SaveSession(sessionID, projectPath, projectName string, metadata map[string]interface{}) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO reflection_sessions (session_id, project_path, project_name, start_time, last_checkpoint, metadata)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (session_id) DO UPDATE SET
			last_checkpoint = EXCLUDED.last_checkpoint,
			metadata = EXCLUDED.metadata
	`

	now := time.Now()
	_, err = s.db.Exec(query, sessionID, projectPath, projectName, now, now, string(metadataJSON))
	return err
}

// SaveOperation records a tool operation
func (s *Storage) SaveOperation(op Operation, sessionID string) error {
	argsJSON, _ := json.Marshal(op.Arguments)
	var resultJSON []byte
	if op.Result != nil {
		resultJSON, _ = json.Marshal(op.Result)
	}

	query := `
		INSERT INTO reflection_operations 
		(session_id, tool_name, operation_type, arguments, result, success, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	success := op.Result != nil && op.Result.Success
	_, err := s.db.Exec(query, sessionID, op.ToolName, op.Type, string(argsJSON),
		string(resultJSON), success, op.Timestamp)
	return err
}

// SaveCheckpoint saves a context checkpoint
func (s *Storage) SaveCheckpoint(checkpoint Checkpoint, sessionID string) error {
	if checkpoint.Snapshot == nil {
		return fmt.Errorf("checkpoint snapshot is nil")
	}

	wmJSON, _ := json.Marshal(checkpoint.Snapshot.WorkingMemory)
	scoreJSON, _ := json.Marshal(checkpoint.Snapshot.Score)
	opsJSON, _ := json.Marshal(checkpoint.Snapshot.Operations)

	query := `
		INSERT INTO reflection_checkpoints 
		(checkpoint_id, session_id, timestamp, reason, working_memory, context_score, operations_snapshot)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, checkpoint.ID, sessionID, checkpoint.Timestamp,
		checkpoint.Reason, string(wmJSON), string(scoreJSON), string(opsJSON))
	return err
}

// LoadCheckpoint retrieves a checkpoint
func (s *Storage) LoadCheckpoint(checkpointID string) (*Checkpoint, error) {
	query := `
		SELECT checkpoint_id, session_id, timestamp, reason, working_memory, context_score, operations_snapshot
		FROM reflection_checkpoints
		WHERE checkpoint_id = ?
	`

	var checkpoint Checkpoint
	var wmJSON, scoreJSON, opsJSON string
	var sessionID string

	err := s.db.QueryRow(query, checkpointID).Scan(
		&checkpoint.ID, &sessionID, &checkpoint.Timestamp, &checkpoint.Reason,
		&wmJSON, &scoreJSON, &opsJSON,
	)
	if err != nil {
		return nil, err
	}

	// Reconstruct the snapshot
	snapshot := &ContextSnapshot{SessionID: sessionID}
	_ = json.Unmarshal([]byte(wmJSON), &snapshot.WorkingMemory)
	_ = json.Unmarshal([]byte(scoreJSON), &snapshot.Score)
	_ = json.Unmarshal([]byte(opsJSON), &snapshot.Operations)
	checkpoint.Snapshot = snapshot

	return &checkpoint, nil
}

// GetLatestCheckpoint retrieves the most recent checkpoint for a session
func (s *Storage) GetLatestCheckpoint(sessionID string) (*Checkpoint, error) {
	query := `
		SELECT checkpoint_id
		FROM reflection_checkpoints
		WHERE session_id = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var checkpointID string
	err := s.db.QueryRow(query, sessionID).Scan(&checkpointID)
	if err != nil {
		return nil, err
	}

	return s.LoadCheckpoint(checkpointID)
}

// SavePattern saves a learned pattern
func (s *Storage) SavePattern(pattern OperationPattern, patternType string, projectContext string) error {
	sequenceJSON, _ := json.Marshal(pattern.Sequence)

	query := `
		INSERT INTO reflection_patterns 
		(pattern_sequence, occurrence_count, success_rate, pattern_type, last_seen, project_context)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (pattern_sequence, project_context) DO UPDATE SET
			occurrence_count = occurrence_count + 1,
			success_rate = EXCLUDED.success_rate,
			last_seen = EXCLUDED.last_seen
	`

	_, err := s.db.Exec(query, string(sequenceJSON), pattern.Occurrences,
		pattern.SuccessRate, patternType, pattern.LastSeen, projectContext)
	return err
}

// LoadPatterns loads learned patterns
func (s *Storage) LoadPatterns(patternType string, minOccurrences int) ([]OperationPattern, error) {
	query := `
		SELECT pattern_sequence, occurrence_count, success_rate, last_seen
		FROM reflection_patterns
		WHERE pattern_type = ? AND occurrence_count >= ?
		ORDER BY occurrence_count DESC, last_seen DESC
	`

	rows, err := s.db.Query(query, patternType, minOccurrences)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []OperationPattern
	for rows.Next() {
		var pattern OperationPattern
		var sequenceJSON string

		err := rows.Scan(&sequenceJSON, &pattern.Occurrences, &pattern.SuccessRate, &pattern.LastSeen)
		if err != nil {
			continue
		}

		_ = json.Unmarshal([]byte(sequenceJSON), &pattern.Sequence)
		patterns = append(patterns, pattern)
	}

	return patterns, nil
}

// SaveWorkingMemory persists the current working memory
func (s *Storage) SaveWorkingMemory(sessionID string, wm *WorkingMemory) error {
	subTasksJSON, _ := json.Marshal(wm.SubTasks)
	completedJSON, _ := json.Marshal(wm.CompletedSteps)
	pendingJSON, _ := json.Marshal(wm.PendingSteps)
	blockersJSON, _ := json.Marshal(wm.Blockers)
	discoveriesJSON, _ := json.Marshal(wm.Discoveries)
	assumptionsJSON, _ := json.Marshal(wm.Assumptions)
	decisionsJSON, _ := json.Marshal(wm.Decisions)

	query := `
		INSERT INTO reflection_working_memory 
		(session_id, current_task, sub_tasks, completed_steps, pending_steps, 
		 blockers, discoveries, assumptions, decisions, last_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (session_id) DO UPDATE SET
			current_task = EXCLUDED.current_task,
			sub_tasks = EXCLUDED.sub_tasks,
			completed_steps = EXCLUDED.completed_steps,
			pending_steps = EXCLUDED.pending_steps,
			blockers = EXCLUDED.blockers,
			discoveries = EXCLUDED.discoveries,
			assumptions = EXCLUDED.assumptions,
			decisions = EXCLUDED.decisions,
			last_updated = EXCLUDED.last_updated
	`

	_, err := s.db.Exec(query, sessionID, wm.CurrentTask,
		string(subTasksJSON), string(completedJSON), string(pendingJSON),
		string(blockersJSON), string(discoveriesJSON), string(assumptionsJSON),
		string(decisionsJSON), time.Now())
	return err
}

// LoadWorkingMemory retrieves working memory for a session
func (s *Storage) LoadWorkingMemory(sessionID string) (*WorkingMemory, error) {
	query := `
		SELECT current_task, sub_tasks, completed_steps, pending_steps,
		       blockers, discoveries, assumptions, decisions
		FROM reflection_working_memory
		WHERE session_id = ?
	`

	var wm WorkingMemory
	var subTasksJSON, completedJSON, pendingJSON, blockersJSON string
	var discoveriesJSON, assumptionsJSON, decisionsJSON string

	err := s.db.QueryRow(query, sessionID).Scan(
		&wm.CurrentTask, &subTasksJSON, &completedJSON, &pendingJSON,
		&blockersJSON, &discoveriesJSON, &assumptionsJSON, &decisionsJSON,
	)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(subTasksJSON), &wm.SubTasks)
	_ = json.Unmarshal([]byte(completedJSON), &wm.CompletedSteps)
	_ = json.Unmarshal([]byte(pendingJSON), &wm.PendingSteps)
	_ = json.Unmarshal([]byte(blockersJSON), &wm.Blockers)
	_ = json.Unmarshal([]byte(discoveriesJSON), &wm.Discoveries)
	_ = json.Unmarshal([]byte(assumptionsJSON), &wm.Assumptions)
	_ = json.Unmarshal([]byte(decisionsJSON), &wm.Decisions)

	return &wm, nil
}

// SaveContextScore records a context score
func (s *Storage) SaveContextScore(sessionID string, score ContextScore) error {
	query := `
		INSERT INTO reflection_context_scores 
		(session_id, timestamp, completeness, relevance, confidence, clarity, code_understanding, depth)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, sessionID, time.Now(), score.Completeness,
		score.Relevance, score.Confidence, score.Clarity, score.CodeUnderstanding, score.Depth)
	return err
}

// SaveReflectionEvent records when reflection was triggered
func (s *Storage) SaveReflectionEvent(sessionID string, trigger string, prompt string, score ContextScore, opCount int) error {
	scoreJSON, _ := json.Marshal(score)

	query := `
		INSERT INTO reflection_events 
		(session_id, timestamp, trigger_reason, prompt, context_score, operation_count)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, sessionID, time.Now(), trigger, prompt, string(scoreJSON), opCount)
	return err
}

// GetSessionStats retrieves statistics for a session
func (s *Storage) GetSessionStats(sessionID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get operation count
	var opCount int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM reflection_operations WHERE session_id = ?
	`, sessionID).Scan(&opCount)
	if err != nil {
		return nil, err
	}
	stats["total_operations"] = opCount

	// Get success rate
	var successCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM reflection_operations WHERE session_id = ? AND success = true
	`, sessionID).Scan(&successCount)
	if err != nil {
		return nil, err
	}
	if opCount > 0 {
		stats["success_rate"] = float64(successCount) / float64(opCount)
	}

	// Get reflection count
	var reflectionCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM reflection_events WHERE session_id = ?
	`, sessionID).Scan(&reflectionCount)
	if err != nil {
		return nil, err
	}
	stats["reflection_count"] = reflectionCount

	// Get average context score
	var avgScore float64
	err = s.db.QueryRow(`
		SELECT AVG((completeness + relevance + confidence + clarity) / 4.0)
		FROM reflection_context_scores WHERE session_id = ?
	`, sessionID).Scan(&avgScore)
	if err == nil {
		stats["average_context_score"] = avgScore
	}

	return stats, nil
}

// RestoreSession loads a previous session's state
func (s *Storage) RestoreSession(sessionID string) (*SessionState, error) {
	// Load session metadata
	var projectPath, projectName string
	var startTime time.Time
	var metadataJSON string

	err := s.db.QueryRow(`
		SELECT project_path, project_name, start_time, metadata
		FROM reflection_sessions WHERE session_id = ?
	`, sessionID).Scan(&projectPath, &projectName, &startTime, &metadataJSON)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Load working memory
	wm, err := s.LoadWorkingMemory(sessionID)
	if err != nil {
		// Create new if doesn't exist
		wm = &WorkingMemory{
			Discoveries: make(map[string]string),
			Assumptions: make(map[string]string),
			Decisions:   make(map[string]Decision),
		}
	}

	// Load recent operations
	rows, err := s.db.Query(`
		SELECT tool_name, operation_type, arguments, result, success, timestamp
		FROM reflection_operations 
		WHERE session_id = ?
		ORDER BY timestamp DESC
		LIMIT 50
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var operations []Operation
	for rows.Next() {
		var op Operation
		var opType int
		var argsJSON, resultJSON sql.NullString
		var success bool

		err := rows.Scan(&op.ToolName, &opType, &argsJSON, &resultJSON, &success, &op.Timestamp)
		if err != nil {
			continue
		}

		op.Type = OperationType(opType)
		if argsJSON.Valid {
			_ = json.Unmarshal([]byte(argsJSON.String), &op.Arguments)
		}
		if resultJSON.Valid {
			var result OperationResult
			_ = json.Unmarshal([]byte(resultJSON.String), &result)
			op.Result = &result
		}

		operations = append(operations, op)
	}

	// Reverse to get chronological order
	for i, j := 0, len(operations)-1; i < j; i, j = i+1, j-1 {
		operations[i], operations[j] = operations[j], operations[i]
	}

	return &SessionState{
		SessionID:     sessionID,
		ProjectPath:   projectPath,
		ProjectName:   projectName,
		StartTime:     startTime,
		WorkingMemory: wm,
		Operations:    operations,
	}, nil
}

// SessionState represents a restored session
type SessionState struct {
	SessionID     string
	ProjectPath   string
	ProjectName   string
	StartTime     time.Time
	WorkingMemory *WorkingMemory
	Operations    []Operation
}

// CleanupOldSessions removes sessions older than the specified duration
func (s *Storage) CleanupOldSessions(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	query := `
		DELETE FROM reflection_sessions 
		WHERE last_checkpoint < ? AND status != 'active'
	`

	_, err := s.db.Exec(query, cutoff)
	return err
}
