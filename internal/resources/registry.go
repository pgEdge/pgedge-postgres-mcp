/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package resources

import (
	"context"
	"sort"

	"pgedge-postgres-mcp/internal/mcp"
)

// Handler is a function that reads a resource
type Handler func() (mcp.ResourceContent, error)

// Resource represents a registered MCP resource
type Resource struct {
	Definition mcp.Resource
	Handler    Handler
}

// Registry manages available MCP resources
type Registry struct {
	resources map[string]Resource
}

// NewRegistry creates a new resource registry
func NewRegistry() *Registry {
	return &Registry{
		resources: make(map[string]Resource),
	}
}

// Register adds a resource to the registry
func (r *Registry) Register(uri string, resource Resource) {
	r.resources[uri] = resource
}

// Get retrieves a resource by URI
func (r *Registry) Get(uri string) (Resource, bool) {
	resource, exists := r.resources[uri]
	return resource, exists
}

// List returns all registered resource definitions, sorted by URI for a
// deterministic order across calls; see the tool registry's List for why
// this matters for prompt caching.
//
// The registration key breaks ties, since it is unique by construction
// even in the case, not exercised by any caller today, of two entries
// advertising the same Definition.URI under different keys: sort.Slice
// gives no ordering guarantee for elements that compare equal.
func (r *Registry) List() []mcp.Resource {
	type keyed struct {
		key      string
		resource mcp.Resource
	}
	entries := make([]keyed, 0, len(r.resources))
	for key, resource := range r.resources {
		entries = append(entries, keyed{key, resource.Definition})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].resource.URI != entries[j].resource.URI {
			return entries[i].resource.URI < entries[j].resource.URI
		}
		return entries[i].key < entries[j].key
	})
	resources := make([]mcp.Resource, len(entries))
	for i, e := range entries {
		resources[i] = e.resource
	}
	return resources
}

// Read retrieves a resource by URI and executes its handler
// Note: This implementation ignores the context parameter for backward compatibility
func (r *Registry) Read(ctx context.Context, uri string) (mcp.ResourceContent, error) {
	resource, exists := r.Get(uri)
	if !exists {
		return mcp.ResourceContent{
			URI: uri,
			Contents: []mcp.ContentItem{
				{
					Type: "text",
					Text: "Resource not found: " + uri,
				},
			},
		}, nil
	}

	return resource.Handler()
}
