package reflection

import (
	"sync"
	"time"
)

// PatternLearner learns from operation patterns
type PatternLearner struct {
	mu sync.RWMutex

	// Pattern tracking
	successPatterns []OperationPattern
	failurePatterns []OperationPattern
	currentSequence []string

	// Learning configuration
	minOccurrences int
	patternWindow  int
}

// OperationPattern represents a sequence of operations
type OperationPattern struct {
	Sequence    []string
	Occurrences int
	SuccessRate float64
	LastSeen    time.Time
}

// ReflectionStats contains statistics about reflections
type ReflectionStats struct {
	TotalOperations     int
	ReflectionCount     int
	AverageContextScore float64
	SuccessfulPatterns  []OperationPattern
	FailurePatterns     []OperationPattern
}

// NewPatternLearner creates a new pattern learner
func NewPatternLearner() *PatternLearner {
	return &PatternLearner{
		minOccurrences: 3,
		patternWindow:  10,
	}
}

// ObserveOperation observes a new operation for pattern learning
func (p *PatternLearner) ObserveOperation(toolName string, success bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Add to current sequence
	p.currentSequence = append(p.currentSequence, toolName)

	// Keep window size limited
	if len(p.currentSequence) > p.patternWindow {
		p.currentSequence = p.currentSequence[1:]
	}

	// Check if current sequence matches any known patterns
	if len(p.currentSequence) >= 3 {
		p.updatePatterns(success)
	}
}

// updatePatterns updates pattern statistics
func (p *PatternLearner) updatePatterns(success bool) {
	// Look for patterns of length 3-5
	for length := 3; length <= 5 && length <= len(p.currentSequence); length++ {
		sequence := p.currentSequence[len(p.currentSequence)-length:]

		if success {
			p.updateSuccessPattern(sequence)
		} else {
			p.updateFailurePattern(sequence)
		}
	}
}

// updateSuccessPattern updates successful pattern statistics
func (p *PatternLearner) updateSuccessPattern(sequence []string) {
	for i, pattern := range p.successPatterns {
		if p.sequenceMatches(pattern.Sequence, sequence) {
			p.successPatterns[i].Occurrences++
			p.successPatterns[i].LastSeen = time.Now()
			p.successPatterns[i].SuccessRate = (p.successPatterns[i].SuccessRate*float64(p.successPatterns[i].Occurrences-1) + 1.0) / float64(p.successPatterns[i].Occurrences)
			return
		}
	}

	// New pattern
	p.successPatterns = append(p.successPatterns, OperationPattern{
		Sequence:    append([]string{}, sequence...),
		Occurrences: 1,
		SuccessRate: 1.0,
		LastSeen:    time.Now(),
	})
}

// updateFailurePattern updates failure pattern statistics
func (p *PatternLearner) updateFailurePattern(sequence []string) {
	for i, pattern := range p.failurePatterns {
		if p.sequenceMatches(pattern.Sequence, sequence) {
			p.failurePatterns[i].Occurrences++
			p.failurePatterns[i].LastSeen = time.Now()
			return
		}
	}

	// New pattern
	p.failurePatterns = append(p.failurePatterns, OperationPattern{
		Sequence:    append([]string{}, sequence...),
		Occurrences: 1,
		SuccessRate: 0.0,
		LastSeen:    time.Now(),
	})
}

// sequenceMatches checks if two sequences match
func (p *PatternLearner) sequenceMatches(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SuggestsReflection checks if patterns suggest reflection is needed
func (p *PatternLearner) SuggestsReflection() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check if current sequence matches a known failure pattern
	for _, pattern := range p.failurePatterns {
		if pattern.Occurrences >= p.minOccurrences {
			if p.currentSequenceMatchesPattern(pattern.Sequence) {
				return true
			}
		}
	}

	// Check for repetitive patterns (might indicate confusion)
	if p.hasRepetitivePattern() {
		return true
	}

	return false
}

// currentSequenceMatchesPattern checks if current sequence contains pattern
func (p *PatternLearner) currentSequenceMatchesPattern(pattern []string) bool {
	if len(p.currentSequence) < len(pattern) {
		return false
	}

	// Check if pattern appears at the end of current sequence
	start := len(p.currentSequence) - len(pattern)
	for i, op := range pattern {
		if p.currentSequence[start+i] != op {
			return false
		}
	}
	return true
}

// hasRepetitivePattern checks for repetitive operations
func (p *PatternLearner) hasRepetitivePattern() bool {
	if len(p.currentSequence) < 6 {
		return false
	}

	// Check for repeated pairs or triples
	recent := p.currentSequence[len(p.currentSequence)-6:]

	// Check for pattern like [A, B, A, B, A, B]
	if recent[0] == recent[2] && recent[2] == recent[4] &&
		recent[1] == recent[3] && recent[3] == recent[5] {
		return true
	}

	// Check for pattern like [A, B, C, A, B, C]
	if len(recent) >= 6 {
		if recent[0] == recent[3] && recent[1] == recent[4] && recent[2] == recent[5] {
			return true
		}
	}

	return false
}

// GetSuccessfulPatterns returns patterns that lead to success
func (p *PatternLearner) GetSuccessfulPatterns() []OperationPattern {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []OperationPattern
	for _, pattern := range p.successPatterns {
		if pattern.Occurrences >= p.minOccurrences && pattern.SuccessRate > 0.7 {
			result = append(result, pattern)
		}
	}
	return result
}

// GetFailurePatterns returns patterns that lead to failure
func (p *PatternLearner) GetFailurePatterns() []OperationPattern {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []OperationPattern
	for _, pattern := range p.failurePatterns {
		if pattern.Occurrences >= p.minOccurrences {
			result = append(result, pattern)
		}
	}
	return result
}

// Reset clears all learned patterns
func (p *PatternLearner) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.successPatterns = nil
	p.failurePatterns = nil
	p.currentSequence = nil
}

// RestorePatterns restores previously learned patterns
func (p *PatternLearner) RestorePatterns(success, failure []OperationPattern) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.successPatterns = success
	p.failurePatterns = failure
}
