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
	"strings"

	"pgedge-mcp/internal/database"
	"pgedge-mcp/internal/llm"
	"pgedge-mcp/internal/mcp"
)

// QueryDatabaseTool creates the query_database tool
func QueryDatabaseTool(dbClient *database.Client, llmClient *llm.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "query_database",
			Description: "Execute a natural language query against the PostgreSQL database. The system will analyze the database schema (including table names, column names, data types, and comments from pg_description) to understand the structure and convert your natural language query into SQL. You can also specify an alternative database connection by including 'at postgres://...' in your query, or set a new default connection with 'set default database to postgres://...'.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Natural language question about the data in the database. Can include connection strings like 'Show users at postgres://host/db' or 'set default database to postgres://host/db'.",
					},
				},
				Required: []string{"query"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			query, ok := args["query"].(string)
			if !ok {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Missing or invalid 'query' parameter",
						},
					},
					IsError: true,
				}, nil
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
						return mcp.ToolResponse{
							Content: []mcp.ContentItem{
								{
									Type: "text",
									Text: fmt.Sprintf("Failed to set default connection to %s: %v", queryCtx.ConnectionString, err),
								},
							},
							IsError: true,
						}, nil
					}

					return mcp.ToolResponse{
						Content: []mcp.ContentItem{
							{
								Type: "text",
								Text: fmt.Sprintf("Successfully set default database connection to:\n%s\n\nMetadata loaded: %d tables/views available.",
									queryCtx.ConnectionString,
									len(dbClient.GetMetadata())),
							},
						},
					}, nil
				} else {
					// Temporary connection for this query only
					err := dbClient.ConnectTo(queryCtx.ConnectionString)
					if err != nil {
						return mcp.ToolResponse{
							Content: []mcp.ContentItem{
								{
									Type: "text",
									Text: fmt.Sprintf("Failed to connect to %s: %v", queryCtx.ConnectionString, err),
								},
							},
							IsError: true,
						}, nil
					}

					// Load metadata if needed
					if !dbClient.IsMetadataLoadedFor(queryCtx.ConnectionString) {
						err = dbClient.LoadMetadataFor(queryCtx.ConnectionString)
						if err != nil {
							return mcp.ToolResponse{
								Content: []mcp.ContentItem{
									{
										Type: "text",
										Text: fmt.Sprintf("Failed to load metadata from %s: %v", queryCtx.ConnectionString, err),
									},
								},
								IsError: true,
							}, nil
						}
					}

					connStr = queryCtx.ConnectionString
					connectionMessage = fmt.Sprintf("Using connection: %s\n\n", connStr)
				}
			}

			// If the cleaned query is empty (e.g., just a connection command), we're done
			if strings.TrimSpace(queryCtx.CleanedQuery) == "" {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Connection command executed successfully. No query to run.",
						},
					},
				}, nil
			}

			// Check if metadata is loaded for the target connection
			if !dbClient.IsMetadataLoadedFor(connStr) {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Database is still initializing. Please wait a moment and try again.\n\nThe server is loading database metadata in the background. This usually takes a few seconds.",
						},
					},
					IsError: true,
				}, nil
			}

			// Generate schema context for the target connection
			schemaContext := generateSchemaContext(dbClient.GetMetadataFor(connStr))

			// Check if LLM is configured
			if !llmClient.IsConfigured() {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("%sNatural language query: %s\n\nDatabase Schema Context:\n%s\n\nERROR: ANTHROPIC_API_KEY environment variable is not set.\n\nTo enable natural language to SQL conversion, please set the ANTHROPIC_API_KEY environment variable.\nYou can optionally set ANTHROPIC_MODEL to specify a different model (default: claude-sonnet-4-5).",
								connectionMessage, queryCtx.CleanedQuery, schemaContext),
						},
					},
					IsError: true,
				}, nil
			}

			// Convert natural language to SQL using the cleaned query
			sqlQuery, err := llmClient.ConvertNLToSQL(queryCtx.CleanedQuery, schemaContext)
			if err != nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("%sFailed to convert natural language to SQL: %v", connectionMessage, err),
						},
					},
					IsError: true,
				}, nil
			}

			// Execute the SQL query on the appropriate connection
			ctx := context.Background()
			pool := dbClient.GetPoolFor(connStr)
			if pool == nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Connection pool not found for: %s", connStr),
						},
					},
					IsError: true,
				}, nil
			}

			rows, err := pool.Query(ctx, sqlQuery)
			if err != nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("%sGenerated SQL:\n%s\n\nError executing query: %v", connectionMessage, sqlQuery, err),
						},
					},
					IsError: true,
				}, nil
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
					return mcp.ToolResponse{
						Content: []mcp.ContentItem{
							{
								Type: "text",
								Text: fmt.Sprintf("Error reading row: %v", err),
							},
						},
						IsError: true,
					}, nil
				}

				row := make(map[string]interface{})
				for i, colName := range columnNames {
					row[colName] = values[i]
				}
				results = append(results, row)
			}

			if err := rows.Err(); err != nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error iterating rows: %v", err),
						},
					},
					IsError: true,
				}, nil
			}

			// Format results
			resultsJSON, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error formatting results: %v", err),
						},
					},
					IsError: true,
				}, nil
			}

			var sb strings.Builder
			if connectionMessage != "" {
				sb.WriteString(connectionMessage)
			}
			sb.WriteString(fmt.Sprintf("Natural Language Query: %s\n\n", queryCtx.CleanedQuery))
			sb.WriteString(fmt.Sprintf("Generated SQL:\n%s\n\n", sqlQuery))
			sb.WriteString(fmt.Sprintf("Results (%d rows):\n%s", len(results), string(resultsJSON)))

			return mcp.ToolResponse{
				Content: []mcp.ContentItem{
					{
						Type: "text",
						Text: sb.String(),
					},
				},
			}, nil
		},
	}
}

func generateSchemaContext(metadata map[string]database.TableInfo) string {
	var sb strings.Builder

	for key, table := range metadata {
		sb.WriteString(fmt.Sprintf("\n%s (%s)\n", key, table.TableType))
		if table.Description != "" {
			sb.WriteString(fmt.Sprintf("  Description: %s\n", table.Description))
		}
		sb.WriteString("  Columns:\n")

		for _, col := range table.Columns {
			sb.WriteString(fmt.Sprintf("    - %s (%s)", col.ColumnName, col.DataType))
			if col.IsNullable == "YES" {
				sb.WriteString(" NULL")
			}
			if col.Description != "" {
				sb.WriteString(fmt.Sprintf(" - %s", col.Description))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
