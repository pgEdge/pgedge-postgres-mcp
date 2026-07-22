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

	"pgedge-postgres-mcp/internal/config"
)

// TestDisplayName_UsesConfiguredName is a regression test for issue #187:
// user-facing strings must show the operator-configured name, never the
// raw connection string.
func TestDisplayName_UsesConfiguredName(t *testing.T) {
	client := NewClientWithConnectionString(
		"postgres://appuser:secret@10.244.1.7:5432/orders_prod",
		&config.NamedDatabaseConfig{Name: "orders-prod", Host: "10.244.1.7", Port: 5432},
	)

	got := client.DisplayName()
	if got != "orders-prod" {
		t.Errorf("DisplayName() = %q, want %q", got, "orders-prod")
	}
	if got == "10.244.1.7" {
		t.Fatal("BUG #187 reproduced: DisplayName() leaked the raw host")
	}
}

// TestDisplayName_FallsBackWhenNoConfig covers the two cases where there is
// no configured name to substitute: no config at all, and a config with an
// empty name (e.g. an ad-hoc connection string the caller typed inline,
// which has no NamedDatabaseConfig behind it at all in practice, but the
// empty-Name case is tested directly since it exercises the same fallback
// branch). The password must still be masked either way.
func TestDisplayName_FallsBackWhenNoConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.NamedDatabaseConfig
	}{
		{name: "nil config", cfg: nil},
		{name: "config with empty name", cfg: &config.NamedDatabaseConfig{Host: "10.244.1.7", Port: 5432}},
	}

	connStr := "postgres://appuser:secret@10.244.1.7:5432/orders_prod"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClientWithConnectionString(connStr, tt.cfg)

			got := client.DisplayName()
			if got != "postgres://appuser:***@10.244.1.7:5432/orders_prod" {
				t.Errorf("DisplayName() = %q, want password-masked connection string", got)
			}
		})
	}
}

func TestConfiguredHost(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.NamedDatabaseConfig
		want string
	}{
		{
			name: "single host",
			cfg:  &config.NamedDatabaseConfig{Name: "db1", Host: "10.244.1.7", Port: 5432},
			want: "10.244.1.7",
		},
		{
			name: "multi-host uses first entry",
			cfg: &config.NamedDatabaseConfig{
				Name:  "db1",
				Hosts: []config.HostEntry{{Host: "10.244.1.7", Port: 5432}, {Host: "10.244.1.8", Port: 5432}},
			},
			want: "10.244.1.7",
		},
		{
			name: "nil config falls back to unix socket",
			cfg:  nil,
			want: "unix socket",
		},
		{
			name: "single-host config with omitted host defaults to localhost",
			cfg:  &config.NamedDatabaseConfig{Name: "db1", Port: 5432},
			want: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClientWithConnectionString("postgres://localhost/test", tt.cfg)
			if got := client.ConfiguredHost(); got != tt.want {
				t.Errorf("ConfiguredHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfiguredPort(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.NamedDatabaseConfig
		want int
	}{
		{
			name: "explicit port",
			cfg:  &config.NamedDatabaseConfig{Name: "db1", Host: "10.244.1.7", Port: 6543},
			want: 6543,
		},
		{
			name: "zero port defaults to 5432",
			cfg:  &config.NamedDatabaseConfig{Name: "db1", Host: "10.244.1.7"},
			want: 5432,
		},
		{
			name: "multi-host uses first entry's port",
			cfg: &config.NamedDatabaseConfig{
				Name:  "db1",
				Hosts: []config.HostEntry{{Host: "10.244.1.7", Port: 6543}, {Host: "10.244.1.8", Port: 6544}},
			},
			want: 6543,
		},
		{
			name: "multi-host first entry zero port defaults to 5432",
			cfg: &config.NamedDatabaseConfig{
				Name:  "db1",
				Hosts: []config.HostEntry{{Host: "10.244.1.7"}},
			},
			want: 5432,
		},
		{
			name: "nil config returns 0",
			cfg:  nil,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClientWithConnectionString("postgres://localhost/test", tt.cfg)
			if got := client.ConfiguredPort(); got != tt.want {
				t.Errorf("ConfiguredPort() = %d, want %d", got, tt.want)
			}
		})
	}
}
