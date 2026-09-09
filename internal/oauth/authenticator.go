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
	"sync"

	"golang.org/x/crypto/bcrypt"

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

// dummyHash returns a bcrypt hash of an unguessable value, generated
// once at the same cost the user store uses. Comparing against it makes
// an unknown or disabled username cost the same as a real password
// check, so the response time does not tell a caller which usernames
// exist. It is generated lazily rather than at package initialisation,
// since a cost-12 hash takes a noticeable fraction of a second.
var dummyHash = sync.OnceValue(func() []byte {
	secret, err := randomToken(32)
	if err != nil {
		// Nothing here is secret-bearing: the value is never compared
		// against anything a caller supplies successfully.
		secret = "dummy password for constant-time comparison"
	}
	h, err := bcrypt.GenerateFromPassword([]byte(secret), auth.BcryptCost)
	if err != nil {
		return nil
	}
	return h
})

// equaliseFailedLogin spends the time a real password check would have
// cost, for a username the store rejected without reaching bcrypt.
func equaliseFailedLogin(password string) {
	if h := dummyHash(); h != nil {
		_ = bcrypt.CompareHashAndPassword(h, []byte(password))
	}
}

// Authenticate verifies username and password against the user store,
// returning the username as the subject on success.
func (a *UserStoreAuthenticator) Authenticate(ctx context.Context, username, password, clientIP string) (string, error) {
	if a.RateLimiter != nil && clientIP != "" {
		if !a.RateLimiter.IsAllowed(clientIP) {
			return "", ErrRateLimited
		}
	}

	// Whether the store will run a bcrypt comparison has to be settled
	// before the attempt, since a failed one disables an account at the
	// lockout threshold and so changes the answer.
	verified := a.Users.CanVerifyPassword(username)

	_, _, err := a.Users.AuthenticateUser(username, password, a.MaxFailedAttempts)
	if err != nil {
		if !verified {
			equaliseFailedLogin(password)
		}
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
