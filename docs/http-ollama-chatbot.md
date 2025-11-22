# HTTP + Ollama Chatbot

A chatbot example that uses Ollama for local LLM inference and connects to the pgEdge Natural Language Agent via HTTP to answer questions about your PostgreSQL database using natural language.

## Overview

This example demonstrates:

- Connecting to the MCP server via HTTP REST API
- Using Ollama for local, privacy-focused LLM inference
- Tool discovery and execution via HTTP
- Multi-turn conversations with context preservation

## Prerequisites

- Python 3.10+
- Ollama installed ([get it here](https://ollama.com/))
- The pgEdge Natural Language Agent binary built and available
- A PostgreSQL database (connections can be configured via environment variable or through the chatbot)

## Installation

**1. Install and configure Ollama:**

First, install Ollama from [ollama.com](https://ollama.com/), then pull a model:

```bash
ollama pull gpt-oss:20b
```

Start the Ollama service (it usually runs automatically after installation):

```bash
ollama serve
```

**2. Navigate to the example directory:**

```bash
cd examples/http-ollama-chatbot
```

**3. Install Python dependencies:**

```bash
pip install -r requirements.txt
```

The `requirements.txt` contains:

```
httpx==0.27.0
ollama==0.3.3
```

**4. Build and start the MCP server in HTTP mode:**

```bash
cd ../..
go build -o bin/pgedge-pg-mcp-svr ./cmd/pgedge-pg-mcp-svr

# Start the server in HTTP mode (with authentication disabled for this example)
./bin/pgedge-pg-mcp-svr -http -addr :8080 -no-auth
```

**Note:** This example uses `-no-auth` for simplicity. For production deployments,
use token-based or user-based authentication (see [Authentication Guide](authentication.md)).

**5. Set up environment (in a new terminal):**

```bash
# Required: MCP server URL
export PGEDGE_MCP_SERVER_URL="http://localhost:8080/mcp/v1"

# Optional: Ollama configuration (defaults shown)
export OLLAMA_BASE_URL="http://localhost:11434"
export OLLAMA_MODEL="gpt-oss:20b"
```

**Note:** The MCP server must be configured with database connection information
before starting. See "Need to configure a database connection?" in the
Troubleshooting section below.

## Running the Chatbot

```bash
python chatbot.py
```

You should see:

```
Using Ollama model: gpt-oss:20b
MCP Server: http://localhost:8080/mcp/v1
✓ Connected to pgEdge Natural Language Agent

PostgreSQL Chatbot (type 'quit' or 'exit' to stop)
============================================================

Example questions:
  - How many tables do I have?
  - Show me the 10 most recent orders
  - What's the total revenue from last month?
  - Which customers have placed more than 5 orders?
```

## Example Interaction

```
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

## Code Walkthrough

### 1. HTTP Connection with JSON-RPC

The chatbot connects to the MCP server via HTTP using the JSON-RPC 2.0 protocol:

```python
async def _jsonrpc_request(self, method: str, params: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
    """Send a JSON-RPC request to the MCP server."""
    request = {
        "jsonrpc": "2.0",
        "id": self._get_next_id(),
        "method": method,
        "params": params or {}
    }

    response = await self.http_client.post(
        self.mcp_server_url,  # http://localhost:8080/mcp/v1
        json=request,
        headers={"Content-Type": "application/json"}
    )
    response.raise_for_status()
    return response.json().get("result", {})

async def list_available_tools(self) -> List[Dict[str, Any]]:
    """Retrieve available tools from the MCP server."""
    result = await self._jsonrpc_request("tools/list")
    return result.get("tools", [])

async def call_tool(self, tool_name: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
    """Call a tool on the MCP server."""
    return await self._jsonrpc_request("tools/call", {
        "name": tool_name,
        "arguments": arguments
    })
```

This uses HTTP with JSON-RPC to communicate with the MCP server instead of stdio, allowing the server and client to run on different machines.

### 2. Ollama Integration

The chatbot uses Ollama for local LLM inference:

```python
from ollama import AsyncClient

self.ollama = AsyncClient(host=self.ollama_base_url)

# Later, to chat:
response = await self.ollama.chat(
    model=self.ollama_model,
    messages=full_messages
)
```

Since Ollama doesn't natively support tool calling like Claude, the chatbot uses a prompt engineering approach:

### 3. Tool Calling via Prompts

The chatbot formats tools for Ollama and instructs it to respond with JSON when it needs to use a tool:

```python
system_message = f"""You are a helpful PostgreSQL database assistant. You have access to the following tools:

{tools_context}

IMPORTANT INSTRUCTIONS:
1. When you need to use a tool, respond with ONLY a JSON object - no other text before or after:
{{
    "tool": "tool_name",
    "arguments": {{
        "param1": "value1",
        "param2": "value2"
    }}
}}

2. After calling a tool, you will receive actual results from the database.
3. You MUST base your response ONLY on the actual tool results provided - never make up or guess data.
4. If you receive tool results, format them clearly for the user.
5. Only use tools when necessary to answer the user's question.
"""
```

The client then attempts to parse the response as JSON. If successful and the JSON contains a "tool" field, it executes the tool. Otherwise, it treats the response as the final answer to the user.

### 4. Agentic Loop

Similar to the stdio example, but adapted for Ollama:

1. Send user query to Ollama with tool definitions in system message
2. Check if Ollama responded with a tool call (JSON format)
3. Execute the tool via HTTP
4. Return results to Ollama
5. Repeat until Ollama provides a final answer

## Environment Variables

**Client Environment Variables:**

- `PGEDGE_MCP_SERVER_URL` (required): URL of the MCP server running in HTTP mode
- `OLLAMA_BASE_URL` (optional): URL of the Ollama service (default: `http://localhost:11434`)
- `OLLAMA_MODEL` (optional): Ollama model to use (default: `gpt-oss:20b`)

**Server Database Configuration:** See "Need to configure a database connection?"
in the Troubleshooting section below.

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

## Advantages of HTTP + Ollama

1. **Privacy**: All LLM inference happens locally - no data sent to external APIs
2. **No API costs**: Free to use after initial hardware investment
3. **Distributed architecture**: Server and client can run on different machines
4. **Offline capable**: Works without internet connection
5. **Scalable**: Can distribute load across multiple HTTP servers

## Disadvantages

1. **Performance**: Depends on local hardware (may be slower than cloud GPUs)
2. **Model quality**: Ollama models may not match Claude's performance for complex tasks
3. **Complexity**: Requires running both MCP server in HTTP mode and Ollama
4. **Tool calling**: Relies on prompt engineering rather than native tool support, which means models may not consistently follow the JSON format or may hallucinate responses

## Source Code

The complete source code for this example is available in the [`examples/http-ollama-chatbot`](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/examples/http-ollama-chatbot) directory.

## Next Steps

- Try the [Stdio + Anthropic Claude Chatbot](stdio-anthropic-chatbot.md) for a simpler setup with more powerful LLM
- Learn about [HTTP mode configuration](deployment.md) for production
- Explore [authentication options](authentication.md) for securing your HTTP endpoint
- Learn about [available tools](tools.md) you can use
