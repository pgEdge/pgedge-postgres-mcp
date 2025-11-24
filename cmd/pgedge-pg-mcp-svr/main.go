/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
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
	"strings"
	"time"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/compactor"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
	"pgedge-postgres-mcp/internal/llmproxy"
	"pgedge-postgres-mcp/internal/mcp"
	"pgedge-postgres-mcp/internal/prompts"
	"pgedge-postgres-mcp/internal/resources"
	"pgedge-postgres-mcp/internal/tools"
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
	tokenFilePath := flag.String("token-file", "", "Path to API token file")

	// Database connection flags
	dbHost := flag.String("db-host", "", "Database host")
	dbPort := flag.Int("db-port", 0, "Database port")
	dbName := flag.String("db-name", "", "Database name")
	dbUser := flag.String("db-user", "", "Database user")
	dbPassword := flag.String("db-password", "", "Database password")
	dbSSLMode := flag.String("db-sslmode", "", "Database SSL mode (disable, require, verify-ca, verify-full)")

	// Token management commands
	addTokenCmd := flag.Bool("add-token", false, "Add a new API token")
	removeTokenCmd := flag.String("remove-token", "", "Remove an API token by ID or hash prefix")
	listTokensCmd := flag.Bool("list-tokens", false, "List all API tokens")
	tokenNote := flag.String("token-note", "", "Annotation for the new token (used with -add-token)")
	tokenExpiry := flag.String("token-expiry", "", "Token expiry duration: '30d', '1y', '2w', '12h', 'never' (used with -add-token)")

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

	// Handle token management commands
	if *addTokenCmd || *removeTokenCmd != "" || *listTokensCmd {
		defaultTokenPath := auth.GetDefaultTokenPath(execPath)
		tokenFile := *tokenFilePath
		if tokenFile == "" {
			tokenFile = defaultTokenPath
		}

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

			if err := addTokenCommand(tokenFile, *tokenNote, expiry); err != nil {
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
		defaultUserPath := auth.GetDefaultUserPath(execPath)
		userFile := *userFilePath
		if userFile == "" {
			userFile = defaultUserPath
		}

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
		}
	})

	// Validate basic flag dependencies before loading full config
	if !*httpMode && (*tlsMode || *certFile != "" || *keyFile != "" || *chainFile != "") {
		fmt.Fprintf(os.Stderr, "ERROR: TLS options (-tls, -cert, -key, -chain) require -http flag\n")
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

	// Load token store if HTTP auth is enabled
	var tokenStore *auth.TokenStore
	var userStore *auth.UserStore
	userFilePathForTools := ""
	if cfg.HTTP.Enabled && cfg.HTTP.Auth.Enabled {
		if _, err := os.Stat(cfg.HTTP.Auth.TokenFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "ERROR: Token file not found: %s\n", cfg.HTTP.Auth.TokenFile)
			fmt.Fprintf(os.Stderr, "Create tokens with: %s -add-token\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "Or disable authentication with: -no-auth\n")
			os.Exit(1)
		}

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

		// Load user store for user authentication
		// Use custom path if specified, otherwise use default
		if *userFilePath != "" {
			userFilePathForTools = *userFilePath
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

	// Create a cancellable context for graceful shutdown of background goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure background goroutines are stopped on exit

	// Initialize client manager for database connections with database configuration
	clientManager := database.NewClientManager(&cfg.Database)

	// Determine authentication mode
	authEnabled := cfg.HTTP.Enabled && cfg.HTTP.Auth.Enabled

	// Create fallback database client for stdio and HTTP-no-auth modes
	// This will be used as the "default" connection if database is configured
	var fallbackClient *database.Client
	if !authEnabled && cfg.Database.User != "" {
		// Create connection to database using config
		connStr := cfg.Database.BuildConnectionString()
		fallbackClient = database.NewClientWithConnectionString(connStr, &cfg.Database)

		// Connect to database
		if err := fallbackClient.Connect(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to connect to database: %v\n", err)
			os.Exit(1)
		}

		// Load metadata
		if err := fallbackClient.LoadMetadata(); err != nil {
			// Close the connection before exiting to avoid connection leak
			fallbackClient.Close()
			fmt.Fprintf(os.Stderr, "ERROR: Failed to load database metadata: %v\n", err)
			os.Exit(1)
		}

		// Set as default connection in client manager
		if err := clientManager.SetClient("default", fallbackClient); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to set default client: %v\n", err)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "Connected to database: %s@%s:%d/%s\n",
			cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Database)
	} else if authEnabled && cfg.Database.User != "" {
		// Auth mode - connections will be created per-session on-demand
		// Create a template client that won't be connected
		connStr := cfg.Database.BuildConnectionString()
		fallbackClient = database.NewClientWithConnectionString(connStr, &cfg.Database)
		fmt.Fprintf(os.Stderr, "Database configured: %s@%s:%d/%s (per-session connections)\n",
			cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Database)
	} else {
		// No database configured
		fallbackClient = database.NewClient(&cfg.Database)
		fmt.Fprintf(os.Stderr, "Database: Not configured\n")
	}

	// Context-aware resource provider
	contextAwareResourceProvider := resources.NewContextAwareRegistry(clientManager, authEnabled)

	// Context-aware tool provider
	contextAwareToolProvider := tools.NewContextAwareProvider(clientManager, contextAwareResourceProvider, authEnabled, fallbackClient, cfg, userStore, userFilePathForTools)
	if err := contextAwareToolProvider.RegisterTools(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to register tools: %v\n", err)
		os.Exit(1)
	}

	// Create MCP server with context-aware providers
	server := mcp.NewServer(contextAwareToolProvider)
	server.SetResourceProvider(contextAwareResourceProvider)

	// Register prompts
	promptRegistry := prompts.NewRegistry()
	promptRegistry.Register("explore-database", prompts.ExploreDatabase())
	promptRegistry.Register("setup-semantic-search", prompts.SetupSemanticSearch())
	promptRegistry.Register("diagnose-query-issue", prompts.DiagnoseQueryIssue())
	server.SetPromptProvider(promptRegistry)

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
			// Chat history compaction endpoint - always available
			mux.HandleFunc("/api/chat/compact", compactor.HandleCompact)

			// User info endpoint - always available
			mux.HandleFunc("/api/user/info", func(w http.ResponseWriter, r *http.Request) {
				// Extract session token from Authorization header
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" {
					http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
					return
				}

				// Extract Bearer token
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token == authHeader {
					http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
					return
				}

				// Validate session token and get username
				username, err := userStore.ValidateSessionToken(token)
				if err != nil {
					http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
					return
				}

				// Return user info as JSON
				w.Header().Set("Content-Type", "application/json")
				//nolint:errcheck // Encoding a simple map should never fail
				json.NewEncoder(w).Encode(map[string]string{
					"username": username,
				})
			})

			// Add LLM proxy handlers if enabled
			if cfg.LLM.Enabled {
				// Create LLM proxy configuration
				llmConfig := &llmproxy.Config{
					Provider:        cfg.LLM.Provider,
					Model:           cfg.LLM.Model,
					AnthropicAPIKey: cfg.LLM.AnthropicAPIKey,
					OpenAIAPIKey:    cfg.LLM.OpenAIAPIKey,
					OllamaURL:       cfg.LLM.OllamaURL,
					MaxTokens:       cfg.LLM.MaxTokens,
					Temperature:     cfg.LLM.Temperature,
				}

				mux.HandleFunc("/api/llm/providers", func(w http.ResponseWriter, r *http.Request) {
					llmproxy.HandleProviders(w, r, llmConfig)
				})
				mux.HandleFunc("/api/llm/models", func(w http.ResponseWriter, r *http.Request) {
					llmproxy.HandleModels(w, r, llmConfig)
				})
				mux.HandleFunc("/api/llm/chat", func(w http.ResponseWriter, r *http.Request) {
					llmproxy.HandleChat(w, r, llmConfig)
				})
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

		if *debug {
			fmt.Fprintf(os.Stderr, "Debug logging: ENABLED\n")
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
