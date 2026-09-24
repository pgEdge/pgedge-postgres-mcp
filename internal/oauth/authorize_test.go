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
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

const goodChallenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
const goodVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

func registerClient(t *testing.T, ts *testServer, redirect string) string {
	t.Helper()
	rec := ts.do("POST", RegisterPath, "application/json", fmt.Sprintf(`{"redirect_uris":[%q],"client_name":"Test"}`, redirect))
	if rec.Code != 201 {
		t.Fatalf("register: %d %s", rec.Code, rec.Body)
	}
	var resp registrationResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return resp.ClientID
}

func authorizeQuery(clientID, redirect string) url.Values {
	return url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect},
		"state": {"xyz"}, "scope": {"mcp"}, "code_challenge": {goodChallenge}, "code_challenge_method": {"S256"}}
}

func extractCSRF(t *testing.T, body string) string {
	t.Helper()
	m := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no csrf token in %s", body)
	}
	return m[1]
}

func TestAuthorizeGetRendersLogin(t *testing.T) {
	ts := newTestServer(t, nil)
	cid := registerClient(t, ts, "https://claude.ai/api/mcp/auth_callback")
	rec := ts.do("GET", AuthorizePath+"?"+authorizeQuery(cid, "https://claude.ai/api/mcp/auth_callback").Encode(), "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `name="username"`) {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("login page must not be cached")
	}
}

func TestAuthorizeUnknownClientDoesNotRedirect(t *testing.T) {
	ts := newTestServer(t, nil)
	rec := ts.do("GET", AuthorizePath+"?"+authorizeQuery("nope", "https://claude.ai/api/mcp/auth_callback").Encode(), "", "")
	if rec.Code != 400 || rec.Header().Get("Location") != "" {
		t.Fatal(rec.Code, rec.Header())
	}
}

func TestAuthorizeBadRedirectDoesNotRedirect(t *testing.T) {
	ts := newTestServer(t, nil)
	cid := registerClient(t, ts, "https://claude.ai/api/mcp/auth_callback")
	rec := ts.do("GET", AuthorizePath+"?"+authorizeQuery(cid, "https://evil.example.com/cb").Encode(), "", "")
	if rec.Code != 400 || rec.Header().Get("Location") != "" {
		t.Fatal(rec.Code)
	}
}

func TestAuthorizeMissingPKCERedirectsWithError(t *testing.T) {
	ts := newTestServer(t, nil)
	cid := registerClient(t, ts, "https://claude.ai/api/mcp/auth_callback")
	q := authorizeQuery(cid, "https://claude.ai/api/mcp/auth_callback")
	q.Del("code_challenge")
	rec := ts.do("GET", AuthorizePath+"?"+q.Encode(), "", "")
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if rec.Code != 302 || loc.Query().Get("error") != "invalid_request" || loc.Query().Get("state") != "xyz" {
		t.Fatal(rec.Code, loc)
	}
}

func TestAuthorizePlainPKCERejected(t *testing.T) {
	ts := newTestServer(t, nil)
	cid := registerClient(t, ts, "https://claude.ai/api/mcp/auth_callback")
	q := authorizeQuery(cid, "https://claude.ai/api/mcp/auth_callback")
	q.Set("code_challenge_method", "plain")
	rec := ts.do("GET", AuthorizePath+"?"+q.Encode(), "", "")
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Query().Get("error") != "invalid_request" {
		t.Fatal(loc)
	}
}

func loginForm(t *testing.T, ts *testServer, cid, redirect, user, pass string) *httptest.ResponseRecorder {
	t.Helper()
	get := ts.do("GET", AuthorizePath+"?"+authorizeQuery(cid, redirect).Encode(), "", "")
	form := authorizeQuery(cid, redirect)
	form.Set("csrf_token", extractCSRF(t, get.Body.String()))
	form.Set("username", user)
	form.Set("password", pass)
	return ts.do("POST", AuthorizePath, "application/x-www-form-urlencoded", form.Encode())
}

func TestAuthorizePostSuccessIssuesCode(t *testing.T) {
	ts := newTestServer(t, nil)
	cid := registerClient(t, ts, "https://claude.ai/api/mcp/auth_callback")
	rec := loginForm(t, ts, cid, "https://claude.ai/api/mcp/auth_callback", "alice", "correct horse")
	if rec.Code != 302 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	code := loc.Query().Get("code")
	if loc.Host != "claude.ai" || code == "" || loc.Query().Get("state") != "xyz" {
		t.Fatal(loc)
	}
	if _, ok := ts.store.TakeCode(hashToken(code)); !ok {
		t.Fatal("code not stored")
	}
}

func TestAuthorizePostWrongPassword(t *testing.T) {
	ts := newTestServer(t, nil)
	cid := registerClient(t, ts, "https://claude.ai/api/mcp/auth_callback")
	rec := loginForm(t, ts, cid, "https://claude.ai/api/mcp/auth_callback", "alice", "wrong")
	if rec.Code != 401 || !strings.Contains(rec.Body.String(), "Invalid username or password") {
		t.Fatal(rec.Code)
	}
	if strings.Contains(rec.Body.String(), "wrong") {
		t.Fatal("password echoed")
	}
}

func TestAuthorizePostBadCSRF(t *testing.T) {
	ts := newTestServer(t, nil)
	cid := registerClient(t, ts, "https://claude.ai/api/mcp/auth_callback")
	form := authorizeQuery(cid, "https://claude.ai/api/mcp/auth_callback")
	form.Set("csrf_token", "bogus")
	form.Set("username", "alice")
	form.Set("password", "correct horse")
	rec := ts.do("POST", AuthorizePath, "application/x-www-form-urlencoded", form.Encode())
	if rec.Code != 400 {
		t.Fatal(rec.Code)
	}
}

func TestAuthorizePostRateLimited(t *testing.T) {
	ts := newTestServer(t, nil)
	ts.auth.fail = ErrRateLimited
	cid := registerClient(t, ts, "https://claude.ai/api/mcp/auth_callback")
	rec := loginForm(t, ts, cid, "https://claude.ai/api/mcp/auth_callback", "alice", "correct horse")
	if rec.Code != 429 {
		t.Fatal(rec.Code)
	}
}

func TestAuthorizeLoopbackAnyPort(t *testing.T) {
	ts := newTestServer(t, nil)
	cid := registerClient(t, ts, "http://127.0.0.1/callback")
	rec := loginForm(t, ts, cid, "http://127.0.0.1:49152/callback", "alice", "correct horse")
	if rec.Code != 302 {
		t.Fatal(rec.Code, rec.Body.String())
	}
}
