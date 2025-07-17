package daemon

import (
	"encoding/json"
	"time"
)

// Request represents a request from client to daemon
type Request struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Response represents a response from daemon to client
type Response struct {
	ID       string          `json:"id"`
	Success  bool            `json:"success"`
	Error    string          `json:"error,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Duration time.Duration   `json:"duration"`
}

// HookRequest wraps a hook message for daemon processing
type HookRequest struct {
	StdinData []byte            `json:"stdin_data"`
	Env       map[string]string `json:"env,omitempty"`
}

// HookResponse contains the result of hook processing
type HookResponse struct {
	Stdout   []byte `json:"stdout"`
	Stderr   []byte `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// PingRequest is used to check daemon health
type PingRequest struct{}

// PingResponse contains daemon status information
type PingResponse struct {
	Version   string        `json:"version"`
	Uptime    time.Duration `json:"uptime"`
	Processed int64         `json:"processed"`
}
