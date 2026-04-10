/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
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
		ConnString:       "postgres://localhost/test",
		Metadata:         make(map[string]TableInfo),
		MetadataLoaded:   true,
		MetadataLoadedAt: time.Now(),
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
	// Test with non-existent connection
	client := NewClient(nil)
	loaded := client.IsMetadataLoadedFor("postgres://localhost/nonexistent")
	if loaded {
		t.Error("IsMetadataLoadedFor() returned true for non-existent connection")
	}

	// Test with metadata not loaded
	client.connections["postgres://localhost/test1"] = &ConnectionInfo{
		ConnString:     "postgres://localhost/test1",
		Metadata:       make(map[string]TableInfo),
		MetadataLoaded: false,
	}
	loaded = client.IsMetadataLoadedFor("postgres://localhost/test1")
	if loaded {
		t.Error("IsMetadataLoadedFor() returned true when metadata not loaded")
	}

	// Test with metadata loaded and fresh (within default 5m TTL)
	client.connections["postgres://localhost/test2"] = &ConnectionInfo{
		ConnString:       "postgres://localhost/test2",
		Metadata:         make(map[string]TableInfo),
		MetadataLoaded:   true,
		MetadataLoadedAt: time.Now(),
	}
	loaded = client.IsMetadataLoadedFor("postgres://localhost/test2")
	if !loaded {
		t.Error("IsMetadataLoadedFor() returned false for fresh metadata")
	}

	// Test with metadata loaded but stale (older than default 5m TTL)
	client.connections["postgres://localhost/test3"] = &ConnectionInfo{
		ConnString:       "postgres://localhost/test3",
		Metadata:         make(map[string]TableInfo),
		MetadataLoaded:   true,
		MetadataLoadedAt: time.Now().Add(-6 * time.Minute),
	}
	loaded = client.IsMetadataLoadedFor("postgres://localhost/test3")
	if loaded {
		t.Error("IsMetadataLoadedFor() returned true for stale metadata")
	}

	// Test with TTL=0 (always refresh)
	clientZero := NewClient(&config.NamedDatabaseConfig{
		MetadataTTL: "0",
	})
	clientZero.connections["postgres://localhost/test4"] = &ConnectionInfo{
		ConnString:       "postgres://localhost/test4",
		Metadata:         make(map[string]TableInfo),
		MetadataLoaded:   true,
		MetadataLoadedAt: time.Now(),
	}
	loaded = clientZero.IsMetadataLoadedFor("postgres://localhost/test4")
	if loaded {
		t.Error("IsMetadataLoadedFor() returned true with TTL=0 (should always refresh)")
	}

	// Test with custom TTL (10 minutes) and fresh metadata
	clientCustom := NewClient(&config.NamedDatabaseConfig{
		MetadataTTL: "10m",
	})
	clientCustom.connections["postgres://localhost/test5"] = &ConnectionInfo{
		ConnString:       "postgres://localhost/test5",
		Metadata:         make(map[string]TableInfo),
		MetadataLoaded:   true,
		MetadataLoadedAt: time.Now().Add(-7 * time.Minute),
	}
	loaded = clientCustom.IsMetadataLoadedFor("postgres://localhost/test5")
	if !loaded {
		t.Error("IsMetadataLoadedFor() returned false for metadata within custom 10m TTL")
	}

	// Test with custom TTL (10 minutes) and stale metadata
	clientCustom.connections["postgres://localhost/test6"] = &ConnectionInfo{
		ConnString:       "postgres://localhost/test6",
		Metadata:         make(map[string]TableInfo),
		MetadataLoaded:   true,
		MetadataLoadedAt: time.Now().Add(-11 * time.Minute),
	}
	loaded = clientCustom.IsMetadataLoadedFor("postgres://localhost/test6")
	if loaded {
		t.Error("IsMetadataLoadedFor() returned true for metadata beyond custom 10m TTL")
	}

	// Test with invalid TTL (falls back to 5m default)
	clientInvalid := NewClient(&config.NamedDatabaseConfig{
		MetadataTTL: "not-a-duration",
	})
	clientInvalid.connections["postgres://localhost/test7"] = &ConnectionInfo{
		ConnString:       "postgres://localhost/test7",
		Metadata:         make(map[string]TableInfo),
		MetadataLoaded:   true,
		MetadataLoadedAt: time.Now().Add(-4 * time.Minute),
	}
	loaded = clientInvalid.IsMetadataLoadedFor("postgres://localhost/test7")
	if !loaded {
		t.Error("IsMetadataLoadedFor() returned false for metadata within default 5m TTL (invalid TTL string)")
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
		ConnString:       "postgres://localhost/test",
		Metadata:         mockMetadata,
		MetadataLoaded:   true,
		MetadataLoadedAt: time.Now(),
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

func TestSetApplicationName(t *testing.T) {
	tests := []struct {
		name     string
		connStr  string
		appName  string
		wantName string
	}{
		{
			name:     "single host",
			connStr:  "postgres://user@localhost:5432/db",
			appName:  "test-app",
			wantName: "test-app",
		},
		{
			name:     "multi-host",
			connStr:  "postgres://user@host1:5432,host2:5433/db",
			appName:  "test-app",
			wantName: "test-app",
		},
		{
			name:     "already has application_name",
			connStr:  "postgres://user@host1:5432/db?application_name=existing",
			appName:  "test-app",
			wantName: "existing",
		},
		{
			name:     "multi-host with target_session_attrs",
			connStr:  "postgres://user@h1:5432,h2:5432/db?target_session_attrs=read-write",
			appName:  "test-app",
			wantName: "test-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := pgxpool.ParseConfig(tt.connStr)
			if err != nil {
				t.Fatalf("failed to parse connection string: %v", err)
			}

			setApplicationName(cfg, tt.appName)

			got := cfg.ConnConfig.RuntimeParams["application_name"]
			if got != tt.wantName {
				t.Errorf("application_name = %q, want %q", got, tt.wantName)
			}
		})
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

func TestClient_IsClosed(t *testing.T) {
	client := NewClient(nil)

	// New client should not be closed
	if client.IsClosed() {
		t.Error("IsClosed() returned true for new client")
	}

	// After Close(), should report closed
	client.Close()
	if !client.IsClosed() {
		t.Error("IsClosed() returned false after Close()")
	}
}

func TestConnectTo_TimesOutForUnreachableHost(t *testing.T) {
	// Use an unreachable address (RFC 5737 TEST-NET-1) with a short timeout
	// to verify that ConnectTo respects the connect_timeout setting.
	dbCfg := &config.NamedDatabaseConfig{
		Name:           "timeout-test",
		Host:           "192.0.2.1",
		Port:           5432,
		User:           "postgres",
		Database:       "testdb",
		ConnectTimeout: "2s",
	}

	client := NewClient(dbCfg)
	connStr := dbCfg.BuildConnectionString()

	start := time.Now()
	err := client.ConnectTo(connStr)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("ConnectTo() should have returned an error for unreachable host")
	}

	// The connection should fail within a reasonable margin of the timeout,
	// not the default 60+ seconds.
	if elapsed > 10*time.Second {
		t.Errorf("ConnectTo() took %v; expected it to fail within ~2s due to connect_timeout", elapsed)
	}
}

func TestConnectTo_InvalidConnectTimeout(t *testing.T) {
	dbCfg := &config.NamedDatabaseConfig{
		Name:           "bad-timeout",
		Host:           "localhost",
		Port:           5432,
		User:           "postgres",
		Database:       "testdb",
		ConnectTimeout: "not-a-duration",
	}

	client := NewClient(dbCfg)
	connStr := dbCfg.BuildConnectionString()

	err := client.ConnectTo(connStr)
	if err == nil {
		t.Fatal("ConnectTo() should have returned an error for invalid connect_timeout")
	}

	expected := "invalid connect_timeout"
	if !containsString(err.Error(), expected) {
		t.Errorf("error message %q should contain %q", err.Error(), expected)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
