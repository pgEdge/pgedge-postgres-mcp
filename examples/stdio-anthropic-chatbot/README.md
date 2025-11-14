# Stdio + Anthropic Claude Chatbot Example

A simple chatbot that uses Anthropic's Claude and connects to the pgEdge Postgres MCP Server via stdio to answer questions about your PostgreSQL database using natural language.

## Quick Start

**1. Install dependencies:**

```bash
pip install -r requirements.txt
```

**2. Set up environment:**

```bash
# Required: Your Anthropic API key
export ANTHROPIC_API_KEY="sk-ant-..."

# Optional: Database connection string
# (You can also configure connections through the chatbot)
export PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:pass@localhost/mydb"
```

**3. Build the MCP server** (if not already built):

```bash
cd ../..
go build -o bin/pgedge-pg-mcp-svr ./cmd/pgedge-pg-mcp-svr
```

**4. Run the chatbot:**

```bash
python chatbot.py
```

## Example Interaction

```
✓ Connected to pgEdge Postgres MCP Server

PostgreSQL Chatbot (type 'quit' or 'exit' to stop)
============================================================

To get started, you can:
  - List saved connections: 'What database connections do I have?'
  - Add a connection: 'Add a connection to my database at postgres://user:pass@host/db'
  - Connect to a saved connection: 'Connect to production'
  - List the available MCP server tools and resources: 'List the tools and resources in the MCP server
  
Example questions:
  - How many tables do I have?
  - Show me the 10 most recent orders
  - What's the total revenue from last month?
  - Which customers have placed more than 5 orders?

You: Connect to production

  → Executing tool: set_database_connection

Claude: Connected to the 'production' database.

You: How many tables do I have?

  → Executing tool: query_database

Claude: You have 8 tables in your database:
- users
- products
- orders
- order_items
- categories
- reviews
- inventory
- shipments

You: quit

Goodbye!
```

## Documentation

For more details, see the [Stdio + Anthropic Claude Chatbot](../../docs/stdio-anthropic-chatbot.md) documentation.

## Environment Variables

- `ANTHROPIC_API_KEY` (required): Your Anthropic API key
- `PGEDGE_POSTGRES_CONNECTION_STRING` (optional): PostgreSQL connection string
- `PGEDGE_MCP_SERVER_PATH` (optional): Custom path to the MCP server binary (default: `../../bin/pgedge-pg-mcp-svr`)

## Troubleshooting

**"Server not found" error:**

Make sure you've built the server:

```bash
cd ../.. && go build -o bin/pgedge-pg-mcp-svr ./cmd/pgedge-pg-mcp-svr
```

Or set the path explicitly:

```bash
export PGEDGE_MCP_SERVER_PATH="/path/to/pgedge-pg-mcp-svr"
```

**"ANTHROPIC_API_KEY environment variable is required":**

Get an API key from https://console.anthropic.com/ and set it:

```bash
export ANTHROPIC_API_KEY="your-key-here"
```

**Need to configure a database connection?**

You can configure connections through the chatbot interface:

- Ask: "Add a connection to my database at postgres://user:password@host:port/database"
- Or set the environment variable: `export PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:password@host:port/database"`
