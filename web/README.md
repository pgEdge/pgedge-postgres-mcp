# pgEdge MCP Client

Web-based client for pgEdge MCP Server. This application provides a clean, modern interface to connect to and monitor your MCP server.

## Features

- **Authentication**: Secure login using MCP server's user authentication
- **System Monitoring**: Real-time PostgreSQL server status and statistics
- **Dark Mode**: Toggle between light and dark themes
- **Responsive Design**: Works on desktop and mobile devices

## Prerequisites

- Node.js 18.x or higher
- A running pgEdge MCP Server (HTTP mode with authentication enabled)

## Configuration

Edit `config.json` to configure your MCP server connection:

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
  }
}
```

**Important**: Change the `session.secret` to a random string in production.

You can also specify a custom config file path using the `CONFIG_FILE` environment variable:

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

5. View the MCP server status and PostgreSQL system information

## Architecture

The web client uses a three-tier architecture:

### Frontend (Port 3000 in dev)
- **Framework**: React 18 with Material-UI (MUI)
- **Build Tool**: Vite
- **State Management**: React Context API
- **Features**: Dashboard with real-time PostgreSQL system info, authentication UI, light/dark theme
- **Testing**: Vitest + React Testing Library

### Backend (Port 3001)
- **Framework**: Express.js
- **Purpose**: Session handling and MCP server proxy
- **Features**:
  - Session-based authentication
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

## Security

- All API calls to the MCP server require authentication
- Session tokens are stored server-side
- Session cookies are HTTP-only
- In production, enable HTTPS and set secure cookies
- Per-token database connection isolation prevents cross-user data access

## Troubleshooting

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
