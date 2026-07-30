# Using LLM Proxy

The MCP server includes an LLM proxy service that enables web clients to chat with various LLM providers while keeping API keys secure on the server side. This guide covers the LLM proxy architecture, endpoints, configuration, and how to build client applications that use it.

The following diagram illustrates the LLM proxy architecture and how clients interact with the MCP server to access LLM providers.

```
┌─────────────┐
│   Browser   │
│  (Port 8081)│
└──────┬──────┘
       │ 1. Fetch providers/models via /api/llm/v1/*
       │ 2. Send chat via /api/llm/v1/chat/stream
       ▼
┌────────────────┐
│  Web Client    │ (nginx + React)
│   Container    │
└────────┬───────┘
         │ Proxy to MCP server
         ▼
┌────────────────┐     ┌──────────────┐
│  MCP Server    │────▶│ Anthropic    │
│  (Port 8080)   │     │ OpenAI       │
│  - JSON-RPC    │────▶│ Ollama       │
│  - LLM Proxy   │     └──────────────┘
│  - Auth        │
└────────────────┘

```

The LLM proxy provides the following key benefits:

- API keys never leave the server.
- Centralized LLM provider management.
- Client-side agentic loop with server-side LLM access.
- Consistent authentication model.

## LLM Proxy Endpoints

The proxy is provided by the
[pgedge-go-llm-lib](https://github.com/pgEdge/pgedge-go-llm-lib)
`llm/proxy` package, which the server mounts at `/api/llm/`. The
library registers its own routes beneath a version segment, so every
endpoint sits under `/api/llm/v1/`. The following five endpoints make
up the documented surface, and each requires a session token unless
authentication is disabled.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/llm/v1/health` | Report reachability of each configured provider |
| GET | `/api/llm/v1/providers` | List configured providers |
| GET | `/api/llm/v1/models` | List the models a provider offers |
| POST | `/api/llm/v1/chat` | Send a chat request and wait for the full response |
| POST | `/api/llm/v1/chat/stream` | Send a chat request and receive the response as it is generated |

The library also serves `/api/llm/v1/embed`,
`/api/llm/v1/embed/multimodal` and `/api/llm/v1/rerank` through the
same handler. These are reachable but are not part of the surface this
server documents or advertises in its OpenAPI specification; use the
`generate_embedding` tool for embedding work instead.

!!! note

    Earlier releases served `/api/llm/providers`, `/api/llm/models` and
    `/api/llm/chat` without the version segment. Those paths now return
    404\. Callers written against them need the `/v1` segment added, and
    the chat request and response bodies use typed content blocks; see
    the [changelog](../changelog.md) for the details of that move.

### GET /api/llm/v1/health

Reports whether each configured provider is reachable. The server
builds a client for every configured provider and pings it with a
five-second deadline, so a hanging provider cannot stall the check.

The response `status` is `ok` when every provider answers, and
`degraded` when at least one does not; a degraded response also carries
the HTTP status `503 Service Unavailable`, which makes the endpoint
usable as a load balancer health check for LLM availability. Note that
this is separate from the server's own `/health` endpoint, which
reports on the server rather than on its providers.

In the following example, the request checks provider health.

**Request:**
```http
GET /api/llm/v1/health HTTP/1.1
Host: localhost:8080
Authorization: Bearer <session-token>
```

**Response:**
```json
{
  "status": "degraded",
  "providers": {
    "anthropic": {
      "status": "ok"
    },
    "ollama": {
      "status": "down",
      "error": "dial tcp 127.0.0.1:11434: connect: connection refused"
    }
  }
}
```

### GET /api/llm/v1/providers

Returns the list of configured LLM providers based on which API keys are available.

In the following example, the request retrieves the list of available LLM providers.

**Request:**
```http
GET /api/llm/v1/providers HTTP/1.1
Host: localhost:8080
Authorization: Bearer <session-token>
```

**Response:**
```json
{
  "providers": [
    {
      "name": "anthropic",
      "display_name": "Anthropic",
      "model": "claude-sonnet-4-5",
      "default": true
    },
    {
      "name": "openai",
      "display_name": "OpenAI"
    },
    {
      "name": "ollama",
      "display_name": "Ollama"
    }
  ],
  "default_provider": "anthropic"
}
```

The `default` and `model` fields are omitted rather than sent as false
or empty, so a provider that is neither the default nor carrying a
configured model appears with just its name and display name.

### GET /api/llm/v1/models?provider=<provider>

Lists available models for the specified provider.

In the following example, the request retrieves the list of available models for the Ollama provider.

**Request:**
```http
GET /api/llm/v1/models?provider=ollama HTTP/1.1
Host: localhost:8080
Authorization: Bearer <session-token>
```

**Response:**
```json
{
  "models": [
    "llama3",
    "mistral",
    "codellama:13b"
  ]
}
```

Adding `&metadata=true` to the request returns objects carrying each
model's metadata in place of the plain identifiers.

**Provider-specific behavior:**

- **Anthropic**: Returns a static list of Claude models (no public API for model listing).
- **OpenAI**: Calls the OpenAI models API.
- **Ollama**: Calls the Ollama `/api/tags` endpoint at the configured PGEDGE_OLLAMA_URL.

### POST /api/llm/v1/chat

Sends a chat request to the configured LLM provider with tool support,
and returns the complete response once the provider has finished
generating it.

Message content is a list of typed blocks rather than a bare string;
the block `type` selects which payload field applies, so a text block
carries `text` and a tool call carries `tool_use`. The recognised types
are `text`, `image`, `document`, `tool_use` and `tool_result`.

In the following example, the request sends a chat message with tools to the LLM provider.

**Request:**

```json
{
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "List the tables in the database"
        }
      ]
    }
  ],
  "tools": [
    {
      "name": "list_tables",
      "description": "Lists all tables in the database",
      "input_schema": {
        "type": "object",
        "properties": {}
      }
    }
  ],
  "provider": "anthropic",
  "model": "claude-sonnet-4-5"
}
```

**Response:**

```json
{
  "content": [
    {
      "type": "tool_use",
      "tool_use": {
        "id": "toolu_123",
        "name": "list_tables",
        "input": {}
      }
    }
  ],
  "stop_reason": "tool_use",
  "usage": {
    "prompt_tokens": 1284,
    "completion_tokens": 47,
    "total_tokens": 1331
  }
}
```

The `stop_reason` is one of `end_turn`, `max_tokens`, `stop_sequence`,
`tool_use`, `content_filter` or `error`. A client running an agentic
loop watches for `tool_use`, executes the requested tool, and sends the
result back as a `tool_result` block whose `tool_use_id` matches the
`id` above.

### POST /api/llm/v1/chat/stream

Sends the same request body as `/api/llm/v1/chat`, but returns the
response as [Server-Sent
Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
so a client can render text as it is generated. The web chat interface
uses this endpoint; the non-streaming endpoint remains available for
callers that would rather wait for a complete response.

Each event carries one JSON-encoded chunk whose `type` describes what it
contains:

- The `text` type carries an incremental text fragment in `text`;
  concatenating these fragments reconstructs the reply.
- The `tool_use_start` type announces a tool call, populating
  `tool_use.id` and `tool_use.name`. Some providers send the arguments
  in one piece here, whilst others follow with deltas.
- The `tool_use_delta` type carries a fragment of the current tool
  call's JSON arguments in `partial`; a client concatenates these
  fragments to rebuild the argument object.

Ordinary chunks arrive as `data:` lines on their own. The final chunk
arrives as an event named `done`, which normally carries the token
`usage` for the request; a stream that fails part way through emits an
event named `error` instead. A client always sees a `done` event, since
the proxy synthesises one when a provider closes the stream without
sending its own, in which case the `usage` field is absent.

In the following example, the response streams a short reply and then
closes with usage totals.

**Request:**
```http
POST /api/llm/v1/chat/stream HTTP/1.1
Host: localhost:8080
Authorization: Bearer <session-token>
Content-Type: application/json
```

**Response:**
```text
data: {"type":"text","text":"The database contains"}

data: {"type":"text","text":" three tables."}

event: done
data: {"type":"done","usage":{"prompt_tokens":1284,"completion_tokens":12,"total_tokens":1296}}

```

Because an error can arrive before, during, or after the content
chunks, a client should treat the `error` event as authoritative
regardless of how much it has already rendered.

## Configuring the LLM Proxy

The LLM proxy is configured via environment variables and YAML config.

In the following example, the configuration file specifies the LLM provider, model, and API key settings.

```yaml
# Configuration file: pgedge-pg-mcp-web.yaml
llm:
    enabled: true
    provider: "anthropic"  # anthropic, openai, gemini, or ollama
    model: "claude-sonnet-4-5"

    # API key configuration (priority: env vars > key files > direct values)
    # Option 1: Environment variables (recommended for development)
    # Option 2: API key files (recommended for production)
    anthropic_api_key_file: "~/.anthropic-api-key"
    openai_api_key_file: "~/.openai-api-key"
    gemini_api_key_file: "~/.gemini-api-key"
    # Option 3: Direct values (not recommended - use env vars or files)
    # anthropic_api_key: "your-key-here"
    # openai_api_key: "your-key-here"
    # gemini_api_key: "your-key-here"

    # Optional: Custom base URLs for API proxies
    # Leave empty to use default provider URLs
    # anthropic_base_url: "https://your-proxy.example.com"
    # openai_base_url: "https://your-proxy.example.com"

    # Ollama configuration
    ollama_url: "http://localhost:11434"

    # Generation parameters
    max_tokens: 4096
    temperature: 0.7
```

The proxy is disabled unless `enabled` (or `PGEDGE_LLM_ENABLED`) is
set to `true`; at least one provider's API key, or a reachable
`ollama_url`, must also be configured, or the server refuses to
start with the proxy enabled.

**API Key Priority:**

API keys are loaded in the following order (highest to lowest):

1. Environment variables (`PGEDGE_ANTHROPIC_API_KEY`,
   `PGEDGE_OPENAI_API_KEY`, `PGEDGE_GEMINI_API_KEY`).
2. API key files (`anthropic_api_key_file`, `openai_api_key_file`,
   `gemini_api_key_file`).
3. Direct configuration values (not recommended).

**Environment variables:**

- `PGEDGE_LLM_ENABLED`: Enable/disable the LLM proxy (default: false).
- `PGEDGE_LLM_PROVIDER`: The default provider.
- `PGEDGE_LLM_MODEL`: The default model.
- `PGEDGE_ANTHROPIC_API_KEY` or `ANTHROPIC_API_KEY`: The Anthropic API key.
- `PGEDGE_ANTHROPIC_BASE_URL`: Custom Anthropic API base URL (for proxies).
- `PGEDGE_OPENAI_API_KEY` or `OPENAI_API_KEY`: The OpenAI API key.
- `PGEDGE_OPENAI_BASE_URL`: Custom OpenAI API base URL (for proxies).
- `PGEDGE_GEMINI_API_KEY` or `GEMINI_API_KEY`: The Google Gemini API key.
- `PGEDGE_OLLAMA_URL`: The Ollama server URL (used for both embeddings and LLM).
- `PGEDGE_LLM_MAX_TOKENS`: The maximum tokens per response.
- `PGEDGE_LLM_TEMPERATURE`: The LLM temperature (0.0-1.0).

**Implementation:** [internal/config/config.go:497-513](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/internal/config/config.go#L497-L513)

## Building Web Clients with JSON-RPC

The web client communicates directly with the MCP server via JSON-RPC 2.0 over HTTP, matching the CLI client architecture.

### Natural Language Agent Client Implementation

In the following example, the MCPClient class implements JSON-RPC 2.0 communication with the MCP server.

**File:** [web/src/lib/mcp-client.js](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/web/src/lib/mcp-client.js)

```javascript
export class MCPClient {
    constructor(baseURL, token) {
        this.baseURL = baseURL;
        this.token = token;
        this.requestID = 0;
    }

    async sendRequest(method, params = null) {
        this.requestID++;

        const request = {
            jsonrpc: '2.0',
            id: this.requestID,
            method: method,
            params: params || {}
        };

        const response = await fetch(this.baseURL, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                ...(this.token && { 'Authorization': `Bearer ${this.token}` })
            },
            body: JSON.stringify(request)
        });

        const jsonResp = await response.json();

        if (jsonResp.error) {
            throw new Error(`RPC error ${jsonResp.error.code}: ${jsonResp.error.message}`);
        }

        return jsonResp.result;
    }
}
```

### Authentication Flow

In the following example, the authentication flow uses the `authenticate_user` tool to obtain a session token.

**Authentication via `authenticate_user` Tool:**

```javascript
// web/src/contexts/AuthContext.jsx

const login = async (username, password) => {
    // Call authenticate_user tool via JSON-RPC
    const authResult = await MCPClient.authenticate(MCP_SERVER_URL, username, password);

    // Store session token
    setSessionToken(authResult.sessionToken);
    localStorage.setItem('mcp-session-token', authResult.sessionToken);

    // Fetch user info from server
    const response = await fetch('/api/user/info', {
        headers: { 'Authorization': `Bearer ${authResult.sessionToken}` }
    });

    const userInfo = await response.json();
    setUser({ authenticated: true, username: userInfo.username });
};
```

**Session Validation:**

In the following example, the session validation checks the token by calling the MCP server.

```javascript
const checkAuth = async () => {
    // Validate session by calling MCP server
    const client = new MCPClient(MCP_SERVER_URL, sessionToken);
    await client.initialize();
    await client.listTools();

    // Fetch user details
    const response = await fetch('/api/user/info', {
        headers: { 'Authorization': `Bearer ${sessionToken}` }
    });

    const userInfo = await response.json();
    setUser({ authenticated: true, username: userInfo.username });
};
```

### Client-Side Agentic Loop

The web client implements the agentic loop in React, calling MCP tools via JSON-RPC.

In the following example, the agentic loop processes user queries by iteratively calling the LLM and executing tools.

```javascript
// web/src/components/ChatInterface.jsx

const processQuery = async (userMessage) => {
    let conversationHistory = [...messages, { role: 'user', content: userMessage }];

    // Agentic loop (max 10 iterations)
    for (let iteration = 0; iteration < 10; iteration++) {
        // Get LLM response via proxy
        const response = await fetch('/api/llm/v1/chat', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${sessionToken}`
            },
            body: JSON.stringify({
                messages: conversationHistory,
                tools: tools,
                provider: selectedProvider,
                model: selectedModel
            })
        });

        const llmResponse = await response.json();

        if (llmResponse.stop_reason === 'tool_use') {
            // Extract tool uses
            const toolUses = llmResponse.content.filter(item => item.type === 'tool_use');

            // Add assistant message with tool uses
            conversationHistory.push({
                role: 'assistant',
                content: llmResponse.content
            });

            // Execute tools via MCP JSON-RPC
            const toolResults = [];
            for (const toolUse of toolUses) {
                const result = await mcpClient.callTool(toolUse.name, toolUse.input);
                toolResults.push({
                    type: 'tool_result',
                    tool_use_id: toolUse.id,
                    content: result.content,
                    is_error: result.isError
                });
            }

            // Add tool results
            conversationHistory.push({
                role: 'user',
                content: toolResults
            });

            // Continue loop
            continue;
        }

        // Got final response - display and exit
        setMessages(conversationHistory);
        break;
    }
};
```