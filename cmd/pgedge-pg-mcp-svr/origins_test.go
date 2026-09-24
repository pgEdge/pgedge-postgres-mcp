/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/mcp"
)

// oauthConfig builds a configuration with HTTP auth and OAuth switched
// on for the given issuer and configured origins.
func oauthConfig(issuer string, origins []string) *config.Config {
	cfg := &config.Config{}
	cfg.HTTP.Enabled = true
	cfg.HTTP.AllowedOrigins = origins
	cfg.HTTP.Auth.Enabled = true
	cfg.HTTP.Auth.OAuth.Issuer = issuer
	return cfg
}

// TestEffectiveAllowedOriginsDescribesThePolicyInForce covers the
// inverted startup log: main built its policy from http.allowed_origins
// alone whilst the request path built a second one with the issuer
// origin added, so an empty list with OAuth active was reported as
// "loopback origins only (any port)" whilst the server was in fact
// refusing http://localhost:5173.
func TestEffectiveAllowedOriginsDescribesThePolicyInForce(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *config.Config
		wantDescribe string
		rejected     string
		accepted     string
	}{
		{
			name:         "oauth active with no configured origins",
			cfg:          oauthConfig("http://localhost:8080", nil),
			wantDescribe: "http://localhost:8080",
			rejected:     "http://localhost:5173",
			accepted:     "http://localhost:8080",
		},
		{
			name:         "oauth active alongside a configured origin",
			cfg:          oauthConfig("https://mcp.example.com", []string{"https://app.example.com"}),
			wantDescribe: "https://app.example.com",
			rejected:     "https://evil.example.com",
			accepted:     "https://mcp.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := mcp.NewOriginPolicy(effectiveAllowedOrigins(tc.cfg))
			if err != nil {
				t.Fatalf("NewOriginPolicy: %v", err)
			}

			describe := policy.Describe()
			if !strings.Contains(describe, tc.wantDescribe) {
				t.Fatalf("Describe() = %q; want it to name %q", describe, tc.wantDescribe)
			}
			if strings.Contains(describe, "loopback origins only") {
				t.Fatalf("Describe() = %q; the policy in force is not loopback-only", describe)
			}
			if !policy.Allow(tc.accepted) {
				t.Fatalf("policy rejects %s, which it describes as allowed", tc.accepted)
			}
			if policy.Allow(tc.rejected) {
				t.Fatalf("policy accepts %s, which its description does not cover", tc.rejected)
			}
		})
	}
}

// TestEffectiveAllowedOriginsWithoutOAuth is the control: with OAuth
// off, the configured list is passed through untouched and an empty one
// still means loopback-only.
func TestEffectiveAllowedOriginsWithoutOAuth(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.Enabled = true

	if got := effectiveAllowedOrigins(cfg); len(got) != 0 {
		t.Fatalf("effectiveAllowedOrigins = %v; want none", got)
	}
	policy, err := mcp.NewOriginPolicy(effectiveAllowedOrigins(cfg))
	if err != nil {
		t.Fatalf("NewOriginPolicy: %v", err)
	}
	if !strings.Contains(policy.Describe(), "loopback origins only") {
		t.Fatalf("Describe() = %q; want the loopback-only description", policy.Describe())
	}
	if !policy.Allow("http://localhost:5173") {
		t.Fatal("loopback-only policy rejected a loopback origin")
	}
}
