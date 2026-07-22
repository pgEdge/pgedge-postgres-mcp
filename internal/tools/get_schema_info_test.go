/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/config"
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
		if !strings.Contains(content, "Failed to load database metadata") {
			t.Errorf("Expected metadata load failure message, got: %s", content)
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

	t.Run("partitioned table shows as PARTITIONED TABLE", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.events": {
				SchemaName:    "public",
				TableName:     "events",
				TableType:     "PARTITIONED TABLE",
				Description:   "Partitioned events table",
				IsPartitioned: true,
				IsPartition:   false,
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
					{
						ColumnName: "event_date",
						DataType:   "date",
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
		if response.IsError {
			t.Error("Expected IsError=false")
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "public\tevents\tPARTITIONED TABLE") {
			t.Error("Expected 'PARTITIONED TABLE' type in TSV output")
		}
	})

	t.Run("child partitions hidden by default", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.events": {
				SchemaName:    "public",
				TableName:     "events",
				TableType:     "PARTITIONED TABLE",
				IsPartitioned: true,
				IsPartition:   false,
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
				},
			},
			"public.events_2024_01": {
				SchemaName:    "public",
				TableName:     "events_2024_01",
				TableType:     "TABLE",
				IsPartitioned: false,
				IsPartition:   true,
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
				},
			},
			"public.events_2024_02": {
				SchemaName:    "public",
				TableName:     "events_2024_02",
				TableType:     "TABLE",
				IsPartitioned: false,
				IsPartition:   true,
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
				},
			},
			"public.users": {
				SchemaName:    "public",
				TableName:     "users",
				TableType:     "TABLE",
				IsPartitioned: false,
				IsPartition:   false,
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

		// Default call: child partitions should be hidden
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

		// Parent partitioned table should be visible
		if !strings.Contains(content, "public\tevents\tPARTITIONED TABLE") {
			t.Error("Expected partitioned parent table to be visible")
		}

		// Regular table should be visible
		if !strings.Contains(content, "public\tusers\tTABLE") {
			t.Error("Expected regular table to be visible")
		}

		// Child partitions should be hidden
		if strings.Contains(content, "events_2024_01") {
			t.Error("Expected child partition events_2024_01 to be hidden by default")
		}
		if strings.Contains(content, "events_2024_02") {
			t.Error("Expected child partition events_2024_02 to be hidden by default")
		}
	})

	t.Run("child partitions shown with include_partitions", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.events": {
				SchemaName:    "public",
				TableName:     "events",
				TableType:     "PARTITIONED TABLE",
				IsPartitioned: true,
				IsPartition:   false,
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
				},
			},
			"public.events_2024_01": {
				SchemaName:    "public",
				TableName:     "events_2024_01",
				TableType:     "TABLE",
				IsPartitioned: false,
				IsPartition:   true,
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

		// Call with include_partitions=true
		response, err := tool.Handler(map[string]interface{}{
			"schema_name":        "public",
			"include_partitions": true,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false")
		}

		content := response.Content[0].Text

		// Parent partitioned table should be visible
		if !strings.Contains(content, "public\tevents\tPARTITIONED TABLE") {
			t.Error("Expected partitioned parent table to be visible")
		}

		// Child partition should now be visible
		if !strings.Contains(content, "public\tevents_2024_01\tTABLE") {
			t.Error("Expected child partition to be visible when include_partitions=true")
		}
	})

	t.Run("auto-summary excludes child partitions from count", func(t *testing.T) {
		// Build >10 regular tables so auto-summary triggers (threshold is 10)
		metadata := map[string]database.TableInfo{}
		regularTables := []string{
			"users", "orders", "products", "categories", "invoices",
			"payments", "shipments", "reviews", "coupons", "sessions",
			"audit_log",
		}
		for _, name := range regularTables {
			metadata["public."+name] = database.TableInfo{
				SchemaName:    "public",
				TableName:     name,
				TableType:     "TABLE",
				IsPartitioned: false,
				IsPartition:   false,
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
				},
			}
		}

		// Add a partitioned parent table
		metadata["public.events"] = database.TableInfo{
			SchemaName:    "public",
			TableName:     "events",
			TableType:     "PARTITIONED TABLE",
			IsPartitioned: true,
			IsPartition:   false,
			Columns: []database.ColumnInfo{
				{
					ColumnName: "id",
					DataType:   "integer",
					IsNullable: "NO",
				},
			},
		}

		// Add child partitions (should NOT be counted)
		for _, name := range []string{"events_2024_01", "events_2024_02", "events_2024_03"} {
			metadata["public."+name] = database.TableInfo{
				SchemaName:    "public",
				TableName:     name,
				TableType:     "TABLE",
				IsPartitioned: false,
				IsPartition:   true,
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
				},
			}
		}

		client := createMockClient(metadata)
		tool := GetSchemaInfoTool(client)

		// No filters: should trigger auto-summary mode
		response, err := tool.Handler(map[string]interface{}{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false")
		}

		content := response.Content[0].Text

		// Should be in auto-summary mode (has the summary header)
		if !strings.Contains(content, "Database Schema Summary") {
			t.Fatal("Expected auto-summary mode to trigger for >10 tables")
		}

		// The count should be 12 (11 regular + 1 partitioned parent),
		// NOT 15 (which would include the 3 child partitions)
		if !strings.Contains(content, "Found 12 tables") {
			t.Errorf("Expected 'Found 12 tables' (excluding child partitions), got:\n%s", content)
		}

		// Child partition names must not appear in the summary
		if strings.Contains(content, "events_2024_01") {
			t.Error("Expected child partition events_2024_01 to be excluded from auto-summary")
		}
		if strings.Contains(content, "events_2024_02") {
			t.Error("Expected child partition events_2024_02 to be excluded from auto-summary")
		}
		if strings.Contains(content, "events_2024_03") {
			t.Error("Expected child partition events_2024_03 to be excluded from auto-summary")
		}

		// The parent partitioned table should be counted (it's in the
		// schema stats). We already verified the count is 12 which
		// includes the parent, confirming it was not excluded.
	})

	t.Run("child partitions hidden in compact mode", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.events": {
				SchemaName:    "public",
				TableName:     "events",
				TableType:     "PARTITIONED TABLE",
				IsPartitioned: true,
				IsPartition:   false,
				Columns: []database.ColumnInfo{
					{
						ColumnName: "id",
						DataType:   "integer",
						IsNullable: "NO",
					},
				},
			},
			"public.events_2024_01": {
				SchemaName:    "public",
				TableName:     "events_2024_01",
				TableType:     "TABLE",
				IsPartitioned: false,
				IsPartition:   true,
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

		// Compact mode: child partitions should be hidden by default
		response, err := tool.Handler(map[string]interface{}{
			"schema_name": "public",
			"compact":     true,
		})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}
		if response.IsError {
			t.Error("Expected IsError=false")
		}

		content := response.Content[0].Text

		if !strings.Contains(content, "public\tevents\tPARTITIONED TABLE") {
			t.Error("Expected partitioned parent in compact mode")
		}
		if strings.Contains(content, "events_2024_01") {
			t.Error("Expected child partition hidden in compact mode")
		}
	})
}

// TestGetSchemaInfoTool_UsesDisplayName is a regression test for issue
// #187: responses must show the operator-configured display name, never
// the raw (possibly internal-only) connection host.
func TestGetSchemaInfoTool_UsesDisplayName(t *testing.T) {
	internalHost := "10.244.1.7"
	cfg := &config.NamedDatabaseConfig{Name: "orders-prod", Host: internalHost, Port: 5432}

	t.Run("empty results path", func(t *testing.T) {
		client := database.NewTestClientWithConfig(
			"postgres://appuser:secret@10.244.1.7:5432/orders_prod",
			map[string]database.TableInfo{},
			cfg,
		)

		tool := GetSchemaInfoTool(client)
		response, err := tool.Handler(map[string]interface{}{"schema_name": "nonexistent"})
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "Connected to: orders-prod") {
			t.Errorf("expected display name in response, got: %s", content)
		}
		if strings.Contains(content, internalHost) {
			t.Errorf("BUG #187 reproduced: response leaked the raw host: %s", content)
		}
	})

	t.Run("normal results path", func(t *testing.T) {
		metadata := map[string]database.TableInfo{
			"public.users": {SchemaName: "public", TableName: "users", TableType: "TABLE"},
		}
		client := database.NewTestClientWithConfig(
			"postgres://appuser:secret@10.244.1.7:5432/orders_prod",
			metadata,
			cfg,
		)

		tool := GetSchemaInfoTool(client)
		response, err := tool.Handler(map[string]interface{}{})
		if err != nil {
			t.Fatalf("Handler returned error: %v", err)
		}

		content := response.Content[0].Text
		if !strings.Contains(content, "Database: orders-prod") {
			t.Errorf("expected display name in response, got: %s", content)
		}
		if strings.Contains(content, internalHost) {
			t.Errorf("BUG #187 reproduced: response leaked the raw host: %s", content)
		}
	})
}
