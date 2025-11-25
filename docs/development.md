# Development Guide

This guide covers building, testing, and developing the pgEdge Natural Language Agent and Chat Client.

## Prerequisites

- **Go 1.21 or later** - [Download](https://go.dev/dl/)
- **Make** - Build automation (optional but recommended)
- **golangci-lint** - Code linting (optional)

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Building

### Quick Start

Build both server and client:

```bash
make build
```

This creates:
- `bin/pgedge-nla-server` - MCP server
- `bin/pgedge-nla-cli` - Go chat client

### Building Individual Components

Build only the server:

```bash
make server
# or
go build -o bin/pgedge-nla-server ./cmd/pgedge-pg-mcp-svr
```

Build only the client:

```bash
make client
# or
go build -o bin/pgedge-nla-cli ./cmd/pgedge-pg-mcp-cli
```

### Building for Multiple Platforms

Build for all supported platforms:

```bash
make build-all
```

Build for specific platforms:

```bash
make build-linux    # Linux (amd64)
make build-darwin   # macOS (amd64 and arm64)
make build-windows  # Windows (amd64)
```

### Clean Build Artifacts

```bash
make clean          # Clean both server and client
make clean-server   # Clean server only
make clean-client   # Clean client only
```

## Testing

### Running All Tests

Run both server and client tests:

```bash
make test
```

### Running Specific Tests

Run server tests only:

```bash
make test-server
# or
go test -v ./internal/mcp/... ./internal/auth/... ./internal/config/... ./internal/crypto/... ./internal/database/... ./internal/resources/... ./internal/tools/... ./cmd/pgedge-pg-mcp-svr/...
```

Run client tests only:

```bash
make test-client
# or
go test -v ./internal/chat/... ./cmd/pgedge-pg-mcp-cli/...
```

### Test Coverage

Check test coverage for server:

```bash
go test -cover ./internal/mcp/... ./internal/auth/... ./internal/config/... ./internal/crypto/... ./internal/database/... ./internal/resources/... ./internal/tools/...
```

Check test coverage for client:

```bash
go test -cover ./internal/chat/...
```

Generate detailed coverage report:

```bash
# For server
go test -coverprofile=coverage-server.out ./internal/mcp/... ./internal/auth/... ./internal/config/... ./internal/crypto/... ./internal/database/... ./internal/resources/... ./internal/tools/...
go tool cover -html=coverage-server.out

# For client
go test -coverprofile=coverage-client.out ./internal/chat/...
go tool cover -html=coverage-client.out
```

### Integration Tests

Integration tests require a running PostgreSQL instance:

```bash
# Start PostgreSQL (example using Docker)
docker run -d --name postgres-test \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 \
  postgres:17

# Set connection string
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING="postgres://postgres:postgres@localhost:5432/testdb?sslmode=disable"

# Run integration tests
go test -v ./test/...
```

## Code Quality

### Linting

Lint all code:

```bash
make lint
```

Lint server code only:

```bash
make lint-server
# or
golangci-lint run ./internal/mcp/... ./internal/auth/... ./internal/config/... ./internal/crypto/... ./internal/database/... ./internal/resources/... ./internal/tools/... ./cmd/pgedge-pg-mcp-svr/...
```

Lint client code only:

```bash
make lint-client
# or
golangci-lint run ./internal/chat/... ./cmd/pgedge-pg-mcp-cli/...
```

### Formatting

Format all code:

```bash
make fmt
# or
go fmt ./...
```

Check formatting without modifying files:

```bash
gofmt -l .
```

### Running go vet

Check for common Go issues:

```bash
go vet ./...
```

## Development Workflow

### 1. Clone and Setup

```bash
git clone https://github.com/pgEdge/pgedge-mcp.git
cd pgedge-mcp
go mod download
```

### 2. Make Changes

Edit the code in your preferred editor. The codebase follows standard Go project structure:

- `cmd/` - Application entry points
- `internal/` - Internal packages
    - `mcp/` - MCP protocol implementation
    - `auth/` - Authentication
    - `config/` - Configuration management
    - `database/` - PostgreSQL integration
    - `tools/` - MCP tool implementations
    - `chat/` - Chat client implementation

### 3. Test Your Changes

```bash
# Run relevant tests
make test-server  # if you changed server code
make test-client  # if you changed client code
make test         # run all tests

# Check test coverage
go test -cover ./internal/...
```

### 4. Lint and Format

```bash
# Format code
make fmt

# Run linter
make lint
```

### 5. Build and Verify

```bash
# Build
make build

# Test the binary
./bin/pgedge-nla-server --version
./bin/pgedge-nla-cli --version
```

## Package Management

### Adding Dependencies

```bash
# Add a new dependency
go get github.com/example/package@latest

# Update go.mod and go.sum
go mod tidy
```

### Updating Dependencies

```bash
# Update all dependencies
go get -u ./...
go mod tidy

# Update a specific dependency
go get -u github.com/example/package@latest
go mod tidy
```

### Verifying Dependencies

```bash
# Verify module checksums
go mod verify

# Download dependencies
go mod download
```

## Debugging

### Server Debugging

Enable debug logging:

```bash
./bin/pgedge-nla-server -debug
```

Debug with specific log levels for different components:

```bash
# Database operation logging
export PGEDGE_DB_LOG_LEVEL="info"    # Basic info: connections, metadata loading, errors
export PGEDGE_DB_LOG_LEVEL="debug"   # Detailed: pool config, schema counts, query details
export PGEDGE_DB_LOG_LEVEL="trace"   # Very detailed: full queries, row counts, timings

# LLM/Embedding operation logging
export PGEDGE_LLM_LOG_LEVEL="info"   # Basic info: API calls, errors, token usage
export PGEDGE_LLM_LOG_LEVEL="debug"  # Detailed: text length, dimensions, timing, models
export PGEDGE_LLM_LOG_LEVEL="trace"  # Very detailed: full request/response details

# Run with logging enabled
./bin/pgedge-nla-server
```

### Client Debugging

The chat client outputs to stdout/stderr. Use verbose flags if available:

```bash
./bin/pgedge-nla-cli --help
```

### Using Delve Debugger

Install Delve:

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

Debug the server:

```bash
dlv debug ./cmd/pgedge-pg-mcp-svr -- -debug
```

Debug tests:

```bash
dlv test ./internal/mcp
```

## Profiling

### CPU Profiling

```bash
go test -cpuprofile=cpu.prof -bench=. ./internal/...
go tool pprof cpu.prof
```

### Memory Profiling

```bash
go test -memprofile=mem.prof -bench=. ./internal/...
go tool pprof mem.prof
```

## Contributing

### Before Submitting a Pull Request

1. **Run all tests**: `make test`
2. **Run linter**: `make lint`
3. **Format code**: `make fmt`
4. **Update documentation** if you changed user-facing behavior
5. **Add tests** for new functionality
6. **Update CHANGELOG** (if present) with your changes

### Commit Message Guidelines

Follow conventional commits format:

```
feat: add support for PostgreSQL 17
fix: resolve connection timeout issue
docs: update installation instructions
test: add tests for authentication flow
chore: update dependencies
```

## Test Organization

### Server Tests

- **Unit Tests**: Test individual components in isolation
    - `internal/mcp/*_test.go` - MCP protocol tests
    - `internal/auth/*_test.go` - Authentication tests
    - `internal/config/*_test.go` - Configuration tests
    - `internal/database/*_test.go` - Database integration tests
    - `internal/tools/*_test.go` - Tool implementation tests

- **Integration Tests**: Test complete workflows
    - `test/*_test.go` - End-to-end integration tests

### Client Tests

- **Unit Tests**: [config_test.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/chat/config_test.go) - Configuration loading and validation
- **Integration Tests**: [client_integration_test.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/chat/client_integration_test.go) - Client connection, LLM initialization, command handling, query processing
- **UI Tests**: [ui_test.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/chat/ui_test.go) - Color output, animations, prompts
- **LLM Tests**: [llm_test.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/chat/llm_test.go) - Anthropic and Ollama client functionality
- **MCP Client Tests**: [mcp_client_test.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/chat/mcp_client_test.go) - HTTP and stdio communication

Current test coverage:
- **Server**: Varies by package, see [Testing](testing.md) for details
- **Client**: 48%+ with comprehensive coverage of critical paths

## Common Development Tasks

### Adding a New MCP Tool

1. Create tool implementation in `internal/tools/`
2. Register tool in `internal/tools/registry.go`
3. Add tests in `internal/tools/*_test.go`
4. Update documentation in `docs/tools.md`

### Adding a New MCP Resource

1. Create resource implementation in `internal/resources/`
2. Register resource in the server initialization
3. Add tests
4. Update documentation in `docs/resources.md`

### Modifying Authentication

1. Update `internal/auth/` package
2. Add/modify tests in `internal/auth/*_test.go`
3. Update documentation in `docs/authentication.md`
4. Consider security implications (see `docs/security.md`)

## Troubleshooting Development Issues

### Build Errors

**Problem**: "cannot find package"

**Solution**: Run `go mod download` and `go mod tidy`

**Problem**: "module declares its path as X but was required as Y"

**Solution**: Check your `go.mod` file for correct module path

### Test Failures

**Problem**: Integration tests fail with "connection refused"

**Solution**: Ensure PostgreSQL is running and connection string is correct

**Problem**: "golangci-lint: command not found"

**Solution**: Install golangci-lint: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

### IDE Setup

For VSCode, recommended settings (`.vscode/settings.json`):

```json
{
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace",
  "go.formatTool": "gofmt",
  "editor.formatOnSave": true,
  "go.testFlags": ["-v"]
}
```

## See Also

- [Architecture](architecture.md) - System architecture and design
- [Testing](testing.md) - Detailed testing guide and coverage
- [CI/CD](ci-cd.md) - Continuous integration and deployment
- [MCP Protocol Reference](mcp-protocol.md) - MCP protocol details
