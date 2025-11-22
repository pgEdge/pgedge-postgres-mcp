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
	"regexp"
	"strings"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/logging"
	"pgedge-postgres-mcp/internal/mcp"
)

// ExecuteExplainTool creates the execute_explain tool for query performance analysis
func ExecuteExplainTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "execute_explain",
			Description: `Execute EXPLAIN ANALYZE on a query to diagnose performance.

<usecase>
Use when:
- Query runs slowly and you need to understand why
- Investigating performance bottlenecks
- Planning index creation to optimize queries
- Understanding query execution strategy
- Analyzing join performance and table scan methods
</usecase>

<what_it_returns>
Returns a detailed execution plan with:
- Query execution time and planning time
- Actual row counts vs estimated row counts
- Buffer usage (shared blocks read/hit)
- Sequential scans vs index scans
- Join methods and sort operations
- Analysis and recommendations for optimization
</what_it_returns>

<when_not_to_use>
DO NOT use for:
- Queries with side effects (INSERT, UPDATE, DELETE, DDL)
  → EXPLAIN ANALYZE will actually execute the query!
- Production workload analysis during peak hours
  → Can add overhead to the database
</when_not_to_use>

<examples>
✓ "Analyze why SELECT * FROM orders WHERE user_id = 123 is slow"
✓ "Explain the execution plan for my join query"
✓ "Why is this aggregation taking so long?"
✗ "Explain my INSERT statement" (will execute the insert!)
</examples>

<safety>
IMPORTANT: This tool executes the query with EXPLAIN ANALYZE within a
READ ONLY transaction to prevent side effects. However, be cautious with:
- Queries that lock resources
- Very long-running queries
- Queries on production systems during peak load
</safety>`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The SQL query to analyze (SELECT queries only)",
					},
					"analyze": map[string]interface{}{
						"type":        "boolean",
						"description": "Run EXPLAIN ANALYZE (executes query) vs plain EXPLAIN (planning only). Default: true",
						"default":     true,
					},
					"buffers": map[string]interface{}{
						"type":        "boolean",
						"description": "Include buffer usage statistics. Default: true",
						"default":     true,
					},
					"format": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"text", "json"},
						"description": "Output format: 'text' for human-readable (default), 'json' for structured data",
						"default":     "text",
					},
				},
				Required: []string{"query"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Extract and validate parameters
			query, ok := args["query"].(string)
			if !ok || query == "" {
				return mcp.NewToolError("Parameter 'query' is required and must be a non-empty string")
			}

			// Defaults
			analyze := true
			buffers := true
			format := "text"

			if val, ok := args["analyze"].(bool); ok {
				analyze = val
			}
			if val, ok := args["buffers"].(bool); ok {
				buffers = val
			}
			if val, ok := args["format"].(string); ok {
				format = val
			}

			// Validate query is a SELECT
			trimmedQuery := strings.TrimSpace(query)
			if !strings.HasPrefix(strings.ToUpper(trimmedQuery), "SELECT") {
				return mcp.NewToolError("Only SELECT queries are supported. EXPLAIN ANALYZE executes the query, which could have side effects for INSERT/UPDATE/DELETE/DDL statements.")
			}

			// Build EXPLAIN command
			var explainCmd strings.Builder
			explainCmd.WriteString("EXPLAIN (")

			var options []string
			if analyze {
				options = append(options, "ANALYZE TRUE")
			}
			if buffers {
				options = append(options, "BUFFERS TRUE")
			}
			if format == "json" {
				options = append(options, "FORMAT JSON")
			}

			explainCmd.WriteString(strings.Join(options, ", "))
			explainCmd.WriteString(") ")
			explainCmd.WriteString(query)

			explainQuery := explainCmd.String()

			// Get database connection
			connStr := dbClient.GetDefaultConnection()
			pool := dbClient.GetPoolFor(connStr)

			ctx := context.Background()

			// Execute EXPLAIN in a READ ONLY transaction
			tx, err := pool.Begin(ctx)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to begin transaction: %v", err))
			}

			committed := false
			defer func() {
				if !committed {
					_ = tx.Rollback(ctx) //nolint:errcheck // rollback in defer after commit is expected to fail
				}
			}()

			// Set transaction to read-only
			_, err = tx.Exec(ctx, "SET TRANSACTION READ ONLY")
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to set transaction to read-only: %v", err))
			}

			// Execute EXPLAIN
			rows, err := tx.Query(ctx, explainQuery)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error executing EXPLAIN: %v\n\nQuery: %s", err, explainQuery))
			}
			defer rows.Close()

			// Collect EXPLAIN output
			var explainOutput []string
			for rows.Next() {
				var line string
				if err := rows.Scan(&line); err != nil {
					return mcp.NewToolError(fmt.Sprintf("Error reading EXPLAIN output: %v", err))
				}
				explainOutput = append(explainOutput, line)
			}

			if err := rows.Err(); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error iterating EXPLAIN output: %v", err))
			}

			// Commit the read-only transaction
			if err := tx.Commit(ctx); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to commit transaction: %v", err))
			}
			committed = true

			// Format the output
			var result strings.Builder
			sanitizedConn := database.SanitizeConnStr(connStr)
			result.WriteString(fmt.Sprintf("Database: %s\n\n", sanitizedConn))
			result.WriteString(fmt.Sprintf("Query:\n%s\n\n", query))
			result.WriteString("Execution Plan:\n")
			result.WriteString(strings.Repeat("=", 80))
			result.WriteString("\n")

			explainText := strings.Join(explainOutput, "\n")
			result.WriteString(explainText)
			result.WriteString("\n")
			result.WriteString(strings.Repeat("=", 80))
			result.WriteString("\n\n")

			// Add analysis and recommendations if format is text and we have ANALYZE data
			if format == "text" && analyze {
				analysis := analyzeExplainOutput(explainText)
				if analysis != "" {
					result.WriteString("Analysis:\n")
					result.WriteString(analysis)
				}
			}

			// Log execution metrics
			logging.Info("execute_explain_executed",
				"query_length", len(query),
				"analyze", analyze,
				"buffers", buffers,
				"format", format,
				"output_lines", len(explainOutput),
			)

			return mcp.NewToolSuccess(result.String())
		},
	}
}

// analyzeExplainOutput extracts key metrics and provides recommendations
func analyzeExplainOutput(explainText string) string {
	var analysis strings.Builder
	issues := []string{}
	recommendations := []string{}

	// Check for sequential scans
	if strings.Contains(explainText, "Seq Scan") {
		seqScanRegex := regexp.MustCompile(`Seq Scan on (\w+)`)
		matches := seqScanRegex.FindAllStringSubmatch(explainText, -1)
		if len(matches) > 0 {
			tables := []string{}
			for _, match := range matches {
				if len(match) > 1 {
					tables = append(tables, match[1])
				}
			}
			if len(tables) > 0 {
				issues = append(issues, fmt.Sprintf("⚠️  Sequential scan(s) detected on: %s", strings.Join(tables, ", ")))
				recommendations = append(recommendations, "Consider adding indexes on frequently filtered columns")
			}
		}
	}

	// Check for high actual time
	timeRegex := regexp.MustCompile(`actual time=[\d.]+\.\.(\d+\.\d+)`)
	matches := timeRegex.FindAllStringSubmatch(explainText, -1)
	if len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				// Parse the time (simplified check)
				if strings.Contains(match[1], "000.") {
					issues = append(issues, "⚠️  Query execution took over 1 second")
					break
				}
			}
		}
	}

	// Check for hash joins with large memory usage
	if strings.Contains(explainText, "Hash Join") {
		issues = append(issues, "ℹ️  Hash join detected")
		if strings.Contains(explainText, "Batches:") {
			recommendations = append(recommendations, "Consider increasing work_mem if hash join is spilling to disk")
		}
	}

	// Check for sorts
	if strings.Contains(explainText, "Sort") {
		if strings.Contains(explainText, "external") || strings.Contains(explainText, "Disk:") {
			issues = append(issues, "⚠️  Sort operation spilling to disk")
			recommendations = append(recommendations, "Consider increasing work_mem or adding an index to avoid sorting")
		}
	}

	// Check for high buffer reads vs hits
	readRegex := regexp.MustCompile(`read=(\d+)`)
	hitRegex := regexp.MustCompile(`hit=(\d+)`)

	reads := readRegex.FindAllStringSubmatch(explainText, -1)
	hits := hitRegex.FindAllStringSubmatch(explainText, -1)

	if len(reads) > 0 && len(hits) > 0 {
		// If we have reads and hits, check the ratio
		if strings.Contains(explainText, "read=") {
			issues = append(issues, "ℹ️  Some blocks read from disk (not cached)")
			recommendations = append(recommendations, "Consider increasing shared_buffers or running the query again to warm the cache")
		}
	}

	// Build analysis output
	if len(issues) > 0 {
		analysis.WriteString("<issues>\n")
		for _, issue := range issues {
			analysis.WriteString(fmt.Sprintf("%s\n", issue))
		}
		analysis.WriteString("</issues>\n\n")
	}

	if len(recommendations) > 0 {
		analysis.WriteString("<recommendations>\n")
		for i, rec := range recommendations {
			analysis.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
		}
		analysis.WriteString("</recommendations>\n")
	}

	return analysis.String()
}
