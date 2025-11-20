# CI/CD Guide

This document describes the continuous integration and continuous deployment workflows for the pgEdge MCP Server.

## Overview

The project uses **GitHub Actions** for automated building, testing, and quality assurance.

## Workflows

### Build Workflow

**File**: `.github/workflows/build.yml`

**Triggers**:

- Push to any branch
- Pull requests

**Steps**:

1. Checkout code
2. Set up Go (versions 1.21, 1.22, 1.23)
3. Download dependencies
4. Build binary
5. Verify binary works

### Test Workflow

**File**: `.github/workflows/test.yml`

**Triggers**:

- Push to any branch
- Pull requests

**Matrix Testing**:

- **Go versions**: 1.21, 1.22, 1.23
- **PostgreSQL versions**: 14, 15, 16, 17

**Steps**:

1. Start PostgreSQL service
2. Set up Go
3. Run unit tests
4. Run integration tests
5. Run linting (golangci-lint)
6. Generate coverage report
7. Upload coverage to Codecov (optional)

## Test Matrix

### Go Versions

| Version | Status | Notes |
|---------|--------|-------|
| 1.21 | Supported | Minimum required version |
| 1.22 | Supported | Recommended |
| 1.23 | Supported | Latest |

### PostgreSQL Versions

| Version | Status | Notes |
|---------|--------|-------|
| 14 | Supported | |
| 15 | Supported | |
| 16 | Supported | Recommended |
| 17 | Supported | Latest |
| 13 and earlier | Best effort | Some features may not work |

## Running Tests Locally

### All Tests

```bash
# Run all tests
go test ./...

# With verbose output
go test -v ./...

# With coverage
go test -v -cover ./...

# With race detection
go test -v -race ./...
```

### Specific Test Suites

```bash
# Unit tests only
go test ./internal/...

# Integration tests only
go test ./test/...

# Specific package
go test ./internal/auth/...

# Specific test
go test -v -run TestAuthTokenGeneration ./internal/auth/...
```

### Linting

The project uses **golangci-lint v1.x** (NOT v2). The configuration file [`.golangci.yml`](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/.golangci.yml) is designed for v1.

#### Installation

```bash
# Install latest v1.x version
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Verify installation
golangci-lint version
# Should show: golangci-lint has version v1.x.x
```

The linter will be installed to `$(go env GOPATH)/bin/golangci-lint`.

#### Running Linter

```bash
# Using Makefile (recommended - handles GOPATH/bin automatically)
make lint

# Direct invocation (if in PATH)
golangci-lint run

# Using full path
$(go env GOPATH)/bin/golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```

#### Linter Configuration

The [`.golangci.yml`](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/.golangci.yml) configuration:

- **Enabled linters**: errcheck, govet, ineffassign, staticcheck, unused, misspell, gocritic
- **Disabled checks**: Style checks that are too strict (octalLiteral, httpNoBody, paramTypeCombine, etc.)
- **Test file exclusions**: Test files skip errcheck and gocritic checks for better readability

#### Version Note

⚠️ **Important**: The project uses golangci-lint **v1.x**, not v2. If you have v2 installed, you'll see an error:

```
Error: you are using a configuration file for golangci-lint v2 with golangci-lint v1
```

To fix this, install v1.x using the command above.

## Test Coverage

### Viewing Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View in terminal
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html

# Open in browser
open coverage.html  # macOS
xdg-open coverage.html  # Linux
```

### Coverage Targets

| Component | Target | Current |
|-----------|--------|---------|
| Overall | >80% | Check CI |
| Authentication | >90% | Check CI |
| Database | >85% | Check CI |
| MCP Protocol | >85% | Check CI |
| Tools | >80% | Check CI |
| Resources | >80% | Check CI |

## Integration Testing

### Prerequisites

Integration tests require:

- PostgreSQL running locally or via environment
- Connection string in `TEST_PGEDGE_POSTGRES_CONNECTION_STRING`
- Optional: Anthropic API key in `TEST_ANTHROPIC_API_KEY`

### Running Integration Tests

```bash
# With default PostgreSQL
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING="postgres://localhost/postgres?sslmode=disable"
go test ./test/...

# With custom PostgreSQL
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:pass@host/db"
export TEST_ANTHROPIC_API_KEY="sk-ant-your-key"
go test ./test/...

# Run specific integration test
go test -v -run TestMCPServerIntegration ./test/...
```

### Integration Test Coverage

Tests cover:

- **MCP Protocol**: Initialize, tools/list, resources/list
- **HTTP Mode**: Server startup, endpoints, authentication
- **HTTPS Mode**: TLS certificate handling, secure connections
- **Authentication**: Token validation, expiry, authorization
- **Tools**: All ten tools with various inputs
- **Resources**: All nine resources
- **Error Handling**: Invalid requests, missing data, timeouts
- **Multi-Database**: Connection switching, temporary queries

## CI Environment Variables

### Required

None - tests run with default settings

### Optional

```yaml
env:
  TEST_PGEDGE_POSTGRES_CONNECTION_STRING: postgres://postgres:postgres@localhost/test
  TEST_ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
  CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
```

## GitHub Actions Configuration

### Build Workflow Example

```yaml
name: Build

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  build:
    name: Build
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.21', '1.22', '1.23']

    steps:
    - name: Checkout code
      uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: ${{ matrix.go-version }}

    - name: Build
      run: go build -v -o pgedge-pg-mcp-svr ./cmd/pgedge-pg-mcp-svr

    - name: Verify binary
      run: ./pgedge-pg-mcp-svr -h
```

### Test Workflow Example

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.21', '1.22', '1.23']
        postgres-version: ['14', '15', '16', '17']

    services:
      postgres:
        image: postgres:${{ matrix.postgres-version }}
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432

    steps:
    - name: Checkout
      uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: ${{ matrix.go-version }}

    - name: Run tests
      env:
        TEST_PGEDGE_POSTGRES_CONNECTION_STRING: postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
      run: go test -v -cover ./...

    - name: Run lint
      uses: golangci/golangci-lint-action@v3
      with:
        version: latest
```

## Pre-Commit Hooks

### Setup

```bash
# Install pre-commit hook
cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash
set -e

echo "Running tests..."
go test ./...

echo "Running linter..."
# Try PATH first, then GOPATH/bin
if command -v golangci-lint >/dev/null 2>&1; then
    golangci-lint run
elif [ -f "$(go env GOPATH)/bin/golangci-lint" ]; then
    $(go env GOPATH)/bin/golangci-lint run
else
    echo "Warning: golangci-lint not found, skipping linter"
fi

echo "All checks passed!"
EOF

chmod +x .git/hooks/pre-commit
```

### Skip Hook (When Needed)

```bash
git commit --no-verify
```

## Release Process

### Manual Release

1. **Update version**:
   ```bash
   # Update version in code
   vim cmd/pgedge-pg-mcp-svr/main.go

   # Commit
   git commit -am "Bump version to X.Y.Z"
   ```

2. **Tag release**:
   ```bash
   git tag -a vX.Y.Z -m "Release version X.Y.Z"
   git push origin vX.Y.Z
   ```

3. **Build binaries**:
   ```bash
   # Build for multiple platforms
   GOOS=linux GOARCH=amd64 go build -o bin/pgedge-pg-mcp-svr-linux-amd64 ./cmd/pgedge-pg-mcp-svr
   GOOS=darwin GOARCH=amd64 go build -o bin/pgedge-pg-mcp-svr-darwin-amd64 ./cmd/pgedge-pg-mcp-svr
   GOOS=darwin GOARCH=arm64 go build -o bin/pgedge-pg-mcp-svr-darwin-arm64 ./cmd/pgedge-pg-mcp-svr
   GOOS=windows GOARCH=amd64 go build -o bin/pgedge-pg-mcp-svr-windows-amd64.exe ./cmd/pgedge-pg-mcp-svr
   ```

4. **Create GitHub release**:

    - Go to GitHub Releases
    - Create new release from tag
    - Upload binaries
    - Add release notes

### Automated Release (Future)

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
    - name: Checkout
      uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'

    - name: Build binaries
      run: |
        GOOS=linux GOARCH=amd64 go build -o bin/pgedge-pg-mcp-svr-linux-amd64 ./cmd/pgedge-pg-mcp-svr
        GOOS=darwin GOARCH=amd64 go build -o bin/pgedge-pg-mcp-svr-darwin-amd64 ./cmd/pgedge-pg-mcp-svr
        GOOS=darwin GOARCH=arm64 go build -o bin/pgedge-pg-mcp-svr-darwin-arm64 ./cmd/pgedge-pg-mcp-svr
        GOOS=windows GOARCH=amd64 go build -o bin/pgedge-pg-mcp-svr-windows-amd64.exe ./cmd/pgedge-pg-mcp-svr

    - name: Create Release
      uses: softprops/action-gh-release@v1
      with:
        files: bin/*
      env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Continuous Deployment

### Docker Hub (Future)

```yaml
# .github/workflows/docker.yml
name: Docker

on:
  push:
    branches: [ main ]
    tags: [ 'v*' ]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - name: Checkout
      uses: actions/checkout@v3

    - name: Docker meta
      id: meta
      uses: docker/metadata-action@v4
      with:
        images: pgedge/postgres-mcp

    - name: Login to Docker Hub
      uses: docker/login-action@v2
      with:
        username: ${{ secrets.DOCKERHUB_USERNAME }}
        password: ${{ secrets.DOCKERHUB_TOKEN }}

    - name: Build and push
      uses: docker/build-push-action@v4
      with:
        push: true
        tags: ${{ steps.meta.outputs.tags }}
```

## Monitoring CI/CD

### Build Status

Check build status:

- GitHub Actions tab
- README badges
- Branch protection rules

### Test Failures

When tests fail:

1. **Check CI logs**:

    - Go to Actions tab
    - Click on failed workflow
    - Examine step logs

2. **Reproduce locally**:

   ```bash
   # Use same Go version
   go test -v ./...

   # Use same PostgreSQL version
   docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:16
   export TEST_PGEDGE_POSTGRES_CONNECTION_STRING="postgres://postgres:postgres@localhost/postgres"
   go test -v ./test/...
   ```

3. **Debug**:

   ```bash
   # Run specific test with debug output
   go test -v -run TestFailingTest ./package/...

   # Add debug logging in code
   log.Printf("Debug: %+v", value)
   ```

## Best Practices

### For Contributors

1. **Run tests before committing**:
   ```bash
   go test ./...
   golangci-lint run
   ```

2. **Write tests for new code**:

    - Unit tests for business logic
    - Integration tests for API endpoints
    - Table-driven tests for multiple scenarios

3. **Keep builds fast**:

    - Mock external dependencies
    - Use parallel tests where possible
    - Skip slow tests in pre-commit hooks

4. **Update CI config when**:

    - Adding new dependencies
    - Changing build process
    - Adding new test suites

### For Maintainers

1. **Review CI failures promptly**
2. **Keep dependencies updated**
3. **Monitor test performance**
4. **Update Go versions as released**
5. **Maintain test coverage above targets**

## Troubleshooting CI/CD

### Build Failures

```bash
# Dependency issues
go mod tidy
go mod verify

# Cache issues (clear locally)
go clean -cache
rm -rf ~/go/pkg/mod

# Version mismatch
go mod edit -go=1.21
go mod tidy
```

### Test Failures

```bash
# Timing issues
# Increase timeouts in tests

# Database connection
# Check PostgreSQL service in CI config

# Random failures
# Add retries for flaky tests
# Investigate race conditions
```

### Lint Failures

#### Version Mismatch Error

```
Error: you are using a configuration file for golangci-lint v2 with golangci-lint v1
```

**Solution**: Install golangci-lint v1.x:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

#### Check Linter Locally

```bash
# Using Makefile
make lint

# Direct invocation
golangci-lint run

# Using full path
$(go env GOPATH)/bin/golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```

#### Update Linter Config

If you need to adjust linting rules:

```bash
# Edit config
vim .golangci.yml

# Disable specific checks in gocritic section:
# disabled-checks:
#   - checkName

# Exclude specific paths or linters in issues section:
# exclude-rules:
#   - path: _test\.go
#     linters: [errcheck]
```

## Related Documentation

- [Testing Guide](testing.md) - Detailed testing procedures
- [Architecture Guide](architecture.md) - Code structure and organization
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
