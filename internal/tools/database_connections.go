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
	"pgedge-postgres-mcp/internal/mcp"
)

// ConnectionManager provides access to saved connections
type ConnectionManager struct {
	tokenStore  *auth.TokenStore
	config      *config.Config       // Used only for token file path when auth enabled
	preferences *config.Preferences  // Used for connections when auth disabled
	authEnabled bool
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(tokenStore *auth.TokenStore, cfg *config.Config, prefs *config.Preferences, authEnabled bool) *ConnectionManager {
	return &ConnectionManager{
		tokenStore:  tokenStore,
		config:      cfg,
		preferences: prefs,
		authEnabled: authEnabled,
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
			Description: "Save a database connection with an alias for later use. The connection will be persisted and available in future sessions. Each connection includes a maintenance database (default: 'postgres') used for initial connections, similar to pgAdmin.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"alias": map[string]interface{}{
						"type":        "string",
						"description": "Friendly name for this connection (e.g., 'production', 'staging', 'local')",
					},
					"connection_string": map[string]interface{}{
						"type":        "string",
						"description": "PostgreSQL connection string (e.g., 'postgres://user:pass@host:port/database')",
					},
					"maintenance_db": map[string]interface{}{
						"type":        "string",
						"description": "Maintenance/initial database to connect to (default: 'postgres'). This is the database used for establishing the initial connection.",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Optional description or notes about this connection",
					},
				},
				Required: []string{"alias", "connection_string"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			alias, ok := args["alias"].(string)
			if !ok || alias == "" {
				return mcp.NewToolError("Error: 'alias' parameter is required")
			}

			connString, ok := args["connection_string"].(string)
			if !ok || connString == "" {
				return mcp.NewToolError("Error: 'connection_string' parameter is required")
			}

			maintenanceDB := "postgres" // Default
			if db, ok := args["maintenance_db"].(string); ok && db != "" {
				maintenanceDB = db
			}

			description := ""
			if desc, ok := args["description"].(string); ok {
				description = desc
			}

			// Get connection store
			ctx := context.Background() // TODO: Pass context from handler
			store, err := connMgr.GetConnectionStore(ctx)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: %v", err))
			}

			// Add connection
			if err := store.Add(alias, connString, maintenanceDB, description); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: %v", err))
			}

			// Save changes
			if err := connMgr.saveChanges(configPath); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error saving changes: %v", err))
			}

			msg := fmt.Sprintf("Successfully saved connection '%s'", alias)
			if description != "" {
				msg += fmt.Sprintf("\nDescription: %s", description)
			}
			msg += fmt.Sprintf("\nMaintenance DB: %s", maintenanceDB)

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
				result.WriteString(fmt.Sprintf("Connection: %s\n", conn.ConnectionString))
				result.WriteString(fmt.Sprintf("Maintenance DB: %s\n", conn.MaintenanceDB))
				if conn.Description != "" {
					result.WriteString(fmt.Sprintf("Description: %s\n", conn.Description))
				}
				result.WriteString(fmt.Sprintf("Created: %s\n", conn.CreatedAt.Format("2006-01-02 15:04:05")))
				if !conn.LastUsedAt.IsZero() {
					result.WriteString(fmt.Sprintf("Last Used: %s\n", conn.LastUsedAt.Format("2006-01-02 15:04:05")))
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
			Description: "Edit an existing saved database connection. You can update the connection string, maintenance database, and/or description.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"alias": map[string]interface{}{
						"type":        "string",
						"description": "Alias of the connection to edit",
					},
					"connection_string": map[string]interface{}{
						"type":        "string",
						"description": "New connection string (optional, leave empty to keep current)",
					},
					"maintenance_db": map[string]interface{}{
						"type":        "string",
						"description": "New maintenance database (optional, leave empty to keep current)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description (optional, leave empty to keep current)",
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

			connString := ""
			if cs, ok := args["connection_string"].(string); ok {
				connString = cs
			}

			maintenanceDB := ""
			if db, ok := args["maintenance_db"].(string); ok {
				maintenanceDB = db
			}

			description := ""
			if desc, ok := args["description"].(string); ok {
				description = desc
			}

			// At least one field must be provided
			if connString == "" && maintenanceDB == "" && description == "" {
				return mcp.NewToolError("Error: At least one field (connection_string, maintenance_db, or description) must be provided")
			}

			// Get connection store
			ctx := context.Background()
			store, err := connMgr.GetConnectionStore(ctx)
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Error: %v", err))
			}

			// Update connection
			if err := store.Update(alias, connString, maintenanceDB, description); err != nil {
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
