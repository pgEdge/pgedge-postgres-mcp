/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import "pgedge-postgres-mcp/internal/mcp"

// readOnlyAnnotations returns the MCP tool annotations for a built-in tool
// that does not modify the database. openWorld should be true when the tool
// calls a service outside the server and its databases, such as an
// embedding provider, so that clients can tell the call may leave the
// deployment and may be billed.
//
// Annotations are hints that clients may use when deciding whether to ask
// for confirmation; the tools enforce their own read-only behaviour.
func readOnlyAnnotations(openWorld bool) *mcp.ToolAnnotations {
	readOnly := true
	return &mcp.ToolAnnotations{
		ReadOnlyHint:  &readOnly,
		OpenWorldHint: &openWorld,
	}
}
