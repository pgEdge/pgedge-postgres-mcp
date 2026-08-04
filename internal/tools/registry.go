/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"sort"

	"pgedge-postgres-mcp/internal/mcp"
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

// List returns all registered tool definitions, sorted by name for a
// deterministic order across calls. tools/list feeds the front of the
// prompt sent to the model, so a reshuffled list invalidates the
// provider-side prompt cache and defeats client-side diffing of the
// advertised set, even when it has not actually changed.
//
// The registration key breaks ties, since it is unique by construction
// (it is a map key) even in the case, not exercised by any caller
// today, of two entries advertising the same Definition.Name under
// different keys: sort.Slice gives no ordering guarantee for elements
// that compare equal, so relying on Name alone would leave those two
// entries' relative order exactly as nondeterministic as before this
// fix.
func (r *Registry) List() []mcp.Tool {
	type keyed struct {
		key  string
		tool mcp.Tool
	}
	entries := make([]keyed, 0, len(r.tools))
	for key, tool := range r.tools {
		entries = append(entries, keyed{key, tool.Definition})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].tool.Name != entries[j].tool.Name {
			return entries[i].tool.Name < entries[j].tool.Name
		}
		return entries[i].key < entries[j].key
	})
	tools := make([]mcp.Tool, len(entries))
	for i, e := range entries {
		tools[i] = e.tool
	}
	return tools
}

// Execute runs a tool by name with the given arguments
func (r *Registry) Execute(ctx context.Context, name string, args map[string]interface{}) (mcp.ToolResponse, error) {
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

	// Inject context into args with a special key for tools that need it
	// This allows handlers to access the context without changing the Handler signature
	// Create a copy of args to avoid mutating the caller's map (race condition)
	argsCopy := make(map[string]interface{}, len(args)+1)
	for k, v := range args {
		argsCopy[k] = v
	}
	argsCopy["__context"] = ctx

	// Note: basic registry doesn't use context, it's mainly for stdio mode
	// ContextAwareProvider uses context for per-token connection isolation in HTTP mode
	return tool.Handler(argsCopy)
}
