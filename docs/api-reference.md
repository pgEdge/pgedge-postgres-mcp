# API Reference

This document provides a complete reference for all API endpoints exposed by
the pgEdge Natural Language Agent.

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

### POST /api/chat/compact

Smart chat history compaction endpoint. Intelligently compresses message
history to reduce token usage while preserving semantically important
context. Uses PostgreSQL and MCP-aware classification to identify anchor
messages, important tool results, schema information, and error messages.

**Request:**
```http
POST /api/chat/compact HTTP/1.1
Content-Type: application/json

{
    "messages": [
        {"role": "user", "content": "Show me the users table"},
        {"role": "assistant", "content": "Here's the schema..."},
        ...
    ],
    "max_tokens": 100000,
    "recent_window": 10,
    "keep_anchors": true,
    "options": {
        "preserve_tool_results": true,
        "preserve_schema_info": true,
        "enable_summarization": true,
        "min_important_messages": 3,
        "token_counter_type": "anthropic",
        "enable_llm_summarization": false,
        "enable_caching": false,
        "enable_analytics": false
    }
}
```

**Parameters:**

- `messages` (required): Array of chat messages to compact
- `max_tokens` (optional): Maximum token budget, default 100000
- `recent_window` (optional): Number of recent messages to preserve, default
  10
- `keep_anchors` (optional): Whether to keep anchor messages, default true
- `options` (optional): Fine-grained compaction options
    - `preserve_tool_results`: Keep all tool execution results
    - `preserve_schema_info`: Keep schema-related messages
    - `enable_summarization`: Create summaries of compressed segments
    - `min_important_messages`: Minimum important messages to keep
    - `token_counter_type`: Token counting strategy - `"generic"`,
      `"openai"`, `"anthropic"`, `"ollama"`
    - `enable_llm_summarization`: Use enhanced summarization (extracts
      actions, entities, errors)
    - `enable_caching`: Enable result caching with SHA256-based keys
    - `enable_analytics`: Track compression metrics

**Response:**
```json
{
    "messages": [
        {"role": "user", "content": "Show me the users table"},
        {"role": "assistant", "content": "[Compressed context: Topics: database queries, Tables: users, 5 messages compressed]"},
        ...
    ],
    "summary": {
        "topics": ["database queries"],
        "tables": ["users"],
        "tools": ["query_database"],
        "description": "[Compressed context: Topics: database queries, Tables: users, Tools used: query_database, 5 messages compressed]"
    },
    "token_estimate": 2500,
    "compaction_info": {
        "original_count": 20,
        "compacted_count": 8,
        "dropped_count": 12,
        "anchor_count": 3,
        "tokens_saved": 7500,
        "compression_ratio": 0.25
    }
}
```

**Message Classification:**

The compactor uses a 5-tier classification system:

- **Anchor** - Critical context (schema changes, user corrections, tool
  schemas)
- **Important** - High-value messages (query analysis, errors, insights)
- **Contextual** - Useful context (keep if space allows)
- **Routine** - Standard messages (can be compressed)
- **Transient** - Low-value messages (short acknowledgments)

**Implementation:** [internal/compactor/](../internal/compactor/)

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
