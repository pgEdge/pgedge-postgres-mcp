# API Reference

This document provides a complete reference for all API endpoints exposed by
the pgEdge Postgres MCP Server.

## MCP JSON-RPC Endpoints

All MCP protocol methods are available via POST `/mcp/v1`:

### initialize

Initializes the MCP connection.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "pgedge-mcp-web",
      "version": "1.0.0"
    }
  }
}
```

### tools/list

Lists available MCP tools.

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}
```

Response:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "query_database",
        "description": "Execute natural language queries against the database",
        "inputSchema": {
          "type": "object",
          "properties": {
            "query": {
              "type": "string",
              "description": "Natural language query"
            }
          },
          "required": ["query"]
        }
      }
    ]
  }
}
```

### tools/call

Calls an MCP tool.

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "query_database",
    "arguments": {
      "query": "How many users are there?"
    }
  }
}
```

### resources/list

Lists available MCP resources.

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "resources/list"
}
```

### resources/read

Reads an MCP resource.

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "resources/read",
  "params": {
    "uri": "pg://system_info"
  }
}
```

## REST API Endpoints

### GET /health

Health check endpoint (no authentication required).

**Response:**
```json
{
  "status": "ok",
  "server": "pgedge-pg-mcp-svr",
  "version": "1.0.0"
}
```

### GET /api/user/info

Returns information about the authenticated user.

**Request:**
```http
GET /api/user/info HTTP/1.1
Authorization: Bearer <session-token>
```

**Response:**
```json
{
  "username": "alice"
}
```

**Implementation:** [cmd/pgedge-pg-mcp-svr/main.go:454-511](../cmd/pgedge-pg-mcp-svr/main.go#L454-L511)

## LLM Proxy Endpoints

The LLM proxy provides REST API endpoints for chat functionality. See the
[LLM Proxy Guide](llm-proxy.md) for detailed documentation on these endpoints:

- `GET /api/llm/providers` - List configured LLM providers
- `GET /api/llm/models?provider=<provider>` - List available models
- `POST /api/llm/chat` - Send chat request with tool support

## See Also

- [LLM Proxy](llm-proxy.md) - LLM proxy endpoints and usage
- [MCP Protocol](mcp-protocol.md) - MCP protocol specification
- [Tools Documentation](tools.md) - Available MCP tools
- [Resources Documentation](resources.md) - Available MCP resources
