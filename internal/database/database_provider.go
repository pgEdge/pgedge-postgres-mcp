/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
	"context"
	"fmt"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/mcp"
)

// StdioDatabaseProvider implements mcp.DatabaseProvider for STDIO mode
// In STDIO mode there's no authentication, so we use a fixed key for all operations
type StdioDatabaseProvider struct {
	clientManager *ClientManager
	sessionKey    string // Key used for session tracking (typically "default")
}

// NewStdioDatabaseProvider creates a new database provider for STDIO mode
func NewStdioDatabaseProvider(clientManager *ClientManager) *StdioDatabaseProvider {
	return &StdioDatabaseProvider{
		clientManager: clientManager,
		sessionKey:    "default",
	}
}

// ListDatabases returns available databases and the current database name
func (p *StdioDatabaseProvider) ListDatabases(ctx context.Context) ([]mcp.DatabaseInfo, string, error) {
	configs := p.clientManager.GetDatabaseConfigs()
	current := p.clientManager.GetCurrentDatabase(p.sessionKey)

	databases := make([]mcp.DatabaseInfo, 0, len(configs))
	for i := range configs {
		cfg := &configs[i]
		databases = append(databases, mcp.DatabaseInfo{
			Name:     cfg.Name,
			Host:     cfg.Host,
			Port:     cfg.Port,
			Database: cfg.Database,
			User:     cfg.User,
			SSLMode:  cfg.SSLMode,
		})
	}

	return databases, current, nil
}

// SelectDatabase sets the current database for the session
// In STDIO mode, we close connections to other databases since only one is typically needed
func (p *StdioDatabaseProvider) SelectDatabase(ctx context.Context, name string) error {
	// Verify the database exists before setting it
	cfg := p.clientManager.GetDatabaseConfig(name)
	if cfg == nil {
		return fmt.Errorf("database '%s' not found", name)
	}

	// Use the variant that closes other connections to avoid accumulation
	return p.clientManager.SetCurrentDatabaseAndCloseOthers(p.sessionKey, name)
}

// HTTPDatabaseProvider implements mcp.DatabaseProvider for HTTP mode
// In HTTP mode, the session key is derived from the authentication token
type HTTPDatabaseProvider struct {
	clientManager  *ClientManager
	authEnabled    bool
	accessChecker  *auth.DatabaseAccessChecker
}

// NewHTTPDatabaseProvider creates a new database provider for HTTP mode
func NewHTTPDatabaseProvider(clientManager *ClientManager, authEnabled bool, accessChecker *auth.DatabaseAccessChecker) *HTTPDatabaseProvider {
	return &HTTPDatabaseProvider{
		clientManager:  clientManager,
		authEnabled:    authEnabled,
		accessChecker:  accessChecker,
	}
}

// getSessionKey returns the session key based on authentication context
func (p *HTTPDatabaseProvider) getSessionKey(ctx context.Context) string {
	if !p.authEnabled {
		return "default"
	}

	tokenHash := auth.GetTokenHashFromContext(ctx)
	if tokenHash == "" {
		return "default"
	}
	return tokenHash
}

// ListDatabases returns available databases and the current database name
// Filters databases based on access control (available_to_users for session users,
// bound database for API tokens)
func (p *HTTPDatabaseProvider) ListDatabases(ctx context.Context) ([]mcp.DatabaseInfo, string, error) {
	sessionKey := p.getSessionKey(ctx)
	configs := p.clientManager.GetDatabaseConfigs()

	// Filter databases based on access control
	var accessibleConfigs []config.NamedDatabaseConfig
	if p.accessChecker != nil {
		accessibleConfigs = p.accessChecker.GetAccessibleDatabases(ctx, configs)
	} else {
		accessibleConfigs = configs
	}

	current := p.clientManager.GetCurrentDatabase(sessionKey)

	// Verify current database is still accessible, otherwise reset to first accessible
	currentAccessible := false
	for i := range accessibleConfigs {
		if accessibleConfigs[i].Name == current {
			currentAccessible = true
			break
		}
	}
	if !currentAccessible && len(accessibleConfigs) > 0 {
		current = accessibleConfigs[0].Name
	}

	databases := make([]mcp.DatabaseInfo, 0, len(accessibleConfigs))
	for i := range accessibleConfigs {
		cfg := &accessibleConfigs[i]
		databases = append(databases, mcp.DatabaseInfo{
			Name:     cfg.Name,
			Host:     cfg.Host,
			Port:     cfg.Port,
			Database: cfg.Database,
			User:     cfg.User,
			SSLMode:  cfg.SSLMode,
		})
	}

	return databases, current, nil
}

// SelectDatabase sets the current database for the session
// Closes connections to other databases to prevent connection buildup on the PostgreSQL server
// Enforces access control based on available_to_users for session users
func (p *HTTPDatabaseProvider) SelectDatabase(ctx context.Context, name string) error {
	sessionKey := p.getSessionKey(ctx)

	// Verify the database exists before setting it
	cfg := p.clientManager.GetDatabaseConfig(name)
	if cfg == nil {
		return fmt.Errorf("database '%s' not found", name)
	}

	// Check access control
	if p.accessChecker != nil && !p.accessChecker.CanAccessDatabase(ctx, cfg) {
		return fmt.Errorf("access denied to database '%s'", name)
	}

	// Close other connections to prevent buildup on the PostgreSQL server
	return p.clientManager.SetCurrentDatabaseAndCloseOthers(sessionKey, name)
}
