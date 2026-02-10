package core

import (
	"bytes"
	"context"
	"encoding/json"
	"text/template"
)

// ToolExecutor executes Liminal tools (get_balance, send_money, etc.).
// This is the key abstraction that enables different implementations:
//   - HTTPExecutor (public SDK) → calls agent_gateway over HTTP
//   - GRPCExecutor (internal) → calls services directly via gRPC
type ToolExecutor interface {
	// Execute runs a read-only tool and returns the result.
	Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error)

	// ExecuteWrite runs a write tool that may require confirmation.
	// Returns RequiresConfirmation=true if user approval is needed.
	ExecuteWrite(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error)

	// Confirm executes a previously confirmed write operation.
	Confirm(ctx context.Context, userID, confirmationID string) (*ExecuteResponse, error)

	// Cancel cancels a pending confirmation.
	Cancel(ctx context.Context, userID, confirmationID string) error
}

// PendingStore is an optional interface for executors that need to cache write
// requests locally. The HTTPExecutor implements this because it needs the tool
// name and input when Confirm() is called (which only receives a confirmation ID).
// The GRPCExecutor does not need this because it uses Redis for the same purpose.
type PendingStore interface {
	StorePending(confirmationID string, req *ExecuteRequest)
}

// ExecuteRequest contains the parameters for tool execution.
type ExecuteRequest struct {
	// UserID is the authenticated user making the request.
	UserID string `json:"user_id"`

	// Tool is the name of the tool to execute.
	Tool string `json:"tool"`

	// Input is the tool parameters as JSON.
	Input json.RawMessage `json:"input"`

	// RequestID for tracing/logging.
	RequestID string `json:"request_id,omitempty"`
}

// ExecuteResponse contains the result of tool execution.
type ExecuteResponse struct {
	// Success indicates whether the execution succeeded.
	Success bool `json:"success"`

	// Data is the result payload.
	Data json.RawMessage `json:"data,omitempty"`

	// Error is set on failure.
	Error string `json:"error,omitempty"`

	// RequiresConfirmation is true for write operations that need user approval.
	RequiresConfirmation bool `json:"requires_confirmation,omitempty"`

	// Confirmation contains details when RequiresConfirmation is true.
	Confirmation *ConfirmationDetails `json:"confirmation,omitempty"`
}

// ConfirmationDetails contains information about a pending confirmation.
type ConfirmationDetails struct {
	// ID is the unique confirmation identifier.
	ID string `json:"id"`

	// Summary is a human-readable description of the action.
	Summary string `json:"summary"`

	// ExpiresAt is when this confirmation expires (unix timestamp).
	ExpiresAt int64 `json:"expires_at"`
}

// ExecutorTool wraps a ToolExecutor to implement the Tool interface.
// This allows Liminal tools to be used with the SDK's engine.
type ExecutorTool struct {
	definition ToolDefinition
	executor   ToolExecutor
}

// NewExecutorTool creates a tool that delegates to a ToolExecutor.
func NewExecutorTool(def ToolDefinition, executor ToolExecutor) *ExecutorTool {
	return &ExecutorTool{
		definition: def,
		executor:   executor,
	}
}

// Name returns the tool's name.
func (t *ExecutorTool) Name() string {
	return t.definition.ToolName
}

// Description returns the tool's description.
func (t *ExecutorTool) Description() string {
	return t.definition.ToolDescription
}

// Schema returns the tool's input schema.
func (t *ExecutorTool) Schema() map[string]interface{} {
	return t.definition.InputSchema
}

// RequiresConfirmation returns whether the tool needs confirmation.
func (t *ExecutorTool) RequiresConfirmation() bool {
	return t.definition.RequiresUserConfirmation
}

// Execute runs the tool via the ToolExecutor.
func (t *ExecutorTool) Execute(ctx context.Context, params *ToolParams) (*ToolResult, error) {
	req := &ExecuteRequest{
		UserID:    params.UserID,
		Tool:      t.definition.ToolName,
		Input:     params.Input,
		RequestID: params.RequestID,
	}

	var resp *ExecuteResponse
	var err error

	if t.definition.RequiresUserConfirmation && params.ConfirmationID != "" {
		// This is a confirmed write operation.
		// If the executor supports pending storage (e.g., HTTPExecutor), store the
		// request first so Confirm() can retrieve the tool name and input.
		if ps, ok := t.executor.(PendingStore); ok {
			ps.StorePending(params.ConfirmationID, req)
		}
		resp, err = t.executor.Confirm(ctx, params.UserID, params.ConfirmationID)
	} else if t.definition.RequiresUserConfirmation {
		// This is a write operation that needs confirmation
		resp, err = t.executor.ExecuteWrite(ctx, req)
	} else {
		// This is a read operation
		resp, err = t.executor.Execute(ctx, req)
	}

	if err != nil {
		return &ToolResult{Success: false, Error: err.Error()}, nil
	}

	var data interface{}
	if len(resp.Data) > 0 {
		json.Unmarshal(resp.Data, &data)
	}

	return &ToolResult{
		Success: resp.Success,
		Data:    data,
		Error:   resp.Error,
	}, nil
}

// GetSummary returns a formatted summary using the template.
func (t *ExecutorTool) GetSummary(input json.RawMessage) string {
	// If no template, return empty string
	if t.definition.SummaryTemplate == "" {
		return ""
	}

	// Parse input JSON into a map for templating
	var data map[string]interface{}
	if err := json.Unmarshal(input, &data); err != nil {
		// If parsing fails, return the template as-is
		return t.definition.SummaryTemplate
	}

	// Create and execute template
	tmpl, err := template.New("summary").Parse(t.definition.SummaryTemplate)
	if err != nil {
		// If template parsing fails, return the template as-is
		return t.definition.SummaryTemplate
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// If template execution fails, return the template as-is
		return t.definition.SummaryTemplate
	}

	return buf.String()
}
