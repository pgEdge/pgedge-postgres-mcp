# Deployment Guide

This guide covers deploying the pgEdge MCP Server as an HTTP/HTTPS service for direct API access.

## Transport Modes

The server supports two transport modes:

1. **stdio mode (default)**: JSON-RPC over standard input/output - used by Claude Desktop
2. **HTTP/HTTPS mode**: JSON-RPC over HTTP - for direct API access, web applications, and external integrations

This guide focuses on HTTP/HTTPS mode. For stdio mode (Claude Desktop), see the main [Configuration Guide](configuration.md).

## Quick Start

### Basic HTTP Server

```bash
# Set environment variables
export ANTHROPIC_API_KEY="sk-ant-your-key"

# Configure database connection (example using environment variables)
export PGHOST="localhost"
export PGPORT="5432"
export PGDATABASE="mydb"
export PGUSER="myuser"
export PGPASSWORD="mypass"

# Start HTTP server on default port 8080
./bin/pgedge-pg-mcp-svr -http
```

### With Custom Port

```bash
./bin/pgedge-pg-mcp-svr -http -addr ":3000"
```

### Production HTTPS Server

```bash
./bin/pgedge-pg-mcp-svr -http -tls \
  -cert /path/to/server.crt \
  -key /path/to/server.key \
  -chain /path/to/ca-chain.crt
```

## Command Line Options

```bash
./bin/pgedge-pg-mcp-svr [options]

HTTP/HTTPS Options:
  -http              Enable HTTP transport mode (default: stdio)
  -addr string       HTTP server address (default ":8080")
  -tls               Enable TLS/HTTPS (requires -http)
  -cert string       Path to TLS certificate file
  -key string        Path to TLS key file
  -chain string      Path to TLS certificate chain file (optional)
  -no-auth           Disable API token authentication
  -token-file        Path to API token file (default: {binary_dir}/pgedge-pg-mcp-svr-tokens.yaml)
```

**Note**: TLS options (`-tls`, `-cert`, `-key`, `-chain`) require the `-http` flag.

For configuration file setup, see [Configuration Guide](configuration.md).

## HTTP Mode

### Server Endpoints

The server provides two endpoints:

- **POST /mcp/v1**: JSON-RPC 2.0 endpoint for MCP requests
- **GET /health**: Health check endpoint (no authentication required)

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "ok",
  "server": "pgedge-pg-mcp-svr",
  "version": "1.0.0-alpha1"
}
```

### Making MCP Requests

First, create an API token (see [Authentication Guide](authentication.md) for details):

```bash
./bin/pgedge-pg-mcp-svr -add-token -token-note "Test" -token-expiry "30d"
```

Then make requests with the token:

```bash
# Initialize connection
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {
        "name": "test-client",
        "version": "1.0.0"
      }
    }
  }'

# List available tools
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list",
    "params": {}
  }'

# Execute a natural language query
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "query_database",
      "arguments": {
        "query": "Show me all users who registered in the last week"
      }
    }
  }'
```

### Development Mode (No Authentication)

**Warning**: Only use this for local development. Never in production.

```bash
./bin/pgedge-pg-mcp-svr -http -no-auth
```

## HTTPS Mode (TLS)

### Self-Signed Certificates (Testing Only)

Generate a self-signed certificate for testing:

```bash
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt \
  -days 365 -nodes -subj "/CN=localhost"
```

Start HTTPS server:

```bash
./bin/pgedge-pg-mcp-svr -http -tls \
  -cert server.crt \
  -key server.key
```

Test with curl (note the `-k` flag to accept self-signed cert):

```bash
curl -k https://localhost:8080/health
```

### Production Certificates (Let's Encrypt)

#### Using Certbot

```bash
# Install certbot
sudo apt-get install certbot  # Ubuntu/Debian
brew install certbot          # macOS

# Generate certificate
sudo certbot certonly --standalone -d yourdomain.com

# Certificates will be in:
# /etc/letsencrypt/live/yourdomain.com/fullchain.pem (cert + chain)
# /etc/letsencrypt/live/yourdomain.com/privkey.pem (private key)
```

#### Start Server with Let's Encrypt Certificates

```bash
./bin/pgedge-pg-mcp-svr -http -tls \
  -cert /etc/letsencrypt/live/yourdomain.com/fullchain.pem \
  -key /etc/letsencrypt/live/yourdomain.com/privkey.pem
```

### Production Certificates (CA-Signed)

For CA-signed certificates from a certificate authority:

```bash
./bin/pgedge-pg-mcp-svr -http -tls \
  -cert /path/to/server.crt \
  -key /path/to/server.key \
  -chain /path/to/ca-chain.crt
```

### Certificate Requirements

- **TLS Version**: Minimum 1.2 (1.3 recommended)
- **Format**: Certificate and key must be PEM-encoded
- **Chain File**: Optional but recommended for CA-signed certificates
- **Permissions**: Private key should be `chmod 600` (owner read/write only)
- **Location**: If paths not specified, server looks in binary directory

### Testing HTTPS

```bash
# With self-signed certificate (testing only)
curl -k https://localhost:8080/health

# With trusted certificate
curl https://yourdomain.com:8080/health

# Test TLS version and cipher
openssl s_client -connect localhost:8080 -tls1_2
```

## Production Deployment Patterns

### Systemd Service

Create `/etc/systemd/system/pgedge-mcp.service`:

```ini
[Unit]
Description=pgEdge MCP Server
After=network.target postgresql.service

[Service]
Type=simple
User=pgedge
Group=pgedge
WorkingDirectory=/opt/pgedge-mcp
ExecStart=/opt/pgedge-mcp/bin/pgedge-pg-mcp-svr -config /etc/pgedge-mcp/config.yaml
Restart=always
RestartSec=10

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/pgedge-mcp

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable pgedge-mcp
sudo systemctl start pgedge-mcp
sudo systemctl status pgedge-mcp
```

### Docker Deployment

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /build
COPY . .
RUN go build -o pgedge-pg-mcp-svr ./cmd/pgedge-pg-mcp-svr

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/pgedge-pg-mcp-svr .
COPY configs/pgedge-pg-mcp-svr.yaml.example config.yaml

EXPOSE 8080
ENTRYPOINT ["./pgedge-pg-mcp-svr"]
CMD ["-config", "config.yaml", "-http"]
```

Build and run:

```bash
docker build -t pgedge-mcp .
docker run -d \
  -p 8080:8080 \
  -e ANTHROPIC_API_KEY="sk-ant-..." \
  -e PGHOST="host.docker.internal" \
  -e PGPORT="5432" \
  -e PGDATABASE="mydb" \
  -e PGUSER="myuser" \
  -e PGPASSWORD="mypass" \
  --name pgedge-mcp \
  pgedge-mcp

# Note: Use host.docker.internal to access PostgreSQL on host machine from container
```

### Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  pgedge-mcp:
    build: .
    ports:
      - "8080:8080"
    environment:
      ANTHROPIC_API_KEY: sk-ant-your-key
      PGHOST: db
      PGPORT: 5432
      PGDATABASE: mydb
      PGUSER: postgres
      PGPASSWORD: password
    depends_on:
      - db
    restart: unless-stopped

  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: password
      POSTGRES_DB: mydb
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
```

Start:

```bash
docker-compose up -d
```

### Reverse Proxy (Nginx)

Create `/etc/nginx/sites-available/pgedge-mcp`:

```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=mcp:10m rate=10r/s;
    limit_req zone=mcp burst=20 nodelay;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support (if needed in future)
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name yourdomain.com;
    return 301 https://$server_name$request_uri;
}
```

Enable and reload:

```bash
sudo ln -s /etc/nginx/sites-available/pgedge-mcp /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

## Load Balancing

For high-availability deployments, run multiple instances behind a load balancer.

### HAProxy Configuration

```haproxy
frontend http_front
    bind *:443 ssl crt /etc/ssl/certs/yourdomain.pem
    default_backend mcp_servers

backend mcp_servers
    balance roundrobin
    option httpchk GET /health
    http-check expect status 200
    server mcp1 10.0.1.10:8080 check
    server mcp2 10.0.1.11:8080 check
    server mcp3 10.0.1.12:8080 check
```

## Monitoring and Observability

### Health Checks

```bash
# Simple health check
curl http://localhost:8080/health

# Health check with timeout
curl --max-time 5 http://localhost:8080/health || echo "Service down"
```

### Log Monitoring

The server writes logs to stderr:

```bash
# Systemd logs
journalctl -u pgedge-mcp -f

# Docker logs
docker logs -f pgedge-mcp

# File logging (redirect stderr)
./bin/pgedge-pg-mcp-svr -http 2>> /var/log/pgedge-mcp/server.log
```

## Security Best Practices

1. **Always use authentication** - See [Authentication Guide](authentication.md)
2. **Use HTTPS in production** - Never HTTP for external access
3. **Restrict network access** - Use firewall rules and private networks
4. **Rotate certificates** - Set up automatic renewal for Let's Encrypt
5. **Use reverse proxy** - Add rate limiting and DDoS protection
6. **Monitor logs** - Set up alerting for errors and suspicious activity

For comprehensive security guidance, see [Security Guide](security.md).

## Troubleshooting

### Server Won't Start

```bash
# Check if port is already in use
lsof -i :8080
netstat -tlnp | grep 8080

# Check file permissions
ls -la bin/pgedge-pg-mcp-svr
ls -la /path/to/server.key  # Should be 600

# Test with verbose logging
./bin/pgedge-pg-mcp-svr -http -addr ":8080" 2>&1 | tee debug.log
```

### Connection Refused

```bash
# Verify server is running
ps aux | grep pgedge-pg-mcp-svr

# Check firewall
sudo ufw status
sudo iptables -L

# Test local connection
curl http://localhost:8080/health
```

### Certificate Issues

```bash
# Verify certificate and key match
openssl x509 -noout -modulus -in server.crt | openssl md5
openssl rsa -noout -modulus -in server.key | openssl md5
# These should match

# Check certificate expiration
openssl x509 -in server.crt -noout -dates

# Verify certificate chain
openssl verify -CAfile ca-chain.crt server.crt
```

For more troubleshooting help, see the [Troubleshooting Guide](troubleshooting.md).

## Related Documentation

- [Configuration Guide](configuration.md) - Configuration file and environment setup
- [Authentication Guide](authentication.md) - API token management
- [Security Guide](security.md) - Security best practices
- [MCP Protocol Guide](mcp_protocol.md) - Protocol implementation details
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
