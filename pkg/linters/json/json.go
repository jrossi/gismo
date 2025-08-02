package json

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jrossi/gismo/pkg/linters"

	json "github.com/goccy/go-json"
	"github.com/kaptinlin/jsonschema"
)

// JSONLinter handles linting of JSON and JSON-L files
type JSONLinter struct {
	config *JSONConfig
	// Object pools for performance
	bufferPool  *sync.Pool
	scannerPool *sync.Pool
	// Schema cache for performance
	schemas map[string]*jsonschema.Schema
}

// NewJSONLinter creates a new JSON linter with default configuration
func NewJSONLinter() *JSONLinter {
	return NewJSONLinterWithConfig(nil)
}

// NewJSONLinterWithConfig creates a new JSON linter with custom configuration
func NewJSONLinterWithConfig(config *JSONConfig) *JSONLinter {
	if config == nil {
		config = DefaultJSONConfig()
	}

	return &JSONLinter{
		config:  config,
		schemas: make(map[string]*jsonschema.Schema),
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 4096))
			},
		},
		scannerPool: &sync.Pool{
			New: func() interface{} {
				return bufio.NewScanner(strings.NewReader(""))
			},
		},
	}
}

// Name returns the linter name
func (l *JSONLinter) Name() string {
	return "json"
}

// CanHandle returns true if this linter can handle the given file
func (l *JSONLinter) CanHandle(filePath string) bool {
	lowerPath := strings.ToLower(filePath)
	return strings.HasSuffix(lowerPath, ".json") ||
		strings.HasSuffix(lowerPath, ".jsonl") ||
		strings.HasSuffix(lowerPath, ".geojson") ||
		strings.HasSuffix(lowerPath, ".ndjson")
}

// SetConfig updates the linter configuration
func (l *JSONLinter) SetConfig(config []byte) error {
	var jsonConfig JSONConfig
	if err := json.Unmarshal(config, &jsonConfig); err != nil {
		return fmt.Errorf("failed to unmarshal json config: %w", err)
	}
	l.config = &jsonConfig
	return nil
}

// Lint performs linting on a single JSON file
func (l *JSONLinter) Lint(ctx context.Context, filePath string, content []byte) (*linters.LintResult, error) {
	result := &linters.LintResult{
		Success: true,
		Issues:  []linters.Issue{},
	}

	// Check file size limit
	if l.config.MaxFileSize != nil && int64(len(content)) > *l.config.MaxFileSize {
		result.Success = false
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("File size %d exceeds limit %d", len(content), *l.config.MaxFileSize),
			Rule:     "file-size",
		})
		return result, nil
	}

	// Detect format
	format := l.detectFormat(filePath, content)

	// Validate based on format
	switch format {
	case FormatJSON:
		if err := l.validateJSON(ctx, filePath, content, result); err != nil {
			return nil, err
		}
	case FormatJSONLines:
		if err := l.validateJSONLines(ctx, filePath, content, result); err != nil {
			return nil, err
		}
	}

	// Add formatting if requested
	if l.config.PrettyPrint != nil && *l.config.PrettyPrint {
		formatted, err := l.formatJSON(content, format)
		if err == nil {
			result.Formatted = formatted
		}
	}

	return result, nil
}

// LintBatch performs linting on multiple JSON files at once for better performance
func (l *JSONLinter) LintBatch(ctx context.Context, files map[string][]byte) (map[string]*linters.LintResult, error) {
	results := make(map[string]*linters.LintResult)
	var mu sync.Mutex

	// Filter JSON files
	jsonFiles := make(map[string][]byte)
	for path, content := range files {
		if l.CanHandle(path) {
			jsonFiles[path] = content
		}
	}

	if len(jsonFiles) == 0 {
		return results, nil
	}

	// Process files in parallel
	var wg sync.WaitGroup
	for filePath, content := range jsonFiles {
		wg.Add(1)
		go func(path string, data []byte) {
			defer wg.Done()

			result, err := l.Lint(ctx, path, data)
			if err != nil {
				result = &linters.LintResult{
					Success: false,
					Issues: []linters.Issue{
						{
							File:     path,
							Line:     1,
							Column:   1,
							Severity: "error",
							Message:  fmt.Sprintf("Linting failed: %v", err),
							Rule:     "internal",
						},
					},
				}
			}

			mu.Lock()
			results[path] = result
			mu.Unlock()
		}(filePath, content)
	}

	wg.Wait()
	return results, nil
}

// detectFormat determines if the content is JSON or JSON-L
func (l *JSONLinter) detectFormat(filePath string, content []byte) JSONFormat {
	// Check file extension first
	if strings.HasSuffix(filePath, ".jsonl") || strings.HasSuffix(filePath, ".ndjson") {
		return FormatJSONLines
	}

	// If format detection is disabled, assume JSON
	if l.config.FormatDetection == nil || !*l.config.FormatDetection {
		return FormatJSON
	}

	// Auto-detect based on content
	if l.isJSONLines(content) {
		return FormatJSONLines
	}

	return FormatJSON
}

// isJSONLines checks if content appears to be JSON Lines format
func (l *JSONLinter) isJSONLines(content []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineCount := 0
	jsonObjectCount := 0

	for scanner.Scan() && lineCount < 10 { // Check first 10 lines
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		lineCount++

		// Check if line starts with { (likely JSON object)
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			jsonObjectCount++
		}
	}

	// If most lines look like JSON objects, it's likely JSON-L
	return lineCount > 1 && jsonObjectCount > lineCount/2
}

// validateJSON validates a single JSON document
func (l *JSONLinter) validateJSON(ctx context.Context, filePath string, content []byte, result *linters.LintResult) error {
	// Skip validation if syntax check is disabled
	if l.isCheckDisabled("syntax") {
		return nil
	}

	// Fast syntax validation
	valid := json.Valid(content)
	if !valid {
		// Find the exact error location
		var data interface{}
		if err := json.Unmarshal(content, &data); err != nil {
			pos := l.findErrorPosition(content, err)
			result.Success = false
			result.Issues = append(result.Issues, linters.Issue{
				File:     filePath,
				Line:     pos.Line,
				Column:   pos.Column,
				Severity: "error",
				Message:  fmt.Sprintf("Invalid JSON syntax: %v", err),
				Rule:     "syntax",
			})
		}
		return nil
	}

	// Structure validation if enabled
	if l.config.ValidationLevel != nil && *l.config.ValidationLevel >= ValidationStructure {
		if err := l.validateJSONStructure(content, filePath, result); err != nil {
			return err
		}
	}

	return nil
}

// validateJSONLines validates JSON Lines format
func (l *JSONLinter) validateJSONLines(ctx context.Context, filePath string, content []byte, result *linters.LintResult) error {
	scanner := bufio.NewScanner(bytes.NewReader(content))

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Validate each line as JSON
		if !json.Valid([]byte(line)) {
			var data interface{}
			if err := json.Unmarshal([]byte(line), &data); err != nil {
				result.Success = false
				result.Issues = append(result.Issues, linters.Issue{
					File:     filePath,
					Line:     lineNum,
					Column:   1,
					Severity: "error",
					Message:  fmt.Sprintf("Invalid JSON syntax on line %d: %v", lineNum, err),
					Rule:     "syntax",
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading JSON-L file: %w", err)
	}

	return nil
}

// validateJSONStructure performs deeper structural validation
func (l *JSONLinter) validateJSONStructure(content []byte, filePath string, result *linters.LintResult) error {
	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		pos := l.findErrorPosition(content, err)
		result.Success = false
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     pos.Line,
			Column:   pos.Column,
			Severity: "error",
			Message:  fmt.Sprintf("JSON structure error: %v", err),
			Rule:     "structure",
		})
		return nil
	}

	// Schema validation if enabled
	if l.config.ValidationLevel != nil && *l.config.ValidationLevel >= ValidationSchema {
		if err := l.validateJSONSchema(data, filePath, result); err != nil {
			return err
		}
	}

	return nil
}

// loadSchema loads and compiles a JSON schema from either inline JSON or file path
func (l *JSONLinter) loadSchema(schemaData *json.RawMessage) (*jsonschema.Schema, error) {
	if schemaData == nil {
		return nil, fmt.Errorf("schema data is nil")
	}

	compiler := jsonschema.NewCompiler()

	// Try to detect if this is a file path (string) or inline schema (object)
	var schemaPath string
	if err := json.Unmarshal(*schemaData, &schemaPath); err == nil {
		// It's a string, treat as file path
		if !filepath.IsAbs(schemaPath) {
			// Make relative paths relative to current working directory
			wd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get working directory: %w", err)
			}
			schemaPath = filepath.Join(wd, schemaPath)
		}

		// Load schema from file
		schemaBytes, err := os.ReadFile(schemaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
		}

		return compiler.Compile(schemaBytes)
	}

	// Not a string, treat as inline schema object
	return compiler.Compile([]byte(*schemaData))
}

// validateJSONSchema validates JSON data against configured schema
func (l *JSONLinter) validateJSONSchema(data interface{}, filePath string, result *linters.LintResult) error {
	if l.config == nil || l.config.JSONSchema == nil {
		return nil
	}

	schema, err := l.loadSchema(l.config.JSONSchema)
	if err != nil {
		result.Success = false
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("Failed to load JSON schema: %v", err),
			Rule:     "schema",
		})
		return nil
	}

	if err := schema.Validate(data); err != nil {
		result.Success = false
		result.Issues = append(result.Issues, linters.Issue{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("JSON schema validation failed: %v", err),
			Rule:     "schema",
		})
	}

	return nil
}

// formatJSON formats JSON content for pretty printing
func (l *JSONLinter) formatJSON(content []byte, format JSONFormat) ([]byte, error) {
	switch format {
	case FormatJSON:
		var data interface{}
		if err := json.Unmarshal(content, &data); err != nil {
			return nil, err
		}
		return json.MarshalIndent(data, "", "  ")

	case FormatJSONLines:
		var result bytes.Buffer
		scanner := bufio.NewScanner(bytes.NewReader(content))

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var data interface{}
			if err := json.Unmarshal([]byte(line), &data); err != nil {
				continue // Skip invalid lines
			}

			formatted, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				continue
			}

			result.Write(formatted)
			result.WriteByte('\n')
		}

		return result.Bytes(), nil
	}

	return content, nil
}

// ErrorPosition represents a position in the file for error reporting
type ErrorPosition struct {
	Line   int
	Column int
}

// findErrorPosition finds the line and column of a JSON error
func (l *JSONLinter) findErrorPosition(content []byte, err error) ErrorPosition {
	// For now, return a basic position
	// In a production implementation, this would parse the error message
	// to extract precise line/column information
	return ErrorPosition{Line: 1, Column: 1}
}

// isCheckDisabled checks if a specific check is disabled
func (l *JSONLinter) isCheckDisabled(checkName string) bool {
	for _, disabled := range l.config.DisabledChecks {
		if disabled == checkName {
			return true
		}
	}
	return false
}
