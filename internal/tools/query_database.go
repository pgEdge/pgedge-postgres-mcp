/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// QueryDatabaseTool creates the query_database tool
func QueryDatabaseTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "query_database",
			Description: "Execute a SQL query against the PostgreSQL database in a read-only transaction. All queries run in read-only mode to prevent data modifications. You can temporarily query a different database by including 'at postgres://...' in your query (e.g., 'SELECT * FROM users at postgres://user@host/other_database'), or set a new default connection with 'set default database to postgres://...'. IMPORTANT: These connection changes are temporary and do NOT modify saved connections.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "SQL query to execute against the database. All queries run in read-only transactions. Can include connection strings like 'SELECT * FROM users at postgres://host/db' or 'set default database to postgres://host/db'.",
					},
				},
				Required: []string{"query"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			query, ok := args["query"].(string)
			if !ok {
				return mcp.NewToolError("Missing or invalid 'query' parameter")
			}

			// Parse query for connection string and intent
			queryCtx := database.ParseQueryForConnection(query)

			// Determine which connection to use
			connStr := dbClient.GetDefaultConnection()
			var connectionMessage string

			// Handle connection string changes
			if queryCtx.ConnectionString != "" {
				if queryCtx.SetAsDefault {
					// User wants to set a new default connection
					err := dbClient.SetDefaultConnection(queryCtx.ConnectionString)
					if err != nil {
						return mcp.NewToolError(fmt.Sprintf("Failed to set default connection to %s: %v", queryCtx.ConnectionString, err))
					}

					return mcp.NewToolSuccess(fmt.Sprintf("Successfully set default database connection to:\n%s\n\nMetadata loaded: %d tables/views available.",
						queryCtx.ConnectionString,
						len(dbClient.GetMetadata())))
				} else {
					// Temporary connection for this query only
					err := dbClient.ConnectTo(queryCtx.ConnectionString)
					if err != nil {
						return mcp.NewToolError(fmt.Sprintf("Failed to connect to %s: %v", queryCtx.ConnectionString, err))
					}

					// Load metadata if needed
					if !dbClient.IsMetadataLoadedFor(queryCtx.ConnectionString) {
						err = dbClient.LoadMetadataFor(queryCtx.ConnectionString)
						if err != nil {
							return mcp.NewToolError(fmt.Sprintf("Failed to load metadata from %s: %v", queryCtx.ConnectionString, err))
						}
					}

					connStr = queryCtx.ConnectionString
					connectionMessage = fmt.Sprintf("Using connection: %s\n\n", connStr)
				}
			}

			// If the cleaned query is empty (e.g., just a connection command), we're done
			if strings.TrimSpace(queryCtx.CleanedQuery) == "" {
				return mcp.NewToolSuccess("Connection command executed successfully. No query to run.")
			}

			// Check if metadata is loaded for the target connection
			if !dbClient.IsMetadataLoadedFor(connStr) {
				return mcp.NewToolError(mcp.DatabaseNotReadyError)
			}

			// Use the cleaned query as SQL
			sqlQuery := strings.TrimSpace(queryCtx.CleanedQuery)

			// Execute the SQL query on the appropriate connection in a read-only transaction
			ctx := context.Background()
			pool := dbClient.GetPoolFor(connStr)
			if pool == nil {
				return mcp.NewToolError(fmt.Sprintf("Connection pool not found for: %s", connStr))
			}

			// Begin a transaction with read-only protection
			tx, err := pool.Begin(ctx)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to begin transaction: %v", err))
			}
			defer func() {
				if err := tx.Rollback(ctx); err != nil {
					// Rollback errors are expected if transaction was already committed/closed
					// Log for debugging but don't treat as fatal
					fmt.Fprintf(os.Stderr, "WARNING: Transaction rollback returned error (may be expected): %v\n", err)
				}
			}()

			// Set transaction to read-only to prevent any data modifications
			_, err = tx.Exec(ctx, "SET TRANSACTION READ ONLY")
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to set transaction read-only: %v", err))
			}

			rows, err := tx.Query(ctx, sqlQuery)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("%sSQL Query:\n%s\n\nError executing query: %v", connectionMessage, sqlQuery, err))
			}
			defer rows.Close()

			// Get column names
			fieldDescriptions := rows.FieldDescriptions()
			var columnNames []string
			for _, fd := range fieldDescriptions {
				columnNames = append(columnNames, string(fd.Name))
			}

			// Collect results
			var results []map[string]interface{}
			for rows.Next() {
				values, err := rows.Values()
				if err != nil {
					return mcp.NewToolError(fmt.Sprintf("Error reading row: %v", err))
				}

				row := make(map[string]interface{})
				for i, colName := range columnNames {
					row[colName] = values[i]
				}
				results = append(results, row)
			}

			if err := rows.Err(); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error iterating rows: %v", err))
			}

			// Format results
			resultsJSON, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error formatting results: %v", err))
			}

			// Commit the read-only transaction
			if err := tx.Commit(ctx); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to commit transaction: %v", err))
			}

			var sb strings.Builder
			if connectionMessage != "" {
				sb.WriteString(connectionMessage)
			}
			sb.WriteString(fmt.Sprintf("SQL Query:\n%s\n\n", sqlQuery))
			sb.WriteString(fmt.Sprintf("Results (%d rows):\n%s", len(results), string(resultsJSON)))

			return mcp.NewToolSuccess(sb.String())
		},
	}
}
