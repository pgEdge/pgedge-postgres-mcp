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
// file provides the Server skeleton, wiring together the store, CSRF
// signer and HTTP handlers behind a single Options value.
package oauth

import (
	"errors"
	"net/http"
	"time"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
)

// Options configures a Server.
type Options struct {
	Config config.OAuthConfig

	// Authenticator verifies resource owner credentials on the login
	// page. Provided by Task 4's UserStoreAuthenticator in production.
	Authenticator Authenticator

	// RateLimiter throttles repeated failed attempts by client IP. May
	// be nil, in which case no rate limiting is applied (as in most
	// tests).
	RateLimiter *auth.RateLimiter

	// ClientIP resolves the client IP address from a request, honouring
	// trusted proxy configuration. May be nil, in which case
	// auth.ExtractIPAddress is used directly.
	ClientIP *auth.ClientIPResolver

	// ExtraRedirects lists additional redirect URIs to accept beyond
	// Config.AllowedRedirectURIs, such as http.allowed_origins plus
	// "/oauth/callback", added by main.
	ExtraRedirects []string

	// Logger receives diagnostic messages, in the style of log.Printf.
	// May be nil, in which case messages are discarded.
	Logger func(format string, args ...any)

	// Now returns the current time. May be nil, in which case
	// time.Now is used; tests override it for deterministic behaviour.
	Now func() time.Time
}

// Server is a minimal OAuth 2.0 authorisation server: metadata,
// registration, authorisation, token, device and revocation endpoints
// backed by an in-memory Store.
type Server struct {
	opts      Options
	store     *Store
	csrf      *csrfSigner
	stopSweep func()
}

// New creates a Server from opts. It validates that an issuer has been
// configured, builds the in-memory store and CSRF signer, and starts the
// background sweeper that expires stale entries.
func New(opts Options) (*Server, error) {
	if opts.Config.Issuer == "" {
		return nil, errors.New("oauth: issuer must not be empty")
	}

	signer, err := newCSRFSigner()
	if err != nil {
		return nil, err
	}

	s := &Server{
		opts:  opts,
		store: NewStore(DefaultLimits),
		csrf:  signer,
	}
	s.stopSweep = s.store.StartSweeper(time.Minute)
	return s, nil
}

// RegisterRoutes registers the authorisation server's HTTP handlers on
// mux. This task registers the metadata, protected resource and dynamic
// client registration endpoints; later tasks add authorisation, token,
// device and revocation routes.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(MetadataPath, s.handleMetadata)
	mux.HandleFunc(ProtectedResourcePath, s.handleProtectedResourceMetadata)
	mux.HandleFunc(RegisterPath, s.handleRegister)
}

// Close stops the background sweeper. It is safe to call more than once.
func (s *Server) Close() {
	if s.stopSweep != nil {
		s.stopSweep()
	}
}

// Issuer returns the configured issuer URL.
func (s *Server) Issuer() string {
	return s.opts.Config.Issuer
}

// ValidateAccessToken reports whether token is a currently valid access
// token, returning the subject and client identifier it was issued to.
func (s *Server) ValidateAccessToken(token string) (subject, clientID string, ok bool) {
	t, found := s.store.GetToken(hashToken(token))
	if !found || t.IsRefresh || !t.ExpiresAt.After(s.now()) {
		return "", "", false
	}
	return t.Subject, t.ClientID, true
}

// now returns the current time, honouring opts.Now when set.
func (s *Server) now() time.Time {
	if s.opts.Now != nil {
		return s.opts.Now()
	}
	return time.Now()
}

// logf logs a formatted diagnostic message, if a logger is configured.
func (s *Server) logf(format string, args ...any) {
	if s.opts.Logger != nil {
		s.opts.Logger(format, args...)
	}
}
