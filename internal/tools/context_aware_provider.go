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
	resourceReg    *resources.Registry
	authEnabled    bool
	fallbackClient *database.Client             // Used when auth is disabled
	serverInfo     ServerInfo                   // Server metadata for server_info tool

	// Cache of registries per client to avoid re-creating tools on every Execute()
	mu             sync.RWMutex
	clientRegistries map[*database.Client]*Registry
}

// NewContextAwareProvider creates a new context-aware tool provider
func NewContextAwareProvider(clientManager *database.ClientManager, llmClient *llm.Client, resourceReg *resources.Registry, authEnabled bool, fallbackClient *database.Client, serverInfo ServerInfo) *ContextAwareProvider {
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

	// Register stateless tools in base registry (these don't depend on DB client)
	provider.baseRegistry.Register("recommend_pg_configuration", RecommendPGConfigurationTool())
	provider.baseRegistry.Register("read_resource", ReadResourceTool(resourceReg))
	provider.baseRegistry.Register("server_info", ServerInfoTool(serverInfo))

	return provider
}

// RegisterTools initializes tool registrations
// This is called at startup to ensure the base registry is populated for List() operations
func (p *ContextAwareProvider) RegisterTools(ctx context.Context) error {
	// Pre-create a registry for the fallback client if auth is disabled
	// This ensures tools are ready for immediate use
	if !p.authEnabled {
		_ = p.getOrCreateRegistryForClient(p.fallbackClient)
	}
	return nil
}

// List returns all registered tool definitions
// This returns a combined list from the base registry and a sample client registry
func (p *ContextAwareProvider) List() []mcp.Tool {
	// Use fallback client to get full tool list for discovery
	registry := p.getOrCreateRegistryForClient(p.fallbackClient)
	return registry.List()
}

// getOrCreateRegistryForClient returns a cached registry for the given client
// or creates a new one if it doesn't exist
func (p *ContextAwareProvider) getOrCreateRegistryForClient(client *database.Client) *Registry {
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
	registry.Register("read_resource", ReadResourceTool(p.resourceReg))
	registry.Register("server_info", ServerInfoTool(p.serverInfo))

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
	// Get the appropriate database client for this request
	dbClient, err := p.getClient(ctx)
	if err != nil {
		return mcp.ToolResponse{
			Content: []mcp.ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Failed to get database client: %v", err),
				},
			},
			IsError: true,
		}, fmt.Errorf("failed to get database client: %w", err)
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
		// Authentication disabled - use fallback client (shared)
		return p.fallbackClient, nil
	}

	// Authentication enabled - get per-token client
	tokenHash := auth.GetTokenHashFromContext(ctx)
	if tokenHash == "" {
		return nil, fmt.Errorf("no authentication token found in request context")
	}

	// Get or create client for this token
	client, err := p.clientManager.GetClient(tokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for token: %w", err)
	}

	return client, nil
}
