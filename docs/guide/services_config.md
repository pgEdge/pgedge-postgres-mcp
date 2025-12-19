# Managing your Services

The MCP configuration properties allow you to streamline configuration of:

* an HTTP server.
* the systemd service manager.
* the nginx reverse proxy.

Each component serves a specific role: the HTTP server handles requests, systemd keeps the service running, and nginx provides TLS termination and routing.

## Configuring an HTTP Server

The pgEdge MCP server and Natural Language Agent run as an HTTP/HTTPS service. After building and deploying the server in either containers or from source, you'll need to configure the deployment to work with your HTTP Server.

The following properties are required to configure a basic HTTP server:

```bash
# Set database connection
export PGHOST=localhost PGPORT=5432 PGDATABASE=mydb
export PGUSER=myuser PGPASSWORD=mypass

# Start the HTTP server
./bin/pgedge-postgres-mcp -http
```

To use HTTPS with TLS, you will need to add in certificate properties:

```bash
# Self-signed certificate (testing only)
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt \
  -days 365 -nodes -subj "/CN=localhost"

# Start the HTTPS server
./bin/pgedge-postgres-mcp -http -tls -cert server.crt -key server.key
```

For production, use certificates from Let's Encrypt or your CA:

```bash
./bin/pgedge-postgres-mcp -http -tls \
  -cert /etc/letsencrypt/live/domain.com/fullchain.pem \
  -key /etc/letsencrypt/live/domain.com/privkey.pem
```

You can use the following command line options to manage your HTTP service; these properties override properties specified in the configuration file:

| Flag | Description |
|------|-------------|
| `-http` | Enable HTTP mode |
| `-addr :PORT` | Listen address (default: `:8080`) |
| `-tls` | Enable HTTPS |
| `-cert PATH` | TLS certificate file |
| `-key PATH` | TLS private key file |
| `-no-auth` | Disable authentication (dev only) |
| `-config PATH` | Configuration file path |

### Example - HTTP/HTTPS Configuration Properties

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
    token_file: ""  # defaults to {binary_dir}/pgedge-postgres-mcp-tokens.yaml
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
secret_file: ""  # defaults to pgedge-postgres-mcp.secret, auto-generated if not present

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
#   prompts:
#     explore_database: true
#     setup_semantic_search: true
#     diagnose_query_issue: true
#     design_schema: true

```


## Adding the Deployment to your Systemd Service

When deploying the MCP server or Natural Language Agent in a Linux production environment, include the appropriate configuration in your systemd service unit.

To add the MCP server to your systemd service, you will need to specify server properties in a file located in:

`/etc/systemd/system/pgedge-postgres-mcp.service`

Include the following properties in the service definition:

```ini
[Unit]
Description=pgEdge Natural Language Agent
After=network.target postgresql.service

[Service]
Type=simple
User=pgedge
Group=pgedge
WorkingDirectory=/opt/pgedge
ExecStart=/opt/pgedge/bin/pgedge-postgres-mcp -config /etc/pgedge/config.yaml
Restart=always
RestartSec=10

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
```

After creating the file, enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable pgedge-postgres-mcp
sudo systemctl start pgedge-postgres-mcp
sudo systemctl status pgedge-postgres-mcp
```

Use the following command to view the service logs:

```bash
journalctl -u pgedge-postgres-mcp -f
```

---

## Configuring a Reverse Proxy

If you're running the MCP server in a production environment, we recommend running behind [nginx](https://nginx.org/en/docs/index.html) with TLS termination.  The following code snippet is the content of a server block that configures nginx with the MCP server:

```nginx
server {
    listen 443 ssl http2;
    server_name mcp.example.com;

    ssl_certificate /etc/letsencrypt/live/mcp.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/mcp.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name mcp.example.com;
    return 301 https://$host$request_uri;
}
```