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
	"fmt"
	"strings"

	"pgedge-mcp/internal/database"
	"pgedge-mcp/internal/mcp"
)

// GetSchemaInfoTool creates the get_schema_info tool
func GetSchemaInfoTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "get_schema_info",
			Description: "Get detailed schema information about the database, including all tables, views, columns, data types, and descriptions from pg_description. Useful for understanding the database structure before querying.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"schema_name": map[string]interface{}{
						"type":        "string",
						"description": "Optional: specific schema name to get info for. If not provided, returns all schemas.",
					},
				},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			schemaName, ok := args["schema_name"].(string)
			if !ok {
				schemaName = "" // Default to empty string (all schemas)
			}

			// Check if metadata is loaded
			if !dbClient.IsMetadataLoaded() {
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

			var sb strings.Builder
			sb.WriteString("Database Schema Information:\n")
			sb.WriteString("============================\n")

			metadata := dbClient.GetMetadata()
			for _, table := range metadata {
				// Filter by schema if requested
				if schemaName != "" && table.SchemaName != schemaName {
					continue
				}

				sb.WriteString(fmt.Sprintf("\n%s.%s (%s)\n", table.SchemaName, table.TableName, table.TableType))
				if table.Description != "" {
					sb.WriteString(fmt.Sprintf("  Description: %s\n", table.Description))
				}

				sb.WriteString("  Columns:\n")
				for _, col := range table.Columns {
					sb.WriteString(fmt.Sprintf("    - %s: %s", col.ColumnName, col.DataType))
					if col.IsNullable == "YES" {
						sb.WriteString(" (nullable)")
					}
					if col.Description != "" {
						sb.WriteString(fmt.Sprintf("\n      Description: %s", col.Description))
					}
					sb.WriteString("\n")
				}
			}

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
