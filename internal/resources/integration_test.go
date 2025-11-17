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
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping integration test")
	}

	client := database.NewClient(nil)
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer client.Close()

	if err := client.LoadMetadata(); err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	// Define all resources to test
	tests := []struct {
		name         string
		resource     Resource
		requiresData bool // Whether the resource requires existing data
	}{
		{
			name:         "pg://system_info",
			resource:     PGSystemInfoResource(client),
			requiresData: false,
		},
		{
			name:         "pg://database/schema",
			resource:     PGDatabaseSchemaResource(client),
			requiresData: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
