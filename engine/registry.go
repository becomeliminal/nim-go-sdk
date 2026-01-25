// Package engine provides the agent execution loop.
package engine

import (
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/becomeliminal/nim-go-sdk/core"
)

// ToolRegistry manages available tools for an agent.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]core.Tool
}

// NewToolRegistry creates a new tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]core.Tool),
	}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool core.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// RegisterAll adds multiple tools to the registry.
func (r *ToolRegistry) RegisterAll(tools ...core.Tool) {
	for _, tool := range tools {
		r.Register(tool)
	}
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) (core.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tool names.
func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ToAPITools converts registered tools to Claude API format.
func (r *ToolRegistry) ToAPITools() []anthropic.ToolUnionParam {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]anthropic.ToolUnionParam, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name(),
				Description: anthropic.String(tool.Description()),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: tool.Schema(),
				},
			},
		})
	}
	return tools
}

// ToAPIToolsFiltered returns tools matching the filter.
func (r *ToolRegistry) ToAPIToolsFiltered(filter func(core.Tool) bool) []anthropic.ToolUnionParam {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []anthropic.ToolUnionParam
	for _, tool := range r.tools {
		if filter(tool) {
			tools = append(tools, anthropic.ToolUnionParam{
				OfTool: &anthropic.ToolParam{
					Name:        tool.Name(),
					Description: anthropic.String(tool.Description()),
					InputSchema: anthropic.ToolInputSchemaParam{
						Properties: tool.Schema(),
					},
				},
			})
		}
	}
	return tools
}

// FilterByNames returns a filter that matches tools by name.
func FilterByNames(names ...string) func(core.Tool) bool {
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}
	return func(t core.Tool) bool {
		return nameSet[t.Name()]
	}
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
