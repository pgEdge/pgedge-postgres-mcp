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
      "name": "pgedge-nla-web",
      "version": "1.0.0-alpha2"
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
  "server": "pgedge-postgres-mcp",
  "version": "1.0.0-alpha2"
}
```

### GET /api/databases

Lists all databases accessible to the authenticated user.

**Request:**
```http
GET /api/databases HTTP/1.1
Authorization: Bearer <session-token>
```

**Response:**
```json
{
    "databases": [
        {
            "name": "production",
            "host": "localhost",
            "port": 5432,
            "database": "myapp",
            "user": "appuser",
            "sslmode": "require"
        },
        {
            "name": "analytics",
            "host": "analytics.example.com",
            "port": 5432,
            "database": "analytics",
            "user": "analyst",
            "sslmode": "require"
        }
    ],
    "current": "production"
}
```

**Response Fields:**

- `databases` - Array of accessible database configurations
    - `name` - Unique name for this database connection
    - `host` - Database server hostname
    - `port` - Database server port
    - `database` - PostgreSQL database name
    - `user` - Database username
    - `sslmode` - SSL connection mode
- `current` - Name of the currently selected database

**Access Control:**

- Users only see databases listed in their `available_databases` configuration
- API tokens only see their bound database (if configured)
- In STDIO mode or with authentication disabled, all configured databases are
  visible

**Implementation:**
[internal/api/databases.go](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/internal/api/databases.go)

### POST /api/databases/select

Selects a database as the current database for subsequent operations.

**Request:**
```http
POST /api/databases/select HTTP/1.1
Content-Type: application/json
Authorization: Bearer <session-token>

{
    "name": "analytics"
}
```

**Parameters:**

- `name` (required) - Name of the database to select

**Success Response (200):**
```json
{
    "success": true,
    "current": "analytics",
    "message": "Database selected successfully"
}
```

**Error Responses:**

*Invalid request (400):*
```json
{
    "success": false,
    "error": "Database name is required"
}
```

*Database not found (404):*
```json
{
    "success": false,
    "error": "Database not found"
}
```

*Access denied (403):*
```json
{
    "success": false,
    "error": "Access denied to this database"
}
```

*API token bound to different database (403):*
```json
{
    "success": false,
    "error": "API token is bound to a different database"
}
```

**Notes:**

- Database selection is per-session (tied to the authentication token)
- API tokens with a bound database cannot switch to a different database
- Users can only select databases they have access to
- The selected database persists for the duration of the session

**Implementation:**
[internal/api/databases.go](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/internal/api/databases.go)

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

**Implementation:** [cmd/pgedge-pg-mcp-svr/main.go:454-511](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/cmd/pgedge-pg-mcp-svr/main.go#L454-L511)

### POST /api/user/logout

Ends the password-login session named by the bearer token, so that the
token can no longer be used. The endpoint requires no authentication. It
leaves API tokens and OAuth tokens alone; the web client calls it on logout.

**Request:**
```http
POST /api/user/logout HTTP/1.1
Authorization: Bearer <session-token>
```

**Response:**

The server answers `204 No Content` whether or not the token was a live
session, which reveals nothing about the token. A request that uses any
method other than `POST` receives `405 Method Not Allowed`.

**Implementation:** [cmd/pgedge-pg-mcp-svr/logout.go](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/cmd/pgedge-pg-mcp-svr/logout.go)

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

**Implementation:** [internal/compactor/](https://github.com/pgEdge/pgedge-postgres-mcp/tree/main/internal/compactor)

## Conversations API

The conversations API provides endpoints for managing chat history persistence.
These endpoints are only available when user authentication is enabled.

### GET /api/conversations

Lists conversations for the authenticated user.

**Request:**
```http
GET /api/conversations?limit=50&offset=0 HTTP/1.1
Authorization: Bearer <session-token>
```

**Query Parameters:**

- `limit` (optional) - Maximum number of conversations to return (default: 50)
- `offset` (optional) - Number of conversations to skip for pagination
  (default: 0)

**Response:**
```json
{
    "conversations": [
        {
            "id": "conv_abc123",
            "title": "Database schema exploration",
            "connection": "production",
            "created_at": "2025-01-15T10:30:00Z",
            "updated_at": "2025-01-15T11:45:00Z",
            "preview": "Show me the users table..."
        }
    ]
}
```

### POST /api/conversations

Creates a new conversation.

**Request:**
```http
POST /api/conversations HTTP/1.1
Content-Type: application/json
Authorization: Bearer <session-token>

{
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "connection": "production",
    "messages": [
        {
            "role": "user",
            "content": "Show me the users table"
        },
        {
            "role": "assistant",
            "content": "Here's the schema for the users table..."
        }
    ]
}
```

**Response (201 Created):**
```json
{
    "id": "conv_abc123",
    "username": "alice",
    "title": "Show me the users table",
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "connection": "production",
    "messages": [...],
    "created_at": "2025-01-15T10:30:00Z",
    "updated_at": "2025-01-15T10:30:00Z"
}
```

### GET /api/conversations/{id}

Retrieves a specific conversation.

**Request:**
```http
GET /api/conversations/conv_abc123 HTTP/1.1
Authorization: Bearer <session-token>
```

**Response:**
```json
{
    "id": "conv_abc123",
    "username": "alice",
    "title": "Database schema exploration",
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "connection": "production",
    "messages": [
        {
            "role": "user",
            "content": "Show me the users table",
            "timestamp": "2025-01-15T10:30:00Z"
        },
        {
            "role": "assistant",
            "content": "Here's the schema...",
            "timestamp": "2025-01-15T10:30:05Z",
            "provider": "anthropic",
            "model": "claude-sonnet-4-20250514"
        }
    ],
    "created_at": "2025-01-15T10:30:00Z",
    "updated_at": "2025-01-15T11:45:00Z"
}
```

### PUT /api/conversations/{id}

Updates a conversation (replaces all messages).

**Request:**
```http
PUT /api/conversations/conv_abc123 HTTP/1.1
Content-Type: application/json
Authorization: Bearer <session-token>

{
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "connection": "production",
    "messages": [...]
}
```

**Response:** Same as GET response with updated data.

### PATCH /api/conversations/{id}

Renames a conversation.

**Request:**
```http
PATCH /api/conversations/conv_abc123 HTTP/1.1
Content-Type: application/json
Authorization: Bearer <session-token>

{
    "title": "New conversation title"
}
```

**Response:**
```json
{
    "success": true
}
```

### DELETE /api/conversations/{id}

Deletes a specific conversation.

**Request:**
```http
DELETE /api/conversations/conv_abc123 HTTP/1.1
Authorization: Bearer <session-token>
```

**Response:**
```json
{
    "success": true
}
```

### DELETE /api/conversations?all=true

Deletes all conversations for the authenticated user.

**Request:**
```http
DELETE /api/conversations?all=true HTTP/1.1
Authorization: Bearer <session-token>
```

**Response:**
```json
{
    "success": true,
    "deleted": 15
}
```

**Implementation:**
[internal/conversations/](https://github.com/pgEdge/pgedge-postgres-mcp/tree/main/internal/conversations)

## LLM Proxy Endpoints

The LLM proxy provides REST API endpoints for chat functionality. See the
[LLM Proxy Guide](../advanced/llm-proxy.md) for detailed documentation on these endpoints:

- `GET /api/llm/providers` - List configured LLM providers
- `GET /api/llm/models?provider=<provider>` - List available models
- `POST /api/llm/chat` - Send chat request with tool support

## Schema Retrieval Examples

The `get_schema_info` tool is the primary method for
discovering database structure. All examples in this
section send requests to `POST /mcp/v1` with Bearer
token authentication.

### Retrieving Table Schema

In the following example, the `curl` command retrieves
schema information for a specific table:

```bash
curl -X POST http://localhost:8080/mcp/v1 \
    -H "Authorization: Bearer YOUR_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {
            "name": "get_schema_info",
            "arguments": {
                "schema_name": "public",
                "table_name": "users"
            }
        }
    }'
```

The server returns a JSON-RPC response with TSV content:

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "content": [
            {
                "type": "text",
                "text": "Database: mydb\n\nschema\ttable\t..."
            }
        ]
    }
}
```

### Parsing TSV Responses in Python

In the following example, the `parse_schema_tsv` function
converts the TSV response into structured data:

```python
def parse_schema_tsv(tsv_text):
    """Parse get_schema_info TSV into structured data."""
    lines = tsv_text.strip().split("\n")
    # Skip header lines (non-TSV content)
    data_lines = [
        l for l in lines if "\t" in l
    ]
    if not data_lines:
        return []

    headers = data_lines[0].split("\t")
    rows = []
    for line in data_lines[1:]:
        values = line.split("\t")
        rows.append(dict(zip(headers, values)))
    return rows

# Usage
result = client.call_tool(
    "get_schema_info",
    {"schema_name": "public"}
)
tables = parse_schema_tsv(result)
for col in tables:
    print(
        f"{col['table']}.{col['column']}: "
        f"{col['data_type']}"
    )
```

### Parsing TSV Responses in JavaScript

In the following example, the `parseSchemasTsv` function
converts the TSV response into an array of objects:

```javascript
function parseSchemasTsv(tsvText) {
    const lines = tsvText.trim().split("\n");
    const dataLines = lines.filter(
        l => l.includes("\t")
    );
    if (dataLines.length === 0) return [];

    const headers = dataLines[0].split("\t");
    return dataLines.slice(1).map(line => {
        const values = line.split("\t");
        const row = {};
        headers.forEach((h, i) => {
            row[h] = values[i] || "";
        });
        return row;
    });
}
```

### Schema Retrieval Errors

The server returns an error when a schema does not
exist or the user lacks permissions.

The following response indicates a missing schema:

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "content": [
            {
                "type": "text",
                "text": "No tables found in schema 'missing'"
            }
        ],
        "isError": true
    }
}
```

A permission error occurs when the database user cannot
access the requested schema:

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "content": [
            {
                "type": "text",
                "text": "permission denied for schema restricted"
            }
        ],
        "isError": true
    }
}
```

Grant the required permissions to resolve access errors:

```sql
GRANT USAGE ON SCHEMA restricted TO your_user;
GRANT SELECT ON ALL TABLES IN SCHEMA restricted
    TO your_user;
```

## Query Execution Examples

The `query_database` tool executes SQL queries in
read-only transactions. The LLM client translates
natural language queries to SQL before sending the
request. Claude Desktop, the web client, and the CLI
all support this translation.

### Executing a SQL Query

In the following example, the `curl` command executes
a SQL query against the selected database:

```bash
curl -X POST http://localhost:8080/mcp/v1 \
    -H "Authorization: Bearer YOUR_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {
            "name": "query_database",
            "arguments": {
                "query": "SELECT id, name, email FROM users LIMIT 5"
            }
        }
    }'
```

The server returns the query results in TSV format:

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "content": [
            {
                "type": "text",
                "text": "SQL Query: ...\n\nid\tname\n1\tAlice\n..."
            }
        ]
    }
}
```

### Error Handling with Python

In the following example, the `execute_query` function
includes retry logic and comprehensive error handling:

```python
import requests
import json
import time

def execute_query(base_url, token, query,
                  max_retries=3):
    """Execute a query with error handling and retry."""
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }
    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {
            "name": "query_database",
            "arguments": {"query": query}
        }
    }

    for attempt in range(max_retries):
        try:
            response = requests.post(
                f"{base_url}/mcp/v1",
                headers=headers,
                json=payload,
                timeout=30
            )

            if response.status_code == 401:
                raise AuthError("Token expired")

            if response.status_code == 429:
                wait = 2 ** attempt
                time.sleep(wait)
                continue

            response.raise_for_status()
            result = response.json()

            if "error" in result:
                raise QueryError(
                    result["error"]["message"]
                )

            return result["result"]["content"][0]["text"]

        except requests.exceptions.Timeout:
            if attempt < max_retries - 1:
                time.sleep(2 ** attempt)
                continue
            raise

    raise Exception("Max retries exceeded")
```

### Query Error Reference

The `query_database` tool returns specific errors for
common failure conditions. The following table lists
errors, their causes, and the recommended solutions:

| Error | Cause | Solution |
|---|---|---|
| `read-only transaction` | Write query | Set `allow_writes: true` |
| `relation does not exist` | Table missing | Check table and schema |
| `permission denied` | No grants | Grant SELECT to user |
| `syntax error` | Invalid SQL | Fix the query syntax |
| `query timeout` | Slow query | Add indexes or simplify |
| `connection refused` | DB offline | Check host and port |

### Result Format

The query response follows a consistent structure
for all results:

- The server returns all query results in TSV format.
- NULL values appear as empty strings in the output.
- The first line of data contains the column headers.
- Metadata lines precede the TSV data block.
- The server truncates results at the configured row
  limit; the default is 100 rows.

For additional integration examples, see the
[Client Examples](client-examples.md) page. For details
about available tools, see
[Tools Documentation](../reference/tools.md).

## See Also

- [LLM Proxy](../advanced/llm-proxy.md) - LLM proxy endpoints and usage
- [MCP Protocol](mcp-protocol.md) - MCP protocol specification
- [Tools Documentation](../reference/tools.md) - Available MCP tools
- [Resources Documentation](../reference/resources.md) - Available MCP resources
