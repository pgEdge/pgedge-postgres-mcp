# pgEdge Postgres MCP Server and Natural Language Agent

[![CI - MCP Server](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-server.yml/badge.svg?branch=main)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-server.yml?query=branch%3Amain)
[![CI - CLI Client](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-cli-client.yml/badge.svg?branch=main)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-cli-client.yml?query=branch%3Amain)
[![CI - Web Client](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-web-client.yml/badge.svg?branch=main)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-web-client.yml?query=branch%3Amain)
[![CI - Docker](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-docker.yml/badge.svg?branch=main)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-docker.yml?query=branch%3Amain)
[![CI - Documentation](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-docs.yml/badge.svg?branch=main)](https://github.com/pgEdge/pgedge-postgres-mcp/actions/workflows/ci-docs.yml?query=branch%3Amain)

- About the pgEdge Postgres MCP Server
    - [pgEdge Postgres MCP Server](docs/index.md)
    - [Choosing the Right Solution](docs/guide/mcp-vs-rag.md)
    - [Best Practices - Querying the Server](docs/guide/querying.md)
- Installing the MCP Server
    - [Quick Start](docs/guide/quickstart.md)
    - [Quickstart Demo with Northwind](docs/guide/quickstart_demo.md)
    - [Deploying on Docker](docs/guide/deploy_docker.md)
    - [Deploying from Source](docs/guide/deploy_source.md)
    - [Testing the MCP Server Deployment](docs/guide/test_server.md)
- Configuring the MCP Server
    - [Specifying Configuration Preferences](docs/guide/configuration.md)
    - [Using Environment Variables to Specify Options](docs/guide/env_variable_config.md)
    - [Including Provider Embeddings in a Configuration File](docs/guide/provider_config.md)
    - [Configuring the Agent for Multiple Databases](docs/guide/multiple_db_config.md)
    - [Configuring Supporting Services; HTTP, systemd, and nginx](docs/guide/services_config.md)
    - [Using an Encryption Secret File](docs/guide/encryption_secret.md)
    - [Enabling or Disabling Features](docs/guide/feature_config.md)
- Configuring and Using a Client Application
    - [Connecting with the Web Client](docs/guide/web-client.md)
    - [Using the Go Chat Client](docs/guide/cli-client.md)
    - [Configuring the Server for use with Claude Desktop](docs/guide/claude_desktop.md)
    - [Configuring the Server for use with Cursor](docs/guide/cursor.md)
- [Reviewing Server Logs](docs/guide/server_logs.md)
- Authentication and Security
    - [Authentication - Overview](docs/guide/authentication.md)
    - [Authentication - User Management](docs/guide/auth_user.md)
    - [Authentication - Token Management](docs/guide/auth_token.md)
    - [Security Checklist](docs/guide/security.md)
    - [Security Management](docs/guide/security_mgmt.md)
- Reference
    - [Using MCP Tools](docs/reference/tools.md)
    - [Using MCP Resources](docs/reference/resources.md)
    - [Using MCP Prompts](docs/reference/prompts.md)
    - [Error Reference](docs/reference/error-reference.md)
    - [Server Configuration File](docs/reference/config-examples/server.md)
    - [API Token Configuration File](docs/reference/config-examples/tokens.md)
    - [CLI Client Configuration Details](docs/reference/config-examples/cli-client.md)
    - [KB Builder Configuration Details](docs/reference/config-examples/kb-builder.md)
- Advanced Topics
    - [Creating Custom Definitions](docs/advanced/custom-definitions.md)
    - [Configuring and Using Knowledgebase Search](docs/advanced/knowledgebase.md)
    - [Building a Custom Knowledge Base](docs/advanced/custom-knowledgebase-tutorial.md)
    - [Using the LLM Proxy](docs/advanced/llm-proxy.md)
    - [Row-Level and Column-Level Security](docs/advanced/row-level-security.md)
    - [Distributed Deployment](docs/advanced/distributed-deployment.md)
- For Developers
    - [For Developers - Overview](docs/developers/overview.md)
    - [MCP Protocol](docs/developers/mcp-protocol.md)
    - [API Reference](docs/developers/api-reference.md)
    - [API Browser](docs/api/browser.md)
    - [Client Examples](docs/developers/client-examples.md)
    - Building Chat Clients
        - [Overview](docs/developers/building-chat-clients.md)
        - [Python (Stdio + Claude)](docs/developers/stdio-anthropic-chatbot.md)
        - [Python (HTTP + Ollama)](docs/developers/http-ollama-chatbot.md)
- Contributing
    - [Development Setup](docs/contributing/development.md)
    - [Architecture](docs/contributing/architecture.md)
    - [Internal Architecture](docs/contributing/internal-architecture.md)
    - [KB Builder](docs/contributing/internal/kb-builder-architecture.md)
    - [Testing](docs/contributing/testing.md)
    - [CI/CD](docs/contributing/ci-cd.md)
- [Accessing Online Help](docs/guide/help.md)
- [Troubleshooting](docs/guide/troubleshooting.md)
- [Release Notes](docs/changelog.md)
- [Licence](docs/LICENSE.md)

The pgEdge Postgres Model Context Protocol (MCP) server enables
SQL queries against PostgreSQL databases through MCP-compatible
clients. The Natural Language Agent provides supporting
functionality that allows you to use natural language to form
SQL queries.

> **Supported Versions:** PostgreSQL 14 and higher.

> **NOT FOR PUBLIC-FACING APPLICATIONS**: This MCP server provides
> LLMs with read access to your entire database schema and data.
> It should only be used for internal tools, developer workflows,
> or environments where all users are trusted. For public-facing
> applications, consider the
> [pgEdge RAG Server](https://github.com/pgedge/pgedge-rag-server)
> instead. See the
> [Choosing the Right Solution](docs/guide/mcp-vs-rag.md) guide
> for details.

## Quick Start

The [Quick Start](docs/guide/quickstart.md) guide covers
installation and setup for all supported clients:

| Client | Transport | Best For |
|--------|-----------|----------|
| CLI (Stdio) | Stdio | Local single-user development |
| CLI (HTTP) | HTTP | Multi-user or remote access |
| Web UI | HTTP | Browser-based chat interface |
| Claude Code | Stdio | Anthropic CLI agent |
| Claude Desktop | Stdio | Anthropic desktop app |
| Cursor | Stdio | AI code editor |
| Windsurf | Stdio | Codeium code editor |
| VS Code Copilot | Stdio | GitHub Copilot agent |

For a guided demo with sample data, see the
[Quickstart Demo with Northwind](docs/guide/quickstart_demo.md).

## Key Features

- **Read-Only Protection** - All queries run in read-only
  transactions by default
- **Resources** - Access PostgreSQL statistics and more
- **Tools** - Query execution, schema analysis, advanced hybrid
  search (BM25+MMR), embedding generation, resource reading,
  and more
- **Prompts** - Guided workflows for semantic search setup,
  database exploration, query diagnostics, and more
- **Production Chat Client** - Full-featured Go client with
  Anthropic prompt caching (90% cost reduction)
- **HTTP/HTTPS Mode** - Direct API access with user and token
  authentication
- **Web Interface** - Modern React-based UI with AI-powered chat
  for natural language database interaction
- **Docker Support** - Pre-built images on
  [GitHub Container Registry](https://github.com/orgs/pgEdge/packages?repo_name=pgedge-postgres-mcp)
  with Docker Compose deployment
- **Secure** - TLS support, user and token auth, read-only
  enforcement
- **Hot Reload** - Automatic reload of authentication files
  without server restart

## Development

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 14 or higher (for testing)
- golangci-lint v1.x (for linting)

### Setup Linter

The project uses golangci-lint v1.x. Install it with:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

Note: The configuration file [`.golangci.yml`](.golangci.yml)
is compatible with golangci-lint v1.x (not v2).

### Building

```bash
git clone https://github.com/pgEdge/pgedge-postgres-mcp.git
cd pgedge-postgres-mcp
make build
```

### Testing

```bash
# Run all tests
make test

# Run server tests with a database
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING=\
  "postgres://localhost/postgres?sslmode=disable"
go test ./...

# Run with coverage
go test -v -cover ./...

# Run linting
make lint
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

- Read-only transaction enforcement (configurable per database)
- User and API token authentication with expiration
- TLS/HTTPS support
- SHA256 token hashing
- File permission enforcement (0600)
- Input validation and sanitization

See the [Security Guide](docs/guide/security.md) for
comprehensive security documentation.

## Troubleshooting

**Tools not visible in Claude Desktop?**
- Use absolute paths in config
- Restart Claude Desktop completely
- Check JSON syntax

**Database connection errors?**
- Ensure database connection is configured before starting the
  server (via config file, environment variables, or
  command-line flags)
- Verify PostgreSQL is running: `pg_isready`
- Check connection parameters are correct

See the [Troubleshooting Guide](docs/guide/troubleshooting.md)
for detailed solutions.

## Support

To report an issue with the software, visit:
[GitHub Issues](https://github.com/pgEdge/pgedge-postgres-mcp/issues)

For more information, visit
[docs.pgedge.com](https://docs.pgedge.com)

This project is licensed under the
[PostgreSQL License](LICENSE.md).
