package engine

import json "github.com/goccy/go-json"

// ToolInput represents the base interface for all tool inputs
type ToolInput interface {
	ToolName() string
}

// ToolOutput represents the base interface for all tool outputs
type ToolOutput interface {
	IsError() bool
}

// WriteToolInput represents input for the Write tool
type WriteToolInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (w WriteToolInput) ToolName() string { return "Write" }

// EditToolInput represents input for the Edit tool
type EditToolInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func (e EditToolInput) ToolName() string { return "Edit" }

// MultiEditToolInput represents input for the MultiEdit tool
type MultiEditToolInput struct {
	FilePath string          `json:"file_path"`
	Edits    []EditOperation `json:"edits"`
}

// EditOperation represents a single edit operation in MultiEdit
type EditOperation struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func (m MultiEditToolInput) ToolName() string { return "MultiEdit" }

// BashToolInput represents input for the Bash tool
type BashToolInput struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	Timeout     int    `json:"timeout,omitempty"`
}

func (b BashToolInput) ToolName() string { return "Bash" }

// ReadToolInput represents input for the Read tool
type ReadToolInput struct {
	FilePath string `json:"file_path"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

func (r ReadToolInput) ToolName() string { return "Read" }

// GenericToolInput represents input for tools we don't have specific types for
type GenericToolInput struct {
	Name       string
	Parameters map[string]json.RawMessage
}

func (g GenericToolInput) ToolName() string { return g.Name }

// StringOutput represents a simple string output from a tool
type StringOutput struct {
	Content string
	Error   bool
}

func (s StringOutput) IsError() bool { return s.Error }

// ParseToolInput attempts to parse tool input into a concrete type
func ParseToolInput(toolName string, data map[string]json.RawMessage) (ToolInput, error) {
	switch toolName {
	case "Write":
		var input WriteToolInput
		if err := unmarshalToolInput(data, &input); err != nil {
			return nil, err
		}
		return input, nil
	case "Edit":
		var input EditToolInput
		if err := unmarshalToolInput(data, &input); err != nil {
			return nil, err
		}
		return input, nil
	case "MultiEdit":
		var input MultiEditToolInput
		if err := unmarshalToolInput(data, &input); err != nil {
			return nil, err
		}
		return input, nil
	case "Bash":
		var input BashToolInput
		if err := unmarshalToolInput(data, &input); err != nil {
			return nil, err
		}
		return input, nil
	case "Read":
		var input ReadToolInput
		if err := unmarshalToolInput(data, &input); err != nil {
			return nil, err
		}
		return input, nil
	default:
		// For unknown tools, use generic input
		return GenericToolInput{
			Name:       toolName,
			Parameters: data,
		}, nil
	}
}

// unmarshalToolInput is a helper to unmarshal from map[string]json.RawMessage
func unmarshalToolInput(data map[string]json.RawMessage, target interface{}) error {
	// Convert map back to JSON then unmarshal to target
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, target)
}
