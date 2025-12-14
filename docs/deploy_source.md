# Building from Source

Deployment of the pgEdge Postgres MCP Server is easy; you can get up and running in a test environment in minutes. Before deploying the server, you need to install and obtain:

- a Postgres database (with pg_description support)
- an LLM Provider API key: [Anthropic](https://console.anthropic.com/),
  [OpenAI](https://platform.openai.com/), or [Ollama](https://ollama.ai/)
  (local/free)

In your Postgres database, you'll need to [create a `LOGIN` user](https://www.postgresql.org/docs/18/sql-createrole.html) for this demo; the user name and password will be shared in the configuration file used for deployment.

After meeting the prerequisites, use the steps that follow to build the MCP server.

**Clone the Repository**

To build from source, first, clone the `pgedge-mcp` repository and navigate into the repository's root directory:

```bash
git clone https://github.com/pgEdge/pgedge-mcp.git
cd pgedge-postgres-mcp
```

Then, build the `pgedge-mcp-server` binary; the file is created in the `bin` directory under your current directory:

```
make build
```

**Create a Configuration File**

The `.env.example` file contains a sample configuration file that we can use for deployment; instead of updating the original, we copy the sample file to `bin/pgedge-mcp-server.yaml`:

```bash
cp .env.example bin/pgedge-mcp-server.yaml
```

Then, edit the configuration file, adding deployment details.  In the `DATABASE CONNECTION` section, provide Postgres connection details:

```bash
# ============================================================================
# DATABASE CONNECTION
# ============================================================================
# PostgreSQL connection details
PGEDGE_DB_HOST=your-postgres-host
PGEDGE_DB_PORT=5432
PGEDGE_DB_NAME=your-database-name
PGEDGE_DB_USER=your-database-user
PGEDGE_DB_PASSWORD=your-database-password
PGEDGE_DB_SSLMODE=prefer
```

Specify the name of your embedding provider in the `EMBEDDING PROVIDER CONFIGURATION` section:

```bash
# ============================================================================
# EMBEDDING PROVIDER CONFIGURATION
# ============================================================================
# Provider for text embeddings: anthropic, openai, or ollama
PGEDGE_EMBEDDING_PROVIDER=anthropic

# Model to use for embeddings
# Anthropic: voyage-3, voyage-3-large (requires API key)
# OpenAI: text-embedding-3-small, text-embedding-3-large (requires API key)
# Ollama: nomic-embed-text, mxbai-embed-large (requires local Ollama)
PGEDGE_EMBEDDING_MODEL=voyage-3
```

Provide your API key in the LLM API KEYS section:

```bash
# ============================================================================
# LLM API KEYS
# ============================================================================
# Anthropic API key (for Claude models and Voyage embeddings)
# Get your key from: https://console.anthropic.com/
PGEDGE_ANTHROPIC_API_KEY=your-anthropic-api-key-here

# OpenAI API key (for GPT models and OpenAI embeddings)
# Get your key from: https://platform.openai.com/
PGEDGE_OPENAI_API_KEY=your-openai-api-key-here

# Ollama server URL (for local models)
# Default: http://localhost:11434 (change if Ollama runs elsewhere)
PGEDGE_OLLAMA_URL=http://localhost:11434
```

During deployment, users are created for the deployment; you can specify user information in the `AUTHENTICATION CONFIGURATION` section.  For a simple test environment, the `INIT_USERS` property is the simplest configuration:

```bash
# ============================================================================
# AUTHENTICATION CONFIGURATION
# ============================================================================
# The server supports both token-based and user-based authentication
# simultaneously. You can initialize both types during container startup.

# Initialize tokens (comma-separated list)
# Use for service-to-service authentication or API access
# Format: token1,token2,token3
# Example: INIT_TOKENS=my-secret-token-1,my-secret-token-2
INIT_TOKENS=

# Initialize users (comma-separated list of username:password pairs)
# Use for interactive user authentication with session tokens
# Format: username1:password1,username2:password2
# Example: INIT_USERS=alice:secret123,bob:secret456
INIT_USERS=

# Client token for CLI access (if using token authentication)
# This should match one of the tokens in INIT_TOKENS
MCP_CLIENT_TOKEN=
```

You also need to specify the LLM provider information in the `LLM CONFIGURATION FOR CLIENTS` section:

```bash
# ============================================================================
# LLM CONFIGURATION FOR CLIENTS
# ============================================================================
# Default LLM provider for chat clients: anthropic, openai, or ollama
PGEDGE_LLM_PROVIDER=anthropic

# Default LLM model for chat clients
# Anthropic: claude-sonnet-4-20250514, claude-opus-4-20250514, etc.
# OpenAI: gpt-5-main, gpt-4o, gpt-4-turbo, etc.
# Ollama: llama3, mistral, etc.
PGEDGE_LLM_MODEL=claude-sonnet-4-20250514
```

**Use the Command Line to Create a User**

```bash
# Add a user for web access
./bin/pgedge-mcp-server -add-user admin -user-password "your_password"
```

**Deploy the Server**

Then, use the following command to start the `pgedge-mcp-server`:

```bash
./bin/pgedge-mcp-server
```

**Connect with a Browser and Authenticate**

Then, use your browser to open [http://localhost:8080](http://localhost:8080) and authenticate with the server.

!!! success "You're ready!"
    Start asking questions about your database in natural language.