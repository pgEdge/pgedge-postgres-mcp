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
	"strings"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// SetDatabaseConnectionTool creates a tool for setting the database connection at runtime
// Now supports both connection strings and aliases to saved connections
func SetDatabaseConnectionTool(clientManager *database.ClientManager, connMgr *ConnectionManager, configPath string) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "set_database_connection",
			Description: "Set the PostgreSQL database connection for this session. You can provide either a full connection string OR an alias to a saved connection. This must be called before using any database-dependent tools. Examples: 'production', 'postgres://user:pass@host/db'",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"connection_string": map[string]interface{}{
						"type":        "string",
						"description": "PostgreSQL connection string OR alias to a saved connection. If using an alias, the saved connection will be retrieved and used. Format: postgres://username:password@host:port/database?options OR saved alias name (e.g., 'production', 'staging')",
					},
				},
				Required: []string{"connection_string"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			connStrOrAlias, ok := args["connection_string"].(string)
			if !ok {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Error: connection_string must be a string",
						},
					},
					IsError: true,
				}, nil
			}

			if connStrOrAlias == "" {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: "Error: connection_string cannot be empty",
						},
					},
					IsError: true,
				}, nil
			}

			connStr := connStrOrAlias
			alias := ""

			// Check if this looks like an alias (no postgres:// prefix)
			if !strings.HasPrefix(connStrOrAlias, "postgres://") && !strings.HasPrefix(connStrOrAlias, "postgresql://") {
				// Try to resolve as alias
				ctx := context.Background()
				store, err := connMgr.GetConnectionStore(ctx)
				if err == nil {
					savedConn, err := store.Get(connStrOrAlias)
					if err == nil {
						// Found saved connection
						connStr = savedConn.ConnectionString
						alias = savedConn.Alias

						// Mark as used (ignore errors as this is non-critical metadata)
						if err := store.MarkUsed(alias); err == nil {
							//nolint:errcheck // Ignore save error as connection can still proceed
							connMgr.saveChanges(configPath)
						}
					}
				}
			}

			// Create a new client with the provided connection string
			client := database.NewClientWithConnectionString(connStr)

			// Test the connection
			if err := client.Connect(); err != nil {
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error: Failed to connect to database: %v", err),
						},
					},
					IsError: true,
				}, nil
			}

			// Load metadata
			if err := client.LoadMetadata(); err != nil {
				client.Close()
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error: Failed to load database metadata: %v", err),
						},
					},
					IsError: true,
				}, nil
			}

			// Set as the default client for this session
			// For stdio mode, use a fixed key
			// For HTTP mode with auth, the context-aware provider will use the token hash
			if err := clientManager.SetClient("default", client); err != nil {
				client.Close()
				return mcp.ToolResponse{
					Content: []mcp.ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Error: Failed to set database connection: %v", err),
						},
					},
					IsError: true,
				}, nil
			}

			metadata := client.GetMetadata()
			return mcp.ToolResponse{
				Content: []mcp.ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Successfully connected to database. Loaded metadata for %d tables/views.", len(metadata)),
					},
				},
			}, nil
		},
	}
}
