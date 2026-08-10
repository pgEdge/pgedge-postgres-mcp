# Deploying in a Docker Container

Deployment of the pgEdge Postgres MCP Server is easy; you can get up and running in a test environment in minutes. Before deploying the server, you need to install and obtain:

- a Postgres database (with pg_description support)
- Docker
- an LLM Provider API key: [Anthropic](https://console.anthropic.com/),
  [OpenAI](https://platform.openai.com/), or [Ollama](https://ollama.ai/)
  (local/free)

In your Postgres database, you'll need to [create a `LOGIN` user](https://www.postgresql.org/docs/18/sql-createrole.html) for this demo; the user name and password will be shared in the configuration file used for deployment.

## Pre-built Container Images

Pre-built container images are available on the
[GitHub Container Registry](https://github.com/orgs/pgEdge/packages?repo_name=pgedge-postgres-mcp).
The following image variants are available:

- `ghcr.io/pgedge/postgres-mcp:latest` - Base server image.
- `ghcr.io/pgedge/postgres-mcp:latest-with-kb` - Server image
  with the built-in knowledgebase.
- `ghcr.io/pgedge/nla-web:latest` - Web client image.

The
[`examples/docker-compose.production.yml`](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/examples/docker-compose.production.yml)
file provides a production-ready Docker Compose configuration
that uses these pre-built images.

## Deploying into a Docker Container

After meeting the prerequisites, use the steps that follow to
deploy into a Docker container. This approach builds the images
locally from source.

**Clone the Repository**

Clone the `pgedge-postgres-mcp` repository and navigate into
the repository's `root` directory:

```bash
git clone https://github.com/pgEdge/pgedge-postgres-mcp.git
cd pgedge-postgres-mcp
```

**Create a Configuration File**

The `.env.example` file contains a sample configuration file that we can use for deployment; instead of updating the original, we copy the sample file to `.env`:

```bash
cp .env.example .env
```

Then, edit `.env`, adding deployment details. In the `DATABASE CONNECTION`
section, provide Postgres connection details:

```bash
# ============================================================================
# DATABASE CONNECTION (Single Database)
# ============================================================================
# PostgreSQL connection details for a single database.
PGEDGE_DB_HOST=your-postgres-host
PGEDGE_DB_PORT=5432
PGEDGE_DB_NAME=your-database-name
PGEDGE_DB_USER=your-database-user
PGEDGE_DB_PASSWORD=your-database-password
PGEDGE_DB_SSLMODE=prefer
```

!!! tip "Multiple Databases"
    To configure multiple databases, use numbered environment variables like
    `PGEDGE_DB_1_NAME`, `PGEDGE_DB_1_HOST`, etc. See the `.env.example` file
    or [Environment Variable Configuration](env_variable_config.md) for
    details.

!!! tip "Local-only Claude `mcp-remote` and multi-database switching"
    API tokens are scoped to a single database, so when an HTTP client (such
    as Claude via `mcp-remote`) sends a fixed bearer token on every request,
    it cannot switch between configured databases. For trusted local
    development only, you can disable HTTP authentication so that the server
    falls back to its default database selection and the client can switch
    databases freely:
    ```bash
    PGEDGE_AUTH_ENABLED=false
    ```
    This setting only affects HTTP mode; `stdio` has no auth layer. Do
    **not** disable authentication in shared or production environments.

Specify the name of your embedding provider in the `EMBEDDING PROVIDER CONFIGURATION` section:

```bash
# ============================================================================
# EMBEDDING PROVIDER CONFIGURATION
# ============================================================================
# Provider for text embeddings: voyage, openai, gemini, or ollama
PGEDGE_EMBEDDING_PROVIDER=voyage

# Model to use for embeddings
# Voyage: voyage-3, voyage-3-large (requires API key)
# OpenAI: text-embedding-3-small, text-embedding-3-large (requires API key)
# Gemini: gemini-embedding-001 (requires API key)
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

# Google Gemini API key (for Gemini models and Gemini embeddings)
# Get your key from: https://aistudio.google.com/apikey
PGEDGE_GEMINI_API_KEY=your-gemini-api-key-here

# Ollama server URL (for local models)
# Default: http://localhost:11434 (change if Ollama runs elsewhere)
PGEDGE_OLLAMA_URL=http://localhost:11434
```


!!! tip "API Key Security"
    For a production environment, mount API key files instead of using environment variables:
    ```yaml
    volumes:
      - ~/.anthropic-api-key:/app/.anthropic-api-key:ro
    ```

When HTTP mode is enabled, the container initializes
authentication during startup. You can specify user
information in the `AUTHENTICATION CONFIGURATION` section.
For a simple test environment, the `INIT_USERS` property
is the simplest configuration:

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
# OpenAI: gpt-4o, gpt-4-turbo, gpt-4o-mini, etc.
# Ollama: llama3, mistral, etc.
PGEDGE_LLM_MODEL=claude-sonnet-4-20250514
```

**Deploy the Server**

After updating the configuration file, you can start the docker container and deploy the server:

```bash
docker-compose up -d
```

**Connect with a Browser**

When the deployment completes, use your browser to open [http://localhost:8081](http://localhost:8081) and log in with the credentials you set in the  `INIT_USERS` property.

!!! success "You're ready!"
    Start asking questions about your database in natural language.

## Complete Docker Compose Configuration

The following `docker-compose.yml` file shows all available
services with detailed configuration options. You can use this
file as a starting point for production deployments.

In the following example, the `docker-compose.yml` file defines
the MCP server and an optional web client service:

```yaml
services:
    # MCP Server (HTTP mode with authentication)
    postgres-mcp:
        build:
            context: .
            dockerfile: Dockerfile.server
        ports:
            - "8080:8080"
        environment:
            - PGEDGE_HTTP_ENABLED=true
        env_file:
            - .env
        volumes:
            # Mount API key files (recommended over env vars)
            - ./config/tokens.yaml:/app/postgres-mcp-tokens.yaml:ro
            - ./config/users.yaml:/app/postgres-mcp-users.yaml:ro
            # Mount knowledgebase database (optional)
            - ./data/kb.db:/app/kb.db:ro
        healthcheck:
            test: ["CMD", "wget", "--spider", "-q",
                   "http://localhost:8080/health"]
            interval: 30s
            timeout: 5s
            retries: 3
            start_period: 10s
        restart: unless-stopped

    # Web Client (optional)
    web:
        build:
            context: .
            dockerfile: Dockerfile.web
        ports:
            - "8081:80"
        environment:
            - MCP_SERVER_URL=http://postgres-mcp:8080
        depends_on:
            postgres-mcp:
                condition: service_healthy
        restart: unless-stopped
```

The `postgres-mcp` service mounts token and user configuration
files as read-only volumes. The `web` service waits for the MCP
server to pass its health check before starting.

## Complete Environment Variable Reference

The `.env` file controls all server behavior. The following
example shows every available configuration option organized
by section.

In the following example, the `.env` file combines all
configuration sections with explanatory comments:

```bash
# ============================================================
# DATABASE CONNECTION (Single Database)
# ============================================================
PGEDGE_DB_HOST=your-postgres-host
PGEDGE_DB_PORT=5432
PGEDGE_DB_NAME=your-database-name
PGEDGE_DB_USER=your-database-user
PGEDGE_DB_PASSWORD=your-database-password
PGEDGE_DB_SSLMODE=prefer

# Allow write queries (INSERT, UPDATE, DELETE, etc.)
# Default: false - all queries run in read-only transactions
# WARNING: Use with caution. See the Security Guide for details.
# PGEDGE_DB_ALLOW_WRITES=false

# ============================================================
# MULTIPLE DATABASES (Optional)
# ============================================================
# Use numbered variables for additional databases.
# PGEDGE_DB_1_NAME=production
# PGEDGE_DB_1_HOST=prod-db.example.com
# PGEDGE_DB_1_PORT=5432
# PGEDGE_DB_1_DATABASE=myapp
# PGEDGE_DB_1_USER=readonly
# PGEDGE_DB_1_PASSWORD=secret
# PGEDGE_DB_1_SSLMODE=require
#
# PGEDGE_DB_2_NAME=staging
# PGEDGE_DB_2_HOST=staging-db.example.com
# PGEDGE_DB_2_PORT=5432
# PGEDGE_DB_2_DATABASE=myapp_staging
# PGEDGE_DB_2_USER=developer
# PGEDGE_DB_2_PASSWORD=secret
# PGEDGE_DB_2_SSLMODE=prefer

# ============================================================
# EMBEDDING PROVIDER CONFIGURATION
# ============================================================
PGEDGE_EMBEDDING_PROVIDER=voyage
PGEDGE_EMBEDDING_MODEL=voyage-3

# ============================================================
# LLM API KEYS
# ============================================================
PGEDGE_ANTHROPIC_API_KEY=your-anthropic-api-key
PGEDGE_OPENAI_API_KEY=your-openai-api-key
PGEDGE_GEMINI_API_KEY=your-gemini-api-key
PGEDGE_OLLAMA_URL=http://localhost:11434

# ============================================================
# LLM CONFIGURATION FOR CLIENTS
# ============================================================
PGEDGE_LLM_PROVIDER=anthropic
PGEDGE_LLM_MODEL=claude-sonnet-4-20250514

# ============================================================
# AUTHENTICATION CONFIGURATION
# ============================================================
INIT_TOKENS=
INIT_USERS=alice:secret123,bob:secret456
MCP_CLIENT_TOKEN=

# ============================================================
# KNOWLEDGEBASE CONFIGURATION (Optional)
# ============================================================
# PGEDGE_KB_ENABLED=true
# PGEDGE_KB_DATABASE_PATH=/app/kb.db
# PGEDGE_KB_EMBEDDING_PROVIDER=voyage
# PGEDGE_KB_EMBEDDING_MODEL=voyage-3
# PGEDGE_KB_VOYAGE_API_KEY=your-voyage-key
# PGEDGE_KB_GEMINI_API_KEY=your-gemini-key

# ============================================================
# SERVER CONFIGURATION
# ============================================================
# Enable HTTP mode (required for Docker Compose deployments)
PGEDGE_HTTP_ENABLED=true
# PGEDGE_HTTP_ADDRESS=:8080
# PGEDGE_DEBUG=false
# PGEDGE_TRACE_FILE=/app/logs/trace.jsonl
```

Uncomment optional sections as needed for your deployment.
See [Environment Variable Configuration](env_variable_config.md)
for detailed descriptions of each variable.

## Quick Start with Docker

Use the following steps to deploy the MCP server in under
five minutes with a minimal configuration.

1. Clone the repository from GitHub.

    ```bash
    git clone \
        https://github.com/pgEdge/pgedge-postgres-mcp.git
    cd pgedge-postgres-mcp
    ```

2. Copy the example configuration file to `.env`.

    ```bash
    cp .env.example .env
    ```

3. Set the database connection variables in the `.env` file:
   `PGEDGE_DB_HOST`, `PGEDGE_DB_USER`, and
   `PGEDGE_DB_PASSWORD`.

4. Set at least one API key such as
   `PGEDGE_ANTHROPIC_API_KEY` or `PGEDGE_OPENAI_API_KEY`.

5. Set the `INIT_USERS` variable with a username and password
   pair; for example, `alice:secret123`.

6. Start the Docker containers in detached mode.

    ```bash
    docker-compose up -d
    ```

7. Open [http://localhost:8081](http://localhost:8081) in a
   browser and log in with the credentials you configured.

The server begins accepting natural language queries after
the containers finish starting.

## Docker Health Checks

The MCP server exposes a `/health` endpoint for monitoring
container status. Docker Compose uses this endpoint to verify
the server is ready before starting dependent services.

You can check the health status of all containers with the
following command:

```bash
docker-compose ps
```

In the following example, the `curl` command queries the
health endpoint directly:

```bash
curl http://localhost:8080/health
```

The server returns a JSON response that confirms the service
status. The following example shows the expected output:

```json
{
    "status": "ok",
    "server": "pgedge-postgres-mcp",
    "version": "..."
}
```

You can view the container logs to diagnose startup issues or
runtime errors. In the following example, the
`docker-compose logs` command displays the MCP server output:

```bash
docker-compose logs postgres-mcp
```

Add the `-f` flag to follow the log output in real time.

## Stdio Mode with Docker

The Docker image defaults to stdio mode when
`PGEDGE_HTTP_ENABLED` is not set. This allows the
image to work with stdio-based MCP clients such as
the Docker Desktop MCP Toolkit, Claude Code, Claude
Desktop, Cursor, Windsurf, and VS Code Copilot.

In the following example, the `docker run` command
starts the server in stdio mode with a database
connection:

```bash
docker run -i --rm \
    --add-host host.docker.internal:host-gateway \
    -e PGEDGE_DB_HOST=host.docker.internal \
    -e PGEDGE_DB_PORT=5432 \
    -e PGEDGE_DB_NAME=mydb \
    -e PGEDGE_DB_USER=myuser \
    -e PGEDGE_DB_PASSWORD=mypass \
    ghcr.io/pgedge/postgres-mcp:latest
```

The `--add-host` flag ensures `host.docker.internal`
resolves correctly on all platforms. See the
[Quick Start](quickstart.md) guide for client-specific
configuration examples.

## See Also

The following resources provide additional configuration and
deployment guidance:

- [Environment Variable Configuration](env_variable_config.md)
  describes all supported environment variables.
- [Multiple Database Configuration](multiple_db_config.md)
  explains how to connect to several databases.
- [Distributed Deployment](../advanced/distributed-deployment.md)
  covers multi-node deployment architectures.
- [Security Guide](security.md) details authentication and
  access control options.