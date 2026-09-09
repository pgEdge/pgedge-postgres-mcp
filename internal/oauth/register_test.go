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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/config"
)

type testServer struct {
	srv   *Server
	mux   *http.ServeMux
	store *Store
	auth  *fakeAuthenticator
	now   time.Time
}

type fakeAuthenticator struct {
	users map[string]string
	fail  error
}

func (f *fakeAuthenticator) Authenticate(_ context.Context, u, p, _ string) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	if f.users[u] == p && p != "" {
		return u, nil
	}
	return "", ErrInvalidCredentials
}

func newTestServer(t *testing.T, mutate func(*Options)) *testServer {
	t.Helper()
	cfg := config.OAuthConfig{
		Issuer:              "https://mcp.example.com",
		AccessTokenLifetime: time.Hour, RefreshTokenLifetime: 24 * time.Hour,
		AuthorizationCodeLifetime: 10 * time.Minute, DeviceCodeLifetime: 15 * time.Minute,
		AllowedRedirectURIs: config.DefaultAllowedRedirectURIs,
		LoginPage:           config.LoginPageConfig{Title: "Sign in", Subtitle: "s", PrimaryColour: "#15AABF", SecondaryColour: "#0C8599"},
	}
	ts := &testServer{auth: &fakeAuthenticator{users: map[string]string{"alice": "correct horse"}}, now: time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)}
	opts := Options{Config: cfg, Authenticator: ts.auth, Now: func() time.Time { return ts.now }}
	if mutate != nil {
		mutate(&opts)
	}
	srv, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	ts.srv, ts.store, ts.mux = srv, srv.store, http.NewServeMux()
	srv.RegisterRoutes(ts.mux)
	return ts
}

func (ts *testServer) do(method, path, contentType string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	ts.mux.ServeHTTP(rec, req)
	return rec
}

func TestRegisterPublicClient(t *testing.T) {
	ts := newTestServer(t, nil)
	rec := ts.do("POST", RegisterPath, "application/json", `{"redirect_uris":["https://claude.ai/api/mcp/auth_callback"],"client_name":"Claude"}`)
	if rec.Code != 201 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var resp registrationResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ClientID == "" || resp.TokenEndpointAuthMethod != "none" {
		t.Fatalf("%+v", resp)
	}
	if _, ok := ts.store.GetClient(resp.ClientID); !ok {
		t.Fatal("client not stored")
	}
}

func TestRegisterRejectsDisallowedRedirect(t *testing.T) {
	ts := newTestServer(t, nil)
	rec := ts.do("POST", RegisterPath, "application/json", `{"redirect_uris":["https://evil.example.com/cb"]}`)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_redirect_uri") {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestRegisterRejectsConfidentialClient(t *testing.T) {
	ts := newTestServer(t, nil)
	rec := ts.do("POST", RegisterPath, "application/json", `{"redirect_uris":["https://claude.ai/api/mcp/auth_callback"],"token_endpoint_auth_method":"client_secret_basic"}`)
	if rec.Code != 400 {
		t.Fatal(rec.Code)
	}
}

func TestRegisterDisabled(t *testing.T) {
	f := false
	ts := newTestServer(t, func(o *Options) { o.Config.AllowDynamicRegistration = &f })
	if rec := ts.do("POST", RegisterPath, "application/json", `{}`); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
}

func TestRegisterMethodNotAllowed(t *testing.T) {
	ts := newTestServer(t, nil)
	if rec := ts.do("GET", RegisterPath, "", ""); rec.Code != 405 {
		t.Fatal(rec.Code)
	}
}
