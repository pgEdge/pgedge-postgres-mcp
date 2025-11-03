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
			URI:         URISettings,
			Name:        "PostgreSQL Server Configuration",
			Description: "Returns PostgreSQL server configuration parameters including current values, default values, pending changes, and descriptions. Queries pg_settings system catalog.",
			MimeType:    "application/json",
		},
		{
			URI:         URISystemInfo,
			Name:        "PostgreSQL System Information",
			Description: "Returns PostgreSQL version, operating system, and build architecture information. Provides a quick way to check server version and platform details.",
			MimeType:    "application/json",
		},
		{
			URI:         URIStatActivity,
			Name:        "PostgreSQL Current Activity",
			Description: "Shows information about currently executing queries and connections. Useful for monitoring active sessions, identifying long-running queries, and understanding current database load. Each row represents one server process with details about its current activity.",
			MimeType:    "application/json",
		},
		{
			URI:         URIStatDatabase,
			Name:        "PostgreSQL Database Statistics",
			Description: "Provides cumulative statistics for each database including transaction counts, block reads/writes, tuple operations, conflicts, and deadlocks. Essential for understanding database-level performance patterns and identifying I/O bottlenecks.",
			MimeType:    "application/json",
		},
		{
			URI:         URIStatUserTables,
			Name:        "PostgreSQL Table Statistics",
			Description: "Shows statistics for user tables including sequential and index scans, tuple operations (inserts/updates/deletes), and vacuum/analyze activity. Critical for identifying tables that need optimization or indexing improvements.",
			MimeType:    "application/json",
		},
		{
			URI:         URIStatUserIndexes,
			Name:        "PostgreSQL Index Statistics",
			Description: "Provides statistics about index usage including scan counts and tuple operations. Essential for identifying unused indexes that can be dropped and finding tables that might benefit from additional indexes. Helps optimize query performance and reduce storage overhead.",
			MimeType:    "application/json",
		},
		{
			URI:         URIStatReplication,
			Name:        "PostgreSQL Replication Status",
			Description: "Shows the status of replication connections from this primary server including WAL sender processes, replication lag, and sync state. Empty if the server is not a replication primary or has no active replicas. Critical for monitoring replication health and identifying lag issues.",
			MimeType:    "application/json",
		},
		{
			URI:         URIStatBgwriter,
			Name:        "PostgreSQL Background Writer Statistics",
			Description: "Provides statistics about the background writer process including checkpoints, buffer writes, and backend fsync operations. Useful for tuning checkpoint and background writer settings for optimal I/O performance. High values of checkpoints_req or buffers_backend may indicate configuration issues.",
			MimeType:    "application/json",
		},
		{
			URI:         URIStatWAL,
			Name:        "PostgreSQL WAL Statistics",
			Description: "Provides Write-Ahead Log (WAL) statistics including WAL records, FPI, bytes, buffers, and sync operations. Available in PostgreSQL 14 and later. Useful for understanding WAL generation patterns, archive performance, and transaction log activity. Returns version error for PostgreSQL 13 and earlier.",
			MimeType:    "application/json",
		},
		{
			URI:         URIStatIOUserTables,
			Name:        "PostgreSQL Table I/O Statistics",
			Description: "Shows disk block I/O statistics for user tables including heap, index, TOAST, and TOAST index blocks. Tracks blocks read from disk vs. cache hits. Essential for identifying I/O bottlenecks and cache efficiency. High read counts indicate potential need for more memory or query optimization.",
			MimeType:    "application/json",
		},
		{
			URI:         URIStatIOUserIndexes,
			Name:        "PostgreSQL Index I/O Statistics",
			Description: "Shows disk block I/O statistics for user indexes. Tracks blocks read from disk vs. cache hits for each index. Essential for identifying indexes causing high I/O load and evaluating cache effectiveness. Helps determine if shared_buffers should be increased or if indexes need optimization.",
			MimeType:    "application/json",
		},
		{
			URI:         URIStatIOUserSequences,
			Name:        "PostgreSQL Sequence I/O Statistics",
			Description: "Shows disk block I/O statistics for user sequences. Tracks blocks read from disk vs. cache hits for sequence objects. Sequences should typically have very high cache hit ratios since they're frequently accessed. Low hit ratios may indicate cache pressure or excessive sequence usage patterns.",
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
					Text: fmt.Sprintf("Failed to get database client: %v\nPlease call set_database_connection first to configure the database connection.", err),
				},
			},
		}, nil
	}

	// Create resource handler with the correct client
	var resource Resource
	switch uri {
	case URISettings:
		resource = PGSettingsResource(dbClient)
	case URISystemInfo:
		resource = PGSystemInfoResource(dbClient)
	case URIStatActivity:
		resource = PGStatActivityResource(dbClient)
	case URIStatDatabase:
		resource = PGStatDatabaseResource(dbClient)
	case URIStatUserTables:
		resource = PGStatUserTablesResource(dbClient)
	case URIStatUserIndexes:
		resource = PGStatUserIndexesResource(dbClient)
	case URIStatReplication:
		resource = PGStatReplicationResource(dbClient)
	case URIStatBgwriter:
		resource = PGStatBgwriterResource(dbClient)
	case URIStatWAL:
		resource = PGStatWALResource(dbClient)
	case URIStatIOUserTables:
		resource = PGStatIOUserTablesResource(dbClient)
	case URIStatIOUserIndexes:
		resource = PGStatIOUserIndexesResource(dbClient)
	case URIStatIOUserSequences:
		resource = PGStatIOUserSequencesResource(dbClient)
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

	// Get or create client for this token (don't auto-connect)
	client, err := r.clientManager.GetOrCreateClient(tokenHash, false)
	if err != nil {
		return nil, fmt.Errorf("no database connection configured for this token: %w", err)
	}

	return client, nil
}
