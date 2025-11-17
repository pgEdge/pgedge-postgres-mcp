/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

// NewTestClient creates a database client for testing with mock data
// This allows tests in other packages to create clients with predetermined metadata
func NewTestClient(connStr string, metadata map[string]TableInfo) *Client {
	client := NewClient(nil)

	// Add mock connection info
	client.connections[connStr] = &ConnectionInfo{
		ConnString:     connStr,
		Pool:           nil, // No actual connection pool needed for tests
		Metadata:       metadata,
		MetadataLoaded: true,
	}

	// Set as default connection
	client.defaultConnStr = connStr

	return client
}
