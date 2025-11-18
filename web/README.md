# pgEdge MCP Client

Web-based client for pgEdge MCP Server. This application provides a clean, modern interface to connect to and monitor your MCP server.

## Features

- **Authentication**: Secure login using MCP server's user authentication
- **AI-Powered Chat**: Agentic LLM interaction with PostgreSQL database using natural language
- **Tool Calling**: LLM can autonomously call MCP tools and resources to answer questions
- **Multi-LLM Support**: Works with Anthropic Claude, OpenAI GPT, or Ollama models
- **System Monitoring**: Real-time PostgreSQL server status and database connection information
- **Conversation History**: Maintains conversation context for multi-turn interactions
- **Dark Mode**: Toggle between light and dark themes
- **Responsive Design**: Works on desktop and mobile devices

## Prerequisites

- Node.js 18.x or higher
- A running pgEdge MCP Server (HTTP mode with authentication enabled)

## Configuration

Edit `config.json` to configure your MCP server connection and LLM provider:

```json
{
  "mcpServer": {
    "url": "http://localhost:8080/mcp/v1",
    "name": "Default MCP Server"
  },
  "session": {
    "secret": "change-this-to-a-random-secret-in-production",
    "maxAge": 86400000
  },
  "server": {
    "port": 3001
  },
  "llm": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-5",
    "maxTokens": 4096,
    "temperature": 0.7,
    "anthropicAPIKeyFile": "~/.pgedge-pg-mcp-web-anthropic-key",
    "openaiAPIKeyFile": "~/.pgedge-pg-mcp-web-openai-key",
    "ollamaURL": "http://localhost:11434"
  }
}
```

**Important**: Change the `session.secret` to a random string in production.

### LLM Configuration

The web client supports three LLM providers:

- **Anthropic Claude**: Set `provider` to `"anthropic"` and configure your API key
- **OpenAI GPT**: Set `provider` to `"openai"` and configure your API key
- **Ollama**: Set `provider` to `"ollama"` and configure the Ollama server URL

#### API Key Configuration

**IMPORTANT**: You must configure an API key before starting the server, or it will fail to start.

API keys should be stored in separate files for security. Create key files in your home directory:

```bash
# For Anthropic Claude (required if using anthropic provider)
echo "your-anthropic-api-key" > ~/.pgedge-pg-mcp-web-anthropic-key
chmod 600 ~/.pgedge-pg-mcp-web-anthropic-key

# For OpenAI (required if using openai provider)
echo "your-openai-api-key" > ~/.pgedge-pg-mcp-web-openai-key
chmod 600 ~/.pgedge-pg-mcp-web-openai-key
```

The paths in `config.json` can use `~` for the home directory:
- `anthropicAPIKeyFile`: Path to file containing Anthropic API key (default: `~/.pgedge-pg-mcp-web-anthropic-key`)
- `openaiAPIKeyFile`: Path to file containing OpenAI API key (default: `~/.pgedge-pg-mcp-web-openai-key`)

#### Environment Variables

You can also configure the LLM using environment variables (which take precedence over key files):

```bash
# LLM Provider (anthropic, openai, or ollama)
export PGEDGE_LLM_PROVIDER=anthropic

# LLM Model
export PGEDGE_LLM_MODEL=claude-sonnet-4-5

# API Keys (can use either PGEDGE_* or standard env vars)
# These take precedence over key files
export ANTHROPIC_API_KEY=your-key-here
export OPENAI_API_KEY=your-key-here

# Ollama Configuration
export PGEDGE_OLLAMA_URL=http://localhost:11434

# Optional: Adjust generation parameters
export PGEDGE_LLM_MAX_TOKENS=4096
export PGEDGE_LLM_TEMPERATURE=0.7
```

**API Key Priority** (from highest to lowest):
1. `PGEDGE_ANTHROPIC_API_KEY` or `PGEDGE_OPENAI_API_KEY` environment variable
2. `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` environment variable
3. Key file specified in config (`anthropicAPIKeyFile` or `openaiAPIKeyFile`)
4. Empty string (will fail validation if provider is selected)

#### Custom Config File

You can specify a custom config file path using the `CONFIG_FILE` environment variable:

```bash
CONFIG_FILE=/path/to/config.json npm run serve:dev
```

### Port Configuration

The web client uses a three-tier architecture:

- **MCP Server**: Port 8080 (HTTP/HTTPS API)
- **Express Backend**: Port 3001 (API proxy and session handling)
- **Vite Dev Server**: Port 3000 (Frontend development server)

## Installation

```bash
cd web
npm install
```

## Development

Start the development servers (both Express backend and Vite dev server required):

```bash
# Option 1: Use the startup script (recommended - starts all three services)
cd ..
./start_web_client.sh

# Option 2: Manual startup in separate terminals
# Terminal 1: Start MCP server
./bin/pgedge-pg-mcp-svr --config bin/pgedge-pg-mcp-web.yaml

# Terminal 2: Start Express backend (API proxy)
cd web
npm run serve:dev

# Terminal 3: Start Vite dev server (frontend)
cd web
npm run dev
```

Then open [http://localhost:3000](http://localhost:3000) in your browser.

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

- **Backend API Tests**: Express server endpoints tested with Supertest
  - POST `/api/login` - User authentication
  - POST `/api/logout` - Session termination
  - GET `/api/auth/status` - Authentication status check
  - GET `/api/mcp/system-info` - PostgreSQL system information

Coverage reports are generated in the `coverage/` directory.

## Production

Build the application:

```bash
npm run build
```

Run the production server:

```bash
NODE_ENV=production npm run serve:prod
```

The Express backend server will run on port 3001 by default (configurable in `config.json`). The built frontend assets are served from the same Express server in production mode.

## Quick Start

For a quick start with everything configured, use the provided startup script from the project root:

```bash
./start_web_client.sh
```

This script will:
- Start the MCP server in HTTP mode (port 8080) with authentication enabled
- Start the Express backend server (port 3001) for API proxy and session management
- Start the Vite development server (port 3000) for the frontend
- Use pre-configured settings from `bin/pgedge-pg-mcp-web.yaml` and `bin/pgedge-mcp-web-config.json`
- Use existing user and token files from the `bin/` directory

The MCP server supports both authentication methods simultaneously:
- **Service Tokens**: Long-lived API tokens from `pgedge-pg-mcp-svr-tokens.yaml` (for programmatic access)
- **User Authentication**: Username/password authentication via `authenticate_user` tool (used by web interface)

Both authentication methods create session tokens that provide per-token database connection isolation for security and resource management.

**Note**: Make sure you have `OPENAI_API_KEY` set in your environment if you want embedding generation to work.

## Manual Usage

1. Start your pgEdge MCP Server with HTTP mode and authentication enabled:
   ```bash
   ./bin/pgedge-pg-mcp-svr --config bin/pgedge-pg-mcp-web.yaml
   ```

2. Start the web client (development or production mode as described above)

3. Open your browser and navigate to the web client URL

4. Login with your MCP server credentials (username and password)

5. View the MCP server status and PostgreSQL system information in the banner

6. Use the chat interface to ask questions about your database:
   - Ask questions in natural language (e.g., "What tables are in my database?")
   - The LLM will autonomously call MCP tools and resources to answer
   - View conversation history and clear it when starting a new topic

## Architecture

The web client uses a three-tier architecture:

### Frontend (Port 3000 in dev)
- **Framework**: React 18 with Material-UI (MUI)
- **Build Tool**: Vite
- **State Management**: React Context API
- **Features**:
  - Chat interface for natural language database interaction
  - Collapsible status banner with PostgreSQL system info
  - Authentication UI
  - Light/dark theme toggle
- **Testing**: Vitest + React Testing Library

### Backend (Port 3001)
- **Framework**: Express.js
- **Purpose**: Session handling, LLM integration, and MCP server proxy
- **Features**:
  - Session-based authentication with conversation history
  - Agentic chat loop with LLM tool calling
  - Multi-LLM support (Anthropic, OpenAI, Ollama)
  - Proxies requests to MCP server with proper token handling
  - HTTP-only cookies for security
- **Testing**: Vitest + Supertest

### MCP Server (Port 8080)
- **Protocol**: JSON-RPC 2.0 over HTTP/HTTPS
- **Authentication**: Token-based (both service tokens and session tokens)
- **Features**:
  - Database connection pooling with per-token isolation
  - Read-only query execution
  - Resource access (PostgreSQL system info, database schema)
  - Tool execution (query, schema info, similarity search, embeddings)

### Authentication Flow
1. User enters username/password in web UI
2. Express backend calls MCP server's `authenticate_user` tool
3. MCP server validates credentials and returns a session token
4. Express backend stores session token in server-side session
5. Subsequent requests include session token as Bearer token to MCP server
6. MCP server uses token hash for per-token database connection isolation

### Agentic Chat Flow
1. User sends a message in the chat interface
2. Express backend creates ChatAgent with conversation history from session
3. ChatAgent fetches available tools and resources from MCP server
4. ChatAgent enters agentic loop (max 10 iterations):
   - Sends user message + conversation history to LLM with available tools
   - If LLM requests tool use:
     - Executes tool/resource via MCP server
     - Adds tool results to conversation
     - Continues loop
   - If LLM provides final text response:
     - Returns response to user
     - Updates conversation history in session
5. Frontend displays LLM response in chat interface

## Security

- All API calls to the MCP server require authentication
- Session tokens are stored server-side
- Session cookies are HTTP-only
- In production, enable HTTPS and set secure cookies
- Per-token database connection isolation prevents cross-user data access

## Troubleshooting

**Backend server fails to start with "API key is required" error:**
- This means the LLM API key file is missing or empty
- Create the required API key file (see API Key Configuration section above)
- For Anthropic: `echo "your-key" > ~/.pgedge-pg-mcp-web-anthropic-key && chmod 600 ~/.pgedge-pg-mcp-web-anthropic-key`
- For OpenAI: `echo "your-key" > ~/.pgedge-pg-mcp-web-openai-key && chmod 600 ~/.pgedge-pg-mcp-web-openai-key`
- Alternatively, set `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` environment variable
- Check backend logs: `tail -50 /tmp/pgedge-mcp-backend.log`

**"Your session has expired" or automatic logout:**
- This happens when the backend server restarts while you're logged in
- Sessions are stored in server memory and are lost on restart
- Simply log in again - this is normal and protects against stale sessions
- In production, consider using a persistent session store (Redis, etc.)

**System info shows "N/A":**
- Ensure MCP server is running and configured with database connection
- Check that authentication is working (you should be able to log in)
- Verify the MCP server has database connection configured (check `pgedge-pg-mcp-web.yaml`)
- Check browser console and backend logs (`/tmp/pgedge-mcp-backend.log`) for errors
- Try hard refresh (Cmd+Shift+R on Mac, Ctrl+Shift+R on Windows/Linux)

**Cannot login:**
- Verify user exists in `pgedge-pg-mcp-svr-users.yaml`
- Check MCP server is running in HTTP mode with authentication enabled
- Review backend logs for authentication errors
- Ensure Express backend is running on port 3001

**Backend server won't start:**
- Check if port 3001 is already in use: `lsof -i :3001`
- Verify Node.js version is 18.x or higher: `node --version`
- Ensure dependencies are installed: `npm install`
- Check CONFIG_FILE path is correct if using custom config

**Frontend won't connect to backend:**
- Verify Vite dev server is running on port 3000
- Check Express backend is running on port 3001
- Look for CORS errors in browser console
- Ensure `credentials: 'include'` is set in fetch requests

**Tests failing:**
- Ensure all dependencies are installed: `npm install`
- Clear test cache: `npx vitest --clearCache`
- Check Node.js version is compatible
- Review test output for specific errors

## License

This software is released under The PostgreSQL License.

Copyright (c) 2025, pgEdge, Inc.
