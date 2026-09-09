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
// device grant is added in a later task.
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
// are, and neither is any outcome of the device grant, since a
// legitimate device client is expected to poll every few seconds and
// receive authorization_pending or slow_down without being penalised.
func tokenGrantShouldRecord(grantType, errCode string) bool {
	if grantType != "authorization_code" && grantType != "refresh_token" {
		return false
	}
	return errCode == "invalid_grant" || errCode == "invalid_client"
}

// handleToken implements the token endpoint. It accepts only POST
// requests with a form-encoded body, dispatching on grant_type to the
// authorisation code, refresh token or (in a later task) device grants.
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	ip := s.clientIP(r)
	if s.opts.RateLimiter != nil && !s.opts.RateLimiter.IsAllowed(ip) {
		w.Header().Set("Retry-After", "60")
		writeJSONError(w, newError("access_denied", "too many requests", http.StatusTooManyRequests))
		return
	}

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
	var e *Error
	switch grantType {
	case "authorization_code":
		e = s.handleAuthorizationCodeGrant(w, r)
	case "refresh_token":
		e = s.handleRefreshTokenGrant(w, r)
	case DeviceGrantType:
		// Implemented in a later task.
		e = newError("unsupported_grant_type", "device grant not yet supported", http.StatusBadRequest)
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

	tr, refreshHash, err := s.issueTokensWithHash(clientID, authCode.Subject, authCode.Scope)
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

	t, ok := s.store.TakeToken(hashToken(refreshToken))
	if !ok {
		s.logf("oauth token: client=%q error=unknown_refresh_token", clientID)
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

	tr, err := s.issueTokens(clientID, t.Subject, t.Scope)
	if err != nil {
		return newError("server_error", "failed to issue tokens", http.StatusServiceUnavailable)
	}

	s.logf("oauth token: client=%q subject=%q grant=refresh_token success", clientID, t.Subject)
	writeJSON(w, http.StatusOK, tr)
	return nil
}

// issueTokens issues a fresh access/refresh token pair for clientID and
// subject with the given scope, storing both in the token store. It is
// used by the authorisation code and refresh token grants, and by the
// device grant.
func (s *Server) issueTokens(clientID, subject, scope string) (tokenResponse, error) {
	tr, _, err := s.issueTokensWithHash(clientID, subject, scope)
	return tr, err
}

// issueTokensWithHash is issueTokens, additionally returning the hash of
// the refresh token it stored, so the authorisation code grant can record
// it against the redeemed code for later replay detection.
func (s *Server) issueTokensWithHash(clientID, subject, scope string) (tokenResponse, string, error) {
	accessToken, err := randomToken(32)
	if err != nil {
		return tokenResponse{}, "", err
	}
	refreshToken, err := randomToken(32)
	if err != nil {
		return tokenResponse{}, "", err
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
