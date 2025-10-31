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
# Database connection (required)
database:
  connection_string: "postgres://localhost/postgres?sslmode=disable"

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
```

A complete example configuration file with detailed comments is available at [configs/pgedge-postgres-mcp.yaml.example](../configs/pgedge-postgres-mcp.yaml.example).

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
- `-db` - PostgreSQL connection string

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

See [Deployment Guide](DEPLOYMENT.md) for details on HTTP/HTTPS server setup.

### Authentication Options

- `-no-auth` - Disable API token authentication
- `-token-file` - Path to token file (default: api-tokens.yaml)
- `-add-token` - Add a new API token
- `-remove-token` - Remove token by ID or hash prefix
- `-list-tokens` - List all API tokens
- `-token-note` - Annotation for new token (with -add-token)
- `-token-expiry` - Token expiry duration: "30d", "1y", "2w", "12h", "never" (with -add-token)

See [Authentication Guide](AUTHENTICATION.md) for details on API token management.

### Examples

**Using Anthropic (cloud):**
```bash
./bin/pgedge-postgres-mcp \
  -db "postgres://localhost/mydb" \
  -llm-provider anthropic \
  -api-key "sk-ant-..." \
  -http \
  -addr ":9090"
```

**Using Ollama (local, free):**
```bash
# First, download the model (one-time setup)
ollama pull qwen2.5-coder:32b

# Then run the server
./bin/pgedge-postgres-mcp \
  -db "postgres://localhost/mydb" \
  -llm-provider ollama \
  -ollama-model qwen2.5-coder:32b
```

## Environment Variables

The server also supports environment variables for configuration options:

### Required Variables

- **`POSTGRES_CONNECTION_STRING`**: PostgreSQL connection string
  - Format: `postgres://username:password@host:port/database?sslmode=disable`
  - Example: `postgres://myuser:mypass@localhost:5432/mydb?sslmode=disable`

### LLM Provider Variables

**For Anthropic:**
- **`LLM_PROVIDER`**: Set to "anthropic" (default)
- **`ANTHROPIC_API_KEY`**: Your Anthropic API key (get from https://console.anthropic.com/)
- **`ANTHROPIC_MODEL`**: Claude model to use (default: "claude-sonnet-4-5")

**For Ollama:**
- **`LLM_PROVIDER`**: Set to "ollama"
- **`OLLAMA_BASE_URL`**: Ollama API URL (default: "http://localhost:11434")
- **`OLLAMA_MODEL`**: Ollama model name (e.g., "qwen2.5-coder:32b")

### Examples

**Anthropic (cloud):**
```bash
export POSTGRES_CONNECTION_STRING="postgres://localhost/mydb?sslmode=disable"
export LLM_PROVIDER="anthropic"
export ANTHROPIC_API_KEY="sk-ant-your-api-key-here"
export ANTHROPIC_MODEL="claude-sonnet-4-5"

./bin/pgedge-postgres-mcp
```

**Ollama (local):**
```bash
export POSTGRES_CONNECTION_STRING="postgres://localhost/mydb?sslmode=disable"
export LLM_PROVIDER="ollama"
export OLLAMA_MODEL="qwen2.5-coder:32b"

./bin/pgedge-postgres-mcp
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
        "POSTGRES_CONNECTION_STRING": "postgres://username:password@localhost:5432/database_name?sslmode=disable",
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
      "args": ["-llm-provider", "ollama", "-ollama-model", "qwen2.5-coder:32b"],
      "env": {
        "POSTGRES_CONNECTION_STRING": "postgres://username:password@localhost:5432/database_name?sslmode=disable"
      }
    }
  }
}
```

**Important Notes:**
- Replace `/absolute/path/to/pgedge-postgres-mcp` with the full path to your project directory
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
# Environment has: POSTGRES_CONNECTION_STRING="postgres://localhost/db1"

./bin/pgedge-postgres-mcp \
  -http \
  -addr ":3000" \
  -db "postgres://localhost/db2"

# Result:
# - Address: :3000 (from command line, highest priority)
# - Database: postgres://localhost/db2 (from command line)
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
env | grep POSTGRES
env | grep ANTHROPIC

# Export them if running in a new shell
export POSTGRES_CONNECTION_STRING="..."
export ANTHROPIC_API_KEY="..."
```

### Claude Desktop Configuration Issues

1. Check JSON syntax in `claude_desktop_config.json`
2. Ensure absolute paths are used (not relative)
3. Restart Claude Desktop after configuration changes
4. Check Claude Desktop logs for errors

For more troubleshooting help, see the [Troubleshooting Guide](TROUBLESHOOTING.md).
