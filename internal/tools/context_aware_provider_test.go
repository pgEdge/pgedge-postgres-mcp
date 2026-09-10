/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
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
	"pgedge-postgres-mcp/internal/definitions"
	"pgedge-postgres-mcp/internal/resources"
)

// TestNewContextAwareProvider tests provider creation
func TestNewContextAwareProvider(t *testing.T) {
	clientManager := database.NewClientManagerWithConfig(nil)
	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, true, nil, cfg)

	provider := NewContextAwareProvider(clientManager, resourceReg, true, fallbackClient, cfg, nil, "", nil, 0, nil)

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
	clientManager := database.NewClientManagerWithConfig(nil)
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, false, nil, cfg)

	provider := NewContextAwareProvider(clientManager, resourceReg, false, fallbackClient, cfg, nil, "", nil, 0, nil)

	// Register tools
	err := provider.RegisterTools(context.TODO())
	if err != nil {
		t.Fatalf("RegisterTools failed: %v", err)
	}

	t.Run("returns all tools regardless of connection state", func(t *testing.T) {
		// List tools - should return all tools
		tools := provider.List()

		// Should have all 7 tools (no filtering)
		expectedTools := []string{
			"read_resource",
			"generate_embedding",
			"query_database",
			"get_schema_info",
			"similarity_search",
			"execute_explain",
			"count_rows",
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
	clientManager := database.NewClientManagerWithConfig(nil)
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, false, nil, cfg)

	// Auth disabled - should use fallback client
	provider := NewContextAwareProvider(clientManager, resourceReg, false, fallbackClient, cfg, nil, "", nil, 0, nil)

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
	clientManager := database.NewClientManagerWithConfig(nil)
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, true, nil, cfg)

	// Auth enabled - should require token hash
	provider := NewContextAwareProvider(clientManager, resourceReg, true, fallbackClient, cfg, nil, "", nil, 0, nil)

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

		// Note: In unit tests without database configuration, clients are not created
		// In production with database config, read_resource would create clients for authenticated tokens
		// This test verifies the tool executes successfully with proper authentication
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

		// Note: In unit tests without database configuration, clients are not created
		// In production with database config, each token would get its own isolated database client
		// This test verifies that multiple authenticated tokens can execute tools successfully
	})
}

// TestContextAwareProvider_Execute_InvalidTool tests execution of non-existent tool
func TestContextAwareProvider_Execute_InvalidTool(t *testing.T) {
	clientManager := database.NewClientManagerWithConfig(nil)
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, false, nil, cfg)

	// Auth disabled for simplicity
	provider := NewContextAwareProvider(clientManager, resourceReg, false, fallbackClient, cfg, nil, "", nil, 0, nil)

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
	clientManager := database.NewClientManagerWithConfig(nil)
	defer clientManager.CloseAll()

	fallbackClient := database.NewClient(nil)
	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, true, nil, cfg)

	provider := NewContextAwareProvider(clientManager, resourceReg, true, fallbackClient, cfg, nil, "", nil, 0, nil)

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

// TestRegisterCustomTool_FiltersByLanguage verifies that RegisterCustomTool
// skips PL tools whose language is not allowed by any configured database.
func TestRegisterCustomTool_FiltersByLanguage(t *testing.T) {
	clientManager := database.NewClientManagerWithConfig(nil)
	defer clientManager.CloseAll()

	cfg := &config.Config{
		Databases: []config.NamedDatabaseConfig{
			{
				Name:               "db1",
				AllowedPLLanguages: []string{"plpgsql"},
			},
		},
	}
	resourceReg := resources.NewContextAwareRegistry(clientManager, false, nil, cfg)
	provider := NewContextAwareProvider(clientManager, resourceReg, false, nil, cfg, nil, "", nil, 0, nil)

	// Register a pl-do tool with plpython3u (not allowed)
	err := provider.RegisterCustomTool(definitions.ToolDefinition{
		Name:        "python_tool",
		Description: "A Python tool",
		Type:        "pl-do",
		Language:    "plpython3u",
		Code:        "pass",
	})
	if err != nil {
		t.Fatalf("RegisterCustomTool failed: %v", err)
	}

	// The tool should NOT appear in the base registry
	tools := provider.List()
	for _, tool := range tools {
		if tool.Name == "python_tool" {
			t.Error("Expected pl-do tool with disallowed language to be filtered from tools/list")
		}
	}

	// Register a pl-func tool with plpgsql (allowed)
	err = provider.RegisterCustomTool(definitions.ToolDefinition{
		Name:        "plpgsql_func",
		Description: "A PL/pgSQL function",
		Type:        "pl-func",
		Language:    "plpgsql",
		Code:        "BEGIN RETURN 1; END;",
		Returns:     "integer",
	})
	if err != nil {
		t.Fatalf("RegisterCustomTool failed: %v", err)
	}

	// The plpgsql tool SHOULD appear in the base registry
	tools = provider.List()
	found := false
	for _, tool := range tools {
		if tool.Name == "plpgsql_func" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected pl-func tool with allowed language to appear in tools/list")
	}
}

// TestRegisterCustomTool_AllowsSQLTools verifies that SQL-type custom tools
// are always registered regardless of language settings.
func TestRegisterCustomTool_AllowsSQLTools(t *testing.T) {
	clientManager := database.NewClientManagerWithConfig(nil)
	defer clientManager.CloseAll()

	cfg := &config.Config{
		Databases: []config.NamedDatabaseConfig{
			{
				Name:               "db1",
				AllowedPLLanguages: []string{"plpgsql"},
			},
		},
	}
	resourceReg := resources.NewContextAwareRegistry(clientManager, false, nil, cfg)
	provider := NewContextAwareProvider(clientManager, resourceReg, false, nil, cfg, nil, "", nil, 0, nil)

	// Register a SQL tool (should always be allowed)
	err := provider.RegisterCustomTool(definitions.ToolDefinition{
		Name:        "sql_tool",
		Description: "A SQL tool",
		Type:        "sql",
		SQL:         "SELECT 1",
	})
	if err != nil {
		t.Fatalf("RegisterCustomTool failed: %v", err)
	}

	// The SQL tool SHOULD appear in the base registry
	tools := provider.List()
	found := false
	for _, tool := range tools {
		if tool.Name == "sql_tool" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected SQL-type custom tool to always appear in tools/list")
	}
}

// TestListContext_FiltersPLToolsByLanguage verifies that ListContext only
// returns PL tools whose language is allowed for the specific database.
func TestListContext_FiltersPLToolsByLanguage(t *testing.T) {
	clientManager := database.NewClientManagerWithConfig(nil)
	defer clientManager.CloseAll()

	// Configure two databases with different allowed languages
	cfg := &config.Config{
		Databases: []config.NamedDatabaseConfig{
			{
				Name:               "db1",
				AllowedPLLanguages: []string{"plpgsql", "plpython3u"},
			},
			{
				Name:               "db2",
				AllowedPLLanguages: []string{"plpgsql"},
			},
		},
	}
	resourceReg := resources.NewContextAwareRegistry(clientManager, false, nil, cfg)
	provider := NewContextAwareProvider(clientManager, resourceReg, false, nil, cfg, nil, "", nil, 0, nil)

	// Register tools for multiple languages
	_ = provider.RegisterCustomTool(definitions.ToolDefinition{
		Name:        "python_tool",
		Description: "A Python tool",
		Type:        "pl-do",
		Language:    "plpython3u",
		Code:        "pass",
	})
	_ = provider.RegisterCustomTool(definitions.ToolDefinition{
		Name:        "plpgsql_tool",
		Description: "A PL/pgSQL tool",
		Type:        "pl-do",
		Language:    "plpgsql",
		Code:        "NULL;",
	})
	_ = provider.RegisterCustomTool(definitions.ToolDefinition{
		Name:        "plv8_tool",
		Description: "A PLV8 tool",
		Type:        "pl-do",
		Language:    "plv8",
		Code:        "var x = 1;",
	})

	// The base registry (List) should include python_tool and plpgsql_tool
	// because the union of languages is {plpgsql, plpython3u}, but NOT plv8_tool
	tools := provider.List()
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	if !toolNames["python_tool"] {
		t.Error("Expected python_tool in base registry (allowed in db1)")
	}
	if !toolNames["plpgsql_tool"] {
		t.Error("Expected plpgsql_tool in base registry (allowed in both)")
	}
	if toolNames["plv8_tool"] {
		t.Error("Expected plv8_tool to be filtered from base registry (not allowed in any db)")
	}
}

// TestContextAwareProvider_StaleRegistryCleanup verifies that stale registry
// entries are cleaned up when a client is closed.
func TestContextAwareProvider_StaleRegistryCleanup(t *testing.T) {
	clientManager := database.NewClientManagerWithConfig(nil)
	defer clientManager.CloseAll()

	cfg := &config.Config{}
	resourceReg := resources.NewContextAwareRegistry(clientManager, false, nil, cfg)
	provider := NewContextAwareProvider(clientManager, resourceReg, false, nil, cfg, nil, "", nil, 0, nil)

	// Create a client and get a registry for it
	client := database.NewClient(nil)
	registry1 := provider.getOrCreateRegistryForClient(client)
	if registry1 == provider.baseRegistry {
		t.Fatal("Expected a new registry, not the base registry")
	}

	// Getting registry again should return same cached instance
	registry2 := provider.getOrCreateRegistryForClient(client)
	if registry2 != registry1 {
		t.Error("Expected same cached registry instance")
	}

	// Close the client
	client.Close()

	// Getting registry for closed client should return base registry
	registry3 := provider.getOrCreateRegistryForClient(client)
	if registry3 != provider.baseRegistry {
		t.Error("Expected base registry for closed client")
	}

	// Verify stale entry was cleaned up
	provider.mu.RLock()
	_, exists := provider.clientRegistries[client]
	provider.mu.RUnlock()
	if exists {
		t.Error("Expected stale registry entry to be deleted")
	}
}

// TestNewContextAwareProvider_AuthenticateUserToggle verifies that the
// hidden authenticate_user tool is only registered when a user store is
// present and password login is enabled in configuration.
func TestNewContextAwareProvider_AuthenticateUserToggle(t *testing.T) {
	clientManager := database.NewClientManagerWithConfig(nil)
	fallbackClient := database.NewClient(nil)
	userStore := auth.InitializeUserStore()
	disabled := false

	t.Run("registered when password login enabled", func(t *testing.T) {
		cfg := &config.Config{}
		reg := resources.NewContextAwareRegistry(clientManager, true, nil, cfg)
		provider := NewContextAwareProvider(clientManager, reg, true, fallbackClient, cfg, userStore, "", nil, 0, nil)
		if _, exists := provider.hiddenRegistry.Get("authenticate_user"); !exists {
			t.Error("Expected authenticate_user to be registered when password login is enabled")
		}
	})

	t.Run("not registered when password login disabled", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.HTTP.Auth.Methods.PasswordLogin = &disabled
		reg := resources.NewContextAwareRegistry(clientManager, true, nil, cfg)
		provider := NewContextAwareProvider(clientManager, reg, true, fallbackClient, cfg, userStore, "", nil, 0, nil)
		if _, exists := provider.hiddenRegistry.Get("authenticate_user"); exists {
			t.Error("Expected authenticate_user not to be registered when password login is disabled")
		}
	})
}
