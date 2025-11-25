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

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/definitions"
	"pgedge-postgres-mcp/internal/mcp"

	"github.com/jackc/pgx/v5"
)

// RegisterSQL registers a SQL-based resource
func (r *ContextAwareRegistry) RegisterSQL(def definitions.ResourceDefinition) error {
	if def.Type != "sql" {
		return fmt.Errorf("resource type must be 'sql', got: %s", def.Type)
	}

	if def.SQL == "" {
		return fmt.Errorf("SQL query is required for SQL resource")
	}

	// Create handler that executes SQL query
	handler := func(ctx context.Context, dbClient *database.Client) (mcp.ResourceContent, error) {
		processor := func(rows pgx.Rows) (interface{}, error) {
			var results []map[string]interface{}

			for rows.Next() {
				// Get column names
				fieldDescriptions := rows.FieldDescriptions()
				values := make([]interface{}, len(fieldDescriptions))
				valuePtrs := make([]interface{}, len(fieldDescriptions))
				for i := range values {
					valuePtrs[i] = &values[i]
				}

				// Scan row
				if err := rows.Scan(valuePtrs...); err != nil {
					return nil, fmt.Errorf("row scan error: %w", err)
				}

				// Build row map
				row := make(map[string]interface{})
				for i, fd := range fieldDescriptions {
					row[string(fd.Name)] = values[i]
				}
				results = append(results, row)
			}

			return results, nil
		}

		return database.ExecuteResourceQuery(dbClient, def.URI, def.SQL, processor)
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
