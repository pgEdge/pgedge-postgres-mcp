/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package oauth implements a minimal OAuth 2.0 authorisation server: this
// file implements the device authorisation grant (RFC 8628): the device
// authorisation request (/oauth/device), the user-facing verification
// page (/oauth/device/verify) and, in token.go, the device grant's token
// endpoint handler.
package oauth

import (
	"crypto/rand"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxDeviceBodyBytes bounds the size of a device authorisation or
// verification request body, so an unauthenticated caller cannot exhaust
// memory with an oversized payload.
const maxDeviceBodyBytes = 64 * 1024

// deviceCodePollInterval is the minimum gap the client is asked to leave
// between polls of the token endpoint for a device code, per RFC 8628
// section 3.2.
const deviceCodePollInterval = 5 * time.Second

// userCodeAlphabet excludes characters that are easily confused when
// read aloud or copied by hand (no 0/O, 1/I/L, vowels that could spell an
// unintended word): only consonants and the digits 2-9.
const userCodeAlphabet = "BCDFGHJKLMNPQRSTVWXZ23456789"

// userCodeRejectionCeiling is the largest multiple of len(userCodeAlphabet)
// not exceeding 256: a random byte at or above this value is discarded
// and redrawn by newUserCode, so reducing the remaining range modulo
// len(userCodeAlphabet) introduces no bias towards any letter.
const userCodeRejectionCeiling = 252 // 28 * 9

// deviceResponse is the RFC 8628 section 3.2 device authorisation
// response body.
type deviceResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

// newUserCode returns a fresh 8-character user code drawn from
// userCodeAlphabet, formatted as two groups of four separated by a
// hyphen (e.g. "WXYZ-2345"). Each character is drawn by rejection
// sampling: a random byte at or above userCodeRejectionCeiling is
// discarded and redrawn, so every letter of the alphabet is equally
// likely, unlike a plain modulo reduction over the full byte range.
func newUserCode() (string, error) {
	var b [8]byte
	draw := make([]byte, 1)
	for i := 0; i < len(b); {
		if _, err := rand.Read(draw); err != nil {
			return "", errors.New("oauth: random user code: " + err.Error())
		}
		if draw[0] >= userCodeRejectionCeiling {
			continue
		}
		b[i] = userCodeAlphabet[int(draw[0])%len(userCodeAlphabet)]
		i++
	}
	return string(b[:4]) + "-" + string(b[4:]), nil
}

// normaliseUserCode upper-cases s, strips any spaces and hyphens the user
// may have typed or that formatting introduced, and re-inserts the
// canonical hyphen after the fourth character. It is applied both when
// storing a freshly generated code and when looking one up, so a code
// typed with different spacing or case still matches.
func normaliseUserCode(s string) string {
	s = strings.ToUpper(s)
	s = strings.NewReplacer(" ", "", "-", "").Replace(s)
	if len(s) > 4 {
		return s[:4] + "-" + s[4:]
	}
	return s
}

// handleDevice implements the device authorisation request (RFC 8628
// section 3.1): POST /oauth/device.
func (s *Server) handleDevice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	ip := s.clientIP(r)
	if s.opts.RateLimiter != nil && !s.opts.RateLimiter.IsAllowed(ip) {
		w.Header().Set("Retry-After", "60")
		writeJSONError(w, newError("access_denied", "too many requests", http.StatusTooManyRequests))
		return
	}

	fail := func(e *Error) {
		if s.opts.RateLimiter != nil {
			s.opts.RateLimiter.RecordFailedAttempt(ip)
		}
		writeJSONError(w, e)
	}

	if r.Method != http.MethodPost {
		fail(newError("invalid_request", "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxDeviceBodyBytes)
	if err := r.ParseForm(); err != nil {
		fail(newError("invalid_request", "malformed request", http.StatusBadRequest))
		return
	}

	clientID := r.PostFormValue("client_id")
	if clientID == "" {
		fail(newError("invalid_request", "client_id is required", http.StatusBadRequest))
		return
	}
	if _, ok := s.store.GetClient(clientID); !ok {
		s.logf("oauth device: client=%q ip=%s error=invalid_client", clientID, ip)
		fail(newError("invalid_client", "unknown client", http.StatusBadRequest))
		return
	}

	scope := r.PostFormValue("scope")
	for _, sc := range strings.Fields(scope) {
		if sc != ScopeMCP {
			fail(newError("invalid_scope", `scope must be "mcp"`, http.StatusBadRequest))
			return
		}
	}

	deviceCode, err := randomToken(32)
	if err != nil {
		fail(newError("server_error", "failed to generate device code", http.StatusInternalServerError))
		return
	}
	userCode, err := newUserCode()
	if err != nil {
		fail(newError("server_error", "failed to generate user code", http.StatusInternalServerError))
		return
	}

	now := s.now()
	d := &DeviceCode{
		DeviceHash: hashToken(deviceCode),
		UserCode:   userCode,
		ClientID:   clientID,
		Scope:      scope,
		ExpiresAt:  now.Add(s.opts.Config.DeviceCodeLifetime),
		Interval:   deviceCodePollInterval,
	}
	if err := s.store.PutDeviceCode(d); err != nil {
		fail(newError("server_error", "failed to store device code", http.StatusServiceUnavailable))
		return
	}

	verificationURI := s.Issuer() + DeviceVerifyPath
	q := url.Values{"user_code": {userCode}}

	s.logf("oauth device: client=%q ip=%s success", clientID, ip)
	writeJSON(w, http.StatusOK, deviceResponse{
		DeviceCode:              deviceCode,
		UserCode:                userCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURI + "?" + q.Encode(),
		ExpiresIn:               int64(s.opts.Config.DeviceCodeLifetime.Seconds()),
		Interval:                int64(deviceCodePollInterval.Seconds()),
	})
}

// pendingDeviceConsent resolves the client name and scope to name on
// the consent page for userCode, so that the resource owner is told
// which client is asking, and for what, before typing anything (RFC
// 8628 section 5.4). Both are empty when the code names no device that
// is still awaiting an answer, in which case the page simply asks for
// the code as before.
func (s *Server) pendingDeviceConsent(userCode string) (client, scope string) {
	if userCode == "" {
		return "", ""
	}
	d, ok := s.store.GetDeviceByUserCode(userCode)
	if !ok || d.Approved || d.Denied || !d.ExpiresAt.After(s.now()) {
		return "", ""
	}
	c, ok := s.store.GetClient(d.ClientID)
	if !ok {
		return d.ClientID, d.Scope
	}
	return clientDisplayName(c), d.Scope
}

// handleDeviceVerify implements the user-facing verification page (RFC
// 8628 section 3.3): GET renders the form pre-filled with the user_code
// from the query string, and POST verifies the resource owner's
// credentials, following the same CSRF and authentication pattern as
// handleAuthorize. It is rate limited per IP like the other endpoints,
// so that guessing user codes is bounded: a failed attempt is recorded
// whenever the submitted code does not resolve to a device that is still
// pending approval (unknown, expired, or already approved or denied),
// without distinguishing which, so a guesser learns nothing about which
// codes are live.
func (s *Server) handleDeviceVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeJSONError(w, newError("invalid_request", "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	ip := s.clientIP(r)
	if s.opts.RateLimiter != nil && !s.opts.RateLimiter.IsAllowed(ip) {
		w.Header().Set("Retry-After", "60")
		writeJSONError(w, newError("access_denied", "too many requests", http.StatusTooManyRequests))
		return
	}

	if r.Method == http.MethodGet {
		userCode := normaliseUserCode(r.URL.Query().Get("user_code"))
		client, scope := s.pendingDeviceConsent(userCode)
		s.logf("oauth device verify: ip=%s rendering form", ip)
		_ = s.page.Render(w, http.StatusOK, LoginPageData{
			CSRFToken:    s.csrf.Issue(s.now()),
			UserCode:     userCode,
			Client:       client,
			Scope:        scope,
			IsDeviceFlow: true,
			Page:         "device",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxDeviceBodyBytes)
	if err := r.ParseForm(); err != nil {
		writeJSONError(w, newError("invalid_request", "malformed request", http.StatusBadRequest))
		return
	}

	userCode := normaliseUserCode(r.PostFormValue("user_code"))

	client, scope := s.pendingDeviceConsent(userCode)

	// reRender re-displays the verification form with an error, issuing
	// a fresh CSRF token so the resource owner can retry without a
	// stale-token failure on the next attempt.
	reRender := func(status int, message string) {
		_ = s.page.Render(w, status, LoginPageData{
			Error:        message,
			CSRFToken:    s.csrf.Issue(s.now()),
			UserCode:     userCode,
			Client:       client,
			Scope:        scope,
			IsDeviceFlow: true,
			Page:         "device",
		})
	}

	if !s.csrf.Verify(r.PostFormValue("csrf_token"), s.now(), authorizeCSRFMaxAge) {
		s.logf("oauth device verify: ip=%s error=csrf_expired", ip)
		reRender(http.StatusBadRequest, "Your session expired, please try again")
		return
	}

	// invalidCode reports the same fixed message for an unknown code, an
	// expired one, and one that has already been resolved (approved or
	// denied), so a caller probing user codes cannot distinguish a live,
	// pending code from any other kind of miss.
	invalidCode := func(d *DeviceCode, ok bool) bool {
		return !ok || !d.ExpiresAt.After(s.now()) || d.Approved || d.Denied
	}

	// Refusing the request needs no credentials: the resource owner has
	// said no, which is an answer the device is entitled to receive as
	// soon as it is given.
	if r.PostFormValue("action") == "deny" {
		d, ok := s.store.GetDeviceByUserCode(userCode)
		if invalidCode(d, ok) || !s.store.DenyDevice(d.DeviceHash) {
			if s.opts.RateLimiter != nil {
				s.opts.RateLimiter.RecordFailedAttempt(ip)
			}
			s.logf("oauth device verify: ip=%s error=unknown_expired_or_resolved_code", ip)
			reRender(http.StatusBadRequest, "That code is not valid or has expired")
			return
		}
		s.logf("oauth device verify: ip=%s denied", ip)
		_ = s.page.Render(w, http.StatusOK, LoginPageData{
			Page:    "done",
			Message: "Request denied. You can close this window.",
		})
		return
	}

	d, ok := s.store.GetDeviceByUserCode(userCode)
	if invalidCode(d, ok) {
		if s.opts.RateLimiter != nil {
			s.opts.RateLimiter.RecordFailedAttempt(ip)
		}
		s.logf("oauth device verify: ip=%s error=unknown_expired_or_resolved_code", ip)
		reRender(http.StatusBadRequest, "That code is not valid or has expired")
		return
	}

	subject, err := s.authenticateForm(r, ip)
	if err != nil {
		if errors.Is(err, ErrRateLimited) {
			s.logf("oauth device verify: ip=%s error=rate_limited", ip)
			reRender(http.StatusTooManyRequests, "Too many attempts, please wait and try again")
			return
		}
		s.logf("oauth device verify: ip=%s error=invalid_credentials", ip)
		reRender(http.StatusUnauthorized, "Invalid username or password")
		return
	}

	// ApproveDevice is the single-writer gate: if another request
	// already approved (or denied) this device code between the lookup
	// above and here, this call loses and the code is reported as
	// invalid rather than silently overwriting the earlier approval.
	if !s.store.ApproveDevice(d.DeviceHash, subject) {
		s.logf("oauth device verify: subject=%q ip=%s error=already_resolved", subject, ip)
		reRender(http.StatusBadRequest, "That code is not valid or has expired")
		return
	}

	s.logf("oauth device verify: subject=%q ip=%s success", subject, ip)
	_ = s.page.Render(w, http.StatusOK, LoginPageData{Page: "done"})
}

// handleDeviceCodeGrant implements the token endpoint's handling of the
// device grant (RFC 8628 section 3.4). Polling bookkeeping and the
// approved check both happen under the store's single lock (via
// TouchDevicePoll and TakeDeviceIfApproved) so that two concurrent polls
// of the same device code can never both be told to proceed: at most one
// can see slow_down == false, and at most one can take the code once
// approved. On success it writes the token response itself and returns
// nil.
func (s *Server) handleDeviceCodeGrant(w http.ResponseWriter, r *http.Request) *Error {
	deviceCode := r.PostFormValue("device_code")
	clientID := r.PostFormValue("client_id")

	if deviceCode == "" || clientID == "" {
		return newError("invalid_request", "device_code and client_id are required", http.StatusBadRequest)
	}

	hash := hashToken(deviceCode)
	d, ok := s.store.GetDeviceByHash(hash)
	if !ok || d.ClientID != clientID {
		s.logf("oauth token: client=%q grant=device error=invalid_grant", clientID)
		return newError("invalid_grant", "device code is invalid or does not belong to this client", http.StatusBadRequest)
	}

	now := s.now()
	if !d.ExpiresAt.After(now) {
		s.store.DeleteDevice(hash)
		s.logf("oauth token: client=%q grant=device error=expired_token", clientID)
		return newError("expired_token", "device code has expired", http.StatusBadRequest)
	}

	polled, tooSoon, ok := s.store.TouchDevicePoll(hash, now)
	if !ok {
		// Raced with another poll that deleted the device code between
		// the checks above and here (expiry sweep, or a concurrent
		// successful issuance).
		s.logf("oauth token: client=%q grant=device error=invalid_grant", clientID)
		return newError("invalid_grant", "device code is invalid or does not belong to this client", http.StatusBadRequest)
	}
	if tooSoon {
		return newError("slow_down", "polling too frequently", http.StatusBadRequest)
	}

	if polled.Denied {
		s.store.DeleteDevice(hash)
		s.logf("oauth token: client=%q grant=device error=access_denied", clientID)
		return newError("access_denied", "the resource owner denied the request", http.StatusBadRequest)
	}
	if !polled.Approved {
		return newError("authorization_pending", "the user has not yet approved this device", http.StatusBadRequest)
	}

	taken, ok := s.store.TakeDeviceIfApproved(hash)
	if !ok {
		// Another concurrent poll took it first.
		return newError("authorization_pending", "the user has not yet approved this device", http.StatusBadRequest)
	}

	tr, err := s.issueTokens(clientID, taken.Subject, taken.Scope, "")
	if err != nil {
		return newError("server_error", "failed to issue tokens", http.StatusServiceUnavailable)
	}

	s.logf("oauth token: client=%q subject=%q grant=device success", clientID, taken.Subject)
	writeJSON(w, http.StatusOK, tr)
	return nil
}
