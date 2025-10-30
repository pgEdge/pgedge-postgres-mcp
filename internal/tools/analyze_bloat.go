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
	"fmt"
	"strings"
	"time"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// TableBloat represents bloat statistics for a database table
type TableBloat struct {
	SchemaName       string
	TableName        string
	LiveTuples       int64
	DeadTuples       int64
	DeadTuplePercent float64
	TotalSize        string
	TotalSizeBytes   int64
	LastVacuum       *time.Time
	LastAutovacuum   *time.Time
	LastAnalyze      *time.Time
	LastAutoanalyze  *time.Time
	Inserts          int64
	Updates          int64
	Deletes          int64
	ModsSinceAnalyze int64
	VacuumCount      int64
	AutovacuumCount  int64
	AnalyzeCount     int64
	AutoanalyzeCount int64
	Recommendations  []string
}

// AnalyzeBloatTool creates a tool for analyzing table and index bloat
func AnalyzeBloatTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "analyze_bloat",
			Description: "Analyzes tables and indexes for bloat (wasted space from dead tuples and fragmentation). Returns statistics about dead tuples, last vacuum/analyze times, and recommendations for maintenance operations like VACUUM, VACUUM FULL, REINDEX, or ANALYZE. Helps identify tables that need maintenance to reclaim space and improve performance.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"schema_name": map[string]interface{}{
						"type":        "string",
						"description": "Optional: Filter by specific schema name (e.g., 'public'). If not provided, analyzes all schemas.",
					},
					"table_name": map[string]interface{}{
						"type":        "string",
						"description": "Optional: Filter by specific table name. Requires schema_name to be provided. If not provided, analyzes all tables in the schema(s).",
					},
					"min_dead_tuple_percent": map[string]interface{}{
						"type":        "number",
						"description": "Optional: Minimum percentage of dead tuples to include in results (0-100). Default: 5. Use lower values to see all tables, higher values to focus on most bloated.",
						"default":     5.0,
					},
					"include_indexes": map[string]interface{}{
						"type":        "boolean",
						"description": "Optional: Include index bloat analysis. Default: true. Set to false for faster results if only table bloat is needed.",
						"default":     true,
					},
				},
				Required: []string{},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Validate parameters
			schemaName := ValidateOptionalStringParam(args, "schema_name", "")
			tableName := ValidateOptionalStringParam(args, "table_name", "")
			minDeadTuplePercent := ValidateOptionalNumberParam(args, "min_dead_tuple_percent", 5.0)
			includeIndexes := ValidateBoolParam(args, "include_indexes", true)

			// Validate constraints
			if tableName != "" && schemaName == "" {
				return mcp.NewToolError("Error: table_name requires schema_name to be specified")
			}

			if minDeadTuplePercent < 0 || minDeadTuplePercent > 100 {
				return mcp.NewToolError("Error: min_dead_tuple_percent must be between 0 and 100")
			}

			// Check if database is ready
			if !dbClient.IsMetadataLoaded() {
				return mcp.NewToolError(mcp.DatabaseNotReadyError)
			}

			// Get connection pool
			pool := dbClient.GetPool()
			if pool == nil {
				return mcp.NewToolError("Database connection not available")
			}

			ctx := context.Background()

			// Build WHERE clause for filtering
			var whereConditions []string
			var queryParams []interface{}
			paramCount := 0

			if schemaName != "" {
				paramCount++
				whereConditions = append(whereConditions, fmt.Sprintf("schemaname = $%d", paramCount))
				queryParams = append(queryParams, schemaName)
			}

			if tableName != "" {
				paramCount++
				whereConditions = append(whereConditions, fmt.Sprintf("relname = $%d", paramCount))
				queryParams = append(queryParams, tableName)
			}

			whereClause := ""
			if len(whereConditions) > 0 {
				whereClause = "WHERE " + strings.Join(whereConditions, " AND ") + " AND "
			} else {
				whereClause = "WHERE "
			}

			// Query for table bloat analysis
			query := `
				SELECT
					schemaname,
					relname,
					n_live_tup,
					n_dead_tup,
					CASE
						WHEN n_live_tup > 0 THEN
							ROUND(100.0 * n_dead_tup / (n_live_tup + n_dead_tup), 2)
						ELSE 0
					END as dead_tuple_percent,
					pg_size_pretty(pg_total_relation_size(schemaname||'.'||relname)) as total_size,
					pg_total_relation_size(schemaname||'.'||relname) as total_size_bytes,
					last_vacuum,
					last_autovacuum,
					last_analyze,
					last_autoanalyze,
					n_tup_ins,
					n_tup_upd,
					n_tup_del,
					n_mod_since_analyze,
					vacuum_count,
					autovacuum_count,
					analyze_count,
					autoanalyze_count
				FROM pg_stat_user_tables
				` + whereClause + `
					n_live_tup + n_dead_tup > 0
				ORDER BY
					CASE
						WHEN n_live_tup > 0 THEN
							100.0 * n_dead_tup / (n_live_tup + n_dead_tup)
						ELSE 0
					END DESC,
					pg_total_relation_size(schemaname||'.'||relname) DESC
			`

			rows, err := pool.Query(ctx, query, queryParams...)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to query table bloat: %v", err))
			}
			defer rows.Close()

			var tables []TableBloat

			for rows.Next() {
				var t TableBloat
				err := rows.Scan(
					&t.SchemaName, &t.TableName,
					&t.LiveTuples, &t.DeadTuples, &t.DeadTuplePercent,
					&t.TotalSize, &t.TotalSizeBytes,
					&t.LastVacuum, &t.LastAutovacuum,
					&t.LastAnalyze, &t.LastAutoanalyze,
					&t.Inserts, &t.Updates, &t.Deletes,
					&t.ModsSinceAnalyze,
					&t.VacuumCount, &t.AutovacuumCount,
					&t.AnalyzeCount, &t.AutoanalyzeCount,
				)
				if err != nil {
					return mcp.NewToolError(fmt.Sprintf("Error reading bloat data: %v", err))
				}

				// Filter by minimum dead tuple percentage
				if t.DeadTuplePercent < minDeadTuplePercent {
					continue
				}

				// Generate recommendations
				t.Recommendations = generateRecommendations(t)

				tables = append(tables, t)
			}

			if err := rows.Err(); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error iterating bloat data: %v", err))
			}

			// Build output
			var output strings.Builder
			output.WriteString("Table Bloat Analysis\n")
			output.WriteString("====================\n\n")

			if len(tables) == 0 {
				output.WriteString(fmt.Sprintf("No tables found with dead tuple percentage >= %.1f%%\n", minDeadTuplePercent))
				if schemaName != "" {
					output.WriteString(fmt.Sprintf("Schema filter: %s\n", schemaName))
				}
				if tableName != "" {
					output.WriteString(fmt.Sprintf("Table filter: %s\n", tableName))
				}
				return mcp.NewToolSuccess(output.String())
			}

			output.WriteString(fmt.Sprintf("Found %d table(s) with bloat >= %.1f%%\n\n", len(tables), minDeadTuplePercent))

			for i, t := range tables {
				output.WriteString(fmt.Sprintf("%d. %s.%s\n", i+1, t.SchemaName, t.TableName))
				output.WriteString(fmt.Sprintf("   Size: %s (%d bytes)\n", t.TotalSize, t.TotalSizeBytes))
				output.WriteString(fmt.Sprintf("   Live tuples: %d\n", t.LiveTuples))
				output.WriteString(fmt.Sprintf("   Dead tuples: %d (%.2f%% of total)\n", t.DeadTuples, t.DeadTuplePercent))
				output.WriteString(fmt.Sprintf("   Modifications since analyze: %d\n", t.ModsSinceAnalyze))

				// Vacuum information
				output.WriteString("   Last maintenance:\n")
				if t.LastVacuum != nil {
					output.WriteString(fmt.Sprintf("     - Manual VACUUM: %s\n", t.LastVacuum.Format(time.RFC3339)))
				}
				if t.LastAutovacuum != nil {
					output.WriteString(fmt.Sprintf("     - Autovacuum: %s\n", t.LastAutovacuum.Format(time.RFC3339)))
				}
				if t.LastAnalyze != nil {
					output.WriteString(fmt.Sprintf("     - Manual ANALYZE: %s\n", t.LastAnalyze.Format(time.RFC3339)))
				}
				if t.LastAutoanalyze != nil {
					output.WriteString(fmt.Sprintf("     - Autoanalyze: %s\n", t.LastAutoanalyze.Format(time.RFC3339)))
				}

				if t.LastVacuum == nil && t.LastAutovacuum == nil {
					output.WriteString("     - Never vacuumed\n")
				}
				if t.LastAnalyze == nil && t.LastAutoanalyze == nil {
					output.WriteString("     - Never analyzed\n")
				}

				// Statistics
				output.WriteString("   Lifetime stats:\n")
				output.WriteString(fmt.Sprintf("     - Inserts: %d, Updates: %d, Deletes: %d\n", t.Inserts, t.Updates, t.Deletes))
				output.WriteString(fmt.Sprintf("     - Vacuums: %d manual + %d auto\n", t.VacuumCount, t.AutovacuumCount))
				output.WriteString(fmt.Sprintf("     - Analyzes: %d manual + %d auto\n", t.AnalyzeCount, t.AutoanalyzeCount))

				// Recommendations
				if len(t.Recommendations) > 0 {
					output.WriteString("   Recommendations:\n")
					for _, rec := range t.Recommendations {
						output.WriteString(fmt.Sprintf("     - %s\n", rec))
					}
				}
				output.WriteString("\n")
			}

			// Add index analysis if requested
			if includeIndexes {
				indexOutput := analyzeIndexBloat(ctx, dbClient, schemaName, tableName)
				output.WriteString(indexOutput)
			}

			// Add general advice
			output.WriteString("General Maintenance Tips:\n")
			output.WriteString("========================\n")
			output.WriteString("- VACUUM reclaims space from dead tuples but doesn't shrink table files\n")
			output.WriteString("- VACUUM FULL rebuilds the table and reclaims disk space (requires table lock)\n")
			output.WriteString("- ANALYZE updates query planner statistics\n")
			output.WriteString("- REINDEX rebuilds indexes to remove bloat\n")
			output.WriteString("- Consider adjusting autovacuum settings if bloat is consistently high\n")
			output.WriteString("- Monitor pg_stat_user_tables and pg_stat_bgwriter for vacuum activity\n")

			return mcp.NewToolSuccess(output.String())
		},
	}
}

// generateRecommendations creates maintenance recommendations based on bloat statistics
func generateRecommendations(t TableBloat) []string {
	var recs []string

	// Dead tuple recommendations
	switch {
	case t.DeadTuplePercent > 20:
		recs = append(recs, "⚠️  URGENT: Run VACUUM immediately - high dead tuple percentage")
		if t.DeadTuplePercent > 50 {
			recs = append(recs, "Consider VACUUM FULL during maintenance window to reclaim disk space")
		}
	case t.DeadTuplePercent > 10:
		recs = append(recs, "Run VACUUM soon to reclaim space")
	case t.DeadTuplePercent > 5:
		recs = append(recs, "Schedule VACUUM during next maintenance window")
	}

	// Analyze recommendations
	if t.ModsSinceAnalyze > 1000 {
		recs = append(recs, "Run ANALYZE to update query planner statistics")
	}

	// Check last maintenance times
	now := time.Now()
	daysSinceVacuum := -1.0
	if t.LastVacuum != nil {
		daysSinceVacuum = now.Sub(*t.LastVacuum).Hours() / 24
	} else if t.LastAutovacuum != nil {
		daysSinceVacuum = now.Sub(*t.LastAutovacuum).Hours() / 24
	}

	if daysSinceVacuum < 0 {
		recs = append(recs, "⚠️  Table has never been vacuumed - run VACUUM")
	} else if daysSinceVacuum > 7 && t.Updates+t.Deletes > 1000 {
		recs = append(recs, fmt.Sprintf("Table last vacuumed %.0f days ago with high write activity", daysSinceVacuum))
	}

	daysSinceAnalyze := -1.0
	if t.LastAnalyze != nil {
		daysSinceAnalyze = now.Sub(*t.LastAnalyze).Hours() / 24
	} else if t.LastAutoanalyze != nil {
		daysSinceAnalyze = now.Sub(*t.LastAutoanalyze).Hours() / 24
	}

	if daysSinceAnalyze < 0 {
		recs = append(recs, "Table has never been analyzed - run ANALYZE")
	} else if daysSinceAnalyze > 7 && t.ModsSinceAnalyze > 1000 {
		recs = append(recs, fmt.Sprintf("Table last analyzed %.0f days ago with %d modifications", daysSinceAnalyze, t.ModsSinceAnalyze))
	}

	if len(recs) == 0 {
		recs = append(recs, "✓ Table maintenance appears adequate")
	}

	return recs
}

// analyzeIndexBloat analyzes index bloat and returns formatted output
func analyzeIndexBloat(ctx context.Context, dbClient *database.Client, schemaName, tableName string) string {
	pool := dbClient.GetPool()
	if pool == nil {
		return "\nIndex Analysis: Database connection not available\n"
	}
	whereClause := ""
	var queryParams []interface{}
	paramCount := 0

	if schemaName != "" {
		paramCount++
		whereClause = fmt.Sprintf("WHERE schemaname = $%d", paramCount)
		queryParams = append(queryParams, schemaName)

		if tableName != "" {
			paramCount++
			whereClause += fmt.Sprintf(" AND tablename = $%d", paramCount)
			queryParams = append(queryParams, tableName)
		}
	}

	query := `
		SELECT
			schemaname,
			tablename,
			indexname,
			idx_scan,
			pg_size_pretty(pg_relation_size(schemaname||'.'||indexname)) as index_size,
			pg_relation_size(schemaname||'.'||indexname) as index_size_bytes
		FROM pg_stat_user_indexes
		` + whereClause + `
		ORDER BY pg_relation_size(schemaname||'.'||indexname) DESC
	`

	rows, err := pool.Query(ctx, query, queryParams...)
	if err != nil {
		return fmt.Sprintf("\nIndex Analysis: Error querying indexes: %v\n", err)
	}
	defer rows.Close()

	var output strings.Builder
	output.WriteString("\nIndex Bloat Analysis\n")
	output.WriteString("====================\n\n")

	indexCount := 0
	for rows.Next() {
		var schema, table, index, size string
		var scans, sizeBytes int64

		if err := rows.Scan(&schema, &table, &index, &scans, &size, &sizeBytes); err != nil {
			continue
		}

		indexCount++
		output.WriteString(fmt.Sprintf("%d. %s.%s (on %s.%s)\n", indexCount, schema, index, schema, table))
		output.WriteString(fmt.Sprintf("   Size: %s (%d bytes)\n", size, sizeBytes))
		output.WriteString(fmt.Sprintf("   Index scans: %d\n", scans))

		// Simple recommendations
		if scans == 0 {
			output.WriteString("   Recommendation: Unused index - consider dropping\n")
		} else if sizeBytes > 100*1024*1024 { // > 100MB
			output.WriteString("   Recommendation: Large index - consider REINDEX if performance degrades\n")
		}
		output.WriteString("\n")
	}

	if indexCount == 0 {
		output.WriteString("No indexes found matching criteria\n")
	}

	return output.String()
}
