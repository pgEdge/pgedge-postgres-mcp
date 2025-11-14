# Go Chat Client

The pgEdge Postgres MCP Go Chat Client is a **production-ready, full-featured** native Go implementation that provides an interactive command-line interface for chatting with your PostgreSQL database using natural language.

This is the recommended client for production use and provides significantly more features and polish than the [Python examples](chatbot-examples.md), which are intended as simple reference implementations to demonstrate the MCP protocol.

## Features

- **Dual Mode Support**: Connect via stdio (subprocess) or HTTP
- **Multiple LLM Providers**: Support for Anthropic Claude, OpenAI, and Ollama
- **Agentic Tool Execution**: Automatically executes database tools based on LLM decisions
- **PostgreSQL-Themed UI**: Colorful output with elephant-themed animations
- **Flexible Configuration**: Configure via YAML file, environment variables, or command-line flags
- **Built-in Commands**: Help, list tools, list resources, clear screen
- **Conversation History**: Maintains context across multiple queries

## Installation

Build the chat client from source:

```bash
# Using Go directly
go build -o bin/pgedge-pg-mcp-cli ./cmd/pgedge-pg-mcp-cli

# Or using Make
make client

# Build both server and client
make build
```

The binary will be created at `bin/pgedge-pg-mcp-cli`.

## Configuration

The chat client can be configured in three ways (in order of precedence):

1. Command-line flags
2. Environment variables
3. Configuration file

### Configuration File

Create a `.pgedge-pg-mcp-cli.yaml` file in one of these locations:

- Current directory: `./.pgedge-pg-mcp-cli.yaml`
- Home directory: `~/.pgedge-pg-mcp-cli.yaml`
- System-wide: `/etc/pgedge-mcp/chat.yaml`

Example configuration:

```yaml
mcp:
    mode: stdio
    server_path: ./bin/pgedge-pg-mcp-svr
    # For HTTP mode:
    # url: http://localhost:8080
    # token: your-token-here

llm:
    provider: anthropic  # Options: anthropic, openai, or ollama
    model: claude-sonnet-4-20250514
    # anthropic_api_key: your-anthropic-key-here  # Or use PGEDGE_ANTHROPIC_API_KEY env var
    # openai_api_key: your-openai-key-here        # Or use PGEDGE_OPENAI_API_KEY env var
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
- `PGEDGE_MCP_TOKEN`: Authentication token (for HTTP mode)
- `PGEDGE_LLM_PROVIDER`: LLM provider (anthropic, openai, or ollama)
- `PGEDGE_LLM_MODEL`: LLM model name
- `PGEDGE_ANTHROPIC_API_KEY`: Anthropic API key
- `PGEDGE_OPENAI_API_KEY`: OpenAI API key
- `PGEDGE_OLLAMA_URL`: Ollama server URL (default: http://localhost:11434)
- `NO_COLOR`: Disable colored output

### Command-Line Flags

```bash
pgedge-pg-mcp-cli [flags]

Flags:
  -config string            Path to configuration file
  -version                  Show version and exit
  -mcp-mode string          MCP connection mode: stdio or http
  -mcp-url string           MCP server URL (for HTTP mode)
  -mcp-server-path string   Path to MCP server binary (for stdio mode)
  -llm-provider string      LLM provider: anthropic, openai, or ollama
  -llm-model string         LLM model to use
  -anthropic-api-key string API key for Anthropic
  -openai-api-key string    API key for OpenAI
  -ollama-url string        Ollama server URL
  -no-color                 Disable colored output
```

## Usage Examples

### Example 1: Stdio Mode with Anthropic Claude

This is the simplest setup for local development.

```bash
# Set your Anthropic API key
export PGEDGE_ANTHROPIC_API_KEY="your-api-key-here"

# Run the chat client
./bin/pgedge-pg-mcp-cli
```

The client will:

1. Start the MCP server as a subprocess
2. Connect via stdin/stdout
3. Use Anthropic Claude for natural language processing

### Example 2: Stdio Mode with OpenAI

Use OpenAI's GPT models for natural language processing.

```bash
# Set your OpenAI API key
export PGEDGE_OPENAI_API_KEY="your-api-key-here"

# Run the chat client with OpenAI
./bin/pgedge-pg-mcp-cli \
  -llm-provider openai \
  -llm-model gpt-4o
```

Supported OpenAI models:

- `gpt-5` - Latest GPT-5 model (recommended)
- `gpt-4o` - GPT-4 Omni
- `gpt-4-turbo` - GPT-4 Turbo
- `gpt-3.5-turbo` - GPT-3.5 Turbo

Note: GPT-5 and o-series models (o1, o3) have specific API constraints:

- Use `max_completion_tokens` instead of `max_tokens`
- Only support default temperature (1)

The client automatically handles these differences.

### Example 3: HTTP Mode with Authentication

Connect to a remote MCP server with authentication.

```bash
# Set the server URL and token
export PGEDGE_MCP_URL="http://localhost:8080"
export PGEDGE_MCP_TOKEN="your-token-here"

# Run the chat client
./bin/pgedge-pg-mcp-cli -mcp-mode http
```

Or, if you don't set the token, the client will prompt you for it.

### Example 4: Ollama for Local LLM

Use Ollama for privacy-sensitive applications or offline usage.

```bash
# Make sure Ollama is running
ollama serve

# Pull a model if you haven't already
ollama pull llama3

# Run the chat client
./bin/pgedge-pg-mcp-cli \
  -llm-provider ollama \
  -llm-model llama3
```

### Example 5: With Configuration File

Create a configuration file and use it:

```bash
# Create config file
cat > .pgedge-pg-mcp-cli.yaml << EOF
mcp:
  mode: http
  url: http://localhost:8080
  token: my-secure-token

llm:
  provider: anthropic
  model: claude-sonnet-4-20250514
  anthropic_api_key: sk-ant-...
EOF

# Run with config file
./bin/pgedge-pg-mcp-cli -config .pgedge-pg-mcp-cli.yaml
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

- **History file**: `~/.pgedge-pg-mcp-cli-history`
- **History limit**: 1000 entries
- **Navigation**: Use Up/Down arrow keys to navigate through command history
- **Search**: Use Ctrl+R for reverse search through history
- **Line editing**: Full line editing with Emacs-style keybindings

The history persists across sessions, so your previous queries and commands are available when you restart the client.

## Example Conversation

This shows the client's elephant-themed UI in action, including the thinking animation and tool execution messages:

```
          _
   ______/ \-.   _           pgEdge Postgres MCP Chat Client
.-/     (    o\_//           Type 'quit' or 'exit' to leave, 'help' for commands
 |  ___  \_/\---'
 |_||  |_||

System: Connected to MCP server (15 tools available)
System: Using LLM: anthropic (claude-sonnet-4-20250514)
────────────────────────────────────────────────────────────────────────────────
You: What database connections do I have?


⠋ Consulting the herd...
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

## Troubleshooting

### Connection Errors

**Problem**: "Failed to connect to MCP server"

**Solutions**:

- In stdio mode, verify the server path is correct: `-mcp-server-path ./bin/pgedge-pg-mcp-svr`
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

- Set the `PGEDGE_ANTHROPIC_API_KEY` environment variable
- Or add `anthropic_api_key` to your configuration file under `llm:`
- Or use the `-anthropic-api-key` command-line flag

### Terminal/Display Issues

**Problem**: Colors look wrong or garbled

**Solutions**:

- Disable colors with the `NO_COLOR=1` environment variable
- Or use the `-no-color` flag
- Or add `no_color: true` to your configuration file under `ui:`

**Problem**: History not working

**Solutions**:

- Check that `~/.pgedge-pg-mcp-cli-history` is writable
- The history file is created automatically on first use
- On some terminals, readline features may be limited

## See Also

**For Users:**
- [Chat Client Config Example](chat-client-config-example.md) - Complete configuration reference
- [Chatbot Examples Overview](chatbot-examples.md) - Compare different chatbot approaches
- [Stdio + Anthropic Claude Chatbot](stdio-anthropic-chatbot.md) - Python stdio example
- [HTTP + Ollama Chatbot](http-ollama-chatbot.md) - Python HTTP + Ollama example
- [MCP Server Configuration](configuration.md) - Configure the MCP server
- [Available Tools](tools.md) - List of database tools you can use

**For Developers:**
- [Development Guide](development.md) - Building, testing, and development workflow
