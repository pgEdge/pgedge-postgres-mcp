# Docker Deployment

The pgEdge Postgres MCP provides Docker containers for easy deployment of the MCP server and web client. This guide covers building, configuring, and running the containerized services.

## Quick Start

The fastest way to get started with Docker:

```bash
# 1. Copy the example environment file
cp .env.example .env

# 2. Edit .env with your configuration
nano .env  # or your preferred editor

# 3. Build and start all services
docker-compose up -d

# 4. Access the web client
open http://localhost:8081
```

## Available Containers

### MCP Server

The MCP server container provides the backend Model Context Protocol service with PostgreSQL database access.

- **Base Image**: `golang:1.23-alpine` (builder), `registry.access.redhat.com/ubi9/ubi-minimal:latest` (runtime)
- **Build**: Multi-stage build with Go 1.23 compilation
- **Size**: ~177MB (includes shadow-utils for proper volume permission handling)
- **Port**: 8080 (HTTP API)
- **Dockerfile**: [Dockerfile.server](../Dockerfile.server)

### Web Client

The web client container provides a browser-based chat interface.

- **Base Image**: `registry.access.redhat.com/ubi9/nodejs-20:latest` (builder), `registry.access.redhat.com/ubi9/nginx-124:latest` (runtime)
- **Build**: Multi-stage build with Node.js/Vite compilation and nginx serving
- **Size**: ~501MB (includes nginx, static assets, and dependencies)
- **Port**: 8081 (HTTP)
- **Dockerfile**: [Dockerfile.web](../Dockerfile.web)

### CLI Client

The CLI client container provides an interactive command-line interface.

- **Base Image**: `golang:1.23-alpine` (builder), `registry.access.redhat.com/ubi9/ubi-micro:latest` (runtime)
- **Build**: Multi-stage build with Go 1.23 compilation
- **Size**: Minimal (~48MB)
- **Usage**: Interactive terminal
- **Dockerfile**: [Dockerfile.cli](../Dockerfile.cli)

## Configuration

### Environment Variables

All configuration is done through environment variables in the `.env` file. Copy [.env.example](../.env.example) to `.env` and customize:

```bash
cp .env.example .env
```

Key configuration sections:

#### Database Connection

```bash
PGEDGE_DB_HOST=your-postgres-host
PGEDGE_DB_PORT=5432
PGEDGE_DB_NAME=your-database-name
PGEDGE_DB_USER=your-database-user
PGEDGE_DB_PASSWORD=your-database-password
PGEDGE_DB_SSLMODE=prefer
```

#### Embedding Provider

```bash
PGEDGE_EMBEDDING_PROVIDER=anthropic  # anthropic, openai, or ollama
PGEDGE_EMBEDDING_MODEL=voyage-3
```

#### API Keys

```bash
# For Anthropic (Claude + Voyage embeddings)
PGEDGE_ANTHROPIC_API_KEY=your-anthropic-api-key-here

# For OpenAI (GPT + OpenAI embeddings)
PGEDGE_OPENAI_API_KEY=your-openai-api-key-here

# For Ollama (local models)
PGEDGE_OLLAMA_URL=http://localhost:11434
```

#### Authentication

**Token-based authentication** (simpler, for trusted environments):

```bash
PGEDGE_AUTH_MODE=token
INIT_TOKENS=token1,token2,token3
```

**User-based authentication** (more secure):

```bash
PGEDGE_AUTH_MODE=user
INIT_USERS=alice:password123,bob:password456
```

#### LLM Configuration for Clients

```bash
PGEDGE_LLM_PROVIDER=anthropic  # anthropic, openai, or ollama
PGEDGE_LLM_MODEL=claude-sonnet-4-20250514
```

See [.env.example](../.env.example) for complete configuration options and examples.

## Building Containers

### Build All Containers

```bash
docker-compose build
```

### Build Individual Containers

```bash
# MCP server
docker build -f Dockerfile.server -t pgedge-mcp-server .

# Web client
docker build -f Dockerfile.web -t pgedge-mcp-web .

# CLI client
docker build -f Dockerfile.cli -t pgedge-mcp-cli .
```

### Build with Custom Registry

```bash
# Tag for your registry
docker build -f Dockerfile.server -t registry.example.com/pgedge-mcp-server:latest .
docker build -f Dockerfile.web -t registry.example.com/pgedge-mcp-web:latest .

# Push to registry
docker push registry.example.com/pgedge-mcp-server:latest
docker push registry.example.com/pgedge-mcp-web:latest
```

## Running with Docker Compose

### Start All Services

```bash
# Start in background
docker-compose up -d

# Start with logs
docker-compose up

# Start specific services
docker-compose up -d mcp-server web-client
```

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f mcp-server
docker-compose logs -f web-client
```

### Stop Services

```bash
# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v
```

### Restart Services

```bash
# Restart all
docker-compose restart

# Restart specific service
docker-compose restart mcp-server
```

## Running Individual Containers

### MCP Server

```bash
docker run -d \
  --name pgedge-mcp-server \
  -p 8080:8080 \
  -e PGEDGE_DB_HOST=postgres.example.com \
  -e PGEDGE_DB_PORT=5432 \
  -e PGEDGE_DB_NAME=mydb \
  -e PGEDGE_DB_USER=postgres \
  -e PGEDGE_DB_PASSWORD=secret \
  -e PGEDGE_ANTHROPIC_API_KEY=sk-ant-... \
  -e INIT_TOKENS=my-secret-token \
  -v pgedge-data:/app/data \
  pgedge-mcp-server
```

### Web Client

```bash
docker run -d \
  --name pgedge-mcp-web \
  -p 8081:8081 \
  --link pgedge-mcp-server:mcp-server \
  pgedge-mcp-web
```

### CLI Client (Interactive)

```bash
docker run -it --rm \
  --name pgedge-mcp-cli \
  --link pgedge-mcp-server:mcp-server \
  -e PGEDGE_MCP_MODE=http \
  -e PGEDGE_MCP_URL=http://mcp-server:8080 \
  -e PGEDGE_MCP_TOKEN=my-secret-token \
  -e PGEDGE_ANTHROPIC_API_KEY=sk-ant-... \
  pgedge-mcp-cli
```

## Usage Examples

### Example 1: Development Setup with Anthropic

Create `.env`:

```bash
# Database
PGEDGE_DB_HOST=localhost
PGEDGE_DB_PORT=5432
PGEDGE_DB_NAME=myapp_dev
PGEDGE_DB_USER=postgres
PGEDGE_DB_PASSWORD=dev_password

# Embeddings
PGEDGE_EMBEDDING_PROVIDER=anthropic
PGEDGE_EMBEDDING_MODEL=voyage-3
PGEDGE_ANTHROPIC_API_KEY=sk-ant-api03-...

# Authentication
PGEDGE_AUTH_MODE=token
INIT_TOKENS=dev-token-1

# LLM
PGEDGE_LLM_PROVIDER=anthropic
PGEDGE_LLM_MODEL=claude-sonnet-4-20250514

# Debug
PGEDGE_DEBUG=true
PGEDGE_DB_LOG_LEVEL=debug
PGEDGE_LLM_LOG_LEVEL=debug
```

Start services:

```bash
docker-compose up -d
```

Access web client at http://localhost:8081

### Example 2: Production Setup with User Authentication

Create `.env`:

```bash
# Database
PGEDGE_DB_HOST=postgres.prod.example.com
PGEDGE_DB_PORT=5432
PGEDGE_DB_NAME=production_db
PGEDGE_DB_USER=mcp_user
PGEDGE_DB_PASSWORD=secure_prod_password
PGEDGE_DB_SSLMODE=require

# Embeddings
PGEDGE_EMBEDDING_PROVIDER=anthropic
PGEDGE_EMBEDDING_MODEL=voyage-3
PGEDGE_ANTHROPIC_API_KEY=sk-ant-api03-...

# Authentication - user mode for production
PGEDGE_AUTH_MODE=user
INIT_USERS=alice:secure_password_1,bob:secure_password_2

# LLM
PGEDGE_LLM_PROVIDER=anthropic
PGEDGE_LLM_MODEL=claude-sonnet-4-20250514

# Logging
PGEDGE_DB_LOG_LEVEL=info
PGEDGE_LLM_LOG_LEVEL=info
```

Start services:

```bash
docker-compose up -d
```

### Example 3: Using Local Ollama for LLM and Embeddings

Create `.env`:

```bash
# Database
PGEDGE_DB_HOST=localhost
PGEDGE_DB_PORT=5432
PGEDGE_DB_NAME=myapp
PGEDGE_DB_USER=postgres
PGEDGE_DB_PASSWORD=password

# Embeddings with Ollama
PGEDGE_EMBEDDING_PROVIDER=ollama
PGEDGE_EMBEDDING_MODEL=nomic-embed-text
PGEDGE_OLLAMA_URL=http://host.docker.internal:11434

# Authentication
PGEDGE_AUTH_MODE=token
INIT_TOKENS=local-token

# LLM with Ollama
PGEDGE_LLM_PROVIDER=ollama
PGEDGE_LLM_MODEL=llama3
```

Make sure Ollama is running on your host:

```bash
# On your host machine
ollama serve

# Pull required models
ollama pull llama3
ollama pull nomic-embed-text
```

Start services:

```bash
docker-compose up -d
```

### Example 4: Using OpenAI

Create `.env`:

```bash
# Database
PGEDGE_DB_HOST=localhost
PGEDGE_DB_PORT=5432
PGEDGE_DB_NAME=myapp
PGEDGE_DB_USER=postgres
PGEDGE_DB_PASSWORD=password

# Embeddings with OpenAI
PGEDGE_EMBEDDING_PROVIDER=openai
PGEDGE_EMBEDDING_MODEL=text-embedding-3-small
PGEDGE_OPENAI_API_KEY=sk-proj-...

# Authentication
PGEDGE_AUTH_MODE=token
INIT_TOKENS=openai-token

# LLM with OpenAI
PGEDGE_LLM_PROVIDER=openai
PGEDGE_LLM_MODEL=gpt-4o
```

Start services:

```bash
docker-compose up -d
```

## Networking

### Service Communication

Services communicate via a Docker bridge network named `pgedge-network`. Internal service names:

- MCP Server: `mcp-server:8080`
- Web Client: `web-client:8081`

### Connecting to External PostgreSQL

If your PostgreSQL database is on the host machine:

```bash
# Use host.docker.internal instead of localhost
PGEDGE_DB_HOST=host.docker.internal
```

If using Docker Compose with a PostgreSQL container, add it to `docker-compose.yml`:

```yaml
services:
    postgres:
        image: postgres:16
        environment:
            POSTGRES_DB: mydb
            POSTGRES_USER: postgres
            POSTGRES_PASSWORD: password
        volumes:
            - postgres-data:/var/lib/postgresql/data
        networks:
            - pgedge-network

    mcp-server:
        # ... existing config ...
        environment:
            PGEDGE_DB_HOST: postgres
            # ... other vars ...

volumes:
    postgres-data:
```

## Persistent Data

Token and user data is stored in a Docker volume named `mcp-data`. This ensures data persists across container restarts.

### Backup Data

```bash
# Backup tokens and users
docker run --rm \
  -v pgedge-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/pgedge-data-backup.tar.gz -C /data .
```

### Restore Data

```bash
# Restore tokens and users
docker run --rm \
  -v pgedge-data:/data \
  -v $(pwd):/backup \
  alpine sh -c "cd /data && tar xzf /backup/pgedge-data-backup.tar.gz"
```

### Inspect Data Volume

```bash
docker run --rm -it \
  -v pgedge-data:/data \
  alpine ls -la /data
```

## Health Checks

Both the MCP server and web client include health checks:

### Check Service Health

```bash
# View health status
docker-compose ps

# MCP server health
curl http://localhost:8080/health

# Web client health
curl http://localhost:8081/health
```

### Custom Health Check Intervals

Modify `docker-compose.yml`:

```yaml
healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
    interval: 30s      # Check every 30 seconds
    timeout: 10s       # Timeout after 10 seconds
    retries: 5         # Retry 5 times before marking unhealthy
    start_period: 30s  # Wait 30 seconds before first check
```

## Troubleshooting

### Container Won't Start

Check logs:

```bash
docker-compose logs mcp-server
docker-compose logs web-client
```

Common issues:

1. **Database connection failed**: Verify `PGEDGE_DB_*` variables
2. **Missing API key**: Set `PGEDGE_ANTHROPIC_API_KEY` or `PGEDGE_OPENAI_API_KEY`
3. **Port already in use**: Change `MCP_SERVER_PORT` or `WEB_CLIENT_PORT` in `.env`

### Can't Connect to Database

If database is on host:

```bash
# macOS/Windows Docker Desktop
PGEDGE_DB_HOST=host.docker.internal

# Linux
PGEDGE_DB_HOST=172.17.0.1  # Docker bridge gateway
```

### Web Client Can't Reach MCP Server

Ensure services are on the same network:

```bash
docker network ls
docker network inspect pgedge-network
```

### Permission Denied Errors

All containers run as non-root user (UID 1001). Ensure volumes have correct permissions:

```bash
# Fix volume permissions
docker run --rm \
  -v pgedge-data:/data \
  alpine chown -R 1001:1001 /data
```

### Debug Mode

Enable debug logging:

```bash
PGEDGE_DEBUG=true
PGEDGE_DB_LOG_LEVEL=debug
PGEDGE_LLM_LOG_LEVEL=debug
```

Restart services:

```bash
docker-compose restart
docker-compose logs -f
```

## Production Considerations

### Security

1. **Use secrets management**: Don't commit `.env` to version control
2. **Enable user authentication**: Use `PGEDGE_AUTH_MODE=user` instead of tokens
3. **Use TLS**: Run behind a reverse proxy (nginx, Traefik) with HTTPS
4. **Network security**: Use Docker networks to isolate services
5. **Update regularly**: Keep base images and dependencies up to date

### Reverse Proxy Setup

Example nginx configuration:

```nginx
upstream mcp-server {
    server localhost:8080;
}

upstream web-client {
    server localhost:8081;
}

server {
    listen 443 ssl http2;
    server_name mcp.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location /mcp/ {
        proxy_pass http://mcp-server/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location / {
        proxy_pass http://web-client/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### Resource Limits

Add resource limits in `docker-compose.yml`:

```yaml
services:
    mcp-server:
        # ... existing config ...
        deploy:
            resources:
                limits:
                    cpus: '2'
                    memory: 2G
                reservations:
                    cpus: '1'
                    memory: 1G
```

### Monitoring

Use Docker health checks and external monitoring:

```bash
# Prometheus metrics (if implemented)
curl http://localhost:8080/metrics

# Container stats
docker stats pgedge-mcp-server pgedge-mcp-web-client
```

## See Also

- [Configuration Reference](configuration.md) - Detailed configuration options
- [Authentication](authentication.md) - User and token authentication setup
- [MCP Server](go-mcp-server.md) - MCP server documentation
- [Web Client](web-client.md) - Web client documentation
- [CLI Client](go-chat-client.md) - CLI client documentation
