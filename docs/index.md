# pgEdge MCP Server Documentation

A Model Context Protocol (MCP) server written in Go that enables natural language queries against PostgreSQL databases.

> 🚧 **WARNING**: This code is in pre-release status and MUST NOT be put into production without thorough testing!

## Quick Links

- **[Configuration Guide](configuration.md)** - Setup and configuration
- **[Docker Deployment](docker-deployment.md)** - Complete Docker Compose deployment
- **[Tools Reference](tools.md)** - All 5 MCP tools
- **[Resources Reference](resources.md)** - All 3 MCP resources
- **[Query Examples](examples.md)** - Usage examples
- **[Deployment Guide](deployment.md)** - HTTP/HTTPS deployment
- **[Troubleshooting](troubleshooting.md)** - Common issues and solutions

## Features

- ✨ **Natural Language to SQL** - Convert plain English questions into SQL queries
- 🔒 **Read-Only Protection** - All queries execute in read-only transactions
- 🤖 **Multiple LLM Support** - Anthropic Claude, OpenAI (GPT-4o, GPT-5), or Ollama (local/free)
- 📊 **3 Resources** - PostgreSQL statistics (pg_stat_*, pg://system_info)
- 🛠️ **5 Tools** - Query execution, schema analysis, advanced hybrid search, embedding generation, resource reading
- 🌐 **HTTP/HTTPS Mode** - Direct API access with token authentication
- 🖥️ **Web Interface** - Modern React-based UI for server monitoring and management
- 💬 **Production Chat Client** - Full-featured Go client with Anthropic prompt caching
- 🔐 **Secure** - TLS support, token authentication, read-only enforcement

## Getting Started

### Prerequisites

- Go 1.21 or higher
- PostgreSQL database (any version with pg_description support)
- LLM Provider: Anthropic Claude API key, OpenAI API key, OR Ollama installation

### Quick Setup

1. **Build the server:**

    ```bash
    make build
    ```

2. **Choose your LLM provider:**

    - **Anthropic Claude**: Get API key at https://console.anthropic.com/
    - **OpenAI**: Get API key at https://platform.openai.com/
    - **Ollama**: Install from https://ollama.ai/ and download a model

3. **Configure for Claude Desktop:**

    Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):
    ```json
    {
        "mcpServers": {
        "pgedge": {
            "command": "/absolute/path/to/bin/pgedge-pg-mcp-svr",
            "env": {
            "PGHOST": "localhost",
            "PGPORT": "5432",
            "PGDATABASE": "mydb",
            "PGUSER": "myuser",
            "PGPASSWORD": "mypass",
            "ANTHROPIC_API_KEY": "sk-ant-your-key"
            }
        }
        }
    }
    ```

4. **Start using:** Restart Claude Desktop and ask questions about your database!

For detailed setup instructions, see **[Configuration Guide](configuration.md)**.

## Documentation

### Essential Guides

#### [Configuration Guide](configuration.md)
Complete configuration reference covering config files, environment variables, command-line flags, and Claude Desktop setup for Anthropic, OpenAI, and Ollama providers.

#### [Docker Deployment Guide](docker-deployment.md)
Complete guide for deploying with Docker and Docker Compose. Includes containerized MCP server and web client setup, environment configuration, production deployment with reverse proxy, security hardening, and troubleshooting.

#### [Tools Documentation](tools.md)
Reference for all 5 MCP tools including `query_database`, `get_schema_info`, `similarity_search`, `generate_embedding`, and `read_resource`.

#### [Resources Documentation](resources.md)
Reference for MCP resources providing access to PostgreSQL system information and database schema overview.

#### [Query Examples](examples.md)
Comprehensive collection of example queries covering schema discovery, data analysis, system monitoring, and multi-database operations.

#### [Deployment Guide](deployment.md)
Production deployment guide for HTTP/HTTPS mode including TLS setup, reverse proxy configuration, Docker deployment, and systemd services.

#### [Authentication Guide](authentication.md)
API token management for HTTP/HTTPS mode including token generation, validation, expiration, and security best practices.

#### [Web Interface](../web/README.md)
Modern React-based web interface for server monitoring and management. Features secure authentication, real-time PostgreSQL system information, and responsive design. Includes quick start guide and deployment instructions.

#### [Go Chat Client](using-cli-client.md)
Production-ready command-line chat client with Anthropic prompt caching (90% cost reduction), support for both stdio and HTTP modes, and comprehensive session management.

### Technical Guides

#### [MCP Protocol Guide](mcp-protocol.md)
Protocol implementation details covering JSON-RPC 2.0 format, transport layers (stdio, HTTP), tool invocation, and resource access.

#### [Security Guide](security.md)
Comprehensive security documentation including threat model, security features, best practices, and compliance considerations.

#### [Architecture Guide](architecture.md)
Internal architecture documentation covering code organization, package structure, and guides for extending the server with new tools and resources.

#### [Testing Guide](testing.md)
Testing documentation covering unit tests, integration tests, PostgreSQL version compatibility testing, and CI/CD integration.

#### [CI/CD Guide](ci-cd.md)
Continuous integration documentation covering GitHub Actions workflows, automated testing, release process, and version management.

#### [Troubleshooting Guide](troubleshooting.md)
Problem-solving guide with common issues, diagnostic procedures, error messages, and debugging tips.

## How It Works

The server operates in four main steps:

1. **Metadata Extraction** - Connects to PostgreSQL and extracts schema information (tables, columns, types, comments)
2. **Natural Language Processing** - Sends questions and schema to LLM for SQL generation
3. **Read-Only Execution** - Executes generated SQL in read-only transactions
4. **Result Formatting** - Returns formatted results to Claude Desktop

All queries via `query_database` are executed in read-only mode, preventing INSERT, UPDATE, DELETE, and DDL operations.

## HTTP/HTTPS Mode

Run as a standalone HTTP server:

```bash
# HTTP
./bin/pgedge-pg-mcp-svr -http

# HTTPS
./bin/pgedge-pg-mcp-svr -http -tls -cert server.crt -key server.key
```

See **[Deployment Guide](deployment.md)** and **[Authentication Guide](authentication.md)** for details.

## Development

### Project Structure

```
pgedge-postgres-mcp/
├── cmd/pgedge-pg-mcp-svr/  # Application entry point
├── internal/                  # Private packages
│   ├── auth/                  # API token authentication
│   ├── config/                # Configuration management
│   ├── database/              # PostgreSQL integration
│   ├── llm/                   # LLM provider clients
│   ├── mcp/                   # MCP protocol implementation
│   ├── resources/             # MCP resource implementations
│   └── tools/                 # MCP tool implementations
├── docs/                      # Documentation
└── test/                      # Integration tests
```

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -v -cover ./...

# Integration tests (requires PostgreSQL)
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:pass@localhost/testdb"
go test ./internal/resources -v -run Integration
```

See **[Testing Guide](testing.md)** for comprehensive testing documentation.

## Security

Key security features:

- ✅ Read-only transaction enforcement
- ✅ API token authentication with expiration
- ✅ TLS/HTTPS support
- ✅ SHA256 token hashing
- ✅ Input validation and sanitization

See **[Security Guide](security.md)** for detailed security documentation.

## Support

- **Documentation**: Browse guides in [docs](index.md) directory
- **Issues**: [GitHub Issues](https://github.com/pgEdge/pgedge-postgres-mcp/issues)
- **Examples**: See [Query Examples](examples.md)

## License

This software is released under The PostgreSQL License.

## Related Projects

- [Model Context Protocol](https://modelcontextprotocol.io/) - MCP specification
- [Claude Desktop](https://claude.ai/) - Anthropic's Claude AI assistant
- [Ollama](https://ollama.ai/) - Run LLMs locally
- [PostgreSQL](https://www.postgresql.org/) - Open source database
- [pgEdge](https://www.pgedge.com/) - Distributed PostgreSQL
