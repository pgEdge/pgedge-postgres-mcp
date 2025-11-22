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
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/database"
)

// Helper function to create a mock database client with test data
func createMockClient(metadata map[string]database.TableInfo) *database.Client {
	return database.NewTestClient("postgres://localhost/test", metadata)
}

func TestGetSchemaInfoTool(t *testing.T) {
	t.Run("database not ready", func(t *testing.T) {
		client := database.NewClient(nil)
		// Don't add any connections - database is not ready

		tool := GetSchemaInfoTool(client)
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when database not ready")
		}
		if len(response.Content) == 0 {
			t.Fatal("Expected content in response")
		}
		content := response.Content[0].Text
		if !strings.Contains(content, "Database is still initializing") {
			t.Errorf("Expected database not ready message, got: %s", content)
		}
	})

	t.Run("empty metadata", func(t *testing.T) {
		client := createMockClient(map[string]database.TableInfo{})

		tool := GetSchemaInfoTool(client)
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false for empty metadata")
		}
		if len(response.Content) == 0 {
			t.Fatal("Expected content in response")
		}
		content := response.Content[0].Text
		// Updated to check for new empty result handling message
		if !strings.Contains(content, "No tables found matching your criteria") {
			t.Error("Expected empty result message with guidance")
		}
		// Should include diagnosis and next steps
		if !strings.Contains(content, "<diagnosis>") {
			t.Error("Expected diagnosis section in empty result")
		}
		if !strings.Contains(content, "<next_steps>") {
			t.Error("Expected next steps section in empty result")
		}
	})

	t.Run("single table with all fields", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.users": {
				SchemaName:  "public",
				TableName:   "users",
				TableType:   "TABLE",
				Description: "User accounts table",
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

		client := createMockClient(metadata)
		tool := GetSchemaInfoTool(client)
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false")
		}

		content := response.Content[0].Text

		// Check table header
		if !strings.Contains(content, "public.users (TABLE)") {
			t.Error("Expected 'public.users (TABLE)'")
		}
		if !strings.Contains(content, "Description: User accounts table") {
			t.Error("Expected table description")
		}

		// Check columns
		if !strings.Contains(content, "Columns:") {
			t.Error("Expected 'Columns:' section")
		}
		if !strings.Contains(content, "id: integer") {
			t.Error("Expected 'id: integer'")
		}
		if !strings.Contains(content, "email: text (nullable)") {
			t.Error("Expected 'email: text (nullable)'")
		}
		if !strings.Contains(content, "Description: Primary key") {
			t.Error("Expected column description for id")
		}
		if !strings.Contains(content, "Description: User email address") {
			t.Error("Expected column description for email")
		}
	})

	t.Run("filter by schema name", func(t *testing.T) {
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
			"private.secrets": {
				SchemaName: "private",
				TableName:  "secrets",
				TableType:  "TABLE",
				Columns: []database.ColumnInfo{
					{
						ColumnName: "key",
						DataType:   "text",
						IsNullable: "NO",
					},
				},
			},
		}

		client := createMockClient(metadata)
		tool := GetSchemaInfoTool(client)

		// Request only public schema
		response, err := tool.Handler(map[string]interface{}{
			"schema_name": "public",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false")
		}

		content := response.Content[0].Text

		if !strings.Contains(content, "public.users") {
			t.Error("Expected 'public.users' to be included")
		}
		if strings.Contains(content, "private.secrets") {
			t.Error("Did not expect 'private.secrets' to be included")
		}
	})

	t.Run("multiple tables and schemas", func(t *testing.T) {
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
			"analytics.events": {
				SchemaName: "analytics",
				TableName:  "events",
				TableType:  "TABLE",
				Columns: []database.ColumnInfo{
					{
						ColumnName: "event_id",
						DataType:   "uuid",
						IsNullable: "NO",
					},
				},
			},
		}

		client := createMockClient(metadata)
		tool := GetSchemaInfoTool(client)
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		content := response.Content[0].Text

		if !strings.Contains(content, "public.users") {
			t.Error("Expected 'public.users'")
		}
		if !strings.Contains(content, "public.orders") {
			t.Error("Expected 'public.orders'")
		}
		if !strings.Contains(content, "analytics.events") {
			t.Error("Expected 'analytics.events'")
		}
	})

	t.Run("view table type", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.user_stats": {
				SchemaName:  "public",
				TableName:   "user_stats",
				TableType:   "VIEW",
				Description: "User statistics view",
				Columns: []database.ColumnInfo{
					{
						ColumnName: "total_users",
						DataType:   "bigint",
						IsNullable: "YES",
					},
				},
			},
		}

		client := createMockClient(metadata)
		tool := GetSchemaInfoTool(client)
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		content := response.Content[0].Text

		if !strings.Contains(content, "public.user_stats (VIEW)") {
			t.Error("Expected 'public.user_stats (VIEW)'")
		}
	})

	t.Run("table without description", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.simple": {
				SchemaName:  "public",
				TableName:   "simple",
				TableType:   "TABLE",
				Description: "",
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

		client := createMockClient(metadata)
		tool := GetSchemaInfoTool(client)
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		content := response.Content[0].Text

		if !strings.Contains(content, "public.simple (TABLE)") {
			t.Error("Expected 'public.simple (TABLE)'")
		}

		// Count occurrences of "Description:" - should not appear for empty descriptions
		descCount := strings.Count(content, "Description:")
		if descCount > 1 {
			// One for the header "Database Schema Information"
			t.Error("Should not contain description labels when descriptions are empty")
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

		client := createMockClient(metadata)
		tool := GetSchemaInfoTool(client)
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		content := response.Content[0].Text

		if !strings.Contains(content, "data: jsonb (nullable)") {
			t.Error("Expected 'data: jsonb (nullable)'")
		}
		if !strings.Contains(content, "tags: text[] (nullable)") {
			t.Error("Expected 'tags: text[] (nullable)'")
		}
		if !strings.Contains(content, "created_at: timestamp with time zone") {
			t.Error("Expected 'created_at: timestamp with time zone'")
		}
		// created_at is NOT nullable, so should not have (nullable) marker
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.Contains(line, "created_at: timestamp with time zone") && strings.Contains(line, "(nullable)") {
				t.Error("created_at should not be marked as nullable")
			}
		}
	})

	t.Run("invalid schema_name type", func(t *testing.T) {
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
		}

		client := createMockClient(metadata)
		tool := GetSchemaInfoTool(client)

		// Pass invalid type for schema_name (should be ignored and default to "")
		response, err := tool.Handler(map[string]interface{}{
			"schema_name": 123, // Invalid type
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected handler to gracefully handle invalid type")
		}

		content := response.Content[0].Text
		// Should show all schemas since invalid type defaults to ""
		if !strings.Contains(content, "public.users") {
			t.Error("Expected 'public.users' when schema_name has invalid type")
		}
	})
}
