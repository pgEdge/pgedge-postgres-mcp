# LLM Proxy

The MCP server includes an LLM proxy service that enables web clients to chat
with various LLM providers while keeping API keys secure on the server side.

This guide covers the LLM proxy architecture, endpoints, configuration, and how
to build client applications that use it.

## Overview

```
┌─────────────┐
│   Browser   │
│  (Port 8081)│
└──────┬──────┘
       │ 1. Fetch providers/models via /api/llm/*
       │ 2. Send chat via /api/llm/chat
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

**Key Benefits:**

- API keys never leave the server
- Centralized LLM provider management
- Client-side agentic loop with server-side LLM access
- Consistent authentication model

## LLM Proxy Endpoints

The LLM proxy provides three REST API endpoints:

### GET /api/llm/providers

Returns the list of configured LLM providers based on which API keys are
available.

**Request:**
```http
GET /api/llm/providers HTTP/1.1
Host: localhost:8080
Authorization: Bearer <session-token>
```

**Response:**
```json
{
  "providers": [
    {
      "name": "anthropic",
      "display": "Anthropic Claude",
      "isDefault": true
    },
    {
      "name": "openai",
      "display": "OpenAI",
      "isDefault": false
    },
    {
      "name": "ollama",
      "display": "Ollama",
      "isDefault": false
    }
  ],
  "defaultModel": "claude-sonnet-4-5"
}
```

**Implementation:** [internal/llmproxy/proxy.go:93-136](../internal/llmproxy/proxy.go#L93-L136)

### GET /api/llm/models?provider=<provider>

Lists available models for the specified provider.

**Request:**
```http
GET /api/llm/models?provider=ollama HTTP/1.1
Host: localhost:8080
Authorization: Bearer <session-token>
```

**Response:**
```json
{
  "models": [
    { "name": "llama3" },
    { "name": "mistral" },
    { "name": "codellama:13b" }
  ]
}
```

**Provider-specific behavior:**

- **Anthropic**: Returns static list of Claude models (no public API for model
  listing)
- **OpenAI**: Calls OpenAI's models API
- **Ollama**: Calls Ollama's `/api/tags` endpoint at configured
  PGEDGE_OLLAMA_URL

**Implementation:** [internal/llmproxy/proxy.go:138-200](../internal/llmproxy/proxy.go#L138-L200)

### POST /api/llm/chat

Sends a chat request to the configured LLM provider with tool support.

**Request:**
```json
{
  "messages": [
    {
      "role": "user",
      "content": "List the tables in the database"
    }
  ],
  "tools": [
    {
      "name": "list_tables",
      "description": "Lists all tables in the database",
      "inputSchema": {
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
      "id": "toolu_123",
      "name": "list_tables",
      "input": {}
    }
  ],
  "stop_reason": "tool_use"
}
```

**Implementation:** [internal/llmproxy/proxy.go:202-295](../internal/llmproxy/proxy.go#L202-L295)

## Configuration

The LLM proxy is configured via environment variables and YAML config:

```yaml
# Configuration file: pgedge-pg-mcp-web.yaml
llm:
    enabled: true
    provider: "anthropic"  # anthropic, openai, or ollama
    model: "claude-sonnet-4-5"

    # API keys (use env vars or files for production)
    anthropic_api_key_file: "~/.anthropic-key"
    openai_api_key_file: "~/.openai-key"

    # Ollama configuration
    ollama_url: "http://localhost:11434"

    # Generation parameters
    max_tokens: 4096
    temperature: 0.7
```

**Environment variables:**

- `PGEDGE_LLM_ENABLED`: Enable/disable LLM proxy (default: true)
- `PGEDGE_LLM_PROVIDER`: Default provider
- `PGEDGE_LLM_MODEL`: Default model
- `PGEDGE_ANTHROPIC_API_KEY` or `ANTHROPIC_API_KEY`: Anthropic API key
- `PGEDGE_OPENAI_API_KEY` or `OPENAI_API_KEY`: OpenAI API key
- `PGEDGE_OLLAMA_URL`: Ollama server URL (used for both embeddings and LLM)
- `PGEDGE_LLM_MAX_TOKENS`: Maximum tokens per response
- `PGEDGE_LLM_TEMPERATURE`: LLM temperature (0.0-1.0)

**Implementation:** [internal/config/config.go:459-489](../internal/config/config.go#L459-L489)

## Building Web Clients with JSON-RPC

The web client communicates directly with the MCP server via JSON-RPC 2.0 over
HTTP, matching the CLI client architecture.

### MCP Client Implementation

**File:** [web/src/lib/mcp-client.js](../web/src/lib/mcp-client.js)

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

The web client implements the agentic loop in React, calling MCP tools via
JSON-RPC:

```javascript
// web/src/components/ChatInterface.jsx

const processQuery = async (userMessage) => {
    let conversationHistory = [...messages, { role: 'user', content: userMessage }];

    // Agentic loop (max 10 iterations)
    for (let iteration = 0; iteration < 10; iteration++) {
        // Get LLM response via proxy
        const response = await fetch('/api/llm/chat', {
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

## See Also

- [API Reference](api-reference.md) - Complete API endpoint documentation
- [MCP Protocol](mcp-protocol.md) - MCP protocol specification
- [Architecture](architecture.md) - System architecture overview
- [Configuration](configuration.md) - Server configuration options
