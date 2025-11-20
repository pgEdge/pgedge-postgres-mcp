# HTTP + Ollama Chatbot Example

A simple chatbot that uses Ollama and connects to the pgEdge Postgres MCP Server via HTTP to answer questions about your PostgreSQL database using natural language.

## Quick Start

**1. Install and configure Ollama:**

First, install Ollama from [ollama.com](https://ollama.com/), then pull a model:

```bash
ollama pull gpt-oss:20b
```

Start the Ollama service (it usually runs automatically after installation):

```bash
ollama serve
```

**2. Install Python dependencies:**

```bash
pip install -r requirements.txt
```

**3. Build and start the MCP server in HTTP mode:**

```bash
cd ../..
go build -o bin/pgedge-pg-mcp-svr ./cmd/pgedge-pg-mcp-svr

# Start the server in HTTP mode (with authentication disabled for this example)
./bin/pgedge-pg-mcp-svr -http -addr :8080 -no-auth
```

**Note:** This example uses `-no-auth` for simplicity. For production deployments,
use token-based or user-based authentication (see [Authentication Guide](../../docs/authentication.md)).

**4. Set up environment (in a new terminal):**

```bash
# Required: MCP server URL
export PGEDGE_MCP_SERVER_URL="http://localhost:8080/mcp/v1"

# Optional: Ollama configuration (defaults shown)
export OLLAMA_BASE_URL="http://localhost:11434"
export OLLAMA_MODEL="gpt-oss:20b"
```

**5. Run the chatbot:**

```bash
python chatbot.py
```

## Example Interaction

```
Using Ollama model: gpt-oss:20b
MCP Server: http://localhost:8080/mcp/v1
✓ Connected to pgEdge Postgres MCP Server

PostgreSQL Chatbot (type 'quit' or 'exit' to stop)
============================================================

Example questions:
  - List all tables: 'What tables are in my database?'
  - Show me the schema: 'Describe the users table'
  - Query data: 'Show me the 10 most recent orders'
  - Aggregate data: 'What's the total revenue from last month?'
  - Complex queries: 'Which customers have placed more than 5 orders?'
  - Search content: 'Find articles about PostgreSQL' (if using vector search)

You: What tables are in my database?

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

You: quit

Goodbye\!
```

## Documentation

For more details, see the [HTTP + Ollama Chatbot](../../docs/http-ollama-chatbot.md) documentation.

## Environment Variables

- `PGEDGE_MCP_SERVER_URL` (required): URL of the MCP server running in HTTP mode
- `OLLAMA_BASE_URL` (optional): URL of the Ollama service (default: `http://localhost:11434`)
- `OLLAMA_MODEL` (optional): Ollama model to use (default: `gpt-oss:20b`)

**Note:** The MCP server must be started with database connection parameters configured via command-line flags or config file.

## Available Ollama Models

You can use any model that Ollama supports. Popular choices:

- `gpt-oss:20b` (default, recommended)
- `llama3`
- `llama3.1`
- `mistral`
- `mixtral`
- `codellama`

To pull a different model:

```bash
ollama pull mistral
export OLLAMA_MODEL="mistral"
```

## Troubleshooting

**"PGEDGE_MCP_SERVER_URL environment variable is required":**

Make sure the MCP server is running in HTTP mode:

```bash
./bin/pgedge-pg-mcp-svr -http -addr :8080 -no-auth
export PGEDGE_MCP_SERVER_URL="http://localhost:8080/mcp/v1"
```

**"Connection refused" or similar HTTP errors:**

1. Check that the MCP server is running: `curl -X POST http://localhost:8080/mcp/v1 -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'`
2. Verify the URL matches your `PGEDGE_MCP_SERVER_URL` (should end with `/mcp/v1`)
3. Check server logs for errors

**"Ollama not found" or model errors:**

1. Install Ollama from https://ollama.com/
2. Pull a model: `ollama pull gpt-oss:20b`
3. Verify Ollama is running: `curl http://localhost:11434/api/tags`

**Need to configure a database connection?**

The MCP server connects to the database at startup. You can configure the connection using:

- Command-line flags: `./bin/pgedge-pg-mcp-svr -http -addr :8080 -no-auth -db-host localhost -db-port 5432 -db-name mydb -db-user myuser -db-password mypass`
- Config file: Create a `pgedge-pg-mcp-svr.yaml` file with database connection parameters
- Environment variables: Set `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` before starting the server

See the [Configuration Guide](../../docs/configuration.md) for more details.
