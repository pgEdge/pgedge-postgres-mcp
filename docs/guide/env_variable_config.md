# Using Environment Variables to Specify Options

The server supports environment variables for all configuration options. All environment variables use the **`PGEDGE_`** prefix to avoid collisions with other software.  The following environment variables specify HTTP/HTTPS Server preferences:

- **`PGEDGE_HTTP_ENABLED`**: Enable HTTP transport mode ("true", "1", "yes" to enable)
- **`PGEDGE_HTTP_ADDRESS`**: HTTP server address (default: ":8080")

The following environment variables specify TLS/HTTPS preferences:

- **`PGEDGE_TLS_ENABLED`**: Enable TLS/HTTPS ("true", "1", "yes" to enable)
- **`PGEDGE_TLS_CERT_FILE`**: Path to TLS certificate file
- **`PGEDGE_TLS_KEY_FILE`**: Path to TLS key file
- **`PGEDGE_TLS_CHAIN_FILE`**: Path to TLS certificate chain file (optional)

The following environment variables specify authentication preferences:

- **`PGEDGE_AUTH_ENABLED`**: Enable API token authentication ("true", "1", "yes" to enable)
- **`PGEDGE_AUTH_TOKEN_FILE`**: Path to API token file
- **`PGEDGE_AUTH_USER_FILE`**: Path to user authentication file

The following environment variables specify LLM provider configuration:

- **`PGEDGE_LLM_ENABLED`**: Enable LLM proxy for web clients
- **`PGEDGE_LLM_PROVIDER`**: LLM provider ("anthropic", "openai", or "ollama")
- **`PGEDGE_LLM_MODEL`**: Default model to use
- **`PGEDGE_ANTHROPIC_API_KEY`**: Anthropic API key (or `ANTHROPIC_API_KEY`)
- **`PGEDGE_ANTHROPIC_BASE_URL`**: Custom Anthropic API base URL (for proxies)
- **`PGEDGE_OPENAI_API_KEY`**: OpenAI API key (or `OPENAI_API_KEY`)
- **`PGEDGE_OPENAI_BASE_URL`**: Custom OpenAI API base URL (for proxies)
- **`PGEDGE_OLLAMA_URL`**: Ollama server URL

The following environment variables specify embedding provider configuration:

- **`PGEDGE_VOYAGE_API_KEY`**: Voyage AI API key (or `VOYAGE_API_KEY`)
- **`PGEDGE_VOYAGE_BASE_URL`**: Custom Voyage API base URL (for proxies)
- **`PGEDGE_OPENAI_EMBEDDING_BASE_URL`**: Custom OpenAI embeddings base URL

The following environment variables specify knowledgebase embedding configuration:

- **`PGEDGE_KB_VOYAGE_API_KEY`**: Voyage API key for knowledgebase
- **`PGEDGE_KB_VOYAGE_BASE_URL`**: Custom Voyage base URL for knowledgebase
- **`PGEDGE_KB_OPENAI_API_KEY`**: OpenAI API key for knowledgebase
- **`PGEDGE_KB_OPENAI_BASE_URL`**: Custom OpenAI base URL for knowledgebase
- **`PGEDGE_KB_OLLAMA_URL`**: Ollama URL for knowledgebase

If you run into issues with your environment variable settings, check:

```bash
# Verify environment variables are set
env | grep PGEDGE

# Export the variables if you are running in a new shell
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_HTTP_ADDRESS=":8080"
```

**Examples - Deploying the MCP Server with Environment Variables**

**Configuring an HTTP server with authentication:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_HTTP_ADDRESS=":8080"
export PGEDGE_AUTH_ENABLED="true"
export PGEDGE_AUTH_TOKEN_FILE="./pgedge-postgres-mcp-tokens.yaml"

./bin/pgedge-postgres-mcp
```

**Configuring a HTTPS server:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_TLS_ENABLED="true"
export PGEDGE_TLS_CERT_FILE="./server.crt"
export PGEDGE_TLS_KEY_FILE="./server.key"

./bin/pgedge-postgres-mcp
```

**Using Environment Variables for Tests:**

Tests use a separate environment variable to avoid confusion with runtime configuration:

```bash
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING="postgres://localhost/postgres?sslmode=disable"
go test ./...
```