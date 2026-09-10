/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package oauth

import (
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/config"
)

// TestNewRejectsIncompleteOptions covers the two Options fields New
// cannot supply a default for: without an issuer it has no identity to
// publish, and without an authenticator the login form would panic on a
// nil interface at the first sign-in attempt rather than at startup.
func TestNewRejectsIncompleteOptions(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{
			name: "no issuer",
			opts: Options{Authenticator: &fakeAuthenticator{}},
			want: "issuer must not be empty",
		},
		{
			name: "no authenticator",
			opts: Options{Config: minimalConfig()},
			want: "authenticator must be provided",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, err := New(tc.opts)
			if err == nil {
				srv.Close()
				t.Fatalf("New succeeded; want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("New error = %v; want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestNewAcceptsCompleteOptions is the positive control for the checks
// above: the same configuration with both fields set must succeed.
func TestNewAcceptsCompleteOptions(t *testing.T) {
	srv, err := New(Options{Config: minimalConfig(), Authenticator: &fakeAuthenticator{}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv.Close()
}

// minimalConfig is the smallest OAuthConfig New will accept.
func minimalConfig() config.OAuthConfig {
	return config.OAuthConfig{
		Issuer:    "https://mcp.example.com",
		LoginPage: config.LoginPageConfig{PrimaryColour: "#000", SecondaryColour: "#000"},
	}
}
