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

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/logging"
	"pgedge-postgres-mcp/internal/mcp"
)

// validateReadOnlyQuery checks whether a query attempts to tamper with
// the read-only transaction setting. Queries that reference
// transaction_read_only or default_transaction_read_only are rejected
// because constructs such as DO blocks with set_config() can bypass
// SET TRANSACTION READ ONLY within a single statement.
func validateReadOnlyQuery(query string) error {
	upper := strings.ToUpper(query)
	if strings.Contains(upper, "TRANSACTION_READ_ONLY") ||
		strings.Contains(upper, "DEFAULT_TRANSACTION_READ_ONLY") {
		return fmt.Errorf(
			"query rejected: queries cannot reference " +
				"'transaction_read_only' when the database " +
				"connection is in read-only mode")
	}
	return nil
}

// stripTrailingSemicolons removes trailing semicolons and whitespace from
// a SQL query so that LIMIT/OFFSET clauses can be safely appended.
func stripTrailingSemicolons(query string) string {
	return strings.TrimRightFunc(query, func(r rune) bool {
		return r == ';' || unicode.IsSpace(r)
	})
}

// QueryDatabaseTool creates the query_database tool
func QueryDatabaseTool(dbClient *database.Client) Tool {
	// Determine the write access description based on configuration
	writeAccessDesc := "All queries run in READ-ONLY transactions (no data modifications possible)"
	allowWrites := dbClient != nil && dbClient.AllowWrites()
	if allowWrites {
		writeAccessDesc = `⚠️ WRITE ACCESS ENABLED: This database connection allows data modifications.
  INSERT, UPDATE, DELETE, DROP, and other write operations ARE PERMITTED.
  Exercise extreme caution when executing queries that modify data.`
	}

	// Build tool annotations based on write access
	boolTrue := true
	boolFalse := false
	var annotations *mcp.ToolAnnotations
	if allowWrites {
		annotations = &mcp.ToolAnnotations{
			ReadOnlyHint:    &boolFalse,
			DestructiveHint: &boolTrue,
		}
	} else {
		annotations = &mcp.ToolAnnotations{
			ReadOnlyHint: &boolTrue,
		}
	}

	return Tool{
		Definition: mcp.Tool{
			Name:        "query_database",
			Annotations: annotations,
			Description: fmt.Sprintf(`Execute SQL queries against the connected PostgreSQL database. Use this tool instead of psql, shell commands, or direct database connections for all SQL operations — it handles connection management, authentication, and access control automatically.

<usecase>
Use query_database when you need:
- Exact matches by ID, status, date ranges, or specific column values
- Aggregations: COUNT, SUM, AVG, GROUP BY, HAVING
- Joins across tables using foreign keys
- Sorting or filtering by structured columns
- Transaction data, user records, system logs with known schema
- Checking existence, counts, or specific field values
</usecase>

<when_not_to_use>
DO NOT use for:
- Natural language content search → use similarity_search instead
- Finding topics, themes, or concepts in text → use similarity_search
- "Documents about X" queries → use similarity_search
- Semantic similarity or meaning-based queries → use similarity_search
</when_not_to_use>

<examples>
✓ "How many orders were placed last week?"
✓ "Show all users with status = 'active' and created_at > '2024-01-01'"
✓ "Average order value grouped by region"
✓ "Get user details for ID 12345"
✗ "Find documents about database performance" → use similarity_search
✗ "Show tickets related to connection issues" → use similarity_search
</examples>

<important>
- %s
- Results are limited to prevent excessive token usage
- Results are returned in TSV (tab-separated values) format for efficiency
</important>

<rate_limit_awareness>
To avoid rate limits (30,000 input tokens/minute):
- ALWAYS use the 'limit' parameter - it defaults to 100 rows
- Start with limit=10 for exploration queries, increase only if needed
- Filter results in WHERE clauses rather than fetching everything
- Use get_schema_info(schema_name="specific") to reduce metadata size
- If rate limited, wait 60 seconds before retrying
</rate_limit_awareness>`, writeAccessDesc),
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "SQL query to execute against the database.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of rows to return (default: 100, max: 1000). Automatically appended to query if not already present. Use higher limits only when necessary to avoid excessive token usage.",
						"default":     100,
						"minimum":     1,
						"maximum":     1000,
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of rows to skip before returning results (for pagination). Use with limit to page through large result sets. Example: offset=100 with limit=100 returns rows 101-200.",
						"default":     0,
						"minimum":     0,
					},
				},
				Required: []string{"query"},
			},
		},
		Handler: func(args map[string]any) (mcp.ToolResponse, error) {
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
						return mcp.NewToolError(fmt.Sprintf("Failed to set default connection to %s: %v", database.SanitizeConnStr(queryCtx.ConnectionString), err))
					}

					return mcp.NewToolSuccess(fmt.Sprintf("Successfully set default database connection to:\n%s\n\nMetadata loaded: %d tables/views available.",
						database.SanitizeConnStr(queryCtx.ConnectionString),
						len(dbClient.GetMetadata())))
				} else {
					// Temporary connection for this query only
					err := dbClient.ConnectTo(queryCtx.ConnectionString)
					if err != nil {
						return mcp.NewToolError(fmt.Sprintf("Failed to connect to %s: %v", database.SanitizeConnStr(queryCtx.ConnectionString), err))
					}

					// Load metadata if needed
					if !dbClient.IsMetadataLoadedFor(queryCtx.ConnectionString) {
						err = dbClient.LoadMetadataFor(queryCtx.ConnectionString)
						if err != nil {
							return mcp.NewToolError(fmt.Sprintf("Failed to load metadata from %s: %v", database.SanitizeConnStr(queryCtx.ConnectionString), err))
						}
					}

					connStr = queryCtx.ConnectionString
					connectionMessage = fmt.Sprintf("Using connection: %s\n\n", database.SanitizeConnStr(connStr))
				}
			}

			// If the cleaned query is empty (e.g., just a connection command), we're done
			if strings.TrimSpace(queryCtx.CleanedQuery) == "" {
				return mcp.NewToolSuccess("Connection command executed successfully. No query to run.")
			}

			// Wait for metadata to load for the target connection
			if !dbClient.IsMetadataLoadedFor(connStr) {
				if err := dbClient.LoadMetadataFor(connStr); err != nil {
					return mcp.NewToolError(fmt.Sprintf("Failed to load database metadata: %v", err))
				}
			}

			// Use the cleaned query as SQL
			sqlQuery := strings.TrimSpace(queryCtx.CleanedQuery)

			// Block queries that attempt to tamper with read-only
			// transaction settings when writes are not allowed
			allowWrites := dbClient != nil && dbClient.AllowWrites()
			if !allowWrites {
				if err := validateReadOnlyQuery(sqlQuery); err != nil {
					return mcp.NewToolError(err.Error())
				}
			}

			// Determine the limit to use
			limit := 100 // default
			if limitVal, ok := args["limit"]; ok {
				switch v := limitVal.(type) {
				case float64:
					limit = int(v)
				case int:
					limit = v
				}
			}

			// Determine the offset to use
			offset := 0 // default
			if offsetVal, ok := args["offset"]; ok {
				switch v := offsetVal.(type) {
				case float64:
					offset = int(v)
				case int:
					offset = v
				}
			}

			// Strip trailing semicolons and whitespace to avoid syntax
			// errors when appending LIMIT/OFFSET.
			sqlQuery = stripTrailingSemicolons(sqlQuery)
			if sqlQuery == "" {
				return mcp.NewToolError("Query is empty")
			}

			// Track if query already had LIMIT/OFFSET clauses
			upperQuery := strings.ToUpper(strings.TrimSpace(sqlQuery))
			hasExistingLimit := strings.Contains(upperQuery, "LIMIT")
			hasExistingOffset := strings.Contains(upperQuery, "OFFSET")

			// Check if this is a SELECT query - only SELECT queries support LIMIT/OFFSET
			// DDL (CREATE, ALTER, DROP) and DML (INSERT, UPDATE, DELETE) don't support LIMIT
			isSelectQuery := strings.HasPrefix(upperQuery, "SELECT") ||
				strings.HasPrefix(upperQuery, "WITH") || // CTEs that typically end in SELECT
				strings.HasPrefix(upperQuery, "TABLE") || // TABLE command (shorthand for SELECT * FROM)
				strings.HasPrefix(upperQuery, "VALUES") // VALUES expression

			// Check if this is a DDL query that modifies schema (requires metadata refresh)
			isDDLQuery := strings.HasPrefix(upperQuery, "CREATE") ||
				strings.HasPrefix(upperQuery, "DROP") ||
				strings.HasPrefix(upperQuery, "ALTER") ||
				strings.HasPrefix(upperQuery, "TRUNCATE")

			// Check if this is a DML query (INSERT, UPDATE, DELETE)
			isDMLQuery := strings.HasPrefix(upperQuery, "INSERT") ||
				strings.HasPrefix(upperQuery, "UPDATE") ||
				strings.HasPrefix(upperQuery, "DELETE")

			// Check if DML has RETURNING clause (returns rows like SELECT)
			hasReturning := isDMLQuery && strings.Contains(upperQuery, "RETURNING")

			// Determine if this query returns rows (needs Query) or not (needs Exec)
			// SELECT, WITH, TABLE, VALUES all return rows
			// DML with RETURNING returns rows
			// DDL and DML without RETURNING do not return rows
			returnsRows := isSelectQuery || hasReturning

			// Only inject LIMIT/OFFSET for SELECT queries that don't already have them
			// Fetch limit+1 to detect if more rows exist
			if isSelectQuery && limit > 0 && !hasExistingLimit {
				sqlQuery = fmt.Sprintf("%s LIMIT %d", sqlQuery, limit+1)
			}
			if isSelectQuery && offset > 0 && !hasExistingOffset {
				sqlQuery = fmt.Sprintf("%s OFFSET %d", sqlQuery, offset)
			}

			// Execute the SQL query on the appropriate connection in a read-only transaction
			ctx := context.Background()
			pool := dbClient.GetPoolFor(connStr)
			if pool == nil {
				return mcp.NewToolError(fmt.Sprintf("Connection pool not found for: %s", database.SanitizeConnStr(connStr)))
			}

			// Begin a transaction with read-only protection
			tx, err := pool.Begin(ctx)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to begin transaction: %v", err))
			}

			// Track whether transaction was committed
			committed := false
			defer func() {
				// Recover from panic to ensure transaction is properly rolled back
				if r := recover(); r != nil {
					// Attempt to rollback on panic
					_ = tx.Rollback(ctx) //nolint:errcheck // Best effort cleanup on panic
					// Re-panic to propagate the error
					panic(r)
				}
				if !committed {
					// Only rollback if not committed - prevents idle transactions
					_ = tx.Rollback(ctx) //nolint:errcheck // rollback in defer after commit is expected to fail
				}
			}()

			// Set transaction to read-only to prevent any data modifications
			// Unless write access is explicitly enabled for this database connection
			if !allowWrites {
				_, err = tx.Exec(ctx, "SET TRANSACTION READ ONLY")
				if err != nil {
					return mcp.NewToolError(fmt.Sprintf("Failed to set transaction read-only: %v", err))
				}
			}

			// Execute the statement using the appropriate method based on whether it returns rows
			var columnNames []string
			var results [][]any
			var commandTag string
			var rowsAffected int64

			if returnsRows {
				// Use Query() for SELECT and DML with RETURNING
				rows, err := tx.Query(ctx, sqlQuery)
				if err != nil {
					errMsg := fmt.Sprintf("%sSQL Query:\n%s\n\nError executing query: %v", connectionMessage, sqlQuery, err)
					return mcp.NewToolError(errMsg)
				}
				defer rows.Close()

				// Get column names
				fieldDescriptions := rows.FieldDescriptions()
				for _, fd := range fieldDescriptions {
					columnNames = append(columnNames, string(fd.Name))
				}

				// Collect results as array of arrays for TSV formatting
				for rows.Next() {
					values, err := rows.Values()
					if err != nil {
						return mcp.NewToolError(fmt.Sprintf("Error reading row: %v", err))
					}
					results = append(results, values)
				}

				if err := rows.Err(); err != nil {
					return mcp.NewToolError(fmt.Sprintf("Error iterating rows: %v", err))
				}

				// Close rows before commit to ensure statement is fully processed
				rows.Close()
			} else {
				// Use Exec() for DDL and DML without RETURNING
				// This is critical: Query() may not properly execute DDL/DML statements
				// due to pgx's prepared statement caching and extended query protocol
				tag, err := tx.Exec(ctx, sqlQuery)
				if err != nil {
					errMsg := fmt.Sprintf("%sSQL Query:\n%s\n\nError executing statement: %v", connectionMessage, sqlQuery, err)
					return mcp.NewToolError(errMsg)
				}
				commandTag = tag.String()
				rowsAffected = tag.RowsAffected()
			}

			// Check if results were truncated (we fetched limit+1 to detect this)
			wasTruncated := false
			if returnsRows && !hasExistingLimit && limit > 0 && len(results) > limit {
				wasTruncated = true
				results = results[:limit] // Truncate to requested limit
			}

			// Format results as TSV (tab-separated values) for row-returning queries
			resultsTSV := ""
			if returnsRows {
				resultsTSV = FormatResultsAsTSV(columnNames, results)
			}

			// Commit the transaction
			if err := tx.Commit(ctx); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to commit transaction: %v", err))
			}
			committed = true

			// Refresh metadata after DDL operations to keep schema info current
			if isDDLQuery && allowWrites {
				_ = dbClient.LoadMetadataFor(connStr) //nolint:errcheck // Best effort refresh
			}

			var sb strings.Builder

			// Always show current database context (unless already shown via connection message)
			if connectionMessage == "" {
				sanitizedConn := database.SanitizeConnStr(connStr)
				fmt.Fprintf(&sb, "Database: %s\n\n", sanitizedConn)
			} else {
				sb.WriteString(connectionMessage)
			}

			fmt.Fprintf(&sb, "SQL Query:\n%s\n\n", sqlQuery)

			if returnsRows {
				// Build the results header with pagination info
				if offset > 0 {
					// Show row range when using pagination
					startRow := offset + 1
					endRow := offset + len(results)
					if wasTruncated {
						fmt.Fprintf(&sb, "Results (rows %d-%d, more available - use offset=%d for next page):\n%s",
							startRow, endRow, offset+limit, resultsTSV)
					} else {
						fmt.Fprintf(&sb, "Results (rows %d-%d):\n%s", startRow, endRow, resultsTSV)
					}
				} else if wasTruncated {
					fmt.Fprintf(&sb, "Results (%d rows shown, more available - use offset=%d for next page or count_rows for total):\n%s",
						len(results), limit, resultsTSV)
				} else {
					fmt.Fprintf(&sb, "Results (%d rows):\n%s", len(results), resultsTSV)
				}
			} else {
				// Format output for DDL/DML statements
				fmt.Fprintf(&sb, "Statement executed successfully.\nCommand: %s", commandTag)
				if rowsAffected > 0 || isDMLQuery {
					fmt.Fprintf(&sb, "\nRows affected: %d", rowsAffected)
				}
			}

			// Log execution metrics
			logging.Info("query_database_executed",
				"query_length", len(sqlQuery),
				"rows_returned", len(results),
				"rows_affected", rowsAffected,
				"offset", offset,
				"was_truncated", wasTruncated,
				"returns_rows", returnsRows,
				"estimated_tokens", len(resultsTSV)/4,
			)

			return mcp.NewToolSuccess(sb.String())
		},
	}
}
