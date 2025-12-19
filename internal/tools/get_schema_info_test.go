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

		// Check TSV header
		if !strings.Contains(content, "schema\ttable\ttype\ttable_desc\tcolumn\tdata_type\tnullable\tcol_desc\tis_pk\tis_unique\tfk_ref\tis_indexed\tidentity\tdefault\tis_vector\tvector_dims") {
			t.Error("Expected TSV header row")
		}

		// Check TSV data rows contain expected values
		if !strings.Contains(content, "public\tusers\tTABLE\tUser accounts table\tid\tinteger\tNO\tPrimary key") {
			t.Error("Expected TSV row for id column")
		}
		if !strings.Contains(content, "public\tusers\tTABLE\tUser accounts table\temail\ttext\tYES\tUser email address") {
			t.Error("Expected TSV row for email column")
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

		// TSV format: check for public schema rows
		if !strings.Contains(content, "public\tusers") {
			t.Error("Expected 'public\\tusers' to be included in TSV")
		}
		if strings.Contains(content, "private\tsecrets") {
			t.Error("Did not expect 'private\\tsecrets' to be included")
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

		// TSV format: check for table rows
		if !strings.Contains(content, "public\tusers") {
			t.Error("Expected 'public\\tusers' in TSV")
		}
		if !strings.Contains(content, "public\torders") {
			t.Error("Expected 'public\\torders' in TSV")
		}
		if !strings.Contains(content, "analytics\tevents") {
			t.Error("Expected 'analytics\\tevents' in TSV")
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

		// TSV format: check for VIEW type in row
		if !strings.Contains(content, "public\tuser_stats\tVIEW") {
			t.Error("Expected 'public\\tuser_stats\\tVIEW' in TSV")
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

		// TSV format: check for table row with empty descriptions
		if !strings.Contains(content, "public\tsimple\tTABLE") {
			t.Error("Expected 'public\\tsimple\\tTABLE' in TSV")
		}
		// In TSV, empty descriptions are just empty fields (tabs next to each other)
		if !strings.Contains(content, "public\tsimple\tTABLE\t\tid\tinteger\tNO\t") {
			t.Error("Expected empty description fields in TSV row")
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

		// TSV format: check for column rows with correct types and nullable status
		if !strings.Contains(content, "\tdata\tjsonb\tYES\t") {
			t.Error("Expected TSV row with 'data\\tjsonb\\tYES'")
		}
		if !strings.Contains(content, "\ttags\ttext[]\tYES\t") {
			t.Error("Expected TSV row with 'tags\\ttext[]\\tYES'")
		}
		if !strings.Contains(content, "\tcreated_at\ttimestamp with time zone\tNO\t") {
			t.Error("Expected TSV row with 'created_at\\ttimestamp with time zone\\tNO'")
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
		// TSV format: check for table row
		if !strings.Contains(content, "public\tusers") {
			t.Error("Expected 'public\\tusers' in TSV when schema_name has invalid type")
		}
	})

	t.Run("filter by schema and table name", func(t *testing.T) {
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
						Description: "User email",
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

		client := createMockClient(metadata)
		tool := GetSchemaInfoTool(client)

		// Request only users table in public schema
		response, err := tool.Handler(map[string]interface{}{
			"schema_name": "public",
			"table_name":  "users",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false")
		}

		content := response.Content[0].Text

		// Should contain users table columns
		if !strings.Contains(content, "public\tusers\tTABLE") {
			t.Error("Expected 'public\\tusers\\tTABLE' in TSV")
		}
		if !strings.Contains(content, "\tid\tinteger\tNO\t") {
			t.Error("Expected id column in TSV")
		}
		if !strings.Contains(content, "\temail\ttext\tYES\t") {
			t.Error("Expected email column in TSV")
		}

		// Should NOT contain orders table
		if strings.Contains(content, "orders") {
			t.Error("Did not expect 'orders' table in response")
		}
	})

	t.Run("table_name without schema_name returns error", func(t *testing.T) {
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

		// Request table_name without schema_name
		response, err := tool.Handler(map[string]interface{}{
			"table_name": "users",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if !response.IsError {
			t.Error("Expected IsError=true when table_name provided without schema_name")
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "table_name requires schema_name") {
			t.Errorf("Expected error about table_name requiring schema_name, got: %s", content)
		}
	})

	t.Run("non-existent table returns helpful message", func(t *testing.T) {
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

		// Request non-existent table
		response, err := tool.Handler(map[string]interface{}{
			"schema_name": "public",
			"table_name":  "nonexistent",
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false (empty results, not an error)")
		}

		content := response.Content[0].Text
		// Should have helpful message about table not found
		if !strings.Contains(content, "No tables found matching your criteria") {
			t.Error("Expected 'No tables found' message")
		}
		if !strings.Contains(content, "public.nonexistent") {
			t.Error("Expected table name in diagnosis")
		}
		if !strings.Contains(content, "<diagnosis>") {
			t.Error("Expected diagnosis section")
		}
		if !strings.Contains(content, "<next_steps>") {
			t.Error("Expected next_steps section")
		}
	})

	t.Run("table_name with compact ignores compact", func(t *testing.T) {
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
						ColumnName: "email",
						DataType:   "text",
						IsNullable: "YES",
					},
				},
			},
		}

		client := createMockClient(metadata)
		tool := GetSchemaInfoTool(client)

		// Request with both table_name and compact=true
		// compact should be ignored when table_name is provided
		response, err := tool.Handler(map[string]interface{}{
			"schema_name": "public",
			"table_name":  "users",
			"compact":     true,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false")
		}

		content := response.Content[0].Text

		// Should have full column details (compact ignored)
		// Full mode header has more columns than compact mode
		if !strings.Contains(content, "schema\ttable\ttype\ttable_desc\tcolumn\tdata_type\tnullable\tcol_desc\tis_pk\tis_unique\tfk_ref\tis_indexed\tidentity\tdefault\tis_vector\tvector_dims") {
			t.Error("Expected full TSV header (compact should be ignored)")
		}
		// Should contain column details
		if !strings.Contains(content, "\tid\tinteger\tNO\t") {
			t.Error("Expected id column details in TSV")
		}
		if !strings.Contains(content, "\temail\ttext\tYES\t") {
			t.Error("Expected email column details in TSV")
		}
	})
}
