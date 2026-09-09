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
// file implements the /oauth/token endpoint, covering the authorisation
// code and refresh token grants (RFC 6749 sections 4.1.3 and 6). The
// device grant itself (RFC 8628 section 3.4) is implemented in device.go.
package oauth

import (
	"encoding/json"
	"net/http"
)

// maxTokenBodyBytes bounds the size of a token request body, so an
// unauthenticated caller cannot exhaust memory with an oversized payload.
const maxTokenBodyBytes = 64 * 1024

// tokenResponse is the RFC 6749 section 5.1 access token response body.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}

// genericAuthCodeInvalidGrant is returned for every failure mode of the
// authorisation code grant that RFC 6749 classes as invalid_grant.
// Collapsing the distinct causes (unknown/expired code, client or
// redirect mismatch, verifier mismatch, replay) to one wire description
// avoids handing an attacker a signal for which part of the request was
// wrong; the distinct cause is still visible to operators via s.logf.
const genericAuthCodeInvalidGrant = "authorisation code is invalid, expired, already used, or does not match this request"

// genericRefreshInvalidGrant is the equivalent collapsed description for
// the refresh token grant.
const genericRefreshInvalidGrant = "refresh token is invalid, expired, or does not belong to this client"

// tokenGrantShouldRecord reports whether a failed token request should
// count against the per-IP rate limiter. Only invalid_grant and
// invalid_client outcomes of the authorization_code and refresh_token
// grants are recorded: unsupported_grant_type and invalid_request never
// are. For the device grant, only invalid_grant (an unknown device code
// or one presented by the wrong client) is recorded; authorization_pending,
// slow_down, expired_token and access_denied never are, since a
// legitimate device client is expected to poll every few seconds and
// must not be penalised for doing so.
func tokenGrantShouldRecord(grantType, errCode string) bool {
	if grantType == DeviceGrantType {
		return errCode == "invalid_grant"
	}
	if grantType != "authorization_code" && grantType != "refresh_token" {
		return false
	}
	return errCode == "invalid_grant" || errCode == "invalid_client"
}

// handleToken implements the token endpoint. It accepts only POST
// requests with a form-encoded body, dispatching on grant_type to the
// authorisation code, refresh token or device grants.
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	ip := s.clientIP(r)

	if r.Method != http.MethodPost {
		writeJSONError(w, newError("invalid_request", "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxTokenBodyBytes)
	if err := r.ParseForm(); err != nil {
		writeJSONError(w, newError("invalid_request", "malformed request", http.StatusBadRequest))
		return
	}

	grantType := r.PostFormValue("grant_type")

	// The limiter is shared with every other endpoint, and a device
	// client is expected to poll every few seconds throughout the
	// grant. Since polling never records a failure of its own, it must
	// not be blocked by unrelated failures from the same address
	// either; the grant's own abuse is bounded by the per-code interval
	// enforced in TouchDevicePoll.
	if grantType != DeviceGrantType && s.opts.RateLimiter != nil && !s.opts.RateLimiter.IsAllowed(ip) {
		w.Header().Set("Retry-After", "60")
		writeJSONError(w, newError("access_denied", "too many requests", http.StatusTooManyRequests))
		return
	}

	// Any grant reaching this point names a client that is still in
	// use, so keep its registration alive.
	if clientID := r.PostFormValue("client_id"); clientID != "" {
		s.store.TouchClient(clientID, s.now())
	}

	var e *Error
	switch grantType {
	case "authorization_code":
		e = s.handleAuthorizationCodeGrant(w, r)
	case "refresh_token":
		e = s.handleRefreshTokenGrant(w, r)
	case DeviceGrantType:
		e = s.handleDeviceCodeGrant(w, r)
	default:
		e = newError("unsupported_grant_type", "unsupported grant_type", http.StatusBadRequest)
	}
	if e == nil {
		return
	}
	if s.opts.RateLimiter != nil && tokenGrantShouldRecord(grantType, e.Code) {
		s.opts.RateLimiter.RecordFailedAttempt(ip)
	}
	writeJSONError(w, e)
}

// handleAuthorizationCodeGrant implements RFC 6749 section 4.1.3, the
// authorisation code grant with PKCE (RFC 7636). On success it writes the
// token response itself and returns nil; on failure it returns the error
// for the caller to write and, where appropriate, count against the rate
// limiter.
func (s *Server) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) *Error {
	code := r.PostFormValue("code")
	clientID := r.PostFormValue("client_id")
	redirectURI := r.PostFormValue("redirect_uri")
	verifier := r.PostFormValue("code_verifier")

	if code == "" || clientID == "" || redirectURI == "" || verifier == "" {
		return newError("invalid_request", "code, client_id, redirect_uri and code_verifier are required", http.StatusBadRequest)
	}

	codeHash := hashToken(code)
	authCode, ok := s.store.TakeCode(codeHash)
	if !ok {
		// Either unknown, or already redeemed: if it was already
		// redeemed, this is a replay, so revoke everything issued from
		// it, per RFC 6749 section 4.1.2.
		if refreshHash, used := s.store.TakeUsedCode(codeHash); used {
			s.store.DeleteToken(refreshHash)
			s.logf("oauth token: client=%q error=replayed_code", clientID)
		} else {
			s.logf("oauth token: client=%q error=unknown_code", clientID)
		}
		return newError("invalid_grant", genericAuthCodeInvalidGrant, http.StatusBadRequest)
	}
	if !authCode.ExpiresAt.After(s.now()) {
		s.logf("oauth token: client=%q error=expired_code", clientID)
		return newError("invalid_grant", genericAuthCodeInvalidGrant, http.StatusBadRequest)
	}
	if authCode.ClientID != clientID || authCode.RedirectURI != redirectURI {
		s.logf("oauth token: client=%q error=client_or_redirect_mismatch", clientID)
		return newError("invalid_grant", genericAuthCodeInvalidGrant, http.StatusBadRequest)
	}
	if !validCodeVerifier(verifier) || !verifyPKCE(verifier, authCode.CodeChallenge, "S256") {
		s.logf("oauth token: client=%q error=verifier_mismatch", clientID)
		return newError("invalid_grant", genericAuthCodeInvalidGrant, http.StatusBadRequest)
	}

	// A code grant starts a new token family, which every rotation of
	// the resulting refresh token then carries forward.
	tr, refreshHash, err := s.issueTokensWithHash(clientID, authCode.Subject, authCode.Scope, "")
	if err != nil {
		return newError("server_error", "failed to issue tokens", http.StatusServiceUnavailable)
	}

	s.store.MarkCodeUsed(codeHash, refreshHash, authCode.ExpiresAt.Add(s.opts.Config.AccessTokenLifetime))

	s.logf("oauth token: client=%q subject=%q grant=authorization_code success", clientID, authCode.Subject)
	writeJSON(w, http.StatusOK, tr)
	return nil
}

// handleRefreshTokenGrant implements RFC 6749 section 6, the refresh
// token grant. The presented refresh token is taken (atomically looked
// up and removed, cascading to the access token it issued) before being
// validated, so that two concurrent presentations of the same refresh
// token can never both succeed: whichever loses the race finds the token
// already gone and fails closed with invalid_grant. On success it writes
// the token response itself and returns nil.
func (s *Server) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) *Error {
	refreshToken := r.PostFormValue("refresh_token")
	clientID := r.PostFormValue("client_id")

	if refreshToken == "" || clientID == "" {
		return newError("invalid_request", "refresh_token and client_id are required", http.StatusBadRequest)
	}

	refreshHash := hashToken(refreshToken)
	t, ok := s.store.TakeToken(refreshHash)
	if !ok {
		// A refresh token that is not on file may simply be unknown, or
		// it may be one this server rotated away: presenting a rotated
		// token means either the client or an attacker holds a copy of
		// it, and there is no way to tell which, so the whole family
		// descended from the original authorisation is revoked.
		if family, rotated := s.store.RotatedRefreshFamily(refreshHash); rotated {
			s.store.DeleteFamily(family)
			s.logf("oauth token: WARNING client=%q family=%q error=replayed_refresh_token, revoking the token family", clientID, family)
		} else {
			s.logf("oauth token: client=%q error=unknown_refresh_token", clientID)
		}
		return newError("invalid_grant", genericRefreshInvalidGrant, http.StatusBadRequest)
	}
	if !t.IsRefresh || !t.ExpiresAt.After(s.now()) || t.ClientID != clientID {
		// The token is already gone (TakeToken removed it above), which
		// is the correct outcome for a misuse of a valid-looking token:
		// an access token presented here, an expired refresh token, or
		// one presented by the wrong client.
		s.logf("oauth token: client=%q error=refresh_token_invalid_expired_or_wrong_client", clientID)
		return newError("invalid_grant", genericRefreshInvalidGrant, http.StatusBadRequest)
	}

	tr, err := s.issueTokens(clientID, t.Subject, t.Scope, t.Family)
	if err != nil {
		return newError("server_error", "failed to issue tokens", http.StatusServiceUnavailable)
	}

	// Remember the token just rotated away, so that a later replay of
	// it is recognised rather than merely rejected as unknown.
	s.store.MarkRefreshRotated(refreshHash, t.Family, s.now().Add(s.opts.Config.RefreshTokenLifetime))

	s.logf("oauth token: client=%q subject=%q grant=refresh_token success", clientID, t.Subject)
	writeJSON(w, http.StatusOK, tr)
	return nil
}

// issueTokens issues a fresh access/refresh token pair for clientID and
// subject with the given scope, storing both in the token store. It is
// used by the authorisation code and refresh token grants, and by the
// device grant. An empty family starts a new one.
func (s *Server) issueTokens(clientID, subject, scope, family string) (tokenResponse, error) {
	tr, _, err := s.issueTokensWithHash(clientID, subject, scope, family)
	return tr, err
}

// issueTokensWithHash is issueTokens, additionally returning the hash of
// the refresh token it stored, so the authorisation code grant can record
// it against the redeemed code for later replay detection.
func (s *Server) issueTokensWithHash(clientID, subject, scope, family string) (tokenResponse, string, error) {
	accessToken, err := randomToken(32)
	if err != nil {
		return tokenResponse{}, "", err
	}
	refreshToken, err := randomToken(32)
	if err != nil {
		return tokenResponse{}, "", err
	}
	if family == "" {
		if family, err = randomToken(16); err != nil {
			return tokenResponse{}, "", err
		}
	}

	accessHash := hashToken(accessToken)
	refreshHash := hashToken(refreshToken)
	now := s.now()

	refresh := &Token{
		Hash:      refreshHash,
		ClientID:  clientID,
		Subject:   subject,
		Scope:     scope,
		ExpiresAt: now.Add(s.opts.Config.RefreshTokenLifetime),
		IsRefresh: true,
		Issued:    []string{accessHash},
		Family:    family,
	}
	if err := s.store.PutToken(refresh); err != nil {
		return tokenResponse{}, "", err
	}

	access := &Token{
		Hash:        accessHash,
		ClientID:    clientID,
		Subject:     subject,
		Scope:       scope,
		ExpiresAt:   now.Add(s.opts.Config.AccessTokenLifetime),
		RefreshHash: refreshHash,
		Family:      family,
	}
	if err := s.store.PutToken(access); err != nil {
		s.store.DeleteToken(refreshHash)
		return tokenResponse{}, "", err
	}

	return tokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.opts.Config.AccessTokenLifetime.Seconds()),
		RefreshToken: refreshToken,
		Scope:        scope,
	}, refreshHash, nil
}

// writeJSON writes v to w as a JSON body with the given HTTP status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
