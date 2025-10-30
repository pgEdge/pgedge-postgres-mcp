/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// PGSettingsResource creates a resource for PostgreSQL configuration parameters
func PGSettingsResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         "pg://settings",
			Name:        "PostgreSQL Server Configuration",
			Description: "Returns PostgreSQL server configuration parameters including current values, default values, pending changes, and descriptions. Queries pg_settings system catalog.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			// Check if database is ready
			if !dbClient.IsMetadataLoaded() {
				return mcp.ResourceContent{
					URI:      "pg://settings",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Database is still initializing. Please wait a moment and try again.",
						},
					},
				}, fmt.Errorf("database not ready")
			}

			// Query pg_settings for configuration parameters
			query := `
				SELECT
					name,
					setting AS current_value,
					unit,
					category,
					short_desc AS description,
					extra_desc AS extra_description,
					context,
					vartype AS type,
					source,
					min_val AS min_value,
					max_val AS max_value,
					enumvals AS enum_values,
					boot_val AS default_value,
					reset_val AS reset_value,
					pending_restart
				FROM pg_settings
				ORDER BY category, name
			`

			ctx := context.Background()
			pool := dbClient.GetPool()
			if pool == nil {
				return mcp.ResourceContent{
					URI:      "pg://settings",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Database connection not available",
						},
					},
				}, fmt.Errorf("no database connection")
			}

			rows, err := pool.Query(ctx, query)
			if err != nil {
				return mcp.ResourceContent{
					URI:      "pg://settings",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Failed to query pg_settings: %v", err),
						},
					},
				}, err
			}
			defer rows.Close()

			// Collect settings
			type Setting struct {
				Name             string   `json:"name"`
				CurrentValue     string   `json:"current_value"`
				Unit             *string  `json:"unit,omitempty"`
				Category         string   `json:"category"`
				Description      string   `json:"description"`
				ExtraDescription *string  `json:"extra_description,omitempty"`
				Context          string   `json:"context"`
				Type             string   `json:"type"`
				Source           string   `json:"source"`
				MinValue         *string  `json:"min_value,omitempty"`
				MaxValue         *string  `json:"max_value,omitempty"`
				EnumValues       []string `json:"enum_values,omitempty"`
				DefaultValue     *string  `json:"default_value,omitempty"`
				ResetValue       *string  `json:"reset_value,omitempty"`
				PendingRestart   bool     `json:"pending_restart"`
			}

			var settings []Setting

			for rows.Next() {
				var s Setting
				var enumValsArray interface{}

				err := rows.Scan(
					&s.Name,
					&s.CurrentValue,
					&s.Unit,
					&s.Category,
					&s.Description,
					&s.ExtraDescription,
					&s.Context,
					&s.Type,
					&s.Source,
					&s.MinValue,
					&s.MaxValue,
					&enumValsArray,
					&s.DefaultValue,
					&s.ResetValue,
					&s.PendingRestart,
				)
				if err != nil {
					return mcp.ResourceContent{
						URI:      "pg://settings",
						MimeType: "application/json",
						Contents: []mcp.ContentItem{
							{
								Type: "text",
								Text: fmt.Sprintf("Error reading row: %v", err),
							},
						},
					}, err
				}

				// Parse enum values if present
				if enumValsArray != nil {
					if enumVals, ok := enumValsArray.([]interface{}); ok {
						for _, v := range enumVals {
							if str, ok := v.(string); ok {
								s.EnumValues = append(s.EnumValues, str)
							}
						}
					}
				}

				settings = append(settings, s)
			}

			if err := rows.Err(); err != nil {
				return mcp.ResourceContent{
					URI:      "pg://settings",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error iterating rows: %v", err),
						},
					},
				}, err
			}

			// Format output
			jsonData, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return mcp.ResourceContent{
					URI:      "pg://settings",
					MimeType: "application/json",
					Contents: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error formatting settings: %v", err),
						},
					},
				}, err
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("PostgreSQL Server Configuration (%d settings)\n\n", len(settings)))
			sb.WriteString("Settings organized by category with current, default, and reset values.\n")
			sb.WriteString("Use the set_pg_configuration tool to modify settings.\n\n")
			sb.WriteString(string(jsonData))

			return mcp.ResourceContent{
				URI:      "pg://settings",
				MimeType: "application/json",
				Contents: []mcp.ContentItem{
					{
						Type: "text",
						Text: sb.String(),
					},
				},
			}, nil
		},
	}
}
