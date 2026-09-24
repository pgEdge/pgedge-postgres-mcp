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

	// AnonymousRateLimiter meters the unauthenticated endpoints that
	// create entries in the store, namely dynamic client registration
	// and the device authorisation request. It has the same limits and
	// client-IP semantics as RateLimiter but a budget of its own,
	// because these endpoints are metered rather than guarded: every
	// request counts, successful or not, so sharing RateLimiter's
	// budget would let a handful of anonymous requests lock every user
	// out of password login, the authenticate_user tool and
	// /api/user/info. May be nil, in which case neither endpoint is
	// throttled (as in most tests).
	AnonymousRateLimiter *auth.RateLimiter

	// SubjectActive reports whether an authenticated subject (a
	// username in the user store) is still allowed to hold OAuth
	// access: it must return false for a user that has been disabled,
	// deleted or locked out. It is consulted on every access token
	// validation and on every refresh, so that revoking a user account
	// takes effect at once rather than when the token happens to
	// expire. May be nil, in which case no such check is made; main
	// always sets it when a user store is configured.
	SubjectActive func(username string) bool

	// OnTokensRevoked, when set, is called with the hashes of access
	// tokens that have just been expired or revoked, whether by a
	// sweep, a revocation request, a family revocation or a subject
	// revocation. main uses it to release the per-token database
	// connection pools keyed on those hashes.
	OnTokensRevoked func(accessTokenHashes []string)

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
	page      *loginPage
	stopSweep func()
}

// New creates a Server from opts. It validates that an issuer has been
// configured, builds the in-memory store and CSRF signer, and starts the
// background sweeper that expires stale entries.
func New(opts Options) (*Server, error) {
	if opts.Config.Issuer == "" {
		return nil, errors.New("oauth: issuer must not be empty")
	}
	// The login form handler calls Authenticator unconditionally, so a
	// nil one is a programming error that would otherwise only show up
	// as a panic on the first sign-in attempt.
	if opts.Authenticator == nil {
		return nil, errors.New("oauth: an authenticator must be provided")
	}

	signer, err := newCSRFSigner()
	if err != nil {
		return nil, err
	}

	page, err := newLoginPage(opts.Config.LoginPage)
	if err != nil {
		return nil, err
	}

	s := &Server{
		opts:  opts,
		store: NewStore(DefaultLimits, opts.Config.RefreshTokenLifetime),
		csrf:  signer,
		page:  page,
	}
	// Installed before the sweeper starts, so that the very first sweep
	// already reports the tokens it expires.
	s.store.SetRevocationHook(opts.OnTokensRevoked)
	s.stopSweep = s.store.StartSweeper(time.Minute)
	return s, nil
}

// RegisterRoutes registers the authorisation server's HTTP handlers on
// mux: metadata, protected resource, dynamic client registration,
// authorisation, token, device and revocation endpoints.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(MetadataPath, s.handleMetadata)
	mux.HandleFunc(ProtectedResourcePath, s.handleProtectedResourceMetadata)
	mux.HandleFunc(RegisterPath, s.handleRegister)
	mux.HandleFunc(AuthorizePath, s.handleAuthorize)
	mux.HandleFunc(TokenPath, s.handleToken)
	mux.HandleFunc(DevicePath, s.handleDevice)
	mux.HandleFunc(DeviceVerifyPath, s.handleDeviceVerify)
	mux.HandleFunc(RevokePath, s.handleRevoke)
	mux.HandleFunc(LogoPath, s.page.ServeLogo)
	mux.HandleFunc(FaviconPath, s.page.ServeFavicon)
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
// A token whose subject is no longer an active user is refused, and the
// rest of that subject's tokens are revoked with it, so that disabling
// or deleting an account ends its OAuth access immediately.
func (s *Server) ValidateAccessToken(token string) (subject, clientID string, ok bool) {
	t, found := s.store.GetToken(hashToken(token))
	if !found || t.IsRefresh || !t.ExpiresAt.After(s.now()) {
		return "", "", false
	}
	if !s.subjectActive(t.Subject) {
		s.logf("oauth: subject=%q is no longer active, revoking its tokens", t.Subject)
		s.RevokeSubject(t.Subject)
		return "", "", false
	}
	return t.Subject, t.ClientID, true
}

// subjectActive reports whether subject may still hold OAuth access,
// consulting Options.SubjectActive when one is configured. With no
// check configured (as in most tests, and in a deployment with no user
// store) every subject is treated as active; with one configured, an
// unknown or disabled subject fails closed.
func (s *Server) subjectActive(subject string) bool {
	if s.opts.SubjectActive == nil {
		return true
	}
	return s.opts.SubjectActive(subject)
}

// RevokeSubject revokes every OAuth token, authorisation code and
// device code standing in subject's name, returning the hashes of the
// access tokens revoked. It is called when a user account is disabled,
// deleted or locked out, and whenever a token is presented by a subject
// that is no longer active.
func (s *Server) RevokeSubject(subject string) []string {
	return s.store.DeleteBySubject(subject)
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
