```yaml
# pgEdge PostgreSQL MCP Server - API Tokens Configuration File
#
# This file stores API authentication tokens and their associated metadata.
# Tokens are automatically managed via command-line tools.
#
# SECURITY NOTES:
#   - This file contains sensitive authentication data
#   - File permissions MUST be 0600 (owner read/write only)
#   - Never commit this file to version control
#   - Add to .gitignore
#
# TOKEN MANAGEMENT COMMANDS:
#   - Create new token:  ./pgedge-postgres-mcp -add-token
#   - List all tokens:   ./pgedge-postgres-mcp -list-tokens
#   - Remove a token:    ./pgedge-postgres-mcp -remove-token <id-or-hash-prefix>
#
# AUTHENTICATION MODES:
#   The server supports two authentication modes:
#
#   1. Global Mode (default):
#      - All tokens share the same database connections
#      - Simpler for single-user or trusted environments
#
#   2. Per-Token Mode:
#      - Each token has its own isolated database connections
#      - Connections stored within this file under each token
#      - Better security for multi-user environments
#      - Enable with: -auth-mode per-token

# ============================================================================
# TOKEN STRUCTURE
# ============================================================================
# Each token entry contains:
#   - hash: SHA256 hash of the actual token (64 hex characters)
#   - annotation: Human-readable description of the token's purpose
#   - created_at: Timestamp when the token was created
#   - expires_at: Optional expiry timestamp (omit or set to null for no expiry)
#   - connections: Optional saved database connections (per-token mode only)

tokens:
    # Example 1: Production API token with expiration
    prod-api-2024-01:
        hash: "a7b3c8d9e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8"
        annotation: "Production API - Team A"
        created_at: 2025-01-15T08:30:00Z
        expires_at: 2025-12-31T23:59:59Z

    # Example 2: Development token without expiration
    dev-local:
        hash: "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2"
        annotation: "Local development environment"
        created_at: 2025-01-10T14:22:00Z
        expires_at: null

    # Example 3: Token with per-token database connections
    # (Only used when server is running in per-token auth mode)
    analytics-service:
        hash: "9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c3b2a1f0e9d8c7b6a5f4e3d2c1b0a9f8"
        annotation: "Analytics service - read-only access"
        created_at: 2025-02-01T09:00:00Z
        expires_at: 2025-06-30T23:59:59Z
        connections:
            connections:
                analytics-db:
                    alias: analytics-db
                    description: Analytics PostgreSQL instance
                    host: analytics.example.com
                    port: 5432
                    user: analytics_readonly
                    password: "encrypted-password-base64-string-here"
                    dbname: analytics
                    sslmode: require
                    created_at: 2025-02-01T09:05:00Z
                    last_used_at: 2025-02-15T14:30:00Z

    # Example 4: Monitoring service token
    monitoring-agent:
        hash: "2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3"
        annotation: "Monitoring agent for health checks"
        created_at: 2025-01-20T16:45:00Z
        expires_at: null

    # Example 5: Temporary token for contractor
    contractor-q1-2025:
        hash: "3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4"
        annotation: "External contractor - Q1 2025 project"
        created_at: 2025-01-05T10:00:00Z
        expires_at: 2025-03-31T23:59:59Z

# ============================================================================
# USAGE EXAMPLES
# ============================================================================

# Example 1: Creating a new token
# $ ./pgedge-postgres-mcp -add-token
# Enter an annotation/description for this token: Production API - Team B
# Enter expiration date (YYYY-MM-DD) or press Enter for no expiration: 2025-12-31
#
# Generated token: wK7vN2xR8pQ3mL9cT6fH4jY1sZ5aB8dE0uV3gM7nW2k=
#
# IMPORTANT: Save this token securely - it cannot be retrieved later!
# Token ID: prod-api-2024-02
# Hash: b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5

# Example 2: Listing all tokens
# $ ./pgedge-postgres-mcp -list-tokens
# Tokens:
#   ID: prod-api-2024-01
#   Hash: a7b3c8d9... (first 12 chars)
#   Annotation: Production API - Team A
#   Created: 2025-01-15 08:30:00
#   Expires: 2025-12-31 23:59:59

# Example 3: Removing a token by ID
# $ ./pgedge-postgres-mcp -remove-token prod-api-2024-01
# Token 'prod-api-2024-01' removed successfully

# Example 4: Removing a token by hash prefix (min 8 characters)
# $ ./pgedge-postgres-mcp -remove-token a7b3c8d9
# Token with hash prefix 'a7b3c8d9' removed successfully

# ============================================================================
# AUTHENTICATION MODE CONFIGURATION
# ============================================================================

# Global Mode (default):
# All tokens share database connections
# To use: Just run the server normally
# $ ./pgedge-postgres-mcp -http

# Per-Token Mode:
# Each token has isolated database connections stored in this file
# To use: Add -auth-mode per-token flag
# $ ./pgedge-postgres-mcp -http -auth-mode per-token

# ============================================================================
# SECURITY BEST PRACTICES
# ============================================================================

# 1. Set proper file permissions (required):
#    chmod 600 pgedge-pg-mcp-svr-tokens.yaml

# 2. Store tokens securely:
#    - Use environment variables or secret managers for the actual tokens
#    - Never log or display tokens
#    - Rotate tokens regularly

# 3. Use expiration dates:
#    - Set expiration for temporary access
#    - Use null/omit expires_at only for permanent service accounts

# 4. Monitor token usage:
#    - Review token list regularly
#    - Remove unused tokens
#    - Audit authentication logs

# 5. Backup this file securely:
#    - Include in secure backup procedures
#    - Protect backups with encryption
#    - Store backups separately from the server
```
