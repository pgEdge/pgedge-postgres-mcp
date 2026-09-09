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
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/auth"
)

func obtainCode(t *testing.T, ts *testServer, redirect string) (cid, code string) {
	t.Helper()
	cid = registerClient(t, ts, redirect)
	rec := loginForm(t, ts, cid, redirect, "alice", "correct horse")
	loc, _ := url.Parse(rec.Header().Get("Location"))
	return cid, loc.Query().Get("code")
}

func exchange(ts *testServer, form url.Values) (*httptest.ResponseRecorder, tokenResponse) {
	rec := ts.do("POST", TokenPath, "application/x-www-form-urlencoded", form.Encode())
	var tr tokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &tr)
	return rec, tr
}

func codeForm(cid, code, redirect string) url.Values {
	return url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {cid}, "redirect_uri": {redirect}, "code_verifier": {goodVerifier}}
}

const claudeCB = "https://claude.ai/api/mcp/auth_callback"

func TestTokenExchangeSuccess(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	rec, tr := exchange(ts, codeForm(cid, code, claudeCB))
	if rec.Code != 200 || tr.AccessToken == "" || tr.RefreshToken == "" || tr.TokenType != "Bearer" || tr.ExpiresIn != 3600 || tr.Scope != "mcp" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store")
	}
	sub, gotCID, ok := ts.srv.ValidateAccessToken(tr.AccessToken)
	if !ok || sub != "alice" || gotCID != cid {
		t.Fatal(sub, gotCID, ok)
	}
}

func TestTokenExchangeCodeSingleUse(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	_, first := exchange(ts, codeForm(cid, code, claudeCB))
	rec, _ := exchange(ts, codeForm(cid, code, claudeCB))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_grant") {
		t.Fatal(rec.Code)
	}
	if _, _, ok := ts.srv.ValidateAccessToken(first.AccessToken); ok {
		t.Fatal("replay should revoke tokens issued from the code")
	}
}

func TestTokenExchangeWrongVerifier(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	f := codeForm(cid, code, claudeCB)
	f.Set("code_verifier", strings.Repeat("b", 43))
	if rec, _ := exchange(ts, f); rec.Code != 400 {
		t.Fatal(rec.Code)
	}
}

func TestTokenExchangeWrongRedirectOrClient(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	f := codeForm(cid, code, "https://claude.ai/other")
	if rec, _ := exchange(ts, f); rec.Code != 400 {
		t.Fatal(rec.Code)
	}
	_, code2 := obtainCode(t, ts, claudeCB)
	f = codeForm("someone-else", code2, claudeCB)
	if rec, _ := exchange(ts, f); rec.Code != 400 {
		t.Fatal(rec.Code)
	}
}

func TestTokenExpiredCode(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	ts.now = ts.now.Add(11 * time.Minute)
	if rec, _ := exchange(ts, codeForm(cid, code, claudeCB)); rec.Code != 400 {
		t.Fatal(rec.Code)
	}
}

func TestAccessTokenExpires(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	_, tr := exchange(ts, codeForm(cid, code, claudeCB))
	ts.now = ts.now.Add(61 * time.Minute)
	if _, _, ok := ts.srv.ValidateAccessToken(tr.AccessToken); ok {
		t.Fatal("expired token accepted")
	}
}

func TestRefreshRotates(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	_, tr := exchange(ts, codeForm(cid, code, claudeCB))
	rec, tr2 := exchange(ts, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tr.RefreshToken}, "client_id": {cid}})
	if rec.Code != 200 || tr2.AccessToken == "" || tr2.RefreshToken == tr.RefreshToken {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if _, _, ok := ts.srv.ValidateAccessToken(tr.AccessToken); ok {
		t.Fatal("old access token should be gone")
	}
	rec, _ = exchange(ts, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tr.RefreshToken}, "client_id": {cid}})
	if rec.Code != 400 {
		t.Fatal("old refresh token should be rejected")
	}
}

func TestRefreshWrongClient(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	_, tr := exchange(ts, codeForm(cid, code, claudeCB))
	rec, _ := exchange(ts, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tr.RefreshToken}, "client_id": {"other"}})
	if rec.Code != 400 {
		t.Fatal(rec.Code)
	}
}

func TestUnsupportedGrant(t *testing.T) {
	ts := newTestServer(t, nil)
	rec, _ := exchange(ts, url.Values{"grant_type": {"password"}})
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "unsupported_grant_type") {
		t.Fatal(rec.Code)
	}
}

func TestAccessTokenIsNotARefreshToken(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	_, tr := exchange(ts, codeForm(cid, code, claudeCB))
	if _, _, ok := ts.srv.ValidateAccessToken(tr.RefreshToken); ok {
		t.Fatal("refresh token accepted as access token")
	}
}

func TestConcurrentRefreshRotationOnlyOneSucceeds(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	_, tr := exchange(ts, codeForm(cid, code, claudeCB))

	const n = 50
	var ready sync.WaitGroup
	ready.Add(n)
	start := make(chan struct{})
	results := make(chan int, n)

	for i := 0; i < n; i++ {
		go func() {
			ready.Done()
			<-start
			rec, _ := exchange(ts, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tr.RefreshToken}, "client_id": {cid}})
			results <- rec.Code
		}()
	}

	// Wait until every goroutine has reached the gate, then release them
	// all at once, so as many as possible are inside the take-and-delete
	// window concurrently rather than trickling in one at a time.
	ready.Wait()
	close(start)

	codes := make(map[int]int, 2)
	for i := 0; i < n; i++ {
		codes[<-results]++
	}

	if codes[200] != 1 {
		t.Fatalf("expected exactly one 200, got %v", codes)
	}
	if codes[400] != n-1 {
		t.Fatalf("expected the rest to be 400, got %v", codes)
	}
}

func TestTokenEndpointRateLimited(t *testing.T) {
	rl := auth.NewRateLimiter(1, 1)
	t.Cleanup(rl.Stop)
	ts := newTestServer(t, func(o *Options) { o.RateLimiter = rl })

	f := url.Values{"grant_type": {"authorization_code"}, "code": {"bogus"}, "client_id": {"nope"}, "redirect_uri": {claudeCB}, "code_verifier": {goodVerifier}}
	exchange(ts, f)
	exchange(ts, f)

	rec, _ := exchange(ts, f)
	if rec.Code != 429 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q", got)
	}
}
