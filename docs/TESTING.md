# Testing Guide

This document describes the test coverage for the pgEdge PostgreSQL MCP Server, with a focus on multi-LLM provider support.

## Test Structure

### Configuration Tests (`internal/config/config_test.go`)

Comprehensive tests for configuration loading and provider selection:

- **Provider Configuration**: Tests for both Anthropic and Ollama providers
- **Validation**: Tests for missing required configuration (API keys, models, connection strings)
- **Configuration Priority**: Tests for CLI flags > Environment variables > Config file > Defaults
- **Invalid Provider**: Tests for unsupported LLM providers
- **Partial Configuration**: Tests for config files with only some values set

**Coverage**: 80.7% of config package

### LLM Client Tests (`internal/llm/client_test.go`)

#### Anthropic Provider Tests
- Basic client creation and configuration
- Successful SQL conversion with mocked API responses
- API error handling (400, 500 status codes)
- Empty response handling
- SQL cleaning (removing markdown, comments, semicolons)
- Complex query handling with JOINs, GROUP BY, ORDER BY

#### Ollama Provider Tests
- Client creation with Ollama provider
- Configuration validation (requires baseURL and model)
- Successful SQL conversion using OpenAI-compatible API format
- API error handling (404, 500 status codes)
- Empty response handling (no choices in response)
- SQL cleaning for Ollama responses
- Complex query handling
- Unsupported provider error handling

**Coverage**: 88.7% of llm package

### Model Management Tests (`cmd/pgedge-postgres-mcp/models_test.go`)

Tests for Ollama model download functionality:

- **Successful Model Pull**: Tests streaming download with progress updates
- **API Errors**: Tests 404 and other API error responses
- **Network Errors**: Tests connection failures
- **Default URL**: Tests that default Ollama URL (http://localhost:11434) is used when not specified
- **Invalid JSON**: Tests handling of malformed API responses
- **Streaming Progress**: Tests progress bar updates during download

All tests use mocked HTTP servers, so they don't require actual Ollama installation or large model downloads.

**Coverage**: 11.8% of cmd/pgedge-postgres-mcp (focused on testable units)

## Running Tests

### Run All Tests
```bash
go test ./...
```

### Run Specific Package Tests
```bash
# Configuration tests
go test ./internal/config -v

# LLM client tests
go test ./internal/llm -v

# Model management tests
go test ./cmd/pgedge-postgres-mcp -v
```

### Run Ollama-Specific Tests
```bash
# All Ollama tests
go test ./internal/llm -v -run ".*Ollama"

# Model pull tests
go test ./cmd/pgedge-postgres-mcp -v -run "TestPullModel"
```

### Run with Coverage
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out  # View in browser
go tool cover -func=coverage.out  # View in terminal
```

## Test Design Philosophy

### Mock-Based Testing

All LLM and model pull tests use `httptest.NewServer()` to create mock HTTP servers. This approach:

- **No External Dependencies**: Tests run without requiring Ollama installation or actual API keys
- **Fast Execution**: No network requests or large downloads
- **Deterministic**: Tests produce consistent results regardless of external service availability
- **CI/CD Friendly**: No need to download multi-gigabyte models in CI pipelines

### Provider Abstraction Testing

Tests verify that:

1. Both providers (Anthropic and Ollama) work through the same interface
2. Configuration correctly routes to the appropriate provider
3. Error handling is consistent across providers
4. SQL cleaning works for both provider response formats

### Edge Cases Covered

- Missing configuration (API keys, models, URLs)
- Network failures and timeouts
- Invalid JSON responses
- Empty or malformed API responses
- SQL with markdown formatting, comments, and extra characters
- Complex multi-line SQL queries

## Integration Testing (Optional)

For testing with actual Ollama models (not run in CI):

```bash
# 1. Install Ollama
# Visit https://ollama.ai/

# 2. Start Ollama
ollama serve

# 3. Pull a small model (optional, ~600MB)
ollama pull tinyllama:1.1b

# 4. Test with a real database connection
export POSTGRES_CONNECTION_STRING="postgres://user:pass@localhost/dbname"
export LLM_PROVIDER="ollama"
export OLLAMA_MODEL="tinyllama:1.1b"
./bin/pgedge-postgres-mcp
```

Note: Integration tests with actual models are not automated in the test suite to avoid large downloads and external dependencies.

## Test Coverage Summary

| Package | Coverage | Focus Areas |
|---------|----------|-------------|
| internal/config | 80.7% | Provider config, validation, priority |
| internal/llm | 88.7% | Both providers, SQL conversion, error handling |
| internal/auth | 57.3% | Token management |
| internal/tools | 48.5% | MCP tools |
| internal/database | 38.4% | DB connections |
| **Overall** | **36.9%** | Core business logic well-covered |

## Adding New Tests

### For New LLM Providers

If adding a new provider (e.g., "openai"), add tests in `internal/llm/client_test.go`:

```go
func TestConvertNLToSQL_NewProvider_Success(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Mock provider's API response
    }))
    defer server.Close()

    client := &Client{
        provider: "newprovider",
        apiKey:   "test-key",
        baseURL:  server.URL,
        model:    "test-model",
    }

    result, err := client.ConvertNLToSQL("test query", "schema")
    // Assertions...
}
```

### For Configuration Changes

Add tests in `internal/config/config_test.go` to verify:
- New configuration fields load correctly
- Validation catches missing required fields
- Priority system works with new fields

## Continuous Integration

All tests run automatically in CI/CD pipelines. The mock-based design ensures:

- No external service dependencies
- Fast execution (< 5 seconds for full suite)
- No large downloads or storage requirements
- Consistent results across environments
