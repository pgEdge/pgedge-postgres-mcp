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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// metadataServer serves body at the well-known metadata path with the
// given status, and nothing else.
func metadataServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newTestChatClient builds a Client wired for HTTP mode against url,
// with a static token available for the non-OAuth path.
func newTestChatClient(url, authMode, token string) *Client {
	cfg := &Config{}
	cfg.MCP.Mode = "http"
	cfg.MCP.URL = url
	cfg.MCP.AuthMode = authMode
	cfg.MCP.Token = token
	ui := NewUI(true, false)
	ui.DisplayStatusMessages = false
	return &Client{config: cfg, ui: ui}
}

// TestAutoModeFallsBackWhenMetadataIsUnauthorised covers the discovery
// fallback finding: a server that answers the well-known path with a
// 401 (because everything is behind authentication) is a server without
// OAuth as far as the CLI is concerned, and auto mode must fall back to
// the previous authentication rather than failing the connection.
func TestAutoModeFallsBackWhenMetadataIsUnauthorised(t *testing.T) {
	srv := metadataServer(t, http.StatusUnauthorized, `{"error":"Missing Authorization header"}`)
	c := newTestChatClient(srv.URL, "auto", "static-token")

	if err := c.connectToMCP(context.Background()); err != nil {
		t.Fatalf("connectToMCP: %v", err)
	}
	if c.oauth != nil {
		t.Error("OAuth client was created for a server that does not advertise OAuth")
	}
	if c.mcp == nil {
		t.Error("no MCP client was created")
	}
}

// TestAutoModeFallsBackOnTransportError covers the same fallback for a
// discovery request that never reaches a server at all.
func TestAutoModeFallsBackOnTransportError(t *testing.T) {
	srv := metadataServer(t, http.StatusOK, `{}`)
	url := srv.URL
	srv.Close()

	c := newTestChatClient(url, "auto", "static-token")
	if err := c.connectToMCP(context.Background()); err != nil {
		t.Fatalf("connectToMCP: %v", err)
	}
	if c.oauth != nil {
		t.Error("OAuth client was created despite discovery failing")
	}
}

// TestOAuthModeFailsWhenMetadataIsUnauthorised confirms the explicit
// oauth mode still reports the failure rather than falling back.
func TestOAuthModeFailsWhenMetadataIsUnauthorised(t *testing.T) {
	srv := metadataServer(t, http.StatusUnauthorized, `{"error":"Missing Authorization header"}`)
	c := newTestChatClient(srv.URL, "oauth", "")

	err := c.connectToMCP(context.Background())
	if err == nil {
		t.Fatal("expected an error in oauth mode")
	}
	if !strings.Contains(err.Error(), "OAuth") {
		t.Errorf("error %q does not mention OAuth", err)
	}
}

// TestDiscoverOAuthRejectsForeignIssuer and its endpoint counterpart
// cover the metadata validation finding: a document that names another
// origin cannot be trusted to point the CLI's credentials anywhere.
func TestDiscoverOAuthRejectsForeignIssuer(t *testing.T) {
	srv := metadataServer(t, http.StatusOK, `{"issuer":"https://evil.example.com",
		"authorization_endpoint":"https://evil.example.com/oauth/authorize",
		"token_endpoint":"https://evil.example.com/oauth/token"}`)

	if _, err := DiscoverOAuth(context.Background(), http.DefaultClient, srv.URL); !errors.Is(err, ErrNoOAuth) {
		t.Fatalf("got %v, want ErrNoOAuth", err)
	}
}

func TestDiscoverOAuthRejectsForeignEndpoint(t *testing.T) {
	// The issuer is built from the request's own Host, so the document
	// names this server correctly and only the token endpoint is
	// foreign.
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		origin := "http://" + r.Host
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(oauthMetadata{
			Issuer:                origin,
			AuthorizationEndpoint: origin + "/oauth/authorize",
			TokenEndpoint:         "https://evil.example.com/oauth/token",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	if _, err := DiscoverOAuth(context.Background(), http.DefaultClient, srv.URL); !errors.Is(err, ErrNoOAuth) {
		t.Fatalf("got %v, want ErrNoOAuth", err)
	}
}

// TestOAuthCacheKeyedByServerOrigin covers the finding that the token
// cache is keyed by the server the CLI was pointed at, not by whatever
// the metadata document calls itself.
func TestOAuthCacheKeyedByServerOrigin(t *testing.T) {
	f := newFakeOAuthServer(t)
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")
	forceLoopbackEnvironment(t)

	oc := &OAuthClient{
		BaseURL:   f.srv.URL + "/", // trailing slash, normalised away
		CachePath: cachePath,
		OpenURL:   loopbackOpenURL,
		Prompt:    func(string) {},
	}
	if err := oc.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	var cache map[string]oauthCacheEntry
	if err := yaml.Unmarshal(data, &cache); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache[f.srv.URL]; !ok {
		t.Fatalf("cache is not keyed by the server origin %q: %v", f.srv.URL, cache)
	}
}

// TestLoginRegistersAfreshEachTime covers the finding that a cached
// client id is not trusted for an interactive login: the server may
// have restarted and forgotten it.
func TestLoginRegistersAfreshEachTime(t *testing.T) {
	f := newFakeOAuthServer(t)
	cachePath := filepath.Join(t.TempDir(), "oauth-tokens.yaml")
	forceLoopbackEnvironment(t)

	seed := map[string]oauthCacheEntry{
		f.srv.URL: {ClientID: "forgotten-client"},
	}
	if err := saveOAuthCache(cachePath, seed); err != nil {
		t.Fatal(err)
	}

	oc := &OAuthClient{
		BaseURL:   f.srv.URL,
		CachePath: cachePath,
		OpenURL:   loopbackOpenURL,
		Prompt:    func(string) {},
	}
	if err := oc.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	var cache map[string]oauthCacheEntry
	if err := yaml.Unmarshal(data, &cache); err != nil {
		t.Fatal(err)
	}
	if got := cache[f.srv.URL].ClientID; got == "forgotten-client" || got == "" {
		t.Fatalf("client id = %q, want a freshly registered one", got)
	}
}

// TestBrowserURLScheme covers the browser opener finding: only http and
// https URLs are ever handed to the platform opener.
func TestBrowserURLScheme(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"https://mcp.example.com/oauth/authorize", true},
		{"http://127.0.0.1:8080/oauth/authorize", true},
		{"javascript:alert(1)", false},
		{"file:///etc/passwd", false},
		{"/oauth/authorize", false},
		{"", false},
	} {
		if got := browserSafeURL(tc.url); got != tc.want {
			t.Errorf("browserSafeURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

// TestSanitiseServerText covers the terminal output finding: control
// characters from a server-supplied string never reach the terminal.
func TestSanitiseServerText(t *testing.T) {
	got := sanitiseServerText("code \x1b[31mRED\x1b[0m\x07 here\nsecond line\r\n")
	if strings.ContainsAny(got, "\x1b\x07\r") {
		t.Fatalf("control characters survived: %q", got)
	}
	if !strings.Contains(got, "\n") {
		t.Fatalf("newlines should be kept: %q", got)
	}
	if !strings.Contains(got, "second line") {
		t.Fatalf("text was lost: %q", got)
	}
}
