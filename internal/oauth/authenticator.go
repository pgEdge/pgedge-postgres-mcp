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
