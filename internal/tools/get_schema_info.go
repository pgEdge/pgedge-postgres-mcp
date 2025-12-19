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
	"fmt"
	"strings"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// GetSchemaInfoTool creates the get_schema_info tool
func GetSchemaInfoTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "get_schema_info",
			Description: `PRIMARY TOOL for discovering database structure and available tables.

<usecase>
Use get_schema_info when you need to:
- Discover what tables exist in the database
- Understand table structure (columns, types, constraints)
- Find tables with specific capabilities (e.g., vector columns)
- Learn column names before writing queries
- Check data types, nullable, and unique constraints
- Understand primary/foreign key relationships for JOINs
</usecase>

<why_use_this_first>
ALWAYS call this tool FIRST when:
- User asks to query data but doesn't specify table names
- You need to write a SQL query and don't know the schema
- User asks "what data is available?"
- Before using similarity_search (to find vector-enabled tables)
- You're unsure about column names or data types
</why_use_this_first>

<key_features>
Returns comprehensive information in TSV format (one row per column):
- schema, table, type, table_desc, column, data_type, nullable, col_desc, is_pk, is_unique, fk_ref, is_indexed, identity, default, is_vector, vector_dims
- All tables and views in the database
- Column names, data types, nullable status
- Primary key (is_pk) and unique constraint (is_unique) indicators
- Foreign key references (fk_ref) in format "schema.table.column"
- Index membership (is_indexed) for query optimization hints
- Identity columns (identity): "a" for ALWAYS, "d" for BY DEFAULT, empty if not identity
- Default values for columns (default)
- Table and column descriptions from pg_description
- Vector column detection (pgvector extension)
- Schema organization
</key_features>

<filtering_options>
- No parameters: Returns summary if >10 tables, TSV details otherwise
- schema_name="public": Filter to specific schema only (always TSV details)
- table_name="users" (with schema_name): Get columns for specific table only
- vector_tables_only=true: Show only tables with pgvector columns (reduces output 10x)
- compact=true: Return table names only (no column details)
</filtering_options>

<auto_summary_mode>
When called without filters on databases with >10 tables, automatically returns
a compact summary showing:
- Total tables and schemas
- Table names per schema (first 5 + count of remaining)
- Vector-enabled tables highlighted
- Suggested next calls for detailed info

This prevents overwhelming token usage on large databases. Use schema_name
filter to get full details for specific schemas.
</auto_summary_mode>

<examples>
✓ "What tables are available?" → get_schema_info()
✓ "Show me tables with vector columns" → get_schema_info(vector_tables_only=true)
✓ "What's in the public schema?" → get_schema_info(schema_name="public")
✓ "Show me the users table structure" → get_schema_info(schema_name="public", table_name="users")
✓ Before writing: "SELECT * FROM users..." → get_schema_info() first to confirm 'users' table exists
</examples>

<important>
Results are returned in TSV (tab-separated values) format for token efficiency.
Use this tool for comprehensive schema exploration.
</important>

<rate_limit_awareness>
To avoid rate limits when calling this tool:
- Use schema_name="specific_schema" to filter output (reduces tokens by 90%)
- Use table_name="specific_table" with schema_name for single table details (reduces tokens by 95%)
- Use vector_tables_only=true when preparing for similarity_search (reduces output 10x)
- Use compact=true for table names only (no column details)
- Avoid calling without parameters in large databases (can return 10k+ tokens)
- Call once and cache results in conversation rather than repeatedly
- If exploring large schemas, filter by schema_name first
</rate_limit_awareness>`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"schema_name": map[string]interface{}{
						"type":        "string",
						"description": "Optional: specific schema name to get info for. If not provided, returns all schemas.",
					},
					"table_name": map[string]interface{}{
						"type":        "string",
						"description": "Optional: specific table name to get columns for. Requires schema_name to also be provided.",
					},
					"vector_tables_only": map[string]interface{}{
						"type":        "boolean",
						"description": "Optional: if true, only return tables with vector columns (for semantic search). Reduces output significantly.",
						"default":     false,
					},
					"compact": map[string]interface{}{
						"type":        "boolean",
						"description": "Optional: if true, return table names only (no column details). Use for quick overview.",
						"default":     false,
					},
				},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			schemaName, ok := args["schema_name"].(string)
			if !ok {
				schemaName = "" // Default to empty string (all schemas)
			}

			tableName, ok := args["table_name"].(string)
			if !ok {
				tableName = ""
			}

			// Validate: table_name requires schema_name
			if tableName != "" && schemaName == "" {
				return mcp.NewToolError("table_name requires schema_name to be specified")
			}

			vectorTablesOnly := false
			if vectorOnly, ok := args["vector_tables_only"].(bool); ok {
				vectorTablesOnly = vectorOnly
			}

			compactMode := false
			if compact, ok := args["compact"].(bool); ok {
				compactMode = compact
			}

			// If table_name is specified, ignore compact mode (user wants column details)
			if tableName != "" {
				compactMode = false
			}

			// Check if metadata is loaded
			if !dbClient.IsMetadataLoaded() {
				return mcp.NewToolError(mcp.DatabaseNotReadyError)
			}

			metadata := dbClient.GetMetadata()

			// Threshold for auto-summary mode (when no filters applied)
			const summaryThreshold = 10

			// First pass: count tables per schema and check for vector columns
			type schemaStats struct {
				tableNames   []string
				vectorTables []string
			}
			schemaMap := make(map[string]*schemaStats)
			totalMatched := 0

			for _, table := range metadata {
				// Filter by schema if requested
				if schemaName != "" && table.SchemaName != schemaName {
					continue
				}

				// Filter by table if requested
				if tableName != "" && table.TableName != tableName {
					continue
				}

				// Check for vector columns
				hasVectorColumn := false
				for i := range table.Columns {
					if table.Columns[i].IsVectorColumn {
						hasVectorColumn = true
						break
					}
				}

				// Filter for vector tables only if requested
				if vectorTablesOnly && !hasVectorColumn {
					continue
				}

				totalMatched++

				// Track stats per schema
				if schemaMap[table.SchemaName] == nil {
					schemaMap[table.SchemaName] = &schemaStats{}
				}
				schemaMap[table.SchemaName].tableNames = append(
					schemaMap[table.SchemaName].tableNames, table.TableName)
				if hasVectorColumn {
					schemaMap[table.SchemaName].vectorTables = append(
						schemaMap[table.SchemaName].vectorTables, table.TableName)
				}
			}

			// Auto-summary mode: when no filters applied and many tables
			autoSummary := schemaName == "" && tableName == "" && !vectorTablesOnly && !compactMode &&
				totalMatched > summaryThreshold

			var sb strings.Builder

			if autoSummary {
				// Smart summary mode for large databases
				sb.WriteString("Database Schema Summary:\n")
				sb.WriteString("========================\n\n")
				sb.WriteString(fmt.Sprintf("Found %d tables across %d schemas.\n\n",
					totalMatched, len(schemaMap)))

				// List schemas with their tables
				for schema, stats := range schemaMap {
					sb.WriteString(fmt.Sprintf("Schema '%s': %d tables\n",
						schema, len(stats.tableNames)))

					// Show first few table names as preview
					previewCount := 5
					if len(stats.tableNames) < previewCount {
						previewCount = len(stats.tableNames)
					}
					preview := stats.tableNames[:previewCount]
					sb.WriteString(fmt.Sprintf("  Tables: %s", strings.Join(preview, ", ")))
					if len(stats.tableNames) > previewCount {
						sb.WriteString(fmt.Sprintf(", ... (+%d more)",
							len(stats.tableNames)-previewCount))
					}
					sb.WriteString("\n")

					// Note vector-enabled tables if any
					if len(stats.vectorTables) > 0 {
						sb.WriteString(fmt.Sprintf("  Vector-enabled: %s\n",
							strings.Join(stats.vectorTables, ", ")))
					}
					sb.WriteString("\n")
				}

				sb.WriteString("<next_steps>\n")
				sb.WriteString("To reduce token usage and get detailed info:\n\n")
				sb.WriteString("1. Get details for a specific schema:\n")
				for schema := range schemaMap {
					sb.WriteString(fmt.Sprintf("   → get_schema_info(schema_name=%q)\n", schema))
				}
				sb.WriteString("\n2. Get only vector-enabled tables:\n")
				sb.WriteString("   → get_schema_info(vector_tables_only=true)\n\n")
				sb.WriteString("3. Get compact view (names only):\n")
				sb.WriteString("   → get_schema_info(compact=true)\n")
				sb.WriteString("</next_steps>\n")
			} else {
				// Standard output modes: TSV format
				if compactMode {
					// Compact mode: table names only (no column details)
					sb.WriteString("schema\ttable\ttype\ttable_desc\n")

					for _, table := range metadata {
						// Filter by schema if requested
						if schemaName != "" && table.SchemaName != schemaName {
							continue
						}

						// Filter by table if requested
						if tableName != "" && table.TableName != tableName {
							continue
						}

						// Filter for vector tables only if requested
						if vectorTablesOnly {
							hasVectorColumn := false
							for i := range table.Columns {
								if table.Columns[i].IsVectorColumn {
									hasVectorColumn = true
									break
								}
							}
							if !hasVectorColumn {
								continue
							}
						}

						sb.WriteString(BuildTSVRow(
							table.SchemaName,
							table.TableName,
							table.TableType,
							table.Description,
						))
						sb.WriteString("\n")
					}
				} else {
					// Full mode: one row per column with all details
					sb.WriteString("schema\ttable\ttype\ttable_desc\tcolumn\tdata_type\tnullable\tcol_desc\tis_pk\tis_unique\tfk_ref\tis_indexed\tidentity\tdefault\tis_vector\tvector_dims\n")

					for _, table := range metadata {
						// Filter by schema if requested
						if schemaName != "" && table.SchemaName != schemaName {
							continue
						}

						// Filter by table if requested
						if tableName != "" && table.TableName != tableName {
							continue
						}

						// Filter for vector tables only if requested
						if vectorTablesOnly {
							hasVectorColumn := false
							for i := range table.Columns {
								if table.Columns[i].IsVectorColumn {
									hasVectorColumn = true
									break
								}
							}
							if !hasVectorColumn {
								continue
							}
						}

						// Output one row per column
						for i := range table.Columns {
							col := &table.Columns[i]
							sb.WriteString(BuildTSVRow(
								table.SchemaName,
								table.TableName,
								table.TableType,
								table.Description,
								col.ColumnName,
								col.DataType,
								col.IsNullable,
								col.Description,
								fmt.Sprintf("%t", col.IsPrimaryKey),
								fmt.Sprintf("%t", col.IsUnique),
								col.ForeignKeyRef,
								fmt.Sprintf("%t", col.IsIndexed),
								col.IsIdentity,
								col.DefaultValue,
								fmt.Sprintf("%t", col.IsVectorColumn),
								fmt.Sprintf("%d", col.VectorDimensions),
							))
							sb.WriteString("\n")
						}
					}
				}
			}

			matchedTables := totalMatched

			// Handle empty results with contextual guidance
			if matchedTables == 0 {
				connStr := dbClient.GetDefaultConnection()
				sanitizedConn := database.SanitizeConnStr(connStr)

				var emptyMsg strings.Builder
				emptyMsg.WriteString("\nNo tables found matching your criteria.\n\n")

				emptyMsg.WriteString("<current_connection>\n")
				emptyMsg.WriteString(fmt.Sprintf("Connected to: %s\n", sanitizedConn))
				emptyMsg.WriteString("</current_connection>\n\n")

				emptyMsg.WriteString("<diagnosis>\n")
				if tableName != "" {
					emptyMsg.WriteString(fmt.Sprintf("Table '%s.%s' not found.\n", schemaName, tableName))
					emptyMsg.WriteString("Possible reasons:\n")
					emptyMsg.WriteString("1. Table name is misspelled (PostgreSQL is case-sensitive)\n")
					emptyMsg.WriteString("2. Table exists in a different schema\n")
					emptyMsg.WriteString("3. You don't have permission to view this table\n")
				} else if schemaName != "" && vectorTablesOnly {
					emptyMsg.WriteString(fmt.Sprintf("No tables with vector columns found in schema '%s'.\n", schemaName))
					emptyMsg.WriteString("Possible reasons:\n")
					emptyMsg.WriteString("1. Schema name is misspelled or doesn't exist\n")
					emptyMsg.WriteString("2. Schema exists but has no tables with vector columns\n")
					emptyMsg.WriteString("3. pgvector extension not installed or not used in this schema\n")
				} else if schemaName != "" {
					emptyMsg.WriteString(fmt.Sprintf("Schema '%s' not found or has no tables.\n", schemaName))
					emptyMsg.WriteString("Possible reasons:\n")
					emptyMsg.WriteString("1. Schema name is misspelled (PostgreSQL is case-sensitive)\n")
					emptyMsg.WriteString("2. Schema exists but is empty (no tables created yet)\n")
					emptyMsg.WriteString("3. You don't have permission to view this schema\n")
				} else if vectorTablesOnly {
					emptyMsg.WriteString("No tables with vector columns found in any schema.\n")
					emptyMsg.WriteString("Possible reasons:\n")
					emptyMsg.WriteString("1. pgvector extension not installed: CREATE EXTENSION vector;\n")
					emptyMsg.WriteString("2. No vector columns exist yet in this database\n")
					emptyMsg.WriteString("3. Connected to wrong database (vector tables might be elsewhere)\n")
				} else {
					emptyMsg.WriteString("Database appears to be completely empty (no tables in any schema).\n")
					emptyMsg.WriteString("Possible reasons:\n")
					emptyMsg.WriteString("1. New database with no tables created yet\n")
					emptyMsg.WriteString("2. Connected to wrong database\n")
					emptyMsg.WriteString("3. Permissions prevent you from viewing any tables\n")
				}
				emptyMsg.WriteString("</diagnosis>\n\n")

				emptyMsg.WriteString("<next_steps>\n")
				emptyMsg.WriteString("Recommended actions:\n\n")
				emptyMsg.WriteString("1. Check current database connection:\n")
				emptyMsg.WriteString("   → read_resource(uri=\"pg://system-info\")\n\n")

				emptyMsg.WriteString("2. List all databases to find the right one:\n")
				emptyMsg.WriteString("   → query_database(query=\"SELECT datname FROM pg_database WHERE datistemplate = false\", limit=20)\n\n")

				if tableName != "" {
					emptyMsg.WriteString("3. List all tables in the schema to find the correct name:\n")
					emptyMsg.WriteString(fmt.Sprintf("   → get_schema_info(schema_name=%q, compact=true)\n\n", schemaName))
				} else if schemaName != "" {
					emptyMsg.WriteString("3. List all available schemas:\n")
					emptyMsg.WriteString("   → query_database(query=\"SELECT schema_name FROM information_schema.schemata ORDER BY schema_name\", limit=50)\n\n")
					emptyMsg.WriteString("4. Try without schema filter:\n")
					emptyMsg.WriteString("   → get_schema_info()\n\n")
				}

				if vectorTablesOnly {
					emptyMsg.WriteString("3. Check if pgvector extension is installed:\n")
					emptyMsg.WriteString("   → query_database(query=\"SELECT * FROM pg_extension WHERE extname = 'vector'\")\n\n")
					emptyMsg.WriteString("4. Try without vector filter to see all tables:\n")
					if schemaName != "" {
						emptyMsg.WriteString(fmt.Sprintf("   → get_schema_info(schema_name=%q)\n\n", schemaName))
					} else {
						emptyMsg.WriteString("   → get_schema_info()\n\n")
					}
				}

				emptyMsg.WriteString("5. Switch to a different database if needed:\n")
				emptyMsg.WriteString("   → query_database(query=\"set default database to postgres://user@host/other_db\")\n")
				emptyMsg.WriteString("</next_steps>\n")

				return mcp.NewToolSuccess(emptyMsg.String())
			}

			// Prepend database context to the response
			connStr := dbClient.GetDefaultConnection()
			sanitizedConn := database.SanitizeConnStr(connStr)
			result := fmt.Sprintf("Database: %s\n\n%s", sanitizedConn, sb.String())

			return mcp.NewToolSuccess(result)
		},
	}
}
