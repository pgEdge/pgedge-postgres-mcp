# Application Development Overview

This section helps developers build applications that use the Natural Language
Agent.

## What You Can Build

The MCP server provides multiple interfaces for building database applications:

- MCP Protocol clients provide chat clients that use the JSON-RPC protocol.
- Web applications use the LLM proxy and REST APIs to access MCP services.
- Custom integrations enable existing applications to integrate with MCP.

## Getting Started with the MCP Server

This section gives a concise path to start developing with the MCP server.

**Understanding the Architecture**

The MCP server exposes two main interfaces:

- JSON-RPC (MCP Protocol) - For MCP-compatible clients
- REST APIs - For LLM proxy and utility endpoints

See [Architecture](../contributing/architecture.md) for the complete system overview.

**Choosing Your Approach**

Choose from the following deployment options when evaluating your approach:

**Option A: Build an MCP Protocol Client**

Best for: Creating chat clients similar to Claude Desktop

- The client implements the Model Context Protocol (JSON-RPC 2.0).
- The client provides direct access to MCP tools and resources.
- The client gives full control over the agentic loop.

See: [Building Chat Clients](building-chat-clients.md)

**Option B: Use the LLM Proxy**

Best for: Web applications that need AI-powered database access

- The server provides server-side API key management.
- The platform includes pre-built LLM provider integration.
- The project exposes REST API endpoints.

See: [LLM Proxy](../advanced/llm-proxy.md)

**Option C: Direct API Integration**

Best for: Custom integrations and automation

- The integration offers direct JSON-RPC access to tools.
- The approach requires no LLM for some workflows.
- The integration is scriptable and automation-friendly.

See: [API Reference](api-reference.md)

## Core Concepts
This section describes key MCP concepts: tools, resources, and authentication.

**Using MCP Tools**

MCP tools are functions that can be called via the protocol:

| Tool | Description |
|---|---|
| `query_database` | The `query_database` tool executes natural language queries. |
| `execute_sql` | The `execute_sql` tool runs SQL directly. |
| `get_schema_info` | The `get_schema_info` tool returns database schema information. |
| `hybrid_search` | The `hybrid_search` tool performs BM25+MMR semantic search. |
| `generate_embedding` | The `generate_embedding` tool creates vector embeddings. |

For more information, see: [Tools Documentation](../reference/tools.md)

**Using MCP Resources**

MCP resources provide read-only access to system information:

| Resource | Description |
|---|---|
| `pg://system_info` | The `pg://system_info` resource provides PostgreSQL server information. |

For more information, see: [Resources Documentation](../reference/resources.md)

### Authentication

The server supports two authentication modes:

* **Token-based** - API tokens for automation
* **User-based** - Username/password for interactive clients

For more information, see: [Authentication](../guide/authentication.md)


## Example: Building a Simple Client

In the following example, the server initializes a session and builds a simple client:

```python
import requests
import json

# MCP server endpoint
MCP_URL = "http://localhost:8080/mcp/v1"
SESSION_TOKEN = "your-token-here"

# Initialize connection
response = requests.post(MCP_URL, json={
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

As a developer, you can find answers to common questions in the following documentation and example links.

**MCP Server Documentation**

- [MCP Protocol Reference](mcp-protocol.md) - Complete protocol specification
- [API Reference](api-reference.md) - All available endpoints
- [LLM Proxy](../advanced/llm-proxy.md) - Building web clients with LLM integration
- [Architecture](../contributing/architecture.md) - System design and components

**Implementation Examples**

- [Python Examples](building-chat-clients.md) - Sample chat clients
    - Stdio + Anthropic Claude
    - HTTP + Ollama
- [Go CLI Client](../guide/cli-client.md) - Full-featured reference
    implementation
- [Web Client](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/web/README.md) - React-based web interface

**Configuration Examples**

- [Server Configuration](../guide/configuration.md) - Configuring the MCP server
- [Deploying the MCP Server with Docker](../guide/deploy_docker.md) - Deploying the MCP server with Docker.
- [Deploying the MCP Server from Source](../guide/deploy_source.md) - Deploying the server from source code.
- [Best Practices for Querying the Server](../guide/querying.md) - Helpful hints that will improve your server queries.

## Support

If you still have questions after reviewing the Development resources listed above, review the Troubleshooting page.  If you run into a problem that you suspect is coming from an issue in the server, log a bug report at the Github site.

- **Questions?** See [Troubleshooting](../guide/troubleshooting.md)
- **Bug reports:** [GitHub Issues](https://github.com/pgEdge/pgedge-postgres-mcp/issues)
