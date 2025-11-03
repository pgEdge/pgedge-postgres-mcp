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
	"fmt"
	"sync"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/llm"
	"pgedge-postgres-mcp/internal/mcp"
	"pgedge-postgres-mcp/internal/resources"
)

// ContextAwareProvider wraps a tool registry and provides per-token database clients
// This ensures connection isolation in HTTP/HTTPS mode with authentication
type ContextAwareProvider struct {
	baseRegistry   *Registry                    // Registry for tool definitions (List operation)
	clientManager  *database.ClientManager
	llmClient      *llm.Client
	resourceReg    *resources.ContextAwareRegistry
	authEnabled    bool
	fallbackClient *database.Client             // Used when auth is disabled
	serverInfo     ServerInfo                   // Server metadata for server_info tool

	// Cache of registries per client to avoid re-creating tools on every Execute()
	mu             sync.RWMutex
	clientRegistries map[*database.Client]*Registry
}

// NewContextAwareProvider creates a new context-aware tool provider
func NewContextAwareProvider(clientManager *database.ClientManager, llmClient *llm.Client, resourceReg *resources.ContextAwareRegistry, authEnabled bool, fallbackClient *database.Client, serverInfo ServerInfo) *ContextAwareProvider {
	provider := &ContextAwareProvider{
		baseRegistry:     NewRegistry(),
		clientManager:    clientManager,
		llmClient:        llmClient,
		resourceReg:      resourceReg,
		authEnabled:      authEnabled,
		fallbackClient:   fallbackClient,
		serverInfo:       serverInfo,
		clientRegistries: make(map[*database.Client]*Registry),
	}

	// Register ALL tools in base registry so they're always visible in tools/list
	// Database-dependent tools will fail gracefully in Execute() if no connection exists
	// This provides better UX - users can discover all tools even before connecting

	// Stateless tools
	provider.baseRegistry.Register("recommend_pg_configuration", RecommendPGConfigurationTool())
	provider.baseRegistry.Register("server_info", ServerInfoTool(serverInfo))
	provider.baseRegistry.Register("set_database_connection", SetDatabaseConnectionTool(clientManager))
	// Note: read_resource tool provides backward compatibility for resource access
	// Resources are also accessible via the native MCP resources/read endpoint
	provider.baseRegistry.Register("read_resource", ReadResourceTool(provider.createResourceAdapter()))

	// Database-dependent tools (registered with nil client placeholders)
	// These will use the actual client from ClientManager when Execute() is called
	provider.baseRegistry.Register("query_database", QueryDatabaseTool(nil, llmClient))
	provider.baseRegistry.Register("get_schema_info", GetSchemaInfoTool(nil))
	provider.baseRegistry.Register("set_pg_configuration", SetPGConfigurationTool(nil))
	provider.baseRegistry.Register("analyze_bloat", AnalyzeBloatTool(nil))
	provider.baseRegistry.Register("read_server_log", ReadServerLogTool(nil))
	provider.baseRegistry.Register("read_postgresql_conf", ReadPostgresqlConfTool(nil))
	provider.baseRegistry.Register("read_pg_hba_conf", ReadPgHbaConfTool(nil))
	provider.baseRegistry.Register("read_pg_ident_conf", ReadPgIdentConfTool(nil))

	return provider
}

// resourceReaderAdapter adapts ContextAwareRegistry to the ResourceReader interface
// This provides backward compatibility for the read_resource tool
type resourceReaderAdapter struct {
	registry *resources.ContextAwareRegistry
}

func (a *resourceReaderAdapter) List() []mcp.Resource {
	return a.registry.List()
}

func (a *resourceReaderAdapter) Read(uri string) (mcp.ResourceContent, error) {
	// Use background context for backward compatibility
	// The ContextAwareRegistry will get the client from the default key
	return a.registry.Read(context.Background(), uri)
}

// createResourceAdapter creates an adapter for the resource registry
func (p *ContextAwareProvider) createResourceAdapter() ResourceReader {
	return &resourceReaderAdapter{
		registry: p.resourceReg,
	}
}

// GetBaseRegistry returns the base registry for adding additional tools
func (p *ContextAwareProvider) GetBaseRegistry() *Registry {
	return p.baseRegistry
}

// RegisterTools initializes tool registrations
// This is called at startup to ensure the base registry is populated for List() operations
func (p *ContextAwareProvider) RegisterTools(ctx context.Context) error {
	// Pre-create a registry for the fallback client if auth is disabled and fallback exists
	// This ensures tools are ready for immediate use
	if !p.authEnabled && p.fallbackClient != nil {
		_ = p.getOrCreateRegistryForClient(p.fallbackClient)
	}
	return nil
}

// List returns all registered tool definitions
// All tools are registered in the base registry for discovery
// Database-dependent tools will fail gracefully if no connection exists
func (p *ContextAwareProvider) List() []mcp.Tool {
	return p.baseRegistry.List()
}

// getOrCreateRegistryForClient returns a cached registry for the given client
// or creates a new one if it doesn't exist
func (p *ContextAwareProvider) getOrCreateRegistryForClient(client *database.Client) *Registry {
	if client == nil {
		// No client available - return base registry only
		return p.baseRegistry
	}

	// Fast path: check if registry already exists (read lock)
	p.mu.RLock()
	if registry, exists := p.clientRegistries[client]; exists {
		p.mu.RUnlock()
		return registry
	}
	p.mu.RUnlock()

	// Slow path: create new registry (write lock)
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if registry, exists := p.clientRegistries[client]; exists {
		return registry
	}

	// Create new registry with all tools for this client
	registry := NewRegistry()

	// Register stateless tools
	registry.Register("recommend_pg_configuration", RecommendPGConfigurationTool())
	registry.Register("server_info", ServerInfoTool(p.serverInfo))
	registry.Register("set_database_connection", SetDatabaseConnectionTool(p.clientManager))
	// Note: Resources are accessed via resources/read MCP endpoint, not via tools

	// Register client-dependent tools
	registry.Register("query_database", QueryDatabaseTool(client, p.llmClient))
	registry.Register("get_schema_info", GetSchemaInfoTool(client))
	registry.Register("set_pg_configuration", SetPGConfigurationTool(client))
	registry.Register("analyze_bloat", AnalyzeBloatTool(client))
	registry.Register("read_server_log", ReadServerLogTool(client))
	registry.Register("read_postgresql_conf", ReadPostgresqlConfTool(client))
	registry.Register("read_pg_hba_conf", ReadPgHbaConfTool(client))
	registry.Register("read_pg_ident_conf", ReadPgIdentConfTool(client))

	// Cache for future use
	p.clientRegistries[client] = registry

	return registry
}

// Execute runs a tool by name with the given arguments and context
// Uses cached per-client registries to avoid re-creating tools on every request
func (p *ContextAwareProvider) Execute(ctx context.Context, name string, args map[string]interface{}) (mcp.ToolResponse, error) {
	// Check if this is a stateless tool that doesn't require a database client
	statelessTools := map[string]bool{
		"recommend_pg_configuration": true,
		"read_resource":              true,
		"server_info":                true,
		"set_database_connection":    true,
	}

	if statelessTools[name] {
		// Execute from base registry (no database client needed)
		return p.baseRegistry.Execute(ctx, name, args)
	}

	// Get the appropriate database client for this request
	dbClient, err := p.getClient(ctx)
	if err != nil {
		return mcp.ToolResponse{
			Content: []mcp.ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to get database client: %v\nPlease call set_database_connection first to configure the database connection.", err),
				},
			},
			IsError: true,
		}, nil // Don't return error, just error response
	}

	// Get the cached registry for this client (or create if first use)
	// This avoids re-creating all tools on every request
	registry := p.getOrCreateRegistryForClient(dbClient)

	// Execute the tool using the client-specific registry
	return registry.Execute(ctx, name, args)
}

// getClient returns the appropriate database client based on authentication state
func (p *ContextAwareProvider) getClient(ctx context.Context) (*database.Client, error) {
	if !p.authEnabled {
		// Authentication disabled - use "default" key in ClientManager
		// Don't auto-connect - user must call set_database_connection first
		client, err := p.clientManager.GetOrCreateClient("default", false)
		if err != nil {
			return nil, fmt.Errorf("no database connection configured: %w", err)
		}
		return client, nil
	}

	// Authentication enabled - get per-token client
	tokenHash := auth.GetTokenHashFromContext(ctx)
	if tokenHash == "" {
		return nil, fmt.Errorf("no authentication token found in request context")
	}

	// Get or create client for this token (don't auto-connect)
	client, err := p.clientManager.GetOrCreateClient(tokenHash, false)
	if err != nil {
		return nil, fmt.Errorf("no database connection configured for this token: %w", err)
	}

	return client, nil
}
