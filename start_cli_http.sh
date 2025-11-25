#!/bin/bash

#--------------------------------------------------------------------------
#
# pgEdge MCP CLI Client Startup Script (HTTP Mode)
#
# Portions copyright (c) 2025, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
# This script starts the MCP server in HTTP mode with authentication and
# LLM proxy enabled, then starts the CLI chat client to connect to it via
# HTTP. When the CLI exits, the MCP server is automatically stopped.
#
# Usage:
#   ./start_cli_http.sh
#   CONFIG_FILE=custom.yaml ./start_cli_http.sh
#
#--------------------------------------------------------------------------

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get script directory (should be project root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SCRIPT_DIR"
BIN_DIR="$PROJECT_DIR/bin"

# Binaries
CLI_BIN="$BIN_DIR/pgedge-pg-mcp-cli"
SERVER_BIN="$BIN_DIR/pgedge-pg-mcp-svr"

# Configuration files
SERVER_CONFIG="$BIN_DIR/pgedge-nla-server-http.yaml"
CLI_CONFIG="${CONFIG_FILE:-$BIN_DIR/pgedge-nla-cli-http.yaml}"

# PID file for cleanup
MCP_SERVER_PID=""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Shutting down services...${NC}"

    if [ ! -z "$MCP_SERVER_PID" ]; then
        echo "Stopping MCP server (PID: $MCP_SERVER_PID)..."
        kill $MCP_SERVER_PID 2>/dev/null || true
        # Wait a moment for graceful shutdown
        sleep 1
        # Force kill if still running
        if kill -0 $MCP_SERVER_PID 2>/dev/null; then
            kill -9 $MCP_SERVER_PID 2>/dev/null || true
        fi
    fi

    echo -e "${GREEN}Cleanup complete${NC}"
}

# Set up trap for cleanup
trap cleanup EXIT INT TERM

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     pgEdge MCP CLI Client Startup (HTTP Mode)              ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Build or rebuild server binary if needed
if [ ! -f "$SERVER_BIN" ]; then
    echo -e "${BLUE}Building MCP server binary...${NC}"
    cd "$PROJECT_DIR"
    make build-server
    if [ $? -ne 0 ]; then
        echo -e "${RED}Error: Failed to build MCP server${NC}"
        exit 1
    fi
else
    echo -e "${BLUE}Checking if server binary needs rebuilding...${NC}"
    # Check if any Go source files are newer than the binary
    if [ -n "$(find "$PROJECT_DIR/cmd/pgedge-pg-mcp-svr" "$PROJECT_DIR/internal" -name "*.go" -newer "$SERVER_BIN" 2>/dev/null)" ]; then
        echo -e "${BLUE}Source files changed, rebuilding MCP server...${NC}"
        cd "$PROJECT_DIR"
        make build-server
        if [ $? -ne 0 ]; then
            echo -e "${RED}Error: Failed to build MCP server${NC}"
            exit 1
        fi
    else
        echo -e "${GREEN}Server binary is up to date${NC}"
    fi
fi

# Build or rebuild CLI binary if needed
if [ ! -f "$CLI_BIN" ]; then
    echo -e "${BLUE}Building CLI client binary...${NC}"
    cd "$PROJECT_DIR"
    make build-client
    if [ $? -ne 0 ]; then
        echo -e "${RED}Error: Failed to build CLI client${NC}"
        exit 1
    fi
else
    echo -e "${BLUE}Checking if CLI binary needs rebuilding...${NC}"
    # Check if any Go source files are newer than the binary
    if [ -n "$(find "$PROJECT_DIR/cmd/pgedge-pg-mcp-cli" "$PROJECT_DIR/internal/chat" -name "*.go" -newer "$CLI_BIN" 2>/dev/null)" ]; then
        echo -e "${BLUE}Source files changed, rebuilding CLI client...${NC}"
        cd "$PROJECT_DIR"
        make build-client
        if [ $? -ne 0 ]; then
            echo -e "${RED}Error: Failed to build CLI client${NC}"
            exit 1
        fi
    else
        echo -e "${GREEN}CLI binary is up to date${NC}"
    fi
fi

# Check if server config file exists
if [ ! -f "$SERVER_CONFIG" ]; then
    echo -e "${RED}Error: Server config file not found at $SERVER_CONFIG${NC}"
    exit 1
fi

# Check if CLI config file exists
if [ ! -f "$CLI_CONFIG" ]; then
    echo -e "${RED}Error: CLI config file not found at $CLI_CONFIG${NC}"
    exit 1
fi

# Display configuration
echo ""
echo -e "${BLUE}Configuration:${NC}"
echo "  Server config: $SERVER_CONFIG"
echo "  CLI config:    $CLI_CONFIG"
echo ""

# Check for database connection
DB_WARNING=""
if [ -z "$PGHOST" ] && [ -z "$PGEDGE_DB_HOST" ] && [ -z "$PGEDGE_POSTGRES_CONNECTION_STRING" ]; then
    DB_WARNING="${YELLOW}Warning: No database connection configured. The MCP server will use localhost:5432 by default.${NC}"
fi

# Check for API keys
API_KEY_WARNING=""
if [ -z "$PGEDGE_ANTHROPIC_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ] && [ ! -f "$HOME/.anthropic-api-key" ] && \
   [ -z "$PGEDGE_OPENAI_API_KEY" ] && [ -z "$OPENAI_API_KEY" ] && [ ! -f "$HOME/.openai-api-key" ]; then
    API_KEY_WARNING="${YELLOW}Warning: No LLM API keys found (env vars or ~/.anthropic-api-key / ~/.openai-api-key files).
         LLM features will not work unless you're using Ollama.${NC}"
fi

# Display warnings if any
if [ ! -z "$DB_WARNING" ]; then
    echo -e "$DB_WARNING"
fi
if [ ! -z "$API_KEY_WARNING" ]; then
    echo -e "$API_KEY_WARNING"
fi
if [ ! -z "$DB_WARNING" ] || [ ! -z "$API_KEY_WARNING" ]; then
    echo ""
fi

# Start MCP server
echo -e "${GREEN}[1/2] Starting MCP Server (HTTP mode with auth and LLM proxy)...${NC}"
cd "$BIN_DIR"
"$SERVER_BIN" --config "$SERVER_CONFIG" > /tmp/pgedge-mcp-server.log 2>&1 &
MCP_SERVER_PID=$!
cd "$PROJECT_DIR"

echo "      PID: $MCP_SERVER_PID"
echo "      Config: $SERVER_CONFIG"
echo "      Log: /tmp/pgedge-mcp-server.log"

# Wait a moment for process to stabilize
sleep 1

# Check if MCP server process is still running (catch immediate failures like port conflicts)
if ! kill -0 $MCP_SERVER_PID 2>/dev/null; then
    echo -e "${RED}Error: MCP Server process exited immediately${NC}"
    echo "This usually means the port is already in use or there's a configuration error."
    echo "Check the log file: /tmp/pgedge-mcp-server.log"
    tail -20 /tmp/pgedge-mcp-server.log
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

echo ""
echo -e "${GREEN}[2/2] Starting CLI Chat Client (HTTP mode)...${NC}"
echo "      Connecting to: http://localhost:8080"
echo "      Config: $CLI_CONFIG"
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo ""

# Run CLI in foreground - when it exits, cleanup will run
cd "$PROJECT_DIR"
"$CLI_BIN" --config "$CLI_CONFIG"
