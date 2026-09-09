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
// file implements the /oauth/authorize endpoint, which renders the login
// page and, on a successful sign-in, issues a single-use authorisation
// code.
package oauth

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxAuthorizeBodyBytes bounds the size of the login form submission, so
// an unauthenticated caller cannot exhaust memory with an oversized body.
const maxAuthorizeBodyBytes = 64 * 1024

// authorizeCSRFMaxAge is how long a CSRF token issued with the login page
// remains valid for the matching form submission.
const authorizeCSRFMaxAge = 10 * time.Minute

// parseAuthorizeParams extracts the authorisation request parameters from
// v, which may come from a GET request's query string or a POST request's
// form body.
func parseAuthorizeParams(v url.Values) AuthorizeParams {
	return AuthorizeParams{
		ResponseType:        v.Get("response_type"),
		ClientID:            v.Get("client_id"),
		RedirectURI:         v.Get("redirect_uri"),
		Scope:               v.Get("scope"),
		State:               v.Get("state"),
		CodeChallenge:       v.Get("code_challenge"),
		CodeChallengeMethod: v.Get("code_challenge_method"),
	}
}

// validateAuthorize resolves the client named by p.ClientID and confirms
// that p.RedirectURI is permitted for it. A client with its own registered
// redirect URIs must match one of those; a client with none registered
// (there is none, currently, since registration requires at least one)
// falls back to the server's configured allow-list plus ExtraRedirects.
// Failure here means the redirect URI itself cannot be trusted, so the
// caller must not redirect the browser back to it.
func (s *Server) validateAuthorize(p AuthorizeParams) (*Client, *Error) {
	invalid := newError("invalid_request", "Invalid client or redirect URI", http.StatusBadRequest)

	client, ok := s.store.GetClient(p.ClientID)
	if !ok {
		return nil, invalid
	}

	allowed := client.RedirectURIs
	if len(allowed) == 0 {
		allowed = append(append([]string(nil), s.opts.Config.AllowedRedirectURIs...), s.opts.ExtraRedirects...)
	}
	if !redirectURIAllowed(p.RedirectURI, allowed) {
		return nil, invalid
	}
	return client, nil
}

// checkAuthorizeParams validates the remaining authorisation request
// parameters, once the client and redirect URI are known good. Any
// failure here is reported by redirecting back to the (now trusted)
// redirect URI with an error, per RFC 6749.
func checkAuthorizeParams(p AuthorizeParams) *Error {
	if p.ResponseType != "code" {
		return newError("unsupported_response_type", `response_type must be "code"`, http.StatusFound)
	}
	if p.State == "" {
		return newError("invalid_request", "state is required", http.StatusFound)
	}
	if !validCodeChallenge(p.CodeChallenge) {
		return newError("invalid_request", "code_challenge is required and must be a valid S256 challenge", http.StatusFound)
	}
	if p.CodeChallengeMethod != "S256" {
		return newError("invalid_request", "code_challenge_method must be S256", http.StatusFound)
	}
	for _, scope := range strings.Fields(p.Scope) {
		if scope != ScopeMCP {
			return newError("invalid_scope", `scope must be "mcp"`, http.StatusFound)
		}
	}
	return nil
}

// clientDisplayName returns the name shown to the resource owner on the
// login page for client, falling back to its ID when it registered
// without a display name.
func clientDisplayName(client *Client) string {
	if client.Name != "" {
		return client.Name
	}
	return client.ID
}

// handleAuthorize implements the authorisation endpoint of the
// authorisation code grant (RFC 6749 section 4.1), rendering a login page
// on GET and, on POST, verifying the submitted credentials and issuing an
// authorisation code.
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeJSONError(w, newError("invalid_request", "method not allowed", http.StatusMethodNotAllowed))
		return
	}
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, maxAuthorizeBodyBytes)
	}
	if err := r.ParseForm(); err != nil {
		writeJSONError(w, newError("invalid_request", "malformed request", http.StatusBadRequest))
		return
	}

	p := parseAuthorizeParams(r.Form)
	ip := s.clientIP(r)

	client, cerr := s.validateAuthorize(p)
	if cerr != nil {
		s.logf("oauth authorize: client=%q ip=%s error=%s", p.ClientID, ip, cerr.Code)
		_ = s.page.Render(w, cerr.Status, LoginPageData{Error: cerr.Description, OAuth: p, Page: "login"})
		return
	}
	display := clientDisplayName(client)

	if perr := checkAuthorizeParams(p); perr != nil {
		s.logf("oauth authorize: client=%q ip=%s error=%s", p.ClientID, ip, perr.Code)
		redirectError(w, r, p.RedirectURI, p.State, perr)
		return
	}

	if r.Method == http.MethodGet {
		s.logf("oauth authorize: client=%q ip=%s rendering login form", p.ClientID, ip)
		_ = s.page.Render(w, http.StatusOK, LoginPageData{
			CSRFToken: s.csrf.Issue(s.now()),
			Client:    display,
			OAuth:     p,
			Page:      "login",
		})
		return
	}

	// reRender re-displays the login form with an error, issuing a fresh
	// CSRF token so the resource owner can retry without a stale-token
	// failure on the next attempt.
	reRender := func(status int, message string) {
		_ = s.page.Render(w, status, LoginPageData{
			Error:     message,
			CSRFToken: s.csrf.Issue(s.now()),
			Client:    display,
			OAuth:     p,
			Page:      "login",
		})
	}

	if !s.csrf.Verify(r.PostFormValue("csrf_token"), s.now(), authorizeCSRFMaxAge) {
		s.logf("oauth authorize: client=%q ip=%s error=csrf_expired", p.ClientID, ip)
		reRender(http.StatusBadRequest, "Your session expired, please try again")
		return
	}

	username := r.PostFormValue("username")
	password := r.PostFormValue("password")
	subject, err := s.opts.Authenticator.Authenticate(r.Context(), username, password, ip)
	if err != nil {
		if errors.Is(err, ErrRateLimited) {
			s.logf("oauth authorize: client=%q ip=%s error=rate_limited", p.ClientID, ip)
			reRender(http.StatusTooManyRequests, "Too many attempts, please wait and try again")
			return
		}
		s.logf("oauth authorize: client=%q ip=%s error=invalid_credentials", p.ClientID, ip)
		reRender(http.StatusUnauthorized, "Invalid username or password")
		return
	}

	code, err := randomToken(32)
	if err != nil {
		s.logf("oauth authorize: client=%q subject=%q ip=%s error=server_error", p.ClientID, subject, ip)
		reRender(http.StatusInternalServerError, "Something went wrong, please try again")
		return
	}

	authCode := &AuthCode{
		Hash:          hashToken(code),
		ClientID:      p.ClientID,
		RedirectURI:   p.RedirectURI,
		Subject:       subject,
		Scope:         p.Scope,
		CodeChallenge: p.CodeChallenge,
		ExpiresAt:     s.now().Add(s.opts.Config.AuthorizationCodeLifetime),
	}
	if err := s.store.PutCode(authCode); err != nil {
		s.logf("oauth authorize: client=%q subject=%q ip=%s error=store_full", p.ClientID, subject, ip)
		reRender(http.StatusServiceUnavailable, "Service busy")
		return
	}

	u, err := url.Parse(p.RedirectURI)
	if err != nil {
		writeJSONError(w, newError("server_error", "invalid redirect_uri", http.StatusInternalServerError))
		return
	}
	q := u.Query()
	q.Set("code", code)
	q.Set("state", p.State)
	u.RawQuery = q.Encode()

	s.logf("oauth authorize: client=%q subject=%q ip=%s success", p.ClientID, subject, ip)
	http.Redirect(w, r, u.String(), http.StatusFound)
}
