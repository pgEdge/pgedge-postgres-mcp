#!/bin/bash

#--------------------------------------------------------------------------
#
# pgEdge MCP Web Client Startup Script
#
# Copyright (c) 2025, pgEdge, Inc.
# This software is released under The PostgreSQL License
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

# Configuration files
SERVER_CONFIG="$BIN_DIR/pgedge-pg-mcp-web.yaml"
WEB_CONFIG="$BIN_DIR/pgedge-mcp-web-config.json"
SERVER_BIN="$BIN_DIR/pgedge-pg-mcp-svr"

# PID files for cleanup
MCP_SERVER_PID=""
BACKEND_SERVER_PID=""
WEB_SERVER_PID=""

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Shutting down services...${NC}"

    if [ ! -z "$WEB_SERVER_PID" ]; then
        echo "Stopping Vite dev server (PID: $WEB_SERVER_PID)..."
        kill $WEB_SERVER_PID 2>/dev/null || true
    fi

    if [ ! -z "$BACKEND_SERVER_PID" ]; then
        echo "Stopping Express backend server (PID: $BACKEND_SERVER_PID)..."
        kill $BACKEND_SERVER_PID 2>/dev/null || true
    fi

    if [ ! -z "$MCP_SERVER_PID" ]; then
        echo "Stopping MCP server (PID: $MCP_SERVER_PID)..."
        kill $MCP_SERVER_PID 2>/dev/null || true
    fi

    echo -e "${GREEN}Cleanup complete${NC}"
}

# Set up trap for cleanup
trap cleanup EXIT INT TERM

# Check if binaries exist
if [ ! -f "$SERVER_BIN" ]; then
    echo -e "${RED}Error: MCP server binary not found at $SERVER_BIN${NC}"
    echo "Please run 'make build-server' first"
    exit 1
fi

# Check if config files exist
if [ ! -f "$SERVER_CONFIG" ]; then
    echo -e "${RED}Error: Server config not found at $SERVER_CONFIG${NC}"
    exit 1
fi

if [ ! -f "$WEB_CONFIG" ]; then
    echo -e "${RED}Error: Web config not found at $WEB_CONFIG${NC}"
    exit 1
fi

# Check for OpenAI API key
if [ -z "$OPENAI_API_KEY" ]; then
    echo -e "${YELLOW}Warning: OPENAI_API_KEY environment variable not set${NC}"
    echo "Embedding generation may not work without it"
fi

# Check if web dependencies are installed
if [ ! -d "$WEB_DIR/node_modules" ]; then
    echo -e "${BLUE}Installing web dependencies...${NC}"
    cd "$WEB_DIR"
    npm install
    cd "$SCRIPT_DIR"
fi

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║         pgEdge MCP Web Client Startup                      ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Start MCP server
echo -e "${GREEN}[1/4] Starting MCP Server (HTTP mode with auth)...${NC}"
cd "$BIN_DIR"
"$SERVER_BIN" --config pgedge-pg-mcp-web.yaml > /tmp/pgedge-mcp-server.log 2>&1 &
MCP_SERVER_PID=$!
cd "$SCRIPT_DIR"

echo "      PID: $MCP_SERVER_PID"
echo "      Config: $SERVER_CONFIG"
echo "      Log: /tmp/pgedge-mcp-server.log"

# Wait for MCP server to be ready
echo -e "${GREEN}[2/4] Waiting for MCP Server to be ready...${NC}"
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo "      MCP Server is ready!"
        break
    fi

    # Check if process is still running
    if ! kill -0 $MCP_SERVER_PID 2>/dev/null; then
        echo -e "${RED}Error: MCP Server process died${NC}"
        echo "Check the log file: /tmp/pgedge-mcp-server.log"
        tail -20 /tmp/pgedge-mcp-server.log
        exit 1
    fi

    RETRY_COUNT=$((RETRY_COUNT + 1))
    sleep 1
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo -e "${RED}Error: MCP Server failed to start within 30 seconds${NC}"
    echo "Check the log file: /tmp/pgedge-mcp-server.log"
    tail -20 /tmp/pgedge-mcp-server.log
    exit 1
fi

# Start Express backend server
echo -e "${GREEN}[3/4] Starting Express Backend Server (API)...${NC}"
cd "$WEB_DIR"
export CONFIG_FILE="$WEB_CONFIG"
npm run serve:dev > /tmp/pgedge-mcp-backend.log 2>&1 &
BACKEND_SERVER_PID=$!
cd "$SCRIPT_DIR"

echo "      PID: $BACKEND_SERVER_PID"
echo "      Port: 3001"
echo "      Config: $WEB_CONFIG"
echo "      Log: /tmp/pgedge-mcp-backend.log"

# Wait for backend to be ready
echo -e "${GREEN}Waiting for Backend Server to be ready...${NC}"
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -s http://localhost:3001/api/session > /dev/null 2>&1; then
        echo "      Backend Server is ready!"
        break
    fi

    # Check if process is still running
    if ! kill -0 $BACKEND_SERVER_PID 2>/dev/null; then
        echo -e "${RED}Error: Backend Server process died${NC}"
        echo "Check the log file: /tmp/pgedge-mcp-backend.log"
        tail -20 /tmp/pgedge-mcp-backend.log
        exit 1
    fi

    RETRY_COUNT=$((RETRY_COUNT + 1))
    sleep 1
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo -e "${RED}Error: Backend Server failed to start within 30 seconds${NC}"
    echo "Check the log file: /tmp/pgedge-mcp-backend.log"
    tail -20 /tmp/pgedge-mcp-backend.log
    exit 1
fi

# Start Vite dev server
echo -e "${GREEN}[4/4] Starting Vite Dev Server (Frontend)...${NC}"
cd "$WEB_DIR"
npm run dev > /tmp/pgedge-mcp-vite.log 2>&1 &
WEB_SERVER_PID=$!
cd "$SCRIPT_DIR"

echo "      PID: $WEB_SERVER_PID"
echo "      Port: 3000"
echo "      Log: /tmp/pgedge-mcp-vite.log"

# Wait for Vite server to be ready
echo -e "${GREEN}Waiting for Vite Server to be ready...${NC}"
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if grep -q "Local:" /tmp/pgedge-mcp-vite.log 2>/dev/null; then
        break
    fi

    # Check if process is still running
    if ! kill -0 $WEB_SERVER_PID 2>/dev/null; then
        echo -e "${RED}Error: Vite Server process died${NC}"
        echo "Check the log file: /tmp/pgedge-mcp-vite.log"
        tail -20 /tmp/pgedge-mcp-vite.log
        exit 1
    fi

    RETRY_COUNT=$((RETRY_COUNT + 1))
    sleep 1
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo -e "${RED}Error: Vite Server failed to start within 30 seconds${NC}"
    echo "Check the log file: /tmp/pgedge-mcp-vite.log"
    tail -20 /tmp/pgedge-mcp-vite.log
    exit 1
fi

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  ✓ pgEdge MCP Web Client is now running!                  ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Services:${NC}"
echo "  • MCP Server:     http://localhost:8080"
echo "  • Backend API:    http://localhost:3001"
echo "  • Web Interface:  http://localhost:3000"
echo ""
echo -e "${BLUE}Logs:${NC}"
echo "  • MCP Server:     /tmp/pgedge-mcp-server.log"
echo "  • Backend API:    /tmp/pgedge-mcp-backend.log"
echo "  • Vite Dev:       /tmp/pgedge-mcp-vite.log"
echo ""
echo -e "${BLUE}Login Credentials (for web interface):${NC}"
echo "  • Username: dpage"
echo "  • Password: (as configured in bin/pgedge-pg-mcp-svr-users.yaml)"
echo ""
echo -e "${BLUE}Authentication:${NC}"
echo "  • MCP Server supports both API tokens and user authentication"
echo "  • Web interface uses user authentication (username/password)"
echo "  • API clients can use tokens from pgedge-pg-mcp-svr-tokens.yaml"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop all services${NC}"
echo ""

# Wait for interrupt
wait $WEB_SERVER_PID
