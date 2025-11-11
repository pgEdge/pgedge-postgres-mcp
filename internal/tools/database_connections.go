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
	"sort"
	"strings"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/crypto"
	"pgedge-postgres-mcp/internal/mcp"
)

// ConnectionManager provides access to saved connections
type ConnectionManager struct {
	tokenStore     *auth.TokenStore
	config         *config.Config       // Used only for token file path when auth enabled
	preferences    *config.Preferences  // Used for connections when auth disabled
	authEnabled    bool
	encryptionKey  *crypto.EncryptionKey
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(tokenStore *auth.TokenStore, cfg *config.Config, prefs *config.Preferences, authEnabled bool, encryptionKey *crypto.EncryptionKey) *ConnectionManager {
	return &ConnectionManager{
		tokenStore:    tokenStore,
		config:        cfg,
		preferences:   prefs,
		authEnabled:   authEnabled,
		encryptionKey: encryptionKey,
	}
}

// GetConnectionStore returns the appropriate connection store based on auth mode
func (cm *ConnectionManager) GetConnectionStore(ctx context.Context) (*auth.SavedConnectionStore, error) {
	if cm.authEnabled {
		// Get token hash from context
		tokenHash := auth.GetTokenHashFromContext(ctx)
		if tokenHash == "" {
			return nil, fmt.Errorf("no authentication token found in request context")
		}

		// Get connection store for this token
		if cm.tokenStore == nil {
			return nil, fmt.Errorf("token store not initialized")
		}

		return cm.tokenStore.GetConnectionStore(tokenHash)
	}

	// Auth disabled - use global preferences
	if cm.preferences == nil {
		return nil, fmt.Errorf("preferences not initialized")
	}

	if cm.preferences.Connections == nil {
		cm.preferences.Connections = auth.NewSavedConnectionStore()
	}

	return cm.preferences.Connections, nil
}

// AddDatabaseConnectionTool creates the add_database_connection tool
func AddDatabaseConnectionTool(connMgr *ConnectionManager, configPath string) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "add_database_connection",
			Description: "Save a database connection with an alias for later use. The connection will be persisted and available in future sessions. Passwords are encrypted before storage. All SSL/TLS parameters are supported for secure connections.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"alias": map[string]interface{}{
						"type":        "string",
						"description": "Friendly name for this connection (e.g., 'production', 'staging', 'local')",
					},
					"host": map[string]interface{}{
						"type":        "string",
						"description": "Database server hostname or IP address (required)",
					},
					"port": map[string]interface{}{
						"type":        "number",
						"description": "Database server port (default: 5432)",
					},
					"user": map[string]interface{}{
						"type":        "string",
						"description": "Database user (required)",
					},
					"password": map[string]interface{}{
						"type":        "string",
						"description": "Database password (will be encrypted before storage)",
					},
					"dbname": map[string]interface{}{
						"type":        "string",
						"description": "Database name (defaults to username if not specified)",
					},
					"sslmode": map[string]interface{}{
						"type":        "string",
						"description": "SSL mode: disable, allow, prefer, require, verify-ca, verify-full (default: prefer)",
					},
					"sslcert": map[string]interface{}{
						"type":        "string",
						"description": "Path to client certificate file for SSL authentication",
					},
					"sslkey": map[string]interface{}{
						"type":        "string",
						"description": "Path to client private key file for SSL authentication",
					},
					"sslrootcert": map[string]interface{}{
						"type":        "string",
						"description": "Path to root CA certificate file for SSL verification",
					},
					"sslpassword": map[string]interface{}{
						"type":        "string",
						"description": "Password for client key file (will be encrypted before storage)",
					},
					"connect_timeout": map[string]interface{}{
						"type":        "number",
						"description": "Connection timeout in seconds",
					},
					"application_name": map[string]interface{}{
						"type":        "string",
						"description": "Application name to identify connections in pg_stat_activity",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Optional description or notes about this connection",
					},
				},
				Required: []string{"alias", "host", "user"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Parse required parameters
			alias, ok := args["alias"].(string)
			if !ok || alias == "" {
				return mcp.NewToolError("Error: 'alias' parameter is required")
			}

			host, ok := args["host"].(string)
			if !ok || host == "" {
				return mcp.NewToolError("Error: 'host' parameter is required")
			}

			user, ok := args["user"].(string)
			if !ok || user == "" {
				return mcp.NewToolError("Error: 'user' parameter is required")
			}

			// Create connection struct
			conn := &auth.SavedConnection{
				Alias: alias,
				Host:  host,
				User:  user,
			}

			// Parse optional parameters
			if port, ok := args["port"].(float64); ok {
				conn.Port = int(port)
			}

			if dbname, ok := args["dbname"].(string); ok && dbname != "" {
				conn.DBName = dbname
			}

			if desc, ok := args["description"].(string); ok && desc != "" {
				conn.Description = desc
			}

			// Encrypt password if provided
			if password, ok := args["password"].(string); ok && password != "" {
				encrypted, err := connMgr.encryptionKey.Encrypt(password)
				if err != nil {
					return mcp.NewToolError(fmt.Sprintf("Error encrypting password: %v", err))
				}
				conn.Password = encrypted
			}

			// SSL parameters
			if sslmode, ok := args["sslmode"].(string); ok && sslmode != "" {
				conn.SSLMode = sslmode
			}
			if sslcert, ok := args["sslcert"].(string); ok && sslcert != "" {
				conn.SSLCert = sslcert
			}
			if sslkey, ok := args["sslkey"].(string); ok && sslkey != "" {
				conn.SSLKey = sslkey
			}
			if sslrootcert, ok := args["sslrootcert"].(string); ok && sslrootcert != "" {
				conn.SSLRootCert = sslrootcert
			}
			if sslpassword, ok := args["sslpassword"].(string); ok && sslpassword != "" {
				encrypted, err := connMgr.encryptionKey.Encrypt(sslpassword)
				if err != nil {
					return mcp.NewToolError(fmt.Sprintf("Error encrypting SSL password: %v", err))
				}
				conn.SSLPassword = encrypted
			}

			// Additional parameters
			if timeout, ok := args["connect_timeout"].(float64); ok {
				conn.ConnectTimeout = int(timeout)
			}
			if appName, ok := args["application_name"].(string); ok && appName != "" {
				conn.ApplicationName = appName
			}

			// Get connection store
			ctx := context.Background()
			store, err := connMgr.GetConnectionStore(ctx)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: %v", err))
			}

			// Add connection
			if err := store.Add(conn); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: %v", err))
			}

			// Save changes
			if err := connMgr.saveChanges(configPath); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error saving changes: %v", err))
			}

			msg := fmt.Sprintf("Successfully saved connection '%s'\n", alias)
			msg += fmt.Sprintf("Host: %s\n", conn.Host)
			if conn.Port != 0 {
				msg += fmt.Sprintf("Port: %d\n", conn.Port)
			}
			msg += fmt.Sprintf("User: %s\n", conn.User)
			if conn.DBName != "" {
				msg += fmt.Sprintf("Database: %s\n", conn.DBName)
			}
			if conn.SSLMode != "" {
				msg += fmt.Sprintf("SSL Mode: %s\n", conn.SSLMode)
			}
			if conn.Description != "" {
				msg += fmt.Sprintf("Description: %s", conn.Description)
			}

			return mcp.NewToolSuccess(msg)
		},
	}
}

// RemoveDatabaseConnectionTool creates the remove_database_connection tool
func RemoveDatabaseConnectionTool(connMgr *ConnectionManager, configPath string) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "remove_database_connection",
			Description: "Remove a saved database connection by its alias. This will permanently delete the saved connection.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"alias": map[string]interface{}{
						"type":        "string",
						"description": "Alias of the connection to remove",
					},
				},
				Required: []string{"alias"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			alias, ok := args["alias"].(string)
			if !ok || alias == "" {
				return mcp.NewToolError("Error: 'alias' parameter is required")
			}

			// Get connection store
			ctx := context.Background()
			store, err := connMgr.GetConnectionStore(ctx)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: %v", err))
			}

			// Remove connection
			if err := store.Remove(alias); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: %v", err))
			}

			// Save changes
			if err := connMgr.saveChanges(configPath); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error saving changes: %v", err))
			}

			return mcp.NewToolSuccess(fmt.Sprintf("Successfully removed connection '%s'", alias))
		},
	}
}

// ListDatabaseConnectionsTool creates the list_database_connections tool
func ListDatabaseConnectionsTool(connMgr *ConnectionManager) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "list_database_connections",
			Description: "List all saved database connections with their aliases, maintenance databases, and descriptions. Use this to see available connections before connecting.",
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Get connection store
			ctx := context.Background()
			store, err := connMgr.GetConnectionStore(ctx)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: %v", err))
			}

			connections := store.List()
			if len(connections) == 0 {
				return mcp.NewToolSuccess("No saved connections found.\n\nUse add_database_connection to save a connection for later use.")
			}

			// Sort by alias
			sort.Slice(connections, func(i, j int) bool {
				return connections[i].Alias < connections[j].Alias
			})

			var result strings.Builder
			result.WriteString(fmt.Sprintf("Saved Database Connections (%d total):\n", len(connections)))
			result.WriteString("==========================================\n\n")

			for _, conn := range connections {
				result.WriteString(fmt.Sprintf("Alias: %s\n", conn.Alias))
				result.WriteString(fmt.Sprintf("  Host: %s", conn.Host))
				if conn.Port != 0 && conn.Port != 5432 {
					result.WriteString(fmt.Sprintf(":%d", conn.Port))
				}
				result.WriteString("\n")
				result.WriteString(fmt.Sprintf("  User: %s\n", conn.User))
				if conn.DBName != "" {
					result.WriteString(fmt.Sprintf("  Database: %s\n", conn.DBName))
				}
				if conn.SSLMode != "" {
					result.WriteString(fmt.Sprintf("  SSL Mode: %s\n", conn.SSLMode))
				}
				if conn.SSLCert != "" {
					result.WriteString(fmt.Sprintf("  SSL Cert: %s\n", conn.SSLCert))
				}
				if conn.Description != "" {
					result.WriteString(fmt.Sprintf("  Description: %s\n", conn.Description))
				}
				result.WriteString(fmt.Sprintf("  Created: %s\n", conn.CreatedAt.Format("2006-01-02 15:04:05")))
				if !conn.LastUsedAt.IsZero() {
					result.WriteString(fmt.Sprintf("  Last Used: %s\n", conn.LastUsedAt.Format("2006-01-02 15:04:05")))
				}
				result.WriteString("\n")
			}

			return mcp.NewToolSuccess(result.String())
		},
	}
}

// EditDatabaseConnectionTool creates the edit_database_connection tool
func EditDatabaseConnectionTool(connMgr *ConnectionManager, configPath string) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "edit_database_connection",
			Description: "Permanently modify a saved database connection's configuration. IMPORTANT: Only use this tool when the user explicitly asks to update, change, or edit a saved connection. DO NOT use this tool to temporarily connect to a different database - use set_database_connection with a full connection string instead. Provide only the fields you want to update. Passwords will be encrypted before storage.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"alias": map[string]interface{}{
						"type":        "string",
						"description": "Alias of the connection to edit (required)",
					},
					"host": map[string]interface{}{
						"type":        "string",
						"description": "New database server hostname or IP address",
					},
					"port": map[string]interface{}{
						"type":        "number",
						"description": "New database server port",
					},
					"user": map[string]interface{}{
						"type":        "string",
						"description": "New database user",
					},
					"password": map[string]interface{}{
						"type":        "string",
						"description": "New database password (will be encrypted)",
					},
					"dbname": map[string]interface{}{
						"type":        "string",
						"description": "New database name",
					},
					"sslmode": map[string]interface{}{
						"type":        "string",
						"description": "New SSL mode",
					},
					"sslcert": map[string]interface{}{
						"type":        "string",
						"description": "New path to client certificate file",
					},
					"sslkey": map[string]interface{}{
						"type":        "string",
						"description": "New path to client private key file",
					},
					"sslrootcert": map[string]interface{}{
						"type":        "string",
						"description": "New path to root CA certificate file",
					},
					"sslpassword": map[string]interface{}{
						"type":        "string",
						"description": "New password for client key file (will be encrypted)",
					},
					"connect_timeout": map[string]interface{}{
						"type":        "number",
						"description": "New connection timeout in seconds",
					},
					"application_name": map[string]interface{}{
						"type":        "string",
						"description": "New application name",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description or notes",
					},
				},
				Required: []string{"alias"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			alias, ok := args["alias"].(string)
			if !ok || alias == "" {
				return mcp.NewToolError("Error: 'alias' parameter is required")
			}

			// Build updates struct with only provided fields
			updates := &auth.SavedConnection{}
			hasUpdates := false

			if host, ok := args["host"].(string); ok && host != "" {
				updates.Host = host
				hasUpdates = true
			}
			if port, ok := args["port"].(float64); ok {
				updates.Port = int(port)
				hasUpdates = true
			}
			if user, ok := args["user"].(string); ok && user != "" {
				updates.User = user
				hasUpdates = true
			}
			if dbname, ok := args["dbname"].(string); ok && dbname != "" {
				updates.DBName = dbname
				hasUpdates = true
			}
			if desc, ok := args["description"].(string); ok && desc != "" {
				updates.Description = desc
				hasUpdates = true
			}

			// Encrypt password if provided
			if password, ok := args["password"].(string); ok && password != "" {
				encrypted, err := connMgr.encryptionKey.Encrypt(password)
				if err != nil {
					return mcp.NewToolError(fmt.Sprintf("Error encrypting password: %v", err))
				}
				updates.Password = encrypted
				hasUpdates = true
			}

			// SSL parameters
			if sslmode, ok := args["sslmode"].(string); ok && sslmode != "" {
				updates.SSLMode = sslmode
				hasUpdates = true
			}
			if sslcert, ok := args["sslcert"].(string); ok && sslcert != "" {
				updates.SSLCert = sslcert
				hasUpdates = true
			}
			if sslkey, ok := args["sslkey"].(string); ok && sslkey != "" {
				updates.SSLKey = sslkey
				hasUpdates = true
			}
			if sslrootcert, ok := args["sslrootcert"].(string); ok && sslrootcert != "" {
				updates.SSLRootCert = sslrootcert
				hasUpdates = true
			}
			if sslpassword, ok := args["sslpassword"].(string); ok && sslpassword != "" {
				encrypted, err := connMgr.encryptionKey.Encrypt(sslpassword)
				if err != nil {
					return mcp.NewToolError(fmt.Sprintf("Error encrypting SSL password: %v", err))
				}
				updates.SSLPassword = encrypted
				hasUpdates = true
			}

			// Additional parameters
			if timeout, ok := args["connect_timeout"].(float64); ok {
				updates.ConnectTimeout = int(timeout)
				hasUpdates = true
			}
			if appName, ok := args["application_name"].(string); ok && appName != "" {
				updates.ApplicationName = appName
				hasUpdates = true
			}

			if !hasUpdates {
				return mcp.NewToolError("Error: At least one field must be provided to update")
			}

			// Get connection store
			ctx := context.Background()
			store, err := connMgr.GetConnectionStore(ctx)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: %v", err))
			}

			// Update connection
			if err := store.Update(alias, updates); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: %v", err))
			}

			// Save changes
			if err := connMgr.saveChanges(configPath); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error saving changes: %v", err))
			}

			return mcp.NewToolSuccess(fmt.Sprintf("Successfully updated connection '%s'", alias))
		},
	}
}

// saveChanges persists connection changes to the appropriate store
func (cm *ConnectionManager) saveChanges(configPath string) error {
	if cm.authEnabled {
		// Save token store (includes per-token connections)
		if cm.tokenStore == nil {
			return fmt.Errorf("token store not initialized")
		}

		// Get token file path from config
		tokenFilePath := ""
		if cm.config != nil && cm.config.HTTP.Auth.TokenFile != "" {
			tokenFilePath = cm.config.HTTP.Auth.TokenFile
		}

		if tokenFilePath == "" {
			return fmt.Errorf("token file path not configured")
		}

		return auth.SaveTokenStore(tokenFilePath, cm.tokenStore)
	}

	// Auth disabled - save global preferences
	if cm.preferences == nil {
		return fmt.Errorf("preferences not initialized")
	}

	if configPath == "" {
		return fmt.Errorf("preferences file path not provided")
	}

	return config.SavePreferences(configPath, cm.preferences)
}
