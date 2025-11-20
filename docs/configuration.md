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
| `embedding.enabled` | N/A | N/A | Enable embedding generation (default: false) |
| `embedding.provider` | N/A | N/A | Embedding provider: "ollama", "voyage", or "openai" |
| `embedding.model` | N/A | N/A | Embedding model name (provider-specific) |
| `embedding.ollama_url` | N/A | `PGEDGE_OLLAMA_URL` | Ollama API URL (default: "http://localhost:11434") |
| `embedding.voyage_api_key` | N/A | `PGEDGE_VOYAGE_API_KEY` | Voyage AI API key for embeddings |
| `embedding.openai_api_key` | N/A | `PGEDGE_OPENAI_API_KEY` | OpenAI API key for embeddings |
| `secret_file` | N/A | `PGEDGE_SECRET_FILE` | Path to encryption secret file (auto-generated if not present) |

## Configuration File

The server can read configuration from a YAML file, making it easier to manage settings without environment variables.

**Default Location**: `pgedge-pg-mcp-svr.yaml` in the same directory as the binary

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
    token_file: ""  # defaults to {binary_dir}/pgedge-pg-mcp-svr-tokens.yaml

# Embedding generation (optional)
embedding:
  enabled: false  # Enable embedding generation from text
  provider: "ollama"  # Options: "ollama", "voyage", or "openai"
  model: "nomic-embed-text"  # Model name (provider-specific)
  ollama_url: "http://localhost:11434"  # Ollama API URL (for ollama provider)
  # voyage_api_key: "pa-..."  # Voyage AI API key (for voyage provider)
  # openai_api_key: "sk-..."  # OpenAI API key (for openai provider)

# Encryption secret file path (optional)
secret_file: ""  # defaults to pgedge-pg-mcp-svr.secret, auto-generated if not present

```

A complete example configuration file with detailed comments is available at [here](config-example.md).

## Encryption Secret File

The server uses a separate encryption secret file to store the encryption key used for password encryption. This file contains a 256-bit AES encryption key used to encrypt and decrypt database passwords.

**Default Location**: `pgedge-pg-mcp-svr.secret` in the same directory as the binary

**Configuration Priority** (highest to lowest):

1. Environment variable: `PGEDGE_SECRET_FILE=/path/to/secret`
2. Configuration file: `secret_file: /path/to/secret`
3. Default: `pgedge-pg-mcp-svr.secret` (same directory as binary)

### Auto-Generation

The secret file is automatically generated on first run if it doesn't exist:

```bash
# First run - secret file will be auto-generated
./bin/pgedge-pg-mcp-svr

# Output:
# Generating new encryption key at /path/to/pgedge-pg-mcp-svr.secret
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
- **Storage**: Store the secret file separately from configuration files
- **Never Commit**: Never commit the secret file to version control
- **Rotation**: If the secret file is lost or compromised, you'll need to regenerate it and re-enter all passwords

**Example - Verify Permissions**:
```bash
ls -la pgedge-pg-mcp-svr.secret
# Should show: -rw------- (600)

# Fix if needed:
chmod 600 pgedge-pg-mcp-svr.secret
```

**Server will exit with an error if permissions are incorrect**:
```
ERROR: Failed to load encryption key from /path/to/pgedge-pg-mcp-svr.secret:
insecure permissions on key file: 0644 (expected 0600).
Please run: chmod 600 /path/to/pgedge-pg-mcp-svr.secret
```

## Embedding Generation Configuration

The server supports generating embeddings from text using three providers: OpenAI (cloud-based), Voyage AI (cloud-based), or Ollama (local, self-hosted). This enables you to convert natural language queries into vector embeddings for semantic search.

### Configuration File

```yaml
embedding:
  enabled: true
  provider: "openai"  # Options: "openai", "voyage", or "ollama"
  model: "text-embedding-3-small"
  openai_api_key: ""  # Set via OPENAI_API_KEY environment variable
```

### Using OpenAI (Cloud Embeddings)

**Advantages**: High quality, industry standard, multiple dimension options

**Configuration**:

```yaml
embedding:
  enabled: true
  provider: "openai"
  model: "text-embedding-3-small"  # 1536 dimensions
  openai_api_key: "sk-proj-your-key-here"
```

**Supported Models**:

- `text-embedding-3-small`: 1536 dimensions (recommended, cost-effective, compatible with most existing databases)
- `text-embedding-3-large`: 3072 dimensions (higher quality, larger vectors)
- `text-embedding-ada-002`: 1536 dimensions (legacy model, still supported)

**Environment Variables**:

```bash
# Use PGEDGE-prefixed environment variable (recommended for isolation)
export PGEDGE_OPENAI_API_KEY="sk-proj-your-key-here"

# Or use standard environment variable (also supported)
export OPENAI_API_KEY="sk-proj-your-key-here"
```

**Note**: Both `PGEDGE_OPENAI_API_KEY` and `OPENAI_API_KEY` are supported. The prefixed version takes priority if both are set.

**Pricing** (as of 2025):

- text-embedding-3-small: $0.020 / 1M tokens
- text-embedding-3-large: $0.130 / 1M tokens

### Using Voyage AI (Cloud Embeddings)

**Advantages**: High quality, managed service

**Configuration**:

```yaml
embedding:
  enabled: true
  provider: "voyage"
  model: "voyage-3"  # 1024 dimensions
  voyage_api_key: "pa-your-key-here"
```

**Supported Models**:

- `voyage-3`: 1024 dimensions (recommended, higher quality)
- `voyage-3-lite`: 512 dimensions (cost-effective)
- `voyage-2`: 1024 dimensions
- `voyage-2-lite`: 1024 dimensions

**Environment Variables**:

```bash
# Use PGEDGE-prefixed environment variable (recommended for isolation)
export PGEDGE_VOYAGE_API_KEY="pa-your-key-here"

# Or use standard environment variable (also supported)
export VOYAGE_API_KEY="pa-your-key-here"
```

**Note**: Both `PGEDGE_VOYAGE_API_KEY` and `VOYAGE_API_KEY` are supported. The prefixed version takes priority if both are set.

### Using Ollama (Local Embeddings)

**Advantages**: Free, private, works offline

**Configuration**:

```yaml
embedding:
  enabled: true
  provider: "ollama"
  model: "nomic-embed-text"  # 768 dimensions
  ollama_url: "http://localhost:11434"
```

**Supported Models**:

- `nomic-embed-text`: 768 dimensions (recommended)
- `mxbai-embed-large`: 1024 dimensions
- `all-minilm`: 384 dimensions

**Setup**:

```bash
# Install Ollama from https://ollama.com/

# Pull embedding model
ollama pull nomic-embed-text

# Verify it's running
curl http://localhost:11434/api/tags
```

### Database Operation Logging

To debug database connections, metadata loading, and queries, enable structured logging:

```bash
# Set log level
export PGEDGE_DB_LOG_LEVEL="info"    # Basic info: connections, queries, metadata loading, errors
export PGEDGE_DB_LOG_LEVEL="debug"   # Detailed: pool config, schema counts, query details
export PGEDGE_DB_LOG_LEVEL="trace"   # Very detailed: full queries, row counts, timings

# Run the server
./bin/pgedge-pg-mcp-svr
```

**Log Levels**:

- `info` (recommended): Logs connections, metadata loading, queries with success/failure and timing
- `debug`: Adds pool configuration, schema/table/column counts, and detailed query information
- `trace`: Adds full query text, arguments, and row counts

**Example Output (info level)**:

```
[DATABASE] [INFO] Connection succeeded: connection=postgres:***@localhost:5432/mydb?sslmode=disable, duration=45ms
[DATABASE] [INFO] Metadata loaded: connection=postgres:***@localhost:5432/mydb?sslmode=disable, table_count=42, duration=123ms
[DATABASE] [INFO] Query succeeded: query=SELECT * FROM users WHERE id = $1, row_count=1, duration=5ms
```

### Embedding Generation Logging

To debug embedding API calls and rate limits, enable structured logging:

```bash
# Set log level
export PGEDGE_LLM_LOG_LEVEL="info"    # Basic info: API calls, errors, token usage
export PGEDGE_LLM_LOG_LEVEL="debug"   # Detailed: text length, dimensions, timing, models
export PGEDGE_LLM_LOG_LEVEL="trace"   # Very detailed: full request/response details

# Run the server
./bin/pgedge-pg-mcp-svr
```

**Log Levels**:

- `info` (recommended): Logs API calls with success/failure, timing, dimensions, token usage, and rate limit errors
- `debug`: Adds text length, API URLs, and provider initialization
- `trace`: Adds request text previews and full response details

**Example Output (info level)**:

```
[LLM] [INFO] Provider initialized: provider=ollama, model=nomic-embed-text, base_url=http://localhost:11434
[LLM] [INFO] API call succeeded: provider=ollama, model=nomic-embed-text, text_length=245, dimensions=768, duration=156ms
[LLM] [INFO] LLM call succeeded: provider=anthropic, model=claude-3-5-sonnet-20241022, operation=chat, input_tokens=100, output_tokens=50, total_tokens=150, duration=1.2s
[LLM] [INFO] RATE LIMIT ERROR: provider=voyage, model=voyage-3-lite, status_code=429, response={"error":...}
```

### Creating a Configuration File

```bash
# Copy the example to the binary directory
cp configs/pgedge-pg-mcp-svr.yaml.example bin/pgedge-pg-mcp-svr.yaml

# Edit with your settings
vim bin/pgedge-pg-mcp-svr.yaml

# Run the server (automatically loads config from default location)
./bin/pgedge-pg-mcp-svr
```

## Command Line Flags

All configuration options can be overridden via command line flags:

### General Options

- `-config` - Path to configuration file (default: same directory as binary)

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
- `-token-file` - Path to token file (default: {binary_dir}/pgedge-pg-mcp-svr-tokens.yaml)
- `-add-token` - Add a new API token
- `-remove-token` - Remove token by ID or hash prefix
- `-list-tokens` - List all API tokens
- `-token-note` - Annotation for new token (with -add-token)
- `-token-expiry` - Token expiry duration: "30d", "1y", "2w", "12h", "never" (with -add-token)

See [Authentication Guide](authentication.md) for details on API token management.

### Examples

**Running in stdio mode:**
```bash
# Configure database connection via environment variables, config file, or flags
./bin/pgedge-pg-mcp-svr
```

**Running in HTTP mode:**
```bash
# Configure database connection via environment variables, config file, or flags
./bin/pgedge-pg-mcp-svr \
  -http \
  -addr ":9090"
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

### Examples

**HTTP server with authentication:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_HTTP_ADDRESS=":8080"
export PGEDGE_AUTH_ENABLED="true"
export PGEDGE_AUTH_TOKEN_FILE="./pgedge-pg-mcp-svr-tokens.yaml"

./bin/pgedge-pg-mcp-svr
```

**HTTPS server:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_TLS_ENABLED="true"
export PGEDGE_TLS_CERT_FILE="./server.crt"
export PGEDGE_TLS_KEY_FILE="./server.key"

./bin/pgedge-pg-mcp-svr
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
      "command": "/absolute/path/to/pgedge-postgres-mcp/bin/pgedge-pg-mcp-svr"
    }
  }
}
```

**Important Notes:**

- Replace `/absolute/path/to/pgedge-postgres-mcp` with the full path to your project directory
- Database connections are configured at server startup via environment variables, config file, or command-line flags
- Claude Desktop's LLM will handle natural language to SQL translation, then this server executes the SQL queries

### Using a Configuration File with Claude Desktop

You can also use a YAML configuration file instead of environment variables:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/pgedge-postgres-mcp/bin/pgedge-pg-mcp-svr",
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

./bin/pgedge-pg-mcp-svr \
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

./bin/pgedge-pg-mcp-svr

# Result:
# - Address: :9090 (environment overrides config file)
```

### Example 3: Config File with Defaults

```bash
# No command line flags, no environment variables
# Config file has partial settings

./bin/pgedge-pg-mcp-svr -config myconfig.yaml

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
ls -la bin/pgedge-pg-mcp-svr.yaml

# Use explicit path
./bin/pgedge-pg-mcp-svr -config /full/path/to/config.yaml

# Check file permissions
chmod 600 bin/pgedge-pg-mcp-svr.yaml  # Should be readable
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
