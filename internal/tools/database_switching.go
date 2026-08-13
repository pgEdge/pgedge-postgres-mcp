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
	"encoding/json"
	"fmt"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// getLLMAccessibleDatabases returns databases accessible for LLM switching
func getLLMAccessibleDatabases(
	ctx context.Context,
	clientManager *database.ClientManager,
	accessChecker *auth.DatabaseAccessChecker,
) []config.NamedDatabaseConfig {
	allConfigs := clientManager.GetDatabaseConfigs()

	var accessibleConfigs []config.NamedDatabaseConfig
	if accessChecker != nil {
		accessibleConfigs = accessChecker.GetAccessibleDatabases(ctx, allConfigs)
	} else {
		accessibleConfigs = allConfigs
	}

	var llmAccessible []config.NamedDatabaseConfig
	for i := range accessibleConfigs {
		if accessibleConfigs[i].IsAllowedForLLMSwitching() {
			llmAccessible = append(llmAccessible, accessibleConfigs[i])
		}
	}
	return llmAccessible
}

// getEffectiveCurrentDB returns the current database if it's in the accessible list.
// Returns empty string if current DB is not accessible to avoid misrepresenting state.
func getEffectiveCurrentDB(tokenHash string, clientManager *database.ClientManager, accessible []config.NamedDatabaseConfig) string {
	current := clientManager.GetCurrentDatabase(tokenHash)
	if current == "" {
		current = clientManager.GetDefaultDatabaseName()
	}

	for i := range accessible {
		if accessible[i].Name == current {
			return current
		}
	}

	// Don't report a different current DB than the session uses
	return ""
}

// populateHostFields adds host connection fields to the response map.
// For multi-host configs, it sets host/port from the first configured entry
// and includes the full hosts array. For single-host, it uses Host and Port directly.
// effectivePort returns the port or 5432 if zero, matching
// BuildConnectionString behavior when YAML omits the port.
func effectivePort(port int) int {
	if port == 0 {
		return 5432
	}
	return port
}

func populateHostFields(entry map[string]interface{}, cfg *config.NamedDatabaseConfig) {
	if len(cfg.Hosts) > 0 {
		entry["host"] = cfg.Hosts[0].Host
		entry["port"] = effectivePort(cfg.Hosts[0].Port)
		hostsList := make([]map[string]interface{}, len(cfg.Hosts))
		for j, h := range cfg.Hosts {
			hostsList[j] = map[string]interface{}{
				"host": h.Host,
				"port": effectivePort(h.Port),
			}
		}
		entry["hosts"] = hostsList
		if cfg.TargetSessionAttrs != "" {
			entry["target_session_attrs"] = cfg.TargetSessionAttrs
		}
	} else {
		entry["host"] = cfg.Host
		entry["port"] = effectivePort(cfg.Port)
	}
}

// buildDatabaseListResponse builds the JSON response for list_database_connections.
// The statusFunc, when non-nil, is called with each database name to determine
// its connection status ("connected" or "unavailable").
func buildDatabaseListResponse(
	databases []config.NamedDatabaseConfig,
	current string,
	statusFunc func(dbName string) string,
) (mcp.ToolResponse, error) {
	dbList := make([]map[string]interface{}, 0, len(databases))
	for i := range databases {
		entry := map[string]interface{}{
			"name":         databases[i].Name,
			"database":     databases[i].Database,
			"allow_writes": databases[i].AllowWrites,
		}

		populateHostFields(entry, &databases[i])

		if statusFunc != nil {
			entry["status"] = statusFunc(databases[i].Name)
		}
		dbList = append(dbList, entry)
	}

	result := map[string]interface{}{"databases": dbList, "current": current}
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolError(fmt.Sprintf("Failed to marshal response: %v", err))
	}
	return mcp.NewToolSuccess(string(jsonBytes))
}

// ListDatabaseConnectionsTool creates a tool for listing available database connections
// This tool is only available when llm_connection_selection is enabled
func ListDatabaseConnectionsTool(
	clientManager *database.ClientManager,
	accessChecker *auth.DatabaseAccessChecker,
	cfg *config.Config,
) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "list_database_connections",
			Description: `List available database connections that you can switch to. Use this tool instead of checking connection parameters manually — it shows all configured databases with their status.

Returns a list of database connections configured for this session, along with
the currently active database. Use this to discover what databases are available
before using select_database_connection to switch.

Each database entry includes:
- name: The connection name (use this with select_database_connection)
- database: The PostgreSQL database name
- host: Database server hostname (first configured host for multi-host configs)
- port: Database server port number (first configured port for multi-host configs)
- hosts: (optional) Array of host/port pairs for multi-host configs
- target_session_attrs: (optional) Session routing for multi-host
- allow_writes: Whether write operations are permitted
- status: Connection status ("connected" or "unavailable")

The response includes which database is currently active. Databases with
status "unavailable" will be connected on demand when selected.`,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
				Required:   []string{},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			ctx := extractContextFromArgs(args)
			tokenHash := auth.GetTokenHashFromContext(ctx)
			if tokenHash == "" {
				tokenHash = "default"
			}

			llmAccessible := getLLMAccessibleDatabases(ctx, clientManager, accessChecker)
			current := getEffectiveCurrentDB(tokenHash, clientManager, llmAccessible)

			statusFunc := func(dbName string) string {
				if clientManager.IsConnected(tokenHash, dbName) {
					return "connected"
				}
				return "unavailable"
			}

			return buildDatabaseListResponse(llmAccessible, current, statusFunc)
		},
	}
}

// SelectDatabaseConnectionTool creates a tool for switching database connections
// This tool is only available when llm_connection_selection is enabled
func SelectDatabaseConnectionTool(
	clientManager *database.ClientManager,
	accessChecker *auth.DatabaseAccessChecker,
	cfg *config.Config,
) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name: "select_database_connection",
			Description: `Switch to a different database connection for subsequent queries. Use this tool instead of reconnecting through psql — it manages connection lifecycle and preserves session state.

Use list_database_connections first to see available options. After switching,
all subsequent database tools (query_database, get_schema_info, etc.) will
operate on the newly selected database.

IMPORTANT: Switching databases may change available schemas, tables, and
permissions. Consider re-examining the schema after switching.`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the database connection to switch to (from list_database_connections)",
					},
				},
				Required: []string{"name"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			// Extract context from args (injected by registry)
			ctx := extractContextFromArgs(args)

			// Get the database name parameter
			name, ok := args["name"].(string)
			if !ok || name == "" {
				return mcp.NewToolError("Missing or invalid 'name' parameter")
			}

			// Get token hash for session identification
			tokenHash := auth.GetTokenHashFromContext(ctx)
			if tokenHash == "" {
				// STDIO mode uses "default" as the session key
				tokenHash = "default"
			}

			// Get database config
			// Use consistent error message to prevent information disclosure
			// (don't reveal whether database exists but is inaccessible)
			dbConfig := cfg.GetDatabaseByName(name)
			if dbConfig == nil {
				return mcp.NewToolError(fmt.Sprintf("Access denied to database '%s'", name))
			}

			// Check user access control
			if accessChecker != nil {
				// For API tokens, enforce database binding
				if auth.IsAPITokenFromContext(ctx) {
					boundDB := accessChecker.GetBoundDatabase(ctx)
					if boundDB != "" && boundDB != name {
						return mcp.NewToolError(fmt.Sprintf("Access denied to database '%s'", name))
					}
				} else if !accessChecker.CanAccessDatabase(ctx, dbConfig) {
					// For session users, check available_to_users
					return mcp.NewToolError(fmt.Sprintf("Access denied to database '%s'", name))
				}
			}

			// Check LLM accessibility (allow_llm_switching)
			if !dbConfig.IsAllowedForLLMSwitching() {
				return mcp.NewToolError(fmt.Sprintf("Access denied to database '%s'", name))
			}

			// Connect before committing the switch, as documented, rather
			// than leaving the database "unavailable" in
			// list_database_connections until some other tool happens to
			// use it: a switch that reports success should leave the
			// target database actually connected. Connecting first, before
			// changing the current database, also means a switch to an
			// unreachable database fails outright and leaves the previous
			// selection in place instead of committing to a database that
			// isn't there.
			if _, err := clientManager.GetClientForDatabase(tokenHash, name); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to connect to database '%s': %v", name, err))
			}

			if err := clientManager.SetCurrentDatabase(tokenHash, name); err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to switch database: %v", err))
			}

			// Build success response
			result := map[string]interface{}{
				"success":      true,
				"message":      fmt.Sprintf("Switched to database: %s", name),
				"current":      name,
				"database":     dbConfig.Database,
				"allow_writes": dbConfig.AllowWrites,
			}

			populateHostFields(result, dbConfig)

			jsonBytes, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return mcp.NewToolError(fmt.Sprintf("Failed to marshal response: %v", err))
			}

			return mcp.NewToolSuccess(string(jsonBytes))
		},
	}
}

// extractContextFromArgs extracts context.Context from tool args
func extractContextFromArgs(args map[string]interface{}) context.Context {
	if ctxRaw, ok := args["__context"]; ok {
		if ctx, ok := ctxRaw.(context.Context); ok {
			return ctx
		}
	}
	return context.Background()
}
