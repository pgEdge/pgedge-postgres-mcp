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
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient(nil)

	if client == nil {
		t.Fatal("NewClient(nil) returned nil")
	}

	if client.connections == nil {
		t.Error("connections map is nil")
	}

	if len(client.connections) != 0 {
		t.Errorf("connections map should be empty, got %d entries", len(client.connections))
	}
}

func TestGetDefaultConnection(t *testing.T) {
	client := NewClient(nil)

	// Test initial state
	defaultConn := client.GetDefaultConnection()
	if defaultConn != "" {
		t.Errorf("GetDefaultConnection() = %q, want empty string", defaultConn)
	}

	// Test after setting default manually
	client.defaultConnStr = "postgres://localhost/test"
	defaultConn = client.GetDefaultConnection()
	if defaultConn != "postgres://localhost/test" {
		t.Errorf("GetDefaultConnection() = %q, want %q", defaultConn, "postgres://localhost/test")
	}
}

func TestListConnections(t *testing.T) {
	client := NewClient(nil)

	// Test with no connections
	connections := client.ListConnections()
	if len(connections) != 0 {
		t.Errorf("ListConnections() returned %d connections, want 0", len(connections))
	}

	// Add some mock connection info (without actual pools)
	client.connections["postgres://localhost/db1"] = &ConnectionInfo{
		ConnString:     "postgres://localhost/db1",
		Metadata:       make(map[string]TableInfo),
		MetadataLoaded: false,
	}
	client.connections["postgres://localhost/db2"] = &ConnectionInfo{
		ConnString:     "postgres://localhost/db2",
		Metadata:       make(map[string]TableInfo),
		MetadataLoaded: false,
	}

	connections = client.ListConnections()
	if len(connections) != 2 {
		t.Errorf("ListConnections() returned %d connections, want 2", len(connections))
	}

	// Verify both connection strings are in the list
	connMap := make(map[string]bool)
	for _, conn := range connections {
		connMap[conn] = true
	}

	if !connMap["postgres://localhost/db1"] {
		t.Error("ListConnections() missing postgres://localhost/db1")
	}
	if !connMap["postgres://localhost/db2"] {
		t.Error("ListConnections() missing postgres://localhost/db2")
	}
}

func TestGetConnectionInfo(t *testing.T) {
	client := NewClient(nil)

	// Test with non-existent connection
	info, exists := client.GetConnectionInfo("postgres://localhost/nonexistent")
	if exists {
		t.Error("GetConnectionInfo() returned exists=true for non-existent connection")
	}
	if info != nil {
		t.Error("GetConnectionInfo() returned non-nil info for non-existent connection")
	}

	// Add a mock connection
	mockInfo := &ConnectionInfo{
		ConnString:     "postgres://localhost/test",
		Metadata:       make(map[string]TableInfo),
		MetadataLoaded: true,
	}
	client.connections["postgres://localhost/test"] = mockInfo

	// Test with existing connection
	info, exists = client.GetConnectionInfo("postgres://localhost/test")
	if !exists {
		t.Error("GetConnectionInfo() returned exists=false for existing connection")
	}
	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil info for existing connection")
	}
	if info.ConnString != "postgres://localhost/test" {
		t.Errorf("GetConnectionInfo() returned wrong ConnString: got %q, want %q", info.ConnString, "postgres://localhost/test")
	}
	if !info.MetadataLoaded {
		t.Error("GetConnectionInfo() returned MetadataLoaded=false, want true")
	}
}

func TestIsMetadataLoadedFor(t *testing.T) {
	client := NewClient(nil)

	// Test with non-existent connection
	loaded := client.IsMetadataLoadedFor("postgres://localhost/nonexistent")
	if loaded {
		t.Error("IsMetadataLoadedFor() returned true for non-existent connection")
	}

	// Add connection with metadata not loaded
	client.connections["postgres://localhost/test1"] = &ConnectionInfo{
		ConnString:     "postgres://localhost/test1",
		Metadata:       make(map[string]TableInfo),
		MetadataLoaded: false,
	}

	loaded = client.IsMetadataLoadedFor("postgres://localhost/test1")
	if loaded {
		t.Error("IsMetadataLoadedFor() returned true when metadata not loaded")
	}

	// Add connection with metadata loaded
	client.connections["postgres://localhost/test2"] = &ConnectionInfo{
		ConnString:     "postgres://localhost/test2",
		Metadata:       make(map[string]TableInfo),
		MetadataLoaded: true,
	}

	loaded = client.IsMetadataLoadedFor("postgres://localhost/test2")
	if !loaded {
		t.Error("IsMetadataLoadedFor() returned false when metadata is loaded")
	}
}

func TestGetMetadataFor(t *testing.T) {
	client := NewClient(nil)

	// Test with non-existent connection
	metadata := client.GetMetadataFor("postgres://localhost/nonexistent")
	if metadata == nil {
		t.Fatal("GetMetadataFor() returned nil for non-existent connection")
	}
	if len(metadata) != 0 {
		t.Errorf("GetMetadataFor() returned %d entries for non-existent connection, want 0", len(metadata))
	}

	// Add connection with metadata
	mockMetadata := map[string]TableInfo{
		"public.users": {
			SchemaName: "public",
			TableName:  "users",
			TableType:  "TABLE",
			Columns: []ColumnInfo{
				{
					ColumnName: "id",
					DataType:   "integer",
					IsNullable: "NO",
				},
				{
					ColumnName: "name",
					DataType:   "text",
					IsNullable: "YES",
				},
			},
		},
		"public.orders": {
			SchemaName: "public",
			TableName:  "orders",
			TableType:  "TABLE",
			Columns: []ColumnInfo{
				{
					ColumnName: "id",
					DataType:   "integer",
					IsNullable: "NO",
				},
			},
		},
	}

	client.connections["postgres://localhost/test"] = &ConnectionInfo{
		ConnString:     "postgres://localhost/test",
		Metadata:       mockMetadata,
		MetadataLoaded: true,
	}

	metadata = client.GetMetadataFor("postgres://localhost/test")
	if len(metadata) != 2 {
		t.Errorf("GetMetadataFor() returned %d entries, want 2", len(metadata))
	}

	// Verify it's a copy (modifications shouldn't affect original)
	metadata["public.newTable"] = TableInfo{
		SchemaName: "public",
		TableName:  "newTable",
	}

	originalMetadata := client.GetMetadataFor("postgres://localhost/test")
	if len(originalMetadata) != 2 {
		t.Error("GetMetadataFor() returned a reference instead of a copy")
	}
}

func TestGetPoolFor(t *testing.T) {
	client := NewClient(nil)

	// Test with non-existent connection
	pool := client.GetPoolFor("postgres://localhost/nonexistent")
	if pool != nil {
		t.Error("GetPoolFor() returned non-nil pool for non-existent connection")
	}

	// Test with existing connection but nil pool
	client.connections["postgres://localhost/test"] = &ConnectionInfo{
		ConnString: "postgres://localhost/test",
		Pool:       nil,
	}

	pool = client.GetPoolFor("postgres://localhost/test")
	if pool != nil {
		t.Error("GetPoolFor() returned non-nil pool when Pool is nil")
	}
}

func TestClose(t *testing.T) {
	client := NewClient(nil)

	// Add some mock connections (without actual pools that need closing)
	client.connections["postgres://localhost/db1"] = &ConnectionInfo{
		ConnString: "postgres://localhost/db1",
		Pool:       nil,
	}
	client.connections["postgres://localhost/db2"] = &ConnectionInfo{
		ConnString: "postgres://localhost/db2",
		Pool:       nil,
	}

	// Close should clear all connections
	client.Close()

	if len(client.connections) != 0 {
		t.Errorf("After Close(), connections map has %d entries, want 0", len(client.connections))
	}
}
