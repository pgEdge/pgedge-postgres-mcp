/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"pgedge-mcp/internal/mcp"
)

// Handler is a function that executes a tool
type Handler func(args map[string]interface{}) (mcp.ToolResponse, error)

// Tool represents a registered MCP tool
type Tool struct {
	Definition mcp.Tool
	Handler    Handler
}

// Registry manages available MCP tools
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(name string, tool Tool) {
	r.tools[name] = tool
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tool definitions
func (r *Registry) List() []mcp.Tool {
	tools := make([]mcp.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool.Definition)
	}
	return tools
}

// Execute runs a tool by name with the given arguments
func (r *Registry) Execute(name string, args map[string]interface{}) (mcp.ToolResponse, error) {
	tool, exists := r.Get(name)
	if !exists {
		return mcp.ToolResponse{
			Content: []mcp.ContentItem{
				{
					Type: "text",
					Text: "Tool not found: " + name,
				},
			},
			IsError: true,
		}, nil
	}

	return tool.Handler(args)
}
