/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package resources

import (
	"context"
	"fmt"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// ContextAwareRegistry wraps a resource registry and provides per-token database clients
// This ensures connection isolation in HTTP/HTTPS mode with authentication
type ContextAwareRegistry struct {
	clientManager *database.ClientManager
	authEnabled   bool
}

// NewContextAwareRegistry creates a new context-aware resource registry
func NewContextAwareRegistry(clientManager *database.ClientManager, authEnabled bool) *ContextAwareRegistry {
	return &ContextAwareRegistry{
		clientManager: clientManager,
		authEnabled:   authEnabled,
	}
}

// List returns all available resource definitions
func (r *ContextAwareRegistry) List() []mcp.Resource {
	// Return static list of all resources
	return []mcp.Resource{
		{
			URI:         URISystemInfo,
			Name:        "PostgreSQL System Information",
			Description: "Returns PostgreSQL version, operating system, and build architecture information. Provides a quick way to check server version and platform details.",
			MimeType:    "application/json",
		},
		{
			URI:         URIDatabaseSchema,
			Name:        "PostgreSQL Database Schema",
			Description: "Returns a lightweight overview of all tables in the database. Lists schema names, table names, and table owners. Use get_schema_info tool for detailed column information.",
			MimeType:    "application/json",
		},
	}
}

// Read retrieves a resource by URI with the appropriate database client
func (r *ContextAwareRegistry) Read(ctx context.Context, uri string) (mcp.ResourceContent, error) {
	// Get the appropriate database client for this request
	dbClient, err := r.getClient(ctx)
	if err != nil {
		return mcp.ResourceContent{
			URI: uri,
			Contents: []mcp.ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
		}, nil
	}

	// Create resource handler with the correct client
	var resource Resource
	switch uri {
	case URISystemInfo:
		resource = PGSystemInfoResource(dbClient)
	case URIDatabaseSchema:
		resource = PGDatabaseSchemaResource(dbClient)
	default:
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

// getClient returns the appropriate database client based on authentication state
func (r *ContextAwareRegistry) getClient(ctx context.Context) (*database.Client, error) {
	if !r.authEnabled {
		// Authentication disabled - use "default" key in ClientManager
		client, err := r.clientManager.GetOrCreateClient("default", false)
		if err != nil {
			return nil, fmt.Errorf("no database connection configured: %w", err)
		}
		return client, nil
	}

	// Authentication enabled - get per-token client
	tokenHash := auth.GetTokenHashFromContext(ctx)
	if tokenHash == "" {
		return nil, fmt.Errorf("no authentication token found in request context")
	}

	// Get or create client for this token
	// Auto-connect if database is configured (authenticated users get automatic database access)
	client, err := r.clientManager.GetOrCreateClient(tokenHash, true)
	if err != nil {
		return nil, fmt.Errorf("no database connection configured for this token: %w", err)
	}

	return client, nil
}
