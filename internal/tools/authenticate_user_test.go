/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/auth"
)

func TestAuthenticateUserTool_Definition(t *testing.T) {
	tool := AuthenticateUserTool(nil, nil, 5)

	if tool.Definition.Name != "authenticate_user" {
		t.Errorf("expected name 'authenticate_user', got %q", tool.Definition.Name)
	}

	if tool.Definition.Description == "" {
		t.Error("expected non-empty description")
	}

	// Check required parameters
	if len(tool.Definition.InputSchema.Required) != 2 {
		t.Errorf("expected 2 required parameters, got %d", len(tool.Definition.InputSchema.Required))
	}

	requiredParams := make(map[string]bool)
	for _, param := range tool.Definition.InputSchema.Required {
		requiredParams[param] = true
	}

	if !requiredParams["username"] {
		t.Error("expected 'username' to be required")
	}
	if !requiredParams["password"] {
		t.Error("expected 'password' to be required")
	}
}

func TestAuthenticateUserTool_MissingUserStore(t *testing.T) {
	// Create tool without user store
	tool := AuthenticateUserTool(nil, nil, 5)

	args := map[string]interface{}{
		"username": "testuser",
		"password": "testpass",
	}

	_, err := tool.Handler(args)
	if err == nil {
		t.Error("expected error when user store is nil")
	}

	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got: %v", err)
	}
}

func TestAuthenticateUserTool_MissingUsername(t *testing.T) {
	userStore := auth.InitializeUserStore()
	tool := AuthenticateUserTool(userStore, nil, 5)

	args := map[string]interface{}{
		"password": "testpass",
	}

	_, err := tool.Handler(args)
	if err == nil {
		t.Error("expected error when username is missing")
	}

	if !strings.Contains(err.Error(), "username is required") {
		t.Errorf("expected 'username is required' error, got: %v", err)
	}
}

func TestAuthenticateUserTool_MissingPassword(t *testing.T) {
	userStore := auth.InitializeUserStore()
	tool := AuthenticateUserTool(userStore, nil, 5)

	args := map[string]interface{}{
		"username": "testuser",
	}

	_, err := tool.Handler(args)
	if err == nil {
		t.Error("expected error when password is missing")
	}

	if !strings.Contains(err.Error(), "password is required") {
		t.Errorf("expected 'password is required' error, got: %v", err)
	}
}

func TestAuthenticateUserTool_EmptyUsername(t *testing.T) {
	userStore := auth.InitializeUserStore()
	tool := AuthenticateUserTool(userStore, nil, 5)

	args := map[string]interface{}{
		"username": "",
		"password": "testpass",
	}

	_, err := tool.Handler(args)
	if err == nil {
		t.Error("expected error when username is empty")
	}

	if !strings.Contains(err.Error(), "non-empty string") {
		t.Errorf("expected 'non-empty string' error, got: %v", err)
	}
}

func TestAuthenticateUserTool_EmptyPassword(t *testing.T) {
	userStore := auth.InitializeUserStore()
	tool := AuthenticateUserTool(userStore, nil, 5)

	args := map[string]interface{}{
		"username": "testuser",
		"password": "",
	}

	_, err := tool.Handler(args)
	if err == nil {
		t.Error("expected error when password is empty")
	}

	if !strings.Contains(err.Error(), "non-empty string") {
		t.Errorf("expected 'non-empty string' error, got: %v", err)
	}
}

func TestAuthenticateUserTool_InvalidUsernameType(t *testing.T) {
	userStore := auth.InitializeUserStore()
	tool := AuthenticateUserTool(userStore, nil, 5)

	args := map[string]interface{}{
		"username": 123, // Invalid type
		"password": "testpass",
	}

	_, err := tool.Handler(args)
	if err == nil {
		t.Error("expected error when username is wrong type")
	}
}

func TestAuthenticateUserTool_InvalidPasswordType(t *testing.T) {
	userStore := auth.InitializeUserStore()
	tool := AuthenticateUserTool(userStore, nil, 5)

	args := map[string]interface{}{
		"username": "testuser",
		"password": 123, // Invalid type
	}

	_, err := tool.Handler(args)
	if err == nil {
		t.Error("expected error when password is wrong type")
	}
}

func TestAuthenticateUserTool_AuthenticationFailed(t *testing.T) {
	userStore := auth.InitializeUserStore()
	// Add a user with known password
	err := userStore.AddUser("testuser", "correctpassword", "test user")
	if err != nil {
		t.Fatalf("failed to add user: %v", err)
	}

	tool := AuthenticateUserTool(userStore, nil, 5)

	args := map[string]interface{}{
		"username": "testuser",
		"password": "wrongpassword",
	}

	_, err = tool.Handler(args)
	if err == nil {
		t.Error("expected error for wrong password")
	}

	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected 'authentication failed' error, got: %v", err)
	}
}

func TestAuthenticateUserTool_Success(t *testing.T) {
	userStore := auth.InitializeUserStore()
	// Add a user with known password
	err := userStore.AddUser("testuser", "correctpassword", "test user")
	if err != nil {
		t.Fatalf("failed to add user: %v", err)
	}

	tool := AuthenticateUserTool(userStore, nil, 5)

	args := map[string]interface{}{
		"username": "testuser",
		"password": "correctpassword",
	}

	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.IsError {
		t.Error("expected success response")
	}

	if len(response.Content) == 0 {
		t.Fatal("expected response content")
	}

	// Parse the JSON response
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response.Content[0].Text), &result); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if result["success"] != true {
		t.Error("expected success=true in response")
	}

	if result["session_token"] == nil || result["session_token"] == "" {
		t.Error("expected non-empty session_token in response")
	}

	if result["message"] != "Authentication successful" {
		t.Errorf("expected 'Authentication successful' message, got %v", result["message"])
	}
}

func TestAuthenticateUserTool_WithRateLimiter(t *testing.T) {
	userStore := auth.InitializeUserStore()
	rateLimiter := auth.NewRateLimiter(5, 60) // 5 attempts per 60 seconds

	// Add a user
	err := userStore.AddUser("testuser", "correctpassword", "test user")
	if err != nil {
		t.Fatalf("failed to add user: %v", err)
	}

	tool := AuthenticateUserTool(userStore, rateLimiter, 5)

	// Create context with IP address
	ctx := context.WithValue(context.Background(), auth.IPAddressContextKey, "192.168.1.1")

	args := map[string]interface{}{
		"__context": ctx,
		"username":  "testuser",
		"password":  "correctpassword",
	}

	// First attempt should succeed
	response, err := tool.Handler(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response.IsError {
		t.Error("expected success response")
	}
}

func TestAuthenticateUserTool_NonexistentUser(t *testing.T) {
	userStore := auth.InitializeUserStore()
	tool := AuthenticateUserTool(userStore, nil, 5)

	args := map[string]interface{}{
		"username": "nonexistent",
		"password": "somepassword",
	}

	_, err := tool.Handler(args)
	if err == nil {
		t.Error("expected error for non-existent user")
	}

	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected 'authentication failed' error, got: %v", err)
	}
}
