# pgEdge Postgres MCP Server

[![CI - MCP Server](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/CI%20-%20MCP%20Server/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-server.yml)
[![CI - CLI Client](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/CI%20-%20CLI%20Client/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-cli-client.yml)
[![CI - Web Client](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/CI%20-%20Web%20Client/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-web-client.yml)
[![CI - Docker Compose](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/CI%20-%20Docker/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-docker.yml)
[![CI - Documentation](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/CI%20-%20Documentation/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-docs.yml)

A Model Context Protocol (MCP) server that enables **SQL queries** against
PostgreSQL databases through MCP-compatible clients like Claude Desktop.

> 🚧 **WARNING**: This code is in pre-release status and MUST NOT be put
> into production without thorough testing!

> ⚠️ **NOT FOR PUBLIC-FACING APPLICATIONS**: This MCP server provides LLMs
> with read access to your entire database schema and data. It should only be
> used for internal tools, developer workflows, or environments where all users
> are trusted. For public-facing applications, consider the
> [pgEdge RAG Server](https://github.com/pgedge/pgedge-rag-server) instead.
> See the [Choosing the Right Solution](docs/guide/mcp-vs-rag.md) guide for
> details.

## Key Features

- 🔒 **Read-Only Protection** - All queries run in read-only transactions
- 📊 **Resources** - Access PostgreSQL statistics and more
- 🛠️ **Tools** - Query execution, schema analysis, advanced hybrid search
  (BM25+MMR), embedding generation, resource reading, and more
- 🧠 **Prompts** - Guided workflows for semantic search setup, database
  exploration, query diagnostics, and more
- 💬 **Production Chat Client** - Full-featured Go client with Anthropic
  prompt caching (90% cost reduction)
- 🌐 **HTTP/HTTPS Mode** - Direct API access with token authentication
- 🖥️ **Web Interface** - Modern React-based UI with AI-powered chat for
  natural language database interaction
- 🐳 **Docker Support** - Complete containerized deployment with Docker
  Compose
- 🔐 **Secure** - TLS support, token auth, read-only enforcement
- 🔄 **Hot Reload** - Automatic reload of authentication files without server
  restart

## Quick Start

### 1. Installation

```bash
git clone <repository-url>
cd pgedge-postgres-mcp
make build
```

### 2. Configure for Claude Code and/or Claude Desktop

**Claude Code**: `.mcp.json` in each of your project directories  
**Claude Desktop on macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`  
**Claude Desktop on Windows**: `%APPDATA%\\Claude\\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/bin/pgedge-mcp-server"
    }
  }
}
```

### 3. Connect to Your Database

Update your Claude Code and/or Claude Desktop configuration to include database connection
parameters:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/bin/pgedge-mcp-server",
      "env": {
        "PGHOST": "localhost",
        "PGPORT": "5432",
        "PGDATABASE": "mydb",
        "PGUSER": "myuser",
        "PGPASSWORD": "mypass"
      }
    }
  }
}
```

Alternatively, use a `.pgpass` file for password management (recommended for
security):

```bash
# ~/.pgpass
localhost:5432:mydb:myuser:mypass
```

Then configure without PGPASSWORD in the config:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/bin/pgedge-mcp-server",
      "env": {
        "PGHOST": "localhost",
        "PGPORT": "5432",
        "PGDATABASE": "mydb",
        "PGUSER": "myuser"
      }
    }
  }
}
```

> **Note:** The server connects to the database at startup using standard
> PostgreSQL environment variables (PG*) or PGEDGE_DB_* variables. Passwords
> can be stored securely in `.pgpass` files.

## Example Queries

The MCP client (like Claude Desktop) can translate natural language to SQL,
which is then executed by this server.

**Schema Discovery:**
- Request schema information using the `get_schema_info` tool
- Execute SQL: `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public';`

**Data Analysis:**
- Execute SQL: `SELECT customer_id, SUM(order_total) FROM orders GROUP BY customer_id ORDER BY SUM(order_total) DESC LIMIT 10;`
- Execute SQL: `SELECT * FROM orders WHERE shipping_time > INTERVAL '7 days';`

**System Monitoring:**
- Use the `pg://stat/activity` resource for current connections
- Execute SQL: `SELECT schemaname, tablename, n_dead_tup FROM pg_stat_user_tables ORDER BY n_dead_tup DESC;`
- Execute SQL: `SELECT sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as cache_hit_ratio FROM pg_statio_user_tables;`

## HTTP/HTTPS Mode

Run as a standalone HTTP server for direct API access:

```bash
# HTTP
./bin/pgedge-mcp-server -http

# HTTPS with TLS
./bin/pgedge-mcp-server -http -tls \
  -cert server.crt \
  -key server.key
```

**API Endpoint:** `POST http://localhost:8080/mcp/v1`

Example request:
```bash
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "query_database",
      "arguments": {
        "natural_language_query": "Show all users"
      }
    }
  }'
```

## CLI Client

A production-ready, full-featured command-line chat interface is available for
interacting with your PostgreSQL database using natural language:

```bash
# Quick start - Stdio mode (MCP server as subprocess)
./start_cli_stdio.sh

# Quick start - HTTP mode (MCP server via HTTP with auth)
./start_cli_http.sh
```

**Features:**
- 💬 Natural language database queries powered by Claude, GPT, or Ollama
- 🔧 Dual mode support (stdio subprocess or HTTP API)
- 💰 Anthropic prompt caching (90% cost reduction on repeated queries)
- ⚡ Runtime configuration with slash commands
- 📝 Persistent command history with readline support
- 🎨 PostgreSQL-themed UI with animations

**Example queries:**
- What tables are in my database?
- Show me the 10 most recent orders
- Which customers have placed more than 5 orders?
- Find documents similar to 'PostgreSQL performance tuning'

**API Key Configuration:**

The CLI client supports three ways to provide LLM API keys (in priority order):

1. **Environment variables** (recommended for development):
   ```bash
   export ANTHROPIC_API_KEY="sk-ant-..."
   export OPENAI_API_KEY="sk-proj-..."
   ```

2. **API key files** (recommended for production):
   ```bash
   echo "sk-ant-..." > ~/.anthropic-api-key
   chmod 600 ~/.anthropic-api-key
   ```

3. **Configuration file values** (not recommended - use env vars or files
   instead)

See **[Using the CLI Client](docs/guide/cli-client.md)** for detailed
documentation.

## Web Client

A web-based management interface is available for monitoring and interacting
with the MCP server:

```bash
# Quick start (starts both MCP server and web interface)
./start_web_client.sh
```

**Features:**
- 🔐 Secure authentication using MCP server credentials
- 📊 Real-time PostgreSQL system information
- 🌓 Light/dark theme support
- 📱 Responsive design for desktop and mobile

**Access:**
- Web Interface: http://localhost:3000
- MCP Server API: http://localhost:8080

See [web/README.md](web/README.md) for detailed documentation.

## Docker Deployment

Deploy the entire stack with Docker Compose for production or development:

```bash
# 1. Copy the example environment file
cp .env.example .env

# 2. Edit .env with your configuration
nano .env  # Add your database connection, API keys, etc.

# 3. Build and start all services
docker-compose up -d
```

**What gets deployed:**
- 🐘 **MCP Server** - Backend service on port 8080
- 🌐 **Web Client** - Browser interface on port 8081
- 🔐 **Authentication** - Token or user-based auth from config
- 💾 **Persistent Storage** - User and token data in Docker volumes

**Quick Access:**
- Web Interface: http://localhost:8081
- MCP API: http://localhost:8080

See **[Deployment Guide](docs/guide/deployment.md)** for complete
documentation including:

- Individual container builds
- Production deployment with reverse proxy
- Security hardening
- Resource limits and monitoring
- Troubleshooting

## Documentation

📚 **[Complete Documentation](docs/index.md)** - Comprehensive guides and
references

### Essential Guides

- **[Configuration Guide](docs/guide/configuration.md)** - Config file,
  environment variables, CLI flags
- **[Deployment Guide](docs/guide/deployment.md)** - HTTP/HTTPS and Docker
  Compose deployment
- **[Using the CLI Client](docs/guide/cli-client.md)** - Production-ready chat
  client with prompt caching
- **[Tools Documentation](docs/reference/tools.md)** - MCP tools reference
- **[Resources Documentation](docs/reference/resources.md)** - MCP resources
  reference
- **[Prompts Documentation](docs/reference/prompts.md)** - MCP prompts reference
- **[Query Examples](docs/reference/examples.md)** - Comprehensive usage
  examples
- **[Authentication Guide](docs/guide/authentication.md)** - API token
  management

### Technical Guides

- **[MCP Protocol Guide](docs/developers/mcp-protocol.md)** - Protocol
  implementation details
- **[Security Guide](docs/guide/security.md)** - Security best practices
- **[Architecture Guide](docs/contributing/architecture.md)** - Code structure
  and extension
- **[LLM Proxy](docs/advanced/llm-proxy.md)** - LLM proxy for web applications
- **[API Reference](docs/developers/api-reference.md)** - Complete API
  documentation
- **[Testing Guide](docs/contributing/testing.md)** - Unit and integration tests
- **[Troubleshooting Guide](docs/guide/troubleshooting.md)** - Common issues and
  solutions

## How It Works

1. **Configure** - Set database connection parameters via environment
   variables, config file, or command-line flags
2. **Start** - Server starts and connects to PostgreSQL, extracting schema
   metadata
3. **Query** - You provide SQL queries via Claude Desktop or API
4. **Execute** - SQL runs in a **read-only transaction**
5. **Return** - Results formatted and returned to the client

**Read-Only Protection:** All queries run in read-only mode - no INSERT,
UPDATE, DELETE, or DDL operations allowed.

**Natural Language Support:** The MCP client (like Claude Desktop with an LLM)
can translate your natural language questions into SQL queries that are then
executed by this server.

## Development

### Prerequisites

- Go 1.21 or higher
- PostgreSQL (for testing)
- golangci-lint v1.x (for linting)

### Setup Linter

The project uses golangci-lint v1.x. Install it with:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

Note: The configuration file [`.golangci.yml`](.golangci.yml) is compatible
with golangci-lint v1.x (not v2).

### Testing

```bash
# Run tests (uses TEST_PGEDGE_POSTGRES_CONNECTION_STRING)
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING=\
  "postgres://localhost/postgres?sslmode=disable"
go test ./...

# Run with coverage
go test -v -cover ./...

# Run linting
make lint
# or directly:
golangci-lint run

# Run locally (configure database connection via environment variables or
# config file)
./bin/pgedge-mcp-server
```

#### Web UI Tests

The web UI has a comprehensive test suite. See
[web/TEST_SUMMARY.md](web/TEST_SUMMARY.md) for details.

```bash
cd web
npm test                # Run all tests
npm run test:watch      # Watch mode
npm run test:coverage   # With coverage
```

## Security

- ✅ Read-only transaction enforcement
- ✅ API token authentication with expiration
- ✅ TLS/HTTPS support
- ✅ SHA256 token hashing
- ✅ File permission enforcement (0600)
- ✅ Input validation and sanitization

See **[Security Guide](docs/guide/security.md)** for comprehensive security
documentation.

## Troubleshooting

**Tools not visible in Claude Desktop?**
- Use absolute paths in config
- Restart Claude Desktop completely
- Check JSON syntax

**Database connection errors?**
- Ensure database connection is configured before starting the server (via
  environment variables, config file, or command-line flags)
- Verify PostgreSQL is running: `pg_isready`
- Check connection parameters are correct (host, port, database, user,
  password)

See **[Troubleshooting Guide](docs/guide/troubleshooting.md)** for detailed
solutions.

## License

This software is released under The PostgreSQL License.

## Support

- **📖 Documentation**: [docs/index.md](docs/index.md)
- **🐛 Issues**:
  [GitHub Issues](https://github.com/pgEdge/pgedge-postgres-mcp/issues)
- **💡 Examples**: [Query Examples](docs/reference/examples.md)

## Related Projects

- [Model Context Protocol](https://modelcontextprotocol.io/) - MCP
  specification
- [Claude Desktop](https://claude.ai/) - Anthropic's Claude AI assistant
- [PostgreSQL](https://www.postgresql.org/) - The world's most advanced open
  source database
