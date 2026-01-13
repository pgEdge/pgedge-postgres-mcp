/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/mcp"
)

// AuthenticateUserTool creates a tool for user authentication
// This tool is NOT advertised to the LLM - it's only for direct client calls
func AuthenticateUserTool(userStore *auth.UserStore, rateLimiter *auth.RateLimiter, maxFailedAttempts int) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "authenticate_user",
			Description: "Authenticates a user and returns a session token for subsequent API calls. This tool is for direct client use only and is not advertised to the LLM.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"username": map[string]interface{}{
						"type":        "string",
						"description": "The username to authenticate",
					},
					"password": map[string]interface{}{
						"type":        "string",
						"description": "The user's password",
					},
				},
				Required: []string{"username", "password"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Extract context from args (injected by registry)
			var ctx context.Context
			if ctxRaw, ok := args["__context"]; ok {
				if ctxVal, ok := ctxRaw.(context.Context); ok {
					ctx = ctxVal
				}
			}
			if ctx == nil {
				ctx = context.Background()
			}

			// Extract username
			usernameRaw, ok := args["username"]
			if !ok {
				return mcp.ToolResponse{}, fmt.Errorf("username is required")
			}
			username, ok := usernameRaw.(string)
			if !ok || username == "" {
				return mcp.ToolResponse{}, fmt.Errorf("username must be a non-empty string")
			}

			// Extract password
			passwordRaw, ok := args["password"]
			if !ok {
				return mcp.ToolResponse{}, fmt.Errorf("password is required")
			}
			password, ok := passwordRaw.(string)
			if !ok || password == "" {
				return mcp.ToolResponse{}, fmt.Errorf("password must be a non-empty string")
			}

			// Check if user store is available
			if userStore == nil {
				return mcp.ToolResponse{}, fmt.Errorf("user authentication is not configured")
			}

			// Get IP address from context for rate limiting
			ipAddress := auth.GetIPAddressFromContext(ctx)

			// Check rate limit if rate limiter is configured
			if rateLimiter != nil && ipAddress != "" {
				if !rateLimiter.IsAllowed(ipAddress) {
					return mcp.ToolResponse{}, fmt.Errorf("too many failed authentication attempts from this IP address, please try again later")
				}
			}

			// Authenticate user
			token, expiration, err := userStore.AuthenticateUser(username, password, maxFailedAttempts)
			if err != nil {
				// Record failed attempt for rate limiting
				if rateLimiter != nil && ipAddress != "" {
					rateLimiter.RecordFailedAttempt(ipAddress)
				}
				return mcp.ToolResponse{}, fmt.Errorf("authentication failed: %w", err)
			}

			// Reset rate limit on successful authentication
			if rateLimiter != nil && ipAddress != "" {
				rateLimiter.Reset(ipAddress)
			}

			// Return session token and expiration as JSON in the text response
			// The client will parse this to extract the token
			response := map[string]interface{}{
				"success":       true,
				"session_token": token,
				"expires_at":    expiration.Format("2006-01-02T15:04:05Z07:00"),
				"message":       "Authentication successful",
			}
			responseBytes, err := json.Marshal(response)
			if err != nil {
				return mcp.ToolResponse{}, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.ToolResponse{
				Content: []mcp.ContentItem{
					{
						Type: "text",
						Text: string(responseBytes),
					},
				},
			}, nil
		},
	}
}
