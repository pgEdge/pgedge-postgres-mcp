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
	"net/http"
	"slices"

	"pgedge-postgres-mcp/internal/auth"
)

// maxRegistrationBodyBytes bounds the size of a dynamic client
// registration request body, so an unauthenticated caller cannot exhaust
// memory with an oversized payload.
const maxRegistrationBodyBytes = 64 * 1024

// maxClientNameRunes bounds the length of a client's display name, as
// stored and echoed back in the registration response.
const maxClientNameRunes = 100

// supportedGrantTypes lists the grant types a dynamically registered
// client may request.
var supportedGrantTypes = []string{"authorization_code", "refresh_token", DeviceGrantType}

// registrationRequest is the RFC 7591 dynamic client registration
// request body.
type registrationRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

// registrationResponse is the RFC 7591 dynamic client registration
// response body.
type registrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

// handleRegister implements RFC 7591 dynamic client registration for
// public (no client secret) clients only.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.opts.Config.DynamicRegistrationAllowed() {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, newError("invalid_request", "method not allowed", http.StatusMethodNotAllowed))
		return
	}

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

	r.Body = http.MaxBytesReader(w, r.Body, maxRegistrationBodyBytes)
	var req registrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(newError("invalid_request", "malformed request body", http.StatusBadRequest))
		return
	}

	if len(req.RedirectURIs) == 0 {
		fail(newError("invalid_redirect_uri", "redirect_uris is required", http.StatusBadRequest))
		return
	}
	allowed := append(append([]string(nil), s.opts.Config.AllowedRedirectURIs...), s.opts.ExtraRedirects...)
	for _, uri := range req.RedirectURIs {
		if !redirectURIAllowed(uri, allowed) {
			fail(newError("invalid_redirect_uri", "redirect_uri not permitted: "+uri, http.StatusBadRequest))
			return
		}
	}

	if req.TokenEndpointAuthMethod != "" && req.TokenEndpointAuthMethod != "none" {
		fail(newError("invalid_request", "only public clients are supported", http.StatusBadRequest))
		return
	}

	grantTypes := req.GrantTypes
	if len(grantTypes) == 0 {
		grantTypes = []string{"authorization_code", "refresh_token"}
	} else {
		for _, gt := range grantTypes {
			if !slices.Contains(supportedGrantTypes, gt) {
				fail(newError("invalid_request", "unsupported grant type: "+gt, http.StatusBadRequest))
				return
			}
		}
	}

	responseTypes := req.ResponseTypes
	if len(responseTypes) == 0 {
		responseTypes = []string{"code"}
	}

	clientName := truncateRunes(req.ClientName, maxClientNameRunes)

	clientID, err := randomToken(16)
	if err != nil {
		fail(newError("server_error", "failed to generate client id", http.StatusInternalServerError))
		return
	}

	now := s.now()
	client := &Client{
		ID:           clientID,
		Name:         clientName,
		RedirectURIs: req.RedirectURIs,
		CreatedAt:    now,
	}
	if err := s.store.PutClient(client); err != nil {
		fail(newError("server_error", "failed to store client", http.StatusInternalServerError))
		return
	}

	resp := registrationResponse{
		ClientID:                clientID,
		ClientIDIssuedAt:        now.Unix(),
		ClientName:              clientName,
		RedirectURIs:            req.RedirectURIs,
		TokenEndpointAuthMethod: "none",
		GrantTypes:              grantTypes,
		ResponseTypes:           responseTypes,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// truncateRunes truncates s to at most n runes.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// clientIP resolves the client IP address for req, using the server's
// configured resolver when set and falling back to auth.ExtractIPAddress
// otherwise.
func (s *Server) clientIP(r *http.Request) string {
	if s.opts.ClientIP != nil {
		return s.opts.ClientIP.Resolve(r)
	}
	return auth.ExtractIPAddress(r)
}
