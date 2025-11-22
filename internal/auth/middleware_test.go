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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestAuthMiddleware_Disabled tests that middleware is bypassed when disabled
func TestAuthMiddleware_Disabled(t *testing.T) {
	tokenStore := &TokenStore{
		Tokens: make(map[string]*Token),
	}

	middleware := AuthMiddleware(tokenStore, nil, false)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}

	if body := rr.Body.String(); body != "success" {
		t.Errorf("Expected 'success', got %q", body)
	}
}

// TestAuthMiddleware_HealthCheck tests that health check endpoint bypasses auth
func TestAuthMiddleware_HealthCheck(t *testing.T) {
	tokenStore := &TokenStore{
		Tokens: make(map[string]*Token),
	}

	middleware := AuthMiddleware(tokenStore, nil, true)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("healthy"))
	}))

	req := httptest.NewRequest("GET", HealthCheckPath, nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK for health check, got %d", rr.Code)
	}

	if body := rr.Body.String(); body != "healthy" {
		t.Errorf("Expected 'healthy', got %q", body)
	}
}

// TestAuthMiddleware_MissingAuthHeader tests rejection of requests without Authorization header
func TestAuthMiddleware_MissingAuthHeader(t *testing.T) {
	tokenStore := &TokenStore{
		Tokens: make(map[string]*Token),
	}

	middleware := AuthMiddleware(tokenStore, nil, true)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for missing auth header")
	}))

	req := httptest.NewRequest("POST", "/mcp/v1", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status Unauthorized, got %d", rr.Code)
	}

	body := strings.TrimSpace(rr.Body.String())
	if !strings.Contains(body, "Missing Authorization header") {
		t.Errorf("Expected 'Missing Authorization header' in response, got %q", body)
	}
}

// TestAuthMiddleware_MalformedAuthHeader tests rejection of malformed Authorization headers
func TestAuthMiddleware_MalformedAuthHeader(t *testing.T) {
	testCases := []struct {
		name          string
		header        string
		expectFormat  bool // true if expecting "Invalid Authorization header format"
		expectInvalid bool // true if expecting "Invalid or unknown token"
	}{
		{"no bearer prefix", "sometoken123", true, false},
		{"wrong prefix", "Basic sometoken123", true, false},
		{"missing token", "Bearer", true, false},
		{"empty token", "Bearer ", false, true}, // Empty token passes format check but fails validation
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokenStore := &TokenStore{
				Tokens: make(map[string]*Token),
			}

			middleware := AuthMiddleware(tokenStore, nil, true)

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("Handler should not be called for malformed auth header")
			}))

			req := httptest.NewRequest("POST", "/mcp/v1", nil)
			req.Header.Set("Authorization", tc.header)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("Expected status Unauthorized, got %d", rr.Code)
			}

			body := strings.TrimSpace(rr.Body.String())
			if tc.expectFormat && !strings.Contains(body, "Invalid Authorization header format") {
				t.Errorf("Expected 'Invalid Authorization header format' in response, got %q", body)
			}
			if tc.expectInvalid && !strings.Contains(body, "Invalid or unknown token") {
				t.Errorf("Expected 'Invalid or unknown token' in response, got %q", body)
			}
		})
	}
}

// TestAuthMiddleware_InvalidToken tests rejection of invalid tokens
func TestAuthMiddleware_InvalidToken(t *testing.T) {
	tokenStore := &TokenStore{
		Tokens: make(map[string]*Token),
	}

	middleware := AuthMiddleware(tokenStore, nil, true)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for invalid token")
	}))

	req := httptest.NewRequest("POST", "/mcp/v1", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-12345")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status Unauthorized, got %d", rr.Code)
	}

	body := strings.TrimSpace(rr.Body.String())
	if !strings.Contains(body, "Invalid or unknown token") {
		t.Errorf("Expected 'Invalid or unknown token' in response, got %q", body)
	}
}

// TestAuthMiddleware_ValidToken tests successful authentication with valid token
func TestAuthMiddleware_ValidToken(t *testing.T) {
	// Generate a valid token string
	tokenString, err := GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create Token struct with hash
	tokenHash := HashToken(tokenString)
	tokenStruct := &Token{
		Hash:       tokenHash,
		ExpiresAt:  nil, // Never expires
		Annotation: "Test token",
		CreatedAt:  time.Now(),
	}

	tokenStore := &TokenStore{
		Tokens: map[string]*Token{
			"test-token-id": tokenStruct,
		},
	}

	middleware := AuthMiddleware(tokenStore, nil, true)

	var capturedContext context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedContext = r.Context()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("authenticated"))
	}))

	req := httptest.NewRequest("POST", "/mcp/v1", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK for valid token, got %d", rr.Code)
	}

	if body := rr.Body.String(); body != "authenticated" {
		t.Errorf("Expected 'authenticated', got %q", body)
	}

	// Verify token hash was stored in context
	ctxHash := GetTokenHashFromContext(capturedContext)
	if ctxHash == "" {
		t.Error("Expected token hash in context, got empty string")
	}

	if ctxHash != tokenHash {
		t.Errorf("Expected token hash %q, got %q", tokenHash, ctxHash)
	}
}

// TestAuthMiddleware_ExpiredToken tests rejection of expired tokens
func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	// Generate a token string
	tokenString, err := GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create Token struct that expires very soon
	expiryTime := time.Now().Add(1 * time.Millisecond)
	tokenHash := HashToken(tokenString)
	tokenStruct := &Token{
		Hash:       tokenHash,
		ExpiresAt:  &expiryTime,
		Annotation: "Test expired token",
		CreatedAt:  time.Now(),
	}

	tokenStore := &TokenStore{
		Tokens: map[string]*Token{
			"test-expired-id": tokenStruct,
		},
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	middleware := AuthMiddleware(tokenStore, nil, true)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for expired token")
	}))

	req := httptest.NewRequest("POST", "/mcp/v1", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status Unauthorized for expired token, got %d", rr.Code)
	}

	body := strings.TrimSpace(rr.Body.String())
	// Should get generic "Invalid or unknown token" message, not internal error details
	if !strings.Contains(body, "Invalid or unknown token") {
		t.Errorf("Expected 'Invalid or unknown token' in response, got %q", body)
	}

	// Verify no internal error details leaked
	if strings.Contains(body, "expired") {
		t.Errorf("Internal error details leaked in response: %q", body)
	}
}

// TestGetTokenHashFromContext tests retrieving token hash from context
func TestGetTokenHashFromContext(t *testing.T) {
	t.Run("with token hash", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), TokenHashContextKey, "test-hash-123")
		hash := GetTokenHashFromContext(ctx)
		if hash != "test-hash-123" {
			t.Errorf("Expected 'test-hash-123', got %q", hash)
		}
	})

	t.Run("without token hash", func(t *testing.T) {
		ctx := context.Background()
		hash := GetTokenHashFromContext(ctx)
		if hash != "" {
			t.Errorf("Expected empty string for missing token hash, got %q", hash)
		}
	})

	t.Run("with wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), TokenHashContextKey, 12345)
		hash := GetTokenHashFromContext(ctx)
		if hash != "" {
			t.Errorf("Expected empty string for wrong type, got %q", hash)
		}
	})
}

// TestAuthMiddleware_NoInfoLeak tests that internal errors don't leak to clients
func TestAuthMiddleware_NoInfoLeak(t *testing.T) {
	// Generate a token string
	tokenString, err := GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create a token struct with an invalid/corrupted hash (simulate internal error)
	tokenStruct := &Token{
		Hash:       "invalid-hash-format", // This will not match the actual token hash
		Annotation: "Test token",
		CreatedAt:  time.Now(),
		ExpiresAt:  nil,
	}

	tokenStore := &TokenStore{
		Tokens: map[string]*Token{
			"test-id": tokenStruct,
		},
	}

	middleware := AuthMiddleware(tokenStore, nil, true)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for token validation error")
	}))

	req := httptest.NewRequest("POST", "/mcp/v1", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status Unauthorized, got %d", rr.Code)
	}

	body := strings.TrimSpace(rr.Body.String())
	// Should get generic error, not internal details
	if body != "Invalid or unknown token" {
		t.Errorf("Expected generic error message, got %q (info leak?)", body)
	}

	// Verify no stack traces, hash details, or other internals
	if strings.Contains(body, "hash") || strings.Contains(body, "corrupt") || strings.Contains(body, "format") {
		t.Errorf("Internal error details leaked: %q", body)
	}
}
