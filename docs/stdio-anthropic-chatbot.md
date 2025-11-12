# Stdio + Anthropic Claude Chatbot

A chatbot example that uses Anthropic's Claude and connects to the pgEdge Postgres MCP Server via stdio to answer questions about your PostgreSQL database using natural language.

## Overview

This example demonstrates:

- Connecting to the MCP server via stdio (standard input/output)
- Using Anthropic's Claude for natural language processing
- Tool discovery and execution
- Multi-turn conversations with context preservation

## Prerequisites

- Python 3.10+
- An Anthropic API key ([get one here](https://console.anthropic.com/))
- The pgEdge Postgres MCP Server binary built and available
- A PostgreSQL database (connections can be configured via environment variable or through the chatbot)

## Installation

**1. Navigate to the example directory:**

```bash
cd examples/stdio-anthropic-chatbot
```

**2. Install dependencies:**

```bash
pip install -r requirements.txt
```

The `requirements.txt` contains:

```
anthropic>=0.40.0
mcp>=1.0.0
```

**3. Set up environment:**

```bash
# Required: Your Anthropic API key
export ANTHROPIC_API_KEY="sk-ant-..."

# Optional: Database connection string
# (You can also configure connections through the chatbot)
export PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:pass@localhost/mydb"
```

**4. Build the MCP server** (if not already built):

```bash
cd ../..
go build -o bin/pgedge-postgres-mcp ./cmd/pgedge-postgres-mcp
```

## Running the Chatbot

```bash
python chatbot.py
```

You should see:

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
```

## Example Interaction

```
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

## Code Walkthrough

### 1. Stdio Connection

The chatbot connects to the MCP server by spawning it as a subprocess and communicating via stdio:

```python
async def connect_to_server(self, server_path: str):
    """Connect to the pgEdge Postgres MCP Server via stdio."""
    server_params = StdioServerParameters(
        command=server_path,
        args=[],
        env=None  # Inherits environment variables
    )

    stdio_transport = await self.exit_stack.enter_async_context(
        stdio_client(server_params)
    )
    self.stdio, self.write = stdio_transport

    self.session = await self.exit_stack.enter_async_context(
        ClientSession(self.stdio, self.write)
    )

    await self.session.initialize()
```

This spawns the MCP server process and establishes stdio communication. The server inherits environment variables (like `PGEDGE_POSTGRES_CONNECTION_STRING` if set).

### 2. Tool Discovery

```python
tools_response = await self.session.list_tools()

available_tools = []
for tool in tools_response.tools:
    available_tools.append({
        "name": tool.name,
        "description": tool.description,
        "input_schema": tool.inputSchema
    })
```

The client retrieves all available tools from the MCP server and converts them to Anthropic's tool format.

### 3. Agentic Loop

The client implements an agentic loop:

1. Send user query to Claude with tool definitions
2. Check if Claude wants to use a tool
3. Execute all tool calls via MCP
4. Return results to Claude
5. Repeat until Claude provides a final answer

```python
while True:
    response = self.anthropic.messages.create(
        model="claude-sonnet-4-20250514",
        max_tokens=4096,
        messages=messages,
        tools=available_tools
    )

    if response.stop_reason == "tool_use":
        # Execute tools and continue
        # ...
    else:
        # Final response
        return final_response
```

## Environment Variables

- `ANTHROPIC_API_KEY` (required): Your Anthropic API key
- `PGEDGE_POSTGRES_CONNECTION_STRING` (optional): PostgreSQL connection string
- `PGEDGE_MCP_SERVER_PATH` (optional): Custom path to the MCP server binary (default: `../../bin/pgedge-postgres-mcp`)

## Troubleshooting

**"Server not found" error:**

Make sure you've built the server:

```bash
cd ../.. && go build -o bin/pgedge-postgres-mcp ./cmd/pgedge-postgres-mcp
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

You can configure connections through the chatbot interface:

- Ask: "Add a connection to my database at postgres://user:password@host:port/database"
- Or set the environment variable: `export PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:password@host:port/database"`

## Source Code

The complete source code for this example is available in the [`examples/stdio-anthropic-chatbot`](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/examples/stdio-anthropic-chatbot) directory.

## Next Steps

- Try the [HTTP + Ollama Chatbot](http-ollama-chatbot.md) for a local, privacy-focused alternative
- Learn about [available tools](tools.md) you can use
- Set up [authentication](authentication.md) for production deployments
