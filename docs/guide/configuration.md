# Specifying your Configuration Preferences

You can provide your configuration preferences in multiple locations; the MCP server and Natural Language Agent give [preference to options specified](#configuration-priority-examples) in the following order (highest to lowest):

1. [**Command line flags**](#command-line-flags) (highest priority)
2. [**Environment variables**](env_variable_config.md)
3. [**Configuration file**](#specifying-properties-in-a-configuration-file)
4. **Hard-coded defaults** (lowest priority)

The server can read configuration preferences from a YAML file, making it easier to manage settings without environment variables.  When configuring your MCP server and Natural Language Agent, keep your use and environment in mind:

* **Development**: Use environment variables or configuration files for easy iteration.
* **Production**: Use configuration files with command-line overrides for sensitive values.
* **Claude Desktop**: Use environment variables in the MCP configuration for simplicity.
* **Secrets Management**: Never commit API keys or passwords to version control.
* **Connection Strings**: Should use SSL/TLS in production (`sslmode=require` or `sslmode=verify-full`).


## Specifying Properties in a Configuration File

By default, the configuration file is named `pgedge-postgres-mcp.yaml`, and resides in the same directory as the binary.  On the command line, you can use the `-config` flag to specify a different location.

A complete example configuration file with detailed comments is available [here](../reference/config-examples/server.md).

The following table lists the configuration options you can use to specify property values:

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
| `data_dir` | N/A | `PGEDGE_DATA_DIR` | Data directory for conversation history (default: `{binary_dir}/data`) |
| `builtins.tools.query_database` | N/A | N/A | Enable query_database tool (default: true) |
| `builtins.tools.get_schema_info` | N/A | N/A | Enable get_schema_info tool (default: true) |
| `builtins.tools.similarity_search` | N/A | N/A | Enable similarity_search tool (default: true) |
| `builtins.tools.execute_explain` | N/A | N/A | Enable execute_explain tool (default: true) |
| `builtins.tools.generate_embedding` | N/A | N/A | Enable generate_embedding tool (default: true) |
| `builtins.tools.search_knowledgebase` | N/A | N/A | Enable search_knowledgebase tool (default: true) |
| `builtins.resources.system_info` | N/A | N/A | Enable pg://system_info resource (default: true) |
| `builtins.prompts.explore_database` | N/A | N/A | Enable explore-database prompt (default: true) |
| `builtins.prompts.setup_semantic_search` | N/A | N/A | Enable setup-semantic-search prompt (default: true) |
| `builtins.prompts.diagnose_query_issue` | N/A | N/A | Enable diagnose-query-issue prompt (default: true) |
| `builtins.prompts.design_schema` | N/A | N/A | Enable design-schema prompt (default: true) |


## Configuration Priority Examples

The following examples demonstrate how the MCP server's configuration priority works.

Example 1: Command Line Override

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

Example 2: Environment Override

```bash
# Config file has: http.address: ":8080"
export PGEDGE_HTTP_ADDRESS=":9090"

./bin/pgedge-postgres-mcp

# Result:
# - Address: :9090 (environment overrides config file)
```

Example 3: Config File with Defaults

```bash
# No command line flags, no environment variables
# Config file has partial settings

./bin/pgedge-postgres-mcp -config myconfig.yaml

# Result:
# - Values from config file where present
# - Hard-coded defaults for missing values
```


## Command Line Flags

Any configuration option specified in the configuration file can be overridden with a command line flag.  Use the following command line options:

**General Options:**

- `-config` - Path to configuration file (default: same directory as binary)

**HTTP/HTTPS Options:**

- `-http` - Enable HTTP transport mode
- `-addr` - HTTP server address (default ":8080")
- `-tls` - Enable TLS/HTTPS (requires -http)
- `-cert` - Path to TLS certificate file
- `-key` - Path to TLS key file
- `-chain` - Path to TLS certificate chain file

**Authentication Options:**

- `-no-auth` - Disable API token authentication
- `-token-file` - Path to token file (default: {binary_dir}/pgedge-postgres-mcp-tokens.yaml)
- `-add-token` - Add a new API token
- `-remove-token` - Remove token by ID or hash prefix
- `-list-tokens` - List all API tokens
- `-token-note` - Annotation for new token (with -add-token)
- `-token-expiry` - Token expiry duration: "30d", "1y", "2w", "12h", "never"
  (with -add-token)
- `-token-database` - Bind token to specific database name (with -add-token,
  empty = first configured database)

See [Authentication Guide](authentication.md) for details on API token management.

### Examples - Running the MCP Server

Starting the server in stdio mode with properties specified in a configuration file in the default location:

```bash
# Configure database connection via environment variables, config file, or flags
./bin/pgedge-postgres-mcp
```

The following example starts the MCP server in HTTP mode using properties specified on the command line and in a configuration file:

```bash
# Configure database connection via environment variables, config file, or flags
./bin/pgedge-postgres-mcp \
  -http \
  -addr ":9090"
```

