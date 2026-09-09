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
	"strings"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/auth"
)

// TestRegistrationIsMeteredEvenWhenSuccessful covers the registration
// flooding finding: every registration, successful or not, counts
// against the per-IP limiter.
func TestRegistrationIsMeteredEvenWhenSuccessful(t *testing.T) {
	rl := auth.NewRateLimiter(1, 2)
	t.Cleanup(rl.Stop)
	ts := newTestServer(t, func(o *Options) { o.RateLimiter = rl })

	body := `{"redirect_uris":["https://claude.ai/api/mcp/auth_callback"]}`
	for i := 0; i < 2; i++ {
		if rec := ts.do("POST", RegisterPath, "application/json", body); rec.Code != 201 {
			t.Fatalf("registration %d: %d %s", i+1, rec.Code, rec.Body)
		}
	}
	if rec := ts.do("POST", RegisterPath, "application/json", body); rec.Code != 429 {
		t.Fatalf("third registration: %d %s", rec.Code, rec.Body)
	}
}

// TestAuthorizeTouchesClient and TestTokenTouchesClient cover the
// LastUsed bookkeeping the client sweeper depends on.
func TestAuthorizeTouchesClient(t *testing.T) {
	ts := newTestServer(t, nil)
	cid := registerClient(t, ts, claudeCB)
	before, _ := ts.store.GetClient(cid)

	ts.now = ts.now.Add(time.Hour)
	ts.do("GET", AuthorizePath+"?"+authorizeQuery(cid, claudeCB).Encode(), "", "")

	after, _ := ts.store.GetClient(cid)
	if !after.LastUsed.After(before.LastUsed) {
		t.Fatalf("LastUsed not refreshed: %v then %v", before.LastUsed, after.LastUsed)
	}
}

func TestTokenTouchesClient(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	before, _ := ts.store.GetClient(cid)

	ts.now = ts.now.Add(time.Minute) // still inside the code's lifetime
	if rec, _ := exchange(ts, codeForm(cid, code, claudeCB)); rec.Code != 200 {
		t.Fatalf("token: %d %s", rec.Code, rec.Body)
	}

	after, _ := ts.store.GetClient(cid)
	if !after.LastUsed.After(before.LastUsed) {
		t.Fatalf("LastUsed not refreshed: %v then %v", before.LastUsed, after.LastUsed)
	}
}

// TestDevicePollSurvivesExhaustedLimiter covers the finding that device
// polling must not be blocked by unrelated failures on the shared
// per-IP limiter, since polling never records a failure itself.
func TestDevicePollSurvivesExhaustedLimiter(t *testing.T) {
	rl := auth.NewRateLimiter(1, 4)
	t.Cleanup(rl.Stop)
	ts := newTestServer(t, func(o *Options) { o.RateLimiter = rl })
	cid, dr := startDevice(t, ts)

	// Exhaust the limiter for this IP with unrelated failures.
	for i := 0; i < 4; i++ {
		rl.RecordFailedAttempt("192.0.2.1")
	}
	if rl.IsAllowed("192.0.2.1") {
		t.Fatal("limiter should be exhausted")
	}

	rec, _ := pollDevice(ts, cid, dr.DeviceCode)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "authorization_pending") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

// TestDeviceVerifyNamesClientAndScope covers RFC 8628 section 5.4: the
// consent page must say which client is asking, before the resource
// owner types anything.
func TestDeviceVerifyNamesClientAndScope(t *testing.T) {
	ts := newTestServer(t, nil)
	_, dr := startDevice(t, ts)

	rec := ts.do("GET", DeviceVerifyPath+"?user_code="+url.QueryEscape(dr.UserCode), "", "")
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, body)
	}
	for _, want := range []string{"Test", "is asking to sign in with scope", "mcp", ">Approve<", `name="action" value="deny"`} {
		if !strings.Contains(body, want) {
			t.Errorf("consent page missing %q\n%s", want, body)
		}
	}
}

// TestDeviceVerifyDeny covers the Deny button: it needs no credentials,
// marks the device code denied, and the token endpoint then reports
// access_denied.
func TestDeviceVerifyDeny(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, dr := startDevice(t, ts)

	get := ts.do("GET", DeviceVerifyPath+"?user_code="+url.QueryEscape(dr.UserCode), "", "")
	form := url.Values{
		"csrf_token": {extractCSRF(t, get.Body.String())},
		"user_code":  {dr.UserCode},
		"action":     {"deny"},
	}
	rec := ts.do("POST", DeviceVerifyPath, "application/x-www-form-urlencoded", form.Encode())
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Request denied.") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}

	ts.now = ts.now.Add(10 * time.Second)
	poll, _ := pollDevice(ts, cid, dr.DeviceCode)
	if poll.Code != 400 || !strings.Contains(poll.Body.String(), "access_denied") {
		t.Fatalf("poll after deny: %d %s", poll.Code, poll.Body)
	}
}

// TestAuthorizeUnknownClientRendersNoForm covers the finding that an
// unknown client or bad redirect URI must not present a credential
// form, since the credentials could not be used for anything good.
func TestAuthorizeUnknownClientRendersNoForm(t *testing.T) {
	ts := newTestServer(t, nil)
	q := authorizeQuery("no-such-client", claudeCB)

	rec := ts.do("GET", AuthorizePath+"?"+q.Encode(), "", "")
	body := rec.Body.String()
	if rec.Code != 400 {
		t.Fatalf("status %d", rec.Code)
	}
	if strings.Contains(body, `name="username"`) || strings.Contains(body, `name="password"`) {
		t.Errorf("credential form rendered for unknown client:\n%s", body)
	}
	if !strings.Contains(body, "Invalid client or redirect URI") {
		t.Errorf("error message missing:\n%s", body)
	}
}

// TestTokenSuccessSetsPragmaNoCache covers the token endpoint's cache
// headers on the success path, not only on errors.
func TestTokenSuccessSetsPragmaNoCache(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	rec, _ := exchange(ts, codeForm(cid, code, claudeCB))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("headers = %v", rec.Header())
	}
}

// TestRefreshReplayRevokesFamily covers refresh token family
// revocation: presenting a rotated refresh token kills every token
// descended from the same authorisation.
func TestRefreshReplayRevokesFamily(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, code := obtainCode(t, ts, claudeCB)
	_, first := exchange(ts, codeForm(cid, code, claudeCB))
	if first.RefreshToken == "" {
		t.Fatal("no refresh token")
	}

	refreshForm := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {first.RefreshToken}, "client_id": {cid}}
	rec, second := exchange(ts, refreshForm)
	if rec.Code != 200 || second.AccessToken == "" {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body)
	}

	// Replay the rotated refresh token.
	replay, _ := exchange(ts, refreshForm)
	if replay.Code != 400 || !strings.Contains(replay.Body.String(), "invalid_grant") {
		t.Fatalf("replay: %d %s", replay.Code, replay.Body)
	}
	if _, _, ok := ts.srv.ValidateAccessToken(second.AccessToken); ok {
		t.Error("access token from the rotated family survived the replay")
	}
	if rec, _ := exchange(ts, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {second.RefreshToken}, "client_id": {cid}}); rec.Code == 200 {
		t.Error("refresh token from the rotated family survived the replay")
	}
}
