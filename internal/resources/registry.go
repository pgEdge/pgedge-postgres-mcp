/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package resources

import (
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

// List returns all registered resource definitions
func (r *Registry) List() []mcp.Resource {
	resources := make([]mcp.Resource, 0, len(r.resources))
	for _, resource := range r.resources {
		resources = append(resources, resource.Definition)
	}
	return resources
}

// Read retrieves a resource by URI and executes its handler
func (r *Registry) Read(uri string) (mcp.ResourceContent, error) {
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
