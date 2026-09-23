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
	"net/url"
	"sync"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/auth"
)

// activeSubjects is a SubjectActive implementation whose answers a test
// can change part way through, standing in for a user store in which an
// account is disabled or deleted whilst its tokens are still live.
type activeSubjects struct {
	mu     sync.Mutex
	active map[string]bool
}

func newActiveSubjects(names ...string) *activeSubjects {
	a := &activeSubjects{active: map[string]bool{}}
	for _, n := range names {
		a.active[n] = true
	}
	return a
}

func (a *activeSubjects) check(username string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.active[username]
}

func (a *activeSubjects) withdraw(username string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.active, username)
}

// revokedHashes records what the OnTokensRevoked hook was handed.
type revokedHashes struct {
	mu     sync.Mutex
	hashes []string
}

func (r *revokedHashes) record(hashes []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hashes = append(r.hashes, hashes...)
}

func (r *revokedHashes) contains(hash string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, h := range r.hashes {
		if h == hash {
			return true
		}
	}
	return false
}

// issueTokenPair drives a full authorisation code grant and returns the
// resulting access and refresh tokens along with the client id.
func issueTokenPair(t *testing.T, ts *testServer) (cid string, tr tokenResponse) {
	t.Helper()
	cid, code := obtainCode(t, ts, claudeCB)
	rec, tr := exchange(ts, codeForm(cid, code, claudeCB))
	if rec.Code != 200 || tr.AccessToken == "" || tr.RefreshToken == "" {
		t.Fatalf("token exchange: %d %s", rec.Code, rec.Body)
	}
	return cid, tr
}

// TestAccessTokenRefusedOnceSubjectIsInactive covers the finding that
// disabling or deleting a user left its OAuth access tokens working
// until they expired, whilst a session token for the same account was
// refused on the very next request.
func TestAccessTokenRefusedOnceSubjectIsInactive(t *testing.T) {
	subjects := newActiveSubjects("alice")
	ts := newTestServer(t, func(o *Options) { o.SubjectActive = subjects.check })
	_, tr := issueTokenPair(t, ts)

	if _, _, ok := ts.srv.ValidateAccessToken(tr.AccessToken); !ok {
		t.Fatal("token should be valid whilst alice is active")
	}

	subjects.withdraw("alice")

	if _, _, ok := ts.srv.ValidateAccessToken(tr.AccessToken); ok {
		t.Fatal("token accepted for a subject that is no longer active")
	}
	// The refusal revokes the rest of the subject's tokens too, so the
	// token is gone from the store rather than merely being refused.
	if _, found := ts.store.GetToken(hashToken(tr.AccessToken)); found {
		t.Fatal("token still on file after the subject was withdrawn")
	}
}

// TestRefreshRefusedOnceSubjectIsInactive is the same finding on the
// other path: the refresh grant never consulted the user store, so a
// disabled user's client could keep minting fresh access tokens from
// the subject recorded on the refresh token.
func TestRefreshRefusedOnceSubjectIsInactive(t *testing.T) {
	subjects := newActiveSubjects("alice")
	ts := newTestServer(t, func(o *Options) { o.SubjectActive = subjects.check })
	cid, tr := issueTokenPair(t, ts)

	subjects.withdraw("alice")

	rec, refreshed := exchange(ts, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tr.RefreshToken},
		"client_id":     {cid},
	})
	if rec.Code != 400 {
		t.Fatalf("refresh for an inactive subject: %d %s", rec.Code, rec.Body)
	}
	if refreshed.AccessToken != "" {
		t.Fatal("refresh issued an access token for an inactive subject")
	}
}

// TestUnknownSubjectFailsClosed covers the case where the account has
// been deleted outright rather than disabled: SubjectActive knows
// nothing about it, and the token must be refused all the same.
func TestUnknownSubjectFailsClosed(t *testing.T) {
	ts := newTestServer(t, func(o *Options) {
		o.SubjectActive = func(string) bool { return false }
	})
	_, tr := issueTokenPair(t, ts)
	if _, _, ok := ts.srv.ValidateAccessToken(tr.AccessToken); ok {
		t.Fatal("token accepted for an unknown subject")
	}
}

// TestRevokeSubjectClearsEverythingInItsName covers Store.DeleteBySubject
// by way of the Server method main calls when a user is disabled,
// deleted or locked out.
func TestRevokeSubjectClearsEverythingInItsName(t *testing.T) {
	ts := newTestServer(t, nil)
	_, tr := issueTokenPair(t, ts)

	revoked := ts.srv.RevokeSubject("alice")

	if len(revoked) != 1 || revoked[0] != hashToken(tr.AccessToken) {
		t.Fatalf("revoked = %v; want the access token hash %s", revoked, hashToken(tr.AccessToken))
	}
	if _, _, ok := ts.srv.ValidateAccessToken(tr.AccessToken); ok {
		t.Fatal("access token survived the subject revocation")
	}
	if _, found := ts.store.GetToken(hashToken(tr.RefreshToken)); found {
		t.Fatal("refresh token survived the subject revocation")
	}
}

// TestRevokedAccessTokenHashesReachTheHook covers the connection pool
// leak: the per-token pools are keyed on the access token's hash, so
// every path that ends a token's life has to report that hash. The hash
// form is the one the rest of the server keys sessions on, which is why
// it is compared against auth.HashToken here.
func TestRevokedAccessTokenHashesReachTheHook(t *testing.T) {
	var hook revokedHashes
	ts := newTestServer(t, func(o *Options) { o.OnTokensRevoked = hook.record })
	cid, tr := issueTokenPair(t, ts)

	accessHash := hashToken(tr.AccessToken)
	if accessHash != auth.HashToken(tr.AccessToken) {
		t.Fatalf("oauth hashes an access token as %s, the client manager keys it on %s",
			accessHash, auth.HashToken(tr.AccessToken))
	}

	// A refresh rotation retires the previous access token, so its pool
	// has to go with it rather than being replaced by a second one.
	rec, rotated := exchange(ts, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tr.RefreshToken},
		"client_id":     {cid},
	})
	if rec.Code != 200 {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body)
	}
	if !hook.contains(accessHash) {
		t.Fatal("rotation did not report the retired access token hash")
	}

	// So does an explicit revocation.
	ts.do("POST", RevokePath, "application/x-www-form-urlencoded",
		url.Values{"token": {rotated.AccessToken}}.Encode())
	if !hook.contains(hashToken(rotated.AccessToken)) {
		t.Fatal("revocation did not report the revoked access token hash")
	}
}

// TestSweepReportsExpiredAccessTokenHashes covers the same leak on the
// background sweeper's path, which is how most tokens end their life.
func TestSweepReportsExpiredAccessTokenHashes(t *testing.T) {
	var hook revokedHashes
	store := NewStore(DefaultLimits, 24*time.Hour)
	store.SetRevocationHook(hook.record)

	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	if err := store.PutToken(&Token{Hash: "access", Subject: "alice", ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	if swept := store.Sweep(now); len(swept) != 0 {
		t.Fatalf("swept a live token: %v", swept)
	}
	swept := store.Sweep(now.Add(2 * time.Hour))
	if len(swept) != 1 || swept[0] != "access" {
		t.Fatalf("Sweep returned %v; want [access]", swept)
	}
	if !hook.contains("access") {
		t.Fatal("Sweep did not report the expired access token hash")
	}
}

// TestDeleteFamilyReportsAccessTokenHashes covers the family revocation
// path, which a replayed code or refresh token triggers.
func TestDeleteFamilyReportsAccessTokenHashes(t *testing.T) {
	var hook revokedHashes
	store := NewStore(DefaultLimits, 24*time.Hour)
	store.SetRevocationHook(hook.record)

	future := time.Now().Add(time.Hour)
	if err := store.PutToken(&Token{Hash: "refresh", IsRefresh: true, Family: "f", ExpiresAt: future}); err != nil {
		t.Fatal(err)
	}
	if err := store.PutToken(&Token{Hash: "access", RefreshHash: "refresh", Family: "f", ExpiresAt: future}); err != nil {
		t.Fatal(err)
	}

	revoked := store.DeleteFamily("f")
	if len(revoked) != 1 || revoked[0] != "access" {
		t.Fatalf("DeleteFamily returned %v; want [access]", revoked)
	}
	if !hook.contains("access") {
		t.Fatal("DeleteFamily did not report the access token hash")
	}
}

// TestRegistrationDoesNotSpendTheLoginBudget covers the finding that ten
// unauthenticated registrations locked every user out of password
// login, because registration metered itself on the limiter that also
// gates the login form, authenticate_user and /api/user/info.
func TestRegistrationDoesNotSpendTheLoginBudget(t *testing.T) {
	login := auth.NewRateLimiter(1, 3)
	t.Cleanup(login.Stop)
	anonymous := auth.NewRateLimiter(1, 100)
	t.Cleanup(anonymous.Stop)

	ts := newTestServer(t, func(o *Options) {
		o.RateLimiter = login
		o.AnonymousRateLimiter = anonymous
	})

	body := `{"redirect_uris":["https://claude.ai/api/mcp/auth_callback"]}`
	for i := 0; i < 10; i++ {
		if rec := ts.do("POST", RegisterPath, "application/json", body); rec.Code != 201 {
			t.Fatalf("registration %d: %d %s", i+1, rec.Code, rec.Body)
		}
	}

	if !login.IsAllowed("192.0.2.1") {
		t.Fatal("registrations exhausted the login budget")
	}

	// And the login form itself still works for the same address.
	cid := registerClient(t, ts, claudeCB)
	rec := loginForm(t, ts, cid, claudeCB, "alice", "correct horse")
	if rec.Code != 302 {
		t.Fatalf("login after a registration flood: %d %s", rec.Code, rec.Body)
	}
}

// TestRevokeFailuresDoNotSpendTheLoginBudget covers the finding that
// /oauth/revoke still recorded its failures on the login limiter after
// registration and the device request had moved off it, so ten empty
// POSTs to it locked password login out for the sending address.
func TestRevokeFailuresDoNotSpendTheLoginBudget(t *testing.T) {
	login := auth.NewRateLimiter(1, 3)
	t.Cleanup(login.Stop)
	// Four: the registration below, then the three revocation failures.
	anonymous := auth.NewRateLimiter(1, 4)
	t.Cleanup(anonymous.Stop)

	ts := newTestServer(t, func(o *Options) {
		o.RateLimiter = login
		o.AnonymousRateLimiter = anonymous
	})
	cid := registerClient(t, ts, claudeCB)

	// One of each failure the endpoint records: wrong method, malformed
	// body, missing token.
	ts.do("GET", RevokePath, "", "")
	ts.do("POST", RevokePath, "application/x-www-form-urlencoded", "%zz")
	ts.do("POST", RevokePath, "application/x-www-form-urlencoded", "")

	if !login.IsAllowed("192.0.2.1") {
		t.Fatal("revocation failures exhausted the login budget")
	}
	rec := ts.do("POST", RevokePath, "application/x-www-form-urlencoded", "token=nothing")
	if rec.Code != 429 {
		t.Fatalf("revoke after exhausting the anonymous budget: %d %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q", got)
	}

	if rec := loginForm(t, ts, cid, claudeCB, "alice", "correct horse"); rec.Code != 302 {
		t.Fatalf("login after revocation failures: %d %s", rec.Code, rec.Body)
	}
}

// TestDeviceRequestsAreMeteredWhenTheySucceed covers the device
// endpoint's half of the same finding: it recorded only failures, so a
// burst of successful requests could fill the bounded device code table
// and deny the grant to everyone without the limiter noticing.
func TestDeviceRequestsAreMeteredWhenTheySucceed(t *testing.T) {
	// Three requests' worth of budget: the registration below spends
	// one, leaving room for exactly two device requests.
	anonymous := auth.NewRateLimiter(1, 3)
	t.Cleanup(anonymous.Stop)
	ts := newTestServer(t, func(o *Options) { o.AnonymousRateLimiter = anonymous })

	cid := registerClient(t, ts, claudeCB)
	form := url.Values{"client_id": {cid}}.Encode()
	for i := 0; i < 2; i++ {
		if rec := ts.do("POST", DevicePath, "application/x-www-form-urlencoded", form); rec.Code != 200 {
			t.Fatalf("device request %d: %d %s", i+1, rec.Code, rec.Body)
		}
	}
	if rec := ts.do("POST", DevicePath, "application/x-www-form-urlencoded", form); rec.Code != 429 {
		t.Fatalf("third device request: %d %s", rec.Code, rec.Body)
	}
}
