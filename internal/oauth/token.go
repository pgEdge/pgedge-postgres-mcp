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

// handleToken implements the token endpoint. It accepts only POST
// requests with a form-encoded body, dispatching on grant_type to the
// authorisation code, refresh token or (in a later task) device grants.
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	if r.Method != http.MethodPost {
		writeJSONError(w, newError("invalid_request", "method not allowed", http.StatusMethodNotAllowed))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxTokenBodyBytes)
	if err := r.ParseForm(); err != nil {
		writeJSONError(w, newError("invalid_request", "malformed request", http.StatusBadRequest))
		return
	}

	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, r)
	case "refresh_token":
		s.handleRefreshTokenGrant(w, r)
	case DeviceGrantType:
		// Implemented in a later task.
		writeJSONError(w, newError("unsupported_grant_type", "device grant not yet supported", http.StatusBadRequest))
	default:
		writeJSONError(w, newError("unsupported_grant_type", "unsupported grant_type", http.StatusBadRequest))
	}
}

// handleAuthorizationCodeGrant implements RFC 6749 section 4.1.3, the
// authorisation code grant with PKCE (RFC 7636).
func (s *Server) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.PostFormValue("code")
	clientID := r.PostFormValue("client_id")
	redirectURI := r.PostFormValue("redirect_uri")
	verifier := r.PostFormValue("code_verifier")

	if code == "" || clientID == "" || redirectURI == "" || verifier == "" {
		writeJSONError(w, newError("invalid_request", "code, client_id, redirect_uri and code_verifier are required", http.StatusBadRequest))
		return
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
		}
		writeJSONError(w, newError("invalid_grant", "code is invalid, expired or already used", http.StatusBadRequest))
		return
	}
	if !authCode.ExpiresAt.After(s.now()) {
		writeJSONError(w, newError("invalid_grant", "code is invalid, expired or already used", http.StatusBadRequest))
		return
	}
	if authCode.ClientID != clientID || authCode.RedirectURI != redirectURI {
		writeJSONError(w, newError("invalid_grant", "client_id or redirect_uri does not match the authorisation request", http.StatusBadRequest))
		return
	}
	if !validCodeVerifier(verifier) || !verifyPKCE(verifier, authCode.CodeChallenge, "S256") {
		writeJSONError(w, newError("invalid_grant", "code_verifier does not match", http.StatusBadRequest))
		return
	}

	tr, refreshHash, err := s.issueTokensWithHash(clientID, authCode.Subject, authCode.Scope)
	if err != nil {
		writeJSONError(w, newError("server_error", "failed to issue tokens", http.StatusServiceUnavailable))
		return
	}

	s.store.MarkCodeUsed(codeHash, refreshHash, authCode.ExpiresAt.Add(s.opts.Config.AccessTokenLifetime))

	s.logf("oauth token: client=%q subject=%q grant=authorization_code success", clientID, authCode.Subject)
	writeJSON(w, http.StatusOK, tr)
}

// handleRefreshTokenGrant implements RFC 6749 section 6, the refresh
// token grant. The presented refresh token is rotated: it is deleted
// (cascading to the access token it issued) and a fresh pair is issued in
// its place.
func (s *Server) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.PostFormValue("refresh_token")
	clientID := r.PostFormValue("client_id")

	if refreshToken == "" || clientID == "" {
		writeJSONError(w, newError("invalid_request", "refresh_token and client_id are required", http.StatusBadRequest))
		return
	}

	t, ok := s.store.GetToken(hashToken(refreshToken))
	if !ok || !t.IsRefresh || !t.ExpiresAt.After(s.now()) || t.ClientID != clientID {
		writeJSONError(w, newError("invalid_grant", "refresh token is invalid, expired or does not belong to this client", http.StatusBadRequest))
		return
	}

	s.store.DeleteToken(t.Hash)

	tr, err := s.issueTokens(clientID, t.Subject, t.Scope)
	if err != nil {
		writeJSONError(w, newError("server_error", "failed to issue tokens", http.StatusServiceUnavailable))
		return
	}

	s.logf("oauth token: client=%q subject=%q grant=refresh_token success", clientID, t.Subject)
	writeJSON(w, http.StatusOK, tr)
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
