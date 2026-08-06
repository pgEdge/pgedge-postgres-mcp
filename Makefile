.PHONY: build build-server build-client clean clean-server clean-client test test-server test-client run install help lint lint-server lint-client vulncheck fmt format gofmt openapi

# Binary names and directories
SERVER_BINARY=pgedge-postgres-mcp
CLIENT_BINARY=pgedge-nla-cli
BIN_DIR=bin
SERVER_CMD_DIR=cmd/pgedge-pg-mcp-svr
CLIENT_CMD_DIR=cmd/pgedge-pg-mcp-cli

# Build variables
GO=go
GOFLAGS=-v

# Default target - build all binaries
all: build

# Build all binaries (server and client)
build: build-server build-client

# Build the server binary
build-server: server
server:
	@echo "Building $(SERVER_BINARY)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(SERVER_BINARY) ./$(SERVER_CMD_DIR)
	@echo "Server build complete: $(BIN_DIR)/$(SERVER_BINARY)"

# Build the client binary
build-client: client
client:
	@echo "Building $(CLIENT_BINARY)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(CLIENT_BINARY) ./$(CLIENT_CMD_DIR)
	@echo "Client build complete: $(BIN_DIR)/$(CLIENT_BINARY)"

# Regenerate the static OpenAPI specification from the programmatic builder
openapi:
	@echo "Generating OpenAPI specification..."
	$(GO) run ./$(SERVER_CMD_DIR) -openapi > docs/api/openapi.json
	@echo "OpenAPI spec written to docs/api/openapi.json"

# Build for multiple platforms (server only for now)
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building server for Linux..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(SERVER_BINARY)-linux-amd64 ./$(SERVER_CMD_DIR)
	@echo "Linux build complete: $(BIN_DIR)/$(SERVER_BINARY)-linux-amd64"

build-darwin:
	@echo "Building server for macOS..."
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(SERVER_BINARY)-darwin-amd64 ./$(SERVER_CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(SERVER_BINARY)-darwin-arm64 ./$(SERVER_CMD_DIR)
	@echo "macOS builds complete: $(BIN_DIR)/$(SERVER_BINARY)-darwin-{amd64,arm64}"

build-windows:
	@echo "Building server for Windows..."
	@mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(SERVER_BINARY)-windows-amd64.exe ./$(SERVER_CMD_DIR)
	@echo "Windows build complete: $(BIN_DIR)/$(SERVER_BINARY)-windows-amd64.exe"

# Clean all build artifacts
clean: clean-server clean-client
	@echo "All clean complete"

# Clean server artifacts
clean-server:
	@echo "Cleaning server artifacts..."
	rm -f $(BIN_DIR)/$(SERVER_BINARY)
	rm -f $(BIN_DIR)/$(SERVER_BINARY)-linux-*
	rm -f $(BIN_DIR)/$(SERVER_BINARY)-darwin-*
	rm -f $(BIN_DIR)/$(SERVER_BINARY)-windows-*
	@echo "Server clean complete"

# Clean client artifacts
clean-client:
	@echo "Cleaning client artifacts..."
	rm -f $(BIN_DIR)/$(CLIENT_BINARY)
	rm -f $(BIN_DIR)/$(CLIENT_BINARY)-linux-*
	rm -f $(BIN_DIR)/$(CLIENT_BINARY)-darwin-*
	rm -f $(BIN_DIR)/$(CLIENT_BINARY)-windows-*
	@echo "Client clean complete"

# Run all tests
test: test-server test-client

# Run server tests
# Lists every server-side package that has tests. The client-side packages are
# covered by test-client, so they are deliberately absent rather than missing.
test-server:
	@echo "Running server tests..."
	$(GO) test -v ./internal/api/... ./internal/auth/... ./internal/compactor/... ./internal/config/... ./internal/conversations/... ./internal/crypto/... ./internal/database/... ./internal/definitions/... ./internal/httperror/... ./internal/llmtracing/... ./internal/logging/... ./internal/mcp/... ./internal/openapi/... ./internal/prompts/... ./internal/redact/... ./internal/resources/... ./internal/search/... ./internal/sqltext/... ./internal/tools/... ./internal/tracing/... ./internal/tsv/... ./$(SERVER_CMD_DIR)/...

# Run client tests
test-client:
	@echo "Running client tests..."
	$(GO) test -v ./internal/chat/... ./$(CLIENT_CMD_DIR)/...

# Run with example environment
run:
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found. Copy configs/.env.example to .env and configure it."; \
		exit 1; \
	fi
	@echo "Running $(SERVER_BINARY)..."
	@export $$(cat .env | xargs) && $(BIN_DIR)/$(SERVER_BINARY)

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "Dependencies installed"

# Install both binaries to GOPATH/bin
install: build
	@echo "Installing $(SERVER_BINARY) to $$(go env GOPATH)/bin..."
	$(GO) install ./$(SERVER_CMD_DIR)
	@echo "Installing $(CLIENT_BINARY) to $$(go env GOPATH)/bin..."
	$(GO) install ./$(CLIENT_CMD_DIR)
	@echo "Install complete: $(SERVER_BINARY) and $(CLIENT_BINARY)"

# Format code (excludes bin/ directory which contains non-Go files with .go extension)
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./cmd/... ./internal/... ./test/...
	@echo "Format complete"

# Alias for fmt
format: fmt

# Run gofmt directly (shows files that would change, then formats)
# Excludes bin/ directory which contains non-Go files with .go extension
gofmt:
	@echo "Running gofmt..."
	@find . -name '*.go' -not -path './bin/*' -exec gofmt -l -w {} +
	@echo "gofmt complete"

# Run linter on all code (requires golangci-lint)
# Explicitly specifies directories to avoid bin/ which contains non-Go files with .go extension
lint:
	@echo "Running linter on all code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./cmd/... ./internal/... ./test/...; \
	elif [ -f "$$(go env GOPATH)/bin/golangci-lint" ]; then \
		$$(go env GOPATH)/bin/golangci-lint run ./cmd/... ./internal/... ./test/...; \
	else \
		echo "golangci-lint not found. Install it with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		echo "  or visit https://golangci-lint.run/usage/install/"; \
	fi

# Run linter on server code
lint-server:
	@echo "Running linter on server code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./internal/mcp/... ./internal/auth/... ./internal/config/... ./internal/crypto/... ./internal/database/... ./internal/resources/... ./internal/tools/... ./internal/tracing/... ./$(SERVER_CMD_DIR)/...; \
	elif [ -f "$$(go env GOPATH)/bin/golangci-lint" ]; then \
		$$(go env GOPATH)/bin/golangci-lint run ./internal/mcp/... ./internal/auth/... ./internal/config/... ./internal/crypto/... ./internal/database/... ./internal/resources/... ./internal/tools/... ./internal/tracing/... ./$(SERVER_CMD_DIR)/...; \
	else \
		echo "golangci-lint not found. Install it with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		echo "  or visit https://golangci-lint.run/usage/install/"; \
	fi

# Run linter on client code
lint-client:
	@echo "Running linter on client code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./internal/chat/... ./$(CLIENT_CMD_DIR)/...; \
	elif [ -f "$$(go env GOPATH)/bin/golangci-lint" ]; then \
		$$(go env GOPATH)/bin/golangci-lint run ./internal/chat/... ./$(CLIENT_CMD_DIR)/...; \
	else \
		echo "golangci-lint not found. Install it with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		echo "  or visit https://golangci-lint.run/usage/install/"; \
	fi

# Check dependencies for known vulnerabilities (requires govulncheck)
# govulncheck's call-graph analysis prioritizes vulnerabilities this code can
# actually reach over every advisory affecting a dependency, though it still
# reports informational package- and module-level findings when no reachable
# symbol is found.
# Note that findings in the standard library reflect the toolchain in use, so
# an out-of-date local Go installation reports issues that CI, which tracks the
# latest patch release, does not.
# Unlike the lint targets, a missing tool is an error rather than a warning: a
# security check that passes silently because it never ran is worse than none.
# govulncheck's own non-zero exit on findings is propagated, so this target is
# usable as a gate.
vulncheck:
	@echo "Running govulncheck over the module..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	elif [ -n "$$(go env GOBIN)" ] && [ -x "$$(go env GOBIN)/govulncheck" ]; then \
		$$(go env GOBIN)/govulncheck ./...; \
	elif [ -x "$$(go env GOPATH)/bin/govulncheck" ]; then \
		$$(go env GOPATH)/bin/govulncheck ./...; \
	else \
		echo "govulncheck not found. Install it with:"; \
		echo "  go install golang.org/x/vuln/cmd/govulncheck@latest"; \
		echo "  or visit https://go.dev/doc/tutorial/govulncheck"; \
		exit 1; \
	fi

# Show help
help:
	@echo "pgEdge Postgres MCP - Makefile commands:"
	@echo ""
	@echo "Building:"
	@echo "  make                - Build all binaries (server, client) (default)"
	@echo "  make build          - Build all binaries (server, client)"
	@echo "  make server         - Build the MCP server"
	@echo "  make client         - Build the chat client"
	@echo "  make build-server   - Build the MCP server (alias)"
	@echo "  make build-client   - Build the chat client (alias)"
	@echo "  make build-all      - Build for all platforms"
	@echo "  make build-linux    - Build for Linux (amd64)"
	@echo "  make build-darwin   - Build for macOS (amd64 and arm64)"
	@echo "  make build-windows  - Build for Windows (amd64)"
	@echo ""
	@echo "Testing:"
	@echo "  make test           - Run all tests (server + client)"
	@echo "  make test-server    - Run server tests only"
	@echo "  make test-client    - Run client tests only"
	@echo ""
	@echo "Formatting:"
	@echo "  make fmt            - Format Go code with go fmt"
	@echo "  make format         - Format Go code with go fmt (alias)"
	@echo "  make gofmt          - Format Go code with gofmt directly"
	@echo ""
	@echo "Linting:"
	@echo "  make lint           - Run linter on all code"
	@echo "  make lint-server    - Run linter on server code only"
	@echo "  make lint-client    - Run linter on client code only"
	@echo ""
	@echo "Security:"
	@echo "  make vulncheck      - Check dependencies for known vulnerabilities"
	@echo ""
	@echo "Cleaning:"
	@echo "  make clean          - Remove all build artifacts"
	@echo "  make clean-server   - Remove server artifacts only"
	@echo "  make clean-client   - Remove client artifacts only"
	@echo ""
	@echo "OpenAPI:"
	@echo "  make openapi        - Regenerate docs/api/openapi.json from source"
	@echo ""
	@echo "Other:"
	@echo "  make run            - Run server with environment from .env file"
	@echo "  make deps           - Install/update dependencies"
	@echo "  make install        - Install both binaries to GOPATH/bin"
	@echo "  make help           - Show this help message"
	@echo ""
	@echo "Knowledgebase:"
	@echo "  The kb-builder tool that produces kb.db is now in its own"
	@echo "  project: https://github.com/pgEdge/pgedge-ai-kb"
