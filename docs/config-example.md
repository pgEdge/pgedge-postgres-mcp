```yaml
# pgEdge PostgreSQL MCP Server Configuration File
#
# Configuration Priority (highest to lowest):
#   1. Command line flags
#   2. Environment variables
#   3. Configuration file values (this file)
#   4. Hard-coded defaults
#
# Copy this file to pgedge-pg-mcp-svr.yaml and customize as needed.
# By default, the server looks for config in the same directory as the binary.

# ============================================================================
# HTTP/HTTPS SERVER CONFIGURATION (Optional - only needed for API access)
# ============================================================================
# By default, the server runs in stdio mode for Claude Desktop.
# Enable HTTP mode for direct API access or web integrations.
http:
    # Enable HTTP transport mode
    # If false, server runs in stdio mode (for Claude Desktop)
    # Default: false
    # Environment variable: PGEDGE_HTTP_ENABLED
    # Command line flag: -http
    enabled: false

    # HTTP server address
    # Format: host:port or :port
    # Default: :8080
    # Environment variable: PGEDGE_HTTP_ADDRESS
    # Command line flag: -addr
    address: ":8080"

    # -------------------------
    # TLS/HTTPS Configuration
    # -------------------------
    tls:
        # Enable HTTPS (requires http.enabled: true)
        # Default: false
        # Environment variable: PGEDGE_TLS_ENABLED
        # Command line flag: -tls
        enabled: false

        # Path to TLS certificate file
        # Default: ./server.crt
        # Environment variable: PGEDGE_TLS_CERT_FILE
        # Command line flag: -cert
        cert_file: "./server.crt"

        # Path to TLS private key file
        # Default: ./server.key
        # Environment variable: PGEDGE_TLS_KEY_FILE
        # Command line flag: -key
        key_file: "./server.key"

        # Path to TLS certificate chain file (optional)
        # Default: "" (empty)
        # Environment variable: PGEDGE_TLS_CHAIN_FILE
        # Command line flag: -chain
        chain_file: ""

    # -------------------------
    # Authentication
    # -------------------------
    auth:
        # Enable API token authentication (requires http.enabled: true)
        # Default: true (authentication is enabled by default)
        # Environment variable: PGEDGE_AUTH_ENABLED
        # Command line flag: -no-auth (to disable)
        enabled: true

        # Path to API token configuration file
        # Default: Same directory as binary (pgedge-pg-mcp-svr-tokens.yaml)
        # Environment variable: PGEDGE_AUTH_TOKEN_FILE
        # Command line flag: -token-file
        token_file: ""

        # Token management commands (no database connection required):
        # - Create token: ./bin/pgedge-pg-mcp-svr -add-token
        # - List tokens:  ./bin/pgedge-pg-mcp-svr -list-tokens
        # - Remove token: ./bin/pgedge-pg-mcp-svr -remove-token <id>

# ============================================================================
# ENCRYPTION SECRET FILE (Optional)
# ============================================================================
# Path to encryption secret file used for encrypting database passwords
# Default: pgedge-pg-mcp-svr.secret in the same directory as the binary
# If the file does not exist, it will be automatically generated on first run
# IMPORTANT: The secret file must have 0600 permissions (owner read/write only)
#            The server will refuse to start if permissions are incorrect
# Environment variable: PGEDGE_SECRET_FILE
# Command line flag: N/A (not available)
secret_file: ""

# ============================================================================
# DATABASE CONFIGURATION
# ============================================================================
# Database connections are configured at server startup via:
#   1. Config file (database section below)
#   2. Environment variables (PGEDGE_DB_* or PG*)
#   3. Command-line flags (-host, -port, -database, -user, -password)
#
# See the database section below for configuration options.
# Environment variables override config file values.
# Command-line flags override both config file and environment variables.

# ============================================================================
# EXAMPLE CONFIGURATIONS
# ============================================================================

# Example 1: Default stdio mode (Claude Desktop)
# http:
#     enabled: false

# Example 2: Local development with HTTP (no authentication - not recommended)
# http:
#     enabled: true
#     address: "localhost:8080"
#     tls:
#         enabled: false
#     auth:
#         enabled: false

# Example 3: Local development with HTTP and authentication
# http:
#     enabled: true
#     address: "localhost:8080"
#     tls:
#         enabled: false
#     auth:
#         enabled: true
#         token_file: "./pgedge-pg-mcp-svr-tokens.yaml"

# Example 4: Production HTTPS deployment with authentication
# http:
#     enabled: true
#     address: ":443"
#     tls:
#         enabled: true
#         cert_file: "/etc/ssl/certs/server.crt"
#         key_file: "/etc/ssl/private/server.key"
#         chain_file: "/etc/ssl/certs/ca-chain.crt"
#     auth:
#         enabled: true
#         token_file: "/etc/pgedge-mcp/tokens.yaml"
# secret_file: "/etc/pgedge-mcp/secret.key"

# ============================================================================
# DATABASE CONFIGURATION
# ============================================================================
# Configure the PostgreSQL database connection.
# All settings can be overridden via environment variables or command line flags.
#
# Environment variables:
#   PGEDGE_DB_HOST or PGHOST
#   PGEDGE_DB_PORT or PGPORT
#   PGEDGE_DB_DATABASE or PGDATABASE
#   PGEDGE_DB_USER or PGUSER
#   PGEDGE_DB_PASSWORD or PGPASSWORD (or use .pgpass file)
#   PGEDGE_DB_SSLMODE or PGSSLMODE
#
# Command line flags:
#   -host, -port, -database, -user, -password, -sslmode
database:
    # Database host
    # Default: localhost
    host: "localhost"

    # Database port
    # Default: 5432
    port: 5432

    # Database name
    # Default: postgres
    database: "postgres"

    # Database user
    # Default: postgres
    user: "postgres"

    # Database password
    # Leave empty to use .pgpass file
    # Default: ""
    password: ""

    # SSL mode: disable, allow, prefer, require, verify-ca, verify-full
    # Default: prefer
    sslmode: "prefer"

    # Connection pool settings
    # Default: 10 max connections, 2 min connections, 5m idle time
    pool_max_conns: 10
    pool_min_conns: 2
    pool_max_conn_idle_time: "5m"

# ============================================================================
# EMBEDDING GENERATION CONFIGURATION
# ============================================================================
# Enable text-to-vector embedding generation using various providers
# Used by the generate_embedding tool for creating embeddings from text
embedding:
    # Enable embedding generation
    # Default: false
    enabled: true

    # Embedding provider: "openai", "voyage", or "ollama"
    # Default: ollama
    provider: "openai"

    # Model name (provider-specific)
    # OpenAI: text-embedding-3-small (1536 dim), text-embedding-3-large (3072 dim)
    # Voyage: voyage-3 (1024 dim), voyage-3-lite (512 dim)
    # Ollama: nomic-embed-text (768 dim), mxbai-embed-large (1024 dim)
    # Default: nomic-embed-text (ollama), text-embedding-3-small (openai),
    #          voyage-3 (voyage)
    model: "text-embedding-3-small"

    # API key configuration (see notes below for priority)
    # For OpenAI
    openai_api_key_file: "~/.openai-api-key"
    # openai_api_key: ""  # Not recommended - use file or env var

    # For Voyage AI
    voyage_api_key_file: "~/.voyage-api-key"
    # voyage_api_key: ""  # Not recommended - use file or env var

    # For Ollama
    ollama_url: "http://localhost:11434"

# ============================================================================
# LLM CONFIGURATION (for web client chat proxy)
# ============================================================================
# This is only needed when running in HTTP mode for the web client
# When running in stdio mode (CLI), this is not used - the CLI client
# manages its own LLM connection
llm:
    # Enable LLM proxy for web clients
    # Default: false (disabled for stdio mode)
    enabled: false

    # LLM provider: "anthropic", "openai", or "ollama"
    # Default: anthropic
    provider: "anthropic"

    # Model name (provider-specific)
    # Anthropic: claude-sonnet-4-5, claude-opus-4-5
    # OpenAI: gpt-5, gpt-4o, gpt-4-turbo
    # Ollama: llama3, llama3.1, mistral
    # Default: claude-sonnet-4-5 (anthropic), gpt-5 (openai), llama3 (ollama)
    model: "claude-sonnet-4-5"

    # API key configuration (see notes below for priority)
    # For Anthropic
    anthropic_api_key_file: "~/.anthropic-api-key"
    # anthropic_api_key: ""  # Not recommended - use file or env var

    # For OpenAI
    openai_api_key_file: "~/.openai-api-key"
    # openai_api_key: ""  # Not recommended - use file or env var

    # For Ollama
    ollama_url: "http://localhost:11434"

    # LLM generation settings
    max_tokens: 4096
    temperature: 0.7

# ============================================================================
# KNOWLEDGEBASE CONFIGURATION
# ============================================================================
# Enable semantic search over pre-built documentation databases
# Provides the search_knowledgebase tool for querying PostgreSQL and pgEdge
# documentation
knowledgebase:
    # Enable knowledgebase search
    # Default: false
    enabled: true

    # Path to knowledgebase SQLite database
    # Default: ""
    database_path: "./pgedge-mcp-kb.db"

    # Embedding provider for knowledgebase similarity search
    # Must match the provider used to build the knowledgebase
    # Options: "openai", "voyage", or "ollama"
    # Default: ollama
    embedding_provider: "voyage"

    # Embedding model (provider-specific)
    # Must match the model used to build the knowledgebase
    # Default: nomic-embed-text (ollama), text-embedding-3-small (openai),
    #          voyage-3 (voyage)
    embedding_model: "voyage-3"

    # API keys for knowledgebase embeddings (independent of generate_embeddings)
    # For Voyage AI
    # embedding_voyage_api_key_file: "~/.voyage-api-key"
    # embedding_voyage_api_key: ""  # Not recommended

    # For OpenAI
    # embedding_openai_api_key_file: "~/.openai-api-key"
    # embedding_openai_api_key: ""  # Not recommended

    # For Ollama
    embedding_ollama_url: "http://localhost:11434"

# ============================================================================
# CUSTOM DEFINITIONS
# ============================================================================
# Path to custom prompts and resources definition file
# Default: "" (no custom definitions)
custom_definitions_path: ""

# ============================================================================
# API KEY CONFIGURATION NOTES
# ============================================================================
# API keys can be configured in three ways (priority order):
#
# 1. Environment variables (HIGHEST PRIORITY):
#    - PGEDGE_OPENAI_API_KEY or OPENAI_API_KEY
#    - PGEDGE_ANTHROPIC_API_KEY or ANTHROPIC_API_KEY
#    - PGEDGE_VOYAGE_API_KEY or VOYAGE_API_KEY
#    - PGEDGE_OLLAMA_URL
#
# 2. API key files (RECOMMENDED):
#    - Store API keys in files with appropriate permissions
#    - Example: echo "sk-..." > ~/.openai-api-key && chmod 600 ~/.openai-api-key
#    - Supports ~ expansion for home directory
#
# 3. Direct configuration values (NOT RECOMMENDED):
#    - Store keys directly in this config file
#    - Less secure than environment variables or key files
```