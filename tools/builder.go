// Package tools provides utilities for creating and configuring tools.
package tools

import (
	"context"
	"encoding/json"

	"github.com/becomeliminal/nim-go-sdk/core"
)

// Builder provides a fluent interface for creating tools.
type Builder struct {
	name                 string
	description          string
	schema               map[string]interface{}
	requiresConfirmation bool
	summaryTemplate      string
	handler              core.ToolHandler
}

// New creates a new tool builder.
func New(name string) *Builder {
	return &Builder{
		name:   name,
		schema: make(map[string]interface{}),
	}
}

// Description sets the tool description.
func (b *Builder) Description(desc string) *Builder {
	b.description = desc
	return b
}

// Schema sets the JSON Schema for the tool parameters.
func (b *Builder) Schema(schema map[string]interface{}) *Builder {
	b.schema = schema
	return b
}

// RequiresConfirmation marks this tool as requiring user confirmation.
// Automatically makes thought parameter required for safety.
func (b *Builder) RequiresConfirmation() *Builder {
	b.requiresConfirmation = true
	// Auto-enable required thought for write operations
	if b.schema != nil {
		b.schema = WithThought(b.schema, true)
	}
	return b
}

// SummaryTemplate sets the template for generating action summaries.
func (b *Builder) SummaryTemplate(template string) *Builder {
	b.summaryTemplate = template
	return b
}

// Handler sets the execution handler for the tool.
func (b *Builder) Handler(h core.ToolHandler) *Builder {
	b.handler = h
	return b
}

// HandlerFunc sets a simple handler function.
func (b *Builder) HandlerFunc(fn func(ctx context.Context, input json.RawMessage) (interface{}, error)) *Builder {
	b.handler = func(ctx context.Context, params *core.ToolParams) (*core.ToolResult, error) {
		result, err := fn(ctx, params.Input)
		if err != nil {
			return &core.ToolResult{Success: false, Error: err.Error()}, nil
		}
		return &core.ToolResult{Success: true, Data: result}, nil
	}
	return b
}

// Build creates the tool.
func (b *Builder) Build() core.Tool {
	return core.NewBaseTool(core.ToolDefinition{
		ToolName:                 b.name,
		ToolDescription:          b.description,
		RequiresUserConfirmation: b.requiresConfirmation,
		SummaryTemplate:          b.summaryTemplate,
		InputSchema:              b.schema,
	}, b.handler)
}

// Config provides a declarative way to create a tool.
type Config struct {
	Name                 string
	Description          string
	Schema               map[string]interface{}
	RequiresConfirmation bool
	SummaryTemplate      string
	Handler              func(ctx context.Context, input json.RawMessage) (interface{}, error)
}

// FromConfig creates a tool from a Config struct.
func FromConfig(cfg Config) core.Tool {
	handler := func(ctx context.Context, params *core.ToolParams) (*core.ToolResult, error) {
		result, err := cfg.Handler(ctx, params.Input)
		if err != nil {
			return &core.ToolResult{Success: false, Error: err.Error()}, nil
		}
		return &core.ToolResult{Success: true, Data: result}, nil
	}

	return core.NewBaseTool(core.ToolDefinition{
		ToolName:                 cfg.Name,
		ToolDescription:          cfg.Description,
		RequiresUserConfirmation: cfg.RequiresConfirmation,
		SummaryTemplate:          cfg.SummaryTemplate,
		InputSchema:              cfg.Schema,
	}, handler)
}
