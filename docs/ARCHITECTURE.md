# Project Architecture

## Directory Structure

```
pgedge-postgres-mcp/
├── cmd/
│   └── pgedge-postgres-mcp/           # Application entry point
│       └── main.go           # Main function, initializes and wires components
│
├── internal/                 # Private application code
│   ├── database/             # PostgreSQL integration
│   │   ├── types.go          # Database-related types
│   │   └── connection.go     # Connection management and metadata loading
│   │
│   ├── llm/                  # LLM integration
│   │   └── client.go         # Claude API client
│   │
│   ├── mcp/                  # MCP protocol implementation
│   │   ├── types.go          # MCP protocol types
│   │   └── server.go         # Protocol handler and stdio server
│   │
│   └── tools/                # MCP tool implementations
│       ├── registry.go       # Tool registration and execution
│       ├── query_database.go # Natural language query tool
│       └── get_schema_info.go # Schema information tool
│
├── docs/                     # Documentation
│   ├── README.md             # Full documentation
│   ├── TROUBLESHOOTING.md    # Troubleshooting guide
│   └── ARCHITECTURE.md       # This file
│
├── configs/                  # Configuration examples
│   ├── .env.example          # Environment variables
│   └── pgedge-postgres-mcp.yaml.example # Server configuration
│
├── bin/                      # Compiled binaries (gitignored)
│
├── go.mod                    # Go module definition
├── go.sum                    # Go module checksums
├── Makefile                  # Build automation
└── README.md                 # Quick start guide
```

## Component Overview

### cmd/pgedge-postgres-mcp
- **Purpose**: Application entry point
- **Responsibilities**:
  - Initialize database client
  - Initialize LLM client
  - Register tools
  - Start MCP server

### internal/database
- **Purpose**: PostgreSQL connection and metadata management
- **Key Components**:
  - `Client`: Manages connection pool and metadata
  - `TableInfo`, `ColumnInfo`: Metadata types
- **Features**:
  - Asynchronous metadata loading
  - Thread-safe metadata access
  - Automatic schema discovery from pg_catalog

### internal/llm
- **Purpose**: Claude AI integration for natural language processing
- **Key Components**:
  - `Client`: Manages API requests to Claude
- **Features**:
  - Natural language to SQL conversion
  - Configurable model selection
  - Error handling and response parsing

### internal/mcp
- **Purpose**: Model Context Protocol implementation
- **Key Components**:
  - `Server`: Handles stdio communication
  - Protocol types (Request, Response, Tool, etc.)
- **Features**:
  - JSON-RPC 2.0 protocol
  - Request routing
  - Protocol version negotiation

### internal/tools
- **Purpose**: MCP tool implementations
- **Key Components**:
  - `Registry`: Tool registration and execution
  - Individual tool implementations
- **Features**:
  - Extensible tool system
  - Easy to add new tools
  - Consistent error handling

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

2. **Register tool** in `cmd/pgedge-postgres-mcp/main.go`:
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
- `POSTGRES_CONNECTION_STRING`: Database connection (required)
- `ANTHROPIC_API_KEY`: Claude API key (required for queries)
- `ANTHROPIC_MODEL`: Claude model ID (optional)

### MCP Configuration
Configure in Claude Desktop's MCP config file:
```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/path/to/bin/pgedge-postgres-mcp",
      "env": {
        "POSTGRES_CONNECTION_STRING": "...",
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
export POSTGRES_CONNECTION_STRING="..."
export ANTHROPIC_API_KEY="..."

# Run server
./bin/pgedge-postgres-mcp

# Send test request
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/pgedge-postgres-mcp
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
