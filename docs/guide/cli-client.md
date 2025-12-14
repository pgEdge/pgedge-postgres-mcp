# Using the Go Chat Client

The Natural Language Agent Go Chat Client is a **production-ready, full-featured** native Go implementation that provides an interactive command-line interface for chatting with your PostgreSQL database using natural language.

This is the recommended client for production use and provides significantly more features and polish than the [Python examples](../developers/building-chat-clients.md), which are intended as simple reference implementations to demonstrate the MCP protocol.

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
go build -o bin/pgedge-nla-cli ./cmd/pgedge-pg-mcp-cli

# Or using Make
make client

# Build both server and client
make build
```

The binary will be created at `bin/pgedge-nla-cli`.

## Quick Start

The easiest way to start the CLI client is using one of the provided startup
scripts, which handle building binaries, validating configuration, and checking
for required API keys:

### Stdio Mode (Recommended for Single User)

```bash
./start_cli_stdio.sh
```

The MCP server runs as a subprocess. Simpler setup, ideal for local development
and single-user scenarios.

### HTTP Mode (Recommended for Multi-User or Remote Access)

```bash
./start_cli_http.sh
```

The MCP server runs as a separate HTTP service with authentication. Supports
multiple concurrent users and remote connections. The server automatically shuts
down when the CLI exits.

Both scripts will:

- **Auto-build**: Build CLI and server binaries if needed or if source files
  changed
- **Validate**: Check that configuration files exist
- **Check environment**: Warn about missing LLM API keys or database
  configuration
- **Start client**: Launch the CLI with proper configuration

### Custom Configuration

You can specify a custom CLI configuration file:

```bash
CONFIG_FILE=/path/to/custom.yaml ./start_cli_stdio.sh
# or
CONFIG_FILE=/path/to/custom.yaml ./start_cli_http.sh
```

### What You Need

Before running the startup script, make sure you have:

1. **LLM Provider** (at least one):
   - Anthropic: Set `ANTHROPIC_API_KEY` or `PGEDGE_ANTHROPIC_API_KEY`
     environment variable, OR create `~/.anthropic-api-key` file
   - OpenAI: Set `OPENAI_API_KEY` or `PGEDGE_OPENAI_API_KEY` environment
     variable, OR create `~/.openai-api-key` file
   - Ollama: Set `PGEDGE_OLLAMA_URL` (e.g., `http://localhost:11434`)

2. **Database Connection** (optional, uses defaults if not set):
   - PostgreSQL variables: `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`,
     `PGPASSWORD`
   - Or connection string: `PGEDGE_POSTGRES_CONNECTION_STRING`
   - Or pgEdge variables: `PGEDGE_DB_HOST`, `PGEDGE_DB_PORT`,
     `PGEDGE_DB_NAME`, `PGEDGE_DB_USER`, `PGEDGE_DB_PASSWORD`

The script will provide helpful warnings if any of these are missing, allowing
you to set them up before proceeding.

## Configuration

The chat client can be configured in three ways (in order of precedence):

1. Command-line flags
2. Environment variables
3. Configuration file

### Configuration File

Create a `.pgedge-nla-cli.yaml` file in one of these locations:

- Current directory: `./.pgedge-nla-cli.yaml`
- Home directory: `~/.pgedge-nla-cli.yaml`
- System-wide: `/etc/pgedge/pgedge-nla-cli.yaml`

Example configuration:

```yaml
mcp:
    mode: stdio
    server_path: ./bin/pgedge-mcp-server
    server_config_path: ./bin/pgedge-mcp-server-stdio.yaml
    # For HTTP mode:
    # url: http://localhost:8080
    # token: your-token-here

llm:
    provider: anthropic  # Options: anthropic, openai, or ollama
    model: claude-sonnet-4-20250514

    # API keys (priority: env vars > key files > direct config values)
    # Option 1: Environment variables (recommended for development)
    # Option 2: API key files (recommended for production)
    anthropic_api_key_file: ~/.anthropic-api-key
    openai_api_key_file: ~/.openai-api-key
    # Option 3: Direct values (not recommended - use env vars or files)
    # anthropic_api_key: your-anthropic-key-here
    # openai_api_key: your-openai-key-here

    max_tokens: 4096
    temperature: 0.7

ui:
    no_color: false
    display_status_messages: true  # Show/hide status messages during tool execution
    render_markdown: true  # Render markdown with formatting and syntax highlighting
```

For a complete configuration file example with all available options and detailed comments, see the [Chat Client Config Example](../reference/config-examples/cli-client.md).

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

**API Key Priority:**

API keys are loaded in the following priority order (highest to lowest):

1. Environment variables (`PGEDGE_ANTHROPIC_API_KEY`, `PGEDGE_OPENAI_API_KEY`)
2. API key files (`~/.anthropic-api-key`, `~/.openai-api-key`)
3. Configuration file values (not recommended)

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

- **Anthropic Claude**: Full support
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
./bin/pgedge-nla-cli
```

The client will:

1. Start the MCP server as a subprocess
2. Connect via stdin/stdout
3. Use Anthropic Claude for natural language processing

### Example 1b: Stdio Mode with API Key File

For production or when you don't want to use environment variables, store
your API key in a file:

```bash
# Create API key file
echo "sk-ant-your-key-here" > ~/.anthropic-api-key
chmod 600 ~/.anthropic-api-key

# Run the chat client (will read from file automatically)
./bin/pgedge-nla-cli
```

**Benefits of using API key files:**

- No need to set environment variables in every shell session
- More secure than hardcoding in configuration files
- Easy to manage different keys for different environments
- Proper file permissions (600) prevent unauthorized access

### Example 2: Stdio Mode with OpenAI

Use OpenAI's GPT models for natural language processing.

```bash
# Set your OpenAI API key
export PGEDGE_OPENAI_API_KEY="your-api-key-here"

# Run the chat client with OpenAI
./bin/pgedge-nla-cli \
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

**Easiest method** - Use the startup script:

```bash
./start_cli_http.sh
```

This automatically starts the MCP server in HTTP mode with authentication and
connects the CLI to it. The server shuts down when you exit the CLI.

**Manual method** - Connect to a running MCP server:

```bash
# Set the server URL and token
export PGEDGE_MCP_URL="http://localhost:8080"
export PGEDGE_MCP_TOKEN="your-token-here"

# Run the chat client
./bin/pgedge-nla-cli -mcp-mode http
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
./bin/pgedge-nla-cli \
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
./bin/pgedge-nla-cli -config .pgedge-pg-mcp-cli.yaml
```

## Interactive Commands

Once the chat client is running, you can use these special commands:

- `help` - Show available commands
- `quit` or `exit` - Exit the chat client (also Ctrl+C or Ctrl+D)
- `clear` - Clear the screen
- `tools` - List available MCP tools
- `resources` - List available MCP resources

## Keyboard Shortcuts

The CLI supports the following keyboard shortcuts:

| Key | Action |
|-----|--------|
| Escape | Cancel the current LLM request and return to prompt |
| Up/Down | Navigate through command history |
| Ctrl+R | Reverse search through command history |
| Ctrl+C | Exit the chat client |
| Ctrl+D | Exit the chat client (EOF) |

### Cancelling Requests

While waiting for an LLM response (during the "Thinking..." animation), you can
press the **Escape** key to cancel the request and return to the prompt
immediately. This is useful when:

- A query is taking too long
- You realize you made a mistake in your question
- You want to ask a different question instead

When you cancel a request, your query remains in the conversation history so the
LLM has context for follow-up questions. The Escape keypress itself is not saved
to any history.

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

Control whether status messages are displayed during tool execution. Useful
for cleaner output or debugging.

**Examples:**

```
You: /set status-messages off
System: Status messages disabled

You: /set status-messages on
System: Status messages enabled
```

#### Color Output

```
/set color <on|off>
/show color
```

Enable or disable colored output in the CLI. When enabled, the interface uses
colors for different message types (errors in red, system messages in yellow,
etc.). When disabled, all output is plain text without ANSI color codes.

This setting is persisted across sessions.

**Examples:**

```
You: /set color off
System: Colored output disabled

You: /set color on
System: Colored output enabled
```

**Note:** The `NO_COLOR` environment variable takes precedence over this
setting. If `NO_COLOR` is set, colors will be disabled regardless of this
preference.

#### Markdown Rendering

```
/set markdown <on|off>
/show markdown
```

Enable or disable markdown rendering in assistant responses. When enabled,
markdown content is rendered with:

- **Formatted headings** - Different colors for different header levels
- **Syntax highlighting** - Color-coded code blocks for multiple languages
- **Styled lists** - Properly formatted bullet points and numbered lists
- **Formatted tables** - Clean table rendering with box-drawing characters
- **Emphasized text** - Bold and italic styling

When disabled, responses are shown as plain text without formatting.

**Examples:**

```
You: /set markdown on
System: Markdown rendering enabled

You: /set markdown off
System: Markdown rendering disabled
```

**Note:** Markdown rendering uses the dark theme by default. If you have
`no_color: true` in your configuration, markdown will be rendered without
colors but still with formatting structure.

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

Query the current LLM provider for available models. For Anthropic, shows a
curated list. For OpenAI and Ollama, queries the provider's API.

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

#### Database Management

When connected to a server with multiple databases configured, you can list,
view, and switch between accessible databases.

```
/list databases
/show database
/set database <name>
```

**List Available Databases:**

Shows all databases you have access to, with an indicator for the currently
selected database.

```
You: /list databases
System: Available databases (3):
  * production (postgres@prod-db.example.com:5432/myapp)
    staging (developer@staging-db.example.com:5432/myapp_staging)
    development (developer@localhost:5432/myapp_dev)
```

**Show Current Database:**

Displays the currently selected database connection.

```
You: /show database
System: Current database: production
```

**Switch Database:**

Switch to a different database connection. The database must be in your list
of accessible databases.

```
You: /set database staging
System: Switched to database: staging
```

**Notes:**

- Database access is controlled by the server configuration (see [Configuration Guide](multiple_db_config.md))
- Your database selection is saved and restored on subsequent sessions
- In STDIO mode, all configured databases are accessible
- API tokens may be bound to a specific database

#### View Settings

```
/show settings
/show <setting>
```

Display current configuration values. Available settings: `status-messages`,
`markdown`, `debug`, `llm-provider`, `llm-model`, `database`, `settings` (all).

**Example:**

```
You: /show settings

Current Settings:
─────────────────────────────────────────────────
UI:
  Status Messages:  on
  Render Markdown:  on
  Color:            on

LLM:
  Provider:         anthropic
  Model:            claude-sonnet-4-20250514
  Max Tokens:       4096
  Temperature:      0.70

MCP:
  Mode:             stdio
  Server Path:      ./bin/pgedge-mcp-server

Database:
  Current:          production
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

- **History file**: `~/.pgedge-nla-cli-history`
- **History limit**: 1000 entries
- **Navigation**: Use Up/Down arrow keys to navigate through command history
- **Search**: Use Ctrl+R for reverse search through history
- **Line editing**: Full line editing with Emacs-style keybindings

The history persists across sessions, so your previous queries and commands
are available when you restart the client.

## Conversation History

When running in HTTP mode with authentication enabled, the CLI automatically
saves your conversations to the server. This allows you to access them across
different sessions and continue where you left off.

### Saving Conversations

To save your current conversation:

```
You: /save
System: Conversation saved: What tables exist? (ID: conv_1765197705551269000)
```

The conversation title is automatically generated from your first message. If
you continue an existing conversation, `/save` updates it rather than creating
a new one.

### Listing Conversations

View all your saved conversations:

```
You: /history
System: Saved conversations (3):

  conv_1765197705551269000 (current) [production]
    Title: What tables exist?
    Updated: Dec 08, 14:32
    Preview: What tables exist in the database?

  conv_1765197612345678000 [staging]
    Title: Query performance
    Updated: Dec 07, 10:15
    Preview: How can I optimize my slow queries?

  conv_1765197501234567000 [production]
    Title: Schema design
    Updated: Dec 06, 16:45
    Preview: Help me design a schema for...
```

Each conversation shows:

- **ID**: Unique identifier for loading/managing the conversation
- **Current marker**: Shows which conversation is currently loaded
- **Database connection**: The database that was active (in brackets)
- **Title**: Auto-generated or custom title
- **Updated**: When the conversation was last modified
- **Preview**: First few words of the initial message

### Loading Conversations

Load a previous conversation to continue it:

```
You: /history load conv_1765197705551269000
System: Loaded conversation: What tables exist?
System: Messages: 4, Provider: anthropic, Model: claude-sonnet-4-20250514
System: Database: production

────────────────────────────── Conversation History ──────────────────────────────

You: What tables exist in the database?
Assistant: Based on the schema, you have the following tables...

────────────────────────────── End of History ──────────────────────────────

You:
```

When loading a conversation:

- The message history is restored
- The LLM provider and model are switched to match what you were using
- The database connection is restored if different from current
- The conversation history is replayed in muted colors for context

### Starting a New Conversation

Clear the current conversation and start fresh:

```
You: /new
System: Started new conversation
```

This clears the message history but doesn't delete any saved conversations.

### Renaming Conversations

Give a conversation a more descriptive title:

```
You: /history rename conv_1765197705551269000 "Database schema exploration"
System: Conversation renamed to: Database schema exploration
```

### Deleting Conversations

Delete a single conversation:

```
You: /history delete conv_1765197705551269000
System: Conversation deleted
```

Delete all your conversations:

```
You: /history delete-all
System: Deleted 3 conversation(s)
```

### Conversation History Commands Summary

| Command | Description |
|---------|-------------|
| `/history` | List all saved conversations |
| `/history load <id>` | Load a saved conversation |
| `/history rename <id> "title"` | Rename a conversation |
| `/history delete <id>` | Delete a conversation |
| `/history delete-all` | Delete all conversations |
| `/new` | Start a new conversation |
| `/save` | Save the current conversation |

**Note:** Conversation history requires HTTP mode with authentication. These
commands are not available in stdio mode.

## Example Conversation

This shows the client's elephant-themed UI in action, including the thinking animation and tool execution messages:

```
          _
   ______/ \-.   _           Natural Language Agent Chat Client
.-/     (    o\_//           Type 'quit' or 'exit' to leave, 'help' for commands
 |  ___  \_/\---'
 |_||  |_||

System: Connected to MCP server
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

- In stdio mode, verify the server path is correct: `-mcp-server-path ./bin/pgedge-mcp-server`
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

- Check that `~/.pgedge-nla-cli-history` is writable
- The history file is created automatically on first use
- On some terminals, readline features may be limited

