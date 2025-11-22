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
	"fmt"
	"os"
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
	baseRegistry   *Registry // Registry for tool definitions (List operation)
	clientManager  *database.ClientManager
	resourceReg    *resources.ContextAwareRegistry
	authEnabled    bool
	fallbackClient *database.Client // Used when auth is disabled
	cfg            *config.Config   // Server configuration (for embedding settings)
	userStore      *auth.UserStore  // User store for authentication
	userFilePath   string           // Path to user file for persisting updates

	// Cache of registries per client to avoid re-creating tools on every Execute()
	mu               sync.RWMutex
	clientRegistries map[*database.Client]*Registry

	// Hidden tools registry (not advertised to LLM but available for execution)
	hiddenRegistry *Registry
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
	registry.Register("similarity_search", SimilaritySearchTool(client, p.cfg))
}

// NewContextAwareProvider creates a new context-aware tool provider
func NewContextAwareProvider(clientManager *database.ClientManager, resourceReg *resources.ContextAwareRegistry, authEnabled bool, fallbackClient *database.Client, cfg *config.Config, userStore *auth.UserStore, userFilePath string) *ContextAwareProvider {
	provider := &ContextAwareProvider{
		baseRegistry:     NewRegistry(),
		clientManager:    clientManager,
		resourceReg:      resourceReg,
		authEnabled:      authEnabled,
		fallbackClient:   fallbackClient,
		cfg:              cfg,
		userStore:        userStore,
		userFilePath:     userFilePath,
		clientRegistries: make(map[*database.Client]*Registry),
		hiddenRegistry:   NewRegistry(),
	}

	// Register ALL tools in base registry so they're always visible in tools/list
	// Database-dependent tools will fail gracefully in Execute() if no connection exists
	// This provides better UX - users can discover all tools even before connecting
	provider.registerStatelessTools(provider.baseRegistry)
	provider.registerDatabaseTools(provider.baseRegistry, nil) // nil client for base registry

	// Register hidden tools (not advertised to LLM but available for execution)
	if userStore != nil {
		provider.hiddenRegistry.Register("authenticate_user", AuthenticateUserTool(userStore))
	}

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

func (a *resourceReaderAdapter) Read(ctx context.Context, uri string) (mcp.ResourceContent, error) {
	// Pass the context through to the ContextAwareRegistry
	// This ensures the authentication token is available for per-token connection isolation
	return a.registry.Read(ctx, uri)
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
// Hidden tools (like authenticate_user) are not included as they're in a separate registry
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
	// Check if this is a hidden tool (like authenticate_user)
	// Hidden tools don't require authentication and are not advertised to LLM
	if p.hiddenRegistry != nil {
		if _, exists := p.hiddenRegistry.Get(name); exists {
			// Tool found in hidden registry - execute it without auth validation
			response, err := p.hiddenRegistry.Execute(ctx, name, args)
			// After authentication, save the updated user store to persist last login time
			if name == "authenticate_user" && err == nil && p.userStore != nil && p.userFilePath != "" {
				if saveErr := auth.SaveUserStore(p.userFilePath, p.userStore); saveErr != nil {
					// Log error but don't fail the authentication
					fmt.Fprintf(os.Stderr, "Warning: failed to save user store: %v\n", saveErr)
				}
			}
			return response, err
		}
	}

	// If authentication is enabled, validate token for ALL non-hidden tools
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

	// Get or create client for this token
	// Auto-connect if database is configured (authenticated users get automatic database access)
	client, err := p.clientManager.GetOrCreateClient(tokenHash, true)
	if err != nil {
		return nil, fmt.Errorf("no database connection configured for this token: %w", err)
	}

	return client, nil
}
