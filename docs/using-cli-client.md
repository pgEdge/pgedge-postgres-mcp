# Go Chat Client

The pgEdge Postgres MCP Go Chat Client is a **production-ready, full-featured** native Go implementation that provides an interactive command-line interface for chatting with your PostgreSQL database using natural language.

This is the recommended client for production use and provides significantly more features and polish than the [Python examples](building-chat-clients.md), which are intended as simple reference implementations to demonstrate the MCP protocol.

## Features

- **Dual Mode Support**: Connect via stdio (subprocess) or HTTP
- **Multiple LLM Providers**: Support for Anthropic Claude, OpenAI, and Ollama
- **Runtime Configuration**: Switch LLM providers and models without restarting (via slash commands)
- **Prompt Caching**: Automatic Anthropic prompt caching to reduce costs and latency (up to 90% savings on cached tokens)
- **Agentic Tool Execution**: Automatically executes database tools based on LLM decisions
- **PostgreSQL-Themed UI**: Colorful output with elephant-themed animations
- **Flexible Configuration**: Configure via YAML file, environment variables, or command-line flags
- **Built-in Commands**: Help, list tools, list resources, clear screen
- **Slash Commands**: Claude Code-style commands for settings and configuration
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
- System-wide: `/etc/pgedge/postgres-mcp/pgedge-pg-mcp-cli.yaml`

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
    display_status_messages: true  # Show/hide status messages during tool execution
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

## Prompt Caching (Anthropic Only)

When using Anthropic Claude as your LLM provider, the chat client automatically uses **prompt caching** to significantly reduce costs and improve response times.

### How It Works

Anthropic's prompt caching feature allows frequently used content (like tool definitions) to be cached on Anthropic's servers for 5 minutes. The chat client automatically implements this optimization:

1. **Tool Definitions Cached**: All MCP tool definitions are cached after the first request
2. **Automatic Detection**: No configuration needed - works automatically when using Anthropic
3. **Cost Savings**: Cached input tokens cost ~90% less than regular input tokens
4. **Lower Latency**: Cached content doesn't need to be reprocessed, reducing response time

### Cache Usage Logging

When caching is active, you'll see log messages showing cache performance:

```
[LLM] [INFO] Prompt Cache - Created: 1247 tokens, Read: 0 tokens (saved ~0% on input)
[LLM] [INFO] Prompt Cache - Created: 0 tokens, Read: 1247 tokens (saved ~89% on input)
```

- **Created**: First time content is sent - creates a cache entry
- **Read**: Subsequent requests - reads from cache instead of reprocessing
- **Saved %**: Percentage of input tokens that were cached (cost reduction)

### Cost Savings Example

Without caching:

- Request 1: 1500 input tokens × $3.00/1M = $0.0045
- Request 2: 1500 input tokens × $3.00/1M = $0.0045
- Total: $0.0090

With caching:

- Request 1: 1500 input tokens × $3.00/1M = $0.0045 (cache created)
- Request 2: 200 new + 1300 cached × $0.30/1M = $0.0010
- Total: $0.0055 (39% savings)

### Requirements

- Only available with Anthropic Claude models
- Cache entries expire after 5 minutes of inactivity
- Tool definitions must remain constant (automatic in our implementation)

### Compatibility

- **Anthropic Claude**: Full support ✅
- **OpenAI**: Not available (OpenAI doesn't support prompt caching)
- **Ollama**: Not available (local models don't have caching)

For more details, see [Anthropic's Prompt Caching documentation](https://docs.anthropic.com/claude/docs/prompt-caching).

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

- `gpt-5-main` - Latest GPT-5 fast model (recommended)
- `gpt-5-thinking` - GPT-5 reasoning model
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

## Slash Commands

The chat client supports **slash commands** for managing settings and configuration without restarting. Similar to Claude Code, commands starting with `/` are processed locally, while unknown commands are sent to the LLM for interpretation.

### Available Slash Commands

#### Display Help

```
/help
```

Shows comprehensive help for all slash commands with examples.

#### Manage Status Messages

```
/set status-messages <on|off>
/show status-messages
```

Control whether status messages are displayed during tool execution. Useful for cleaner output or debugging.

**Examples:**

```
You: /set status-messages off
System: Status messages disabled

You: /set status-messages on
System: Status messages enabled
```

#### Switch LLM Provider

```
/set llm-provider <provider>
/show llm-provider
```

Change the LLM provider at runtime without restarting the client. Valid providers: `anthropic`, `openai`, `ollama`.

**Examples:**

```
You: /set llm-provider openai
System: LLM provider set to: openai (model: gpt-4o)

You: /set llm-provider anthropic
System: LLM provider set to: anthropic (model: claude-sonnet-4-20250514)
```

#### Change LLM Model

```
/set llm-model <model>
/show llm-model
```

Switch to a different model from the current provider. Use `/list models` to see available options.

**Examples:**

```
You: /set llm-model claude-3-opus-20240229
System: LLM model set to: claude-3-opus-20240229 (provider: anthropic)

You: /set llm-model gpt-4-turbo
System: LLM model set to: gpt-4-turbo (provider: openai)
```

#### List Available Models

```
/list models
```

Query the current LLM provider for available models. For Anthropic, shows a curated list. For OpenAI and Ollama, queries the provider's API.

**Example:**

```
You: /list models
System: Available models from anthropic (7):
  * claude-sonnet-4-20250514 (current)
    claude-3-7-sonnet-20250219
    claude-3-5-sonnet-20241022
    claude-3-5-sonnet-20240620
    claude-3-opus-20240229
    claude-3-sonnet-20240229
    claude-3-haiku-20240307
```

#### View Settings

```
/show settings
/show <setting>
```

Display current configuration values. Available settings: `status-messages`, `llm-provider`, `llm-model`, `settings` (all).

**Example:**

```
You: /show settings

Current Settings:
─────────────────────────────────────────────────
UI:
  Status Messages:  on
  No Color:         no

LLM:
  Provider:         anthropic
  Model:            claude-sonnet-4-20250514
  Max Tokens:       4096
  Temperature:      0.70

MCP:
  Mode:             stdio
  Server Path:      ./bin/pgedge-pg-mcp-svr
─────────────────────────────────────────────────
```

### Unknown Slash Commands

If you use a slash command that doesn't match any built-in command, it will be sent to the LLM for interpretation. This allows natural language commands like:

```
You: /explain how indexes work in PostgreSQL
```

The LLM will receive this query and respond with an explanation.

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
You: What tables are in my database?

⠋ Consulting the herd...
  → Executing tool: get_schema_info

Assistant: You have 8 tables in your database:
- users
- products
- orders
- order_items
- categories
- reviews
- inventory
- shipments

You: Show me the 10 most recent orders

  → Executing tool: query_database

Assistant: Here are the 10 most recent orders:

| order_id | customer_name | order_date          | total   | status      |
|----------|---------------|---------------------|---------|-------------|
| 1045     | Alice Smith   | 2024-03-15 14:32:00 | $125.99 | Shipped     |
| 1044     | Bob Jones     | 2024-03-15 12:18:00 | $89.50  | Processing  |
| 1043     | Carol White   | 2024-03-14 16:45:00 | $210.00 | Delivered   |
| ...      | ...           | ...                 | ...     | ...         |

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
- [Chatbot Examples Overview](building-chat-clients.md) - Compare different chatbot approaches
- [Stdio + Anthropic Claude Chatbot](stdio-anthropic-chatbot.md) - Python stdio example
- [HTTP + Ollama Chatbot](http-ollama-chatbot.md) - Python HTTP + Ollama example
- [MCP Server Configuration](configuration.md) - Configure the MCP server
- [Available Tools](tools.md) - List of database tools you can use

**For Developers:**
- [Development Guide](development.md) - Building, testing, and development workflow
