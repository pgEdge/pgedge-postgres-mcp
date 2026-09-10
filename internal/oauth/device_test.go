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
	"regexp"
	"strings"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/auth"
)

func startDevice(t *testing.T, ts *testServer) (cid string, dr deviceResponse) {
	t.Helper()
	cid = registerClient(t, ts, claudeCB)
	rec := ts.do("POST", DevicePath, "application/x-www-form-urlencoded", url.Values{"client_id": {cid}, "scope": {"mcp"}}.Encode())
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &dr)
	return cid, dr
}

func pollDevice(ts *testServer, cid, deviceCode string) (*httptest.ResponseRecorder, tokenResponse) {
	return exchange(ts, url.Values{"grant_type": {DeviceGrantType}, "device_code": {deviceCode}, "client_id": {cid}})
}

func approveDevice(t *testing.T, ts *testServer, userCode, user, pass string) *httptest.ResponseRecorder {
	t.Helper()
	get := ts.do("GET", DeviceVerifyPath+"?user_code="+url.QueryEscape(userCode), "", "")
	form := url.Values{"csrf_token": {extractCSRF(t, get.Body.String())}, "user_code": {userCode}, "username": {user}, "password": {pass}}
	return ts.do("POST", DeviceVerifyPath, "application/x-www-form-urlencoded", form.Encode())
}

func TestDeviceResponseShape(t *testing.T) {
	ts := newTestServer(t, nil)
	_, dr := startDevice(t, ts)
	if !regexp.MustCompile(`^[BCDFGHJKLMNPQRSTVWXZ2-9]{4}-[BCDFGHJKLMNPQRSTVWXZ2-9]{4}$`).MatchString(dr.UserCode) {
		t.Fatal(dr.UserCode)
	}
	if dr.VerificationURI != "https://mcp.example.com/oauth/device/verify" || !strings.Contains(dr.VerificationURIComplete, "user_code="+dr.UserCode) {
		t.Fatal(dr)
	}
	if dr.ExpiresIn != 900 || dr.Interval != 5 {
		t.Fatal(dr)
	}
}

func TestDeviceFlowEndToEnd(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, dr := startDevice(t, ts)
	rec, _ := pollDevice(ts, cid, dr.DeviceCode)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "authorization_pending") {
		t.Fatal(rec.Code, rec.Body.String())
	}
	ts.now = ts.now.Add(6 * time.Second)
	if rec := approveDevice(t, ts, strings.ToLower(strings.ReplaceAll(dr.UserCode, "-", " ")), "alice", "correct horse"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "return to your application") {
		t.Fatal(rec.Code, rec.Body.String())
	}
	rec, tr := pollDevice(ts, cid, dr.DeviceCode)
	if rec.Code != 200 || tr.AccessToken == "" {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if sub, _, ok := ts.srv.ValidateAccessToken(tr.AccessToken); !ok || sub != "alice" {
		t.Fatal(sub, ok)
	}
	ts.now = ts.now.Add(6 * time.Second)
	if rec, _ := pollDevice(ts, cid, dr.DeviceCode); rec.Code != 400 {
		t.Fatal("device code should be single issuance")
	}
}

func TestDeviceSlowDown(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, dr := startDevice(t, ts)
	pollDevice(ts, cid, dr.DeviceCode)
	ts.now = ts.now.Add(time.Second)
	rec, _ := pollDevice(ts, cid, dr.DeviceCode)
	if !strings.Contains(rec.Body.String(), "slow_down") {
		t.Fatal(rec.Body.String())
	}
}

// TestDeviceSlowDownDoesNotDeferTheNextPoll is the regression test for a
// rejected poll advancing LastPolled: a client polling at a fixed period
// shorter than the interval used to be told to slow down for ever,
// because each rejection pushed the deadline out again.
func TestDeviceSlowDownDoesNotDeferTheNextPoll(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, dr := startDevice(t, ts)

	// The first poll is accepted and sets the deadline.
	if rec, _ := pollDevice(ts, cid, dr.DeviceCode); !strings.Contains(rec.Body.String(), "authorization_pending") {
		t.Fatalf("first poll: %s", rec.Body.String())
	}

	// Two more inside the interval are rejected, and must not move it.
	for _, offset := range []time.Duration{2 * time.Second, 4 * time.Second} {
		ts.now = ts.now.Add(2 * time.Second)
		rec, _ := pollDevice(ts, cid, dr.DeviceCode)
		if !strings.Contains(rec.Body.String(), "slow_down") {
			t.Fatalf("poll at +%s: %s", offset, rec.Body.String())
		}
	}

	// Just past one interval from the first accepted poll, so accepted.
	ts.now = ts.now.Add(time.Second + time.Millisecond)
	rec, _ := pollDevice(ts, cid, dr.DeviceCode)
	if !strings.Contains(rec.Body.String(), "authorization_pending") {
		t.Fatalf("poll one interval after the first accepted one: %s", rec.Body.String())
	}
}

func TestDeviceExpired(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, dr := startDevice(t, ts)
	ts.now = ts.now.Add(16 * time.Minute)
	rec, _ := pollDevice(ts, cid, dr.DeviceCode)
	if !strings.Contains(rec.Body.String(), "expired_token") {
		t.Fatal(rec.Body.String())
	}
}

func TestDeviceWrongClientOrUnknownCode(t *testing.T) {
	ts := newTestServer(t, nil)
	_, dr := startDevice(t, ts)
	if rec, _ := pollDevice(ts, "other", dr.DeviceCode); !strings.Contains(rec.Body.String(), "invalid_grant") {
		t.Fatal(rec.Body.String())
	}
	if rec := approveDevice(t, ts, "ZZZZ-ZZZZ", "alice", "correct horse"); rec.Code != 400 {
		t.Fatal(rec.Code)
	}
}

func TestDeviceUnknownClient(t *testing.T) {
	ts := newTestServer(t, nil)
	rec := ts.do("POST", DevicePath, "application/x-www-form-urlencoded", "client_id=nope")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_client") {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestDeviceMissingClientID(t *testing.T) {
	ts := newTestServer(t, nil)
	rec := ts.do("POST", DevicePath, "application/x-www-form-urlencoded", "")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestDeviceBadScope(t *testing.T) {
	ts := newTestServer(t, nil)
	cid := registerClient(t, ts, claudeCB)
	rec := ts.do("POST", DevicePath, "application/x-www-form-urlencoded", url.Values{"client_id": {cid}, "scope": {"bogus"}}.Encode())
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_scope") {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestDeviceMethodNotAllowed(t *testing.T) {
	ts := newTestServer(t, nil)
	if rec := ts.do("GET", DevicePath, "", ""); rec.Code != 405 {
		t.Fatal(rec.Code)
	}
}

func TestDeviceRateLimited(t *testing.T) {
	rl := auth.NewRateLimiter(1, 1)
	t.Cleanup(rl.Stop)
	ts := newTestServer(t, func(o *Options) { o.RateLimiter = rl })
	ts.do("GET", DevicePath, "", "")
	ts.do("GET", DevicePath, "", "")
	rec := ts.do("POST", DevicePath, "application/x-www-form-urlencoded", "client_id=nope")
	if rec.Code != 429 || rec.Header().Get("Retry-After") != "60" {
		t.Fatal(rec.Code, rec.Header())
	}
}

func TestDeviceVerifyMethodNotAllowed(t *testing.T) {
	ts := newTestServer(t, nil)
	if rec := ts.do("PUT", DeviceVerifyPath, "", ""); rec.Code != 405 {
		t.Fatal(rec.Code)
	}
}

func TestDeviceVerifyBadCSRF(t *testing.T) {
	ts := newTestServer(t, nil)
	_, dr := startDevice(t, ts)
	form := url.Values{"csrf_token": {"bogus"}, "user_code": {dr.UserCode}, "username": {"alice"}, "password": {"correct horse"}}
	rec := ts.do("POST", DeviceVerifyPath, "application/x-www-form-urlencoded", form.Encode())
	if rec.Code != 400 {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestDeviceVerifyWrongPassword(t *testing.T) {
	ts := newTestServer(t, nil)
	_, dr := startDevice(t, ts)
	rec := approveDevice(t, ts, dr.UserCode, "alice", "wrong")
	if rec.Code != 401 || !strings.Contains(rec.Body.String(), "Invalid username or password") {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestDeviceVerifyRateLimited(t *testing.T) {
	ts := newTestServer(t, nil)
	ts.auth.fail = ErrRateLimited
	_, dr := startDevice(t, ts)
	rec := approveDevice(t, ts, dr.UserCode, "alice", "correct horse")
	if rec.Code != 429 {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestDeviceCodeGrantMissingFields(t *testing.T) {
	ts := newTestServer(t, nil)
	rec, _ := exchange(ts, url.Values{"grant_type": {DeviceGrantType}})
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestDeviceMalformedBody(t *testing.T) {
	ts := newTestServer(t, nil)
	rec := ts.do("POST", DevicePath, "application/x-www-form-urlencoded", "%zz")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestDeviceVerifyMalformedBody(t *testing.T) {
	ts := newTestServer(t, nil)
	rec := ts.do("POST", DeviceVerifyPath, "application/x-www-form-urlencoded", "%zz")
	if rec.Code != 400 {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestNormaliseUserCodeShortInput(t *testing.T) {
	if got := normaliseUserCode("ab"); got != "AB" {
		t.Fatal(got)
	}
}

func TestDeviceVerifyApprovalSingleWriter(t *testing.T) {
	ts := newTestServer(t, nil)
	ts.auth.users["bob"] = "hunter2 hunter2"
	cid, dr := startDevice(t, ts)

	if rec := approveDevice(t, ts, dr.UserCode, "alice", "correct horse"); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if rec := approveDevice(t, ts, dr.UserCode, "bob", "hunter2 hunter2"); rec.Code != 400 {
		t.Fatal(rec.Code, rec.Body.String())
	}

	ts.now = ts.now.Add(6 * time.Second)
	rec, tr := pollDevice(ts, cid, dr.DeviceCode)
	if rec.Code != 200 || tr.AccessToken == "" {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if sub, _, ok := ts.srv.ValidateAccessToken(tr.AccessToken); !ok || sub != "alice" {
		t.Fatal(sub, ok)
	}
}

func TestDeviceVerifyRateLimitedByIP(t *testing.T) {
	rl := auth.NewRateLimiter(1, 2)
	t.Cleanup(rl.Stop)
	ts := newTestServer(t, func(o *Options) { o.RateLimiter = rl })

	// Two POSTs with a bogus, never-issued user code, each recording a
	// failed attempt (guessing a code counts, unlike a login form retry
	// with a valid CSRF token and just the wrong password).
	if rec := approveDevice(t, ts, "ZZZZ-ZZZZ", "alice", "correct horse"); rec.Code != 400 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if rec := approveDevice(t, ts, "ZZZZ-ZZZZ", "alice", "correct horse"); rec.Code != 400 {
		t.Fatal(rec.Code, rec.Body.String())
	}

	rec := ts.do("GET", DeviceVerifyPath, "", "")
	if rec.Code != 429 || rec.Header().Get("Retry-After") != "60" {
		t.Fatal(rec.Code, rec.Header())
	}
}

func TestDeviceCodeGrantDenied(t *testing.T) {
	ts := newTestServer(t, nil)
	cid, dr := startDevice(t, ts)
	d, ok := ts.store.GetDeviceByHash(hashToken(dr.DeviceCode))
	if !ok {
		t.Fatal("device not stored")
	}
	d.Denied = true
	ts.store.UpdateDevice(d)
	rec, _ := pollDevice(ts, cid, dr.DeviceCode)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "access_denied") {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if _, ok := ts.store.GetDeviceByHash(hashToken(dr.DeviceCode)); ok {
		t.Fatal("denied device code should be deleted")
	}
}
