/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestExecuteResourceQuery(t *testing.T) {
	t.Run("database not ready", func(t *testing.T) {
		client := NewClient(nil)
		// Don't add any connections - database is not ready

		query := "SELECT * FROM test"
		processor := func(rows pgx.Rows) (interface{}, error) {
			return nil, nil
		}

		content, err := ExecuteResourceQuery(client, "test://uri", query, processor)

		// NewResourceError returns nil error, but includes error message in content
		if err != nil {
			t.Errorf("Expected nil error, got: %v", err)
		}
		if !strings.Contains(content.URI, "test://uri") {
			t.Errorf("Expected URI to be set, got: %s", content.URI)
		}
		if len(content.Contents) == 0 {
			t.Fatal("Expected content in response")
		}
		text := content.Contents[0].Text
		if !strings.Contains(text, "Database not ready") {
			t.Errorf("Expected database not ready message, got: %s", text)
		}
	})

	t.Run("processor error returns error", func(t *testing.T) {
		// This test verifies that processor errors are properly propagated
		// We can't easily test this without a real database or extensive mocking
		// but the pattern is tested in integration tests
		t.Skip("Requires real database connection or extensive mocking")
	})

	t.Run("successful query execution", func(t *testing.T) {
		// This test verifies the full happy path
		// Requires real database connection
		t.Skip("Requires real database connection - covered by integration tests")
	})
}

func TestRowProcessor(t *testing.T) {
	t.Run("processor function signature", func(t *testing.T) {
		// Verify the RowProcessor type signature is correct
		processor := RowProcessor(func(rows pgx.Rows) (interface{}, error) {
			return map[string]string{"test": "value"}, nil
		})

		if processor == nil {
			t.Error("Processor should not be nil")
		}
	})

	t.Run("processor can return various types", func(t *testing.T) {
		// Test that processor can return different types
		processors := []RowProcessor{
			func(rows pgx.Rows) (interface{}, error) {
				return []map[string]interface{}{{"key": "value"}}, nil
			},
			func(rows pgx.Rows) (interface{}, error) {
				return map[string]interface{}{"count": 42}, nil
			},
			func(rows pgx.Rows) (interface{}, error) {
				return "string result", nil
			},
		}

		for i, proc := range processors {
			result, err := proc(nil)
			if err != nil {
				t.Errorf("Processor %d returned error: %v", i, err)
			}
			if result == nil {
				t.Errorf("Processor %d returned nil result", i)
			}
		}
	})

	t.Run("processor can return errors", func(t *testing.T) {
		processor := func(rows pgx.Rows) (interface{}, error) {
			return nil, errors.New("test error")
		}

		result, err := processor(nil)
		if err == nil {
			t.Error("Expected error from processor")
		}
		if !strings.Contains(err.Error(), "test error") {
			t.Errorf("Expected 'test error', got: %v", err)
		}
		if result != nil {
			t.Error("Expected nil result when error occurs")
		}
	})
}

// Mock tests for resource patterns
func TestResourcePatterns(t *testing.T) {
	t.Run("single row result pattern", func(t *testing.T) {
		// Test pattern used by pg_stat_bgwriter (single row)
		processor := func(rows pgx.Rows) (interface{}, error) {
			// Simulates single-row query like pg_stat_bgwriter
			data := map[string]interface{}{
				"checkpoints_timed": 100,
				"checkpoints_req":   10,
			}
			return data, nil
		}

		result, err := processor(nil)
		if err != nil {
			t.Fatalf("Processor returned error: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected map[string]interface{} result")
		}

		if resultMap["checkpoints_timed"] != 100 {
			t.Error("Expected checkpoints_timed to be 100")
		}
	})

	t.Run("multi row result pattern", func(t *testing.T) {
		// Test pattern used by most resources (multiple rows)
		processor := func(rows pgx.Rows) (interface{}, error) {
			// Simulates multi-row query like pg_stat_activity
			activities := []map[string]interface{}{
				{"pid": 123, "state": "active"},
				{"pid": 456, "state": "idle"},
			}
			return map[string]interface{}{
				"activity_count": len(activities),
				"activities":     activities,
			}, nil
		}

		result, err := processor(nil)
		if err != nil {
			t.Fatalf("Processor returned error: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected map[string]interface{} result")
		}

		if resultMap["activity_count"] != 2 {
			t.Error("Expected activity_count to be 2")
		}
	})

	t.Run("version checking pattern", func(t *testing.T) {
		// Test pattern used by pg_stat_wal (version-dependent queries)
		checkVersion := func(versionNum int) bool {
			return versionNum >= 140000 // PostgreSQL 14+
		}

		if !checkVersion(150000) {
			t.Error("PostgreSQL 15 should pass version check for 14+")
		}
		if checkVersion(130000) {
			t.Error("PostgreSQL 13 should fail version check for 14+")
		}
	})

	t.Run("enum parsing pattern", func(t *testing.T) {
		// Test pattern used by pg_settings (parsing PostgreSQL arrays)
		enumValsArray := []interface{}{"on", "off", "auto"}
		var enumValues []string

		for _, v := range enumValsArray {
			if str, ok := v.(string); ok {
				enumValues = append(enumValues, str)
			}
		}

		if len(enumValues) != 3 {
			t.Errorf("Expected 3 enum values, got %d", len(enumValues))
		}
		if enumValues[0] != "on" || enumValues[1] != "off" || enumValues[2] != "auto" {
			t.Errorf("Enum values not parsed correctly: %v", enumValues)
		}
	})
}

// Integration-style tests for resource helper framework
func TestResourceHelperIntegration(t *testing.T) {
	t.Run("helper provides consistent error messages", func(t *testing.T) {
		client := NewClient(nil)

		query := "SELECT 1"
		processor := func(rows pgx.Rows) (interface{}, error) {
			return nil, nil
		}

		content, err := ExecuteResourceQuery(client, "test://resource", query, processor)

		// NewResourceError returns nil for error parameter
		if err != nil {
			t.Errorf("Expected nil error, got: %v", err)
		}
		if content.URI != "test://resource" {
			t.Errorf("Expected URI 'test://resource', got: %s", content.URI)
		}
		// Verify error message is in content
		if len(content.Contents) == 0 {
			t.Fatal("Expected content with error message")
		}
		if !strings.Contains(content.Contents[0].Text, "Database not ready") {
			t.Errorf("Expected error message in content, got: %s", content.Contents[0].Text)
		}
	})

	t.Run("helper handles nil processor", func(t *testing.T) {
		client := NewTestClient("postgres://test", make(map[string]TableInfo))

		query := "SELECT 1"

		// Test that calling with nil processor doesn't panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ExecuteResourceQuery panicked with nil processor: %v", r)
			}
		}()

		// This will fail at the metadata check, which is fine for this test
		_, _ = ExecuteResourceQuery(client, "test://resource", query, nil)
	})
}

// Test helper for creating mock connection info
func createMockConnInfo(loaded bool, pool *pgxpool.Pool) *ConnectionInfo {
	return &ConnectionInfo{
		ConnString:     "postgres://test",
		Pool:           pool,
		Metadata:       make(map[string]TableInfo),
		MetadataLoaded: loaded,
	}
}

func TestConnectionInfo(t *testing.T) {
	t.Run("connection info structure", func(t *testing.T) {
		info := createMockConnInfo(true, nil)

		if info.ConnString != "postgres://test" {
			t.Errorf("Expected ConnString 'postgres://test', got: %s", info.ConnString)
		}
		if !info.MetadataLoaded {
			t.Error("Expected MetadataLoaded to be true")
		}
		if info.Metadata == nil {
			t.Error("Expected Metadata to be initialized")
		}
	})
}
