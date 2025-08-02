package json

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	linters2 "github.com/jrossi/gismo/pkg/linters"
)

func TestNewJSONLinter(t *testing.T) {
	linter := NewJSONLinter()
	if linter == nil {
		t.Fatal("NewJSONLinter returned nil")
	}
	if linter.Name() != "json" {
		t.Errorf("Expected name 'json', got %s", linter.Name())
	}
}

func TestJSONLinter_Name(t *testing.T) {
	linter := NewJSONLinter()
	if linter.Name() != "json" {
		t.Errorf("Expected name 'json', got %s", linter.Name())
	}
}

func TestJSONLinter_CanHandle(t *testing.T) {
	linter := NewJSONLinter()
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{"JSON file", "test.json", true},
		{"JSON Lines file", "test.jsonl", true},
		{"JSON in directory", "path/to/test.json", true},
		{"Go file", "test.go", false},
		{"Python file", "test.py", false},
		{"Text file", "test.txt", false},
		{"No extension", "test", false},
		{"Hidden JSON file", ".test.json", true},
		{"JSON with uppercase", "TEST.JSON", true},
		{"JSONL with uppercase", "TEST.JSONL", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linter.CanHandle(tt.filePath)
			if got != tt.want {
				t.Errorf("CanHandle(%s) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestJSONLinter_SetConfig(t *testing.T) {
	linter := NewJSONLinter()

	// Test valid config
	config := DefaultJSONConfig()
	maxSize := int64(2048)
	config.MaxFileSize = &maxSize

	configBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = linter.SetConfig(configBytes)
	if err != nil {
		t.Errorf("SetConfig failed: %v", err)
	}

	// Test invalid config
	invalidConfig := []byte(`{"invalid": "json"`)
	err = linter.SetConfig(invalidConfig)
	if err == nil {
		t.Error("SetConfig should have failed with invalid JSON")
	}
}

func TestJSONLinter_Lint_ValidJSON(t *testing.T) {
	linter := NewJSONLinter()
	ctx := context.Background()

	validJSON := `{
		"name": "test",
		"age": 25,
		"active": true,
		"tags": ["json", "test"],
		"nested": {
			"key": "value"
		}
	}`

	result, err := linter.Lint(ctx, "test.json", []byte(validJSON))
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success=true, got %v", result.Success)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Expected no issues, got %d", len(result.Issues))
	}
}

func TestJSONLinter_Lint_InvalidJSON(t *testing.T) {
	linter := NewJSONLinter()
	ctx := context.Background()

	invalidJSON := `{
		"name": "test",
		"age": 25,
		"active": true,
		"tags": ["json", "test"],
		"nested": {
			"key": "value",
		}
	}`

	result, err := linter.Lint(ctx, "test.json", []byte(invalidJSON))
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	t.Logf("Result: Success=%v, Issues=%d", result.Success, len(result.Issues))
	for i, issue := range result.Issues {
		t.Logf("Issue %d: %+v", i, issue)
	}

	if result.Success {
		t.Error("Expected success=false for invalid JSON")
	}

	if len(result.Issues) == 0 {
		t.Error("Expected at least one issue for invalid JSON")
		return
	}

	// Check that the issue has correct properties
	issue := result.Issues[0]
	if issue.Severity != "error" {
		t.Errorf("Expected severity %s, got %s", "error", issue.Severity)
	}
	if issue.Rule != "syntax" {
		t.Errorf("Expected rule 'syntax', got %s", issue.Rule)
	}
}

func TestJSONLinter_Lint_ValidJSONLines(t *testing.T) {
	linter := NewJSONLinter()
	ctx := context.Background()

	validJSONL := `{"name": "user1", "id": 1}
{"name": "user2", "id": 2}
{"name": "user3", "id": 3}`

	result, err := linter.Lint(ctx, "test.jsonl", []byte(validJSONL))
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success=true, got %v", result.Success)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Expected no issues, got %d", len(result.Issues))
	}
}

func TestJSONLinter_Lint_InvalidJSONLines(t *testing.T) {
	linter := NewJSONLinter()
	ctx := context.Background()

	invalidJSONL := `{"name": "user1", "id": 1}
{"name": "user2", "id": 2,}
{"name": "user3", "id": 3}`

	result, err := linter.Lint(ctx, "test.jsonl", []byte(invalidJSONL))
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	if result.Success {
		t.Error("Expected success=false for invalid JSON Lines")
	}

	if len(result.Issues) == 0 {
		t.Error("Expected at least one issue for invalid JSON Lines")
		return
	}

	// Check that the issue indicates the correct line
	issue := result.Issues[0]
	if issue.Line != 2 {
		t.Errorf("Expected line 2, got %d", issue.Line)
	}
}

func TestJSONLinter_Lint_SizeLimit(t *testing.T) {
	linter := NewJSONLinter()
	ctx := context.Background()

	// Set a small size limit
	config := DefaultJSONConfig()
	maxSize := int64(10) // 10 bytes
	config.MaxFileSize = &maxSize

	configBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = linter.SetConfig(configBytes)
	if err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	// Test with content exceeding limit
	largeJSON := `{"name": "this is a large JSON object that exceeds the size limit"}`

	result, err := linter.Lint(ctx, "test.json", []byte(largeJSON))
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	if result.Success {
		t.Error("Expected success=false for oversized file")
	}

	if len(result.Issues) == 0 {
		t.Error("Expected at least one issue for oversized file")
	}

	// Check that the issue is about file size
	issue := result.Issues[0]
	if issue.Rule != "file-size" {
		t.Errorf("Expected rule 'file-size', got %s", issue.Rule)
	}
}

func TestJSONLinter_Lint_PrettyPrint(t *testing.T) {
	linter := NewJSONLinter()
	ctx := context.Background()

	// Enable pretty printing
	config := DefaultJSONConfig()
	config.PrettyPrint = &[]bool{true}[0]

	configBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	err = linter.SetConfig(configBytes)
	if err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	// Test with minified JSON
	minifiedJSON := `{"name":"test","data":{"numbers":[1,2,3]}}`

	result, err := linter.Lint(ctx, "test.json", []byte(minifiedJSON))
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success=true, got %v", result.Success)
	}

	if len(result.Formatted) == 0 {
		t.Error("Expected formatted output")
	}

	// Check that formatted output is different from input
	if string(result.Formatted) == minifiedJSON {
		t.Error("Formatted output should be different from minified input")
	}
}

func TestJSONLinter_LintBatch(t *testing.T) {
	linter := NewJSONLinter()
	ctx := context.Background()

	files := map[string][]byte{
		"valid.json":   []byte(`{"name": "test", "valid": true}`),
		"invalid.json": []byte(`{"name": "test", "valid": true,}`),
		"test.jsonl":   []byte("{\"id\": 1}\n{\"id\": 2}"),
	}

	results, err := linter.LintBatch(ctx, files)
	if err != nil {
		t.Fatalf("LintBatch failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Check results
	validResult := results["valid.json"]
	if !validResult.Success {
		t.Error("Expected valid.json to pass")
	}

	invalidResult := results["invalid.json"]
	if invalidResult.Success {
		t.Error("Expected invalid.json to fail")
	}

	jsonlResult := results["test.jsonl"]
	if !jsonlResult.Success {
		t.Errorf("Expected test.jsonl to pass, but got %d issues:", len(jsonlResult.Issues))
		for i, issue := range jsonlResult.Issues {
			t.Errorf("  Issue %d: %+v", i, issue)
		}
	}
}

func TestJSONLinter_detectFormat(t *testing.T) {
	linter := NewJSONLinter()

	tests := []struct {
		name     string
		filePath string
		content  []byte
		want     JSONFormat
	}{
		{
			name:     "JSON file extension",
			filePath: "test.json",
			content:  []byte(`{"test": "json"}`),
			want:     FormatJSON,
		},
		{
			name:     "JSONL file extension",
			filePath: "test.jsonl",
			content:  []byte(`{"test": "json"}`),
			want:     FormatJSONLines,
		},
		{
			name:     "JSON content detection",
			filePath: "test.txt",
			content:  []byte(`{"test": "json"}`),
			want:     FormatJSON,
		},
		{
			name:     "JSONL content detection",
			filePath: "test.txt",
			content:  []byte("{\"line\": 1}\n{\"line\": 2}"),
			want:     FormatJSONLines,
		},
		{
			name:     "Empty content",
			filePath: "test.json",
			content:  []byte(``),
			want:     FormatJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linter.detectFormat(tt.filePath, tt.content)
			if got != tt.want {
				t.Errorf("detectFormat(%s, %s) = %v, want %v", tt.filePath, string(tt.content), got, tt.want)
			}
		})
	}
}

func TestJSONLinter_EmptyFile(t *testing.T) {
	linter := NewJSONLinter()
	ctx := context.Background()

	result, err := linter.Lint(ctx, "empty.json", []byte(""))
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	if result.Success {
		t.Error("Expected success=false for empty file")
	}

	if len(result.Issues) == 0 {
		t.Error("Expected at least one issue for empty file")
	}
}

func TestJSONLinter_ContextCancellation(t *testing.T) {
	linter := NewJSONLinter()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	validJSON := `{"test": "json"}`

	result, err := linter.Lint(ctx, "test.json", []byte(validJSON))
	if err != nil {
		t.Fatalf("Lint failed: %v", err)
	}

	// Should still work for simple cases even with canceled context
	// since JSON parsing is very fast
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestDefaultJSONConfig(t *testing.T) {
	config := DefaultJSONConfig()

	if config.MaxFileSize == nil {
		t.Error("Expected MaxFileSize to be set")
	}

	if *config.MaxFileSize != 1024*1024 {
		t.Errorf("Expected MaxFileSize %d, got %d", 1024*1024, *config.MaxFileSize)
	}

	if config.ValidationLevel == nil {
		t.Error("Expected ValidationLevel to be set")
	}

	if *config.ValidationLevel != ValidationSyntax {
		t.Errorf("Expected ValidationLevel %s, got %s", ValidationSyntax, *config.ValidationLevel)
	}
}

func TestJSONLinter_InterfaceCompliance(t *testing.T) {
	linter := NewJSONLinter()

	// Test that it implements the required interfaces
	var _ linters2.Linter = linter
	var _ linters2.BatchingLinter = linter
}

func TestJSONLinter_SchemaValidation(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		schema       string
		expectError  bool
		expectedRule string
	}{
		{
			name: "valid_inline_schema",
			content: `{
				"name": "John Doe",
				"age": 30,
				"email": "john@example.com"
			}`,
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "number"},
					"email": {"type": "string", "format": "email"}
				},
				"required": ["name", "age"]
			}`,
			expectError: false,
		},
		{
			name: "invalid_json_missing_required",
			content: `{
				"name": "John Doe",
				"email": "john@example.com"
			}`,
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "number"},
					"email": {"type": "string"}
				},
				"required": ["name", "age"]
			}`,
			expectError:  true,
			expectedRule: "schema",
		},
		{
			name: "invalid_json_wrong_type",
			content: `{
				"name": "John Doe",
				"age": "thirty",
				"email": "john@example.com"
			}`,
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "number"},
					"email": {"type": "string"}
				},
				"required": ["name", "age"]
			}`,
			expectError:  true,
			expectedRule: "schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with inline schema
			schemaJSON := json.RawMessage(tt.schema)
			validationLevel := ValidationSchema
			config := &JSONConfig{
				ValidationLevel: &validationLevel,
				JSONSchema:      &schemaJSON,
			}

			linter := NewJSONLinterWithConfig(config)
			result, err := linter.Lint(context.Background(), "test.json", []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			hasSchemaError := false
			for _, issue := range result.Issues {
				if issue.Rule == tt.expectedRule && issue.Severity == "error" {
					hasSchemaError = true
					break
				}
			}

			if tt.expectError && !hasSchemaError {
				t.Errorf("Expected schema validation error, but got none. Issues: %+v", result.Issues)
			}
			if !tt.expectError && hasSchemaError {
				t.Errorf("Expected no schema validation error, but got one")
			}
		})
	}
}

func TestJSONLinter_SchemaFile(t *testing.T) {
	// Create a temporary schema file
	schemaContent := `{
		"type": "object",
		"properties": {
			"id": {"type": "integer"},
			"name": {"type": "string"},
			"active": {"type": "boolean"}
		},
		"required": ["id", "name"]
	}`

	tmpFile, err := os.CreateTemp("", "schema-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp schema file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(schemaContent); err != nil {
		t.Fatalf("Failed to write schema file: %v", err)
	}
	tmpFile.Close()

	tests := []struct {
		name         string
		content      string
		expectError  bool
		expectedRule string
	}{
		{
			name: "valid_json_with_file_schema",
			content: `{
				"id": 123,
				"name": "Test Item",
				"active": true
			}`,
			expectError: false,
		},
		{
			name: "invalid_json_missing_required",
			content: `{
				"id": 123,
				"active": true
			}`,
			expectError:  true,
			expectedRule: "schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with file path
			schemaPath := json.RawMessage(`"` + tmpFile.Name() + `"`)
			validationLevel := ValidationSchema
			config := &JSONConfig{
				ValidationLevel: &validationLevel,
				JSONSchema:      &schemaPath,
			}

			linter := NewJSONLinterWithConfig(config)
			result, err := linter.Lint(context.Background(), "test.json", []byte(tt.content))
			if err != nil {
				t.Fatalf("Lint() error = %v", err)
			}

			hasSchemaError := false
			for _, issue := range result.Issues {
				if issue.Rule == tt.expectedRule && issue.Severity == "error" {
					hasSchemaError = true
					break
				}
			}

			if tt.expectError && !hasSchemaError {
				t.Errorf("Expected schema validation error, but got none. Issues: %+v", result.Issues)
			}
			if !tt.expectError && hasSchemaError {
				t.Errorf("Expected no schema validation error, but got one")
			}
		})
	}
}
