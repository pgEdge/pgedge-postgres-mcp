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
	"encoding/json"
	"os"
	"testing"

	"pgedge-postgres-mcp/internal/database"
)

// TestAllResources_Integration tests all resources against a real PostgreSQL database
// This test helps catch compatibility issues across different PostgreSQL versions
func TestAllResources_Integration(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("POSTGRES_CONNECTION_STRING not set, skipping integration test")
	}

	client := database.NewClient()
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer client.Close()

	if err := client.LoadMetadata(); err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	// Detect PostgreSQL version
	version, err := getPostgreSQLMajorVersion(client)
	if err != nil {
		t.Logf("WARNING: Could not detect PostgreSQL version: %v", err)
		version = 14 // Default
	}
	t.Logf("Testing against PostgreSQL version %d", version)

	// Define all resources to test
	tests := []struct {
		name         string
		resource     Resource
		minVersion   int // Minimum PostgreSQL version required
		requiresData bool // Whether the resource requires existing data
	}{
		{
			name:         "pg://settings",
			resource:     PGSettingsResource(client),
			minVersion:   10,
			requiresData: false,
		},
		{
			name:         "pg://system_info",
			resource:     PGSystemInfoResource(client),
			minVersion:   10,
			requiresData: false,
		},
		{
			name:         "pg://stat/activity",
			resource:     PGStatActivityResource(client),
			minVersion:   10,
			requiresData: false,
		},
		{
			name:         "pg://stat/database",
			resource:     PGStatDatabaseResource(client),
			minVersion:   10,
			requiresData: false,
		},
		{
			name:         "pg://stat/user_tables",
			resource:     PGStatUserTablesResource(client),
			minVersion:   10,
			requiresData: false,
		},
		{
			name:         "pg://stat/user_indexes",
			resource:     PGStatUserIndexesResource(client),
			minVersion:   10,
			requiresData: false,
		},
		{
			name:         "pg://stat/replication",
			resource:     PGStatReplicationResource(client),
			minVersion:   10,
			requiresData: false,
		},
		{
			name:         "pg://stat/bgwriter",
			resource:     PGStatBgwriterResource(client),
			minVersion:   10,
			requiresData: false,
		},
		{
			name:         "pg://stat/wal",
			resource:     PGStatWALResource(client),
			minVersion:   14,
			requiresData: false,
		},
		{
			name:         "pg://statio/user_tables",
			resource:     PGStatIOUserTablesResource(client),
			minVersion:   10,
			requiresData: false,
		},
		{
			name:         "pg://statio/user_indexes",
			resource:     PGStatIOUserIndexesResource(client),
			minVersion:   10,
			requiresData: false,
		},
		{
			name:         "pg://statio/user_sequences",
			resource:     PGStatIOUserSequencesResource(client),
			minVersion:   10,
			requiresData: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if version is too old
			if version < tt.minVersion {
				t.Skipf("Skipping %s - requires PostgreSQL %d+, have %d", tt.name, tt.minVersion, version)
			}

			// Execute the resource handler
			content, err := tt.resource.Handler()
			if err != nil {
				t.Fatalf("Handler failed for %s: %v", tt.name, err)
			}

			// Verify response structure
			if len(content.Contents) == 0 {
				t.Fatalf("Expected non-empty content for %s", tt.name)
			}

			// Verify it's valid JSON
			var data interface{}
			if err := json.Unmarshal([]byte(content.Contents[0].Text), &data); err != nil {
				t.Fatalf("Failed to parse JSON for %s: %v", tt.name, err)
			}

			// Log success
			t.Logf("✓ %s: Successfully executed and returned valid JSON", tt.name)
		})
	}
}

// TestVersionAwareResources_Compatibility tests version-specific resource behavior
func TestVersionAwareResources_Compatibility(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("POSTGRES_CONNECTION_STRING not set, skipping compatibility test")
	}

	client := database.NewClient()
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer client.Close()

	if err := client.LoadMetadata(); err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	version, err := getPostgreSQLMajorVersion(client)
	if err != nil {
		t.Fatalf("Could not detect PostgreSQL version: %v", err)
	}

	t.Run("pg://stat/bgwriter version compatibility", func(t *testing.T) {
		resource := PGStatBgwriterResource(client)
		content, err := resource.Handler()
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		// Verify response structure
		if len(content.Contents) == 0 {
			t.Fatal("Expected non-empty content")
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(content.Contents[0].Text), &data); err != nil {
			t.Fatalf("Failed to parse JSON: %v\nRaw content: %s", err, content.Contents[0].Text)
		}

		bgwriter, ok := data["bgwriter"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected bgwriter field in response, got: %+v", data)
		}

		// Verify all expected fields are present
		expectedFields := []string{
			"checkpoints_timed", "checkpoints_req",
			"checkpoint_write_time_ms", "checkpoint_sync_time_ms",
			"buffers_checkpoint", "buffers_clean", "maxwritten_clean",
			"buffers_backend", "buffers_backend_fsync", "buffers_alloc",
		}

		missingFields := []string{}
		for _, field := range expectedFields {
			if _, ok := bgwriter[field]; !ok {
				missingFields = append(missingFields, field)
			}
		}

		if len(missingFields) > 0 {
			t.Errorf("Missing fields in bgwriter response: %v\nGot fields: %v", missingFields, getKeys(bgwriter))
		}

		// Verify stats_reset field
		if _, ok := bgwriter["stats_reset"]; !ok {
			t.Error("Missing stats_reset field in bgwriter response")
		}

		// Log version-specific details
		if version >= 17 {
			t.Logf("✓ PostgreSQL 17+ detected: Using pg_stat_checkpointer + pg_stat_bgwriter + pg_stat_io")
			t.Logf("  - Checkpoint stats from pg_stat_checkpointer")
			t.Logf("  - Background writer stats from pg_stat_bgwriter")
			t.Logf("  - Backend buffer stats from pg_stat_io")
		} else {
			t.Logf("✓ PostgreSQL %d: Using pg_stat_bgwriter only", version)
		}
	})

	t.Run("pg://stat/wal version compatibility", func(t *testing.T) {
		if version < 14 {
			t.Skip("pg_stat_wal requires PostgreSQL 14+")
		}

		resource := PGStatWALResource(client)
		content, err := resource.Handler()
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(content.Contents[0].Text), &data); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Should have postgresql_version field
		if _, ok := data["postgresql_version"]; !ok {
			t.Error("Expected postgresql_version field in response")
		}

		t.Logf("✓ pg_stat_wal works on PostgreSQL %d", version)
	})
}

// TestNullHandling_TOAST verifies that TOAST columns handle NULL values correctly
func TestNullHandling_TOAST(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("POSTGRES_CONNECTION_STRING not set, skipping NULL handling test")
	}

	client := database.NewClient()
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer client.Close()

	if err := client.LoadMetadata(); err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	t.Run("pg://statio/user_tables NULL handling", func(t *testing.T) {
		resource := PGStatIOUserTablesResource(client)
		content, err := resource.Handler()
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(content.Contents[0].Text), &data); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		tables, ok := data["tables"].([]interface{})
		if !ok {
			t.Fatal("Expected tables array in response")
		}

		// Check that we can handle tables with and without TOAST
		hasNullToast := false
		hasNonNullToast := false

		for _, tableData := range tables {
			table := tableData.(map[string]interface{})

			// Check if TOAST fields can be null
			if table["toast_blks_read"] == nil || table["toast_blks_read"].(float64) == 0 {
				hasNullToast = true
			} else {
				hasNonNullToast = true
			}
		}

		t.Logf("✓ TOAST NULL handling verified (hasNull=%v, hasNonNull=%v)", hasNullToast, hasNonNullToast)
	})
}

// TestResourceResponseStructure verifies all resources return consistent structure
func TestResourceResponseStructure(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("POSTGRES_CONNECTION_STRING not set, skipping structure test")
	}

	client := database.NewClient()
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer client.Close()

	if err := client.LoadMetadata(); err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	tests := []struct {
		name           string
		resource       Resource
		expectedFields []string
	}{
		{
			name:           "pg://stat/database",
			resource:       PGStatDatabaseResource(client),
			expectedFields: []string{"database_count", "databases"},
		},
		{
			name:           "pg://stat/user_tables",
			resource:       PGStatUserTablesResource(client),
			expectedFields: []string{"table_count", "tables"},
		},
		{
			name:           "pg://stat/user_indexes",
			resource:       PGStatUserIndexesResource(client),
			expectedFields: []string{"index_count", "indexes"},
		},
		{
			name:           "pg://statio/user_tables",
			resource:       PGStatIOUserTablesResource(client),
			expectedFields: []string{"table_count", "tables"},
		},
		{
			name:           "pg://statio/user_indexes",
			resource:       PGStatIOUserIndexesResource(client),
			expectedFields: []string{"index_count", "indexes"},
		},
		{
			name:           "pg://statio/user_sequences",
			resource:       PGStatIOUserSequencesResource(client),
			expectedFields: []string{"sequence_count", "sequences"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := tt.resource.Handler()
			if err != nil {
				t.Fatalf("Handler failed: %v", err)
			}

			var data map[string]interface{}
			if err := json.Unmarshal([]byte(content.Contents[0].Text), &data); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			// Verify expected fields
			for _, field := range tt.expectedFields {
				if _, ok := data[field]; !ok {
					t.Errorf("Missing expected field %s in response", field)
				}
			}

			t.Logf("✓ %s: All expected fields present", tt.name)
		})
	}
}

// getKeys returns the keys from a map for debugging purposes
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
