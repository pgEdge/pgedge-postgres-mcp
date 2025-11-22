/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
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

// AuthMiddleware creates an HTTP middleware that validates API tokens and session tokens
func AuthMiddleware(tokenStore *TokenStore, userStore *UserStore, enabled bool) func(http.Handler) http.Handler {
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

			// Check if this is an authenticate_user tool call (which should bypass auth)
			if isAuthenticateUserCall(r) {
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

			// Try to validate as API token first
			validAPIToken, err := tokenStore.ValidateToken(token)
			if err == nil && validAPIToken {
				// Valid API token - use token hash for connection isolation
				tokenHash := HashToken(token)
				ctx := context.WithValue(r.Context(), TokenHashContextKey, tokenHash)
				r = r.WithContext(ctx)
				next.ServeHTTP(w, r)
				return
			}

			// Try to validate as session token if userStore is available
			if userStore != nil {
				username, err := userStore.ValidateSessionToken(token)
				if err == nil && username != "" {
					// Valid session token - use token hash for connection isolation
					tokenHash := HashToken(token)
					ctx := context.WithValue(r.Context(), TokenHashContextKey, tokenHash)
					r = r.WithContext(ctx)
					next.ServeHTTP(w, r)
					return
				}
			}

			// Neither API token nor session token is valid
			http.Error(w, "Invalid or unknown token", http.StatusUnauthorized)
		})
	}
}

// isAuthenticateUserCall checks if the request is a tools/call for authenticate_user
// This function reads and restores the request body
func isAuthenticateUserCall(r *http.Request) bool {
	// Defensive nil check for request
	if r == nil {
		return false
	}

	// Defensive nil check for request body
	if r.Body == nil {
		return false
	}

	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	defer func() {
		// Restore the body for the next handler
		r.Body = io.NopCloser(bytes.NewBuffer(body))
	}()

	// Parse as JSON-RPC request
	var req struct {
		Method string                 `json:"method"`
		Params map[string]interface{} `json:"params"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}

	// Check if it's a tools/call for authenticate_user
	if req.Method == "tools/call" {
		if name, ok := req.Params["name"].(string); ok {
			return name == "authenticate_user"
		}
	}

	return false
}
