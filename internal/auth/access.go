/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"context"

	"pgedge-postgres-mcp/internal/config"
)

// DatabaseAccessChecker handles database access control based on authentication context
type DatabaseAccessChecker struct {
	tokenStore  *TokenStore
	authEnabled bool
	isSTDIO     bool
}

// NewDatabaseAccessChecker creates a new database access checker
func NewDatabaseAccessChecker(tokenStore *TokenStore, authEnabled, isSTDIO bool) *DatabaseAccessChecker {
	return &DatabaseAccessChecker{
		tokenStore:  tokenStore,
		authEnabled: authEnabled,
		isSTDIO:     isSTDIO,
	}
}

// CanAccessDatabase checks if the current request context has access to a database
// Access rules:
//   - STDIO mode: all databases accessible
//   - Auth disabled (--no-auth): all databases accessible
//   - API token: only bound database (or first if empty)
//   - Session user: check available_to_users (empty = all)
func (dac *DatabaseAccessChecker) CanAccessDatabase(ctx context.Context, db *config.NamedDatabaseConfig) bool {
	// STDIO mode - all databases available
	if dac.isSTDIO {
		return true
	}

	// Auth disabled - all databases available
	if !dac.authEnabled {
		return true
	}

	// Check if API token
	if IsAPITokenFromContext(ctx) {
		// API tokens are bound to specific databases via GetBoundDatabase
		// This check is for listing purposes - actual binding is handled separately
		return true
	}

	// Session user - check available_to_users
	username := GetUsernameFromContext(ctx)
	if username == "" {
		// No username in context and not API token - deny access
		return false
	}

	return dac.isUserAllowed(username, db.AvailableToUsers)
}

// isUserAllowed checks if a username is in the allowed list
// Empty allowedUsers means all users are allowed
func (dac *DatabaseAccessChecker) isUserAllowed(username string, allowedUsers []string) bool {
	// Empty list means all users allowed
	if len(allowedUsers) == 0 {
		return true
	}

	// Check if username is in allowed list
	for _, allowed := range allowedUsers {
		if allowed == username {
			return true
		}
	}

	return false
}

// GetBoundDatabase returns the database name that an API token is bound to
// Returns empty string if not an API token or if token has no specific binding
// The caller should treat empty as "use first configured database"
func (dac *DatabaseAccessChecker) GetBoundDatabase(ctx context.Context) string {
	// Only API tokens have database bindings
	if !IsAPITokenFromContext(ctx) {
		return ""
	}

	// Get the token hash from context
	tokenHash := GetTokenHashFromContext(ctx)
	if tokenHash == "" || dac.tokenStore == nil {
		return ""
	}

	// Look up token by hash
	token := dac.tokenStore.GetTokenByHash(tokenHash)
	if token == nil {
		return ""
	}

	return token.Database
}

// GetAccessibleDatabases returns the list of databases accessible to the current context
// For API tokens, returns only the bound database (or first if unbound)
// For session users, filters by available_to_users
// For STDIO/no-auth mode, returns all databases
func (dac *DatabaseAccessChecker) GetAccessibleDatabases(ctx context.Context, databases []config.NamedDatabaseConfig) []config.NamedDatabaseConfig {
	// STDIO mode or auth disabled - return all
	if dac.isSTDIO || !dac.authEnabled {
		return databases
	}

	// API token - return only bound database
	if IsAPITokenFromContext(ctx) {
		boundDB := dac.GetBoundDatabase(ctx)

		// If token is bound to a specific database, return only that
		if boundDB != "" {
			for i := range databases {
				if databases[i].Name == boundDB {
					return []config.NamedDatabaseConfig{databases[i]}
				}
			}
			// Bound database not found - return empty (error case)
			return nil
		}

		// Token not bound - return first database
		if len(databases) > 0 {
			return []config.NamedDatabaseConfig{databases[0]}
		}
		return nil
	}

	// Session user - filter by available_to_users
	username := GetUsernameFromContext(ctx)
	if username == "" {
		return nil
	}

	var accessible []config.NamedDatabaseConfig
	for i := range databases {
		if dac.isUserAllowed(username, databases[i].AvailableToUsers) {
			accessible = append(accessible, databases[i])
		}
	}

	return accessible
}
