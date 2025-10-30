/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"fmt"
	"os"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/llm"
	"pgedge-postgres-mcp/internal/mcp"
	"pgedge-postgres-mcp/internal/resources"
	"pgedge-postgres-mcp/internal/tools"
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

	// Register resources first (so they can be used by tools)
	resourceRegistry := resources.NewRegistry()

	// System information resources
	resourceRegistry.Register(resources.URISettings, resources.PGSettingsResource(dbClient))
	resourceRegistry.Register(resources.URISystemInfo, resources.PGSystemInfoResource(dbClient))

	// Statistics resources
	resourceRegistry.Register(resources.URIStatActivity, resources.PGStatActivityResource(dbClient))
	resourceRegistry.Register(resources.URIStatDatabase, resources.PGStatDatabaseResource(dbClient))
	resourceRegistry.Register(resources.URIStatUserTables, resources.PGStatUserTablesResource(dbClient))
	resourceRegistry.Register(resources.URIStatUserIndexes, resources.PGStatUserIndexesResource(dbClient))
	resourceRegistry.Register(resources.URIStatReplication, resources.PGStatReplicationResource(dbClient))
	resourceRegistry.Register(resources.URIStatBgwriter, resources.PGStatBgwriterResource(dbClient))
	resourceRegistry.Register(resources.URIStatWAL, resources.PGStatWALResource(dbClient))

	// Register tools
	toolRegistry := tools.NewRegistry()
	toolRegistry.Register("query_database", tools.QueryDatabaseTool(dbClient, llmClient))
	toolRegistry.Register("get_schema_info", tools.GetSchemaInfoTool(dbClient))
	toolRegistry.Register("set_pg_configuration", tools.SetPGConfigurationTool(dbClient))
	toolRegistry.Register("recommend_pg_configuration", tools.RecommendPGConfigurationTool())
	toolRegistry.Register("analyze_bloat", tools.AnalyzeBloatTool(dbClient))
	toolRegistry.Register("read_server_log", tools.ReadServerLogTool(dbClient))
	toolRegistry.Register("read_postgresql_conf", tools.ReadPostgresqlConfTool(dbClient))
	toolRegistry.Register("read_pg_hba_conf", tools.ReadPgHbaConfTool(dbClient))
	toolRegistry.Register("read_pg_ident_conf", tools.ReadPgIdentConfTool(dbClient))
	toolRegistry.Register("read_resource", tools.ReadResourceTool(resourceRegistry))

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
