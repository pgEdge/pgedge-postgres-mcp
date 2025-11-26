# Natural Language Agent Web Client

Modern web-based client for the Natural Language Agent. This application provides a clean, intuitive interface to interact with your PostgreSQL database using natural language.

## Features

- **Authentication**: Secure login using MCP server's user authentication
- **AI-Powered Chat**: Agentic LLM interaction with PostgreSQL database using
  natural language
- **Tool Calling**: LLM can autonomously call MCP tools and resources to answer
  questions
- **Multi-LLM Support**: Works with Anthropic Claude, OpenAI GPT, or Ollama
  models
- **Multi-Database Support**: Switch between configured databases with
  per-user access control
- **System Monitoring**: Real-time PostgreSQL server status and database
  connection information
- **Conversation History**: Maintains conversation context for multi-turn
  interactions
- **Dark Mode**: Toggle between light and dark themes
- **Responsive Design**: Works on desktop and mobile devices

## Architecture

The web client uses a **two-tier architecture** communicating directly with the MCP server via JSON-RPC:

### Frontend (Port 5173 in dev, served via nginx in production)
- **Framework**: React 18 with Material-UI (MUI)
- **Build Tool**: Vite
- **State Management**: React Context API
- **Communication**: Direct JSON-RPC 2.0 calls to MCP server
- **Features**:
  - Chat interface with client-side agentic loop
  - Collapsible status banner with PostgreSQL system info
  - Authentication UI
  - Light/dark theme toggle
- **Testing**: Vitest + React Testing Library

### MCP Server (Port 8080)
- **Protocol**: JSON-RPC 2.0 over HTTP/HTTPS
- **Authentication**: Token-based (both service tokens and session tokens)
- **LLM Proxy**: Server-side LLM integration keeping API keys secure
- **Features**:
  - Database connection pooling with per-token isolation
  - Read-only query execution
  - Resource access (PostgreSQL system info, database schema)
  - Tool execution (query, schema info, similarity search, embeddings)
  - LLM proxy endpoints (`/api/llm/*`) for chat functionality

## Prerequisites

- Node.js 18.x or higher
- A running Natural Language Agent configured for HTTP mode with:
  - Authentication enabled
  - LLM proxy enabled (for chat functionality)
  - Database connection configured

## Quick Start

The easiest way to start the web client is using the provided startup script from the project root:

```bash
./start_web_client.sh
```

This script will:
- Start the MCP server in HTTP mode (port 8080) with authentication and LLM proxy enabled
- Start the Vite development server (port 5173) for the frontend
- Use pre-configured settings from `bin/pgedge-pg-mcp-web.yaml`
- Use existing user files from the `bin/` directory

The MCP server supports both authentication methods simultaneously:
- **Service Tokens**: Long-lived API tokens from `pgedge-nla-server-tokens.yaml` (for programmatic access)
- **User Authentication**: Username/password authentication via `authenticate_user` tool (used by web interface)

Both authentication methods create session tokens that provide per-token database connection isolation for security and resource management.

Then open [http://localhost:5173](http://localhost:5173) in your browser.

## Configuration

The web client is configured entirely through the MCP server's configuration file (`bin/pgedge-pg-mcp-web.yaml`). No separate web client configuration is needed.

### MCP Server Configuration

Edit `bin/pgedge-pg-mcp-web.yaml` to configure:

**Database Connection:**
```yaml
database:
    host: "localhost"
    port: 5432
    database: "your_database"
    user: "your_user"
    password: ""  # Leave empty to use .pgpass file
```

**HTTP Server:**
```yaml
http:
    enabled: true
    address: ":8080"
    auth:
        enabled: true
        token_file: "./pgedge-nla-server-tokens.yaml"
```

**LLM Proxy (for chat functionality):**
```yaml
llm:
    enabled: true
    provider: "anthropic"  # Options: "anthropic", "openai", "ollama"
    model: "claude-sonnet-4-5"

    # Option 1: Environment variables (RECOMMENDED)
    # Set PGEDGE_ANTHROPIC_API_KEY or PGEDGE_OPENAI_API_KEY

    # Option 2: API key files (RECOMMENDED for production)
    # anthropic_api_key_file: "~/.pgedge-anthropic-key"
    # openai_api_key_file: "~/.pgedge-openai-key"

    # Option 3: Direct API keys (NOT RECOMMENDED)
    # anthropic_api_key: ""
    # openai_api_key: ""
```

**User Authentication:**
```yaml
user_file: "./pgedge-nla-server-users.yaml"
```

See [MCP Server Configuration Documentation](../docs/configuration.md) for complete configuration options.

### LLM API Keys

The LLM API keys are configured **server-side** for security. The web client never sees or handles API keys directly. Configure API keys using one of these methods (in priority order):

1. **Environment Variables** (recommended for development):
   ```bash
   export PGEDGE_ANTHROPIC_API_KEY="your-key-here"
   # or
   export PGEDGE_OPENAI_API_KEY="your-key-here"
   ```

2. **API Key Files** (recommended for production):
   ```bash
   # Create key file
   echo "your-api-key" > ~/.pgedge-anthropic-key
   chmod 600 ~/.pgedge-anthropic-key

   # Configure in pgedge-pg-mcp-web.yaml
   # llm:
   #     anthropic_api_key_file: "~/.pgedge-anthropic-key"
   ```

3. **Direct in Config** (not recommended - only for local testing):
   ```yaml
   llm:
       anthropic_api_key: "your-key-here"
   ```

## Installation

```bash
cd web
npm install
```

## Development

Start the development server:

```bash
# Option 1: Use the startup script (recommended - starts both MCP server and Vite)
cd ..
./start_web_client.sh

# Option 2: Manual startup in separate terminals
# Terminal 1: Start MCP server
./bin/pgedge-nla-server --config bin/pgedge-pg-mcp-web.yaml

# Terminal 2: Start Vite dev server (frontend)
cd web
npm run dev
```

Then open [http://localhost:5173](http://localhost:5173) in your browser.

## Testing

Run the test suite:

```bash
# Run all tests
npm test

# Run tests in watch mode
npm run test:watch

# Run tests with coverage report
npm run test:coverage

# Run tests with UI
npm run test:ui
```

### Test Coverage

The test suite includes:

- **Frontend Component Tests**: React components tested with Testing Library
  - Login component (authentication flow, validation, error handling)
  - MainContent component (data fetching, loading states, display)
  - AuthContext (authentication state management)
  - Header component (navigation, user menu)
  - HelpPanel component (help documentation display)

Coverage reports are generated in the `coverage/` directory.

## Production Deployment

### Docker Compose (Recommended)

The easiest way to deploy in production is using Docker Compose:

```bash
# Configure environment variables in .env
cp .env.example .env
# Edit .env with your configuration

# Start all services
docker-compose up -d
```

This will start:
- MCP server (port 8080)
- Web client via nginx (port 80/443)

See [Docker Deployment Documentation](../docs/docker-deployment.md) for details.

### Manual Build

Build the application for production:

```bash
cd web
npm run build
```

The built static assets will be in the `dist/` directory. Serve these files using:
- nginx (recommended - see `docker/nginx.conf` for example configuration)
- Any static file server

Configure your web server to:
- Serve static files from `dist/`
- Proxy `/mcp/v1` and `/api/*` to the MCP server (port 8080)
- Enable HTTPS for production

Example nginx configuration:
```nginx
server {
    listen 80;
    server_name your-domain.com;

    # Serve static frontend files
    location / {
        root /path/to/web/dist;
        try_files $uri $uri/ /index.html;
    }

    # Proxy MCP server endpoints
    location /mcp/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## How It Works

### Authentication Flow
1. User enters username/password in web UI
2. Frontend calls MCP server's `authenticate_user` tool via JSON-RPC
3. MCP server validates credentials and returns a session token
4. Frontend stores session token in memory (AuthContext)
5. All subsequent requests include session token as Bearer token in Authorization header
6. MCP server uses token hash for per-token database connection isolation

### Agentic Chat Flow
1. User sends a message in the chat interface
2. Frontend fetches available MCP tools via `tools/list` JSON-RPC call
3. **Client-side agentic loop** runs in the browser (max 10 iterations):
   - Sends user message + conversation history to MCP server's `/api/llm/chat` endpoint with available tools
   - MCP server proxies request to LLM provider (API keys stay server-side)
   - If LLM requests tool use:
     - Frontend executes tool via MCP server's `tools/call` JSON-RPC
     - Adds tool results to conversation
     - Continues loop
   - If LLM provides final text response:
     - Returns response to user
     - Conversation history maintained client-side
4. Frontend displays LLM response in chat interface

**Key Difference from CLI**: The agentic loop (LLM → tool execution → LLM) runs **client-side** in the browser, while the CLI client runs it server-side. The MCP server provides the LLM proxy to keep API keys secure.

### Database Selection

When multiple databases are configured on the server, users can switch between
them:

1. **Database Selector**: Click the database icon (storage icon) in the green
   status banner to open the database selector popover
2. **Available Databases**: The list shows all databases the user has access
   to, based on the `available_to_users` configuration
3. **Current Database**: The currently selected database is highlighted with
   a checkmark
4. **Switch Database**: Click any database in the list to switch to it
5. **Connection Details**: Each database entry shows the connection string
   (user@host:port/database)

**Important Notes:**

- The database selector is only visible when multiple databases are configured
- Database switching is disabled while the LLM is processing a query to
  prevent data consistency issues
- Your database selection is saved and restored on subsequent sessions
- Access control is enforced server-side (see
  [Configuration Guide](../docs/configuration.md#multiple-database-management))

## Security

- All API calls to the MCP server require authentication
- Session tokens are stored client-side in memory only (cleared on page reload)
- LLM API keys are kept server-side and never sent to the browser
- In production, enable HTTPS for encrypted communication
- Per-token database connection isolation prevents cross-user data access
- Consider using secure, HTTP-only cookies for session management in production

## Troubleshooting

**"Cannot connect to server" or "Failed to fetch" errors:**
- Ensure MCP server is running on port 8080
- Check that LLM proxy is enabled in MCP server configuration (`llm.enabled: true`)
- Verify API keys are configured server-side (environment variables or key files)
- Check browser console for detailed error messages
- Review MCP server logs for errors

**"Your session has expired" or automatic logout:**
- Session tokens are stored in memory and cleared on page reload
- This is normal behavior - simply log in again
- In production, consider implementing persistent session storage

**System info shows "N/A":**
- Ensure MCP server is running and configured with database connection
- Check that authentication is working (you should be able to log in)
- Verify the MCP server has database connection configured in YAML
- Try hard refresh (Cmd+Shift+R on Mac, Ctrl+Shift+R on Windows/Linux)

**Cannot login:**
- Verify user exists in `pgedge-nla-server-users.yaml`
- Check MCP server is running in HTTP mode with authentication enabled
- Ensure MCP server is accessible at the configured URL
- Check browser console and MCP server logs for errors

**Chat not working / LLM errors:**
- Verify LLM proxy is enabled in MCP server configuration
- Check that API keys are configured server-side (see LLM API Keys section)
- Ensure MCP server has network access to LLM provider APIs
- Review MCP server logs for LLM-related errors
- Try different LLM provider/model in the chat interface dropdown

**Vite dev server won't start:**
- Check if port 5173 is already in use: `lsof -i :5173`
- Verify Node.js version is 18.x or higher: `node --version`
- Ensure dependencies are installed: `npm install`
- Clear node_modules and reinstall: `rm -rf node_modules && npm install`

**Tests failing:**
- Ensure all dependencies are installed: `npm install`
- Clear test cache: `npx vitest --clearCache`
- Check Node.js version is compatible
- Review test output for specific errors

## License

This software is released under The PostgreSQL License.

Portions copyright (c) 2025, pgEdge, Inc.
