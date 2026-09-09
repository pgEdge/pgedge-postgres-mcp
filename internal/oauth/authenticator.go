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
	"context"
	"errors"

	"pgedge-postgres-mcp/internal/auth"
)

// ErrInvalidCredentials is returned by Authenticate when the supplied
// username or password does not match a known user.
var ErrInvalidCredentials = errors.New("oauth: invalid credentials")

// ErrRateLimited is returned by Authenticate when the caller's client IP
// has exceeded the configured rate of failed login attempts.
var ErrRateLimited = errors.New("oauth: rate limited")

// Authenticator verifies a resource owner's credentials during the login
// page flow, returning the subject identifier on success.
//
// Task 4 provides UserStoreAuthenticator, the concrete implementation
// backed by the server's configured user store.
type Authenticator interface {
	Authenticate(ctx context.Context, username, password, clientIP string) (string, error)
}

// UserStoreAuthenticator implements Authenticator against the server's
// in-memory user store, mirroring the behaviour of the authenticate_user
// MCP tool: a configured RateLimiter is checked before consulting the
// store, and failed attempts are recorded against the client IP so that
// repeated bad logins are throttled.
type UserStoreAuthenticator struct {
	// Users is the user store to authenticate against.
	Users *auth.UserStore

	// RateLimiter throttles failed login attempts per client IP. It is
	// optional; when nil, or when clientIP is empty, no rate limiting
	// is applied.
	RateLimiter *auth.RateLimiter

	// MaxFailedAttempts is passed through to Users.AuthenticateUser; if
	// greater than zero the account is disabled after that many
	// consecutive failed attempts.
	MaxFailedAttempts int
}

// Authenticate verifies username and password against the user store,
// returning the username as the subject on success.
func (a *UserStoreAuthenticator) Authenticate(ctx context.Context, username, password, clientIP string) (string, error) {
	if a.RateLimiter != nil && clientIP != "" {
		if !a.RateLimiter.IsAllowed(clientIP) {
			return "", ErrRateLimited
		}
	}

	_, _, err := a.Users.AuthenticateUser(username, password, a.MaxFailedAttempts)
	if err != nil {
		if a.RateLimiter != nil && clientIP != "" {
			a.RateLimiter.RecordFailedAttempt(clientIP)
		}
		return "", ErrInvalidCredentials
	}

	if a.RateLimiter != nil && clientIP != "" {
		a.RateLimiter.Reset(clientIP)
	}

	return username, nil
}
