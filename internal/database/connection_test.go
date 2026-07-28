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
	"context"
	"database/sql"
	"os"
	"reflect"
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

// TestLoadMetadata_TableWithNoColumns is a regression test for issue #126.
//
// Before the fix, LoadMetadata's row scan declared columnName/dataType/
// isNullable as plain string. The metadata query LEFT JOINs against
// column_info, so a table with zero columns (e.g. CREATE TABLE foo()) yields
// a row whose ci.* fields are all NULL — pgx then aborts the row scan with
// "cannot scan NULL into *string", failing the entire metadata load and
// surfacing as the misleading "no database connection configured" error.
//
// This test creates such a table, loads metadata, and asserts that the load
// succeeds and that the empty table is present in the metadata with zero
// columns. It is gated on TEST_PGEDGE_POSTGRES_CONNECTION_STRING, matching
// the convention used by tests in the test/ package.
func TestLoadMetadata_TableWithNoColumns(t *testing.T) {
	connStr := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connStr == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set; skipping live-DB regression test for issue #126")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a dedicated pool to set up and tear down the fixture so we do not
	// interfere with whatever the Client builds internally.
	setupPool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to open setup pool: %v", err)
	}
	defer setupPool.Close()

	const tableName = "pgedge_mcp_issue126_empty"
	if _, err := setupPool.Exec(ctx, "DROP TABLE IF EXISTS public."+tableName); err != nil {
		t.Fatalf("failed to drop preexisting fixture table: %v", err)
	}
	if _, err := setupPool.Exec(ctx, "CREATE TABLE public."+tableName+"()"); err != nil {
		t.Fatalf("failed to create empty-columns fixture table: %v", err)
	}
	defer func() {
		_, _ = setupPool.Exec(context.Background(), "DROP TABLE IF EXISTS public."+tableName)
	}()

	client := NewClientWithConnectionString(connStr, nil)
	defer client.Close()

	if err := client.ConnectTo(connStr); err != nil {
		t.Fatalf("ConnectTo failed: %v", err)
	}

	if err := client.LoadMetadataFor(connStr); err != nil {
		t.Fatalf("LoadMetadataFor returned error for database containing a zero-column table; this is the regression in issue #126: %v", err)
	}

	meta := client.GetMetadataFor(connStr)
	key := "public." + tableName
	tableInfo, ok := meta[key]
	if !ok {
		t.Fatalf("expected metadata to contain %q, got keys: %v", key, mapKeys(meta))
	}
	if len(tableInfo.Columns) != 0 {
		t.Errorf("expected empty-columns table to have 0 columns, got %d", len(tableInfo.Columns))
	}
}

// TestLoadMetadata_ColumnWithMultipleFKs is a regression test for issue
// #171. A single column can participate in more than one foreign-key
// constraint; before the fix the fk_columns CTE emitted one row per FK,
// and the downstream LEFT JOIN multiplied the per-column rows, producing
// duplicate ColumnInfo entries for that column.
//
// This test creates a child column referenced by two different parent
// tables, loads metadata, and asserts that the column appears exactly
// once and that ForeignKeyRefs carries both references in deterministic
// (sorted) order. It is gated on TEST_PGEDGE_POSTGRES_CONNECTION_STRING,
// matching the convention used by the other live-DB regression tests.
func TestLoadMetadata_ColumnWithMultipleFKs(t *testing.T) {
	connStr := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connStr == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set; skipping live-DB regression test for issue #171")
	}

	cleanup := setupMultiFKFixture(t, connStr)
	defer cleanup()

	client := NewClientWithConnectionString(connStr, nil)
	defer client.Close()

	if err := client.ConnectTo(connStr); err != nil {
		t.Fatalf("ConnectTo failed: %v", err)
	}
	if err := client.LoadMetadataFor(connStr); err != nil {
		t.Fatalf("LoadMetadataFor returned error: %v", err)
	}

	meta := client.GetMetadataFor(connStr)
	key := "public.pgedge_mcp_issue171_child"
	tableInfo, ok := meta[key]
	if !ok {
		t.Fatalf("expected metadata to contain %q, got keys: %v", key, mapKeys(meta))
	}

	refCols := columnsNamed(tableInfo.Columns, "ref")
	if len(refCols) != 1 {
		t.Fatalf("expected column %q to appear exactly once, got %d (issue #171 duplicate rows)", "ref", len(refCols))
	}

	want := []string{
		"public.pgedge_mcp_issue171_parent_a.id",
		"public.pgedge_mcp_issue171_parent_b.id",
	}
	if !reflect.DeepEqual(refCols[0].ForeignKeyRefs, want) {
		t.Errorf("expected both FK references in sorted order:\n got:  %v\n want: %v",
			refCols[0].ForeignKeyRefs, want)
	}
}

// setupMultiFKFixture creates a child table whose `ref` column is
// referenced by two different parent tables (the issue #171 fixture) and
// returns a cleanup function that drops the fixture and closes the pool.
// It fails the test if any setup statement errors.
func setupMultiFKFixture(t *testing.T, connStr string) func() {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		cancel()
		t.Fatalf("failed to open setup pool: %v", err)
	}

	const dropStmt = "DROP TABLE IF EXISTS public.pgedge_mcp_issue171_child, " +
		"public.pgedge_mcp_issue171_parent_a, public.pgedge_mcp_issue171_parent_b CASCADE"
	stmts := []string{
		dropStmt,
		"CREATE TABLE public.pgedge_mcp_issue171_parent_a (id integer PRIMARY KEY)",
		"CREATE TABLE public.pgedge_mcp_issue171_parent_b (id integer PRIMARY KEY)",
		"CREATE TABLE public.pgedge_mcp_issue171_child (" +
			"ref integer, " +
			"CONSTRAINT fk_a FOREIGN KEY (ref) REFERENCES public.pgedge_mcp_issue171_parent_a(id), " +
			"CONSTRAINT fk_b FOREIGN KEY (ref) REFERENCES public.pgedge_mcp_issue171_parent_b(id))",
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			pool.Close()
			cancel()
			t.Fatalf("failed to set up fixture (%q): %v", stmt, err)
		}
	}

	return func() {
		_, _ = pool.Exec(context.Background(), dropStmt)
		pool.Close()
		cancel()
	}
}

// columnsNamed returns the columns in cols whose name equals name.
func columnsNamed(cols []ColumnInfo, name string) []ColumnInfo {
	var out []ColumnInfo
	for _, c := range cols {
		if c.ColumnName == name {
			out = append(out, c)
		}
	}
	return out
}

func mapKeys(m map[string]TableInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestVectorColumnInfo(t *testing.T) {
	cases := []struct {
		typeName string
		dataType string
		wantVec  bool
		wantType string
		wantDims int
	}{
		{"vector", "vector(1536)", true, "vector", 1536},
		{"halfvec", "halfvec(1024)", true, "halfvec", 1024},
		{"halfvec", "halfvec", true, "halfvec", 0},
		{"text", "text", false, "", 0},
	}
	for _, tc := range cases {
		r := metadataRow{
			TypeName: sql.NullString{String: tc.typeName, Valid: true},
			DataType: sql.NullString{String: tc.dataType, Valid: true},
		}
		gotVec, gotType, gotDims := vectorColumnInfo(r)
		if gotVec != tc.wantVec || gotType != tc.wantType || gotDims != tc.wantDims {
			t.Errorf("vectorColumnInfo(typeName=%q,dataType=%q) = (%v,%q,%d), want (%v,%q,%d)",
				tc.typeName, tc.dataType, gotVec, gotType, gotDims,
				tc.wantVec, tc.wantType, tc.wantDims)
		}
	}
}

// TestAfterReleaseRestoresReadOnlyDefault verifies the AfterRelease hook in
// isolation from every other read-only layer: it poisons a connection's
// session default directly, through the raw pgx connection rather than
// through any tool, so neither the statement guard (readonly_guard.go, which
// now rejects RESET ALL and SET SESSION CHARACTERISTICS before either reaches
// the database) nor a tool's own per-transaction pgx.ReadOnly access mode
// (set on every BeginTx regardless of the session default) can mask a broken
// reassertion. A test that poisons the session by calling a tool, as
// test/integration_test.go's smuggling subtests do, cannot tell this hook
// apart from those other two layers: both already prevent the poisoning
// statement from ever running, so the probe passes whether or not this hook
// still works. It is gated on TEST_PGEDGE_POSTGRES_CONNECTION_STRING,
// matching the convention used elsewhere in this file.
func TestAfterReleaseRestoresReadOnlyDefault(t *testing.T) {
	connStr := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connStr == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pinned to one connection, so the acquire after release is guaranteed
	// to return the exact connection this test poisoned, not a different
	// idle one.
	dbConfig := &config.NamedDatabaseConfig{PoolMaxConns: 1}
	client := NewClientWithConnectionString(connStr, dbConfig)
	defer client.Close()

	if err := client.ConnectTo(connStr); err != nil {
		t.Fatalf("ConnectTo failed: %v", err)
	}

	pool := client.GetPoolFor(connStr)
	if pool == nil {
		t.Fatal("GetPoolFor returned nil after a successful ConnectTo")
	}

	acquired, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("failed to acquire connection: %v", err)
	}

	// AfterConnect should have already set this to "on"; confirm the
	// baseline before poisoning it, so a failure below is legible as a
	// regression rather than the setting having never been "on" at all.
	var before string
	if err := acquired.QueryRow(ctx,
		"SELECT current_setting('default_transaction_read_only')").Scan(&before); err != nil {
		t.Fatalf("failed to read baseline setting: %v", err)
	}
	if before != "on" {
		t.Fatalf("default_transaction_read_only = %q immediately after connect, want \"on\"; "+
			"AfterConnect did not apply", before)
	}

	// Poison the session directly on the raw connection. This is the same
	// statement RESET ALL issues; going around every tool and the guard is
	// the point, since both already block RESET ALL from a caller.
	if _, err := acquired.Exec(ctx, "RESET ALL"); err != nil {
		t.Fatalf("failed to poison session: %v", err)
	}

	var poisoned string
	if err := acquired.QueryRow(ctx,
		"SELECT current_setting('default_transaction_read_only')").Scan(&poisoned); err != nil {
		t.Fatalf("failed to read poisoned setting: %v", err)
	}
	if poisoned != "off" {
		t.Fatalf("default_transaction_read_only = %q after RESET ALL, want \"off\"; "+
			"the poisoning step itself did not work, so this test cannot prove anything", poisoned)
	}

	// Releasing back to the pool is what should trigger AfterRelease.
	acquired.Release()

	reacquired, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("failed to reacquire connection: %v", err)
	}
	defer reacquired.Release()

	var after string
	if err := reacquired.QueryRow(ctx,
		"SELECT current_setting('default_transaction_read_only')").Scan(&after); err != nil {
		t.Fatalf("failed to read setting after reacquire: %v", err)
	}
	if after != "on" {
		t.Errorf("default_transaction_read_only = %q after release and reacquire, want \"on\"; "+
			"AfterRelease did not restore the session default", after)
	}
}
