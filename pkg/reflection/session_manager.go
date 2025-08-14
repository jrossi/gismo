package reflection

import (
	"fmt"
	"sync"
	"time"
)

// SessionManager manages session state and checkpoints
type SessionManager struct {
	mu sync.RWMutex

	sessionID       string
	startTime       time.Time
	checkpoints     []Checkpoint
	reflectionCount int

	// Auto-checkpoint configuration
	autoCheckpoint     bool
	checkpointInterval time.Duration
	lastCheckpoint     time.Time
}

// Checkpoint represents a saved context state
type Checkpoint struct {
	ID        string
	Timestamp time.Time
	Reason    string
	Snapshot  *ContextSnapshot
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessionID:      generateSessionID(),
		startTime:      time.Now(),
		lastCheckpoint: time.Now(),
	}
}

// EnableAutoCheckpoint enables automatic checkpointing
func (s *SessionManager) EnableAutoCheckpoint(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.autoCheckpoint = true
	s.checkpointInterval = interval
}

// DisableAutoCheckpoint disables automatic checkpointing
func (s *SessionManager) DisableAutoCheckpoint() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.autoCheckpoint = false
}

// ShouldCheckpoint checks if auto-checkpoint is due
func (s *SessionManager) ShouldCheckpoint() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.autoCheckpoint {
		return false
	}

	return time.Since(s.lastCheckpoint) >= s.checkpointInterval
}

// SaveCheckpoint saves a context checkpoint
func (s *SessionManager) SaveCheckpoint(snapshot *ContextSnapshot, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	checkpoint := Checkpoint{
		ID:        generateCheckpointID(),
		Timestamp: time.Now(),
		Reason:    reason,
		Snapshot:  snapshot,
	}

	s.checkpoints = append(s.checkpoints, checkpoint)
	s.lastCheckpoint = time.Now()

	// Keep only last 10 checkpoints to avoid memory issues
	if len(s.checkpoints) > 10 {
		s.checkpoints = s.checkpoints[len(s.checkpoints)-10:]
	}

	return nil
}

// LoadCheckpoint loads a specific checkpoint
func (s *SessionManager) LoadCheckpoint(checkpointID string) (*ContextSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, cp := range s.checkpoints {
		if cp.ID == checkpointID {
			return cp.Snapshot, nil
		}
	}

	return nil, fmt.Errorf("checkpoint not found: %s", checkpointID)
}

// GetLatestCheckpoint returns the most recent checkpoint
func (s *SessionManager) GetLatestCheckpoint() (*ContextSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints available")
	}

	return s.checkpoints[len(s.checkpoints)-1].Snapshot, nil
}

// ListCheckpoints returns all available checkpoints
func (s *SessionManager) ListCheckpoints() []Checkpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Checkpoint, len(s.checkpoints))
	copy(result, s.checkpoints)
	return result
}

// IncrementReflectionCount increments the reflection counter
func (s *SessionManager) IncrementReflectionCount() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reflectionCount++
}

// ReflectionCount returns the number of reflections triggered
func (s *SessionManager) ReflectionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.reflectionCount
}

// GetSessionID returns the current session ID
func (s *SessionManager) GetSessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sessionID
}

// GetSessionDuration returns how long the session has been running
func (s *SessionManager) GetSessionDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return time.Since(s.startTime)
}

// Reset resets the session manager
func (s *SessionManager) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessionID = generateSessionID()
	s.startTime = time.Now()
	s.checkpoints = nil
	s.reflectionCount = 0
	s.lastCheckpoint = time.Now()
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// generateCheckpointID generates a unique checkpoint ID
func generateCheckpointID() string {
	return fmt.Sprintf("checkpoint_%d", time.Now().UnixNano())
}
