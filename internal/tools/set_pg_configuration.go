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

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// SetPGConfigurationTool creates a tool for setting PostgreSQL configuration parameters
func SetPGConfigurationTool(dbClient *database.Client) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "set_pg_configuration",
			Description: "Set PostgreSQL server configuration parameters using ALTER SYSTEM SET command. Changes persist across server restarts. Some parameters require a server restart to take effect. Use the pg://settings resource to view current configuration.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"parameter": map[string]interface{}{
						"type":        "string",
						"description": "Name of the configuration parameter to set (e.g., 'max_connections', 'shared_buffers')",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "Value to set for the parameter. Use 'DEFAULT' to reset to default value.",
					},
				},
				Required: []string{"parameter", "value"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			parameter, ok := args["parameter"].(string)
			if !ok || parameter == "" {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Missing or invalid 'parameter' argument",
						},
					},
					IsError: true,
				}, nil
			}

			value, ok := args["value"].(string)
			if !ok {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Missing or invalid 'value' argument",
						},
					},
					IsError: true,
				}, nil
			}

			// Check if database is ready
			if !dbClient.IsMetadataLoaded() {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Database is still initializing. Please wait a moment and try again.",
						},
					},
					IsError: true,
				}, nil
			}

			pool := dbClient.GetPool()
			if pool == nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Database connection not available",
						},
					},
					IsError: true,
				}, nil
			}

			ctx := context.Background()

			// First, get information about the parameter
			paramInfoQuery := `
				SELECT
					name,
					context,
					vartype,
					short_desc,
					setting AS current_value,
					pending_restart
				FROM pg_settings
				WHERE name = $1
			`

			var paramInfo struct {
				Name           string
				Context        string
				VarType        string
				Description    string
				CurrentValue   string
				PendingRestart bool
			}

			err := pool.QueryRow(ctx, paramInfoQuery, parameter).Scan(
				&paramInfo.Name,
				&paramInfo.Context,
				&paramInfo.VarType,
				&paramInfo.Description,
				&paramInfo.CurrentValue,
				&paramInfo.PendingRestart,
			)

			if err != nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Parameter '%s' not found or error querying pg_settings: %v\n\nUse the pg://settings resource to view available parameters.", parameter, err),
						},
					},
					IsError: true,
				}, nil
			}

			// Construct ALTER SYSTEM SET command
			var sqlCommand string
			if strings.EqualFold(value, "DEFAULT") {
				sqlCommand = fmt.Sprintf("ALTER SYSTEM SET %s = DEFAULT", parameter)
			} else {
				// Quote the value appropriately
				sqlCommand = fmt.Sprintf("ALTER SYSTEM SET %s = '%s'", parameter, strings.ReplaceAll(value, "'", "''"))
			}

			// Execute the ALTER SYSTEM SET command
			_, err = pool.Exec(ctx, sqlCommand)
			if err != nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Failed to set parameter '%s': %v\n\nSQL: %s", parameter, err, sqlCommand),
						},
					},
					IsError: true,
				}, nil
			}

			// Reload configuration to apply changes that don't require restart
			_, reloadErr := pool.Exec(ctx, "SELECT pg_reload_conf()")

			// Get the new value
			var newValue string
			var pendingRestart bool
			err = pool.QueryRow(ctx, "SELECT setting, pending_restart FROM pg_settings WHERE name = $1", parameter).Scan(&newValue, &pendingRestart)
			if err != nil {
				newValue = value
			}

			// Build response
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Configuration parameter '%s' updated successfully.\n\n", parameter))
			sb.WriteString(fmt.Sprintf("Parameter: %s\n", paramInfo.Name))
			sb.WriteString(fmt.Sprintf("Description: %s\n", paramInfo.Description))
			sb.WriteString(fmt.Sprintf("Type: %s\n", paramInfo.VarType))
			sb.WriteString(fmt.Sprintf("Context: %s\n\n", paramInfo.Context))
			sb.WriteString(fmt.Sprintf("Previous value: %s\n", paramInfo.CurrentValue))
			sb.WriteString(fmt.Sprintf("New value: %s\n\n", newValue))

			if reloadErr == nil {
				sb.WriteString("Configuration reloaded successfully.\n")
			}

			if pendingRestart || paramInfo.Context == "postmaster" {
				sb.WriteString("\n⚠️  WARNING: This parameter requires a server restart to take effect.\n")
				sb.WriteString("The change has been saved to postgresql.auto.conf but will not be active until the server is restarted.\n")
			} else {
				sb.WriteString("\n✓ Change is now active.\n")
			}

			sb.WriteString(fmt.Sprintf("\nSQL executed: %s\n", sqlCommand))

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
