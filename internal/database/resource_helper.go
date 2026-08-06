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
	"context"
	"encoding/json"
	"fmt"

	"pgedge-postgres-mcp/internal/mcp"

	"github.com/jackc/pgx/v5"
)

// RowProcessor is a function that processes query rows and returns the result data
type RowProcessor func(rows pgx.Rows) (interface{}, error)

// ExecuteResourceQuery executes a SQL query and returns a formatted ResourceContent
// This helper abstracts the common pattern used by all resource implementations:
// - Check if database is ready
// - Get connection pool
// - Execute query
// - Process rows with custom processor
// - Marshal to JSON
// - Return formatted ResourceContent
func ExecuteResourceQuery(client *Client, uri string, query string, processor RowProcessor) (mcp.ResourceContent, error) {
	// Ensure metadata is loaded, reloading it when the TTL has expired.
	// Stale metadata is not the same thing as an unready database, and
	// reporting DATABASE_NOT_READY for it left the client retrying
	// against a database that was fine (issue #218).
	if err := client.EnsureMetadata(); err != nil {
		return mcp.NewResourceJSONError(uri, mcp.DatabaseNotReadyMessage, mcp.ErrorCodeDatabaseNotReady, true)
	}

	// Get connection pool
	pool := client.GetPool()
	if pool == nil {
		return mcp.ResourceContent{}, fmt.Errorf("no connection pool available")
	}

	// Execute query
	ctx := context.Background()
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return mcp.ResourceContent{}, fmt.Errorf("failed to query: %w", err)
	}
	defer rows.Close()

	// Process rows with custom processor
	data, err := processor(rows)
	if err != nil {
		return mcp.ResourceContent{}, fmt.Errorf("failed to process rows: %w", err)
	}

	// Check for row iteration errors
	if err := rows.Err(); err != nil {
		return mcp.ResourceContent{}, fmt.Errorf("error iterating rows: %w", err)
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.ResourceContent{}, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Return formatted resource
	return mcp.NewResourceSuccess(uri, "application/json", string(jsonData))
}
