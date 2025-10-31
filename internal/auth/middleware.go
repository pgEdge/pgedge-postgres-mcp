/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// TokenHashContextKey is the context key for storing the authenticated token hash
	TokenHashContextKey contextKey = "token_hash"

	// HealthCheckPath is the path for the health check endpoint (bypasses authentication)
	HealthCheckPath = "/health"
)

// GetTokenHashFromContext retrieves the token hash from the request context
// Returns empty string if no token hash is found (e.g., unauthenticated request)
func GetTokenHashFromContext(ctx context.Context) string {
	if hash, ok := ctx.Value(TokenHashContextKey).(string); ok {
		return hash
	}
	return ""
}

// AuthMiddleware creates an HTTP middleware that validates API tokens
func AuthMiddleware(tokenStore *TokenStore, enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication if disabled
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip authentication for health check endpoint
			if r.URL.Path == HealthCheckPath {
				next.ServeHTTP(w, r)
				return
			}

			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
				return
			}

			// Parse Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid Authorization header format. Expected: Bearer <token>", http.StatusUnauthorized)
				return
			}

			token := parts[1]

			// Validate token
			valid, err := tokenStore.ValidateToken(token)
			if err != nil {
				// Log detailed error for debugging
				_, _ = fmt.Fprintf(os.Stderr, "Token validation error: %v\n", err)
				// Return generic error to client (don't leak internal details)
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			if !valid {
				http.Error(w, "Invalid or unknown token", http.StatusUnauthorized)
				return
			}

			// Hash the token to use as connection pool key
			tokenHash := HashToken(token)

			// Store token hash in context for per-token connection isolation
			ctx := context.WithValue(r.Context(), TokenHashContextKey, tokenHash)
			r = r.WithContext(ctx)

			// Token is valid, proceed with request
			next.ServeHTTP(w, r)
		})
	}
}
