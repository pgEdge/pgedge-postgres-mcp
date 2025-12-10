# Quick Start

Get up and running in minutes. Choose your deployment method:

- [Docker](#docker-deployment) - Recommended for most users
- [Native](#native-deployment) - Build from source

## Prerequisites

- PostgreSQL database (any version with pg_description support)
- LLM Provider API key: [Anthropic](https://console.anthropic.com/),
  [OpenAI](https://platform.openai.com/), or [Ollama](https://ollama.ai/)
  (local/free)

---

## Docker Deployment

### 1. Clone and Configure

```bash
git clone https://github.com/pgEdge/pgedge-mcp.git
cd pgedge-postgres-mcp

cp .env.example .env
```

### 2. Edit `.env`

Set your database connection and API keys:

```bash
# Database
PGEDGE_DB_HOST=host.docker.internal  # Use this to connect to host PostgreSQL
PGEDGE_DB_PORT=5432
PGEDGE_DB_NAME=mydb
PGEDGE_DB_USER=postgres
PGEDGE_DB_PASSWORD=your_password

# LLM Provider (choose one)
PGEDGE_LLM_PROVIDER=anthropic
PGEDGE_LLM_MODEL=claude-sonnet-4-20250514
PGEDGE_ANTHROPIC_API_KEY=sk-ant-...

# Authentication
INIT_USERS=admin:your_password
```

### 3. Start Services

```bash
docker-compose up -d
```

### 4. Access the Web Interface

Open [http://localhost:8081](http://localhost:8081) and log in with the
credentials you set in `INIT_USERS`.

!!! success "You're ready!"
    Start asking questions about your database in natural language.

---

## Native Deployment

### 1. Build

```bash
git clone https://github.com/pgEdge/pgedge-mcp.git
cd pgedge-postgres-mcp
make build
```

### 2. Configure

Create `bin/pgedge-mcp-server.yaml`:

```yaml
databases:
  - name: "default"
    host: "localhost"
    port: 5432
    database: "mydb"
    user: "postgres"
    password: "your_password"

http:
  enabled: true
  address: ":8080"
  auth:
    enabled: true
```

### 3. Set API Key

```bash
echo "sk-ant-your-key" > ~/.anthropic-api-key
chmod 600 ~/.anthropic-api-key
```

### 4. Create User and Start

```bash
# Add a user for web access
./bin/pgedge-mcp-server -add-user admin -user-password "your_password"

# Start the server
./bin/pgedge-mcp-server
```

### 5. Access

Open [http://localhost:8080](http://localhost:8080) and log in.

---

## Using with Claude Desktop

For Claude Desktop integration (stdio mode), add to your Claude Desktop config:

**macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/path/to/bin/pgedge-mcp-server",
      "env": {
        "PGHOST": "localhost",
        "PGPORT": "5432",
        "PGDATABASE": "mydb",
        "PGUSER": "myuser",
        "PGPASSWORD": "mypass"
      }
    }
  }
}
```

Restart Claude Desktop and start asking questions about your database.

---

## Next Steps

- [Configuration Guide](guide/configuration.md) - Detailed configuration options
- [Authentication](guide/authentication.md) - User and token management
- [CLI Client](guide/cli-client.md) - Command-line interface
- [Tools Reference](reference/tools.md) - Available database tools
