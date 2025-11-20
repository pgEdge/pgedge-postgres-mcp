# GitHub Actions CI Workflows

This directory contains CI workflows for the pgEdge Postgres MCP Server project.

## Workflows

### [`ci-server.yml`](ci-server.yml)
**MCP Server Tests**

- Runs on: Push/PR to main/develop
- Matrix: Go 1.21-1.23, PostgreSQL 14-17
- Tests:
  - Code linting and formatting
  - Unit tests with race detection
  - Integration tests with PostgreSQL
  - Coverage reporting

### [`ci-cli-client.yml`](ci-cli-client.yml)
**CLI Client Tests**

- Runs on: Push/PR to main/develop
- Tests the command-line chat client functionality

### [`ci-web-client.yml`](ci-web-client.yml)
**Web Client Tests**

- Runs on: Push/PR to main/develop
- Tests the web-based chat interface

### [`ci-docs.yml`](ci-docs.yml)
**Documentation Tests**

- Runs on: Push/PR to main/develop
- Validates documentation builds with MkDocs

### [`ci-docker-compose.yml`](ci-docker.yml)
**Docker Compose Integration Tests**

- Runs on: Push/PR to main/develop, manual trigger
- Tests:
  - Building all Docker images (server, web, cli)
  - Starting services with docker-compose
  - Health checks for all services
  - MCP server endpoints (health, tools/list, resources/list)
  - Token-based authentication
  - User-based authentication
  - Web UI accessibility
  - Database connectivity
  - Both authenticated and no-auth modes

This workflow provides comprehensive end-to-end testing of the complete Docker
deployment to ensure all services work together correctly.

## Running Workflows Locally

### Prerequisites

- Docker and Docker Compose installed
- PostgreSQL 17 running locally (for server tests)
- Go 1.23+ (for server/CLI tests)
- Node.js 18+ (for web client tests)

### Testing Docker Compose Locally

```bash
# Create .env file
cp .env.example .env
# Edit .env with test values

# Build and start services
docker-compose build
docker-compose up -d

# Run manual tests
curl http://localhost:8080/health
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'

# Stop services
docker-compose down -v
```

## Workflow Status Badges

Add these to your README.md:

```markdown
[![CI - Server](https://github.com/pgEdge/pgedge-mcp/actions/workflows/ci-server.yml/badge.svg)](https://github.com/pgEdge/pgedge-mcp/actions/workflows/ci-server.yml)
[![CI - CLI Client](https://github.com/pgEdge/pgedge-mcp/actions/workflows/ci-cli-client.yml/badge.svg)](https://github.com/pgEdge/pgedge-mcp/actions/workflows/ci-cli-client.yml)
[![CI - Web Client](https://github.com/pgEdge/pgedge-mcp/actions/workflows/ci-web-client.yml/badge.svg)](https://github.com/pgEdge/pgedge-mcp/actions/workflows/ci-web-client.yml)
[![CI - Docker Compose](https://github.com/pgEdge/pgedge-mcp/actions/workflows/ci-docker-compose.yml/badge.svg)](https://github.com/pgEdge/pgedge-mcp/actions/workflows/ci-docker-compose.yml)
```

## Troubleshooting

### Docker Compose Tests Failing

1. **Check logs**: The workflow uploads service logs on failure
2. **Test locally**: Run docker-compose locally with the same .env configuration
3. **Database connection**: Ensure `host.docker.internal` resolves to the host
4. **Health checks**: Verify Dockerfiles have proper health check commands

### Authentication Tests Failing

1. **Token format**: Ensure INIT_TOKENS uses comma-separated values
2. **User format**: Ensure INIT_USERS uses `username:password` format
3. **File permissions**: Token/user files should be created with correct permissions

### Service Startup Timeout

1. **Increase timeout**: Adjust `timeout-minutes` in workflow
2. **Check image size**: Large images may take longer to build/start
3. **Resource limits**: GitHub Actions has CPU/memory limits

## Contributing

When adding new features:

1. **Add tests** to appropriate workflows
2. **Update this README** if adding new workflows
3. **Test locally** before pushing
4. **Check workflow runs** in GitHub Actions tab
