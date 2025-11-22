# Internal Architecture

This guide covers the internal architecture, implementation details, and
development workflows for contributors to the pgEdge Natural Language Agent
project.

## Request Flow

```
1. Browser → nginx (port 8081)
2. nginx → MCP Server (port 8080) for /mcp/v1 and /api/* requests
3. MCP Server validates session token
4. MCP Server routes request:
   - /mcp/v1 → JSON-RPC handler
   - /api/llm/* → LLM proxy handler
   - /api/user/info → User info handler
5. Response → nginx → Browser
```

## Session Token Management

**Server-side:**

- User accounts stored in `pgedge-pg-mcp-svr-users.yaml`
- Session tokens stored in memory only (not persisted to disk)
- Each authenticated user receives a session token with 24-hour expiration
- Tokens validated on every request via `Authorization: Bearer <token>` header

**Client-side:**

- Session token stored in `localStorage` as `mcp-session-token`
- Token sent with every request in Authorization header
- Token cleared on logout or validation failure

**Implementation:** [internal/users/users.go](../internal/users/users.go)

## Database Connection Per Session

Each session token is associated with a separate database connection pool:

```go
// internal/database/connection.go

// ConnectionManager manages per-session database connections
type ConnectionManager struct {
    configs map[string]*Config  // sessionToken -> db config
    pools   map[string]*sql.DB  // sessionToken -> connection pool
    mu      sync.RWMutex
}

func (m *ConnectionManager) GetConnection(sessionToken string) (*sql.DB, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    pool, exists := m.pools[sessionToken]
    if !exists {
        return nil, fmt.Errorf("no connection for session")
    }

    return pool, nil
}
```

This ensures:

- Connection isolation between users
- Per-user database credentials
- Proper connection cleanup on session expiry

## LLM Client Abstraction

The LLM proxy uses a unified client interface for all providers:

```go
// internal/chat/llm.go

type LLMClient interface {
    Chat(ctx context.Context, messages []Message, tools interface{}) (LLMResponse, error)
    ListModels(ctx context.Context) ([]string, error)
}

type anthropicClient struct { /* ... */ }
type openaiClient struct { /* ... */ }
type ollamaClient struct { /* ... */ }
```

Each client implements provider-specific API calls while presenting a
consistent interface.

**Implementation:** [internal/chat/llm.go](../internal/chat/llm.go)

## Docker Container Architecture

### Container Communication

```
┌──────────────────┐
│  web-client:8081 │
│  (nginx + React) │
└────────┬─────────┘
         │ Docker network: pgedge-network
         │ Internal hostname: mcp-server
         ▼
┌──────────────────┐     ┌──────────────┐
│  mcp-server:8080 │────▶│  PostgreSQL  │
│  (Go binary)     │     │  (external)  │
└──────────────────┘     └──────────────┘
```

**Key points:**

- Web client proxies `/mcp/v1` and `/api/*` to `http://mcp-server:8080`
- MCP server connects to external PostgreSQL via configured host
- All services on `pgedge-network` Docker bridge network

### nginx Configuration

**File:** [docker/nginx.conf](../docker/nginx.conf)

```nginx
# Proxy JSON-RPC requests to MCP server
location /mcp/v1 {
    proxy_pass http://mcp-server:8080/mcp/v1;
    proxy_http_version 1.1;
    proxy_set_header Authorization $http_authorization;
    # ... other headers
}

# Proxy API requests to MCP server
location /api/ {
    proxy_pass http://mcp-server:8080/api/;
    proxy_http_version 1.1;
    proxy_set_header Authorization $http_authorization;
    # ... other headers
}

# SPA routing - serve index.html for all other routes
location / {
    try_files $uri $uri/ /index.html;
}
```

### Build Process

**Web Client Build:**

```dockerfile
# Stage 1: Build React app
FROM nodejs:20 AS builder
WORKDIR /workspace
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Serve with nginx
FROM nginx:latest
COPY --from=builder /workspace/dist /opt/app-root/src
COPY docker/nginx.conf /etc/nginx/nginx.conf
```

**MCP Server Build:**

```dockerfile
# Stage 1: Build Go binary
FROM golang:1.23-alpine AS builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o pgedge-pg-mcp-svr ./cmd/pgedge-pg-mcp-svr

# Stage 2: Minimal runtime
FROM ubi9/ubi-minimal:latest
COPY --from=builder /workspace/pgedge-pg-mcp-svr /app/
COPY docker/init-server.sh /app/
CMD ["/app/init-server.sh"]
```

## Development Workflow

### Running Locally (Development Mode)

```bash
# Terminal 1: Start MCP server
cd bin
./pgedge-pg-mcp-svr -http -addr :8080 -config pgedge-pg-mcp-web.yaml

# Terminal 2: Start Vite dev server
cd web
npm run dev
```

The Vite dev server (port 5173) proxies `/mcp/v1` and `/api/*` to
`localhost:8080`.

### Running in Docker (Production Mode)

```bash
# Build and start all containers
docker-compose up -d

# View logs
docker-compose logs -f

# Access web client
open http://localhost:8081
```

### Adding New LLM Proxy Endpoints

1. Add handler function in `internal/llmproxy/proxy.go`
2. Register handler in `cmd/pgedge-pg-mcp-svr/main.go`:

```go
func SetupHandlers(mux *http.ServeMux, llmProxyConfig *llmproxy.Config) {
    // Existing handlers...

    // Add new handler
    mux.HandleFunc("/api/llm/my-endpoint", func(w http.ResponseWriter, r *http.Request) {
        llmproxy.HandleMyEndpoint(w, r, llmProxyConfig)
    })
}
```

3. Update web client to call new endpoint
4. Rebuild containers

### Adding New MCP Tools

1. Create tool implementation in `internal/tools/`
2. Register tool in `internal/tools/registry.go`:

```go
func RegisterTools(registry *Registry, db *database.Client, llm *llm.Client) {
    // Existing tools...

    // Add new tool
    registry.Register(Tool{
        Name:        "my_tool",
        Description: "Description of my tool",
        InputSchema: schema,
        Handler:     myToolHandler,
    })
}
```

3. Tool automatically available via `tools/list` and `tools/call`

## Security Considerations

### API Key Management

**Never expose API keys to the browser:**

- ✅ Store keys in server environment variables or files
- ✅ Use LLM proxy endpoints from web client
- ❌ Never send API keys to browser
- ❌ Never store API keys in localStorage

### Session Token Security

- Tokens stored in `localStorage` (XSS vulnerable)
- Use HTTPS in production to prevent MITM attacks
- Set appropriate token expiration times
- Implement token refresh mechanism if needed

### Database Connection Security

- Use SSL/TLS for database connections (`PGEDGE_DB_SSLMODE=require`)
- Use per-session database credentials when possible
- Validate all user inputs before executing SQL
- Use parameterized queries to prevent SQL injection

### Docker Security

- Run containers as non-root user (UID 1001)
- Use minimal base images (UBI Micro)
- Scan images for vulnerabilities
- Use secrets management for production (not `.env` file)

## Performance Optimization

### Database Connection Pooling

Each session token has its own connection pool to ensure isolation while
maintaining performance:

```go
pool.SetMaxOpenConns(10)
pool.SetMaxIdleConns(5)
pool.SetConnMaxLifetime(time.Hour)
```

### LLM Response Caching

Consider implementing response caching for identical queries:

```go
type CachedResponse struct {
    key       string
    response  LLMResponse
    timestamp time.Time
}
```

### Async Tool Execution

For tools that don't depend on each other, execute them in parallel:

```javascript
const toolResults = await Promise.all(
    toolUses.map(toolUse => mcpClient.callTool(toolUse.name, toolUse.input))
);
```

## Debugging

### Enable Debug Logging

```bash
# Server-side
export PGEDGE_DEBUG=true
export PGEDGE_DB_LOG_LEVEL=debug
export PGEDGE_LLM_LOG_LEVEL=debug

# Docker
docker-compose logs -f mcp-server | grep -i error
```

### Browser DevTools

1. Open DevTools → Network tab
2. Filter by `/mcp/v1` or `/api/llm`
3. Inspect request/response payloads
4. Check Authorization headers

### Common Issues

**401 Unauthorized:**
- Check session token is being sent
- Verify token hasn't expired
- Check token exists in users file

**404 Not Found:**
- Verify nginx is proxying correctly
- Check MCP server is running
- Verify endpoint path is correct

**Connection Refused:**
- Check MCP server is listening on port 8080
- Verify Docker network connectivity
- Check firewall rules

## See Also

- [Development Setup](development.md) - Development environment setup
- [Testing](testing.md) - Running tests and test coverage
- [CI/CD](ci-cd.md) - Continuous integration and deployment
- [Architecture](architecture.md) - High-level architecture overview
