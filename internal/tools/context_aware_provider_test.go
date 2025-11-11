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
)

// TestNewContextAwareProvider tests provider creation
func TestNewContextAwareProvider(t *testing.T) {
	clientManager := database.NewClientManager()
	// nil no longer needed
	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:    "Test Server",
		Company: "Test Co",
		Version: "1.0.0",
	}

	provider := NewContextAwareProvider(clientManager, nil, true, fallbackClient, serverInfo, nil, nil, nil, "", nil)

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
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	clientManager := database.NewClientManager()
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:    "Test Server",
		Company: "Test Co",
		Version: "1.0.0",
	}

	provider := NewContextAwareProvider(clientManager, nil, true, fallbackClient, serverInfo, nil, nil, nil, "", nil)

	// Register tools
	err := provider.RegisterTools(context.TODO())
	if err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	// List tools
	tools := provider.List()

	// Should have 10 tools registered
	expectedTools := []string{
		"query_database",
		"get_schema_info",
		"set_pg_configuration",
		"server_info",
		"set_database_connection",
		"read_resource",
		"add_database_connection",
		"remove_database_connection",
		"list_database_connections",
		"edit_database_connection",
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

	// nil no longer needed
	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:    "Test Server",
		Company: "Test Co",
		Version: "1.0.0",
	}

	// Auth disabled - should use fallback client
	provider := NewContextAwareProvider(clientManager, nil, false, fallbackClient, serverInfo, nil, nil, nil, "", nil)

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
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	clientManager := database.NewClientManager()
	defer clientManager.CloseAll()

	// nil no longer needed
	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:    "Test Server",
		Company: "Test Co",
		Version: "1.0.0",
	}

	// Auth enabled - should require token hash
	provider := NewContextAwareProvider(clientManager, nil, true, fallbackClient, serverInfo, nil, nil, nil, "", nil)

	t.Run("missing token hash returns error", func(t *testing.T) {
		// Context without token hash
		ctx := context.Background()

		// Execute server_info (even though it doesn't need DB, context validation happens first)
		_, err := provider.Execute(ctx, "server_info", map[string]interface{}{})
		if err == nil {
			t.Fatal("Expected error for missing token hash, got nil")
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

		// Note: server_info is a stateless tool, so no client should be created
		if count := clientManager.GetClientCount(); count != 0 {
			t.Errorf("Expected 0 clients for stateless tool, got %d", count)
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

		// Third token
		ctx3 := context.WithValue(context.Background(), auth.TokenHashContextKey, "token-hash-3")
		_, err = provider.Execute(ctx3, "server_info", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Execute failed for token 3: %v", err)
		}

		// Note: server_info is a stateless tool, so no clients should be created
		if count := clientManager.GetClientCount(); count != 0 {
			t.Errorf("Expected 0 clients for stateless tool, got %d", count)
		}
	})
}

// TestContextAwareProvider_Execute_InvalidTool tests execution of non-existent tool
func TestContextAwareProvider_Execute_InvalidTool(t *testing.T) {
	clientManager := database.NewClientManager()
	defer clientManager.CloseAll()

	// nil no longer needed
	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:    "Test Server",
		Company: "Test Co",
		Version: "1.0.0",
	}

	// Auth disabled for simplicity
	provider := NewContextAwareProvider(clientManager, nil, false, fallbackClient, serverInfo, nil, nil, nil, "", nil)

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
	if os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING") == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set, skipping database test")
	}

	clientManager := database.NewClientManager()
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient()
	serverInfo := ServerInfo{
		Name:    "Test Server",
		Company: "Test Co",
		Version: "1.0.0",
	}

	provider := NewContextAwareProvider(clientManager, nil, true, fallbackClient, serverInfo, nil, nil, nil, "", nil)

	// Register with context containing token hash
	ctx := context.WithValue(context.Background(), auth.TokenHashContextKey, "registration-token")

	err := provider.RegisterTools(ctx)
	if err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	// Note: RegisterTools doesn't create clients - clients are created on-demand
	// when Execute() is called with database-dependent tools
	if count := clientManager.GetClientCount(); count != 0 {
		t.Errorf("Expected 0 clients after registration (clients created on-demand), got %d", count)
	}

	// Verify tools are registered in base registry
	tools := provider.List()
	if len(tools) == 0 {
		t.Error("Expected tools to be registered")
	}
}
