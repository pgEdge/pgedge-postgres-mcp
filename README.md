# pgEdge MCP Server

[![Build Server](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/Build%20Server/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/build-server.yml)
[![Build Client](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/Build%20Client/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/build-client.yml)
[![Test Server](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/Test%20Server/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/test-server.yml)
[![Test Client](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/Test%20Client/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/test-client.yml)
[![Lint Server](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/Lint%20Server/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/lint-server.yml)
[![Lint Client](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/Lint%20Client/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/lint-client.yml)
[![Docs](https://github.com/pgEdge/pgedge-postgres-mcp/workflows/Docs/badge.svg)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/docs.yml)

A Model Context Protocol (MCP) server that enables **SQL queries** against PostgreSQL databases through MCP-compatible clients like Claude Desktop.

```
SELECT * FROM customers WHERE created_at > CURRENT_DATE - INTERVAL '1 month';
SELECT product_id, SUM(revenue) as total FROM sales GROUP BY product_id ORDER BY total DESC LIMIT 10;
SELECT tablename, pg_table_size(tablename::regclass) as size FROM pg_tables WHERE schemaname = 'public' ORDER BY size DESC;
```

> 🚧 **WARNING**: This code is in pre-release status and MUST NOT be put into production without thorough testing!

## Key Features

- 🔒 **Read-Only Protection** - All queries run in read-only transactions
- 📊 **3 Resources** - Access PostgreSQL statistics
- 🛠️ **6 Tools** - Query execution, schema analysis, semantic search (pgvector), embedding generation, resource reading
- 🌐 **HTTP/HTTPS Mode** - Direct API access with token authentication
- 🔐 **Secure** - TLS support, token auth, read-only enforcement

## Quick Start

### 1. Installation

```bash
git clone <repository-url>
cd pgedge-postgres-mcp
make build
```

### 2. Configure for Claude Desktop

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/bin/pgedge-postgres-mcp"
    }
  }
}
```

### 3. Connect to Your Database

Update your Claude Desktop configuration to include database connection parameters:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/bin/pgedge-postgres-mcp",
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

Alternatively, use a `.pgpass` file for password management (recommended for security):

```bash
# ~/.pgpass
localhost:5432:mydb:myuser:mypass
```

Then configure without PGPASSWORD in the config:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/bin/pgedge-postgres-mcp",
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

> **Note:** The server connects to the database at startup using standard PostgreSQL environment variables (PG*) or PGEDGE_DB_* variables. Passwords can be stored securely in `.pgpass` files.

## Example Queries

The MCP client (like Claude Desktop) can translate natural language to SQL, which is then executed by this server.

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
./bin/pgedge-postgres-mcp -http

# HTTPS with TLS
./bin/pgedge-postgres-mcp -http -tls \
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

## Documentation

📚 **[Complete Documentation](docs/index.md)** - Comprehensive guides and references

### Essential Guides
- **[Configuration Guide](docs/configuration.md)** - Config file, environment variables, CLI flags
- **[Tools Documentation](docs/tools.md)** - All 9 MCP tools reference
- **[Resources Documentation](docs/resources.md)** - All 4 MCP resources reference
- **[Query Examples](docs/examples.md)** - Comprehensive usage examples
- **[Deployment Guide](docs/deployment.md)** - HTTP/HTTPS production deployment
- **[Authentication Guide](docs/authentication.md)** - API token management

### Technical Guides
- **[MCP Protocol Guide](docs/mcp_protocol.md)** - Protocol implementation details
- **[Security Guide](docs/security.md)** - Security best practices
- **[Architecture Guide](docs/architecture.md)** - Code structure and extension
- **[Testing Guide](docs/testing.md)** - Unit and integration tests
- **[Troubleshooting Guide](docs/troubleshooting.md)** - Common issues and solutions

## How It Works

1. **Configure** - Call `set_database_connection` tool with your PostgreSQL connection string
2. **Connect** - Server connects to PostgreSQL and extracts schema metadata
3. **Query** - You provide SQL queries via Claude Desktop or API
4. **Execute** - SQL runs in a **read-only transaction**
5. **Return** - Results formatted and returned to the client

**Read-Only Protection:** All queries run in read-only mode - no INSERT, UPDATE, DELETE, or DDL operations allowed.

**Natural Language Support:** The MCP client (like Claude Desktop with an LLM) can translate your natural language questions into SQL queries that are then executed by this server.

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

Note: The configuration file [`.golangci.yml`](.golangci.yml) is compatible with golangci-lint v1.x (not v2).

### Testing

```bash
# Run tests (uses TEST_PGEDGE_POSTGRES_CONNECTION_STRING)
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING="postgres://localhost/postgres?sslmode=disable"
go test ./...

# Run with coverage
go test -v -cover ./...

# Run linting
make lint
# or directly:
golangci-lint run

# Run locally
./bin/pgedge-postgres-mcp
# Then use set_database_connection tool to connect to database
```

## Security

- ✅ Read-only transaction enforcement
- ✅ API token authentication with expiration
- ✅ TLS/HTTPS support
- ✅ SHA256 token hashing
- ✅ File permission enforcement (0600)
- ✅ Input validation and sanitization

See **[Security Guide](docs/security.md)** for comprehensive security documentation.

## Troubleshooting

**Tools not visible in Claude Desktop?**
- Use absolute paths in config
- Restart Claude Desktop completely
- Check JSON syntax

**Database connection errors?**
- Call `set_database_connection` tool first with your connection string
- Verify PostgreSQL is running: `pg_isready`
- Check connection string format: `postgres://user:pass@host:port/dbname`

See **[Troubleshooting Guide](docs/troubleshooting.md)** for detailed solutions.

## License

This software is released under The PostgreSQL License.

## Support

- **📖 Documentation**: [docs/index.md](docs/index.md)
- **🐛 Issues**: [GitHub Issues](https://github.com/pgEdge/pgedge-postgres-mcp/issues)
- **💡 Examples**: [Query Examples](docs/examples.md)

## Related Projects

- [Model Context Protocol](https://modelcontextprotocol.io/) - MCP specification
- [Claude Desktop](https://claude.ai/) - Anthropic's Claude AI assistant
- [PostgreSQL](https://www.postgresql.org/) - The world's most advanced open source database
