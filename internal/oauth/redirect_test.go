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

import "testing"

func TestRedirectURIAllowed(t *testing.T) {
	allowed := []string{"https://claude.ai/api/mcp/auth_callback", "http://127.0.0.1/callback", "http://localhost/callback"}
	cases := map[string]bool{
		"https://claude.ai/api/mcp/auth_callback":        true,
		"https://claude.ai/api/mcp/auth_callback?x=1":    false,
		"https://evil.example.com/api/mcp/auth_callback": false,
		"http://127.0.0.1:53211/callback":                true,
		"http://localhost:8000/callback":                 true,
		"http://localhost:8000/other":                    false,
		"https://127.0.0.1:53211/callback":               false,
		"http://127.0.0.2:53211/callback":                false,
		"http://attacker@127.0.0.1:9999/callback":        false,
		"HTTPS://Claude.AI/api/mcp/auth_callback":        true,
		"https://claude.ai/API/mcp/auth_callback":        false,
	}
	for uri, want := range cases {
		if got := redirectURIAllowed(uri, allowed); got != want {
			t.Errorf("%s: got %v want %v", uri, got, want)
		}
	}
}
