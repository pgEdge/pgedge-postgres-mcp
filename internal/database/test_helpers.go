/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
	"time"

	"pgedge-postgres-mcp/internal/config"
)

// NewTestClient creates a database client for testing with mock data
// This allows tests in other packages to create clients with predetermined metadata
func NewTestClient(connStr string, metadata map[string]TableInfo) *Client {
	client := NewClient(nil)

	// Add mock connection info
	client.connections[connStr] = &ConnectionInfo{
		ConnString:       connStr,
		Pool:             nil, // No actual connection pool needed for tests
		Metadata:         metadata,
		MetadataLoaded:   true,
		MetadataLoadedAt: time.Now(),
	}

	// Set as default connection
	client.defaultConnStr = connStr

	return client
}

// NewTestClientWithConfig is NewTestClient plus an attached
// NamedDatabaseConfig, for tests that need to exercise config-derived
// behavior such as DisplayName, ConfiguredHost, ConfiguredPort, or
// AllowWrites.
func NewTestClientWithConfig(connStr string, metadata map[string]TableInfo, cfg *config.NamedDatabaseConfig) *Client {
	client := NewTestClient(connStr, metadata)
	client.dbConfig = cfg
	return client
}
