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
	"fmt"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/mcp"
)

// SetDatabaseConnectionTool creates a tool for setting the database connection at runtime
func SetDatabaseConnectionTool(clientManager *database.ClientManager) Tool {
	return Tool{
		Definition: mcp.Tool{
			Name:        "set_database_connection",
			Description: "Set the PostgreSQL database connection string for this session. This must be called before using any database-dependent tools.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"connection_string": map[string]interface{}{
						"type":        "string",
						"description": "PostgreSQL connection string in the format: postgres://username:password@host:port/database?options",
					},
				},
				Required: []string{"connection_string"},
			},
		},
		Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
			connStr, ok := args["connection_string"].(string)
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

			if connStr == "" {
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
