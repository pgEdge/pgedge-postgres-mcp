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

// Query result limit constants for PostgreSQL statistics queries
const (
	// DefaultQueryLimit is the default maximum number of rows to return
	// from statistics queries to prevent overwhelming MCP clients with large result sets
	DefaultQueryLimit = 100
)
