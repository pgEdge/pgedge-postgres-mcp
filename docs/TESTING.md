# Testing

The pgEdge MCP Server includes comprehensive testing at multiple levels: unit tests, integration tests, linting, and MCP protocol compliance tests.

## Running Tests

### Unit Tests

The project includes a comprehensive unit test suite covering all major components:

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage report
go test -cover ./...

# Generate detailed coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Test Coverage:**
- `internal/database`: Parser functions, connection management
- `internal/llm`: LLM client with HTTP mocking
- `internal/tools`: Tool registry and helper functions
- `internal/resources`: Resource registry
- `test`: Integration tests for the compiled MCP server binary

The tests use mocking where appropriate to avoid requiring external dependencies (database connections, API keys) for unit tests.

## Integrated Linting

The test suite automatically runs `golangci-lint` if it's installed on your system:

```bash
# Install golangci-lint (if not already installed)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run tests (including lint checks)
go test ./...
```

The lint test (`TestLint` in `lint_test.go`) will:
- Run automatically as part of `go test`
- Skip gracefully if golangci-lint is not installed
- Skip gracefully if there are configuration issues
- Report lint errors as test failures

This ensures code quality checks run alongside your tests without requiring separate commands.

**Linter Configuration:**

The project uses `.golangci.yml` for linter configuration. Enabled linters include:
- `errcheck`: Check for unchecked errors
- `govet`: Examines Go source code and reports suspicious constructs
- `ineffassign`: Detects ineffectual assignments
- `staticcheck`: Go static analysis
- `unused`: Checks for unused constants, variables, functions, and types
- `misspell`: Finds commonly misspelled English words
- `gocritic`: Provides diagnostics that check for bugs, performance, and style issues

## Integration Tests

The project includes integration tests that test the compiled MCP server binary end-to-end by communicating via the MCP protocol (JSON-RPC over stdio):

```bash
# Run integration tests
cd test && go test -v

# Set custom database connection for integration tests (optional)
TEST_POSTGRES_CONNECTION_STRING="postgres://localhost/testdb?sslmode=disable" \
  go test -v ./test

# Run with custom API key (optional)
TEST_ANTHROPIC_API_KEY="your-key" \
  go test -v ./test
```

**What the Integration Tests Cover:**
- MCP protocol initialize handshake
- tools/list - Listing all available tools
- resources/list - Listing all available resources
- resources/read - Reading the pg://settings and pg://system_info resources
- tools/call - Calling the get_schema_info tool
- query_database - Natural language query "What is the PostgreSQL version?" (requires TEST_ANTHROPIC_API_KEY)
- JSON-RPC request/response format validation
- Server startup and graceful shutdown

The integration tests automatically build the binary if it doesn't exist and handle server lifecycle management. Tests include retry logic to account for asynchronous metadata loading.

**Note:** The `QueryPostgreSQLVersion` test requires a valid Anthropic API key set in the `TEST_ANTHROPIC_API_KEY` environment variable, as it tests the full end-to-end flow including LLM natural language to SQL conversion. If the API key is not provided, this test will be skipped. The test works with any PostgreSQL version (9.x, 10+, development versions, beta versions, etc.) without hardcoding version numbers.

## MCP Compliance Tests

The project includes dedicated compliance tests (`test/mcp_compliance_test.go`) that verify the server properly implements the MCP specification:

**What the Compliance Tests Cover:**
- Capability advertisement in initialize response
- Tool definitions have required fields (name, description, inputSchema)
- Resource definitions have required fields (uri, name, description, mimeType)
- All registered tools and resources are properly advertised
- Correct number of tools and resources are exposed

These tests ensure that MCP clients like Claude Desktop can properly discover and use all available functionality.

## Testing with MCP Inspector

You can test the server interactively using the MCP Inspector tool:

```bash
npx @modelcontextprotocol/inspector /path/to/bin/pgedge-postgres-mcp
```

The inspector provides a web interface for:
- Viewing available tools and resources
- Calling tools with custom parameters
- Reading resources
- Inspecting JSON-RPC messages

## CI/CD Testing

The project uses GitHub Actions for continuous integration with multiple test workflows.

### Build Workflow (`.github/workflows/build.yml`)

- **Triggers**: Push and pull requests to main/develop branches
- **Go Versions**: Tests against Go 1.21, 1.22, and 1.23
- **Steps**:
  - Checkout code
  - Set up Go with caching
  - Verify dependencies
  - Build the binary
  - Upload build artifacts (from Go 1.23)

### Test Workflow (`.github/workflows/test.yml`)

Includes multiple jobs:

**Unit Tests**
- Runs on Go 1.21, 1.22, and 1.23
- Executes all unit tests with race detection
- Generates code coverage reports (HTML and text)
- Uploads coverage artifacts for download
- Includes golangci-lint checks

**Integration Tests**
- Runs on Go 1.23
- Tests against PostgreSQL versions 14, 15, 16, and 17
- Uses Docker containers for PostgreSQL services
- Runs with and without Anthropic API key (if configured)

### Secrets Configuration

To enable all CI/CD features, configure these GitHub repository secrets:

- `ANTHROPIC_API_KEY`: (Optional) For running integration tests with actual LLM queries

## Test Best Practices

### Writing Unit Tests

1. **Mock external dependencies**: Use interfaces and mocks for database connections and HTTP clients
2. **Test error cases**: Verify error handling and edge cases
3. **Use table-driven tests**: For testing multiple scenarios with similar logic
4. **Avoid test pollution**: Ensure tests don't depend on each other

Example:
```go
func TestParser(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"valid query", "SELECT 1", "SELECT 1", false},
        {"empty query", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Parse(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if result != tt.expected {
                t.Errorf("Parse() = %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### Writing Integration Tests

1. **Test the full stack**: Build and run the actual binary
2. **Use realistic scenarios**: Test real-world usage patterns
3. **Handle asynchronous operations**: Use retries for operations that may take time
4. **Clean up resources**: Ensure processes are killed and connections closed

### Continuous Integration

- All tests run automatically on push and pull requests
- Tests run against multiple Go versions (1.21, 1.22, 1.23)
- Integration tests run against multiple PostgreSQL versions (14, 15, 16, 17)
- Coverage reports are generated and available as artifacts
- Lint checks ensure code quality standards

## Troubleshooting Tests

**Tests fail with "database not ready":**
- Increase retry timeout in integration tests
- Check PostgreSQL connection string
- Verify PostgreSQL is running

**Lint tests fail:**
- Run `golangci-lint run` locally to see all issues
- Check `.golangci.yml` configuration
- Ensure Go version compatibility

**Integration tests timeout:**
- Check if binary builds successfully
- Verify PostgreSQL connection
- Look for error messages in test output
- Check that ports are not already in use

**Race detector failures:**
- Run with `-race` flag: `go test -race ./...`
- Fix any data races reported
- Ensure proper synchronization in concurrent code
