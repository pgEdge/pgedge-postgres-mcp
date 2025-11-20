# Project Architecture

## Directory Structure

```
pgedge-postgres-mcp/
├── cmd/
│   ├── pgedge-pg-mcp-svr/    # MCP server entry point
│   │   └── main.go           # HTTP/stdio server, routes, handlers
│   └── pgedge-pg-mcp-cli/    # Chat CLI client entry point
│       └── main.go           # Interactive chat client
│
├── internal/                 # Private application code
│   ├── auth/                 # Authentication
│   │   ├── token.go          # API token management
│   │   └── middleware.go     # HTTP auth middleware
│   │
│   ├── chat/                 # Chat client implementation
│   │   ├── client.go         # Main chat client logic
│   │   ├── llm.go            # LLM client abstraction
│   │   ├── mcp_client.go     # MCP JSON-RPC client
│   │   └── commands.go       # Slash commands
│   │
│   ├── config/               # Configuration management
│   │   └── config.go         # YAML config and env vars
│   │
│   ├── database/             # PostgreSQL integration
│   │   ├── connection.go     # Per-session connection pools
│   │   └── types.go          # Database-related types
│   │
│   ├── embedding/            # Embedding generation
│   │   ├── anthropic.go      # Voyage embeddings
│   │   ├── openai.go         # OpenAI embeddings
│   │   └── ollama.go         # Ollama embeddings
│   │
│   ├── llmproxy/             # LLM proxy for web client
│   │   └── proxy.go          # Provider/model/chat endpoints
│   │
│   ├── mcp/                  # MCP protocol implementation
│   │   ├── types.go          # MCP protocol types
│   │   └── server.go         # JSON-RPC handler
│   │
│   ├── tools/                # MCP tool implementations
│   │   ├── registry.go       # Tool registration
│   │   ├── query_*.go        # Database query tools
│   │   ├── schema_*.go       # Schema info tools
│   │   └── auth_*.go         # Authentication tools
│   │
│   └── users/                # User management
│       └── users.go          # User auth and sessions
│
├── web/                      # Web client (React)
│   ├── src/
│   │   ├── components/       # React components
│   │   │   ├── ChatInterface.jsx
│   │   │   ├── Login.jsx
│   │   │   └── ...
│   │   ├── contexts/         # React contexts
│   │   │   └── AuthContext.jsx
│   │   ├── lib/              # Client libraries
│   │   │   └── mcp-client.js # JSON-RPC client
│   │   └── App.jsx           # Main app component
│   ├── package.json
│   └── vite.config.js        # Vite configuration
│
├── docker/                   # Docker configuration
│   ├── Dockerfile.server     # MCP server container
│   ├── Dockerfile.web        # Web client container
│   ├── Dockerfile.cli        # CLI client container
│   ├── docker-compose.yml    # Multi-container deployment
│   ├── nginx.conf            # nginx proxy config
│   └── init-server.sh        # Server init script
│
├── docs/                     # Documentation
│   ├── index.md              # Getting started
│   ├── architecture.md       # This file
│   ├── api-reference.md      # API endpoint documentation
│   ├── llm-proxy.md          # LLM proxy guide
│   ├── internal-architecture.md # Technical deep-dive
│   └── ...                   # Other docs
│
├── bin/                      # Compiled binaries (gitignored)
├── go.mod                    # Go module definition
├── Makefile                  # Build automation
└── README.md                 # Quick start guide
```

## System Overview

The pgEdge Postgres MCP consists of three main components:

1. **MCP Server**: Go-based server providing MCP protocol, LLM proxy, and
   database tools
2. **Web Client**: React SPA with client-side agentic loop
3. **CLI Client**: Go-based interactive command-line chat interface

### Deployment Architecture

```
┌─────────────┐
│   Browser   │
└──────┬──────┘
       │ HTTP
       ▼
┌────────────────┐  nginx proxies  ┌────────────────┐
│  web-client    │─────────────────▶│  mcp-server    │
│  (nginx+React) │  /mcp/v1, /api   │  (Go binary)   │
│  Port 8081     │                  │  Port 8080     │
└────────────────┘                  └────────┬───────┘
                                             │
                    ┌────────────────────────┼────────────────┐
                    │                        │                │
                    ▼                        ▼                ▼
              ┌──────────┐            ┌──────────┐     ┌──────────┐
              │PostgreSQL│            │Anthropic │     │  Ollama  │
              │          │            │ OpenAI   │     │(optional)│
              └──────────┘            └──────────┘     └──────────┘
```

## Component Overview

### cmd/pgedge-pg-mcp-svr (MCP Server)

- **Purpose**: Main MCP server providing tools, authentication, and LLM proxy
- **Transport Modes**:
    - stdio mode: For Claude Desktop integration
    - HTTP mode: For web client, CLI client, and API access

- **Responsibilities**:
    - Initialize database connection manager
    - Initialize embedding providers
    - Register MCP tools
    - Serve JSON-RPC 2.0 protocol
    - Provide LLM proxy endpoints for web client
    - Handle user authentication and sessions

### cmd/pgedge-pg-mcp-cli (CLI Client)

- **Purpose**: Interactive command-line chat interface
- **Features**:
    - Client-side agentic loop
    - Direct LLM integration (Anthropic, OpenAI, Ollama)
    - Connects to MCP server via stdio or HTTP
    - Slash commands (/help, /models, /tables, etc.)
    - Conversation history management

### web/ (Web Client)

- **Purpose**: Browser-based React chat interface
- **Architecture**: Single-page application (SPA) built with React and
  Material-UI
- **Communication**:
    - JSON-RPC 2.0 to MCP server for tools (`/mcp/v1`)
    - REST API to MCP server for LLM proxy (`/api/llm/*`)
    - REST API for user info (`/api/user/info`)

- **Features**:
    - Client-side agentic loop (matches CLI architecture)
    - Session-based authentication
    - Theme switching (light/dark mode)
    - Real-time tool execution display
    - Conversation history in React state

### internal/llmproxy (LLM Proxy)

- **Purpose**: Proxy LLM requests from web client to keep API keys secure
- **Endpoints**:
    - `GET /api/llm/providers` - List configured providers
    - `GET /api/llm/models?provider=<name>` - List models for provider
    - `POST /api/llm/chat` - Proxy chat requests to LLM

- **Supported Providers**:
    - Anthropic Claude (via Messages API)
    - OpenAI GPT (via Chat Completions API)
    - Ollama (local models via HTTP API)

- **Benefits**:
    - API keys never exposed to browser
    - Centralized provider configuration
    - Consistent authentication model

### internal/chat (Chat Client Library)

- **Purpose**: Shared chat client logic for CLI and web
- **Key Components**:
    - `LLMClient` interface: Abstraction over LLM providers
    - `MCPClient`: JSON-RPC 2.0 client for MCP server
    - Agentic loop implementation
    - Message history management

### internal/database

- **Purpose**: PostgreSQL connection and metadata management
- **Key Features**:
    - Per-session connection pools (one pool per session token)
    - Connection isolation between users
    - Async metadata loading (schema, tables, columns)
    - Thread-safe metadata access
    - Automatic schema discovery from `pg_catalog`

### internal/embedding

- **Purpose**: Text embedding generation for semantic search
- **Supported Providers**:
    - Anthropic Voyage embeddings
    - OpenAI embeddings
    - Ollama local embeddings

### internal/mcp

- **Purpose**: Model Context Protocol (JSON-RPC 2.0) implementation
- **Key Components**:
    - JSON-RPC request/response handling
    - Protocol version negotiation
    - Method routing (`initialize`, `tools/list`, `tools/call`, etc.)

- **Transport Support**:
    - stdio (for Claude Desktop)
    - HTTP (for web client, CLI, and API access)

### internal/tools

- **Purpose**: MCP tool implementations
- **Available Tools**:
    - `query_database`: Natural language database queries
    - `list_tables`: List all database tables
    - `describe_table`: Get table schema
    - `execute_sql`: Execute raw SQL
    - `search_tables_semantic`: Semantic search over tables
    - `authenticate_user`: User login (returns session token)
    - And more...

- **Features**:
    - Extensible tool registry
    - Consistent error handling
    - Tool-specific database access

### internal/users

- **Purpose**: User authentication and session management
- **Features**:
    - Username/password authentication
    - Session token generation and validation
    - Token expiration management
    - YAML-based user storage

## Data Flow

### Initialization Flow
```
main()
  ├─> Create database client
  ├─> Create LLM client
  ├─> Start background task:
  │     ├─> Connect to database
  │     └─> Load metadata
  ├─> Create tool registry
  ├─> Register tools
  └─> Start MCP server
```

### Query Execution Flow
```
Client sends query request
  ↓
MCP Server receives JSON-RPC
  ↓
Server routes to tools/call
  ↓
Registry executes tool
  ↓
Tool handler:
  ├─> Check metadata loaded
  ├─> Generate schema context
  ├─> Call LLM to convert NL→SQL
  ├─> Execute SQL on database
  └─> Format and return results
  ↓
MCP Server sends JSON-RPC response
  ↓
Client receives results
```

## Adding New Tools

To add a new MCP tool:

1. **Create tool file** in `internal/tools/`:
   ```go
   // internal/tools/my_new_tool.go
   package tools

   import "pgedge-postgres-mcp/internal/mcp"

   func MyNewTool(/* dependencies */) Tool {
       return Tool{
           Definition: mcp.Tool{
               Name:        "my_new_tool",
               Description: "Description of what the tool does",
               InputSchema: mcp.InputSchema{
                   Type: "object",
                   Properties: map[string]interface{}{
                       "param1": map[string]interface{}{
                           "type":        "string",
                           "description": "Parameter description",
                       },
                   },
                   Required: []string{"param1"},
               },
           },
           Handler: func(args map[string]interface{}) (mcp.ToolResponse, error) {
               // Tool implementation
               return mcp.ToolResponse{
                   Content: []mcp.ContentItem{
                       {Type: "text", Text: "Result"},
                   },
               }, nil
           },
       }
   }
   ```

2. **Register tool** in `cmd/pgedge-pg-mcp-svr/main.go`:
   ```go
   registry.Register("my_new_tool", tools.MyNewTool(dbClient, llmClient))
   ```

3. **Rebuild**:
   ```bash
   make build
   ```

## Adding Resources (Future)

To add MCP resources support:

1. Create `internal/resources/` package
2. Define resource types and handlers
3. Add resources capability to `mcp.Server`
4. Register resources in main

## Adding Prompts (Future)

To add MCP prompts support:

1. Create `internal/prompts/` package
2. Define prompt templates and handlers
3. Add prompts capability to `mcp.Server`
4. Register prompts in main

## Configuration

### Environment Variables
- `PGEDGE_POSTGRES_CONNECTION_STRING`: Database connection (required)
- `ANTHROPIC_API_KEY`: Claude API key (required for queries)
- `ANTHROPIC_MODEL`: Claude model ID (optional)

### MCP Configuration
Configure in Claude Desktop's MCP config file:
```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/path/to/bin/pgedge-pg-mcp-svr",
      "env": {
        "PGEDGE_POSTGRES_CONNECTION_STRING": "...",
        "ANTHROPIC_API_KEY": "..."
      }
    }
  }
}
```

## Design Principles

1. **Modularity**: Each package has a single, well-defined responsibility
2. **Extensibility**: Easy to add new tools without modifying existing code
3. **Thread Safety**: Concurrent access is properly synchronized
4. **Async Init**: Server starts immediately; database loads in background
5. **Standard Layout**: Follows Go project layout conventions
6. **Type Safety**: Strong typing throughout the codebase

## Testing Strategy

### Unit Tests
```bash
make test
```

### Manual Testing
```bash
# Set environment
export PGEDGE_POSTGRES_CONNECTION_STRING="..."
export ANTHROPIC_API_KEY="..."

# Run server
./bin/pgedge-pg-mcp-svr

# Send test request
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/pgedge-pg-mcp-svr
```

## Dependencies

- **github.com/jackc/pgx/v5**: PostgreSQL driver
- Standard library for HTTP, JSON, I/O

## Future Enhancements

1. **Resources**: Add support for database schema as MCP resources
2. **Prompts**: Add prompt templates for common query patterns
3. **Caching**: Cache schema metadata with invalidation
4. **Metrics**: Add Prometheus metrics for monitoring
5. **Config Files**: Support YAML/TOML configuration files
6. **Multiple Databases**: Support multiple database connections
7. **Query History**: Track and cache recent queries
8. **Query Validation**: Validate SQL before execution
9. **Rate Limiting**: Protect against excessive LLM API usage
10. **Logging**: Structured logging with levels
