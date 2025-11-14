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
        # Environment variable: PGEDGE_MCP_TOKEN_FILE
        # Command line flag: -token-file
        token_file: ""

        # Token management commands (no database connection required):
        # - Create token: ./pgedge-postgres-mcp -add-token
        # - List tokens:  ./pgedge-postgres-mcp -list-tokens
        # - Remove token: ./pgedge-postgres-mcp -remove-token <id>

# ============================================================================
# ENCRYPTION SECRET FILE (Optional)
# ============================================================================
# Path to encryption secret file used for encrypting database passwords
# Default: pgedge-pg-mcp-svr.secret in the same directory as the binary
# If the file does not exist, it will be automatically generated on first run
# IMPORTANT: The secret file must have 0600 permissions (owner read/write only)
#            The server will refuse to start if permissions are incorrect
# Environment variable: PGEDGE_SECRET_FILE
# Command line flag: -secret-file
secret_file: ""

# ============================================================================
# DATABASE CONFIGURATION
# ============================================================================
# Database connections are configured at runtime using the MCP tools.
# Use the set_database_connection tool to connect to a database by providing:
#   - A PostgreSQL connection string, OR
#   - An alias to a previously saved connection
#
# To save connections for later use, use the add_database_connection tool
# which stores individual connection parameters (host, port, user, password,
# database name, SSL settings, etc.) with encrypted passwords.
#
# Examples:
#   - Connect via string: set_database_connection("postgres://user:pass@host/db")
#   - Save connection:    add_database_connection(alias="prod", host="...", ...)
#   - Connect via alias:  set_database_connection("prod")
#   - List connections:   list_database_connections()
#   - Edit connection:    edit_database_connection(alias="prod", host="...")
#   - Remove connection:  remove_database_connection(alias="prod")

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
```