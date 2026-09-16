/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDiscoveryIsBounded covers the finding that OAuth discovery ran
// with no deadline of its own: a server that accepts the connection and
// then never answers hung the CLI at startup, before anything had been
// printed, with no way to tell what it was waiting for.
func TestDiscoveryIsBounded(t *testing.T) {
	// Shorten the deadline so the test does not sit through the real
	// one. The tests in this package do not run in parallel, so the
	// variable is not being read concurrently.
	original := oauthDiscoveryTimeout
	oauthDiscoveryTimeout = 200 * time.Millisecond
	t.Cleanup(func() { oauthDiscoveryTimeout = original })

	released := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(_ http.ResponseWriter, r *http.Request) {
		// Answer nothing at all until the test is finished with it.
		select {
		case <-released:
		case <-r.Context().Done():
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(func() { close(released); srv.Close() })

	c := newTestChatClient(srv.URL, "auto", "static-token")

	done := make(chan error, 1)
	go func() { done <- c.connectToMCP(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("connectToMCP: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("connectToMCP hung on a server that never answered discovery")
	}

	if c.oauth != nil {
		t.Error("an OAuth client was created despite discovery timing out")
	}
	if c.mcp == nil {
		t.Error("auto mode did not fall back to the configured authentication")
	}
}

// TestTokenKeepsTheCacheOnTransportFailure covers the finding that any
// refresh failure cleared the on-disk token cache, so a momentary
// network problem destroyed a refresh token the server still considered
// valid and forced a fresh interactive sign-in.
func TestTokenKeepsTheCacheOnTransportFailure(t *testing.T) {
	f := newFakeOAuthServer(t)
	baseURL := f.srv.URL
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")

	now := time.Now()
	writeTestCache(t, cachePath, map[string]oauthCacheEntry{
		baseURL: {
			ClientID:     "client-1",
			AccessToken:  "stale-access",
			RefreshToken: "refresh-seed",
			ExpiresAt:    now.Add(30 * time.Second),
		},
	})

	oc := &OAuthClient{BaseURL: baseURL, CachePath: cachePath, Now: func() time.Time { return now }}
	// Discover the metadata whilst the server is still up, then take it
	// away, so the refresh itself fails in transit rather than being
	// answered.
	if err := oc.ensureMeta(context.Background()); err != nil {
		t.Fatalf("ensureMeta: %v", err)
	}
	f.srv.Close()

	if token := oc.Token(); token != "" {
		t.Fatalf("Token() = %q; want an empty string when the refresh could not be made", token)
	}

	cache, err := loadOAuthCache(cachePath)
	if err != nil {
		t.Fatalf("loadOAuthCache: %v", err)
	}
	entry, ok := cache[oc.cacheKey()]
	if !ok || entry.RefreshToken != "refresh-seed" {
		data, _ := os.ReadFile(cachePath)
		t.Fatalf("a transport failure discarded the refresh token; cache is now %s", data)
	}
}

// TestTokenClearsTheCacheOnInvalidGrant is the control: a definitive
// rejection does mean the cached refresh token is worthless, and it
// must still be discarded.
func TestTokenClearsTheCacheOnInvalidGrant(t *testing.T) {
	f := newFakeOAuthServer(t)
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")

	now := time.Now()
	writeTestCache(t, cachePath, map[string]oauthCacheEntry{
		f.srv.URL: {
			ClientID:     "client-1",
			AccessToken:  "stale-access",
			RefreshToken: "not-a-known-refresh-token",
			ExpiresAt:    now.Add(30 * time.Second),
		},
	})

	oc := &OAuthClient{BaseURL: f.srv.URL, CachePath: cachePath, Now: func() time.Time { return now }}

	if token := oc.Token(); token != "" {
		t.Fatalf("Token() = %q; want an empty string after a rejected refresh", token)
	}

	cache, err := loadOAuthCache(cachePath)
	if err != nil {
		t.Fatalf("loadOAuthCache: %v", err)
	}
	if _, ok := cache[oc.cacheKey()]; ok {
		t.Fatal("a rejected refresh left the cached entry in place")
	}
}
