#!/bin/bash

#--------------------------------------------------------------------------
#
# pgEdge MCP Web Client Development Startup Script
#
# Portions copyright (c) 2025, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
# Note: For production deployments, use Docker Compose instead:
#   docker-compose up
#
#--------------------------------------------------------------------------

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$SCRIPT_DIR/bin"
WEB_DIR="$SCRIPT_DIR/web"

# Source directories
CMD_SERVER_DIR="$SCRIPT_DIR/cmd/pgedge-pg-mcp-svr"
INTERNAL_DIR="$SCRIPT_DIR/internal"

# Configuration files
SERVER_CONFIG="$BIN_DIR/pgedge-postgres-mcp-http.yaml"
SERVER_BIN="$BIN_DIR/pgedge-postgres-mcp"

# Log files
SERVER_LOG="/tmp/pgedge-postgres-mcp.log"
VITE_LOG="/tmp/pgedge-nla-vite.log"

# PID files for cleanup
MCP_SERVER_PID=""
WEB_SERVER_PID=""

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Shutting down services...${NC}"

    if [ ! -z "$WEB_SERVER_PID" ]; then
        echo "Stopping Vite dev server (PID: $WEB_SERVER_PID)..."
        kill $WEB_SERVER_PID 2>/dev/null || true
    fi

    if [ ! -z "$MCP_SERVER_PID" ]; then
        echo "Stopping MCP server (PID: $MCP_SERVER_PID)..."
        kill $MCP_SERVER_PID 2>/dev/null || true
    fi

    echo -e "${GREEN}Cleanup complete${NC}"
}

# Set up trap for cleanup
trap cleanup EXIT INT TERM

# Build or rebuild server binary if needed
if [ ! -f "$SERVER_BIN" ]; then
    echo -e "${BLUE}Building MCP server binary...${NC}"
    cd "$SCRIPT_DIR"
    make build-server
    if [ $? -ne 0 ]; then
        echo -e "${RED}Error: Failed to build MCP server${NC}"
        exit 1
    fi
else
    echo -e "${BLUE}Checking if server binary needs rebuilding...${NC}"
    # Check if any Go source files are newer than the binary
    if [ -n "$(find "$CMD_SERVER_DIR" "$INTERNAL_DIR" -name "*.go" -newer "$SERVER_BIN" 2>/dev/null)" ]; then
        echo -e "${BLUE}Source files changed, rebuilding MCP server...${NC}"
        cd "$SCRIPT_DIR"
        make build-server
        if [ $? -ne 0 ]; then
            echo -e "${RED}Error: Failed to build MCP server${NC}"
            exit 1
        fi
    else
        echo -e "${GREEN}Server binary is up to date${NC}"
    fi
fi

# Check if config files exist
if [ ! -f "$SERVER_CONFIG" ]; then
    echo -e "${RED}Error: Server config not found at $SERVER_CONFIG${NC}"
    exit 1
fi

# Check if web dependencies need updating
if [ ! -d "$WEB_DIR/node_modules" ]; then
    echo -e "${BLUE}Installing web dependencies...${NC}"
    cd "$WEB_DIR"
    npm install
    cd "$SCRIPT_DIR"
else
    echo -e "${BLUE}Checking if web dependencies need updating...${NC}"
    # Check if package.json or package-lock.json are newer than node_modules
    if [ "$WEB_DIR/package.json" -nt "$WEB_DIR/node_modules" ] || [ "$WEB_DIR/package-lock.json" -nt "$WEB_DIR/node_modules" ]; then
        echo -e "${BLUE}Package files changed, running npm install...${NC}"
        cd "$WEB_DIR"
        npm install
        cd "$SCRIPT_DIR"
    else
        echo -e "${GREEN}Web dependencies are up to date${NC}"
    fi
fi

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     pgEdge MCP Web Client Development Startup              ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Start MCP server
echo -e "${GREEN}[1/2] Starting MCP Server (HTTP mode with auth and LLM proxy)...${NC}"
cd "$BIN_DIR"
"$SERVER_BIN" --config "$SERVER_CONFIG" > "$SERVER_LOG" 2>&1 &
MCP_SERVER_PID=$!
cd "$SCRIPT_DIR"

echo "      PID: $MCP_SERVER_PID"
echo "      Config: $SERVER_CONFIG"
echo "      Log: $SERVER_LOG"

# Wait a moment for process to stabilize
sleep 1

# Check if MCP server process is still running (catch immediate failures like port conflicts)
if ! kill -0 $MCP_SERVER_PID 2>/dev/null; then
    echo -e "${RED}Error: MCP Server process exited immediately${NC}"
    echo "This usually means the port is already in use or there's a configuration error."
    echo "Check the log file: $SERVER_LOG"
    tail -20 "$SERVER_LOG"
    exit 1
fi

# Wait for MCP server to be ready
echo -e "${GREEN}Waiting for MCP Server to be ready...${NC}"
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo "      MCP Server is ready!"
        break
    fi

    # Check if process is still running
    if ! kill -0 $MCP_SERVER_PID 2>/dev/null; then
        echo -e "${RED}Error: MCP Server process died during startup${NC}"
        echo "Check the log file: $SERVER_LOG"
        tail -20 "$SERVER_LOG"
        exit 1
    fi

    RETRY_COUNT=$((RETRY_COUNT + 1))
    sleep 1
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo -e "${RED}Error: MCP Server failed to start within 30 seconds${NC}"
    echo "Check the log file: $SERVER_LOG"
    tail -20 "$SERVER_LOG"
    exit 1
fi

# Start Vite dev server
echo -e "${GREEN}[2/2] Starting Vite Dev Server (Frontend)...${NC}"
cd "$WEB_DIR"
npm run dev > "$VITE_LOG" 2>&1 &
WEB_SERVER_PID=$!
cd "$SCRIPT_DIR"

echo "      PID: $WEB_SERVER_PID"
echo "      Port: 5173 (Vite default)"
echo "      Log: $VITE_LOG"

# Wait a moment for process to stabilize
sleep 1

# Check if Vite server process is still running (catch immediate failures like port conflicts)
if ! kill -0 $WEB_SERVER_PID 2>/dev/null; then
    echo -e "${RED}Error: Vite Server process exited immediately${NC}"
    echo "This usually means port 5173 is already in use or there's a configuration error."
    echo "Check the log file: $VITE_LOG"
    tail -20 "$VITE_LOG"
    exit 1
fi

# Wait for Vite server to be ready
echo -e "${GREEN}Waiting for Vite Server to be ready...${NC}"
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if grep -q "Local:" "$VITE_LOG" 2>/dev/null; then
        break
    fi

    # Check if process is still running
    if ! kill -0 $WEB_SERVER_PID 2>/dev/null; then
        echo -e "${RED}Error: Vite Server process died during startup${NC}"
        echo "Check the log file: $VITE_LOG"
        tail -20 "$VITE_LOG"
        exit 1
    fi

    RETRY_COUNT=$((RETRY_COUNT + 1))
    sleep 1
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo -e "${RED}Error: Vite Server failed to start within 30 seconds${NC}"
    echo "Check the log file: $VITE_LOG"
    tail -20 "$VITE_LOG"
    exit 1
fi

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  ✓ pgEdge MCP Web Client Development is now running!       ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Services:${NC}"
echo "  • MCP Server:     http://localhost:8080"
echo "  • Web Interface:  http://localhost:5173"
echo ""
echo -e "${BLUE}Logs:${NC}"
echo "  • MCP Server:     $SERVER_LOG"
echo "  • Vite Dev:       $VITE_LOG"
echo ""
echo -e "${BLUE}Login Credentials (for web interface):${NC}"
echo "  • Username: dpage"
echo "  • Password: (as configured in bin/pgedge-postgres-mcp-users.yaml)"
echo ""
echo -e "${BLUE}Architecture:${NC}"
echo "  • Web client communicates directly with MCP server via JSON-RPC"
echo "  • LLM API keys are stored server-side (configured in MCP server YAML)"
echo "  • MCP server provides tool/resource access and LLM proxy endpoints"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop all services${NC}"
echo ""

# Wait for interrupt
wait $WEB_SERVER_PID
