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
	"strings"
	"testing"

	"pgedge-mcp/internal/database"
)

func TestGenerateSchemaContext(t *testing.T) {
	t.Run("empty metadata", func(t *testing.T) {
		metadata := make(map[string]database.TableInfo)
		result := generateSchemaContext(metadata)

		if result != "" {
			t.Errorf("generateSchemaContext() = %q, want empty string", result)
		}
	})

	t.Run("single table with columns", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.users": {
				SchemaName:  "public",
				TableName:   "users",
				TableType:   "TABLE",
				Description: "User accounts",
				Columns: []database.ColumnInfo{
					{
						ColumnName:  "id",
						DataType:    "integer",
						IsNullable:  "NO",
						Description: "Primary key",
					},
					{
						ColumnName:  "email",
						DataType:    "text",
						IsNullable:  "YES",
						Description: "User email address",
					},
				},
			},
		}

		result := generateSchemaContext(metadata)

		// Check that result contains expected elements
		if !strings.Contains(result, "public.users (TABLE)") {
			t.Error("Result should contain 'public.users (TABLE)'")
		}
		if !strings.Contains(result, "Description: User accounts") {
			t.Error("Result should contain table description")
		}
		if !strings.Contains(result, "Columns:") {
			t.Error("Result should contain 'Columns:'")
		}
		if !strings.Contains(result, "id (integer)") {
			t.Error("Result should contain 'id (integer)'")
		}
		if !strings.Contains(result, "Primary key") {
			t.Error("Result should contain column description 'Primary key'")
		}
		if !strings.Contains(result, "email (text) NULL") {
			t.Error("Result should contain 'email (text) NULL'")
		}
		if !strings.Contains(result, "User email address") {
			t.Error("Result should contain column description 'User email address'")
		}
	})

	t.Run("multiple tables", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.users": {
				SchemaName: "public",
				TableName:  "users",
				TableType:  "TABLE",
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
				},
			},
			"public.orders": {
				SchemaName: "public",
				TableName:  "orders",
				TableType:  "TABLE",
				Columns: []database.ColumnInfo{
					{
						ColumnName: "order_id",
						DataType:   "bigint",
						IsNullable: "NO",
					},
				},
			},
		}

		result := generateSchemaContext(metadata)

		if !strings.Contains(result, "public.users") {
			t.Error("Result should contain 'public.users'")
		}
		if !strings.Contains(result, "public.orders") {
			t.Error("Result should contain 'public.orders'")
		}
	})

	t.Run("view with nullable columns", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.user_stats": {
				SchemaName:  "public",
				TableName:   "user_stats",
				TableType:   "VIEW",
				Description: "User statistics view",
				Columns: []database.ColumnInfo{
					{
						ColumnName: "user_count",
						DataType:   "bigint",
						IsNullable: "YES",
					},
				},
			},
		}

		result := generateSchemaContext(metadata)

		if !strings.Contains(result, "public.user_stats (VIEW)") {
			t.Error("Result should contain 'public.user_stats (VIEW)'")
		}
		if !strings.Contains(result, "user_count (bigint) NULL") {
			t.Error("Result should indicate nullable column with NULL")
		}
	})

	t.Run("materialized view", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.cached_stats": {
				SchemaName: "public",
				TableName:  "cached_stats",
				TableType:  "MATERIALIZED VIEW",
				Columns: []database.ColumnInfo{
					{
						ColumnName: "total",
						DataType:   "numeric",
						IsNullable: "NO",
					},
				},
			},
		}

		result := generateSchemaContext(metadata)

		if !strings.Contains(result, "public.cached_stats (MATERIALIZED VIEW)") {
			t.Error("Result should contain 'public.cached_stats (MATERIALIZED VIEW)'")
		}
	})

	t.Run("column without description", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.simple": {
				SchemaName: "public",
				TableName:  "simple",
				TableType:  "TABLE",
				Columns: []database.ColumnInfo{
					{
						ColumnName:  "id",
						DataType:    "integer",
						IsNullable:  "NO",
						Description: "",
					},
				},
			},
		}

		result := generateSchemaContext(metadata)

		// Should contain column info but without description separator
		if !strings.Contains(result, "id (integer)") {
			t.Error("Result should contain 'id (integer)'")
		}
		// Should not have the description separator (") - ") when description is empty
		lines := strings.Split(result, "\n")
		for _, line := range lines {
			if strings.Contains(line, "id (integer)") && strings.Contains(line, ") - ") {
				t.Error("Result should not contain description separator for empty description")
			}
		}
	})

	t.Run("table without description", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.test": {
				SchemaName:  "public",
				TableName:   "test",
				TableType:   "TABLE",
				Description: "",
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
				},
			},
		}

		result := generateSchemaContext(metadata)

		if !strings.Contains(result, "public.test (TABLE)") {
			t.Error("Result should contain 'public.test (TABLE)'")
		}
		if strings.Contains(result, "Description:") {
			t.Error("Result should not contain 'Description:' when table description is empty")
		}
	})

	t.Run("complex data types", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.complex": {
				SchemaName: "public",
				TableName:  "complex",
				TableType:  "TABLE",
				Columns: []database.ColumnInfo{
					{
						ColumnName: "data",
						DataType:   "jsonb",
						IsNullable: "YES",
					},
					{
						ColumnName: "tags",
						DataType:   "text[]",
						IsNullable: "YES",
					},
					{
						ColumnName: "created_at",
						DataType:   "timestamp with time zone",
						IsNullable: "NO",
					},
				},
			},
		}

		result := generateSchemaContext(metadata)

		if !strings.Contains(result, "data (jsonb) NULL") {
			t.Error("Result should contain 'data (jsonb) NULL'")
		}
		if !strings.Contains(result, "tags (text[]) NULL") {
			t.Error("Result should contain 'tags (text[]) NULL'")
		}
		if !strings.Contains(result, "created_at (timestamp with time zone)") {
			t.Error("Result should contain 'created_at (timestamp with time zone)'")
		}
	})
}
