/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package resources

import (
	"encoding/json"
	"os"
	"testing"

	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
)

// TestPGSystemInfoResource_UsesConfiguredDisplayValues is a regression test
// for issue #187: pg://system_info must report the operator-configured
// host/port/name, never the live-resolved server address from
// inet_server_addr(), which can be an internal-only address (a container
// or pod IP) that differs from, and may be unreachable via, the address
// the operator actually configured.
func TestPGSystemInfoResource_UsesConfiguredDisplayValues(t *testing.T) {
	connStr := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connStr == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set; skipping live-DB regression test for issue #187")
	}

	// A deliberately distinctive configured host/name pair. The live
	// server may report a completely different address (e.g. a Docker
	// bridge IP) via inet_server_addr() - if that ever leaks through,
	// this test must fail.
	const configuredHost = "configured-display-host.example.internal"
	const configuredName = "orders-prod-display-name"
	cfg := &config.NamedDatabaseConfig{
		Name: configuredName,
		Host: configuredHost,
		Port: 6543,
	}

	client := database.NewClientWithConnectionString(connStr, cfg)
	defer client.Close()

	if err := client.ConnectTo(connStr); err != nil {
		t.Fatalf("ConnectTo failed: %v", err)
	}
	if err := client.LoadMetadata(); err != nil {
		t.Fatalf("LoadMetadata failed: %v", err)
	}

	resource := PGSystemInfoResource(client)
	content, err := resource.Handler()
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
	if len(content.Contents) == 0 {
		t.Fatal("expected non-empty content")
	}

	var info SystemInfo
	if err := json.Unmarshal([]byte(content.Contents[0].Text), &info); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if info.Host != configuredHost {
		t.Errorf("host = %q, want configured host %q", info.Host, configuredHost)
	}
	if info.Port != 6543 {
		t.Errorf("port = %d, want configured port 6543", info.Port)
	}
	if info.ConnectionName != configuredName {
		t.Errorf("connection_name = %q, want %q", info.ConnectionName, configuredName)
	}

	// The definitive check: whatever inet_server_addr() actually resolves
	// to on this connection, it must not be what the caller sees.
	if info.Host == "unix socket" && configuredHost != "unix socket" {
		t.Errorf("BUG #187 reproduced: host fell back to the live-resolved value instead of the configured one")
	}
}
