package engine

import (
	"bytes"
	"fmt"
	"io"

	json "github.com/goccy/go-json"
)

// Parser handles high-performance JSON parsing of hook messages
type Parser struct {
}

// NewParser creates a new parser instance
func NewParser() *Parser {
	return &Parser{}
}

// ParseHookMessage parses a generic hook message to determine its type
func (p *Parser) ParseHookMessage(data []byte) (HookMessage, error) {
	// First, parse just the base message to get the event type
	var base BaseHookMessage
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("failed to parse base message: %w", err)
	}

	// Parse the specific message type based on the event
	switch base.HookEventName {
	case PreToolUseEvent:
		var msg PreToolUseMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse PreToolUse message: %w", err)
		}
		return &msg, nil

	case PostToolUseEvent:
		var msg PostToolUseMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse PostToolUse message: %w", err)
		}
		return &msg, nil

	case NotificationEvent:
		var msg NotificationMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse Notification message: %w", err)
		}
		return &msg, nil

	case StopEvent:
		var msg StopMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse Stop message: %w", err)
		}
		return &msg, nil

	case SubagentStopEvent:
		var msg SubagentStopMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse SubagentStop message: %w", err)
		}
		return &msg, nil

	case PreCompactEvent:
		var msg PreCompactMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse PreCompact message: %w", err)
		}
		return &msg, nil

	default:
		return nil, fmt.Errorf("unknown hook event type: %s", base.HookEventName)
	}
}

// ParseHookResponse parses a hook response message
func (p *Parser) ParseHookResponse(data []byte) (*HookResponse, error) {
	var response HookResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse hook response: %w", err)
	}
	return &response, nil
}

// StreamParser handles streaming JSON lines
type StreamParser struct {
	decoder *json.Decoder
}

// NewStreamParser creates a parser for streaming JSON lines
func NewStreamParser(reader io.Reader) *StreamParser {
	return &StreamParser{
		decoder: json.NewDecoder(reader),
	}
}

// ParseNext parses the next message from the stream
func (sp *StreamParser) ParseNext() (HookMessage, error) {
	// Read raw message first to determine type
	var raw json.RawMessage
	if err := sp.decoder.Decode(&raw); err != nil {
		return nil, err
	}

	// Use regular parser to handle type detection
	parser := NewParser()
	return parser.ParseHookMessage(raw)
}

// ParseMultiple parses multiple JSON objects from a byte slice
// Useful for processing hook output that contains multiple JSON objects
func (p *Parser) ParseMultiple(data []byte) ([]HookMessage, error) {
	var messages []HookMessage
	decoder := json.NewDecoder(bytes.NewReader(data))

	for decoder.More() {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return messages, fmt.Errorf("failed to decode message: %w", err)
		}

		msg, err := p.ParseHookMessage(raw)
		if err != nil {
			return messages, fmt.Errorf("failed to parse message: %w", err)
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// MarshalHookResponse serializes a hook response to JSON
func (p *Parser) MarshalHookResponse(response *HookResponse) ([]byte, error) {
	data, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	// Add newline to match encoding/json behavior
	return append(data, '\n'), nil
}

// MarshalHookMessage serializes any hook message to JSON
func (p *Parser) MarshalHookMessage(message HookMessage) ([]byte, error) {
	data, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	// Add newline to match encoding/json behavior
	return append(data, '\n'), nil
}
