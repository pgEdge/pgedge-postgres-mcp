/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"context"
	"os"
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/llm"
)

// TestNewContextAwareProvider tests provider creation
func TestNewContextAwareProvider(t *testing.T) {
	clientManager := database.NewClientManager()
	llmClient := llm.NewClient("anthropic", "test-key", "https://api.anthropic.com/v1", "claude-sonnet-4-5")
	// nil no longer needed
	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:     "Test Server",
		Company:  "Test Co",
		Version:  "1.0.0",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-5",
	}

	provider := NewContextAwareProvider(clientManager, llmClient, nil, true, fallbackClient, serverInfo)

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	if provider.baseRegistry == nil {
		t.Error("Expected baseRegistry to be initialized")
	}

	if provider.clientManager != clientManager {
		t.Error("Expected clientManager to be set correctly")
	}

	if provider.authEnabled != true {
		t.Error("Expected authEnabled to be true")
	}

	if provider.serverInfo.Name != "Test Server" {
		t.Error("Expected serverInfo to be set correctly")
	}
}

// TestContextAwareProvider_List tests tool listing
func TestContextAwareProvider_List(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	clientManager := database.NewClientManager()
	defer clientManager.CloseAll()

	llmClient := llm.NewClient("anthropic", "test-key", "https://api.anthropic.com/v1", "claude-sonnet-4-5")
	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:     "Test Server",
		Company:  "Test Co",
		Version:  "1.0.0",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-5",
	}

	provider := NewContextAwareProvider(clientManager, llmClient, nil, true, fallbackClient, serverInfo)

	// Register tools
	err := provider.RegisterTools(nil)
	if err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	// List tools
	tools := provider.List()

	// Should have 11 tools registered
	expectedTools := []string{
		"query_database",
		"get_schema_info",
		"set_pg_configuration",
		"recommend_pg_configuration",
		"analyze_bloat",
		"read_server_log",
		"read_postgresql_conf",
		"read_pg_hba_conf",
		"read_pg_ident_conf",
		"read_resource",
		"server_info",
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(tools))
	}

	// Check that all expected tools are present
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, expectedName := range expectedTools {
		if !toolNames[expectedName] {
			t.Errorf("Expected tool %q not found in list", expectedName)
		}
	}

	// Verify server_info is included
	if !toolNames["server_info"] {
		t.Error("server_info tool should be registered")
	}
}

// TestContextAwareProvider_Execute_NoAuth tests execution without authentication
func TestContextAwareProvider_Execute_NoAuth(t *testing.T) {
	// This test doesn't require database connection, testing server_info tool
	clientManager := database.NewClientManager()
	defer clientManager.CloseAll()

	llmClient := llm.NewClient("anthropic", "test-key", "https://api.anthropic.com/v1", "claude-sonnet-4-5")
	// nil no longer needed
	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:     "Test Server",
		Company:  "Test Co",
		Version:  "1.0.0",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-5",
	}

	// Auth disabled - should use fallback client
	provider := NewContextAwareProvider(clientManager, llmClient, nil, false, fallbackClient, serverInfo)

	// Context without token hash
	ctx := context.Background()

	// Execute server_info (doesn't require database)
	response, err := provider.Execute(ctx, "server_info", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if response.IsError {
		t.Error("Expected successful response")
	}

	// Verify response contains server info
	if len(response.Content) == 0 {
		t.Fatal("Expected non-empty response content")
	}

	output := response.Content[0].Text
	if !strings.Contains(output, "Test Server") {
		t.Error("Expected output to contain server name")
	}
}

// TestContextAwareProvider_Execute_WithAuth tests execution with authentication
func TestContextAwareProvider_Execute_WithAuth(t *testing.T) {
	// Skip if no database connection available (needed for client creation)
	if os.Getenv("POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	clientManager := database.NewClientManager()
	defer clientManager.CloseAll()

	llmClient := llm.NewClient("anthropic", "test-key", "https://api.anthropic.com/v1", "claude-sonnet-4-5")
	// nil no longer needed
	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:     "Test Server",
		Company:  "Test Co",
		Version:  "1.0.0",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-5",
	}

	// Auth enabled - should require token hash
	provider := NewContextAwareProvider(clientManager, llmClient, nil, true, fallbackClient, serverInfo)

	t.Run("missing token hash returns error", func(t *testing.T) {
		// Context without token hash
		ctx := context.Background()

		// Execute server_info (even though it doesn't need DB, context validation happens first)
		_, err := provider.Execute(ctx, "server_info", map[string]interface{}{})
		if err == nil {
			t.Error("Expected error for missing token hash")
		}

		if !strings.Contains(err.Error(), "no authentication token") {
			t.Errorf("Expected 'no authentication token' error, got: %v", err)
		}
	})

	t.Run("with valid token hash succeeds", func(t *testing.T) {
		// Context with token hash
		ctx := context.WithValue(context.Background(), auth.TokenHashContextKey, "test-token-hash-123")

		// Execute server_info (doesn't require database queries)
		response, err := provider.Execute(ctx, "server_info", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if response.IsError {
			t.Error("Expected successful response")
		}

		// Verify response contains server info
		if len(response.Content) == 0 {
			t.Fatal("Expected non-empty response content")
		}

		output := response.Content[0].Text
		if !strings.Contains(output, "Test Server") {
			t.Error("Expected output to contain server name")
		}

		// Verify a client was created for this token
		if count := clientManager.GetClientCount(); count != 1 {
			t.Errorf("Expected 1 client to be created, got %d", count)
		}
	})

	t.Run("multiple tokens get different clients", func(t *testing.T) {
		// First token
		ctx1 := context.WithValue(context.Background(), auth.TokenHashContextKey, "token-hash-1")
		_, err := provider.Execute(ctx1, "server_info", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Execute failed for token 1: %v", err)
		}

		// Second token
		ctx2 := context.WithValue(context.Background(), auth.TokenHashContextKey, "token-hash-2")
		_, err = provider.Execute(ctx2, "server_info", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Execute failed for token 2: %v", err)
		}

		// Should have 3 clients now (test-token-hash-123, token-hash-1, token-hash-2)
		if count := clientManager.GetClientCount(); count != 3 {
			t.Errorf("Expected 3 clients for different tokens, got %d", count)
		}
	})
}

// TestContextAwareProvider_Execute_InvalidTool tests execution of non-existent tool
func TestContextAwareProvider_Execute_InvalidTool(t *testing.T) {
	clientManager := database.NewClientManager()
	defer clientManager.CloseAll()

	llmClient := llm.NewClient("anthropic", "test-key", "https://api.anthropic.com/v1", "claude-sonnet-4-5")
	// nil no longer needed
	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:     "Test Server",
		Company:  "Test Co",
		Version:  "1.0.0",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-5",
	}

	// Auth disabled for simplicity
	provider := NewContextAwareProvider(clientManager, llmClient, nil, false, fallbackClient, serverInfo)

	ctx := context.Background()

	// Execute non-existent tool
	response, err := provider.Execute(ctx, "nonexistent_tool", map[string]interface{}{})
	if err != nil {
		t.Errorf("Expected nil error (error in response), got: %v", err)
	}

	if !response.IsError {
		t.Error("Expected error response for non-existent tool")
	}

	// Verify error message
	if len(response.Content) == 0 {
		t.Fatal("Expected error message in response")
	}

	errorMsg := response.Content[0].Text
	// With runtime database connection, we now get a "no database connection" error
	// for non-stateless tools when database isn't configured
	if !strings.Contains(errorMsg, "no database connection configured") && !strings.Contains(errorMsg, "Tool not found") {
		t.Errorf("Expected 'no database connection configured' or 'Tool not found' error, got: %s", errorMsg)
	}
}

// TestContextAwareProvider_RegisterTools_WithContext tests registering with context
func TestContextAwareProvider_RegisterTools_WithContext(t *testing.T) {
	// Skip if no database connection available
	if os.Getenv("POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	clientManager := database.NewClientManager()
	defer clientManager.CloseAll()

	llmClient := llm.NewClient("anthropic", "test-key", "https://api.anthropic.com/v1", "claude-sonnet-4-5")
	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:     "Test Server",
		Company:  "Test Co",
		Version:  "1.0.0",
		Provider: "anthropic",
		Model:    "claude-sonnet-4-5",
	}

	provider := NewContextAwareProvider(clientManager, llmClient, nil, true, fallbackClient, serverInfo)

	// Register with context containing token hash
	ctx := context.WithValue(context.Background(), auth.TokenHashContextKey, "registration-token")

	err := provider.RegisterTools(ctx)
	if err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	// Verify a client was created for this token
	if count := clientManager.GetClientCount(); count != 1 {
		t.Errorf("Expected 1 client after registration, got %d", count)
	}

	// Verify tools are registered
	tools := provider.List()
	if len(tools) == 0 {
		t.Error("Expected tools to be registered")
	}
}
