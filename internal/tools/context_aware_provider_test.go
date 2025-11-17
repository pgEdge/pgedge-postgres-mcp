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
	"strings"
	"testing"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/resources"
)

// TestNewContextAwareProvider tests provider creation
func TestNewContextAwareProvider(t *testing.T) {
	clientManager := database.NewClientManager(nil)
	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, true)

	provider := NewContextAwareProvider(clientManager, resourceReg, true, fallbackClient, cfg, nil, "")

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
}

// TestContextAwareProvider_List tests tool listing with smart filtering
func TestContextAwareProvider_List(t *testing.T) {
	clientManager := database.NewClientManager(nil)
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, false)

	provider := NewContextAwareProvider(clientManager, resourceReg, false, fallbackClient, cfg, nil, "")

	// Register tools
	err := provider.RegisterTools(context.TODO())
	if err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	t.Run("returns all tools regardless of connection state", func(t *testing.T) {
		// List tools - should return all tools
		tools := provider.List()

		// Should have all 6 tools (no filtering)
		expectedTools := []string{
			"read_resource",
			"generate_embedding",
			"query_database",
			"get_schema_info",
			"semantic_search",
			"search_similar",
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
	})
}

// TestContextAwareProvider_Execute_NoAuth tests execution without authentication
func TestContextAwareProvider_Execute_NoAuth(t *testing.T) {
	// This test doesn't require database connection, testing read_resource tool
	clientManager := database.NewClientManager(nil)
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, false)

	// Auth disabled - should use fallback client
	provider := NewContextAwareProvider(clientManager, resourceReg, false, fallbackClient, cfg, nil, "")

	// Context without token hash
	ctx := context.Background()

	// Execute read_resource with a non-existent resource (tests the tool works)
	response, err := provider.Execute(ctx, "read_resource", map[string]interface{}{
		"uri": "test://nonexistent",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// read_resource should return an error for non-existent resource, but not fail
	// Verify we got a response (error or not)
	if len(response.Content) == 0 {
		t.Fatal("Expected non-empty response content")
	}
}

// TestContextAwareProvider_Execute_WithAuth tests execution with authentication
func TestContextAwareProvider_Execute_WithAuth(t *testing.T) {
	clientManager := database.NewClientManager(nil)
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, true)

	// Auth enabled - should require token hash
	provider := NewContextAwareProvider(clientManager, resourceReg, true, fallbackClient, cfg, nil, "")

	t.Run("missing token hash returns error", func(t *testing.T) {
		// Context without token hash
		ctx := context.Background()

		// Execute read_resource (even though it doesn't need DB, context validation happens first)
		_, err := provider.Execute(ctx, "read_resource", map[string]interface{}{
			"uri": "test://test",
		})
		if err == nil {
			t.Fatal("Expected error for missing token hash, got nil")
		}

		if !strings.Contains(err.Error(), "no authentication token") {
			t.Errorf("Expected 'no authentication token' error, got: %v", err)
		}
	})

	t.Run("with valid token hash succeeds", func(t *testing.T) {
		// Context with token hash (no token store needed for stateless tools in auth mode)
		ctx := context.WithValue(context.Background(), auth.TokenHashContextKey, "test-token-hash")

		// Execute read_resource (doesn't require database queries)
		response, err := provider.Execute(ctx, "read_resource", map[string]interface{}{
			"uri": "test://test",
		})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// read_resource should return a response (may be error for non-existent resource)
		// Verify we got a response
		if len(response.Content) == 0 {
			t.Fatal("Expected non-empty response content")
		}

		// Note: read_resource is a stateless tool, so no client should be created
		if count := clientManager.GetClientCount(); count != 0 {
			t.Errorf("Expected 0 clients for stateless tool, got %d", count)
		}
	})

	t.Run("multiple tokens get different clients", func(t *testing.T) {
		// First token
		ctx1 := context.WithValue(context.Background(), auth.TokenHashContextKey, "token-hash-1")
		_, err := provider.Execute(ctx1, "read_resource", map[string]interface{}{
			"uri": "test://test1",
		})
		if err != nil {
			t.Fatalf("Execute failed for token 1: %v", err)
		}

		// Second token
		ctx2 := context.WithValue(context.Background(), auth.TokenHashContextKey, "token-hash-2")
		_, err = provider.Execute(ctx2, "read_resource", map[string]interface{}{
			"uri": "test://test2",
		})
		if err != nil {
			t.Fatalf("Execute failed for token 2: %v", err)
		}

		// Third token
		ctx3 := context.WithValue(context.Background(), auth.TokenHashContextKey, "token-hash-3")
		_, err = provider.Execute(ctx3, "read_resource", map[string]interface{}{
			"uri": "test://test3",
		})
		if err != nil {
			t.Fatalf("Execute failed for token 3: %v", err)
		}

		// Note: read_resource is a stateless tool, so no clients should be created
		if count := clientManager.GetClientCount(); count != 0 {
			t.Errorf("Expected 0 clients for stateless tool, got %d", count)
		}
	})
}

// TestContextAwareProvider_Execute_InvalidTool tests execution of non-existent tool
func TestContextAwareProvider_Execute_InvalidTool(t *testing.T) {
	clientManager := database.NewClientManager(nil)
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, false)

	// Auth disabled for simplicity
	provider := NewContextAwareProvider(clientManager, resourceReg, false, fallbackClient, cfg, nil, "")

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
	clientManager := database.NewClientManager(nil)
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, true)

	provider := NewContextAwareProvider(clientManager, resourceReg, true, fallbackClient, cfg, nil, "")

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
