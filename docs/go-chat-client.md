# Go Chat Client

The pgEdge Postgres MCP Go Chat Client is a native Go implementation that provides an interactive command-line interface for chatting with your PostgreSQL database using natural language.

## Features

- **Dual Mode Support**: Connect via stdio (subprocess) or HTTP
- **Multiple LLM Providers**: Support for Anthropic Claude and Ollama
- **Agentic Tool Execution**: Automatically executes database tools based on LLM decisions
- **PostgreSQL-Themed UI**: Colorful output with elephant-themed animations
- **Flexible Configuration**: Configure via YAML file, environment variables, or command-line flags
- **Built-in Commands**: Help, list tools, list resources, clear screen
- **Conversation History**: Maintains context across multiple queries

## Installation

Build the chat client from source:

```bash
# Using Go directly
go build -o bin/pgedge-postgres-mcp-chat ./cmd/pgedge-postgres-mcp-chat

# Or using Make
make client

# Build both server and client
make build
```

The binary will be created at `bin/pgedge-postgres-mcp-chat`.

## Configuration

The chat client can be configured in three ways (in order of precedence):

1. Command-line flags
2. Environment variables
3. Configuration file

### Configuration File

Create a `.pgedge-mcp-chat.yaml` file in one of these locations:

- Current directory: `./.pgedge-mcp-chat.yaml`
- Home directory: `~/.pgedge-mcp-chat.yaml`
- System-wide: `/etc/pgedge-mcp/chat.yaml`

Example configuration:

```yaml
mcp:
    mode: stdio
    server_path: ./bin/pgedge-postgres-mcp
    # For HTTP mode:
    # url: http://localhost:8080
    # token: your-token-here

llm:
    provider: anthropic
    model: claude-sonnet-4-20250514
    # api_key: your-api-key-here  # Or use ANTHROPIC_API_KEY env var
    max_tokens: 4096
    temperature: 0.7

ui:
    no_color: false
```

For a complete configuration file example with all available options and detailed comments, see the [Chat Client Config Example](chat-client-config-example.md).

### Environment Variables

- `PGEDGE_MCP_MODE`: Connection mode (stdio or http)
- `PGEDGE_MCP_URL`: MCP server URL (for HTTP mode)
- `PGEDGE_MCP_SERVER_PATH`: Path to MCP server binary (for stdio mode)
- `PGEDGE_POSTGRES_MCP_SERVER_TOKEN`: Authentication token (for HTTP mode)
- `PGEDGE_LLM_PROVIDER`: LLM provider (anthropic or ollama)
- `PGEDGE_LLM_MODEL`: LLM model name
- `ANTHROPIC_API_KEY`: Anthropic API key
- `OLLAMA_BASE_URL`: Ollama server URL (default: http://localhost:11434)
- `NO_COLOR`: Disable colored output

### Command-Line Flags

```bash
pgedge-postgres-mcp-chat [flags]

Flags:
  -config string            Path to configuration file
  -version                  Show version and exit
  -mcp-mode string          MCP connection mode: stdio or http
  -mcp-url string           MCP server URL (for HTTP mode)
  -mcp-server-path string   Path to MCP server binary (for stdio mode)
  -llm-provider string      LLM provider: anthropic or ollama
  -llm-model string         LLM model to use
  -api-key string           API key for LLM provider
  -ollama-url string        Ollama server URL
  -no-color                 Disable colored output
```

## Usage Examples

### Example 1: Stdio Mode with Anthropic Claude

This is the simplest setup for local development.

```bash
# Set your Anthropic API key
export ANTHROPIC_API_KEY="your-api-key-here"

# Run the chat client
./bin/pgedge-postgres-mcp-chat
```

The client will:

1. Start the MCP server as a subprocess
2. Connect via stdin/stdout
3. Use Anthropic Claude for natural language processing

### Example 2: HTTP Mode with Authentication

Connect to a remote MCP server with authentication.

```bash
# Set the server URL and token
export PGEDGE_MCP_URL="http://localhost:8080"
export PGEDGE_POSTGRES_MCP_SERVER_TOKEN="your-token-here"

# Run the chat client
./bin/pgedge-postgres-mcp-chat -mcp-mode http
```

Or, if you don't set the token, the client will prompt you for it.

### Example 3: Ollama for Local LLM

Use Ollama for privacy-sensitive applications or offline usage.

```bash
# Make sure Ollama is running
ollama serve

# Pull a model if you haven't already
ollama pull llama3

# Run the chat client
./bin/pgedge-postgres-mcp-chat \
  -llm-provider ollama \
  -llm-model llama3
```

### Example 4: With Configuration File

Create a configuration file and use it:

```bash
# Create config file
cat > .pgedge-mcp-chat.yaml << EOF
mcp:
  mode: http
  url: http://localhost:8080
  token: my-secure-token

llm:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: sk-ant-...
EOF

# Run with config file
./bin/pgedge-postgres-mcp-chat -config .pgedge-mcp-chat.yaml
```

## Interactive Commands

Once the chat client is running, you can use these special commands:

- `help` - Show available commands
- `quit` or `exit` - Exit the chat client (also Ctrl+C or Ctrl+D)
- `clear` - Clear the screen
- `tools` - List available MCP tools
- `resources` - List available MCP resources

### Command History

The chat client includes full readline support with persistent command history:

- **History file**: `~/.pgedge-postgres-mcp-chat-history`
- **History limit**: 1000 entries
- **Navigation**: Use Up/Down arrow keys to navigate through command history
- **Search**: Use Ctrl+R for reverse search through history
- **Line editing**: Full line editing with Emacs-style keybindings

The history persists across sessions, so your previous queries and commands are available when you restart the client.

## Example Conversation

```
    ___
   /   \
  | @ @ |  pgEdge Postgres MCP Chat Client
  |  >  |
   \___/   Type 'quit' or 'exit' to leave, 'help' for commands
    | |

System: Connected to MCP server (15 tools available)
System: Using LLM: anthropic (claude-sonnet-4-20250514)
────────────────────────────────────────────────────────────────────────────────
You: What database connections do I have?

  → Executing tool: list_database_connections

Assistant: You have 3 saved database connections: production, staging, and development.

You: Connect to production and show me the users table

  → Executing tool: set_database_connection
  → Executing tool: query_database

Assistant: Here are the users from your production database:

| id | username | email                  | created_at          |
|----|----------|------------------------|---------------------|
| 1  | alice    | alice@example.com      | 2024-01-15 10:30:00 |
| 2  | bob      | bob@example.com        | 2024-01-20 14:45:00 |
| 3  | charlie  | charlie@example.com    | 2024-02-01 09:15:00 |

You: quit

System: Goodbye!
```

## Development and Testing

### Running Tests

The chat client has comprehensive test coverage including unit tests, integration tests, and UI tests.

Run all chat client tests:

```bash
# Using Go directly
go test -v ./internal/chat/...

# Or using Make
make test-client

# Check test coverage
go test -cover ./internal/chat/...

# Generate detailed coverage report
go test -coverprofile=coverage.out ./internal/chat/...
go tool cover -html=coverage.out
```

### Test Organization

The test suite includes:

- **Unit tests** ([config_test.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/chat/config_test.go)): Configuration loading and validation
- **Integration tests** ([client_integration_test.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/chat/client_integration_test.go)):
  - Client connection (HTTP and stdio modes)
  - LLM initialization (Anthropic and Ollama)
  - Command handling (help, clear, tools, resources)
  - Query processing with tool execution
  - Error handling and edge cases
  - Context cancellation and graceful shutdown
- **UI tests** ([ui_test.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/chat/ui_test.go)): Color output, animations, prompts
- **LLM tests** ([llm_test.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/chat/llm_test.go)): Anthropic and Ollama client functionality
- **MCP client tests** ([mcp_client_test.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/chat/mcp_client_test.go)): HTTP and stdio communication

Current test coverage is over 48% and includes all critical paths.

### Building for Multiple Platforms

Build the client for different platforms:

```bash
# Build for all platforms
make build-all

# Build for specific platforms
make build-linux    # Linux (amd64)
make build-darwin   # macOS (amd64 and arm64)
make build-windows  # Windows (amd64)
```

### Code Quality

Run the linter on the chat client code:

```bash
# Using Go tools
golangci-lint run ./internal/chat/... ./cmd/pgedge-postgres-mcp-chat/...

# Or using Make
make lint-client
```

Format the code:

```bash
go fmt ./internal/chat/... ./cmd/pgedge-postgres-mcp-chat/...

# Or using Make
make fmt
```

## Troubleshooting

### Connection Errors

**Problem**: "Failed to connect to MCP server"

**Solutions**:

- In stdio mode, verify the server path is correct: `-mcp-server-path ./bin/pgedge-postgres-mcp`
- In HTTP mode, verify the URL is correct: `-mcp-url http://localhost:8080`
- Check if the MCP server is running (in HTTP mode)
- Verify authentication token is set (in HTTP mode with auth enabled)

### LLM Errors

**Problem**: "LLM error: authentication failed"

**Solutions**:

- For Anthropic: Verify `ANTHROPIC_API_KEY` is set correctly
- For Ollama: Verify Ollama is running (`ollama serve`) and the model is pulled (`ollama pull llama3`)
- Check the model name is correct

**Problem**: "Ollama: model not found"

**Solutions**:

```bash
# List available models
ollama list

# Pull the model you want to use
ollama pull llama3
```

### Configuration Issues

**Problem**: "Configuration error: invalid mode"

**Solutions**:

- Valid modes are `stdio` or `http`
- Check your configuration file or command-line flags
- Mode must be specified if not using default

**Problem**: "Missing API key for Anthropic"

**Solutions**:

- Set the `ANTHROPIC_API_KEY` environment variable
- Or add `api_key` to your configuration file under `llm:`
- Or use the `-api-key` command-line flag

### Terminal/Display Issues

**Problem**: Colors look wrong or garbled

**Solutions**:

- Disable colors with the `NO_COLOR=1` environment variable
- Or use the `-no-color` flag
- Or add `no_color: true` to your configuration file under `ui:`

**Problem**: History not working

**Solutions**:

- Check that `~/.pgedge-postgres-mcp-chat-history` is writable
- The history file is created automatically on first use
- On some terminals, readline features may be limited

## See Also

- [Chat Client Config Example](chat-client-config-example.md) - Complete configuration reference
- [Chatbot Examples Overview](chatbot-examples.md) - Compare different chatbot approaches
- [Stdio + Anthropic Claude Chatbot](stdio-anthropic-chatbot.md) - Python stdio example
- [HTTP + Ollama Chatbot](http-ollama-chatbot.md) - Python HTTP + Ollama example
- [MCP Server Configuration](configuration.md) - Configure the MCP server
- [Available Tools](tools.md) - List of database tools you can use
