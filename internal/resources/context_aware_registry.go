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
	"fmt"
	"sort"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// ContextAwareHandler is a function that reads a resource with context and database client
type ContextAwareHandler func(ctx context.Context, dbClient *database.Client) (mcp.ResourceContent, error)

// ContextAwareRegistry wraps a resource registry and provides per-token database clients
// This ensures connection isolation in HTTP/HTTPS mode with authentication
type ContextAwareRegistry struct {
	clientManager   *database.ClientManager
	authEnabled     bool
	accessChecker   *auth.DatabaseAccessChecker
	customResources map[string]customResource
	cfg             *config.Config
}

// customResource represents a user-defined resource
type customResource struct {
	definition mcp.Resource
	handler    ContextAwareHandler
}

// NewContextAwareRegistry creates a new context-aware resource registry
func NewContextAwareRegistry(clientManager *database.ClientManager, authEnabled bool, accessChecker *auth.DatabaseAccessChecker, cfg *config.Config) *ContextAwareRegistry {
	return &ContextAwareRegistry{
		clientManager:   clientManager,
		authEnabled:     authEnabled,
		accessChecker:   accessChecker,
		customResources: make(map[string]customResource),
		cfg:             cfg,
	}
}

// List returns all available resource definitions
//
// Sorted by URI for a deterministic order across calls: customResources
// below is a map, so its iteration order is randomised per call, and an
// unsorted list changes on every tools/list, invalidating the
// provider-side prompt cache and any client-side diffing of the set.
//
// The registration key (the built-in entry's own URI, or a custom
// resource's map key) breaks ties, since it is unique by construction
// even in the case, not exercised by any caller today, of two entries
// advertising the same URI under different keys: sort.Slice gives no
// ordering guarantee for elements that compare equal.
func (r *ContextAwareRegistry) List() []mcp.Resource {
	type keyed struct {
		key      string
		resource mcp.Resource
	}
	entries := []keyed{}

	// Start with static built-in resources (only include enabled ones)
	if r.cfg.Builtins.Resources.IsResourceEnabled(URISystemInfo) {
		entries = append(entries, keyed{
			key: URISystemInfo,
			resource: mcp.Resource{
				URI:         URISystemInfo,
				Name:        "postgresql_system_info",
				Description: "Returns PostgreSQL version, operating system, and build architecture information. Provides a quick way to check server version and platform details.",
				MimeType:    "application/json",
			},
		})
	}

	// Add custom resources
	for key, customRes := range r.customResources {
		entries = append(entries, keyed{key: key, resource: customRes.definition})
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

// Read retrieves a resource by URI with the appropriate database client.
//
// Whether uri names a resource this registry knows about at all is
// checked first, before ever acquiring a database client: an unknown
// URI is a protocol-level error (mcp.ErrResourceNotFound, which the MCP
// layer translates to a JSON-RPC error per the spec) that has nothing
// to do with database connectivity, so it must not depend on a
// database connection succeeding first. Get the client only once the
// resource is confirmed to exist.
func (r *ContextAwareRegistry) Read(ctx context.Context, uri string) (mcp.ResourceContent, error) {
	// Check if this is a custom resource first
	if customRes, exists := r.customResources[uri]; exists {
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
		return customRes.handler(ctx, dbClient)
	}

	if uri != URISystemInfo {
		return mcp.ResourceContent{}, mcp.ErrResourceNotFound
	}

	// Check if the built-in resource is enabled
	if !r.cfg.Builtins.Resources.IsResourceEnabled(uri) {
		return mcp.ResourceContent{
			URI: uri,
			Contents: []mcp.ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Resource '%s' is not available", uri),
				},
			},
		}, nil
	}

	// Get the appropriate database client for built-in resources
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

	resource := PGSystemInfoResource(dbClient)

	return resource.Handler()
}

// getClient returns the appropriate database client based on authentication state
// and the currently selected database for the token
func (r *ContextAwareRegistry) getClient(ctx context.Context) (*database.Client, error) {
	if !r.authEnabled {
		// Authentication disabled - use "default" key in ClientManager
		// Get the current database for this session
		currentDB := r.clientManager.GetCurrentDatabase("default")
		if currentDB == "" {
			currentDB = r.clientManager.GetDefaultDatabaseName()
		}

		client, err := r.clientManager.GetClientForDatabase("default", currentDB)
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

	// Get the current database for this token
	currentDB := r.clientManager.GetCurrentDatabase(tokenHash)

	// Check if current database is accessible, otherwise find first accessible one
	if r.accessChecker != nil {
		allConfigs := r.clientManager.GetDatabaseConfigs()
		accessibleConfigs := r.accessChecker.GetAccessibleDatabases(ctx, allConfigs)

		if len(accessibleConfigs) == 0 {
			username := auth.GetUsernameFromContext(ctx)
			if username != "" {
				return nil, fmt.Errorf("no databases are configured for user '%s' - contact your administrator", username)
			}
			return nil, fmt.Errorf("no accessible databases for this user")
		}

		// Check if current database is in accessible list
		currentAccessible := false
		for i := range accessibleConfigs {
			if accessibleConfigs[i].Name == currentDB {
				currentAccessible = true
				break
			}
		}

		// If current is not accessible, use first accessible database
		if !currentAccessible {
			currentDB = accessibleConfigs[0].Name
			// Update the current database for this token (best effort - operation continues regardless)
			_ = r.clientManager.SetCurrentDatabase(tokenHash, currentDB) //nolint:errcheck // preference update, doesn't affect operation
		}
	}

	// Fallback to default if still empty
	if currentDB == "" {
		currentDB = r.clientManager.GetDefaultDatabaseName()
	}

	// Get or create client for this token's current database
	client, err := r.clientManager.GetClientForDatabase(tokenHash, currentDB)
	if err != nil {
		return nil, fmt.Errorf("no database connection configured for this token: %w", err)
	}

	return client, nil
}
