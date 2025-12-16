#!/bin/bash

#--------------------------------------------------------------------------
#
# pgEdge MCP CLI Client Startup Script (Stdio Mode)
#
# Portions copyright (c) 2025, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
# This script starts the CLI chat client in stdio mode. The CLI client
# will automatically spawn the MCP server as a subprocess and communicate
# via standard input/output. The MCP server will automatically use its
# config file (bin/pgedge-postgres-mcp-stdio.yaml) located in the same directory
# as the server binary.
#
# Usage:
#   ./start_cli_stdio.sh
#   CONFIG_FILE=custom.yaml ./start_cli_stdio.sh
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

# Source directories
CMD_CLI_DIR="$PROJECT_DIR/cmd/pgedge-pg-mcp-cli"
CMD_SERVER_DIR="$PROJECT_DIR/cmd/pgedge-pg-mcp-svr"
INTERNAL_DIR="$PROJECT_DIR/internal"
INTERNAL_CHAT_DIR="$PROJECT_DIR/internal/chat"

# Binaries
CLI_BIN="$BIN_DIR/pgedge-nla-cli"
SERVER_BIN="$BIN_DIR/pgedge-postgres-mcp"

# Configuration file (can be overridden with CONFIG_FILE env var)
CONFIG_FILE="${CONFIG_FILE:-$BIN_DIR/pgedge-nla-cli-stdio.yaml}"

# Server config
SERVER_CONFIG="$BIN_DIR/pgedge-postgres-mcp-stdio.yaml"

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     pgEdge MCP CLI Client Startup (Stdio Mode)             ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

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
    if [ -n "$(find "$CMD_CLI_DIR" "$INTERNAL_CHAT_DIR" -name "*.go" -newer "$CLI_BIN" 2>/dev/null)" ]; then
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

# Build or rebuild server binary if needed (required for stdio mode)
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
    if [ -n "$(find "$CMD_SERVER_DIR" "$INTERNAL_DIR" -name "*.go" -newer "$SERVER_BIN" 2>/dev/null)" ]; then
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

# Check if CLI config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: CLI config file not found at $CONFIG_FILE${NC}"
    echo ""
    echo "Please ensure the CLI configuration file exists:"
    echo "  $BIN_DIR/pgedge-nla-cli-stdio.yaml"
    echo ""
    echo "Or specify a custom config file:"
    echo "  CONFIG_FILE=/path/to/config.yaml $0"
    exit 1
fi

# Check if server config file exists
if [ ! -f "$SERVER_CONFIG" ]; then
    echo -e "${RED}Error: Server config file not found at $SERVER_CONFIG${NC}"
    echo ""
    echo "The MCP server requires a configuration file with database settings."
    echo "Please ensure the server configuration file exists:"
    echo "  $SERVER_CONFIG"
    echo ""
    echo "You can copy the example config file:"
    echo "  cp $BIN_DIR/pgedge-postgres-mcp.yaml.example $SERVER_CONFIG"
    exit 1
fi

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  ✓ Starting pgEdge MCP CLI Client                          ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Configuration:${NC}"
echo "  • CLI config:    $CONFIG_FILE"
echo "  • CLI binary:    $CLI_BIN"
echo "  • Server binary: $SERVER_BIN"
echo "  • Server config: $SERVER_CONFIG"
echo "  • Mode:          stdio (MCP server spawned as subprocess)"
echo ""
echo -e "${BLUE}Features:${NC}"
echo "  • Natural language database queries"
echo "  • Schema exploration and analysis"
echo "  • Hybrid search (BM25 + vector similarity)"
echo "  • Auto-completion and command history"
echo "  • Anthropic prompt caching (90% cost reduction)"
echo ""
echo -e "${BLUE}Example queries:${NC}"
echo "  • What tables are in my database?"
echo "  • Show me the 10 most recent orders"
echo "  • Which customers have placed more than 5 orders?"
echo "  • Find documents similar to 'PostgreSQL performance tuning'"
echo ""
echo -e "${BLUE}Commands:${NC}"
echo "  • Type your question and press Enter"
echo "  • Use /config to view current configuration"
echo "  • Use /tools to list available database tools"
echo "  • Use /quit or /exit to exit"
echo "  • Press Ctrl+C to exit at any time"
echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo ""

# Change to bin directory so relative paths in config work
cd "$PROJECT_DIR/bin"

# Start the CLI client with the config file
# The CLI will spawn the MCP server automatically via stdio
exec "$CLI_BIN" --config "$(basename "$CONFIG_FILE")"
