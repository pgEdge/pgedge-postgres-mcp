# Deployment Guide

Deploy the Natural Language Agent as an HTTP/HTTPS service. Choose your method:

- [Docker Compose](#docker-compose) - Recommended for production
- [Native Binary](#native-binary) - Direct installation
- [Systemd Service](#systemd-service) - Linux service management

For Claude Desktop integration (stdio mode), see the
[Quick Start](../quickstart.md#using-with-claude-desktop).

---

## Docker Compose

The recommended deployment method using pre-built containers.

### Quick Start

```bash
# Clone repository
git clone https://github.com/pgEdge/pgedge-postgres-mcp.git
cd pgedge-postgres-mcp

# Configure
cp .env.example .env
# Edit .env with your settings

# Start all services
docker-compose up -d
```

### Available Containers

| Container | Port | Description |
|-----------|------|-------------|
| `mcp-server` | 8080 | MCP server with PostgreSQL access |
| `web-client` | 8081 | React web interface |

### Configuration

All settings are in the `.env` file:

```bash
# Database Connection
PGEDGE_DB_HOST=host.docker.internal
PGEDGE_DB_PORT=5432
PGEDGE_DB_NAME=mydb
PGEDGE_DB_USER=postgres
PGEDGE_DB_PASSWORD=secret
PGEDGE_DB_SSLMODE=prefer

# LLM Provider
PGEDGE_LLM_PROVIDER=anthropic  # or openai, ollama
PGEDGE_LLM_MODEL=claude-sonnet-4-20250514
PGEDGE_ANTHROPIC_API_KEY=sk-ant-...

# Authentication
INIT_USERS=admin:password123
# Or for API tokens: INIT_TOKENS=token1,token2

# Optional: Embeddings
PGEDGE_EMBEDDING_ENABLED=true
PGEDGE_EMBEDDING_PROVIDER=voyage
PGEDGE_EMBEDDING_MODEL=voyage-3
```

!!! tip "API Key Security"
    For production, mount API key files instead of using environment variables:
    ```yaml
    volumes:
      - ~/.anthropic-api-key:/app/.anthropic-api-key:ro
    ```

### Data Persistence

The MCP server stores persistent data in a dedicated directory:

- **Authentication tokens** (`tokens.json`)
- **User credentials** (`users.json`)
- **Conversation history** (`conversations.db`)
- **User preferences**

The Docker Compose configuration mounts a named volume (`mcp-data`) to
`/app/data`, ensuring data persists across container restarts.

To use a custom host path instead of a named volume:

```yaml
volumes:
  # Mount host directory instead of named volume
  - ./data:/app/data
```

!!! warning "Permissions"
    Ensure the host directory has appropriate permissions (owned by UID 1001)
    or the container may fail to start:
    ```bash
    mkdir -p ./data && chown 1001:1001 ./data
    ```

You can also configure a custom data directory location via environment
variable:

```bash
PGEDGE_DATA_DIR=/var/lib/pgedge/data
```

### Container Management

```bash
# View logs
docker-compose logs -f mcp-server

# Restart services
docker-compose restart

# Stop services
docker-compose down

# Rebuild after code changes
docker-compose build && docker-compose up -d
```

### Connecting to Host PostgreSQL

Use `host.docker.internal` instead of `localhost`:

```bash
PGEDGE_DB_HOST=host.docker.internal
```

On Linux, you may need to use the Docker bridge IP (`172.17.0.1`).

---

## Native Binary

### Build

```bash
git clone https://github.com/pgEdge/pgedge-postgres-mcp.git
cd pgedge-postgres-mcp
make build
```

### Basic HTTP Server

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

## Systemd Service

For Linux production deployments.

### Create Service File

`/etc/systemd/system/pgedge-mcp-server.service`:

```ini
[Unit]
Description=pgEdge Natural Language Agent
After=network.target postgresql.service

[Service]
Type=simple
User=pgedge
Group=pgedge
WorkingDirectory=/opt/pgedge
ExecStart=/opt/pgedge/bin/pgedge-mcp-server -config /etc/pgedge/config.yaml
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

### Enable and Start

```bash
sudo systemctl daemon-reload
sudo systemctl enable pgedge-mcp-server
sudo systemctl start pgedge-mcp-server
sudo systemctl status pgedge-mcp-server
```

### View Logs

```bash
journalctl -u pgedge-mcp-server -f
```

---

## Reverse Proxy

For production, run behind nginx with TLS termination.

### Nginx Configuration

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

---

## Health Checks

All deployment methods expose a health endpoint:

```bash
curl http://localhost:8080/health
```

Response:
```json
{"status": "ok", "server": "pgedge-mcp-server", "version": "1.0.0"}
```

---

## Troubleshooting

### Port Already in Use

```bash
lsof -i :8080
# Kill the process or use a different port with -addr
```

### Database Connection Failed

```bash
# Test connection directly
psql -h localhost -U postgres -d mydb -c "SELECT 1"

# Check environment variables
env | grep PG
```

### Docker Can't Reach Host Database

- macOS/Windows: Use `host.docker.internal`
- Linux: Use `172.17.0.1` or configure Docker network

### Certificate Issues

```bash
# Verify certificate matches key
openssl x509 -noout -modulus -in server.crt | openssl md5
openssl rsa -noout -modulus -in server.key | openssl md5
# Both should match

# Check expiration
openssl x509 -in server.crt -noout -dates
```

---

## See Also

- [Configuration](configuration.md) - All configuration options
- [Authentication](authentication.md) - User and token setup
- [Security](security.md) - Security best practices
