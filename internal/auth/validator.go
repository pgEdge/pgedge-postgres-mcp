/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// IdentityKind identifies which credential type authenticated a request.
type IdentityKind string

const (
	// IdentityAPIToken marks a request authenticated with an API token.
	IdentityAPIToken IdentityKind = "api"

	// IdentitySession marks a request authenticated with a username/password
	// session token.
	IdentitySession IdentityKind = "session"

	// IdentityOAuth marks a request authenticated with an OAuth 2.0 access
	// token.
	IdentityOAuth IdentityKind = "oauth"
)

// Identity describes the credential that authenticated a request.
type Identity struct {
	Kind      IdentityKind
	TokenHash string
	Username  string // empty for API tokens
}

// OAuthValidator validates OAuth 2.0 access tokens. It is satisfied by
// *oauth.Server; it is defined here, rather than imported from
// internal/oauth, to avoid an import cycle between the two packages.
type OAuthValidator interface {
	ValidateAccessToken(token string) (subject, clientID string, ok bool)
	Issuer() string
}

// Methods toggles which credential kinds Validator.Validate accepts.
type Methods struct {
	APITokens     bool
	PasswordLogin bool
	OAuth         bool
}

// Validator checks a bearer token against API tokens, session tokens and
// OAuth access tokens, subject to the toggles in Methods.
type Validator struct {
	Tokens *TokenStore    // may be nil
	Users  *UserStore     // may be nil
	OAuth  OAuthValidator // may be nil

	Methods Methods

	// ExtraPublicPaths lists additional request paths that bypass
	// authentication entirely. main.go populates this from
	// oauth.PublicPaths(), since internal/auth cannot import
	// internal/oauth without an import cycle.
	ExtraPublicPaths []string
}

// ErrInvalidToken is returned by Validate when a token does not match any
// enabled credential kind.
var ErrInvalidToken = errors.New("invalid or unknown token")

// Validate checks token against each enabled credential kind, in order: API
// token, session token, then OAuth access token.
func (v *Validator) Validate(token string) (Identity, error) {
	if v.Methods.APITokens && v.Tokens != nil {
		if ok, err := v.Tokens.ValidateToken(token); err == nil && ok {
			return Identity{Kind: IdentityAPIToken, TokenHash: HashToken(token)}, nil
		}
	}

	if v.Methods.PasswordLogin && v.Users != nil {
		if username, err := v.Users.ValidateSessionToken(token); err == nil && username != "" {
			return Identity{Kind: IdentitySession, TokenHash: HashToken(token), Username: username}, nil
		}
	}

	if v.OAuthEnabled() {
		if subject, _, ok := v.OAuth.ValidateAccessToken(token); ok {
			return Identity{Kind: IdentityOAuth, TokenHash: HashToken(token), Username: subject}, nil
		}
	}

	return Identity{}, ErrInvalidToken
}

// ContextWithIdentity sets the existing TokenHashContextKey,
// UsernameContextKey and IsAPITokenContextKey context values from id.
func (v *Validator) ContextWithIdentity(ctx context.Context, id Identity) context.Context {
	ctx = context.WithValue(ctx, TokenHashContextKey, id.TokenHash)
	if id.Username != "" {
		ctx = context.WithValue(ctx, UsernameContextKey, id.Username)
	}
	ctx = context.WithValue(ctx, IsAPITokenContextKey, id.Kind == IdentityAPIToken)
	return ctx
}

// PublicPaths returns the request paths that bypass authentication: the
// built-in health, user info and OpenAPI endpoints, plus any paths OAuth
// registered via ExtraPublicPaths.
func (v *Validator) PublicPaths() []string {
	paths := []string{HealthCheckPath, UserInfoPath, OpenAPIPath}
	if v.OAuthEnabled() {
		paths = append(paths, v.ExtraPublicPaths...)
	}
	return paths
}

// OAuthEnabled reports whether OAuth access tokens are accepted.
func (v *Validator) OAuthEnabled() bool {
	return v.Methods.OAuth && v.OAuth != nil
}

// ChallengeHeader returns the WWW-Authenticate header value advertising the
// OAuth protected resource metadata endpoint, for use on 401 responses when
// OAuthEnabled returns true.
func (v *Validator) ChallengeHeader() string {
	return fmt.Sprintf("Bearer resource_metadata=%q", v.OAuth.Issuer()+"/.well-known/oauth-protected-resource")
}

// ParseBearer extracts the token from an Authorization header value of the
// form "Bearer <token>".
func ParseBearer(header string) (string, bool) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", false
	}
	return parts[1], true
}
