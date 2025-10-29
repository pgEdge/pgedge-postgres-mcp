/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL Licence
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"fmt"
	"os"

	"pgedge-mcp/internal/database"
	"pgedge-mcp/internal/llm"
	"pgedge-mcp/internal/mcp"
	"pgedge-mcp/internal/resources"
	"pgedge-mcp/internal/tools"
)

func main() {
	// Create clients
	dbClient := database.NewClient()
	llmClient := llm.NewClient()

	// Initialize database in background
	go func() {
		if err := dbClient.Connect(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to connect to database: %v\n", err)
			return
		}

		if err := dbClient.LoadMetadata(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to load database metadata: %v\n", err)
			return
		}

		fmt.Fprintf(os.Stderr, "Database ready: %d tables/views loaded\n", len(dbClient.GetMetadata()))
	}()

	// Register tools
	toolRegistry := tools.NewRegistry()
	toolRegistry.Register("query_database", tools.QueryDatabaseTool(dbClient, llmClient))
	toolRegistry.Register("get_schema_info", tools.GetSchemaInfoTool(dbClient))
	toolRegistry.Register("set_pg_configuration", tools.SetPGConfigurationTool(dbClient))

	// Register resources
	resourceRegistry := resources.NewRegistry()
	resourceRegistry.Register("pg://settings", resources.PGSettingsResource(dbClient))

	// Start MCP server
	server := mcp.NewServer(toolRegistry)
	server.SetResourceProvider(resourceRegistry)

	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	// Cleanup
	dbClient.Close()
}
