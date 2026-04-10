/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"pgedge-postgres-mcp/internal/api"
	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/compactor"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/conversations"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/definitions"
	"pgedge-postgres-mcp/internal/llmproxy"
	"pgedge-postgres-mcp/internal/mcp"
	"pgedge-postgres-mcp/internal/openapi"
	"pgedge-postgres-mcp/internal/prompts"
	"pgedge-postgres-mcp/internal/resources"
	"pgedge-postgres-mcp/internal/tools"
	"pgedge-postgres-mcp/internal/tracing"
)

const (
	// Token cleanup configuration
	tokenCleanupInterval = 5 * time.Minute  // How often to check for expired tokens
	tokenCleanupTimeout  = 30 * time.Second // Max time allowed for cleanup operations
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
	httpMode := flag.Bool("http", false, "Enable HTTP transport mode (default: stdio)")
	httpAddr := flag.String("addr", "", "HTTP server address")
	tlsMode := flag.Bool("tls", false, "Enable TLS/HTTPS (requires -http)")
	certFile := flag.String("cert", "", "Path to TLS certificate file")
	keyFile := flag.String("key", "", "Path to TLS key file")
	chainFile := flag.String("chain", "", "Path to TLS certificate chain file (optional)")
	noAuth := flag.Bool("no-auth", false, "Disable API token authentication in HTTP mode")
	debug := flag.Bool("debug", false, "Enable debug logging (logs HTTP requests/responses)")
	traceFile := flag.String("trace-file", "", "Path to trace output file (JSONL format)")
	tokenFilePath := flag.String("token-file", "", "Path to API token file")
	showOpenAPI := flag.Bool("openapi", false, "Output OpenAPI specification as JSON and exit")

	// Database connection flags
	dbHost := flag.String("db-host", "", "Database host")
	dbPort := flag.Int("db-port", 0, "Database port")
	dbName := flag.String("db-name", "", "Database name")
	dbUser := flag.String("db-user", "", "Database user")
	dbPassword := flag.String("db-password", "", "Database password")
	dbSSLMode := flag.String("db-sslmode", "", "Database SSL mode (disable, require, verify-ca, verify-full)")
	dbHosts := flag.String("db-hosts", "", "Comma-separated host:port pairs for multi-host connections (e.g., \"host1:5432,host2:5433\")")
	dbTargetSessionAttrs := flag.String("db-target-session-attrs", "", "Target session attributes for multi-host (e.g., \"read-write\", \"any\", \"primary\", \"standby\")")

	// Token management commands
	addTokenCmd := flag.Bool("add-token", false, "Add a new API token")
	removeTokenCmd := flag.String("remove-token", "", "Remove an API token by ID or hash prefix")
	listTokensCmd := flag.Bool("list-tokens", false, "List all API tokens")
	tokenNote := flag.String("token-note", "", "Annotation for the new token (used with -add-token)")
	tokenExpiry := flag.String("token-expiry", "", "Token expiry duration: '30d', '1y', '2w', '12h', 'never' (used with -add-token)")
	tokenDatabase := flag.String("token-database", "", "Bind token to specific database name (used with -add-token, empty = first configured database)")

	// User management commands
	userFilePath := flag.String("user-file", "", "Path to user file")
	addUserCmd := flag.Bool("add-user", false, "Add a new user")
	updateUserCmd := flag.Bool("update-user", false, "Update an existing user")
	deleteUserCmd := flag.Bool("delete-user", false, "Delete a user")
	listUsersCmd := flag.Bool("list-users", false, "List all users")
	enableUserCmd := flag.Bool("enable-user", false, "Enable a user account")
	disableUserCmd := flag.Bool("disable-user", false, "Disable a user account")
	username := flag.String("username", "", "Username for user management commands")
	userPassword := flag.String("password", "", "Password for user management commands (prompted if not provided)")
	userNote := flag.String("user-note", "", "Annotation for the new user (used with -add-user)")

	flag.Parse()

	// Reject unexpected positional arguments
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments: %s\nRun with -help for usage information.\n", strings.Join(flag.Args(), " "))
		os.Exit(1)
	}

	// Handle -openapi flag: output specification and exit
	if *showOpenAPI {
		spec := openapi.BuildSpec()
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(spec); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to encode OpenAPI spec: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle token management commands
	if *addTokenCmd || *removeTokenCmd != "" || *listTokensCmd {
		configPath := *configFile
		defaultTokenPath := auth.GetDefaultTokenPath(execPath)
		tokenFile := *tokenFilePath

		// Determine if we need to load config:
		// - add-token always needs config for database names
		// - remove-token and list-tokens need config only if token-file not specified
		needsConfig := *addTokenCmd || tokenFile == ""

		var cfg *config.Config
		if needsConfig {
			// Require config file to exist
			if !config.ConfigFileExists(configPath) {
				fmt.Fprintf(os.Stderr, "ERROR: Configuration file not found: %s\n", configPath)
				fmt.Fprintf(os.Stderr, "Specify your configuration file with the -config flag:\n")
				fmt.Fprintf(os.Stderr, "  %s -config <path-to-config.yaml> -add-token\n", os.Args[0])
				if !*addTokenCmd {
					// For remove/list commands, also offer the -token-file option
					fmt.Fprintf(os.Stderr, "Or specify the token file directly with -token-file:\n")
					fmt.Fprintf(os.Stderr, "  %s -list-tokens -token-file <path-to-tokens.json>\n", os.Args[0])
				}
				os.Exit(1)
			}

			var loadErr error
			cfg, loadErr = config.LoadConfig(configPath, config.CLIFlags{})
			if loadErr != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", loadErr)
				os.Exit(1)
			}

			// Determine token file path from config if not specified via CLI
			if tokenFile == "" {
				if cfg.HTTP.Auth.TokenFile != "" {
					tokenFile = cfg.HTTP.Auth.TokenFile
				} else {
					tokenFile = defaultTokenPath
				}
			}
		}
		// If needsConfig is false, tokenFile was explicitly provided via CLI

		if *addTokenCmd {
			var expiry time.Duration
			switch {
			case *tokenExpiry != "" && *tokenExpiry != "never":
				var err error
				expiry, err = parseDuration(*tokenExpiry)
				if err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: Invalid expiry duration: %v\n", err)
					os.Exit(1)
				}
			case *tokenExpiry == "":
				expiry = 0 // Will prompt user
			default:
				expiry = -1 // Never expires
			}

			// Get database names for selection (config already loaded above)
			if len(cfg.Databases) == 0 {
				fmt.Fprintf(os.Stderr, "ERROR: No databases configured in %s\n", configPath)
				fmt.Fprintf(os.Stderr, "Add at least one database configuration before creating tokens.\n")
				os.Exit(1)
			}

			var availableDatabases []string
			for i := range cfg.Databases {
				availableDatabases = append(availableDatabases, cfg.Databases[i].Name)
			}

			if err := addTokenCommand(tokenFile, *tokenNote, *tokenDatabase, expiry, availableDatabases); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if *removeTokenCmd != "" {
			if err := removeTokenCommand(tokenFile, *removeTokenCmd); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if *listTokensCmd {
			if err := listTokensCommand(tokenFile); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// Handle user management commands
	if *addUserCmd || *updateUserCmd || *deleteUserCmd || *listUsersCmd || *enableUserCmd || *disableUserCmd {
		configPath := *configFile
		defaultUserPath := auth.GetDefaultUserPath(execPath)
		userFile := *userFilePath

		// Only require config file if user-file was not explicitly provided
		if userFile == "" {
			// Need config file to get user file path
			if !config.ConfigFileExists(configPath) {
				fmt.Fprintf(os.Stderr, "ERROR: Configuration file not found: %s\n", configPath)
				fmt.Fprintf(os.Stderr, "Specify your configuration file with the -config flag:\n")
				fmt.Fprintf(os.Stderr, "  %s -config <path-to-config.yaml> -add-user\n", os.Args[0])
				fmt.Fprintf(os.Stderr, "Or specify the user file directly with -user-file:\n")
				fmt.Fprintf(os.Stderr, "  %s -add-user -user-file <path-to-users.json>\n", os.Args[0])
				os.Exit(1)
			}

			cfg, loadErr := config.LoadConfig(configPath, config.CLIFlags{})
			if loadErr != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to load configuration: %v\n", loadErr)
				os.Exit(1)
			}

			// Determine user file path from config
			if cfg.HTTP.Auth.UserFile != "" {
				userFile = cfg.HTTP.Auth.UserFile
			} else {
				userFile = defaultUserPath
			}
		}
		// else: userFile was explicitly provided via CLI, use it directly

		if *addUserCmd {
			if err := addUserCommand(userFile, *username, *userPassword, *userNote); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if *updateUserCmd {
			if err := updateUserCommand(userFile, *username, *userPassword, *userNote); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if *deleteUserCmd {
			if err := deleteUserCommand(userFile, *username); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if *listUsersCmd {
			if err := listUsersCommand(userFile); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if *enableUserCmd {
			if err := enableUserCommand(userFile, *username); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if *disableUserCmd {
			if err := disableUserCommand(userFile, *username); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// Track which flags were explicitly set
	cliFlags := config.CLIFlags{}
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "config":
			cliFlags.ConfigFileSet = true
			cliFlags.ConfigFile = *configFile
		case "http":
			cliFlags.HTTPEnabledSet = true
			cliFlags.HTTPEnabled = *httpMode
		case "addr":
			cliFlags.HTTPAddrSet = true
			cliFlags.HTTPAddr = *httpAddr
		case "tls":
			cliFlags.TLSEnabledSet = true
			cliFlags.TLSEnabled = *tlsMode
		case "cert":
			cliFlags.TLSCertSet = true
			cliFlags.TLSCertFile = *certFile
		case "key":
			cliFlags.TLSKeySet = true
			cliFlags.TLSKeyFile = *keyFile
		case "chain":
			cliFlags.TLSChainSet = true
			cliFlags.TLSChainFile = *chainFile
		case "no-auth":
			cliFlags.AuthEnabledSet = true
			cliFlags.AuthEnabled = !*noAuth // Invert because it's "no-auth"
		case "token-file":
			cliFlags.AuthTokenSet = true
			cliFlags.AuthTokenFile = *tokenFilePath
		case "user-file":
			cliFlags.AuthUserSet = true
			cliFlags.AuthUserFile = *userFilePath
		case "db-host":
			cliFlags.DBHostSet = true
			cliFlags.DBHost = *dbHost
		case "db-port":
			cliFlags.DBPortSet = true
			cliFlags.DBPort = *dbPort
		case "db-name":
			cliFlags.DBNameSet = true
			cliFlags.DBName = *dbName
		case "db-user":
			cliFlags.DBUserSet = true
			cliFlags.DBUser = *dbUser
		case "db-password":
			cliFlags.DBPassSet = true
			cliFlags.DBPassword = *dbPassword
		case "db-sslmode":
			cliFlags.DBSSLSet = true
			cliFlags.DBSSLMode = *dbSSLMode
		case "db-hosts":
			cliFlags.DBHostsSet = true
			cliFlags.DBHosts = *dbHosts
		case "db-target-session-attrs":
			cliFlags.DBTargetSessionAttrsSet = true
			cliFlags.DBTargetSessionAttrs = *dbTargetSessionAttrs
		case "trace-file":
			cliFlags.TraceFileSet = true
			cliFlags.TraceFile = *traceFile
		}
	})

	// Validate basic flag dependencies before loading full config
	if !*httpMode && (*tlsMode || *certFile != "" || *keyFile != "" || *chainFile != "") {
		fmt.Fprintf(os.Stderr, "ERROR: TLS options (-tls, -cert, -key, -chain) require -http flag\n")
		flag.Usage()
		os.Exit(1)
	}

	// Validate mutual exclusion of --db-host and --db-hosts
	if cliFlags.DBHostSet && cliFlags.DBHostsSet {
		fmt.Fprintf(os.Stderr, "ERROR: -db-host and -db-hosts are mutually exclusive\n")
		flag.Usage()
		os.Exit(1)
	}

	// Determine which config file to load and save to
	configPath := *configFile
	if !cliFlags.ConfigFileSet {
		// Use default config path (will be created if needed for saving connections)
		configPath = defaultConfigPath
	}

	// For loading, only attempt to load if file exists
	configPathForLoad := ""
	if config.ConfigFileExists(configPath) {
		configPathForLoad = configPath
	}

	// Load configuration (empty path means no config file, will use env vars and defaults)
	cfg, err := config.LoadConfig(configPathForLoad, cliFlags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	// Set default token file path if not specified and HTTP is enabled
	if cfg.HTTP.Enabled && cfg.HTTP.Auth.TokenFile == "" {
		cfg.HTTP.Auth.TokenFile = auth.GetDefaultTokenPath(execPath)
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

	// Initialize tracing if configured
	if cfg.TraceFile != "" {
		if err := tracing.Initialize(cfg.TraceFile); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to initialize tracing: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Tracing: ENABLED (%s)\n", cfg.TraceFile)
		}
		defer tracing.Close()
	}

	// Load token store if HTTP auth is enabled
	var tokenStore *auth.TokenStore
	var userStore *auth.UserStore
	userFilePathForTools := ""
	if cfg.HTTP.Enabled && cfg.HTTP.Auth.Enabled {
		if _, err := os.Stat(cfg.HTTP.Auth.TokenFile); os.IsNotExist(err) {
			// Token file doesn't exist - create empty store
			// Tokens can be added via CLI commands
			tokenStore = auth.InitializeTokenStore()
			fmt.Fprintf(os.Stderr, "Token file not found, initialized empty token store\n")
		} else {
			tokenStore, err = auth.LoadTokenStore(cfg.HTTP.Auth.TokenFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to load token file: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Loaded %d API token(s) from %s\n", len(tokenStore.Tokens), cfg.HTTP.Auth.TokenFile)

			// Start watching the token file for changes
			if err := tokenStore.StartWatching(); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Failed to start watching token file: %v\n", err)
				fmt.Fprintf(os.Stderr, "         Token changes will require server restart\n")
			} else {
				fmt.Fprintf(os.Stderr, "Watching %s for changes\n", cfg.HTTP.Auth.TokenFile)
			}
		}

		// Load user store for user authentication
		// Use config value if set (from config file, env var, or CLI flag), otherwise use default
		if cfg.HTTP.Auth.UserFile != "" {
			userFilePathForTools = cfg.HTTP.Auth.UserFile
		} else {
			userFilePathForTools = auth.GetDefaultUserPath(execPath)
		}

		if _, err := os.Stat(userFilePathForTools); os.IsNotExist(err) {
			// User file doesn't exist - create empty store
			// Users can be added via CLI commands
			userStore = auth.InitializeUserStore()
			fmt.Fprintf(os.Stderr, "User file not found, initialized empty user store\n")
		} else {
			userStore, err = auth.LoadUserStore(userFilePathForTools)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to load user file: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Loaded %d user(s) from %s\n", len(userStore.Users), userFilePathForTools)

			// Start watching the user file for changes
			if err := userStore.StartWatching(); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Failed to start watching user file: %v\n", err)
				fmt.Fprintf(os.Stderr, "         User changes will require server restart\n")
			} else {
				fmt.Fprintf(os.Stderr, "Watching %s for changes\n", userFilePathForTools)
			}
		}
	}

	// Create rate limiter for authentication if HTTP auth is enabled
	var rateLimiter *auth.RateLimiter
	if cfg.HTTP.Enabled && cfg.HTTP.Auth.Enabled {
		rateLimiter = auth.NewRateLimiter(cfg.HTTP.Auth.RateLimitWindowMinutes, cfg.HTTP.Auth.RateLimitMaxAttempts)
		fmt.Fprintf(os.Stderr, "Rate limiting enabled: %d attempts per %d minutes per IP\n",
			cfg.HTTP.Auth.RateLimitMaxAttempts, cfg.HTTP.Auth.RateLimitWindowMinutes)
		if cfg.HTTP.Auth.MaxFailedAttemptsBeforeLockout > 0 {
			fmt.Fprintf(os.Stderr, "Account lockout enabled: %d failed attempts before lockout\n",
				cfg.HTTP.Auth.MaxFailedAttemptsBeforeLockout)
		}
	}

	// Create a cancellable context for graceful shutdown of background goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure background goroutines are stopped on exit

	// Ensure rate limiter cleanup goroutine is stopped on exit
	if rateLimiter != nil {
		defer rateLimiter.Stop()
	}

	// Get the first database configuration (if any)
	var firstDB *config.NamedDatabaseConfig
	if len(cfg.Databases) > 0 {
		firstDB = &cfg.Databases[0]
	}

	// Initialize client manager for database connections with all database configurations
	clientManager := database.NewClientManager(cfg.Databases)

	// Determine authentication mode
	authEnabled := cfg.HTTP.Enabled && cfg.HTTP.Auth.Enabled

	// Create fallback database client for stdio and HTTP-no-auth modes
	// This will be used as the "default" connection if database is configured
	var fallbackClient *database.Client
	if !authEnabled && firstDB != nil && firstDB.User != "" {
		// Attempt to connect to each configured database at startup.
		// Failures are logged as warnings; the server continues even
		// if no databases are reachable. Lazy reconnection in
		// GetClientForDatabase / GetOrCreateClient will retry on demand.
		var firstConnected *database.Client
		for i := range cfg.Databases {
			db := &cfg.Databases[i]
			if db.User == "" {
				continue
			}
			connStr := db.BuildConnectionString()
			client := database.NewClientWithConnectionString(connStr, db)

			if err := client.Connect(); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Failed to connect to database '%s' (%s@%s:%d/%s): %v\n",
					db.Name, db.User, db.Host, db.Port, db.Database, err)
				client.Close()
				continue
			}

			if err := client.LoadMetadata(); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Failed to load metadata for database '%s' (%s@%s:%d/%s): %v\n",
					db.Name, db.User, db.Host, db.Port, db.Database, err)
				client.Close()
				continue
			}

			if err := clientManager.SetClientForDatabase("default", db.Name, client); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Failed to register client for database '%s': %v\n",
					db.Name, err)
				client.Close()
				continue
			}

			fmt.Fprintf(os.Stderr, "Connected to database: %s@%s:%d/%s\n",
				db.User, db.Host, db.Port, db.Database)

			if firstConnected == nil {
				firstConnected = client
			}
		}

		if firstConnected != nil {
			fallbackClient = firstConnected
		} else {
			// No databases reachable - start with an unconfigured client.
			// Tools will attempt to connect on demand.
			fallbackClient = database.NewClient(nil)
			fmt.Fprintf(os.Stderr, "WARNING: No databases are currently reachable; the server will retry connections on demand\n")
		}
	} else if authEnabled && firstDB != nil && firstDB.User != "" {
		// Auth mode - connections will be created per-session on-demand
		// Create a template client that won't be connected
		connStr := firstDB.BuildConnectionString()
		fallbackClient = database.NewClientWithConnectionString(connStr, firstDB)
		fmt.Fprintf(os.Stderr, "Database configured: %s@%s:%d/%s (per-session connections)\n",
			firstDB.User, firstDB.Host, firstDB.Port, firstDB.Database)
	} else {
		// No database configured
		fallbackClient = database.NewClient(nil)
		fmt.Fprintf(os.Stderr, "Database: Not configured\n")
	}

	// Create access checker for database access control (used by providers and database provider)
	// In STDIO mode, pass nil since there's no access control
	var accessChecker *auth.DatabaseAccessChecker
	if cfg.HTTP.Enabled && authEnabled {
		accessChecker = auth.NewDatabaseAccessChecker(tokenStore, authEnabled, false)
	}

	// Context-aware resource provider
	contextAwareResourceProvider := resources.NewContextAwareRegistry(clientManager, authEnabled, accessChecker, cfg)

	// Context-aware tool provider
	contextAwareToolProvider := tools.NewContextAwareProvider(clientManager, contextAwareResourceProvider, authEnabled, fallbackClient, cfg, userStore, userFilePathForTools, rateLimiter, cfg.HTTP.Auth.MaxFailedAttemptsBeforeLockout, accessChecker)
	if err := contextAwareToolProvider.RegisterTools(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to register tools: %v\n", err)
		os.Exit(1)
	}

	// Create MCP server with context-aware providers
	server := mcp.NewServer(contextAwareToolProvider)
	server.SetResourceProvider(contextAwareResourceProvider)

	// Set up database provider based on mode
	// For STDIO mode, use a fixed session key
	// For HTTP mode, use the auth token as session key with access control
	if cfg.HTTP.Enabled {
		databaseProvider := database.NewHTTPDatabaseProvider(clientManager, authEnabled, accessChecker)
		server.SetDatabaseProvider(databaseProvider)
	} else {
		databaseProvider := database.NewStdioDatabaseProvider(clientManager)
		server.SetDatabaseProvider(databaseProvider)
	}

	// Register prompts (only enabled ones)
	promptRegistry := prompts.NewRegistry()
	if cfg.Builtins.Prompts.IsPromptEnabled("explore-database") {
		promptRegistry.Register("explore-database", prompts.ExploreDatabase())
	}
	if cfg.Builtins.Prompts.IsPromptEnabled("setup-semantic-search") {
		promptRegistry.Register("setup-semantic-search", prompts.SetupSemanticSearch())
	}
	if cfg.Builtins.Prompts.IsPromptEnabled("diagnose-query-issue") {
		promptRegistry.Register("diagnose-query-issue", prompts.DiagnoseQueryIssue())
	}
	if cfg.Builtins.Prompts.IsPromptEnabled("design-schema") {
		promptRegistry.Register("design-schema", prompts.DesignSchema())
	}
	server.SetPromptProvider(promptRegistry)

	// Load custom definitions if configured
	if cfg.CustomDefinitionsPath != "" {
		fmt.Fprintf(os.Stderr, "Loading custom definitions from: %s\n", cfg.CustomDefinitionsPath)
		defs, err := definitions.LoadDefinitions(cfg.CustomDefinitionsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to load custom definitions: %v\n", err)
			os.Exit(1)
		}

		// Register custom prompts
		for _, promptDef := range defs.Prompts {
			if err := promptRegistry.RegisterStatic(promptDef); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to register prompt %s: %v\n", promptDef.Name, err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Registered custom prompt: %s\n", promptDef.Name)
		}

		// Register custom resources
		for _, resDef := range defs.Resources {
			switch resDef.Type {
			case "sql":
				if err := contextAwareResourceProvider.RegisterSQL(resDef); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: Failed to register resource %s: %v\n", resDef.URI, err)
					os.Exit(1)
				}
				fmt.Fprintf(os.Stderr, "Registered custom SQL resource: %s\n", resDef.URI)
			case "static":
				if err := contextAwareResourceProvider.RegisterStatic(resDef); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: Failed to register resource %s: %v\n", resDef.URI, err)
					os.Exit(1)
				}
				fmt.Fprintf(os.Stderr, "Registered custom static resource: %s\n", resDef.URI)
			}
		}

		// Register custom tools
		for i := range defs.Tools {
			if err := contextAwareToolProvider.RegisterCustomTool(defs.Tools[i]); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: Failed to register tool %s: %v\n", defs.Tools[i].Name, err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Registered custom %s tool: %s\n", defs.Tools[i].Type, defs.Tools[i].Name)
		}

		fmt.Fprintf(os.Stderr, "Loaded %d custom prompt(s), %d custom resource(s), and %d custom tool(s)\n",
			len(defs.Prompts), len(defs.Resources), len(defs.Tools))
	}

	// Start periodic cleanup of expired tokens if auth is enabled
	if cfg.HTTP.Enabled && cfg.HTTP.Auth.Enabled {
		// Clean up expired tokens on startup (no connections exist yet)
		if removed, _ := tokenStore.CleanupExpiredTokens(); removed > 0 {
			fmt.Fprintf(os.Stderr, "Removed %d expired token(s)\n", removed)
			// Save the cleaned store
			if err := auth.SaveTokenStore(cfg.HTTP.Auth.TokenFile, tokenStore); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Failed to save cleaned token file: %v\n", err)
			}
		}

		// Start periodic cleanup goroutine
		go func() {
			ticker := time.NewTicker(tokenCleanupInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if removed, hashes := tokenStore.CleanupExpiredTokens(); removed > 0 {
						fmt.Fprintf(os.Stderr, "Removed %d expired token(s)\n", removed)

						// Create a timeout context for cleanup operations to prevent indefinite blocking
						cleanupCtx, cancel := context.WithTimeout(context.Background(), tokenCleanupTimeout)

						// Clean up database connections for expired tokens
						done := make(chan error, 1)
						go func() {
							done <- clientManager.RemoveClients(hashes)
						}()

						select {
						case err := <-done:
							if err != nil {
								fmt.Fprintf(os.Stderr, "WARNING: Failed to cleanup connections: %v\n", err)
							}
						case <-cleanupCtx.Done():
							fmt.Fprintf(os.Stderr, "WARNING: Connection cleanup timed out\n")
						}

						// Cancel context after cleanup is done
						cancel()

						// Save the cleaned store
						if err := auth.SaveTokenStore(cfg.HTTP.Auth.TokenFile, tokenStore); err != nil {
							fmt.Fprintf(os.Stderr, "WARNING: Failed to save cleaned token file: %v\n", err)
						}
					}
				}
			}
		}()

		fmt.Fprintf(os.Stderr, "Authentication: ENABLED\n")
	} else if cfg.HTTP.Enabled {
		fmt.Fprintf(os.Stderr, "Authentication: DISABLED\n")
	} else {
		fmt.Fprintf(os.Stderr, "Mode: STDIO\n")
	}

	// Initialize conversation store for HTTP mode with auth
	var convStore *conversations.Store
	if cfg.HTTP.Enabled && cfg.HTTP.Auth.Enabled && userStore != nil {
		// Use configured data directory, or default to a directory next to the executable
		dataDir := cfg.DataDir
		if dataDir == "" {
			dataDir = filepath.Join(filepath.Dir(execPath), "data")
		}
		var err error
		convStore, err = conversations.NewStore(dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to initialize conversation store: %v\n", err)
			fmt.Fprintf(os.Stderr, "         Conversation history will not be available\n")
		} else {
			fmt.Fprintf(os.Stderr, "Conversation store: %s/conversations.db\n", dataDir)
			defer convStore.Close()
		}
	}

	if cfg.HTTP.Enabled {
		// HTTP/HTTPS mode
		// Create HTTP server configuration
		httpConfig := &mcp.HTTPConfig{
			Addr:        cfg.HTTP.Address,
			TLSEnable:   cfg.HTTP.TLS.Enabled,
			CertFile:    cfg.HTTP.TLS.CertFile,
			KeyFile:     cfg.HTTP.TLS.KeyFile,
			ChainFile:   cfg.HTTP.TLS.ChainFile,
			AuthEnabled: cfg.HTTP.Auth.Enabled,
			TokenStore:  tokenStore,
			UserStore:   userStore,
			Debug:       *debug,
		}

		// Setup additional HTTP handlers
		httpConfig.SetupHandlers = func(mux *http.ServeMux) error {
			// Helper to wrap handlers with authentication when enabled
			authWrapper := func(handler http.HandlerFunc) http.HandlerFunc {
				if !cfg.HTTP.Auth.Enabled {
					return handler
				}
				return func(w http.ResponseWriter, r *http.Request) {
					// Extract token from Authorization header
					authHeader := r.Header.Get("Authorization")
					if authHeader == "" {
						http.Error(w, "Missing Authorization header",
							http.StatusUnauthorized)
						return
					}

					// Extract Bearer token
					token := strings.TrimPrefix(authHeader, "Bearer ")
					if token == authHeader {
						http.Error(w, "Invalid Authorization header format",
							http.StatusUnauthorized)
						return
					}

					// Try API token first, then session token
					if _, err := tokenStore.ValidateToken(token); err != nil {
						// Try session token if user auth is enabled
						if userStore != nil {
							if _, err := userStore.ValidateSessionToken(token); err != nil {
								http.Error(w, "Invalid or expired token",
									http.StatusUnauthorized)
								return
							}
						} else {
							http.Error(w, "Invalid or expired token",
								http.StatusUnauthorized)
							return
						}
					}

					// Token valid, proceed with handler
					handler(w, r)
				}
			}

			// Chat history compaction endpoint - requires auth when enabled
			mux.HandleFunc("/api/chat/compact",
				authWrapper(compactor.HandleCompact))

			// User info endpoint - returns auth status (no error if not logged in)
			mux.HandleFunc("/api/user/info", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				// Extract session token from Authorization header
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" {
					//nolint:errcheck // Encoding a simple map should never fail
					json.NewEncoder(w).Encode(map[string]any{
						"authenticated": false,
					})
					return
				}

				// Extract Bearer token
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token == authHeader {
					//nolint:errcheck // Encoding a simple map should never fail
					json.NewEncoder(w).Encode(map[string]any{
						"authenticated": false,
						"error":         "Invalid Authorization header format",
					})
					return
				}

				// Validate session token and get username
				username, err := userStore.ValidateSessionToken(token)
				if err != nil {
					//nolint:errcheck // Encoding a simple map should never fail
					json.NewEncoder(w).Encode(map[string]any{
						"authenticated": false,
						"error":         "Invalid or expired session",
					})
					return
				}

				// Return user info as JSON
				//nolint:errcheck // Encoding a simple map should never fail
				json.NewEncoder(w).Encode(map[string]any{
					"authenticated": true,
					"username":      username,
				})
			})

			// Add LLM proxy handlers if enabled
			if cfg.LLM.Enabled {
				// Create LLM proxy configuration
				llmConfig := &llmproxy.Config{
					Provider:         cfg.LLM.Provider,
					Model:            cfg.LLM.Model,
					AnthropicAPIKey:  cfg.LLM.AnthropicAPIKey,
					AnthropicBaseURL: cfg.LLM.AnthropicBaseURL,
					OpenAIAPIKey:     cfg.LLM.OpenAIAPIKey,
					OpenAIBaseURL:    cfg.LLM.OpenAIBaseURL,
					OllamaURL:        cfg.LLM.OllamaURL,
					MaxTokens:        cfg.LLM.MaxTokens,
					Temperature:      cfg.LLM.Temperature,
				}

				// Provider/model listing requires auth when enabled
				mux.HandleFunc("/api/llm/providers",
					authWrapper(func(w http.ResponseWriter, r *http.Request) {
						llmproxy.HandleProviders(w, r, llmConfig)
					}))
				mux.HandleFunc("/api/llm/models",
					authWrapper(func(w http.ResponseWriter, r *http.Request) {
						llmproxy.HandleModels(w, r, llmConfig)
					}))
				// Chat endpoint requires auth (makes actual LLM API calls)
				mux.HandleFunc("/api/llm/chat",
					authWrapper(func(w http.ResponseWriter, r *http.Request) {
						llmproxy.HandleChat(w, r, llmConfig)
					}))
			}

			// Database listing and selection endpoints
			accessChecker := auth.NewDatabaseAccessChecker(tokenStore, authEnabled, false)
			dbHandler := api.NewDatabaseHandler(clientManager, accessChecker, false, authEnabled)
			mux.HandleFunc("/api/databases", authWrapper(dbHandler.HandleListDatabases))
			mux.HandleFunc("/api/databases/select", authWrapper(dbHandler.HandleSelectDatabase))

			// OpenAPI specification endpoint (no auth required;
			// bypassed in auth middleware via auth.OpenAPIPath)
			specJSON, err := json.MarshalIndent(openapi.BuildSpec(), "", "  ")
			if err != nil {
				return fmt.Errorf("failed to build OpenAPI spec: %w", err)
			}
			mux.HandleFunc("/api/openapi.json", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "public, max-age=3600")
				//nolint:errcheck // Write error only occurs if client disconnects
				w.Write(specJSON)
			})

			// Conversation history endpoints (only if store is available)
			if convStore != nil && userStore != nil {
				convHandler := conversations.NewHandler(convStore, userStore)
				convHandler.RegisterRoutes(mux, authWrapper)
				fmt.Fprintf(os.Stderr, "Conversation history: ENABLED\n")
			}

			return nil
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

		if cfg.HTTP.Auth.Enabled {
			fmt.Fprintf(os.Stderr, "Authentication: ENABLED\n")
		} else {
			fmt.Fprintf(os.Stderr, "Authentication: DISABLED (warning: server is not secured)\n")
		}

		if cfg.LLM.Enabled {
			fmt.Fprintf(os.Stderr, "LLM Proxy: ENABLED (provider: %s, model: %s)\n", cfg.LLM.Provider, cfg.LLM.Model)
		} else {
			fmt.Fprintf(os.Stderr, "LLM Proxy: DISABLED\n")
		}

		if cfg.Knowledgebase.Enabled {
			apiKeyStatus := "not set"
			if cfg.Knowledgebase.EmbeddingVoyageAPIKey != "" {
				apiKeyStatus = "loaded"
			} else if cfg.Knowledgebase.EmbeddingOpenAIAPIKey != "" {
				apiKeyStatus = "loaded"
			}
			fmt.Fprintf(os.Stderr, "Knowledgebase: ENABLED (provider: %s, model: %s, API key: %s)\n",
				cfg.Knowledgebase.EmbeddingProvider, cfg.Knowledgebase.EmbeddingModel, apiKeyStatus)
		} else {
			fmt.Fprintf(os.Stderr, "Knowledgebase: DISABLED\n")
		}

		if *debug {
			fmt.Fprintf(os.Stderr, "Debug logging: ENABLED\n")
		}

		// Set up SIGHUP handler for configuration reload (HTTP mode only)
		// Re-use the outer cliFlags (populated by flag.Visit) so that
		// the reload path knows which flags were explicitly provided.
		reloadCLIFlags := config.CLIFlags{
			DBHost:                  *dbHost,
			DBHostSet:               cliFlags.DBHostSet,
			DBPort:                  *dbPort,
			DBPortSet:               cliFlags.DBPortSet,
			DBName:                  *dbName,
			DBNameSet:               cliFlags.DBNameSet,
			DBUser:                  *dbUser,
			DBUserSet:               cliFlags.DBUserSet,
			DBPassword:              *dbPassword,
			DBPassSet:               cliFlags.DBPassSet,
			DBSSLMode:               *dbSSLMode,
			DBSSLSet:                cliFlags.DBSSLSet,
			DBHosts:                 *dbHosts,
			DBHostsSet:              cliFlags.DBHostsSet,
			DBTargetSessionAttrs:    *dbTargetSessionAttrs,
			DBTargetSessionAttrsSet: cliFlags.DBTargetSessionAttrsSet,
		}
		reloadableCfg := config.NewReloadableConfig(cfg, configPath, reloadCLIFlags)

		// Register callback to update client manager when databases change
		reloadableCfg.OnReload(func(newCfg *config.Config) {
			clientManager.UpdateDatabaseConfigs(newCfg.Databases)
		})

		// Start SIGHUP listener
		sighup := make(chan os.Signal, 1)
		signal.Notify(sighup, syscall.SIGHUP)
		go func() {
			for range sighup {
				fmt.Fprintf(os.Stderr, "Received SIGHUP, reloading configuration...\n")
				if err := reloadableCfg.Reload(); err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: Failed to reload config: %v\n", err)
				}
				if tracing.IsEnabled() {
					tracing.LogConfigReload("", map[string]any{
						"event": "sighup",
					})
				}
			}
		}()

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
	if clientManager != nil {
		// Close all per-token connections
		if err := clientManager.CloseAll(); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Error closing database connections: %v\n", err)
		}
	}

	// Stop file watchers
	if tokenStore != nil {
		tokenStore.StopWatching()
	}
	if userStore != nil {
		userStore.StopWatching()
	}
}
