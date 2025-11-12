# Configuration Guide

The pgEdge MCP Server supports multiple configuration methods with the following priority (highest to lowest):

1. **Command line flags** (highest priority)
2. **Environment variables**
3. **Configuration file**
4. **Hard-coded defaults** (lowest priority)

## Configuration Options Summary

| Configuration File Option | CLI Flag | Environment Variable | Description |
|--------------------------|----------|---------------------|-------------|
| `http.enabled` | `-http` | `PGEDGE_HTTP_ENABLED` | Enable HTTP/HTTPS transport mode |
| `http.address` | `-addr` | `PGEDGE_HTTP_ADDRESS` | HTTP server bind address (default: ":8080") |
| `http.tls.enabled` | `-tls` | `PGEDGE_TLS_ENABLED` | Enable TLS/HTTPS (requires HTTP mode) |
| `http.tls.cert_file` | `-cert` | `PGEDGE_TLS_CERT_FILE` | Path to TLS certificate file |
| `http.tls.key_file` | `-key` | `PGEDGE_TLS_KEY_FILE` | Path to TLS private key file |
| `http.tls.chain_file` | `-chain` | `PGEDGE_TLS_CHAIN_FILE` | Path to TLS certificate chain file (optional) |
| `http.auth.enabled` | `-no-auth` | `PGEDGE_AUTH_ENABLED` | Enable API token authentication (default: true) |
| `http.auth.token_file` | `-token-file` | `PGEDGE_AUTH_TOKEN_FILE` | Path to API tokens file |
| `preferences_file` | `-preferences-file` | `PGEDGE_PREFERENCES_FILE` | Path to user preferences file |
| `secret_file` | `-secret-file` | `PGEDGE_SECRET_FILE` | Path to encryption secret file (auto-generated if not present) |

## Configuration File

The server can read configuration from a YAML file, making it easier to manage settings without environment variables.

**Default Location**: `pgedge-postgres-mcp.yaml` in the same directory as the binary

**Custom Location**: Use the `-config` flag to specify a different path

### Example Configuration

```yaml
# HTTP/HTTPS server (optional)
http:
  enabled: false
  address: ":8080"
  tls:
    enabled: false
    cert_file: "./server.crt"
    key_file: "./server.key"
    chain_file: ""
  auth:
    enabled: true
    token_file: ""  # defaults to {binary_dir}/api-tokens.yaml

# User preferences file path (optional)
preferences_file: ""  # defaults to pgedge-postgres-mcp-prefs.yaml

# Encryption secret file path (optional)
secret_file: ""  # defaults to pgedge-postgres-mcp.secret, auto-generated if not present

```

A complete example configuration file with detailed comments is available at [here](config-example.md).

## User Preferences File

The server uses a separate preferences file for user-modifiable settings. This file is created automatically when you save your first database connection and is separate from the main configuration file for security reasons (the server should not modify its own configuration).

**Default Location**: `pgedge-postgres-mcp-prefs.yaml` in the same directory as the binary

**Configuration Priority** (highest to lowest):

1. Command line flag: `-preferences-file /path/to/prefs.yaml`
2. Environment variable: `PGEDGE_PREFERENCES_FILE=/path/to/prefs.yaml`
3. Configuration file: `preferences_file: /path/to/prefs.yaml`
4. Default: `pgedge-postgres-mcp-prefs.yaml` (same directory as binary)

### Saved Database Connections

When authentication is **disabled**, database connections are stored globally in the preferences file. Connection parameters are stored individually, and passwords are encrypted using AES-256-GCM encryption:

```yaml
connections:
  connections:
    production:
      alias: production
      host: prod-host.example.com
      port: 5432
      user: dbuser
      password: "encrypted_base64_password_here"  # AES-256-GCM encrypted
      dbname: mydb
      sslmode: verify-full
      sslrootcert: /path/to/ca.crt
      description: "Production database"
      created_at: 2025-01-15T10:00:00Z
      last_used_at: 2025-01-15T14:30:00Z
    staging:
      alias: staging
      host: staging-host.example.com
      port: 5432
      user: dbuser
      password: "encrypted_base64_password_here"  # AES-256-GCM encrypted
      dbname: mydb
      sslmode: require
      description: "Staging environment"
      created_at: 2025-01-15T10:00:00Z
```

When authentication is **enabled**, connections are stored per-token in the API tokens file ([example](api-tokens-example.md)) instead using the same format.

**Security**: Passwords are encrypted before storage using the encryption key from the secret file (`pgedge-postgres-mcp.secret`).

### Connection Management

The server provides tools to manage saved database connections:

- **`add_database_connection`** - Save a connection with an alias
- **`remove_database_connection`** - Remove a saved connection
- **`list_database_connections`** - List all saved connections
- **`edit_database_connection`** - Update an existing connection
- **`set_database_connection`** - Connect using an alias or connection string

**When authentication is enabled:**

- Connections are stored per-token in the API tokens file ([example](api-tokens-example.md))
- Each user has their own isolated set of saved connections

**When authentication is disabled:**

- Connections are stored globally in the preferences file (`pgedge-postgres-mcp-prefs.yaml`)
- All users share the same set of saved connections

**Example workflow:**
```
# Add a connection with an alias
add_database_connection(
  alias="production",
  host="host",
  user="user",
  password="pass",
  dbname="db",
  description="Production database"
)

# Later, connect using just the alias
set_database_connection(connection_string="production")

# List all saved connections
list_database_connections()

# Remove a connection
remove_database_connection(alias="production")
```

## Encryption Secret File

The server uses a separate encryption secret file to store the encryption key used for password encryption. This file contains a 256-bit AES encryption key used to encrypt and decrypt database passwords.

**Default Location**: `pgedge-postgres-mcp.secret` in the same directory as the binary

**Configuration Priority** (highest to lowest):

1. Command line flag: `-secret-file /path/to/secret`
2. Environment variable: `PGEDGE_SECRET_FILE=/path/to/secret`
3. Configuration file: `secret_file: /path/to/secret`
4. Default: `pgedge-postgres-mcp.secret` (same directory as binary)

### Auto-Generation

The secret file is automatically generated on first run if it doesn't exist:

```bash
# First run - secret file will be auto-generated
./bin/pgedge-postgres-mcp

# Output:
# Generating new encryption key at /path/to/pgedge-postgres-mcp.secret
# Encryption key saved successfully
```

### File Format

The secret file contains a base64-encoded 256-bit encryption key:

```
base64_encoded_32_byte_key_here==
```

### Security Considerations

- **File Permissions**:
    - The secret file is created with `0600` permissions (owner read/write only)
    - The server will **refuse to start** if the secret file has incorrect permissions
    - This prevents accidentally exposing the encryption key to other users on the system

- **Backup**: Back up the secret file securely - without it, encrypted passwords cannot be decrypted
- **Storage**: Store the secret file separately from configuration and preferences files
- **Never Commit**: Never commit the secret file to version control
- **Rotation**: If the secret file is lost or compromised, you'll need to regenerate it and re-enter all passwords

**Example - Verify Permissions**:
```bash
ls -la pgedge-postgres-mcp.secret
# Should show: -rw------- (600)

# Fix if needed:
chmod 600 pgedge-postgres-mcp.secret
```

**Server will exit with an error if permissions are incorrect**:
```
ERROR: Failed to load encryption key from /path/to/pgedge-postgres-mcp.secret:
insecure permissions on key file: 0644 (expected 0600).
Please run: chmod 600 /path/to/pgedge-postgres-mcp.secret
```

### Creating a Configuration File

```bash
# Copy the example to the binary directory
cp configs/pgedge-postgres-mcp.yaml.example bin/pgedge-postgres-mcp.yaml

# Edit with your settings
vim bin/pgedge-postgres-mcp.yaml

# Run the server (automatically loads config from default location)
./bin/pgedge-postgres-mcp
```

## Command Line Flags

All configuration options can be overridden via command line flags:

### General Options

- `-config` - Path to configuration file (default: same directory as binary)
- `-preferences-file` - Path to user preferences file (default: same directory as binary)

### HTTP/HTTPS Options

- `-http` - Enable HTTP transport mode
- `-addr` - HTTP server address (default ":8080")
- `-tls` - Enable TLS/HTTPS (requires -http)
- `-cert` - Path to TLS certificate file
- `-key` - Path to TLS key file
- `-chain` - Path to TLS certificate chain file

See [Deployment Guide](deployment.md) for details on HTTP/HTTPS server setup.

### Authentication Options

- `-no-auth` - Disable API token authentication
- `-token-file` - Path to token file (default: {binary_dir}/api-tokens.yaml)
- `-add-token` - Add a new API token
- `-remove-token` - Remove token by ID or hash prefix
- `-list-tokens` - List all API tokens
- `-token-note` - Annotation for new token (with -add-token)
- `-token-expiry` - Token expiry duration: "30d", "1y", "2w", "12h", "never" (with -add-token)

See [Authentication Guide](authentication.md) for details on API token management.

### Examples

**Running in stdio mode:**
```bash
./bin/pgedge-postgres-mcp
# Then use set_database_connection tool to connect
```

**Running in HTTP mode:**
```bash
./bin/pgedge-postgres-mcp \
  -http \
  -addr ":9090"
# Then use set_database_connection tool to connect
```

## Environment Variables

The server supports environment variables for all configuration options. All environment variables use the **`PGEDGE_`** prefix to avoid collisions with other software.

### HTTP/HTTPS Server Variables

- **`PGEDGE_HTTP_ENABLED`**: Enable HTTP transport mode ("true", "1", "yes" to enable)
- **`PGEDGE_HTTP_ADDRESS`**: HTTP server address (default: ":8080")

### TLS/HTTPS Variables

- **`PGEDGE_TLS_ENABLED`**: Enable TLS/HTTPS ("true", "1", "yes" to enable)
- **`PGEDGE_TLS_CERT_FILE`**: Path to TLS certificate file
- **`PGEDGE_TLS_KEY_FILE`**: Path to TLS key file
- **`PGEDGE_TLS_CHAIN_FILE`**: Path to TLS certificate chain file (optional)

### Authentication Variables

- **`PGEDGE_AUTH_ENABLED`**: Enable API token authentication ("true", "1", "yes" to enable)
- **`PGEDGE_AUTH_TOKEN_FILE`**: Path to API token file

### Other Settings

- **`PGEDGE_PREFERENCES_FILE`**: Path to user preferences file (default: pgedge-postgres-mcp-prefs.yaml in binary directory)

### Examples

**HTTP server with authentication:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_HTTP_ADDRESS=":8080"
export PGEDGE_AUTH_ENABLED="true"
export PGEDGE_AUTH_TOKEN_FILE="./api-tokens.yaml"

./bin/pgedge-postgres-mcp
```

**HTTPS server:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_TLS_ENABLED="true"
export PGEDGE_TLS_CERT_FILE="./server.crt"
export PGEDGE_TLS_KEY_FILE="./server.key"

./bin/pgedge-postgres-mcp
```

**For Tests:**

Tests use a separate environment variable to avoid confusion with runtime configuration:

```bash
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING="postgres://localhost/postgres?sslmode=disable"
go test ./...
```

## Configuration for Claude Desktop

To use this MCP server with Claude Desktop, add it to your MCP configuration file.

### Configuration File Location

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

### Configuration Format

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/pgedge-postgres-mcp/bin/pgedge-postgres-mcp"
    }
  }
}
```

**Important Notes:**

- Replace `/absolute/path/to/pgedge-postgres-mcp` with the full path to your project directory
- Database connections are configured at runtime via the `set_database_connection` tool for security
- Claude Desktop's LLM will handle natural language to SQL translation, then this server executes the SQL queries

### Using a Configuration File with Claude Desktop

You can also use a YAML configuration file instead of environment variables:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/pgedge-postgres-mcp/bin/pgedge-postgres-mcp",
      "args": ["-config", "/absolute/path/to/your-config.yaml"]
    }
  }
}
```

## Configuration Priority Examples

Understanding how configuration priority works:

### Example 1: Command Line Override

```bash
# Config file has: address: ":8080"
# Environment has: PGEDGE_HTTP_ENABLED="true"

./bin/pgedge-postgres-mcp \
  -http \
  -addr ":3000"

# Result:
# - HTTP enabled: true (from command line, highest priority)
# - Address: :3000 (from command line, highest priority)
```

### Example 2: Environment Override

```bash
# Config file has: http.address: ":8080"
export PGEDGE_HTTP_ADDRESS=":9090"

./bin/pgedge-postgres-mcp

# Result:
# - Address: :9090 (environment overrides config file)
```

### Example 3: Config File with Defaults

```bash
# No command line flags, no environment variables
# Config file has partial settings

./bin/pgedge-postgres-mcp -config myconfig.yaml

# Result:
# - Values from config file where present
# - Hard-coded defaults for missing values
```

## Best Practices

1. **Development**: Use environment variables or config files for easy iteration
2. **Production**: Use configuration files with command-line overrides for sensitive values
3. **Claude Desktop**: Use environment variables in the MCP configuration for simplicity
4. **Secrets Management**: Never commit API keys or passwords to version control
5. **Connection Strings**: Use SSL/TLS in production (`sslmode=require` or `sslmode=verify-full`)

## Troubleshooting Configuration

### Configuration Not Loading

```bash
# Check if config file exists
ls -la bin/pgedge-postgres-mcp.yaml

# Use explicit path
./bin/pgedge-postgres-mcp -config /full/path/to/config.yaml

# Check file permissions
chmod 600 bin/pgedge-postgres-mcp.yaml  # Should be readable
```

### Environment Variables Not Working

```bash
# Verify environment variables are set
env | grep PGEDGE

# Export them if running in a new shell
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_HTTP_ADDRESS=":8080"
```

### Claude Desktop Configuration Issues

1. Check JSON syntax in `claude_desktop_config.json`
2. Ensure absolute paths are used (not relative)
3. Restart Claude Desktop after configuration changes
4. Check Claude Desktop logs for errors

For more troubleshooting help, see the [Troubleshooting Guide](troubleshooting.md).
