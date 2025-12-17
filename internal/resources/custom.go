/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
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
	"pgedge-postgres-mcp/internal/definitions"
	"pgedge-postgres-mcp/internal/mcp"
	"pgedge-postgres-mcp/internal/tsv"
)

// RegisterSQL registers a SQL-based resource
func (r *ContextAwareRegistry) RegisterSQL(def definitions.ResourceDefinition) error {
	if def.Type != "sql" {
		return fmt.Errorf("resource type must be 'sql', got: %s", def.Type)
	}

	if def.SQL == "" {
		return fmt.Errorf("SQL query is required for SQL resource")
	}

	// Create handler that executes SQL query and returns TSV format
	handler := func(ctx context.Context, dbClient *database.Client) (mcp.ResourceContent, error) {
		// Check if metadata is loaded
		if !dbClient.IsMetadataLoaded() {
			return mcp.NewResourceError(def.URI, mcp.DatabaseNotReadyErrorShort)
		}

		// Get connection pool
		pool := dbClient.GetPool()
		if pool == nil {
			return mcp.ResourceContent{}, fmt.Errorf("no connection pool available")
		}

		// Execute query
		rows, err := pool.Query(ctx, def.SQL)
		if err != nil {
			return mcp.ResourceContent{}, fmt.Errorf("failed to query: %w", err)
		}
		defer rows.Close()

		// Build TSV output
		var output strings.Builder

		// Get column names for header
		fieldDescriptions := rows.FieldDescriptions()
		columnNames := make([]string, len(fieldDescriptions))
		for i, fd := range fieldDescriptions {
			columnNames[i] = string(fd.Name)
		}
		output.WriteString(strings.Join(columnNames, "\t"))
		output.WriteString("\n")

		// Process rows
		for rows.Next() {
			values := make([]interface{}, len(fieldDescriptions))
			valuePtrs := make([]interface{}, len(fieldDescriptions))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			// Scan row
			if err := rows.Scan(valuePtrs...); err != nil {
				return mcp.ResourceContent{}, fmt.Errorf("row scan error: %w", err)
			}

			// Build TSV row
			rowValues := make([]string, len(values))
			for i, v := range values {
				rowValues[i] = tsv.FormatValue(v)
			}
			output.WriteString(strings.Join(rowValues, "\t"))
			output.WriteString("\n")
		}

		// Check for row iteration errors
		if err := rows.Err(); err != nil {
			return mcp.ResourceContent{}, fmt.Errorf("error iterating rows: %w", err)
		}

		return mcp.NewResourceSuccess(def.URI, "text/tab-separated-values", output.String())
	}

	// Register resource
	r.customResources[def.URI] = customResource{
		definition: mcp.Resource{
			URI:         def.URI,
			Name:        def.Name,
			Description: def.Description,
			MimeType:    "text/tab-separated-values",
		},
		handler: handler,
	}

	return nil
}

// RegisterStatic registers a static data resource
func (r *ContextAwareRegistry) RegisterStatic(def definitions.ResourceDefinition) error {
	if def.Type != "static" {
		return fmt.Errorf("resource type must be 'static', got: %s", def.Type)
	}

	if def.Data == nil {
		return fmt.Errorf("data is required for static resource")
	}

	// Create handler that returns static data
	handler := func(ctx context.Context, dbClient *database.Client) (mcp.ResourceContent, error) {
		// Format data based on type
		var jsonData []byte
		var err error

		switch v := def.Data.(type) {
		case string, int, int64, float64, bool:
			// Single scalar value
			jsonData, err = json.MarshalIndent(v, "", "  ")
		case []interface{}:
			// Array - could be single row, multiple values, or 2D array
			jsonData, err = json.MarshalIndent(v, "", "  ")
		case map[string]interface{}:
			// Object
			jsonData, err = json.MarshalIndent(v, "", "  ")
		default:
			// Unknown type - try to marshal as-is
			jsonData, err = json.MarshalIndent(v, "", "  ")
		}

		if err != nil {
			return mcp.ResourceContent{
				URI: def.URI,
				Contents: []mcp.ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("JSON encoding error: %v", err),
					},
				},
			}, nil
		}

		return mcp.ResourceContent{
			URI:      def.URI,
			MimeType: def.MimeType,
			Contents: []mcp.ContentItem{
				{
					Type: "text",
					Text: string(jsonData),
				},
			},
		}, nil
	}

	// Register resource
	r.customResources[def.URI] = customResource{
		definition: mcp.Resource{
			URI:         def.URI,
			Name:        def.Name,
			Description: def.Description,
			MimeType:    def.MimeType,
		},
		handler: handler,
	}

	return nil
}
