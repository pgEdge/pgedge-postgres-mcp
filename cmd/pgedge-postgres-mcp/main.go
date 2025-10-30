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
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/llm"
	"pgedge-postgres-mcp/internal/mcp"
	"pgedge-postgres-mcp/internal/resources"
	"pgedge-postgres-mcp/internal/tools"
)

func main() {
	// Command line flags
	httpMode := flag.Bool("http", false, "Enable HTTP transport mode (default: stdio)")
	httpAddr := flag.String("addr", ":8080", "HTTP server address (requires -http)")
	httpsMode := flag.Bool("https", false, "Enable HTTPS (requires -http)")
	certFile := flag.String("cert", "", "Path to TLS certificate file (requires -http and -https, default: ./server.crt)")
	keyFile := flag.String("key", "", "Path to TLS key file (requires -http and -https, default: ./server.key)")
	chainFile := flag.String("chain", "", "Path to TLS certificate chain file (optional, requires -http and -https)")

	flag.Parse()

	// Validate flags
	if !*httpMode && (*httpsMode || *certFile != "" || *keyFile != "" || *chainFile != "") {
		fmt.Fprintf(os.Stderr, "ERROR: TLS options (-https, -cert, -key, -chain) require -http flag\n")
		flag.Usage()
		os.Exit(1)
	}

	if *httpMode && *httpsMode {
		// Set default certificate paths if not provided
		if *certFile == "" {
			execPath, err := os.Executable()
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to get executable path: %v\n", err)
				os.Exit(1)
			}
			execDir := filepath.Dir(execPath)
			*certFile = filepath.Join(execDir, "server.crt")
		}

		if *keyFile == "" {
			execPath, err := os.Executable()
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to get executable path: %v\n", err)
				os.Exit(1)
			}
			execDir := filepath.Dir(execPath)
			*keyFile = filepath.Join(execDir, "server.key")
		}

		// Verify certificate and key files exist
		if _, err := os.Stat(*certFile); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Certificate file not found: %s\n", *certFile)
			os.Exit(1)
		}
		if _, err := os.Stat(*keyFile); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Key file not found: %s\n", *keyFile)
			os.Exit(1)
		}

		// Verify chain file if provided
		if *chainFile != "" {
			if _, err := os.Stat(*chainFile); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Chain file not found: %s\n", *chainFile)
				os.Exit(1)
			}
		}
	}

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

	var err error
	if *httpMode {
		// HTTP/HTTPS mode
		config := &mcp.HTTPConfig{
			Addr:      *httpAddr,
			TLSEnable: *httpsMode,
			CertFile:  *certFile,
			KeyFile:   *keyFile,
			ChainFile: *chainFile,
		}

		if *httpsMode {
			fmt.Fprintf(os.Stderr, "Starting MCP server in HTTPS mode on %s\n", *httpAddr)
			fmt.Fprintf(os.Stderr, "Certificate: %s\n", *certFile)
			fmt.Fprintf(os.Stderr, "Key: %s\n", *keyFile)
			if *chainFile != "" {
				fmt.Fprintf(os.Stderr, "Chain: %s\n", *chainFile)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Starting MCP server in HTTP mode on %s\n", *httpAddr)
		}

		err = server.RunHTTP(config)
	} else {
		// Default stdio mode
		err = server.Run()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	// Cleanup
	dbClient.Close()
}
