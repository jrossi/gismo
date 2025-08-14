package reflection

import (
	"sync"
	"time"

	json "github.com/goccy/go-json"
)

// OperationType represents the type of operation performed
type OperationType int

const (
	OperationTypeUnspecified OperationType = iota
	OperationTypeSearch
	OperationTypeRead
	OperationTypeWrite
	OperationTypeAnalyze
	OperationTypeExternalSearch
	OperationTypeSymbolLookup
	OperationTypeReferenceSearch
)

// Operation represents a single tool operation
type Operation struct {
	ToolName  string
	Arguments map[string]json.RawMessage
	Timestamp time.Time
	Type      OperationType
	Intent    string
	Result    *OperationResult
}

// OperationResult represents the result of an operation
type OperationResult struct {
	Success bool
	Output  json.RawMessage
	Error   string
}

// ContextScore represents the quality of the current context
type ContextScore struct {
	Completeness      float64
	Relevance         float64
	Confidence        float64
	Clarity           float64
	Depth             int
	CodeUnderstanding float64
}

// ContextAccumulator tracks operations and builds context understanding
type ContextAccumulator struct {
	mu         sync.RWMutex
	operations []Operation

	// Context tracking
	knowledgeQueries   []string
	symbolsExplored    map[string]*SymbolContext
	filesRead          map[string]time.Time
	filesModified      map[string][]Modification
	externalReferences []ExternalReference

	// Statistics
	totalOperations int
	successfulOps   int
	failedOps       int
	averageScore    float64
	scoreCount      int
}

// SymbolContext represents understanding of a code symbol
type SymbolContext struct {
	Name         string
	Type         string
	Location     string
	References   []string
	LastAccessed time.Time
}

// Modification represents a file modification
type Modification struct {
	Timestamp time.Time
	Type      string
	Summary   string
}

// ExternalReference represents external knowledge accessed
type ExternalReference struct {
	Source    string
	Query     string
	Timestamp time.Time
	Relevant  bool
}

// NewContextAccumulator creates a new context accumulator
func NewContextAccumulator() *ContextAccumulator {
	return &ContextAccumulator{
		operations:      make([]Operation, 0),
		symbolsExplored: make(map[string]*SymbolContext),
		filesRead:       make(map[string]time.Time),
		filesModified:   make(map[string][]Modification),
	}
}

// RecordOperation records a new operation
func (c *ContextAccumulator) RecordOperation(op Operation) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.operations = append(c.operations, op)
	c.totalOperations++

	// Track specific types
	switch op.Type {
	case OperationTypeSymbolLookup:
		// Extract symbol name if available
		if nameRaw, ok := op.Arguments["name"]; ok {
			var name string
			if json.Unmarshal(nameRaw, &name) == nil {
				if _, exists := c.symbolsExplored[name]; !exists {
					c.symbolsExplored[name] = &SymbolContext{
						Name:         name,
						LastAccessed: op.Timestamp,
					}
				}
			}
		}
	case OperationTypeRead:
		// Track file reads
		if pathRaw, ok := op.Arguments["file_path"]; ok {
			var path string
			if json.Unmarshal(pathRaw, &path) == nil {
				c.filesRead[path] = op.Timestamp
			}
		}
	}
}

// UpdateLastOperation updates the most recent operation with results
func (c *ContextAccumulator) UpdateLastOperation(result OperationResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.operations) > 0 {
		c.operations[len(c.operations)-1].Result = &result

		if result.Success {
			c.successfulOps++
		} else {
			c.failedOps++
		}
	}
}

// CalculateContextScore calculates the current context quality score
func (c *ContextAccumulator) CalculateContextScore() ContextScore {
	c.mu.RLock()
	defer c.mu.RUnlock()

	score := ContextScore{
		Depth: len(c.operations),
	}

	// Calculate completeness based on successful operations
	if c.totalOperations > 0 {
		score.Completeness = float64(c.successfulOps) / float64(c.totalOperations)
	}

	// Calculate confidence based on variety of information sources
	sourceCount := 0
	if len(c.knowledgeQueries) > 0 {
		sourceCount++
	}
	if len(c.symbolsExplored) > 0 {
		sourceCount++
	}
	if len(c.filesRead) > 0 {
		sourceCount++
	}
	if len(c.externalReferences) > 0 {
		sourceCount++
	}
	score.Confidence = float64(sourceCount) / 4.0

	// Calculate relevance based on recent operations
	recentOps := 0
	cutoff := time.Now().Add(-5 * time.Minute)
	for i := len(c.operations) - 1; i >= 0 && i >= len(c.operations)-10; i-- {
		if c.operations[i].Timestamp.After(cutoff) {
			recentOps++
		}
	}
	if recentOps > 0 {
		score.Relevance = float64(recentOps) / 10.0
	}

	// Calculate clarity based on operation patterns
	if c.hasLogicalProgression() {
		score.Clarity = 0.8
	} else {
		score.Clarity = 0.4
	}

	// Calculate code understanding
	if len(c.symbolsExplored) > 0 {
		score.CodeUnderstanding = min(1.0, float64(len(c.symbolsExplored))/20.0)
	}

	// Update average score
	c.averageScore = (c.averageScore*float64(c.scoreCount) +
		(score.Completeness+score.Confidence+score.Relevance+score.Clarity)/4.0) /
		float64(c.scoreCount+1)
	c.scoreCount++

	return score
}

// hasLogicalProgression checks if operations follow a logical pattern
func (c *ContextAccumulator) hasLogicalProgression() bool {
	if len(c.operations) < 3 {
		return true
	}

	// Check for search -> read -> write pattern
	searchFound := false
	readFound := false
	for _, op := range c.operations {
		if op.Type == OperationTypeSearch || op.Type == OperationTypeSymbolLookup {
			searchFound = true
		} else if op.Type == OperationTypeRead && searchFound {
			readFound = true
		} else if op.Type == OperationTypeWrite && readFound {
			return true
		}
	}

	return false
}

// TotalOperations returns the total number of operations
func (c *ContextAccumulator) TotalOperations() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.totalOperations
}

// AverageContextScore returns the average context score
func (c *ContextAccumulator) AverageContextScore() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.averageScore
}

// GetRecentOperations returns the N most recent operations
func (c *ContextAccumulator) GetRecentOperations(n int) []Operation {
	c.mu.RLock()
	defer c.mu.RUnlock()

	start := len(c.operations) - n
	if start < 0 {
		start = 0
	}

	result := make([]Operation, len(c.operations[start:]))
	copy(result, c.operations[start:])
	return result
}

// Clear resets the accumulator
func (c *ContextAccumulator) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.operations = make([]Operation, 0)
	c.symbolsExplored = make(map[string]*SymbolContext)
	c.filesRead = make(map[string]time.Time)
	c.filesModified = make(map[string][]Modification)
	c.externalReferences = nil
	c.knowledgeQueries = nil
}

// Helper function
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
