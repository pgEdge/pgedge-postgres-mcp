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

	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/llm"
	"pgedge-postgres-mcp/internal/mcp"
	"pgedge-postgres-mcp/internal/resources"
	"pgedge-postgres-mcp/internal/tools"
)

func main() {
	// Get executable path for default config location
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to get executable path: %v\n", err)
		os.Exit(1)
	}
	defaultConfigPath := config.GetDefaultConfigPath(execPath)

	// Command line flags
	configFile := flag.String("config", defaultConfigPath, "Path to configuration file")
	dbConnString := flag.String("db", "", "PostgreSQL connection string (overrides config file)")
	apiKey := flag.String("api-key", "", "Anthropic API key (overrides config file)")
	model := flag.String("model", "", "Anthropic model to use (overrides config file)")
	httpMode := flag.Bool("http", false, "Enable HTTP transport mode (default: stdio)")
	httpAddr := flag.String("addr", "", "HTTP server address")
	httpsMode := flag.Bool("https", false, "Enable HTTPS (requires -http)")
	certFile := flag.String("cert", "", "Path to TLS certificate file")
	keyFile := flag.String("key", "", "Path to TLS key file")
	chainFile := flag.String("chain", "", "Path to TLS certificate chain file (optional)")

	flag.Parse()

	// Track which flags were explicitly set
	cliFlags := config.CLIFlags{}
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "config":
			cliFlags.ConfigFileSet = true
			cliFlags.ConfigFile = *configFile
		case "db":
			cliFlags.ConnectionStringSet = true
			cliFlags.ConnectionString = *dbConnString
		case "api-key":
			cliFlags.APIKeySet = true
			cliFlags.APIKey = *apiKey
		case "model":
			cliFlags.ModelSet = true
			cliFlags.Model = *model
		case "http":
			cliFlags.HTTPEnabledSet = true
			cliFlags.HTTPEnabled = *httpMode
		case "addr":
			cliFlags.HTTPAddrSet = true
			cliFlags.HTTPAddr = *httpAddr
		case "https":
			cliFlags.TLSEnabledSet = true
			cliFlags.TLSEnabled = *httpsMode
		case "cert":
			cliFlags.TLSCertSet = true
			cliFlags.TLSCertFile = *certFile
		case "key":
			cliFlags.TLSKeySet = true
			cliFlags.TLSKeyFile = *keyFile
		case "chain":
			cliFlags.TLSChainSet = true
			cliFlags.TLSChainFile = *chainFile
		}
	})

	// Validate basic flag dependencies before loading full config
	if !*httpMode && (*httpsMode || *certFile != "" || *keyFile != "" || *chainFile != "") {
		fmt.Fprintf(os.Stderr, "ERROR: TLS options (-https, -cert, -key, -chain) require -http flag\n")
		flag.Usage()
		os.Exit(1)
	}

	// Determine which config file to load
	configPath := *configFile
	if !cliFlags.ConfigFileSet {
		// Check if default config exists
		if config.ConfigFileExists(defaultConfigPath) {
			configPath = defaultConfigPath
		} else {
			// No config file, rely on env vars and CLI flags
			configPath = ""
		}
	}

	// Load configuration
	cfg, err := config.LoadConfig(configPath, cliFlags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	// Set environment variables for clients that read them directly
	// This ensures backward compatibility
	if err := os.Setenv("POSTGRES_CONNECTION_STRING", cfg.Database.ConnectionString); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to set environment variable: %v\n", err)
		os.Exit(1)
	}
	if cfg.Anthropic.APIKey != "" {
		if err := os.Setenv("ANTHROPIC_API_KEY", cfg.Anthropic.APIKey); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to set environment variable: %v\n", err)
			os.Exit(1)
		}
	}
	if cfg.Anthropic.Model != "" {
		if err := os.Setenv("ANTHROPIC_MODEL", cfg.Anthropic.Model); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to set environment variable: %v\n", err)
			os.Exit(1)
		}
	}

	// Verify TLS files exist if HTTPS is enabled
	if cfg.HTTP.TLS.Enabled {
		if _, err := os.Stat(cfg.HTTP.TLS.CertFile); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Certificate file not found: %s\n", cfg.HTTP.TLS.CertFile)
			os.Exit(1)
		}
		if _, err := os.Stat(cfg.HTTP.TLS.KeyFile); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Key file not found: %s\n", cfg.HTTP.TLS.KeyFile)
			os.Exit(1)
		}
		if cfg.HTTP.TLS.ChainFile != "" {
			if _, err := os.Stat(cfg.HTTP.TLS.ChainFile); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Chain file not found: %s\n", cfg.HTTP.TLS.ChainFile)
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

	if cfg.HTTP.Enabled {
		// HTTP/HTTPS mode
		httpConfig := &mcp.HTTPConfig{
			Addr:      cfg.HTTP.Address,
			TLSEnable: cfg.HTTP.TLS.Enabled,
			CertFile:  cfg.HTTP.TLS.CertFile,
			KeyFile:   cfg.HTTP.TLS.KeyFile,
			ChainFile: cfg.HTTP.TLS.ChainFile,
		}

		if cfg.HTTP.TLS.Enabled {
			fmt.Fprintf(os.Stderr, "Starting MCP server in HTTPS mode on %s\n", cfg.HTTP.Address)
			fmt.Fprintf(os.Stderr, "Certificate: %s\n", cfg.HTTP.TLS.CertFile)
			fmt.Fprintf(os.Stderr, "Key: %s\n", cfg.HTTP.TLS.KeyFile)
			if cfg.HTTP.TLS.ChainFile != "" {
				fmt.Fprintf(os.Stderr, "Chain: %s\n", cfg.HTTP.TLS.ChainFile)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Starting MCP server in HTTP mode on %s\n", cfg.HTTP.Address)
		}

		err = server.RunHTTP(httpConfig)
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
