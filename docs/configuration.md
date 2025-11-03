# Configuration Guide

The pgEdge MCP Server supports multiple configuration methods with the following priority (highest to lowest):

1. **Command line flags** (highest priority)
2. **Environment variables**
3. **Configuration file**
4. **Hard-coded defaults** (lowest priority)

## Configuration File

The server can read configuration from a YAML file, making it easier to manage settings without environment variables.

**Default Location**: `pgedge-postgres-mcp.yaml` in the same directory as the binary

**Custom Location**: Use the `-config` flag to specify a different path

### Example Configuration

```yaml
# LLM Provider selection (required)
llm:
  provider: anthropic  # or "ollama"

# Anthropic configuration (when provider is "anthropic")
anthropic:
  api_key: "sk-ant-your-api-key-here"
  model: "claude-sonnet-4-5"

# Ollama configuration (when provider is "ollama")
ollama:
  base_url: http://localhost:11434
  model: qwen2.5-coder:32b

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
    token_file: ""  # defaults to api-tokens.yaml

# User preferences file path (optional)
preferences_file: ""  # defaults to pgedge-postgres-mcp-prefs.yaml

```

A complete example configuration file with detailed comments is available at [pgedge-postgres-mcp.yaml.example](pgedge-postgres-mcp.yaml.example).

## User Preferences File

The server uses a separate preferences file for user-modifiable settings. This file is created automatically when you save your first database connection and is separate from the main configuration file for security reasons (the server should not modify its own configuration).

**Default Location**: `pgedge-postgres-mcp-prefs.yaml` in the same directory as the binary

**Configuration Priority** (highest to lowest):
1. Command line flag: `-preferences-file /path/to/prefs.yaml`
2. Environment variable: `PREFERENCES_FILE=/path/to/prefs.yaml`
3. Configuration file: `preferences_file: /path/to/prefs.yaml`
4. Default: `pgedge-postgres-mcp-prefs.yaml` (same directory as binary)

### Saved Database Connections

When authentication is **disabled**, database connections are stored globally in the preferences file:

```yaml
connections:
  connections:
    production:
      alias: production
      connection_string: "postgres://user:pass@prod-host:5432/mydb"
      maintenance_db: "postgres"
      description: "Production database"
      created_at: 2025-01-15T10:00:00Z
      last_used_at: 2025-01-15T14:30:00Z
    staging:
      alias: staging
      connection_string: "postgres://user:pass@staging-host:5432/mydb"
      maintenance_db: "postgres"
      description: "Staging environment"
      created_at: 2025-01-15T10:00:00Z
```

When authentication is **enabled**, connections are stored per-token in the API tokens file ([api-tokens.yaml](api-tokens.yaml)) instead.

### Connection Management

The server provides tools to manage saved database connections:

- **`add_database_connection`** - Save a connection with an alias
- **`remove_database_connection`** - Remove a saved connection
- **`list_database_connections`** - List all saved connections
- **`edit_database_connection`** - Update an existing connection
- **`set_database_connection`** - Connect using an alias or connection string

**When authentication is enabled:**
- Connections are stored per-token in the API tokens file ([api-tokens.yaml](api-tokens.yaml))
- Each user has their own isolated set of saved connections

**When authentication is disabled:**
- Connections are stored globally in the preferences file (`pgedge-postgres-mcp-prefs.yaml`)
- All users share the same set of saved connections

**Example workflow:**
```
# Add a connection with an alias
add_database_connection(
  alias="production",
  connection_string="postgres://user:pass@host/db",
  maintenance_db="postgres",
  description="Production database"
)

# Later, connect using just the alias
set_database_connection(connection_string="production")

# List all saved connections
list_database_connections()

# Remove a connection
remove_database_connection(alias="production")
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

### LLM Provider Options

- `-llm-provider` - LLM provider to use: "anthropic" or "ollama"
- `-api-key` - Anthropic API key (when using Anthropic)
- `-model` - Anthropic model to use (default: "claude-sonnet-4-5")
- `-ollama-url` - Ollama API base URL (default: "http://localhost:11434")
- `-ollama-model` - Ollama model name (required when using Ollama)

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
- `-token-file` - Path to token file (default: api-tokens.yaml)
- `-add-token` - Add a new API token
- `-remove-token` - Remove token by ID or hash prefix
- `-list-tokens` - List all API tokens
- `-token-note` - Annotation for new token (with -add-token)
- `-token-expiry` - Token expiry duration: "30d", "1y", "2w", "12h", "never" (with -add-token)

See [Authentication Guide](authentication.md) for details on API token management.

### Examples

**Using Anthropic (cloud):**
```bash
./bin/pgedge-postgres-mcp \
  -llm-provider anthropic \
  -api-key "sk-ant-..." \
  -http \
  -addr ":9090"
# Then use set_database_connection tool to connect
```

**Using Ollama (local, free):**
```bash
# First, download the model (one-time setup)
ollama pull qwen2.5-coder:32b

# Then run the server
./bin/pgedge-postgres-mcp \
  -llm-provider ollama \
  -ollama-model qwen2.5-coder:32b
# Then use set_database_connection tool to connect
```

## Environment Variables

The server also supports environment variables for configuration options:

### LLM Provider Variables

**For Anthropic:**

- **`LLM_PROVIDER`**: Set to "anthropic" (default)
- **`ANTHROPIC_API_KEY`**: Your Anthropic API key (get from https://console.anthropic.com/)
- **`ANTHROPIC_MODEL`**: Claude model to use (default: "claude-sonnet-4-5")

**For Ollama:**

- **`LLM_PROVIDER`**: Set to "ollama"
- **`OLLAMA_BASE_URL`**: Ollama API URL (default: "http://localhost:11434")
- **`OLLAMA_MODEL`**: Ollama model name (e.g., "qwen2.5-coder:32b")

**Other Settings:**

- **`PREFERENCES_FILE`**: Path to user preferences file (default: pgedge-postgres-mcp-prefs.yaml in binary directory)

### Examples

**Anthropic (cloud):**

```bash
export LLM_PROVIDER="anthropic"
export ANTHROPIC_API_KEY="sk-ant-your-api-key-here"
export ANTHROPIC_MODEL="claude-sonnet-4-5"

./bin/pgedge-postgres-mcp
# Then use set_database_connection tool to connect to your database
```

**Ollama (local):**

```bash
export LLM_PROVIDER="ollama"
export OLLAMA_MODEL="qwen2.5-coder:32b"

./bin/pgedge-postgres-mcp
# Then use set_database_connection tool to connect to your database
```

**For Tests:**

Tests use a separate environment variable to avoid confusion with runtime configuration:

```bash
export TEST_POSTGRES_CONNECTION_STRING="postgres://localhost/postgres?sslmode=disable"
go test ./...
```

## Configuration for Claude Desktop

To use this MCP server with Claude Desktop, add it to your MCP configuration file.

### Configuration File Location

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

### Configuration Format

**Option 1: Using Anthropic (cloud, default):**

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/pgedge-postgres-mcp/bin/pgedge-postgres-mcp",
      "env": {
        "ANTHROPIC_API_KEY": "sk-ant-your-api-key-here"
      }
    }
  }
}
```

**Option 2: Using Ollama (local, free):**

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/pgedge-postgres-mcp/bin/pgedge-postgres-mcp",
      "args": ["-llm-provider", "ollama", "-ollama-model", "qwen2.5-coder:32b"]
    }
  }
}
```

**Important Notes:**
- Replace `/absolute/path/to/pgedge-postgres-mcp` with the full path to your project directory
- Database connections are configured at runtime via the `set_database_connection` tool for security
- For Ollama: Make sure to install Ollama and download the model first:
    ```bash
    # Install from https://ollama.ai/
    ollama serve
    ollama pull qwen2.5-coder:32b
    ```

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
# Environment has: ANTHROPIC_API_KEY="key-from-env"

./bin/pgedge-postgres-mcp \
  -http \
  -addr ":3000" \
  -api-key "key-from-cli"

# Result:
# - Address: :3000 (from command line, highest priority)
# - API Key: key-from-cli (from command line)
```

### Example 2: Environment Override

```bash
# Config file has: api_key: "key-from-file"
export ANTHROPIC_API_KEY="key-from-env"

./bin/pgedge-postgres-mcp

# Result:
# - API Key: key-from-env (environment overrides config file)
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
env | grep ANTHROPIC
env | grep OLLAMA

# Export them if running in a new shell
export ANTHROPIC_API_KEY="..."
# Or for Ollama:
export OLLAMA_MODEL="qwen2.5-coder:32b"
```

### Claude Desktop Configuration Issues

1. Check JSON syntax in `claude_desktop_config.json`
2. Ensure absolute paths are used (not relative)
3. Restart Claude Desktop after configuration changes
4. Check Claude Desktop logs for errors

For more troubleshooting help, see the [Troubleshooting Guide](troubleshooting.md).
