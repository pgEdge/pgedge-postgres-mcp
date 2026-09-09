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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// TokenHashContextKey is the context key for storing the authenticated token hash
	TokenHashContextKey contextKey = "token_hash"

	// IPAddressContextKey is the context key for storing the client IP address
	IPAddressContextKey contextKey = "ip_address"

	// UsernameContextKey is the context key for storing the session username
	UsernameContextKey contextKey = "username"

	// IsAPITokenContextKey is the context key for indicating if auth was via API token
	IsAPITokenContextKey contextKey = "is_api_token"

	// HealthCheckPath is the path for the health check endpoint (bypasses authentication)
	HealthCheckPath = "/health"

	// UserInfoPath is the path for the user info endpoint (bypasses auth to return auth status)
	UserInfoPath = "/api/user/info"

	// OpenAPIPath is the path for the OpenAPI specification endpoint (public for API discoverability)
	OpenAPIPath = "/api/openapi.json"
)

// GetTokenHashFromContext retrieves the token hash from the request context
// Returns empty string if no token hash is found (e.g., unauthenticated request)
func GetTokenHashFromContext(ctx context.Context) string {
	if hash, ok := ctx.Value(TokenHashContextKey).(string); ok {
		return hash
	}
	return ""
}

// GetIPAddressFromContext retrieves the client IP address from the request context
// Returns empty string if no IP address is found
func GetIPAddressFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(IPAddressContextKey).(string); ok {
		return ip
	}
	return ""
}

// GetUsernameFromContext retrieves the session username from the request context
// Returns empty string if no username is found (e.g., API token or unauthenticated request)
func GetUsernameFromContext(ctx context.Context) string {
	if username, ok := ctx.Value(UsernameContextKey).(string); ok {
		return username
	}
	return ""
}

// IsAPITokenFromContext checks if the request was authenticated via API token
// Returns false if not set (e.g., session token or unauthenticated request)
func IsAPITokenFromContext(ctx context.Context) bool {
	if isAPIToken, ok := ctx.Value(IsAPITokenContextKey).(bool); ok {
		return isAPIToken
	}
	return false
}

// JSONErrorResponse represents a JSON error response for API endpoints
type JSONErrorResponse struct {
	Error string `json:"error"`
}

// writeJSONError writes a JSON error response with the given status code and message
func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	//nolint:errcheck // Error would only occur if connection is closed
	json.NewEncoder(w).Encode(JSONErrorResponse{Error: message})
}

// ExtractIPAddress returns the address of the peer that opened the connection,
// ignoring forwarding headers entirely.
//
// Forwarding headers are deliberately not consulted here: a caller can set them
// for itself, and the reverse proxy configurations we document append to
// X-Forwarded-For rather than replacing it, so its leftmost entry is whatever
// the caller chose to send. Deployments behind a proxy opt into reading a header
// through http.client_ip, which honours it only from a trusted peer; see
// ClientIPResolver.
func ExtractIPAddress(r *http.Request) string {
	return (*ClientIPResolver)(nil).Resolve(r)
}

// AuthMiddleware creates an HTTP middleware that validates API tokens,
// session tokens and OAuth access tokens through v, subject to the
// credential kinds v.Methods enables.
func AuthMiddleware(v *Validator, enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication if disabled
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip authentication for public endpoints (needed before login)
			for _, p := range v.PublicPaths() {
				if r.URL.Path == p {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check if this is an authenticate_user tool call (which should
			// bypass auth, but only when password login is enabled)
			if v.Methods.PasswordLogin && isAuthenticateUserCall(r) {
				next.ServeHTTP(w, r)
				return
			}

			unauthorized := func(message string) {
				if v.OAuthEnabled() {
					w.Header().Set("WWW-Authenticate", v.ChallengeHeader())
				}
				writeJSONError(w, message, http.StatusUnauthorized)
			}

			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				unauthorized("Missing Authorization header")
				return
			}

			// Parse Bearer token
			token, ok := ParseBearer(authHeader)
			if !ok {
				unauthorized("Invalid Authorization header format. Expected: Bearer <token>")
				return
			}

			id, err := v.Validate(token)
			if err != nil {
				unauthorized("Invalid or unknown token")
				return
			}

			r = r.WithContext(v.ContextWithIdentity(r.Context(), id))
			next.ServeHTTP(w, r)
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

	// Read the body with a size limit to prevent memory exhaustion.
	// The limit matches MaxRequestBodySize (10MB) from the HTTP server.
	const maxBodySize = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
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
