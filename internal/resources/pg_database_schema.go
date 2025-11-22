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
	"fmt"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"

	"github.com/jackc/pgx/v5"
)

// PGDatabaseSchemaResource creates a resource for database schema overview
func PGDatabaseSchemaResource(dbClient *database.Client) Resource {
	return Resource{
		Definition: mcp.Resource{
			URI:  URIDatabaseSchema,
			Name: "PostgreSQL Database Schema",
			Description: `Lightweight table listing: schema names, table names, and owners only.

<usecase>
Use this resource for:
- Quick overview of database structure
- Finding all schemas and tables
- Checking table ownership
- Initial database discovery
</usecase>

<limitations>
Does NOT include:
- Column details (use get_schema_info tool instead)
- Data types, constraints, indexes
- Primary/foreign key relationships
- Table descriptions from pg_description
- Vector column detection
</limitations>

<when_to_use_tools>
For detailed schema exploration, use get_schema_info tool which provides:
- All columns with data types and nullable status
- Primary/foreign key constraints
- Table and column descriptions
- Vector column detection for similarity_search
- Much more comprehensive information
</when_to_use_tools>

<recommendation>
This resource is best for quick table discovery. For actual query writing or schema analysis, prefer the get_schema_info tool.
</recommendation>`,
			MimeType: "application/json",
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
