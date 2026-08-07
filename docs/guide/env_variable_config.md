# Using Environment Variables to Specify Options

The server supports environment variables for all configuration options. All
environment variables use the **`PGEDGE_`** prefix to avoid collisions with
other software.

## Database Configuration

The following environment variables configure a single database connection:

- **`PGEDGE_DB_HOST`**: PostgreSQL server hostname
- **`PGEDGE_DB_PORT`**: PostgreSQL server port (default: 5432)
- **`PGEDGE_DB_NAME`**: Database name
- **`PGEDGE_DB_USER`**: Database username
- **`PGEDGE_DB_PASSWORD`**: Database password
- **`PGEDGE_DB_SSLMODE`**: SSL mode (disable, prefer, require,
  verify-ca, verify-full)
- **`PGEDGE_DB_ALLOW_WRITES`**: Allow write queries such as
  INSERT, UPDATE, and DELETE (default: false). See
  [Database Write Access](security.md#database-write-access).

The server also reads standard PostgreSQL environment variables as
fallbacks when `PGEDGE_DB_*` variables are not set:

- **`PGHOST`**: Database host (fallback for `PGEDGE_DB_HOST`)
- **`PGPORT`**: Database port (fallback for `PGEDGE_DB_PORT`)
- **`PGDATABASE`**: Database name (fallback for `PGEDGE_DB_NAME`)
- **`PGUSER`**: Database user (fallback for `PGEDGE_DB_USER`)
- **`PGPASSWORD`**: Database password (fallback for
  `PGEDGE_DB_PASSWORD`)
- **`PGSSLMODE`**: SSL mode (fallback for `PGEDGE_DB_SSLMODE`)

### Multiple Database Configuration

For Docker deployments, you can configure multiple databases using numbered
environment variables. The init script automatically generates a YAML config
file when these variables are detected.

Use the pattern `PGEDGE_DB_N_*` where N is a number from 1 to 99:

- **`PGEDGE_DB_N_NAME`**: Display name for the database (required)
- **`PGEDGE_DB_N_HOST`**: PostgreSQL server hostname
- **`PGEDGE_DB_N_PORT`**: PostgreSQL server port (default: 5432)
- **`PGEDGE_DB_N_DATABASE`**: Database name (defaults to NAME)
- **`PGEDGE_DB_N_USER`**: Database username (default: postgres)
- **`PGEDGE_DB_N_PASSWORD`**: Database password
- **`PGEDGE_DB_N_SSLMODE`**: SSL mode (default: prefer)
- **`PGEDGE_DB_N_ALLOW_WRITES`**: Allow write operations (default: false)

Numbers do not need to be sequential; gaps are skipped automatically. For
example, you can define PGEDGE_DB_1_*, PGEDGE_DB_5_*, and PGEDGE_DB_10_*.

**Example - Two databases in Docker:**

```bash
# Production database (read-only)
PGEDGE_DB_1_NAME=production
PGEDGE_DB_1_HOST=prod-db.example.com
PGEDGE_DB_1_DATABASE=prod_db
PGEDGE_DB_1_USER=readonly_user
PGEDGE_DB_1_PASSWORD=secret123
PGEDGE_DB_1_SSLMODE=require
PGEDGE_DB_1_ALLOW_WRITES=false

# Development database (with writes)
PGEDGE_DB_2_NAME=development
PGEDGE_DB_2_HOST=localhost
PGEDGE_DB_2_DATABASE=dev_db
PGEDGE_DB_2_USER=postgres
PGEDGE_DB_2_PASSWORD=devpass
PGEDGE_DB_2_ALLOW_WRITES=true
```

For more information on multiple database configuration, see
[Multiple Database Configuration](multiple_db_config.md).

### LLM Database Switching

When multiple databases are configured, the following environment variable
controls whether the LLM can switch between databases:

- **`PGEDGE_LLM_DB_SWITCHING`**: Enable LLM database switching tools
  ("true", "1", "yes" to enable; default: disabled)

When enabled, the LLM has access to `list_database_connections` and
`select_database_connection` tools. When disabled, users can still switch
databases manually via CLI commands or Web UI.

## HTTP/HTTPS Server Configuration

The following environment variables specify HTTP/HTTPS Server preferences:

- **`PGEDGE_HTTP_ENABLED`**: Enable HTTP transport mode
  ("true", "1", "yes" to enable). Required for Docker
  Compose deployments with the web client; omit for
  stdio mode.
- **`PGEDGE_HTTP_ADDRESS`**: HTTP server address (default: ":8080")

The following environment variables specify TLS/HTTPS preferences:

- **`PGEDGE_TLS_ENABLED`**: Enable TLS/HTTPS ("true", "1", "yes" to enable)
- **`PGEDGE_TLS_CERT_FILE`**: Path to TLS certificate file
- **`PGEDGE_TLS_KEY_FILE`**: Path to TLS key file
- **`PGEDGE_TLS_CHAIN_FILE`**: Path to TLS certificate chain file (optional)

The following environment variables specify authentication preferences:

- **`PGEDGE_AUTH_ENABLED`**: Enable API token authentication ("true", "1", "yes" to enable)
- **`PGEDGE_AUTH_TOKEN_FILE`**: Path to API token file
- **`PGEDGE_AUTH_USER_FILE`**: Path to user authentication file

The following environment variables specify authentication rate
limiting and account lockout preferences:

- **`PGEDGE_AUTH_MAX_FAILED_ATTEMPTS_BEFORE_LOCKOUT`**: Lock
  account after N failed attempts (0 = disabled, default: 0)
- **`PGEDGE_AUTH_RATE_LIMIT_WINDOW_MINUTES`**: Time window for
  rate limiting in minutes (default: 15)
- **`PGEDGE_AUTH_RATE_LIMIT_MAX_ATTEMPTS`**: Maximum failed
  attempts per IP per window (default: 10)

## LLM Proxy Configuration

The following environment variables specify LLM provider
configuration for the web client chat proxy:

- **`PGEDGE_LLM_ENABLED`**: Enable LLM proxy for web clients
  ("true", "1", "yes" to enable)
- **`PGEDGE_LLM_PROVIDER`**: LLM provider ("anthropic", "openai",
  or "ollama")
- **`PGEDGE_LLM_MODEL`**: Default model to use
- **`PGEDGE_LLM_MAX_TOKENS`**: Maximum tokens for LLM response
  (default: 4096)
- **`PGEDGE_ANTHROPIC_API_KEY`**: Anthropic API key (or
  `ANTHROPIC_API_KEY`)
- **`PGEDGE_ANTHROPIC_BASE_URL`**: Custom Anthropic API base URL
  (for proxies)
- **`PGEDGE_OPENAI_API_KEY`**: OpenAI API key (or
  `OPENAI_API_KEY`)
- **`PGEDGE_OPENAI_BASE_URL`**: Custom OpenAI API base URL
  (for proxies)
- **`PGEDGE_OLLAMA_URL`**: Ollama server URL

## Embedding Provider Configuration

The following environment variables specify embedding provider
configuration for the `generate_embedding` tool:

- **`PGEDGE_EMBEDDING_ENABLED`**: Enable embedding generation
  ("true", "1", "yes" to enable)
- **`PGEDGE_EMBEDDING_PROVIDER`**: Embedding provider ("ollama",
  "voyage", or "openai")
- **`PGEDGE_EMBEDDING_MODEL`**: Embedding model name
- **`PGEDGE_VOYAGE_API_KEY`**: Voyage AI API key (or
  `VOYAGE_API_KEY`)
- **`PGEDGE_VOYAGE_BASE_URL`**: Custom Voyage API base URL
  (for proxies)
- **`PGEDGE_OPENAI_EMBEDDING_BASE_URL`**: Custom OpenAI embeddings
  base URL (for proxies)

## Knowledgebase Configuration

The following environment variables specify knowledgebase
configuration. The knowledgebase embedding settings are independent
of the embedding provider settings above.

- **`PGEDGE_KB_ENABLED`**: Enable knowledgebase search ("true",
  "1", "yes" to enable)
- **`PGEDGE_KB_DATABASE_PATH`**: Path to knowledgebase SQLite
  database
- **`PGEDGE_KB_EMBEDDING_PROVIDER`**: Embedding provider for KB
  search ("openai", "voyage", or "ollama")
- **`PGEDGE_KB_EMBEDDING_MODEL`**: Embedding model for KB search
- **`PGEDGE_KB_VOYAGE_API_KEY`**: Voyage API key for knowledgebase
  (or `VOYAGE_API_KEY`)
- **`PGEDGE_KB_VOYAGE_BASE_URL`**: Custom Voyage base URL for
  knowledgebase
- **`PGEDGE_KB_OPENAI_API_KEY`**: OpenAI API key for knowledgebase
  (or `OPENAI_API_KEY`)
- **`PGEDGE_KB_OPENAI_BASE_URL`**: Custom OpenAI base URL for
  knowledgebase
- **`PGEDGE_KB_OLLAMA_URL`**: Ollama URL for knowledgebase

## Other Configuration

The following environment variables specify miscellaneous server
preferences:

- **`PGEDGE_TRACE_FILE`**: Path to JSONL trace file for debugging
  MCP interactions (disabled by default)
- **`PGEDGE_SECRET_FILE`**: Path to encryption secret file
- **`PGEDGE_CUSTOM_DEFINITIONS_PATH`**: Path to custom prompts and
  resources definition file
- **`PGEDGE_DATA_DIR`**: Data directory for conversation history

If you run into issues with your environment variable settings, check:

```bash
# Verify environment variables are set
env | grep PGEDGE

# Export the variables if you are running in a new shell
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_HTTP_ADDRESS=":8080"
```

**Examples - Deploying the MCP Server with Environment Variables**

**Configuring an HTTP server with authentication:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_HTTP_ADDRESS=":8080"
export PGEDGE_AUTH_ENABLED="true"
export PGEDGE_AUTH_TOKEN_FILE="./postgres-mcp-tokens.yaml"

./bin/pgedge-postgres-mcp
```

**Configuring a HTTPS server:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_TLS_ENABLED="true"
export PGEDGE_TLS_CERT_FILE="./server.crt"
export PGEDGE_TLS_KEY_FILE="./server.key"

./bin/pgedge-postgres-mcp
```

**Using Environment Variables for Tests:**

Tests use a separate environment variable to avoid confusion with runtime configuration:

```bash
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING="postgres://localhost/postgres?sslmode=disable"
go test ./...
```