#!/bin/bash

#--------------------------------------------------------------------------
#
# pgEdge MCP Server Codespaces Demo Startup Script
#
# Copyright (c) 2025 - 2026, pgEdge, Inc.
# This software is released under The PostgreSQL License
#
#--------------------------------------------------------------------------

# Starts the pgEdge MCP Server Codespaces demo.
#
# Usage (from repo root):
#   ./examples/codespaces-demo/start.sh
#
# What it does:
#   1. Checks for API keys (Codespace secrets → .env → interactive prompt)
#   2. Starts PostgreSQL + MCP Server + Web Client via docker compose
#   3. Waits for services to be healthy
#   4. Prints the Web UI URL and login credentials
set -e

# Resolve paths relative to this script
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="$SCRIPT_DIR/.env"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"

# Ensure .env exists
if [ ! -f "$ENV_FILE" ]; then
  cp "$SCRIPT_DIR/.env.example" "$ENV_FILE"
fi

MCP_PORT=$(grep '^MCP_SERVER_PORT=' "$ENV_FILE" 2>/dev/null | cut -d= -f2)
MCP_PORT="${MCP_PORT:-8080}"
WEB_PORT=$(grep '^WEB_CLIENT_PORT=' "$ENV_FILE" 2>/dev/null | cut -d= -f2)
WEB_PORT="${WEB_PORT:-8081}"

# ─── 1. Apply Codespace secrets to .env if present ─────────────────────────

# Escape sed replacement metacharacters (& \ /) in a value
escape_sed() { printf '%s' "$1" | sed 's/[&\\/]/\\&/g'; }

if [ -n "$PGEDGE_ANTHROPIC_API_KEY" ]; then
  ant_esc=$(escape_sed "$PGEDGE_ANTHROPIC_API_KEY")
  sed -i "s/^PGEDGE_ANTHROPIC_API_KEY=.*/PGEDGE_ANTHROPIC_API_KEY=$ant_esc/" "$ENV_FILE"
fi
if [ -n "$PGEDGE_OPENAI_API_KEY" ]; then
  oai_esc=$(escape_sed "$PGEDGE_OPENAI_API_KEY")
  sed -i "s/^PGEDGE_OPENAI_API_KEY=.*/PGEDGE_OPENAI_API_KEY=$oai_esc/" "$ENV_FILE"
  # Only switch provider to OpenAI if no Anthropic key is present
  if [ -z "$PGEDGE_ANTHROPIC_API_KEY" ]; then
    sed -i "s/^PGEDGE_LLM_PROVIDER=.*/PGEDGE_LLM_PROVIDER=openai/" "$ENV_FILE"
    sed -i "s/^PGEDGE_LLM_MODEL=.*/PGEDGE_LLM_MODEL=gpt-4o/" "$ENV_FILE"
  fi
fi

# ─── 2. Check if any API key is configured ─────────────────────────────────

ANT_KEY=$(grep '^PGEDGE_ANTHROPIC_API_KEY=' "$ENV_FILE" | cut -d= -f2-)
OAI_KEY=$(grep '^PGEDGE_OPENAI_API_KEY=' "$ENV_FILE" | cut -d= -f2-)

# ─── 3. If no key, prompt with hidden paste ────────────────────────────────

if [ -z "$ANT_KEY" ] && [ -z "$OAI_KEY" ]; then
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  pgEdge MCP Server Demo — API Key Setup"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "  An LLM API key is required. Enter at least one below."
  echo "  (Your input is hidden. Press Enter to skip a provider.)"
  echo ""

  # Anthropic
  read -s -r -p "  Anthropic API key (Enter to skip): " ant_input < /dev/tty
  echo ""
  if [ -n "$ant_input" ]; then
    ant_esc=$(escape_sed "$ant_input")
    sed -i "s/^PGEDGE_ANTHROPIC_API_KEY=.*/PGEDGE_ANTHROPIC_API_KEY=$ant_esc/" "$ENV_FILE"
    ANT_KEY="$ant_input"
  fi

  # OpenAI
  read -s -r -p "  OpenAI API key (Enter to skip):    " oai_input < /dev/tty
  echo ""
  if [ -n "$oai_input" ]; then
    oai_esc=$(escape_sed "$oai_input")
    sed -i "s/^PGEDGE_OPENAI_API_KEY=.*/PGEDGE_OPENAI_API_KEY=$oai_esc/" "$ENV_FILE"
    # If no Anthropic key, switch provider to OpenAI
    if [ -z "$ANT_KEY" ]; then
      sed -i "s/^PGEDGE_LLM_PROVIDER=.*/PGEDGE_LLM_PROVIDER=openai/" "$ENV_FILE"
      sed -i "s/^PGEDGE_LLM_MODEL=.*/PGEDGE_LLM_MODEL=gpt-4o/" "$ENV_FILE"
    fi
    OAI_KEY="$oai_input"
  fi

  echo ""

  # Still no key?
  if [ -z "$ANT_KEY" ] && [ -z "$OAI_KEY" ]; then
    echo "  ✗  No API key provided. Cannot start the demo."
    echo ""
    echo "  Get a key from:"
    echo "    Anthropic: https://console.anthropic.com/"
    echo "    OpenAI:    https://platform.openai.com/"
    echo ""
    exit 1
  fi

  echo "  ✓  API key saved"
  echo ""
fi

# ─── 4. Start services ────────────────────────────────────────────────────

echo ""
echo "Starting pgEdge MCP Server demo..."
echo ""

docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d

echo ""
echo "Waiting for services to be healthy..."

# Wait for MCP server health (up to 90 seconds)
for i in $(seq 1 18); do
  if curl -sf http://localhost:$MCP_PORT/health > /dev/null 2>&1; then
    break
  fi
  sleep 5
done

# Build the Web UI URL
if [ -n "$CODESPACE_NAME" ] && [ -n "$GITHUB_CODESPACES_PORT_FORWARDING_DOMAIN" ]; then
  WEB_URL="https://${CODESPACE_NAME}-${WEB_PORT}.${GITHUB_CODESPACES_PORT_FORWARDING_DOMAIN}"
else
  WEB_URL="http://localhost:$WEB_PORT"
fi

# Wait for web client (up to 30 more seconds)
for i in $(seq 1 6); do
  if curl -sf http://localhost:$WEB_PORT/health > /dev/null 2>&1; then
    break
  fi
  sleep 5
done

# ─── 5. Print result ──────────────────────────────────────────────────────

if curl -sf http://localhost:$WEB_PORT/health > /dev/null 2>&1; then
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  pgEdge MCP Server Demo is running!"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "  Web UI:  $WEB_URL"
  echo "  Login:   demo / demo123"
  echo ""
  echo "  Try asking:"
  echo "    What tables are in the database?"
  echo "    Show me the top 10 products by sales"
  echo "    Which customers have placed more than 5 orders?"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
else
  echo ""
  echo "Services are starting. Check progress with:"
  echo "  docker compose -f $COMPOSE_FILE logs -f"
  echo ""
  echo "Once ready, open: $WEB_URL"
  echo ""
fi
