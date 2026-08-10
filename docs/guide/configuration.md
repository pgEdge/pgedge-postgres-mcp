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

By default, the configuration file is named `postgres-mcp.yaml`. The server
searches for this file in `/etc/pgedge/` first, then in the same directory
as the binary. On the command line, you can use the `-config` flag to
specify a different location.

A complete example configuration file with detailed comments is available [here](../reference/config-examples/server.md).

The following tables list the configuration options you can use to
specify property values, grouped by section.

### HTTP/HTTPS Server

| Configuration File Option | CLI Flag | Environment Variable | Description |
|--------------------------|----------|---------------------|-------------|
| `http.enabled` | `-http` | `PGEDGE_HTTP_ENABLED` | Enable HTTP/HTTPS transport mode (default: false) |
| `http.address` | `-addr` | `PGEDGE_HTTP_ADDRESS` | HTTP server bind address (default: ":8080") |
| `http.tls.enabled` | `-tls` | `PGEDGE_TLS_ENABLED` | Enable TLS/HTTPS; requires HTTP mode (default: false) |
| `http.tls.cert_file` | `-cert` | `PGEDGE_TLS_CERT_FILE` | Path to TLS certificate file |
| `http.tls.key_file` | `-key` | `PGEDGE_TLS_KEY_FILE` | Path to TLS private key file |
| `http.tls.chain_file` | `-chain` | `PGEDGE_TLS_CHAIN_FILE` | Path to TLS certificate chain file (optional) |
| `http.auth.enabled` | `-no-auth` | `PGEDGE_AUTH_ENABLED` | Enable API token authentication (default: true) |
| `http.auth.token_file` | `-token-file` | `PGEDGE_AUTH_TOKEN_FILE` | Path to API tokens file |
| `http.auth.user_file` | `-user-file` | `PGEDGE_AUTH_USER_FILE` | Path to user authentication file |
| `http.auth.max_failed_attempts_before_lockout` | N/A | `PGEDGE_AUTH_MAX_FAILED_ATTEMPTS_BEFORE_LOCKOUT` | Lock account after N failed attempts (0 = disabled, default: 0) |
| `http.auth.rate_limit_window_minutes` | N/A | `PGEDGE_AUTH_RATE_LIMIT_WINDOW_MINUTES` | Time window for rate limiting in minutes (default: 15) |
| `http.auth.rate_limit_max_attempts` | N/A | `PGEDGE_AUTH_RATE_LIMIT_MAX_ATTEMPTS` | Max failed attempts per IP per window (default: 10) |
| `http.client_ip.source` | N/A | `PGEDGE_HTTP_CLIENT_IP_SOURCE` | Where the client address is read from; `socket` or `header` (default: "socket") |
| `http.client_ip.header` | N/A | `PGEDGE_HTTP_CLIENT_IP_HEADER` | Forwarding header read when the source is `header` (default: "X-Real-IP") |
| `http.client_ip.trusted_proxies` | N/A | `PGEDGE_HTTP_CLIENT_IP_TRUSTED_PROXIES` | Comma-separated addresses or CIDR blocks permitted to set that header |
| `http.allowed_origins` | N/A | `PGEDGE_HTTP_ALLOWED_ORIGINS` | Comma-separated web origins accepted in the `Origin` header (default: loopback origins only) |

### Browser Origins

The server validates the `Origin` header that a browser sets on every
request it makes. The `http.allowed_origins` setting lists the origins
that the server accepts; a request carrying any other origin is refused
with `403 Forbidden`.

This check prevents DNS rebinding, in which a page on a site the user
is merely visiting makes that site's own hostname resolve to a loopback
address, so that the browser treats requests to a locally running
server as same-origin and hands the page the responses. The server
therefore judges a request by the origin the browser reports, and never
by the `Host` header, which the attacker controls just as freely.

By default, no origins are configured, and the server accepts only
loopback origins: `localhost`, `127.0.0.1` and `::1`, under either
scheme and on any port. This suits local use, including the bundled web
client, without any configuration.

A deployment that publishes the web client under a real hostname must
name that origin, because the browser reports the address the user
visited:

```yaml
http:
  allowed_origins:
    - https://mcp.example.com         # The origin users visit
    - https://mcp.internal.example    # More than one may be listed
```

Naming any origin replaces the loopback default rather than adding to
it, so list the loopback origins as well if you also reach the server
locally. An origin is a scheme, a host and a port; `https://example.com`
and `http://example.com` are different origins, as are two ports on the
same host. The server rejects a malformed entry at startup rather than
ignoring it, and reports the effective policy in its startup output.

Requests that carry no `Origin` header at all are unaffected, so
command line clients, scripts and other non-browser callers need no
configuration here. The header is a statement a browser makes about
itself; an attacker who can set arbitrary headers is not working
through a browser, and so is not what this control addresses.

### Client Address Resolution

The server attributes each request to a client address, which it uses
for login rate limiting and request logging. The `http.client_ip`
settings control where that address comes from.

By default, the source is `socket`; the server takes the address from
the network connection and ignores forwarding headers entirely. This
default is correct for a server that clients reach directly, and a
caller cannot influence it.

Deployments behind a reverse proxy see the proxy's address on every
connection, so they need the real address from a forwarding header
instead. Set the source to `header` and list the proxies you trust:

```yaml
http:
  client_ip:
    source: header
    header: X-Real-IP
    trusted_proxies:
      - 192.0.2.10          # A single proxy address
      - 192.0.2.0/24        # Or a CIDR block
```

The server honours the header only when the connection comes from one
of the listed proxies; a request that arrives from anywhere else is
attributed to its own connection address. Starting the server with the
source set to `header` and no trusted proxies configured is an error,
because a forwarding header carries no authority unless the server
knows which peer is entitled to set it.

`X-Forwarded-For` also works as the header, and the server handles it
as the history list it is. Each proxy appends the address it received
the request from, so the server reads the list from right to left,
skipping entries that match `trusted_proxies` and taking the first one
that does not. Reading from the left instead would return whatever the
original caller chose to send, since anything a caller invents ends up
at the far left of the list.

### Database Connections

Each entry in the `databases` list supports the following options.
See [Multiple Database Configuration](multiple_db_config.md) for
details on configuring multiple databases and access control.

| Configuration File Option | CLI Flag | Environment Variable | Description |
|--------------------------|----------|---------------------|-------------|
| `databases[].name` | N/A | `PGEDGE_DB_N_NAME` | Unique name for the database connection (required) |
| `databases[].host` | `-db-host` | `PGEDGE_DB_HOST`, `PGHOST` | Database server hostname (default: "localhost"). Mutually exclusive with `hosts`. |
| `databases[].port` | `-db-port` | `PGEDGE_DB_PORT`, `PGPORT` | Database server port (default: 5432) |
| `databases[].hosts` | `-db-hosts` | `PGEDGE_DB_HOSTS` | List of `host`/`port` pairs for multi-host failover. Mutually exclusive with `host`. |
| `databases[].target_session_attrs` | `-db-target-session-attrs` | `PGEDGE_DB_TARGET_SESSION_ATTRS` | Session routing for multi-host: `any`, `read-write`, `read-only`, `primary`, `standby`, `prefer-standby` (libpq default: `any`) |
| `databases[].database` | `-db-name` | `PGEDGE_DB_NAME`, `PGDATABASE` | Database name (default: "postgres") |
| `databases[].user` | `-db-user` | `PGEDGE_DB_USER`, `PGUSER` | Database user |
| `databases[].password` | `-db-password` | `PGEDGE_DB_PASSWORD`, `PGPASSWORD` | Database password (uses `.pgpass` if not set) |
| `databases[].sslmode` | `-db-sslmode` | `PGEDGE_DB_SSLMODE`, `PGSSLMODE` | SSL mode: disable, prefer, require, verify-ca, verify-full (default: "prefer") |
| `databases[].sslcert` | `-db-sslcert` | `PGEDGE_DB_SSLCERT`, `PGSSLCERT` | Path to the client certificate file, for client certificate authentication. Must be set together with `sslkey`. |
| `databases[].sslkey` | `-db-sslkey` | `PGEDGE_DB_SSLKEY`, `PGSSLKEY` | Path to the client private key file, for client certificate authentication. Must be set together with `sslcert`. |
| `databases[].sslrootcert` | `-db-sslrootcert` | `PGEDGE_DB_SSLROOTCERT`, `PGSSLROOTCERT` | Path to the CA certificate file used to verify the server. Takes effect only under `sslmode` `require`, `verify-ca`, or `verify-full`; ignored under other modes, matching libpq. |
| `databases[].allow_writes` | N/A | `PGEDGE_DB_ALLOW_WRITES`, `PGEDGE_DB_N_ALLOW_WRITES` | Allow write queries such as INSERT, UPDATE, and DELETE (default: false). See [Database Write Access](security.md#database-write-access). |
| `databases[].allow_llm_switching` | N/A | N/A | Allow LLM to discover and switch to the database (default: true). See [Excluding Databases from LLM Switching](multiple_db_config.md#excluding-databases-from-llm-switching). |
| `databases[].allowed_pl_languages` | N/A | N/A | PL languages allowed for custom tools, such as `["plpgsql"]`; use `["*"]` for all (default: `["plpgsql"]`). See [Custom Definitions](../advanced/custom-definitions.md). |
| `databases[].available_to_users` | N/A | N/A | Usernames allowed to access the database; empty list means all users (default: `[]`) |
| `databases[].pool_max_conns` | N/A | N/A | Maximum connections in the pool (default: 4) |
| `databases[].pool_min_conns` | N/A | N/A | Minimum connections in the pool (default: 0) |
| `databases[].pool_max_conn_idle_time` | N/A | N/A | Maximum idle time before a connection is closed (default: "30m") |
| `databases[].pool_health_check_period` | N/A | N/A | Interval for background pool health checks (default: "1m", or "30s" when multiple hosts are configured) |
| `databases[].pool_max_conn_lifetime` | N/A | N/A | Maximum lifetime of a connection before the pool closes the connection (default: "1h", or "5m" when multiple hosts are configured) |
| `databases[].connect_timeout` | N/A | N/A | Timeout for the initial database connection as a Go duration string such as "10s" or "30s" (default: "10s") |
| `databases[].metadata_ttl` | N/A | N/A | How long cached schema metadata remains valid before automatic refresh as a Go duration string such as "5m" or "30s"; use "0" to refresh on every request (default: "5m") |

The two pool settings above take shorter defaults when a database
configures more than one host, because a connection to a host that has
failed is only discovered when something checks it. Recycling
connections every five minutes and checking idle ones every thirty
seconds keeps a failed host's connections from lingering in the pool,
at the cost of reconnecting more often than a single-host deployment
needs to. The two settings are independent: giving one an explicit value
overrides only that setting, and the other still takes its multi-host
default.

CLI flags and single-database environment variables (`PGEDGE_DB_*`,
`PG*`) apply to the first database in the list. Use numbered
environment variables (`PGEDGE_DB_N_*`) to configure multiple
databases; see
[Environment Variables](env_variable_config.md#multiple-database-configuration)
for details.

### LLM Proxy

The LLM proxy is required for the web client in HTTP mode. The CLI
client manages its own LLM connection and does not use these settings.

| Configuration File Option | CLI Flag | Environment Variable | Description |
|--------------------------|----------|---------------------|-------------|
| `llm.enabled` | N/A | `PGEDGE_LLM_ENABLED` | Enable LLM proxy for web clients (default: false) |
| `llm.provider` | N/A | `PGEDGE_LLM_PROVIDER` | LLM provider: "anthropic", "openai", or "ollama" (default: "anthropic") |
| `llm.model` | N/A | `PGEDGE_LLM_MODEL` | Model name (default: provider-specific) |
| `llm.anthropic_api_key` | N/A | `PGEDGE_ANTHROPIC_API_KEY`, `ANTHROPIC_API_KEY` | Anthropic API key (prefer key file or env var) |
| `llm.anthropic_api_key_file` | N/A | N/A | Path to file containing Anthropic API key |
| `llm.anthropic_base_url` | N/A | `PGEDGE_ANTHROPIC_BASE_URL` | Custom Anthropic API base URL for proxies |
| `llm.openai_api_key` | N/A | `PGEDGE_OPENAI_API_KEY`, `OPENAI_API_KEY` | OpenAI API key (prefer key file or env var) |
| `llm.openai_api_key_file` | N/A | N/A | Path to file containing OpenAI API key |
| `llm.openai_base_url` | N/A | `PGEDGE_OPENAI_BASE_URL` | Custom OpenAI API base URL for proxies |
| `llm.ollama_url` | N/A | `PGEDGE_OLLAMA_URL` | Ollama server URL (default: "http://localhost:11434") |
| `llm.max_tokens` | N/A | `PGEDGE_LLM_MAX_TOKENS` | Maximum tokens for LLM response (default: 4096) |

### Embedding Generation

| Configuration File Option | CLI Flag | Environment Variable | Description |
|--------------------------|----------|---------------------|-------------|
| `embedding.enabled` | N/A | `PGEDGE_EMBEDDING_ENABLED` | Enable embedding generation (default: false) |
| `embedding.provider` | N/A | `PGEDGE_EMBEDDING_PROVIDER` | Embedding provider: "ollama", "voyage", "openai", or "gemini" |
| `embedding.model` | N/A | `PGEDGE_EMBEDDING_MODEL` | Embedding model name (provider-specific) |
| `embedding.ollama_url` | N/A | `PGEDGE_OLLAMA_URL` | Ollama API URL (default: "http://localhost:11434") |
| `embedding.voyage_api_key` | N/A | `PGEDGE_VOYAGE_API_KEY`, `VOYAGE_API_KEY` | Voyage AI API key for embeddings |
| `embedding.voyage_api_key_file` | N/A | N/A | Path to file containing Voyage API key |
| `embedding.voyage_base_url` | N/A | `PGEDGE_VOYAGE_BASE_URL` | Custom Voyage API base URL for proxies |
| `embedding.openai_api_key` | N/A | `PGEDGE_OPENAI_API_KEY`, `OPENAI_API_KEY` | OpenAI API key for embeddings |
| `embedding.openai_api_key_file` | N/A | N/A | Path to file containing OpenAI API key |
| `embedding.openai_base_url` | N/A | `PGEDGE_OPENAI_EMBEDDING_BASE_URL` | Custom OpenAI embedding base URL for proxies |
| `embedding.gemini_api_key` | N/A | `PGEDGE_GEMINI_API_KEY`, `GEMINI_API_KEY` | Google Gemini API key for embeddings |
| `embedding.gemini_api_key_file` | N/A | N/A | Path to file containing Gemini API key |
| `embedding.gemini_base_url` | N/A | `PGEDGE_GEMINI_EMBEDDING_BASE_URL` | Custom Gemini embedding base URL for proxies |

### Knowledgebase

The knowledgebase embedding configuration is independent of the
`embedding` section above.

| Configuration File Option | CLI Flag | Environment Variable | Description |
|--------------------------|----------|---------------------|-------------|
| `knowledgebase.enabled` | N/A | `PGEDGE_KB_ENABLED` | Enable knowledgebase search (default: false) |
| `knowledgebase.database_path` | N/A | `PGEDGE_KB_DATABASE_PATH` | Path to knowledgebase SQLite database |
| `knowledgebase.embedding_provider` | N/A | `PGEDGE_KB_EMBEDDING_PROVIDER` | Embedding provider for KB search: "openai", "voyage", "gemini", or "ollama" |
| `knowledgebase.embedding_model` | N/A | `PGEDGE_KB_EMBEDDING_MODEL` | Embedding model for KB search (must match KB build) |
| `knowledgebase.embedding_voyage_api_key` | N/A | `PGEDGE_KB_VOYAGE_API_KEY`, `VOYAGE_API_KEY` | Voyage AI API key for KB search |
| `knowledgebase.embedding_voyage_api_key_file` | N/A | N/A | Path to file containing Voyage API key for KB search |
| `knowledgebase.embedding_openai_api_key` | N/A | `PGEDGE_KB_OPENAI_API_KEY`, `OPENAI_API_KEY` | OpenAI API key for KB search |
| `knowledgebase.embedding_openai_api_key_file` | N/A | N/A | Path to file containing OpenAI API key for KB search |
| `knowledgebase.embedding_gemini_api_key` | N/A | `PGEDGE_KB_GEMINI_API_KEY`, `GEMINI_API_KEY` | Google Gemini API key for KB search |
| `knowledgebase.embedding_gemini_api_key_file` | N/A | N/A | Path to file containing Gemini API key for KB search |
| `knowledgebase.embedding_gemini_base_url` | N/A | `PGEDGE_KB_GEMINI_BASE_URL` | Custom Gemini base URL for KB search |
| `knowledgebase.embedding_ollama_url` | N/A | `PGEDGE_KB_OLLAMA_URL` | Ollama API URL for KB search |

### Built-in Features

See [Enabling or Disabling Built-in Features](feature_config.md)
for details.

| Configuration File Option | CLI Flag | Environment Variable | Description |
|--------------------------|----------|---------------------|-------------|
| `builtins.tools.query_database` | N/A | `PGEDGE_BUILTIN_TOOL_QUERY_DATABASE` | Enable query_database tool (default: true) |
| `builtins.tools.get_schema_info` | N/A | `PGEDGE_BUILTIN_TOOL_GET_SCHEMA_INFO` | Enable get_schema_info tool (default: true) |
| `builtins.tools.similarity_search` | N/A | `PGEDGE_BUILTIN_TOOL_SIMILARITY_SEARCH` | Enable similarity_search tool (default: true) |
| `builtins.tools.execute_explain` | N/A | `PGEDGE_BUILTIN_TOOL_EXECUTE_EXPLAIN` | Enable execute_explain tool (default: true) |
| `builtins.tools.generate_embedding` | N/A | `PGEDGE_BUILTIN_TOOL_GENERATE_EMBEDDING` | Enable generate_embedding tool (default: true) |
| `builtins.tools.search_knowledgebase` | N/A | `PGEDGE_BUILTIN_TOOL_SEARCH_KNOWLEDGEBASE` | Enable search_knowledgebase tool (default: true) |
| `builtins.tools.count_rows` | N/A | `PGEDGE_BUILTIN_TOOL_COUNT_ROWS` | Enable count_rows tool (default: true) |
| `builtins.tools.llm_connection_selection` | N/A | `PGEDGE_BUILTIN_TOOL_LLM_CONNECTION_SELECTION` | Enable LLM database switching tools (default: false) |
| `builtins.resources.system_info` | N/A | `PGEDGE_BUILTIN_RESOURCE_SYSTEM_INFO` | Enable pg://system_info resource (default: true) |
| `builtins.prompts.explore_database` | N/A | `PGEDGE_BUILTIN_PROMPT_EXPLORE_DATABASE` | Enable explore-database prompt (default: true) |
| `builtins.prompts.setup_semantic_search` | N/A | `PGEDGE_BUILTIN_PROMPT_SETUP_SEMANTIC_SEARCH` | Enable setup-semantic-search prompt (default: true) |
| `builtins.prompts.diagnose_query_issue` | N/A | `PGEDGE_BUILTIN_PROMPT_DIAGNOSE_QUERY_ISSUE` | Enable diagnose-query-issue prompt (default: true) |
| `builtins.prompts.design_schema` | N/A | `PGEDGE_BUILTIN_PROMPT_DESIGN_SCHEMA` | Enable design-schema prompt (default: true) |

### Other Options

| Configuration File Option | CLI Flag | Environment Variable | Description |
|--------------------------|----------|---------------------|-------------|
| `trace_file` | `-trace-file` | `PGEDGE_TRACE_FILE` | Path to JSONL trace file for debugging (disabled by default) |
| `secret_file` | N/A | `PGEDGE_SECRET_FILE` | Path to encryption secret file (auto-generated if not present) |
| `custom_definitions_path` | N/A | `PGEDGE_CUSTOM_DEFINITIONS_PATH` | Path to custom prompts and resources definition file |
| `data_dir` | N/A | `PGEDGE_DATA_DIR` | Data directory for conversation history (default: `{binary_dir}/data`) |


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

Any configuration option specified in the configuration file can be
overridden with a command line flag. Use the following command line
options:

**General Options:**

- `-config` - Path to configuration file (default: same directory
  as binary)
- `-debug` - Enable debug logging (logs HTTP requests and responses)
- `-trace-file` - Path to JSONL trace file for debugging MCP
  interactions (disabled by default)

**HTTP/HTTPS Options:**

- `-http` - Enable HTTP transport mode
- `-addr` - HTTP server address (default: ":8080")
- `-tls` - Enable TLS/HTTPS (requires `-http`)
- `-cert` - Path to TLS certificate file
- `-key` - Path to TLS key file
- `-chain` - Path to TLS certificate chain file

**Database Options:**

- `-db-host` - Database host
- `-db-port` - Database port
- `-db-name` - Database name
- `-db-user` - Database user
- `-db-password` - Database password
- `-db-sslmode` - Database SSL mode (disable, require, verify-ca,
  verify-full)
- `-db-sslcert` - Path to the client certificate file, for client
  certificate authentication. Must be set together with `-db-sslkey`.
- `-db-sslkey` - Path to the client private key file, for client
  certificate authentication. Must be set together with `-db-sslcert`.
- `-db-sslrootcert` - Path to the CA certificate file used to verify
  the server. Takes effect only under `-db-sslmode` `require`,
  `verify-ca`, or `verify-full`; under `disable`, `allow`, or `prefer`
  (the default) the certificate is never checked against it and the
  value is silently ignored, matching libpq.
- `-db-hosts` - Comma-separated `host:port` pairs for multi-host
  failover (e.g., `host1:5432,host2:5432`)
- `-db-target-session-attrs` - Session routing attribute for
  multi-host connections (libpq default: `any`)

These flags apply to the first database in the configuration list.

**Authentication Options:**

- `-no-auth` - Disable API token authentication
- `-token-file` - Path to token file
  (default: {binary_dir}/postgres-mcp-tokens.yaml)
- `-user-file` - Path to user authentication file
  (default: {binary_dir}/postgres-mcp-users.yaml)

**Token Management Options:**

- `-add-token` - Add a new API token
- `-remove-token` - Remove token by ID or hash prefix
- `-list-tokens` - List all API tokens
- `-token-note` - Annotation for new token (with `-add-token`)
- `-token-expiry` - Token expiry duration: "30d", "1y", "2w",
  "12h", "never" (with `-add-token`)
- `-token-database` - Bind token to specific database name
  (with `-add-token`, empty = first configured database)

**User Management Options:**

- `-add-user` - Add a new user
- `-update-user` - Update an existing user
- `-delete-user` - Delete a user
- `-list-users` - List all users
- `-enable-user` - Enable a user account
- `-disable-user` - Disable a user account
- `-username` - Username for user management commands
- `-password` - Password for user management commands (prompted if
  not provided)
- `-user-note` - Annotation for the new user (with `-add-user`)

See [Authentication Guide](authentication.md) for details on token
and user management.

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

