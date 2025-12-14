# Configuring an HTTP Server

The pgEdge MCP server and Natural Language Agent run as an HTTP/HTTPS service. After building and deploying the server in either containers or from source, you'll need to configure the deployment to work with your HTTP Server.

## Configuring a Basic HTTP Server

```bash
# Set database connection
export PGHOST=localhost PGPORT=5432 PGDATABASE=mydb
export PGUSER=myuser PGPASSWORD=mypass

# Start HTTP server
./bin/pgedge-mcp-server -http
```

### HTTPS with TLS

```bash
# Self-signed certificate (testing only)
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt \
  -days 365 -nodes -subj "/CN=localhost"

# Start HTTPS server
./bin/pgedge-mcp-server -http -tls -cert server.crt -key server.key
```

For production, use certificates from Let's Encrypt or your CA:

```bash
./bin/pgedge-mcp-server -http -tls \
  -cert /etc/letsencrypt/live/domain.com/fullchain.pem \
  -key /etc/letsencrypt/live/domain.com/privkey.pem
```

### Command Line Options

| Flag | Description |
|------|-------------|
| `-http` | Enable HTTP mode |
| `-addr :PORT` | Listen address (default: `:8080`) |
| `-tls` | Enable HTTPS |
| `-cert PATH` | TLS certificate file |
| `-key PATH` | TLS private key file |
| `-no-auth` | Disable authentication (dev only) |
| `-config PATH` | Configuration file path |

---

## Example Configuration

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