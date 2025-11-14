# MCP Protocol Implementation

This document describes how the pgEdge MCP Server implements the Model Context Protocol (MCP).

## Protocol Version

This server implements **MCP version `2024-11-05`**.

## Transport Modes

The server supports two transport modes:

### 1. stdio Mode (Default)

**Used by**: Claude Desktop and other MCP clients

**Communication**: JSON-RPC 2.0 over standard input/output

**How it works**:

- Server reads JSON-RPC messages from `stdin`
- Server writes JSON-RPC responses to `stdout`
- Errors and diagnostics written to `stderr`

**Starting in stdio mode**:
```bash
./bin/pgedge-pg-mcp-svr
```

### 2. HTTP/HTTPS Mode

**Used by**: Web applications, direct API access, custom integrations

**Communication**: JSON-RPC 2.0 over HTTP/HTTPS with streaming support

**Endpoints**:

- `POST /mcp/v1` - JSON-RPC endpoint
- `GET /health` - Health check endpoint

**How it works**:

- Client sends HTTP POST request with JSON-RPC payload
- Server processes request and returns JSON-RPC response
- Supports HTTP/1.1 with keep-alive
- Streaming responses for real-time data

**Starting in HTTP mode**:
```bash
./bin/pgedge-pg-mcp-svr -http -addr ":8080"
```

For deployment details, see [Deployment Guide](deployment.md).

## MCP Capabilities

The server advertises these capabilities during initialization:

```json
{
  "capabilities": {
    "tools": {},
    "resources": {},
    "prompts": {}
  }
}
```

### Tools

Ten callable functions for database interaction and management:

1. **server_info** - Get MCP server information
2. **query_database** - Execute natural language queries against PostgreSQL
3. **get_schema_info** - Get detailed database schema information
4. **set_pg_configuration** - Modify PostgreSQL configuration parameters
5. **read_resource** - Read MCP resources by URI
6. **add_database_connection** - Save database connection with alias
7. **remove_database_connection** - Remove saved database connection
8. **list_database_connections** - List all saved database connections
9. **edit_database_connection** - Edit existing database connection
10. **set_database_connection** - Set connection string for current session

For detailed tool documentation, see [Tools Documentation](tools.md).

### Resources

Four read-only resources for system information and statistics:

1. **pg://system_info** - PostgreSQL version and system information
2. **pg://settings** - Server configuration parameters
3. **pg://stat/activity** - Current activity and connections
4. **pg://stat/replication** - Replication status

For detailed resource documentation, see [Resources Documentation](resources.md).

### Prompts

Custom prompts for common database tasks (extensible).

## JSON-RPC Messages

### Request Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "method_name",
  "params": {
    "param1": "value1",
    "param2": "value2"
  }
}
```

### Response Format

**Success**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "data": "..."
  }
}
```

**Error**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid Request"
  }
}
```

## MCP Methods

### Initialize

Establish connection and negotiate capabilities.

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "roots": {
        "listChanged": true
      }
    },
    "clientInfo": {
      "name": "my-client",
      "version": "1.0.0"
    }
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {},
      "resources": {},
      "prompts": {}
    },
    "serverInfo": {
      "name": "pgedge-postgres-mcp",
      "version": "1.0.0-alpha1"
    }
  }
}
```

### List Tools

Get available tools.

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list",
  "params": {}
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "query_database",
        "description": "Execute a natural language query...",
        "inputSchema": {
          "type": "object",
          "properties": {
            "query": {
              "type": "string",
              "description": "Natural language question..."
            }
          },
          "required": ["query"]
        }
      }
      // ... more tools
    ]
  }
}
```

### Call Tool

Execute a tool.

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "query_database",
    "arguments": {
      "query": "Show me all active users"
    }
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Natural Language Query: Show me all active users\n\nGenerated SQL:\nSELECT * FROM users WHERE status = 'active';\n\nResults:\n..."
      }
    ]
  }
}
```

### List Resources

Get available resources.

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "resources/list",
  "params": {}
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "resources": [
      {
        "uri": "pg://system_info",
        "name": "PostgreSQL System Information",
        "description": "Returns PostgreSQL version...",
        "mimeType": "application/json"
      }
      // ... more resources
    ]
  }
}
```

### Read Resource

Read a specific resource.

**Request**:
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

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "contents": [
      {
        "uri": "pg://system_info",
        "mimeType": "application/json",
        "text": "{\"version\":\"PostgreSQL 16.0\",\"os\":\"Linux\",\"arch\":\"x86_64\"}"
      }
    ]
  }
}
```

## Error Codes

Standard JSON-RPC error codes:

| Code | Message | Meaning |
|------|---------|---------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid Request | Invalid JSON-RPC request |
| -32601 | Method not found | Method doesn't exist |
| -32602 | Invalid params | Invalid method parameters |
| -32603 | Internal error | Server error |

Custom error codes:

| Code | Message | Meaning |
|------|---------|---------|
| -32001 | Database not ready | Database still initializing |
| -32002 | Tool not found | Requested tool doesn't exist |
| -32003 | Resource not found | Requested resource doesn't exist |

## Streaming (HTTP Mode)

In HTTP mode, the server supports streaming responses for tools that generate large outputs.

**How it works**:

1. Client sends request to `/mcp/v1`
2. Server processes request
3. Server streams response chunks
4. Client receives progressive updates

**Example - Streaming Query Results**:

The `query_database` tool can stream results as they're generated:

```javascript
const response = await fetch('https://localhost:8080/mcp/v1', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer YOUR_TOKEN',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'tools/call',
    params: {
      name: 'query_database',
      arguments: { query: 'Show all users' }
    }
  })
});

// Stream response
const reader = response.body.getReader();
while (true) {
  const { done, value } = await reader.read();
  if (done) break;
  // Process chunk
  console.log(new TextDecoder().decode(value));
}
```

## Authentication (HTTP Mode)

HTTP mode uses Bearer token authentication.

**Request with authentication**:
```bash
curl -X POST https://localhost:8080/mcp/v1 \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{...}'
```

**Unauthenticated request returns 401**:
```json
{
  "error": "Unauthorized"
}
```

For token management, see [Authentication Guide](authentication.md).

## Protocol Extensions

### Multi-Database Support

The `query_database` tool supports querying multiple databases:

**Temporary connection**:
```json
{
  "method": "tools/call",
  "params": {
    "name": "query_database",
    "arguments": {
      "query": "Show users at postgres://otherhost/otherdb"
    }
  }
}
```

**Change default connection**:
```json
{
  "method": "tools/call",
  "params": {
    "name": "query_database",
    "arguments": {
      "query": "set default database to postgres://newhost/newdb"
    }
  }
}
```

### Read-Only Transaction Enforcement

All queries via `query_database` are executed in read-only transactions:

```sql
BEGIN;
SET TRANSACTION READ ONLY;
-- User's query here
COMMIT;
```

This prevents data modification while allowing full read access.

## Implementation Details

### Server Lifecycle

1. **Startup**:

    - Parse configuration
    - Connect to PostgreSQL
    - Load database metadata (tables, columns, types)
    - Initialize tool and resource registries
    - Start transport (stdio or HTTP)

2. **Request Handling**:

    - Receive JSON-RPC message
    - Validate request format
    - Route to appropriate handler
    - Execute tool/resource
    - Return JSON-RPC response

3. **Shutdown**:

    - Close database connections
    - Clean up resources
    - Exit gracefully

### Metadata Loading

On startup, the server loads:

- All tables and views (excluding system schemas)
- Column names and data types
- Nullability constraints
- Table and column comments from `pg_description`

This metadata is used for:

- Schema information tools
- Natural language to SQL conversion
- Query validation

### Error Handling

The server provides detailed error messages in development:

- Database connection errors
- SQL syntax errors
- Permission errors
- API key errors

In production, errors are logged but sanitized in responses.

## Testing MCP Protocol

### With MCP Inspector

```bash
npx @modelcontextprotocol/inspector /path/to/bin/pgedge-pg-mcp-svr
```

The MCP Inspector provides a web UI for testing:

- Initialize connection
- List tools and resources
- Call tools interactively
- View request/response JSON

### With curl (HTTP mode)

```bash
# Start server
./bin/pgedge-pg-mcp-svr -http -no-auth

# Initialize
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "curl", "version": "1.0"}
    }
  }'

# List tools
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Content-Type": "application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list",
    "params": {}
  }'
```

### Integration Tests

The project includes comprehensive integration tests for MCP protocol compliance:

```bash
go test ./test/... -v -run TestMCPCompliance
```

Tests verify:

- Protocol version negotiation
- Tool and resource registration
- Request/response format
- Error handling
- Streaming support (HTTP mode)

For more testing information, see [Testing Guide](testing.md).

## Related Documentation

- [Tools Documentation](tools.md) - Detailed tool reference
- [Resources Documentation](resources.md) - Detailed resource reference
- [Deployment Guide](deployment.md) - HTTP/HTTPS mode deployment
- [Authentication Guide](authentication.md) - HTTP authentication
- [Examples](examples.md) - Query examples and patterns
- [Testing Guide](testing.md) - Testing procedures
- [Architecture Guide](architecture.md) - Implementation details
