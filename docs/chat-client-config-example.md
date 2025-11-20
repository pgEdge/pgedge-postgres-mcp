```yaml
# pgEdge Postgres MCP Chat Client Configuration Example
#
# Configuration Priority (highest to lowest):
#   1. Command line flags
#   2. Environment variables
#   3. Configuration file values (this file)
#   4. Hard-coded defaults
#
# Copy this file to .pgedge-pg-mcp-cli.yaml or ~/.pgedge-pg-mcp-cli.yaml
# and customize it for your environment.

# ============================================================================
# MCP SERVER CONNECTION CONFIGURATION
# ============================================================================
mcp:
    # Connection mode: "stdio" or "http"
    # stdio: Spawns MCP server as subprocess (local only)
    # http: Connects to MCP server via HTTP/HTTPS (can be remote)
    # Default: stdio
    # Environment variable: PGEDGE_MCP_MODE
    # Command line flag: -mcp-mode
    mode: stdio

    # -------------------------
    # Stdio Mode Configuration
    # -------------------------
    # Path to MCP server binary (for stdio mode)
    # Default: ./bin/pgedge-pg-mcp-svr
    # Environment variable: PGEDGE_MCP_SERVER_PATH
    # Command line flag: -mcp-server-path
    server_path: ./bin/pgedge-pg-mcp-svr

    # Path to MCP server config file (for stdio mode)
    # If specified, the --config flag will be passed to the server binary
    # If not specified, the server will use its default config file lookup
    # Default: (none - server uses default config lookup)
    # Environment variable: PGEDGE_MCP_SERVER_CONFIG_PATH
    server_config_path: ./bin/pgedge-pg-mcp-svr-stdio.yaml

    # -------------------------
    # HTTP Mode Configuration
    # -------------------------
    # MCP server URL (for HTTP mode)
    # Should include protocol and optionally path: http://host:port or https://host:port
    # The /mcp/v1 path will be appended automatically if not present
    # Default: http://localhost:8080
    # Environment variable: PGEDGE_MCP_URL
    # Command line flag: -mcp-url
    # url: http://localhost:8080

    # Authentication token for HTTP mode
    # Token priority: PGEDGE_MCP_TOKEN env var >
    #                 ~/.pgedge-pg-mcp-cli-token file >
    #                 this config value >
    #                 prompt at startup
    # Environment variable: PGEDGE_MCP_TOKEN
    # Token file: ~/.pgedge-pg-mcp-cli-token
    # token: your-token-here

    # Use TLS/HTTPS for HTTP mode
    # Default: false
    # Command line flag: (inferred from URL protocol)
    # tls: false

# ============================================================================
# LLM PROVIDER CONFIGURATION
# ============================================================================
llm:
    # Provider: "anthropic", "openai", or "ollama"
    # anthropic: Uses Anthropic's Claude API (requires API key)
    # openai: Uses OpenAI's GPT API (requires API key)
    # ollama: Uses locally running Ollama server (no API key needed)
    # Default: anthropic
    # Environment variable: PGEDGE_LLM_PROVIDER
    # Command line flag: -llm-provider
    provider: anthropic

    # Model to use
    # Anthropic models: claude-sonnet-4-20250514, claude-opus-4-20250514, etc.
    # OpenAI models: gpt-5-main, gpt-5-thinking, gpt-4o, gpt-4-turbo, gpt-3.5-turbo, etc.
    # Ollama models: llama3, llama3.1, mistral, gpt-oss:20b, etc.
    # Default: claude-sonnet-4-20250514 (anthropic), gpt-5-main (openai), or llama3 (ollama)
    # Environment variable: PGEDGE_LLM_MODEL
    # Command line flag: -llm-model
    model: claude-sonnet-4-20250514

    # -------------------------
    # Anthropic Configuration
    # -------------------------
    # API key for Anthropic
    # Get your API key from: https://console.anthropic.com/
    #
    # Priority (highest to lowest):
    # 1. Environment variable: PGEDGE_ANTHROPIC_API_KEY or ANTHROPIC_API_KEY
    # 2. API key file: anthropic_api_key_file
    # 3. Direct config value: anthropic_api_key (not recommended)
    #
    # Command line flag: -anthropic-api-key
    #
    # Option 1: Environment variable (recommended for development)
    # export PGEDGE_ANTHROPIC_API_KEY="sk-ant-your-key-here"
    #
    # Option 2: API key file (recommended for production)
    anthropic_api_key_file: ~/.anthropic-api-key
    #
    # Option 3: Direct value (not recommended - use env var or file)
    # anthropic_api_key: your-anthropic-api-key-here

    # -------------------------
    # OpenAI Configuration
    # -------------------------
    # API key for OpenAI
    # Get your API key from: https://platform.openai.com/
    #
    # Priority (highest to lowest):
    # 1. Environment variable: PGEDGE_OPENAI_API_KEY or OPENAI_API_KEY
    # 2. API key file: openai_api_key_file
    # 3. Direct config value: openai_api_key (not recommended)
    #
    # Command line flag: -openai-api-key
    #
    # Option 1: Environment variable (recommended for development)
    # export PGEDGE_OPENAI_API_KEY="sk-proj-your-key-here"
    #
    # Option 2: API key file (recommended for production)
    openai_api_key_file: ~/.openai-api-key
    #
    # Option 3: Direct value (not recommended - use env var or file)
    # openai_api_key: your-openai-api-key-here

    # Maximum tokens for LLM response
    # For GPT-5 and o-series models, automatically uses max_completion_tokens
    # For older models, uses max_tokens
    # Default: 4096
    # Command line flag: (not available)
    max_tokens: 4096

    # Temperature for sampling (0.0-1.0)
    # Lower = more focused/deterministic, Higher = more creative/random
    # Note: GPT-5 and o-series models only support default temperature (1)
    # Default: 0.7
    # Command line flag: (not available)
    temperature: 0.7

    # -------------------------
    # Ollama Configuration
    # -------------------------
    # Ollama server URL
    # Default: http://localhost:11434
    # Environment variable: PGEDGE_OLLAMA_URL
    # Command line flag: -ollama-url
    ollama_url: http://localhost:11434

# ============================================================================
# USER INTERFACE CONFIGURATION
# ============================================================================
ui:
    # Disable colored output
    # Useful for environments that don't support ANSI color codes
    # Default: false
    # Command line flag: -no-color
    no_color: false

    # Display status messages during tool execution
    # Shows messages like "→ Executing tool: query_database" during operations
    # Can be toggled at runtime with /set status-messages <on|off>
    # Default: true
    # Command line flag: (not available, use /set command at runtime)
    display_status_messages: true
```

## Configuration Examples

### Stdio Mode with Anthropic Claude

```yaml
mcp:
    mode: stdio
    server_path: ./bin/pgedge-pg-mcp-svr
    server_config_path: ./bin/pgedge-pg-mcp-svr-stdio.yaml

llm:
    provider: anthropic
    model: claude-sonnet-4-20250514
    # Set PGEDGE_ANTHROPIC_API_KEY environment variable
```

Then run:

```bash
export PGEDGE_ANTHROPIC_API_KEY="your-key-here"
./bin/pgedge-pg-mcp-cli
```

### Stdio Mode with OpenAI

```yaml
mcp:
    mode: stdio
    server_path: ./bin/pgedge-pg-mcp-svr
    server_config_path: ./bin/pgedge-pg-mcp-svr-stdio.yaml

llm:
    provider: openai
    model: gpt-5-main
    # Set PGEDGE_OPENAI_API_KEY environment variable
```

Then run:

```bash
export PGEDGE_OPENAI_API_KEY="your-key-here"
./bin/pgedge-pg-mcp-cli
```

### HTTP Mode with Authentication

```yaml
mcp:
    mode: http
    url: https://mcp.example.com:8080
    tls: true

llm:
    provider: anthropic
    model: claude-sonnet-4-20250514
```

Then run:

```bash
export PGEDGE_ANTHROPIC_API_KEY="your-key-here"
export PGEDGE_MCP_TOKEN="your-mcp-token"
./bin/pgedge-pg-mcp-cli
```

### Local Setup with Ollama

```yaml
mcp:
    mode: stdio
    server_path: ./bin/pgedge-pg-mcp-svr
    server_config_path: ./bin/pgedge-pg-mcp-svr-stdio.yaml

llm:
    provider: ollama
    model: llama3
    ollama_url: http://localhost:11434
```

Then run:

```bash
# Make sure Ollama is running with the model pulled
ollama pull llama3
./bin/pgedge-pg-mcp-cli
```

### Remote HTTP Server with Ollama

```yaml
mcp:
    mode: http
    url: http://mcp-server.internal:8080

llm:
    provider: ollama
    model: gpt-oss:20b
    ollama_url: http://localhost:11434
```

## Environment Variables

The chat client supports the following environment variables (in order of precedence):

### MCP Connection

- `PGEDGE_MCP_MODE`: Connection mode (`stdio` or `http`)
- `PGEDGE_MCP_SERVER_PATH`: Path to MCP server binary (stdio mode)
- `PGEDGE_MCP_SERVER_CONFIG_PATH`: Path to MCP server config file (stdio
    mode)
- `PGEDGE_MCP_URL`: MCP server URL (http mode)
- `PGEDGE_MCP_TOKEN`: Authentication token (http mode)

### LLM Configuration

- `PGEDGE_LLM_PROVIDER`: LLM provider (`anthropic`, `openai`, or `ollama`)
- `PGEDGE_LLM_MODEL`: Model to use
- `PGEDGE_ANTHROPIC_API_KEY`: Anthropic API key
- `PGEDGE_OPENAI_API_KEY`: OpenAI API key
- `PGEDGE_OLLAMA_URL`: Ollama server URL

## Command Line Flags

All configuration options can be overridden with command line flags:

```bash
./bin/pgedge-pg-mcp-cli \
    -config /path/to/config.yaml \
    -mcp-mode http \
    -mcp-url https://mcp.example.com:8080 \
    -llm-provider anthropic \
    -llm-model claude-opus-4-20250514 \
    -anthropic-api-key your-anthropic-key \
    -no-color
```

Run `./bin/pgedge-pg-mcp-cli --help` to see all available flags.

## Token File Location

For HTTP mode authentication, the token can be stored in:

```
~/.pgedge-pg-mcp-cli-token
```

This file should contain only the token (no newlines or extra whitespace).

## See Also

- [Go Chat Client Documentation](using-cli-client.md) - Complete usage guide
- [MCP Server Configuration](config-example.md) - Configure the MCP server
- [Authentication](authentication.md) - Set up API tokens for HTTP mode
