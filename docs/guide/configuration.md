# Configuration Guide

The Natural Language Agent supports multiple configuration methods with the following priority (highest to lowest):

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
| `http.auth.max_failed_attempts_before_lockout` | N/A | `PGEDGE_AUTH_MAX_FAILED_ATTEMPTS_BEFORE_LOCKOUT` | Lock account after N failed attempts (0 = disabled, default: 0) |
| `http.auth.rate_limit_window_minutes` | N/A | `PGEDGE_AUTH_RATE_LIMIT_WINDOW_MINUTES` | Time window for rate limiting in minutes (default: 15) |
| `http.auth.rate_limit_max_attempts` | N/A | `PGEDGE_AUTH_RATE_LIMIT_MAX_ATTEMPTS` | Max failed attempts per IP per window (default: 10) |
| `embedding.enabled` | N/A | `PGEDGE_EMBEDDING_ENABLED` | Enable embedding generation (default: false) |
| `embedding.provider` | N/A | `PGEDGE_EMBEDDING_PROVIDER` | Embedding provider: "ollama", "voyage", or "openai" |
| `embedding.model` | N/A | `PGEDGE_EMBEDDING_MODEL` | Embedding model name (provider-specific) |
| `embedding.ollama_url` | N/A | `PGEDGE_OLLAMA_URL` | Ollama API URL (default: "http://localhost:11434") |
| `embedding.voyage_api_key` | N/A | `PGEDGE_VOYAGE_API_KEY`, `VOYAGE_API_KEY` | Voyage AI API key for embeddings |
| `embedding.voyage_api_key_file` | N/A | N/A | Path to file containing Voyage API key |
| `embedding.openai_api_key` | N/A | `PGEDGE_OPENAI_API_KEY`, `OPENAI_API_KEY` | OpenAI API key for embeddings |
| `embedding.openai_api_key_file` | N/A | N/A | Path to file containing OpenAI API key |
| `knowledgebase.enabled` | N/A | `PGEDGE_KB_ENABLED` | Enable knowledgebase search (default: false) |
| `knowledgebase.database_path` | N/A | `PGEDGE_KB_DATABASE_PATH` | Path to knowledgebase SQLite database |
| `knowledgebase.embedding_provider` | N/A | `PGEDGE_KB_EMBEDDING_PROVIDER` | Embedding provider for KB search: "openai", "voyage", or "ollama" (independent of `embedding` section) |
| `knowledgebase.embedding_model` | N/A | `PGEDGE_KB_EMBEDDING_MODEL` | Embedding model for KB search (must match KB build) |
| `knowledgebase.embedding_voyage_api_key` | N/A | `PGEDGE_KB_VOYAGE_API_KEY`, `VOYAGE_API_KEY` | Voyage AI API key for KB search (independent of `embedding` section) |
| `knowledgebase.embedding_voyage_api_key_file` | N/A | N/A | Path to file containing Voyage API key for KB search |
| `knowledgebase.embedding_openai_api_key` | N/A | `PGEDGE_KB_OPENAI_API_KEY`, `OPENAI_API_KEY` | OpenAI API key for KB search (independent of `embedding` section) |
| `knowledgebase.embedding_openai_api_key_file` | N/A | N/A | Path to file containing OpenAI API key for KB search |
| `knowledgebase.embedding_ollama_url` | N/A | `PGEDGE_KB_OLLAMA_URL` | Ollama API URL for KB search |
| `secret_file` | N/A | `PGEDGE_SECRET_FILE` | Path to encryption secret file (auto-generated if not present) |
| `builtins.tools.query_database` | N/A | N/A | Enable query_database tool (default: true) |
| `builtins.tools.get_schema_info` | N/A | N/A | Enable get_schema_info tool (default: true) |
| `builtins.tools.similarity_search` | N/A | N/A | Enable similarity_search tool (default: true) |
| `builtins.tools.execute_explain` | N/A | N/A | Enable execute_explain tool (default: true) |
| `builtins.tools.generate_embedding` | N/A | N/A | Enable generate_embedding tool (default: true) |
| `builtins.tools.search_knowledgebase` | N/A | N/A | Enable search_knowledgebase tool (default: true) |
| `builtins.resources.system_info` | N/A | N/A | Enable pg://system_info resource (default: true) |
| `builtins.resources.database_schema` | N/A | N/A | Enable pg://database/schema resource (default: true) |
| `builtins.prompts.explore_database` | N/A | N/A | Enable explore-database prompt (default: true) |
| `builtins.prompts.setup_semantic_search` | N/A | N/A | Enable setup-semantic-search prompt (default: true) |
| `builtins.prompts.diagnose_query_issue` | N/A | N/A | Enable diagnose-query-issue prompt (default: true) |
| `builtins.prompts.design_schema` | N/A | N/A | Enable design-schema prompt (default: true) |

## Configuration File

The server can read configuration from a YAML file, making it easier to manage settings without environment variables.

**Default Location**: `pgedge-mcp-server.yaml` in the same directory as the binary

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
    token_file: ""  # defaults to {binary_dir}/pgedge-mcp-server-tokens.yaml
    max_failed_attempts_before_lockout: 5  # Lock account after N failed attempts (0 = disabled)
    rate_limit_window_minutes: 15  # Time window for rate limiting
    rate_limit_max_attempts: 10  # Max failed attempts per IP per window

# Database connections (required)
# Multiple databases can be configured; each must have a unique name.
# available_to_users restricts access to specific session users (empty = all)
# Note: available_to_users is ignored in STDIO mode and --no-auth mode
databases:
  - name: "default"
    host: "localhost"
    port: 5432
    database: "postgres"
    user: "postgres"
    password: ""  # Leave empty to use .pgpass file
    sslmode: "prefer"
    pool_max_conns: 10
    pool_min_conns: 2
    pool_max_conn_idle_time: "5m"
    available_to_users: []  # Empty = all users can access

  # Example: Additional database with restricted access
  - name: "development"
    host: "localhost"
    port: 5433
    database: "devdb"
    user: "developer"
    password: ""
    sslmode: "prefer"
    available_to_users:
      - "alice"
      - "bob"

# Embedding generation (optional)
embedding:
  enabled: false  # Enable embedding generation from text
  provider: "ollama"  # Options: "ollama", "voyage", or "openai"
  model: "nomic-embed-text"  # Model name (provider-specific)
  ollama_url: "http://localhost:11434"  # Ollama API URL (for ollama provider)
  # voyage_api_key: "pa-..."  # Voyage AI API key (for voyage provider)
  # openai_api_key: "sk-..."  # OpenAI API key (for openai provider)

# Knowledgebase configuration (optional)
# IMPORTANT: This section has INDEPENDENT API key configuration from the embedding
# and LLM sections. This allows you to use different embedding providers for
# semantic search vs. the generate_embeddings tool.
knowledgebase:
  enabled: false  # Enable knowledgebase search
  database_path: ""  # Path to knowledgebase SQLite database
  embedding_provider: "voyage"  # Provider for KB search: "voyage", "openai", or "ollama"
  embedding_model: "voyage-3"  # Model for KB search (must match KB build)

  # API Key Configuration Priority (highest to lowest):
  # 1. Environment variables: PGEDGE_KB_VOYAGE_API_KEY, PGEDGE_KB_OPENAI_API_KEY
  # 2. API key file: embedding_voyage_api_key_file, embedding_openai_api_key_file
  # 3. Direct config value: embedding_voyage_api_key, embedding_openai_api_key
  embedding_voyage_api_key_file: "~/.voyage-api-key"  # For voyage provider
  # embedding_openai_api_key_file: "~/.openai-api-key"  # For openai provider
  # embedding_voyage_api_key: ""  # Direct key (NOT RECOMMENDED)
  # embedding_openai_api_key: ""  # Direct key (NOT RECOMMENDED)
  embedding_ollama_url: "http://localhost:11434"  # For ollama provider

# Encryption secret file path (optional)
secret_file: ""  # defaults to pgedge-mcp-server.secret, auto-generated if not present

# Built-in tools, resources, and prompts (optional)
# All are enabled by default. Set to false to disable.
# builtins:
#   tools:
#     query_database: true
#     get_schema_info: true
#     similarity_search: true
#     execute_explain: true
#     generate_embedding: true
#     search_knowledgebase: true
#   resources:
#     system_info: true
#     database_schema: true
#   prompts:
#     explore_database: true
#     setup_semantic_search: true
#     diagnose_query_issue: true
#     design_schema: true

```

A complete example configuration file with detailed comments is available at [here](../reference/config-examples/server.md).

## Multiple Database Management

The Natural Language Agent supports configuring multiple PostgreSQL databases,
allowing users to switch between different database connections at runtime.
This is particularly useful for environments with separate development,
staging, and production databases, or when providing access to multiple
projects.

### Configuring Multiple Databases

Each database must have a unique name that users reference when switching
connections:

```yaml
databases:
  - name: "production"
    host: "prod-db.example.com"
    port: 5432
    database: "myapp"
    user: "readonly_user"
    sslmode: "require"
    available_to_users: []  # All users can access

  - name: "staging"
    host: "staging-db.example.com"
    port: 5432
    database: "myapp_staging"
    user: "developer"
    sslmode: "prefer"
    available_to_users:
      - "alice"
      - "bob"
      - "qa_team"

  - name: "development"
    host: "localhost"
    port: 5432
    database: "myapp_dev"
    user: "developer"
    sslmode: "disable"
    available_to_users:
      - "alice"
      - "bob"
```

### Access Control

The `available_to_users` field controls which session users can access each
database:

- **Empty list (`[]`)**: All authenticated users can access the database
- **User list**: Only the specified usernames can access the database
- **API tokens**: Bound to a specific database via the token's `database` field
  (see [Authentication Guide](authentication.md))

**Access control is enforced in HTTP mode only.** In STDIO mode or when
authentication is disabled (`--no-auth`), all databases are accessible to
everyone.

### Default Database Selection

When a user connects, the system automatically selects a default database
using this priority:

1. **Saved preference**: If the user previously selected a database and it's
   still accessible, that database is used
2. **First accessible database**: Otherwise, the first database in the
   configuration list that the user has access to is selected
3. **No database**: If no databases are accessible, database operations will
   fail with an appropriate error message

**Example scenarios:**

| User | Accessible Databases | Default Selection |
|------|---------------------|-------------------|
| alice | production, staging, development | production (first) |
| bob | production, staging, development | production (first) |
| qa_team | production, staging | production (first) |
| guest | production | production (only option) |
| unknown | (none) | Error: no accessible databases |

### Runtime Database Switching

Users can switch between accessible databases at runtime using the client
interfaces:

**CLI Client:**

```
/list databases        # Show available databases
/show database         # Show current database
/set database staging  # Switch to staging database
```

**Web UI:**

Click the database icon in the status banner to open the database selector.
Select a database from the list to switch connections.

**Note:** Database switching is disabled while an LLM query is being
processed to prevent data consistency issues.

### Database Selection Persistence

When a user selects a database:

- The selection is saved to the user's session preferences
- On subsequent connections, the saved preference is restored (if still
  accessible)
- If the preferred database is no longer accessible (e.g., removed from
  configuration or user permissions changed), the system falls back to the
  first accessible database

## Encryption Secret File

The server uses a separate encryption secret file to store the encryption key used for password encryption. This file contains a 256-bit AES encryption key used to encrypt and decrypt database passwords.

**Default Location**: `pgedge-mcp-server.secret` in the same directory as the binary

**Configuration Priority** (highest to lowest):

1. Environment variable: `PGEDGE_SECRET_FILE=/path/to/secret`
2. Configuration file: `secret_file: /path/to/secret`
3. Default: `pgedge-mcp-server.secret` (same directory as binary)

### Auto-Generation

The secret file is automatically generated on first run if it doesn't exist:

```bash
# First run - secret file will be auto-generated
./bin/pgedge-mcp-server

# Output:
# Generating new encryption key at /path/to/pgedge-mcp-server.secret
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
ls -la pgedge-mcp-server.secret
# Should show: -rw------- (600)

# Fix if needed:
chmod 600 pgedge-mcp-server.secret
```

**Server will exit with an error if permissions are incorrect**:
```
ERROR: Failed to load encryption key from /path/to/pgedge-mcp-server.secret:
insecure permissions on key file: 0644 (expected 0600).
Please run: chmod 600 /path/to/pgedge-mcp-server.secret
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

  # API key configuration (priority: env vars > key file > direct value)
  # Option 1: Environment variable (recommended for development)
  # Option 2: API key file (recommended for production)
  openai_api_key_file: "~/.openai-api-key"
  # Option 3: Direct value (not recommended - use env var or file)
  # openai_api_key: "sk-proj-your-key-here"
```

**Supported Models**:

- `text-embedding-3-small`: 1536 dimensions (recommended, cost-effective,
  compatible with most existing databases)
- `text-embedding-3-large`: 3072 dimensions (higher quality, larger vectors)
- `text-embedding-ada-002`: 1536 dimensions (legacy model, still supported)

**API Key Configuration**:

API keys are loaded in the following priority order (highest to lowest):

1. **Environment variables**:
   ```bash
   # Use PGEDGE-prefixed environment variable (recommended for isolation)
   export PGEDGE_OPENAI_API_KEY="sk-proj-your-key-here"

   # Or use standard environment variable (also supported)
   export OPENAI_API_KEY="sk-proj-your-key-here"
   ```

2. **API key files** (recommended for production):
   ```bash
   # Create API key file
   echo "sk-proj-your-key-here" > ~/.openai-api-key
   chmod 600 ~/.openai-api-key
   ```

3. **Direct configuration value** (not recommended - use env vars or files
   instead)

**Note**: Both `PGEDGE_OPENAI_API_KEY` and `OPENAI_API_KEY` are supported.
The prefixed version takes priority if both are set.

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

  # API key configuration (priority: env vars > key file > direct value)
  # Option 1: Environment variable (recommended for development)
  # Option 2: API key file (recommended for production)
  voyage_api_key_file: "~/.voyage-api-key"
  # Option 3: Direct value (not recommended - use env var or file)
  # voyage_api_key: "pa-your-key-here"
```

**Supported Models**:

- `voyage-3`: 1024 dimensions (recommended, higher quality)
- `voyage-3-lite`: 512 dimensions (cost-effective)
- `voyage-2`: 1024 dimensions
- `voyage-2-lite`: 1024 dimensions

**API Key Configuration**:

API keys are loaded in the following priority order (highest to lowest):

1. **Environment variables**:
   ```bash
   # Use PGEDGE-prefixed environment variable (recommended for isolation)
   export PGEDGE_VOYAGE_API_KEY="pa-your-key-here"

   # Or use standard environment variable (also supported)
   export VOYAGE_API_KEY="pa-your-key-here"
   ```

2. **API key files** (recommended for production):
   ```bash
   # Create API key file
   echo "pa-your-key-here" > ~/.voyage-api-key
   chmod 600 ~/.voyage-api-key
   ```

3. **Direct configuration value** (not recommended - use env vars or files
   instead)

**Note**: Both `PGEDGE_VOYAGE_API_KEY` and `VOYAGE_API_KEY` are supported.
The prefixed version takes priority if both are set.

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
./bin/pgedge-mcp-server
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
./bin/pgedge-mcp-server
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
cp configs/pgedge-mcp-server.yaml.example bin/pgedge-mcp-server.yaml

# Edit with your settings
vim bin/pgedge-mcp-server.yaml

# Run the server (automatically loads config from default location)
./bin/pgedge-mcp-server
```

## Enabling/Disabling Built-in Features

You can selectively enable or disable built-in tools, resources, and prompts.
All features are **enabled by default**. When a feature is disabled:

- It is not advertised to the LLM in list operations
- Attempts to use it return an error message

### Configuration File

```yaml
builtins:
  tools:
    query_database: true        # Execute SQL queries
    get_schema_info: true       # Get schema information
    similarity_search: false    # Disable vector similarity search
    execute_explain: true       # Execute EXPLAIN queries
    generate_embedding: false   # Disable embedding generation
    search_knowledgebase: true  # Search documentation knowledgebase
  resources:
    system_info: true           # pg://system_info
    database_schema: true       # pg://database/schema
  prompts:
    explore_database: true      # explore-database prompt
    setup_semantic_search: true # setup-semantic-search prompt
    diagnose_query_issue: true  # diagnose-query-issue prompt
    design_schema: true         # design-schema prompt
```

### Notes

- The `read_resource` tool is always enabled as it's required for listing
  resources
- Features can also be disabled by other configuration settings (e.g.,
  `search_knowledgebase` requires `knowledgebase.enabled: true`)

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
- `-token-file` - Path to token file (default: {binary_dir}/pgedge-mcp-server-tokens.yaml)
- `-add-token` - Add a new API token
- `-remove-token` - Remove token by ID or hash prefix
- `-list-tokens` - List all API tokens
- `-token-note` - Annotation for new token (with -add-token)
- `-token-expiry` - Token expiry duration: "30d", "1y", "2w", "12h", "never"
  (with -add-token)
- `-token-database` - Bind token to specific database name (with -add-token,
  empty = first configured database)

See [Authentication Guide](authentication.md) for details on API token
management.

### Examples

**Running in stdio mode:**
```bash
# Configure database connection via environment variables, config file, or flags
./bin/pgedge-mcp-server
```

**Running in HTTP mode:**
```bash
# Configure database connection via environment variables, config file, or flags
./bin/pgedge-mcp-server \
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
export PGEDGE_AUTH_TOKEN_FILE="./pgedge-mcp-server-tokens.yaml"

./bin/pgedge-mcp-server
```

**HTTPS server:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_TLS_ENABLED="true"
export PGEDGE_TLS_CERT_FILE="./server.crt"
export PGEDGE_TLS_KEY_FILE="./server.key"

./bin/pgedge-mcp-server
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
      "command": "/absolute/path/to/pgedge-postgres-mcp/bin/pgedge-mcp-server"
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
      "command": "/absolute/path/to/pgedge-postgres-mcp/bin/pgedge-mcp-server",
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

./bin/pgedge-mcp-server \
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

./bin/pgedge-mcp-server

# Result:
# - Address: :9090 (environment overrides config file)
```

### Example 3: Config File with Defaults

```bash
# No command line flags, no environment variables
# Config file has partial settings

./bin/pgedge-mcp-server -config myconfig.yaml

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
ls -la bin/pgedge-mcp-server.yaml

# Use explicit path
./bin/pgedge-mcp-server -config /full/path/to/config.yaml

# Check file permissions
chmod 600 bin/pgedge-mcp-server.yaml  # Should be readable
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
