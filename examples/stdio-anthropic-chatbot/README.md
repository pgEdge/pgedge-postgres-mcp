# Stdio + Anthropic Claude Chatbot Example

A simple chatbot that uses Anthropic's Claude and connects to the pgEdge Natural Language Agent via stdio to answer questions about your PostgreSQL database using natural language.

## Quick Start

**1. Install dependencies:**

```bash
pip install -r requirements.txt
```

**2. Set up environment:**

```bash
# Required: Your Anthropic API key
export ANTHROPIC_API_KEY="sk-ant-..."

# Required: Database connection configuration
# The server connects to the database at startup using PostgreSQL environment variables
export PGHOST="localhost"
export PGPORT="5432"
export PGDATABASE="mydb"
export PGUSER="myuser"
export PGPASSWORD="mypass"  # Or use ~/.pgpass file for better security
```

**3. Build the MCP server** (if not already built):

```bash
cd ../..
go build -o bin/pgedge-postgres-mcp ./cmd/pgedge-pg-mcp-svr
```

**4. Run the chatbot:**

```bash
python chatbot.py
```

## Example Interaction

```
✓ Connected to pgEdge Natural Language Agent

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

Claude: You have 8 tables in your database:
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

Claude: Here are the 10 most recent orders:
1. Order #1234 - Customer: John Doe - Date: 2024-01-15 - Total: $125.99
2. Order #1233 - Customer: Jane Smith - Date: 2024-01-14 - Total: $89.50
...

You: quit

Goodbye!
```

## Documentation

For more details, see the [Stdio + Anthropic Claude Chatbot](../../docs/stdio-anthropic-chatbot.md) documentation.

## Environment Variables

- `ANTHROPIC_API_KEY` (required): Your Anthropic API key
- PostgreSQL connection variables (required): `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` (or use ~/.pgpass file)
- `PGEDGE_MCP_SERVER_PATH` (optional): Custom path to the MCP server binary (default: `../../bin/pgedge-postgres-mcp`)

## Troubleshooting

**"Server not found" error:**

Make sure you've built the server:

```bash
cd ../.. && go build -o bin/pgedge-postgres-mcp ./cmd/pgedge-pg-mcp-svr
```

Or set the path explicitly:

```bash
export PGEDGE_MCP_SERVER_PATH="/path/to/pgedge-postgres-mcp"
```

**"ANTHROPIC_API_KEY environment variable is required":**

Get an API key from https://console.anthropic.com/ and set it:

```bash
export ANTHROPIC_API_KEY="your-key-here"
```

**Need to configure a database connection?**

Set the PostgreSQL environment variables before starting the chatbot:

```bash
export PGHOST="localhost"
export PGPORT="5432"
export PGDATABASE="mydb"
export PGUSER="myuser"
export PGPASSWORD="mypass"  # Or use ~/.pgpass file for better security
```

The MCP server connects to the database at startup using these environment variables.
