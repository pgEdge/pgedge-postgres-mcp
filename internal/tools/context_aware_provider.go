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
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
	"pgedge-postgres-mcp/internal/resources"
)

// ContextAwareProvider wraps a tool registry and provides per-token database clients
// This ensures connection isolation in HTTP/HTTPS mode with authentication
type ContextAwareProvider struct {
	baseRegistry    *Registry // Registry for tool definitions (List operation)
	clientManager   *database.ClientManager
	resourceReg     *resources.ContextAwareRegistry
	authEnabled     bool
	fallbackClient  *database.Client // Used when auth is disabled
	cfg             *config.Config   // Server configuration (for embedding settings)

	// Cache of registries per client to avoid re-creating tools on every Execute()
	mu               sync.RWMutex
	clientRegistries map[*database.Client]*Registry
}

// registerStatelessTools registers all stateless tools (those that don't require a database client)
func (p *ContextAwareProvider) registerStatelessTools(registry *Registry) {
	// Note: read_resource tool provides backward compatibility for resource access
	// Resources are also accessible via the native MCP resources/read endpoint
	registry.Register("read_resource", ReadResourceTool(p.createResourceAdapter()))

	// Embedding generation tool (stateless, only requires config)
	registry.Register("generate_embedding", GenerateEmbeddingTool(p.cfg))
}

// registerDatabaseTools registers all database-dependent tools
func (p *ContextAwareProvider) registerDatabaseTools(registry *Registry, client *database.Client) {
	registry.Register("query_database", QueryDatabaseTool(client))
	registry.Register("get_schema_info", GetSchemaInfoTool(client))
	registry.Register("semantic_search", SemanticSearchTool(client, p.cfg))
	registry.Register("search_similar", SearchSimilarTool(client, p.cfg))
}

// NewContextAwareProvider creates a new context-aware tool provider
func NewContextAwareProvider(clientManager *database.ClientManager, resourceReg *resources.ContextAwareRegistry, authEnabled bool, fallbackClient *database.Client, cfg *config.Config) *ContextAwareProvider {
	provider := &ContextAwareProvider{
		baseRegistry:     NewRegistry(),
		clientManager:    clientManager,
		resourceReg:      resourceReg,
		authEnabled:      authEnabled,
		fallbackClient:   fallbackClient,
		cfg:              cfg,
		clientRegistries: make(map[*database.Client]*Registry),
	}

	// Register ALL tools in base registry so they're always visible in tools/list
	// Database-dependent tools will fail gracefully in Execute() if no connection exists
	// This provides better UX - users can discover all tools even before connecting
	provider.registerStatelessTools(provider.baseRegistry)
	provider.registerDatabaseTools(provider.baseRegistry, nil) // nil client for base registry

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

// List returns registered tool definitions based on connection state
// Smart filtering: only returns database-dependent tools when a connection exists
// This reduces token usage by not advertising unusable tools
func (p *ContextAwareProvider) List() []mcp.Tool {
	// Check if we have an active database connection
	hasConnection := p.hasActiveConnection()

	allTools := p.baseRegistry.List()

	// If we have a connection, return all tools
	if hasConnection {
		return allTools
	}

	// No connection - filter to only stateless tools
	statelessTools := map[string]bool{
		"manage_connections": true,
		"read_resource":      true,
		"generate_embedding": true,
	}

	filtered := make([]mcp.Tool, 0, len(statelessTools))
	for _, tool := range allTools {
		if statelessTools[tool.Name] {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

// hasActiveConnection checks if there's an active database connection
func (p *ContextAwareProvider) hasActiveConnection() bool {
	if !p.authEnabled {
		// Auth disabled - check if default client exists and has a connection
		client, err := p.clientManager.GetOrCreateClient("default", false)
		if err != nil || client == nil {
			return false
		}
		// Check if client has metadata loaded (indicates successful connection)
		return client.IsMetadataLoadedFor(client.GetDefaultConnection())
	}

	// Auth enabled - check if any clients exist
	// In auth mode, we can't know which token is requesting without context
	// So we return all tools if any client exists, or only stateless if none
	return p.clientManager.GetClientCount() > 0
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

	// Register all tools using helper methods to avoid duplication
	p.registerStatelessTools(registry)
	p.registerDatabaseTools(registry, client)

	// Cache for future use
	p.clientRegistries[client] = registry

	return registry
}

// Execute runs a tool by name with the given arguments and context
// Uses cached per-client registries to avoid re-creating tools on every request
func (p *ContextAwareProvider) Execute(ctx context.Context, name string, args map[string]interface{}) (mcp.ToolResponse, error) {
	// If authentication is enabled, validate token for ALL tools
	if p.authEnabled {
		tokenHash := auth.GetTokenHashFromContext(ctx)
		if tokenHash == "" {
			return mcp.ToolResponse{}, fmt.Errorf("no authentication token found in request context")
		}
	}

	// Check if this is a stateless tool that doesn't require a database client
	statelessTools := map[string]bool{
		"read_resource":      true, // Resource access tool
		"generate_embedding": true, // Embedding generation doesn't need database
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
					Text: fmt.Sprintf("Failed to get database client: %v\nPlease ensure database connection is configured via environment variables or config file.", err),
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
