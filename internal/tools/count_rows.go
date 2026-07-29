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
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/logging"
	"pgedge-postgres-mcp/internal/mcp"
)

// CountRowsTool creates the count_rows tool for lightweight row counting
func CountRowsTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "count_rows",
			Description: `Get the row count of a table with optional filtering. Use this tool instead of running SELECT COUNT(*) through psql or shell commands — it provides efficient counting with proper access control.

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
- WHERE clause must be a simple filter predicate against the named table;
  subqueries are rejected. Use query_database for anything that needs one
- Returns a single integer count - minimal token usage
</important>`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]any{
					"table": map[string]any{
						"type":        "string",
						"description": "Name of the table to count rows from",
					},
					"schema": map[string]any{
						"type":        "string",
						"description": "Schema name (default: public)",
						"default":     "public",
					},
					"where": map[string]any{
						"type":        "string",
						"description": "Optional WHERE clause condition (without the WHERE keyword). Must be a simple filter predicate; subqueries are rejected. Example: \"status = 'active' AND created_at > '2024-01-01'\"",
					},
				},
				Required: []string{"table"},
			},
		},
		Handler: func(args map[string]any) (mcp.ToolResponse, error) {
			table, ok := args["table"].(string)
			if !ok || table == "" {
				return mcp.NewToolError("Missing or invalid 'table' parameter")
			}

			// Get schema, default to public
			schema := "public"
			if s, ok := args["schema"].(string); ok && s != "" {
				schema = s
			}

			// Get optional WHERE clause. The condition is interpolated into
			// the count query, so it is screened with the same guard as any
			// other caller-supplied SQL. This tool always counts inside a
			// read-only transaction, so the guard applies whether or not the
			// connection permits writes elsewhere.
			//
			// validateReadOnlyQuery only recognises constructs that escape
			// read-only mode; an arbitrary boolean subquery is ordinary
			// legal SQL and is invisible to it. count_rows's where is
			// documented and intended as a simple predicate against the
			// named table, so validateCountRowsWhereClause additionally
			// rejects a subquery, which closes the cross-table blind
			// injection vector reported in issue #200: without it, a where
			// clause let a caller run a boolean- or error-based oracle
			// against any table the connected role can read, not just the
			// one named in the call. query_database and execute_explain are
			// unaffected; their entire purpose is running arbitrary SQL,
			// subqueries included.
			whereClause := ""
			if w, ok := args["where"].(string); ok && w != "" {
				if err := validateReadOnlyQuery(w); err != nil {
					logging.Warn("read_only_query_rejected",
						"database", dbClient.DisplayName(),
						"tool", "count_rows",
						"reason", err.Error(),
						"statement", w,
					)
					return mcp.NewToolError(err.Error())
				}
				if err := validateCountRowsWhereClause(w); err != nil {
					logging.Warn("count_rows_where_clause_rejected",
						"database", dbClient.DisplayName(),
						"tool", "count_rows",
						"reason", err.Error(),
						"statement", w,
					)
					return mcp.NewToolError(err.Error())
				}
				whereClause = w
			}

			// Get connection
			connStr := dbClient.GetDefaultConnection()
			if !dbClient.IsMetadataLoadedFor(connStr) {
				if err := dbClient.LoadMetadataFor(connStr); err != nil {
					return mcp.NewToolError(fmt.Sprintf("Failed to load database metadata: %v", err))
				}
			}

			pool := dbClient.GetPoolFor(connStr)
			if pool == nil {
				return mcp.NewToolError(fmt.Sprintf("Connection pool not found for: %s", dbClient.DisplayName()))
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

			// Execute in a read-only transaction. The access mode is set on
			// the BEGIN itself rather than by a following statement, so there
			// is no window in which the transaction is writable.
			ctx := context.Background()
			tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
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
			fmt.Fprintf(&sb, "Database: %s\n\n", dbClient.DisplayName())
			fmt.Fprintf(&sb, "SQL Query:\n%s\n\n", sqlQuery)
			fmt.Fprintf(&sb, "Count: %d", count)

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

// subqueryPattern matches a bare SELECT or TABLE keyword — the two
// constructs PostgreSQL's grammar allows to start a row-returning
// "simple_select" that can appear as a parenthesised subquery. TABLE
// tablename is shorthand for SELECT * FROM tablename and reads exactly the
// same data; a where clause of "(TABLE other_table) = value" leaks another
// table's contents through the same boolean oracle as "(SELECT ... FROM
// other_table) = value" without the word SELECT ever appearing. VALUES is
// the grammar's third alternative but is deliberately not blocked here: it
// cannot itself reference a table (VALUES (...) admits no FROM clause, so
// it can only construct a literal row, not read one), and unlike SELECT and
// TABLE it is not a fully reserved keyword in PostgreSQL — it can
// legitimately be an unquoted column name — so banning it would risk
// rejecting a genuine predicate for no security benefit.
//
// Checked against the comment-stripped, literal-blanked residue produced
// by stripSQLNoise, so a literal string such as 'select this row' cannot
// trigger a false rejection, and a comment cannot be used to split either
// keyword to slip past.
var subqueryPattern = regexp.MustCompile(`(?i)\b(SELECT|TABLE)\b`)

// validateCountRowsWhereClause rejects a where clause containing a
// subquery.
//
// count_rows's where parameter is documented as a simple filter predicate
// against the named table (see the tool's own examples: "status = 'active'
// AND created_at > '2024-01-01'"); no legitimate use of it needs a
// subquery. Without this check, a subquery in where lets a caller run a
// boolean- or error-based blind-injection oracle against any table the
// connected role can read, not only the table named in the call — see
// issue #200. As with the DO-block and set_config() rejections in
// readonly_guard.go, no fixed pattern can separate a "safe" subquery from a
// dangerous one, so every subquery is refused outright rather than trying
// to recognise which ones are dangerous. query_database and
// execute_explain do not call this: their entire purpose is running
// arbitrary SQL, subqueries included, and validateReadOnlyQuery is the
// only guard applicable there.
func validateCountRowsWhereClause(where string) error {
	residue, _ := stripSQLNoise(where)
	if subqueryPattern.MatchString(residue) {
		return fmt.Errorf(
			"where clause rejected: subqueries are not permitted; " +
				"count_rows only accepts a simple filter predicate " +
				"against the named table (use query_database for " +
				"anything that needs a subquery)")
	}
	return nil
}
