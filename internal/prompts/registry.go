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
//
// The registration key breaks ties, since it is unique by construction
// even in the case, not exercised by any caller today, of two entries
// advertising the same Definition.Name under different keys: sort.Slice
// gives no ordering guarantee for elements that compare equal.
func (r *Registry) List() []mcp.Prompt {
	type keyed struct {
		key    string
		prompt mcp.Prompt
	}
	entries := make([]keyed, 0, len(r.prompts))
	for key, prompt := range r.prompts {
		entries = append(entries, keyed{key, prompt.Definition})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].prompt.Name != entries[j].prompt.Name {
			return entries[i].prompt.Name < entries[j].prompt.Name
		}
		return entries[i].key < entries[j].key
	})
	prompts := make([]mcp.Prompt, len(entries))
	for i, e := range entries {
		prompts[i] = e.prompt
	}
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
