# Application Development Overview

This section helps developers build applications that use the pgEdge Postgres
MCP Server.

## What You Can Build

The MCP server provides multiple interfaces for building database applications:

1. **MCP Protocol Clients** - Build chat clients that use the JSON-RPC
   protocol
2. **Web Applications** - Use the LLM proxy and REST APIs
3. **Custom Integrations** - Integrate with existing applications

## Quick Start

### 1. Understanding the Architecture

The MCP server exposes two main interfaces:

- **JSON-RPC (MCP Protocol)** - For MCP-compatible clients
- **REST APIs** - For LLM proxy and utility endpoints

See [Architecture](architecture.md) for the complete system overview.

### 2. Choose Your Approach

**Option A: Build an MCP Protocol Client**

Best for: Creating chat clients similar to Claude Desktop

- Implements Model Context Protocol (JSON-RPC 2.0)
- Direct access to MCP tools and resources
- Full control over agentic loop

See: [Building Chat Clients](building-chat-clients.md)

**Option B: Use the LLM Proxy**

Best for: Web applications that need AI-powered database access

- Server-side API key management
- Pre-built LLM provider integration
- REST API endpoints

See: [LLM Proxy](llm-proxy.md)

**Option C: Direct API Integration**

Best for: Custom integrations and automation

- Direct JSON-RPC access to tools
- No LLM required
- Scriptable and automation-friendly

See: [API Reference](api-reference.md)

## Core Concepts

### MCP Tools

MCP tools are functions that can be called via the protocol:

- `query_database` - Execute natural language queries
- `execute_sql` - Run SQL directly
- `get_schema_info` - Get database schema
- `hybrid_search` - BM25+MMR semantic search
- `generate_embedding` - Create vector embeddings

See: [Tools Documentation](tools.md)

### MCP Resources

MCP resources provide read-only access to system information:

- `pg://system_info` - PostgreSQL server information
- `pg://stat/activity` - Current database activity
- `pg://stat/database` - Database statistics

See: [Resources Documentation](resources.md)

### Authentication

The server supports two authentication modes:

1. **Token-based** - API tokens for automation
2. **User-based** - Username/password for interactive clients

See: [Authentication](authentication.md)

## Example: Building a Simple Client

Here's a minimal example in Python:

```python
import requests
import json

# MCP server endpoint
MCP_URL = "http://localhost:8080/mcp/v1"
SESSION_TOKEN = "your-token-here"

# Initialize connection
response = requests.post(MCP_URL, json={
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
        "protocolVersion": "2024-11-05",
        "capabilities": {},
        "clientInfo": {
            "name": "my-client",
            "version": "1.0.0"
        }
    }
}, headers={
    "Authorization": f"Bearer {SESSION_TOKEN}"
})

# Call a tool
response = requests.post(MCP_URL, json={
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
        "name": "query_database",
        "arguments": {
            "query": "How many tables are in the database?"
        }
    }
}, headers={
    "Authorization": f"Bearer {SESSION_TOKEN}"
})

result = response.json()
print(result)
```

## Development Resources

### Documentation

- **[MCP Protocol Reference](mcp-protocol.md)** - Complete protocol
  specification
- **[API Reference](api-reference.md)** - All available endpoints
- **[LLM Proxy](llm-proxy.md)** - Building web clients with LLM integration
- **[Architecture](architecture.md)** - System design and components

### Example Implementations

- **[Python Examples](building-chat-clients.md)** - Sample chat clients
  - Stdio + Anthropic Claude
  - HTTP + Ollama
- **[Go CLI Client](using-cli-client.md)** - Full-featured reference
  implementation
- **[Web Client](../web/README.md)** - React-based web interface

### Configuration

- **[Server Configuration](configuration.md)** - Configure the MCP server
- **[Docker Deployment](docker-deployment.md)** - Deploy with Docker
- **[Query Examples](examples.md)** - Common use cases

## Next Steps

1. **Read the Protocol** - Understand [MCP Protocol](mcp-protocol.md)
2. **Review Examples** - See [Building Chat Clients](building-chat-clients.md)
3. **Try the APIs** - Use [API Reference](api-reference.md)
4. **Deploy** - Follow [Docker Deployment](docker-deployment.md)

## Support

- **Questions?** See [Troubleshooting](troubleshooting.md)
- **Bug reports:** [GitHub Issues](https://github.com/pgEdge/pgedge-postgres-mcp/issues)
- **Examples:** [Query Examples](examples.md)
