/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	// Test HTTP defaults
	if cfg.HTTP.Enabled {
		t.Error("Expected HTTP to be disabled by default")
	}

	if cfg.HTTP.Address != ":8080" {
		t.Errorf("Expected default address ':8080', got %s", cfg.HTTP.Address)
	}

	if cfg.HTTP.TLS.Enabled {
		t.Error("Expected TLS to be disabled by default")
	}

	if !cfg.HTTP.Auth.Enabled {
		t.Error("Expected Auth to be enabled by default")
	}
}
