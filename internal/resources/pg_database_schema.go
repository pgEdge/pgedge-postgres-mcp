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
	"fmt"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"

	"github.com/jackc/pgx/v5"
)

// PGDatabaseSchemaResource creates a resource for database schema overview
func PGDatabaseSchemaResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:         URIDatabaseSchema,
			Name:        "PostgreSQL Database Schema",
			Description: "Returns a lightweight overview of all tables in the database. Lists schema names, table names, and table owners. Use get_schema_info tool for detailed column information.",
			MimeType:    "application/json",
		},
		Handler: func() (mcp.ResourceContent, error) {
			query := `
                SELECT
                    schemaname,
                    tablename,
                    tableowner
                FROM pg_tables
                WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
                ORDER BY schemaname, tablename
            `

			processor := func(rows pgx.Rows) (interface{}, error) {
				tables := []TableInfo{}

				for rows.Next() {
					var table TableInfo
					err := rows.Scan(&table.SchemaName, &table.TableName, &table.TableOwner)
					if err != nil {
						return nil, fmt.Errorf("failed to scan table info: %w", err)
					}
					tables = append(tables, table)
				}

				return map[string]interface{}{
					"tables": tables,
					"count":  len(tables),
				}, nil
			}

			return database.ExecuteResourceQuery(dbClient, URIDatabaseSchema, query, processor)
		},
	}
}

// TableInfo represents basic table information
type TableInfo struct {
	SchemaName string `json:"schema"`
	TableName  string `json:"table"`
	TableOwner string `json:"owner"`
}
