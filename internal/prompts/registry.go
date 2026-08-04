/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package prompts

import (
	"fmt"
	"sort"

	"pgedge-postgres-mcp/internal/mcp"
)

// Prompt represents a registered MCP prompt
type Prompt struct {
	Definition mcp.Prompt
	Handler    func(args map[string]string) mcp.PromptResult
}

// Registry manages available MCP prompts
type Registry struct {
	prompts map[string]Prompt
}

// NewRegistry creates a new prompt registry
func NewRegistry() *Registry {
	return &Registry{
		prompts: make(map[string]Prompt),
	}
}

// Register adds a prompt to the registry
func (r *Registry) Register(name string, prompt Prompt) {
	r.prompts[name] = prompt
}

// Get retrieves a prompt by name
func (r *Registry) Get(name string) (Prompt, bool) {
	prompt, exists := r.prompts[name]
	return prompt, exists
}

// List returns all registered prompt definitions, sorted by name for a
// deterministic order across calls; see the tool registry's List for why
// this matters for prompt caching.
func (r *Registry) List() []mcp.Prompt {
	prompts := make([]mcp.Prompt, 0, len(r.prompts))
	for _, prompt := range r.prompts {
		prompts = append(prompts, prompt.Definition)
	}
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].Name < prompts[j].Name
	})
	return prompts
}

// Execute runs a prompt by name with the given arguments
func (r *Registry) Execute(name string, args map[string]string) (mcp.PromptResult, error) {
	prompt, exists := r.Get(name)
	if !exists {
		// Build list of available prompt names (sorted alphabetically)
		available := make([]string, 0, len(r.prompts))
		for promptName := range r.prompts {
			available = append(available, promptName)
		}
		sort.Strings(available)
		return mcp.PromptResult{}, fmt.Errorf("prompt %q not found. Available prompts: %v", name, available)
	}

	return prompt.Handler(args), nil
}
