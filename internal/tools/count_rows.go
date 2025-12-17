/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"fmt"
	"strings"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/logging"
	"pgedge-postgres-mcp/internal/mcp"
)

// CountRowsTool creates the count_rows tool for lightweight row counting
func CountRowsTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "count_rows",
			Description: `Get the row count of a table with optional filtering.

<usecase>
Use count_rows to efficiently determine data volume:
- Check total row count before querying large tables
- Verify filter conditions match expected number of rows
- Plan query strategies based on data size
- Validate data existence without fetching rows
</usecase>

<examples>
✓ count_rows(table="orders") → Total orders in database
✓ count_rows(table="orders", schema="sales") → Orders in sales schema
✓ count_rows(table="orders", where="status = 'pending'") → Pending orders only
✓ count_rows(table="users", where="created_at > '2024-01-01'") → Recent users
</examples>

<important>
- Much more efficient than SELECT * with LIMIT for checking data volume
- Use this before query_database to plan appropriate LIMIT values
- WHERE clause is optional - omit for total count
- Returns a single integer count - minimal token usage
</important>`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"table": map[string]interface{}{
						"type":        "string",
						"description": "Name of the table to count rows from",
					},
					"schema": map[string]interface{}{
						"type":        "string",
						"description": "Schema name (default: public)",
						"default":     "public",
					},
					"where": map[string]interface{}{
						"type":        "string",
						"description": "Optional WHERE clause condition (without the WHERE keyword). Example: \"status = 'active' AND created_at > '2024-01-01'\"",
					},
				},
				Required: []string{"table"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			table, ok := args["table"].(string)
			if !ok || table == "" {
				return mcp.NewToolError("Missing or invalid 'table' parameter")
			}

			// Get schema, default to public
			schema := "public"
			if s, ok := args["schema"].(string); ok && s != "" {
				schema = s
			}

			// Get optional WHERE clause
			whereClause := ""
			if w, ok := args["where"].(string); ok && w != "" {
				whereClause = w
			}

			// Get connection
			connStr := dbClient.GetDefaultConnection()
			if !dbClient.IsMetadataLoadedFor(connStr) {
				return mcp.NewToolError(mcp.DatabaseNotReadyError)
			}

			pool := dbClient.GetPoolFor(connStr)
			if pool == nil {
				return mcp.NewToolError(fmt.Sprintf("Connection pool not found for: %s", database.SanitizeConnStr(connStr)))
			}

			// Build the COUNT query with proper quoting
			var sqlQuery string
			if whereClause != "" {
				sqlQuery = fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE %s`,
					quoteIdentifier(schema),
					quoteIdentifier(table),
					whereClause)
			} else {
				sqlQuery = fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s`,
					quoteIdentifier(schema),
					quoteIdentifier(table))
			}

			// Execute in a read-only transaction
			ctx := context.Background()
			tx, err := pool.Begin(ctx)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to begin transaction: %v", err))
			}

			committed := false
			defer func() {
				if r := recover(); r != nil {
					_ = tx.Rollback(ctx) //nolint:errcheck // Best effort cleanup on panic
					panic(r)
				}
				if !committed {
					_ = tx.Rollback(ctx) //nolint:errcheck // rollback in defer after commit is expected to fail
				}
			}()

			// Set transaction to read-only
			_, err = tx.Exec(ctx, "SET TRANSACTION READ ONLY")
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to set transaction read-only: %v", err))
			}

			var count int64
			err = tx.QueryRow(ctx, sqlQuery).Scan(&count)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("SQL Query:\n%s\n\nError: %v", sqlQuery, err))
			}

			if err := tx.Commit(ctx); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to commit transaction: %v", err))
			}
			committed = true

			// Log execution
			logging.Info("count_rows_executed",
				"schema", schema,
				"table", table,
				"has_where", whereClause != "",
				"count", count,
			)

			// Build response
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Database: %s\n\n", database.SanitizeConnStr(connStr)))
			sb.WriteString(fmt.Sprintf("SQL Query:\n%s\n\n", sqlQuery))
			sb.WriteString(fmt.Sprintf("Count: %d", count))

			return mcp.NewToolSuccess(sb.String())
		},
	}
}

// quoteIdentifier quotes a SQL identifier to prevent injection
func quoteIdentifier(name string) string {
	// Double any existing double quotes and wrap in double quotes
	escaped := strings.ReplaceAll(name, `"`, `""`)
	return `"` + escaped + `"`
}
