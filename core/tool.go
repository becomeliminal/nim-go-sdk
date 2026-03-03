package core

import (
	"bytes"
	"context"
	"encoding/json"
	"text/template"
)

// Tool is the interface for all tools available to agents.
type Tool interface {
	// Name returns the tool's unique identifier.
	Name() string

	// Description returns a human-readable description for Claude.
	Description() string

	// Schema returns the JSON Schema for the tool's parameters.
	Schema() map[string]interface{}

	// RequiresConfirmation returns true if this tool needs user approval.
	RequiresConfirmation() bool

	// Execute runs the tool with the given parameters.
	Execute(ctx context.Context, params *ToolParams) (*ToolResult, error)

	// GetSummary returns a human-readable summary of the action.
	GetSummary(input json.RawMessage) string
}

// ToolParams contains all parameters needed for tool execution.
type ToolParams struct {
	// UserID is the authenticated user making the request.
	UserID string

	// Input is the tool parameters as JSON.
	Input json.RawMessage

	// ConfirmationID is set for confirmed write operations.
	ConfirmationID string

	// RequestID for tracing/logging.
	RequestID string

	// ConversationID is the conversation this tool call belongs to.
	ConversationID string

	// MessageID is the user message that triggered this agent turn.
	MessageID string
}

// ToolResult contains the result of a tool execution.
type ToolResult struct {
	// Success indicates whether the tool executed successfully.
	Success bool `json:"success"`

	// Data is the result payload to send back to Claude.
	Data interface{} `json:"data,omitempty"`

	// Error is set on failure.
	Error string `json:"error,omitempty"`

	// Metadata contains additional info (e.g., transaction hash).
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolDefinition contains static tool metadata.
type ToolDefinition struct {
	// Name is the tool's unique identifier.
	ToolName string

	// Description is the human-readable description.
	ToolDescription string

	// RequiresUserConfirmation indicates if user approval is needed.
	RequiresUserConfirmation bool

	// SummaryTemplate is a Go template for generating summaries.
	SummaryTemplate string

	// InputSchema is the JSON Schema for parameters.
	InputSchema map[string]interface{}
}

// BaseTool provides common tool functionality.
type BaseTool struct {
	definition ToolDefinition
	handler    ToolHandler
}

// ToolHandler is a function that executes a tool.
type ToolHandler func(ctx context.Context, params *ToolParams) (*ToolResult, error)

// NewBaseTool creates a BaseTool from a definition and handler.
func NewBaseTool(def ToolDefinition, handler ToolHandler) *BaseTool {
	return &BaseTool{
		definition: def,
		handler:    handler,
	}
}

// Name returns the tool's name.
func (t *BaseTool) Name() string {
	return t.definition.ToolName
}

// Description returns the tool's description.
func (t *BaseTool) Description() string {
	return t.definition.ToolDescription
}

// Schema returns the tool's input schema.
func (t *BaseTool) Schema() map[string]interface{} {
	return t.definition.InputSchema
}

// RequiresConfirmation returns whether the tool needs confirmation.
func (t *BaseTool) RequiresConfirmation() bool {
	return t.definition.RequiresUserConfirmation
}

// Execute runs the tool handler.
func (t *BaseTool) Execute(ctx context.Context, params *ToolParams) (*ToolResult, error) {
	if t.handler == nil {
		return &ToolResult{Success: false, Error: "no handler configured"}, nil
	}
	return t.handler(ctx, params)
}

// GetSummary returns a formatted summary using the template.
func (t *BaseTool) GetSummary(input json.RawMessage) string {
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

// Definition returns the underlying ToolDefinition.
func (t *BaseTool) Definition() ToolDefinition {
	return t.definition
}
