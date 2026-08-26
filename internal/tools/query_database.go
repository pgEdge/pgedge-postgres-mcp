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
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/logging"
	"pgedge-postgres-mcp/internal/mcp"
	"pgedge-postgres-mcp/internal/sqltext"
)

// stripTrailingSemicolons removes trailing semicolons and whitespace from
// a SQL query so that LIMIT/OFFSET clauses can be safely appended.
func stripTrailingSemicolons(query string) string {
	return strings.TrimRightFunc(query, func(r rune) bool {
		return r == ';' || unicode.IsSpace(r)
	})
}

// defaultRowLimit, minRowLimit and maxRowLimit mirror the bounds the
// query_database tool's "limit" argument advertises in its schema. The
// schema is a client-side hint only; these constants are what the server
// actually enforces.
const (
	defaultRowLimit = 100
	minRowLimit     = 1
	maxRowLimit     = 1000
)

// resolveRowLimit extracts the caller-supplied "limit" argument and clamps
// it to the tool's advertised bounds. A missing, non-numeric, or
// out-of-range value (including zero or negative, which previously
// disabled the safety cap entirely rather than falling back to it - issue
// #273) falls back to defaultRowLimit instead of removing the cap.
func resolveRowLimit(args map[string]any) int {
	limitVal, ok := args["limit"]
	if !ok {
		return defaultRowLimit
	}

	var limit int
	switch v := limitVal.(type) {
	case float64:
		// The schema declares "limit" as an integer; a fractional,
		// infinite, or NaN value violates that contract outright rather
		// than rounding to something that happens to look in-bounds.
		// The range check runs before int(v): a magnitude beyond what
		// int can represent (e.g. math.MaxFloat64) makes that conversion
		// implementation-dependent per the Go spec, so it must be
		// rejected on the float64 value itself, not after converting.
		if v != math.Trunc(v) || math.IsInf(v, 0) ||
			v < float64(minRowLimit) || v > float64(maxRowLimit) {
			return defaultRowLimit
		}
		limit = int(v)
	case int:
		limit = v
	default:
		return defaultRowLimit
	}

	if limit < minRowLimit || limit > maxRowLimit {
		return defaultRowLimit
	}
	return limit
}

// limitKeywordPattern and offsetKeywordPattern match a LIMIT/OFFSET clause
// keyword bounded by characters that can't appear in a PostgreSQL unquoted
// identifier, so that "credit_limit" or "foo$limit" (the dollar sign is a
// legal identifier character after the first position) doesn't count as a
// clause. Go's regexp package has no lookaround, so the boundary characters
// are captured alongside the keyword and excluded via the submatch index.
var limitKeywordPattern = regexp.MustCompile(`(?i)(?:^|[^A-Z0-9_$])(LIMIT)(?:[^A-Z0-9_$]|$)`)
var offsetKeywordPattern = regexp.MustCompile(`(?i)(?:^|[^A-Z0-9_$])(OFFSET)(?:[^A-Z0-9_$]|$)`)

// fetchFirstPattern matches the SQL-standard row-limiting clause
// "FETCH { FIRST | NEXT } [ count ] { ROW | ROWS } { ONLY | WITH TIES }",
// which contains no occurrence of the word "LIMIT" at all. Appending our
// own LIMIT on top of one gives PostgreSQL two row-limiting clauses on
// the same statement, which it rejects outright (issue #276), so this
// needs its own detection rather than falling out of limitKeywordPattern.
// Only the "FETCH" keyword itself is captured, matching the convention
// the other patterns use for the parenthesis-depth check in
// queryHasClause; [^;]*? keeps the match from running past the end of
// the statement it belongs to.
//
// WITH TIES is a distinct, real terminator from ONLY -- PostgreSQL
// accepts "FETCH FIRST n ROWS WITH TIES" to include rows tied with the
// last one on the ORDER BY key, and rejects a LIMIT appended after it
// exactly the same way it rejects one after ROWS ONLY, so both need to
// count as an existing limit (issue #276 follow-up). PostgreSQL has no
// PERCENT variant of this clause at all -- unlike WITH TIES, that's not
// a gap to close, since Postgres's own parser already rejects it before
// this tool's LIMIT injection ever becomes relevant.
var fetchFirstPattern = regexp.MustCompile(`(?is)(?:^|[^A-Z0-9_$])(FETCH)\s+(?:FIRST|NEXT)\b[^;]*?\b(?:ROW|ROWS)\s+(?:ONLY|WITH\s+TIES)\b`)

// stripLeadingParens peels away wrapping parentheses so a statement written
// as "(SELECT ...)" is still recognised as a SELECT by the keyword-prefix
// checks below (issue #275). It only ever narrows what's inspected for
// classification purposes; the query actually executed is untouched.
func stripLeadingParens(s string) string {
	for strings.HasPrefix(s, "(") {
		depth := 0
		closeAt := -1
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					closeAt = i
				}
			}
			if closeAt >= 0 {
				break
			}
		}
		if closeAt < 0 {
			break // unbalanced; leave as-is for the prefix check to reject
		}
		inner := strings.TrimSpace(s[1:closeAt])
		if inner == "" {
			break
		}
		s = inner
	}
	return s
}

// normalizeForClassification produces the text used to decide a
// statement's type (SELECT/DDL/DML/RETURNING): comments and literals
// stripped via sqltext.Strip, any wrapping parentheses peeled off, upper-
// cased. Without this a leading comment or a parenthesised SELECT falls
// through every HasPrefix check below, which meant no LIMIT was appended
// and no truncation marker was ever emitted for it (issue #275).
func normalizeForClassification(sqlQuery string) string {
	residue, _ := sqltext.Strip(sqlQuery)
	return strings.ToUpper(stripLeadingParens(strings.TrimSpace(residue)))
}

// redundantWrapDepth reports how many layers of parentheses wrap the
// ENTIRE statement -- e.g. 2 for "((SELECT ...))" but 0 for
// "(SELECT ...) UNION (SELECT ...)", where the first '(' closes well
// before the end of the string rather than at it. Unlike
// stripLeadingParens, this never discards any text: it only measures how
// many redundant layers exist, since a caller needs the untouched
// residue to still search it (issue #275 follow-up).
//
// This distinction matters because such a wrap contributes nothing to
// the statement's meaning: "(SELECT ... LIMIT 5)" behaves exactly like
// "SELECT ... LIMIT 5" as a complete statement, so a clause inside it is
// just as much "top level" as one with no wrapping at all.
func redundantWrapDepth(s string) int {
	s = strings.TrimSpace(s)
	depth := 0
	for strings.HasPrefix(s, "(") {
		matchDepth := 0
		closeAt := -1
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case '(':
				matchDepth++
			case ')':
				matchDepth--
				if matchDepth == 0 {
					closeAt = i
				}
			}
			if closeAt >= 0 {
				break
			}
		}
		// Only count this layer if its closing paren is the last
		// non-whitespace character in the string -- otherwise something
		// follows it (a UNION, a trailing LIMIT, ...) and the wrap isn't
		// whole-statement, so matches inside it stay at their real depth.
		if closeAt < 0 || strings.TrimSpace(s[closeAt+1:]) != "" {
			break
		}
		inner := strings.TrimSpace(s[1:closeAt])
		if inner == "" {
			break
		}
		depth++
		s = inner
	}
	return depth
}

// queryHasClause reports whether a SQL statement already contains a
// top-level LIMIT or OFFSET clause. It checks the statement's residue, with
// comments removed and string literals, dollar-quoted blocks and quoted
// identifiers replaced by placeholders, so a caller can't defeat the safety
// cap by mentioning "limit" or "offset" in a string literal, a quoted
// column alias, or a comment (issue #260).
//
// A match only counts at the statement's effective top level: parenthesis
// depth zero for an unwrapped statement, so a LIMIT/OFFSET that belongs to
// a subquery or CTE, such as
// "SELECT * FROM t WHERE id IN (SELECT id FROM u LIMIT 1)", isn't mistaken
// for a clause on the outer statement; or redundantWrapDepth deeper when
// the whole statement is itself wrapped in parentheses, such as
// "(SELECT * FROM t LIMIT 5)", so that case isn't mistaken for having no
// top-level clause at all and getting a second one appended alongside it
// (issue #275 follow-up).
func queryHasClause(pattern *regexp.Regexp, sqlQuery string) bool {
	residue, _ := sqltext.Strip(sqlQuery)
	topLevelDepth := redundantWrapDepth(residue)
	for _, loc := range pattern.FindAllStringSubmatchIndex(residue, -1) {
		// loc[2] and loc[3] bound the captured keyword itself, excluding
		// the boundary characters matched alongside it.
		if parenDepthAt(residue, loc[2]) == topLevelDepth {
			return true
		}
	}
	return false
}

// parenDepthAt returns the parenthesis nesting depth at byte offset pos in
// s, counting unmatched '(' seen before it.
func parenDepthAt(s string, pos int) int {
	depth := 0
	for i := 0; i < pos; i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth
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
					// Log the rejected statement in full. A rejection here is
					// an attempt to escape read-only mode, and the statement
					// text is the only useful evidence of what was tried.
					logging.Warn("read_only_query_rejected",
						"database", dbClient.DisplayName(),
						"reason", err.Error(),
						"statement", sqlQuery,
					)
					return mcp.NewToolError(err.Error())
				}
			}

			// Determine the limit to use, clamped to the schema's advertised
			// bounds so an out-of-range value falls back to the default
			// rather than disabling the cap (issue #273).
			limit := resolveRowLimit(args)

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

			// Track if query already had LIMIT/OFFSET clauses, or the
			// SQL-standard FETCH FIRST/NEXT ... ROWS ONLY equivalent
			// (issue #276). Checked against the statement's residue rather
			// than raw text, so a literal or identifier that merely
			// mentions "limit"/"offset" doesn't defeat the safety cap
			// (issue #260).
			hasExistingLimit := queryHasClause(limitKeywordPattern, sqlQuery) ||
				queryHasClause(fetchFirstPattern, sqlQuery)
			hasExistingOffset := queryHasClause(offsetKeywordPattern, sqlQuery)

			upperQuery := normalizeForClassification(sqlQuery)

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

			// Truncation is only detectable for the statements that had a
			// limit+1 appended below, so it is keyed on the classification
			// rather than on what the server turned out to return.
			truncationDetectable := isSelectQuery

			// An inline LIMIT the caller wrote themselves is honoured
			// verbatim and was previously the one path with no enforced
			// ceiling at all (issue #273): a query ending "LIMIT 30000"
			// bypasses the "limit" argument entirely. Wrap the whole
			// statement so the cap holds regardless of where the LIMIT
			// clause came from; the inner LIMIT still narrows the result as
			// written, it just can no longer widen it past maxRowLimit.
			rowCapApplied := false
			effectiveLimit := limit
			if isSelectQuery && hasExistingLimit {
				sqlQuery = fmt.Sprintf("SELECT * FROM (%s) AS pgedge_mcp_row_cap LIMIT %d", sqlQuery, maxRowLimit+1)
				rowCapApplied = true
				effectiveLimit = maxRowLimit
			}

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
				// An ad-hoc connection string the caller supplied inline is
				// echoed back sanitized (they already know what they typed);
				// otherwise show the configured display name, never the raw
				// default connection's host (issue #187).
				display := dbClient.DisplayName()
				if queryCtx.ConnectionString != "" {
					display = database.SanitizeConnStr(connStr)
				}
				return mcp.NewToolError(fmt.Sprintf("Connection pool not found for: %s", display))
			}

			// Begin the transaction with its access mode set on the BEGIN
			// itself. Issuing SET TRANSACTION READ ONLY as a separate
			// statement afterwards leaves a window in which the transaction
			// exists but is still read-write, and relies on that statement
			// succeeding; requesting the mode up front does neither.
			txOptions := pgx.TxOptions{}
			if !allowWrites {
				txOptions.AccessMode = pgx.ReadOnly
			}
			tx, err := pool.BeginTx(ctx, txOptions)
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

			// Execute the statement using the appropriate method based on whether it returns rows
			var columnNames []string
			var results [][]any
			var commandTag string
			var rowsAffected int64

			// Decide which pgx entry point runs the caller's SQL.
			//
			// Query() always uses the extended query protocol, which carries
			// exactly one statement per message: PostgreSQL rejects anything
			// else with "cannot insert multiple commands into a prepared
			// statement". Exec() is different. When it is given no bind
			// parameters pgx falls back to the simple query protocol
			// unconditionally, and that protocol accepts any number of
			// semicolon-separated statements in one message. A caller could
			// therefore append their own statements after the one the tool
			// meant to run, including statements that alter the transaction
			// access mode before a write.
			//
			// So on a read-only connection every statement goes through
			// Query(), which removes that channel entirely rather than trying
			// to recognise the payloads that use it. Where writes are
			// explicitly enabled there is no boundary left to protect, and
			// Exec() is kept so that multi-statement scripts still work.
			useExtendedProtocol := returnsRows || !allowWrites

			if useExtendedProtocol {
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

				// Collect results as array of arrays for TSV formatting. A
				// statement that returns nothing simply yields no rows here.
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

				// Close rows before commit to ensure statement is fully
				// processed; the command tag is only complete afterwards.
				rows.Close()
				tag := rows.CommandTag()
				commandTag = tag.String()
				rowsAffected = tag.RowsAffected()

				// Whether to present a result set is settled by what the
				// server actually described, which is more reliable than
				// inferring it from the statement's leading keyword. This
				// matters for statements that return rows without starting
				// with SELECT, such as SHOW and EXPLAIN, whose output was
				// previously discarded in favour of the command tag.
				returnsRows = len(columnNames) > 0
			} else {
				tag, err := tx.Exec(ctx, sqlQuery)
				if err != nil {
					errMsg := fmt.Sprintf("%sSQL Query:\n%s\n\nError executing statement: %v", connectionMessage, sqlQuery, err)
					return mcp.NewToolError(errMsg)
				}
				commandTag = tag.String()
				rowsAffected = tag.RowsAffected()
			}

			// Check if results were truncated (we fetched limit+1, or the
			// hard row-cap ceiling+1, to detect this)
			wasTruncated := false
			if truncationDetectable && (rowCapApplied || !hasExistingLimit) && len(results) > effectiveLimit {
				wasTruncated = true
				results = results[:effectiveLimit] // Truncate to requested/effective limit
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
				fmt.Fprintf(&sb, "Database: %s\n\n", dbClient.DisplayName())
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
							startRow, endRow, offset+effectiveLimit, resultsTSV)
					} else {
						fmt.Fprintf(&sb, "Results (rows %d-%d):\n%s", startRow, endRow, resultsTSV)
					}
				} else if wasTruncated {
					fmt.Fprintf(&sb, "Results (%d rows shown, more available - use offset=%d for next page or count_rows for total):\n%s",
						len(results), effectiveLimit, resultsTSV)
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
