#!/bin/sh
set -e

# pgEdge Postgres MCP Server Initialization Script
# This script initializes users and tokens from environment variables

echo "Starting pgEdge Postgres MCP Server..."

# Build command line arguments
ARGS=""

# Add HTTP mode and address
ARGS="$ARGS -http -addr :8080"

# Add database connection if provided
if [ -n "$PGEDGE_DB_HOST" ]; then
    ARGS="$ARGS -db-host $PGEDGE_DB_HOST"
fi

if [ -n "$PGEDGE_DB_PORT" ]; then
    ARGS="$ARGS -db-port $PGEDGE_DB_PORT"
fi

if [ -n "$PGEDGE_DB_NAME" ]; then
    ARGS="$ARGS -db-name $PGEDGE_DB_NAME"
fi

if [ -n "$PGEDGE_DB_USER" ]; then
    ARGS="$ARGS -db-user $PGEDGE_DB_USER"
fi

if [ -n "$PGEDGE_DB_PASSWORD" ]; then
    ARGS="$ARGS -db-password $PGEDGE_DB_PASSWORD"
fi

if [ -n "$PGEDGE_DB_SSLMODE" ]; then
    ARGS="$ARGS -db-sslmode $PGEDGE_DB_SSLMODE"
fi

# Add token file path if provided
if [ -n "$PGEDGE_TOKEN_FILE" ]; then
    ARGS="$ARGS -token-file $PGEDGE_TOKEN_FILE"
else
    # Default token file location
    ARGS="$ARGS -token-file /app/data/tokens.json"
fi

# Add user file path if provided
if [ -n "$PGEDGE_USERS_FILE" ]; then
    ARGS="$ARGS -user-file $PGEDGE_USERS_FILE"
else
    # Default user file location
    ARGS="$ARGS -user-file /app/data/users.json"
fi

# Add debug flag if requested
if [ "$PGEDGE_DEBUG" = "true" ]; then
    ARGS="$ARGS -debug"
fi

# Create data directory with proper permissions
mkdir -p /app/data
chown 1001:1001 /app/data

# Initialize token file if INIT_TOKENS is provided
TOKEN_FILE="${PGEDGE_TOKEN_FILE:-/app/data/tokens.json}"
if [ -n "$INIT_TOKENS" ]; then
    echo "Initializing tokens from INIT_TOKENS environment variable..."

    # Create tokens.json from INIT_TOKENS (expected format: token1,token2,token3)
    echo "{" > "$TOKEN_FILE"
    FIRST=true
    IFS=','
    for token in $INIT_TOKENS; do
        if [ "$FIRST" = true ]; then
            FIRST=false
        else
            echo "," >> "$TOKEN_FILE"
        fi
        echo "  \"$token\": {" >> "$TOKEN_FILE"
        echo "    \"created_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"," >> "$TOKEN_FILE"
        echo "    \"description\": \"Auto-generated token\"" >> "$TOKEN_FILE"
        echo -n "  }" >> "$TOKEN_FILE"
    done
    echo "" >> "$TOKEN_FILE"
    echo "}" >> "$TOKEN_FILE"

    echo "Created token file with $(echo "$INIT_TOKENS" | tr ',' '\n' | wc -l | tr -d ' ') tokens"
else
    # Create empty tokens.json if not using token authentication
    echo "{}" > "$TOKEN_FILE"
    echo "Created empty token file (not using token authentication)"
fi

# Initialize users file if INIT_USERS is provided
USERS_FILE="${PGEDGE_USERS_FILE:-/app/data/users.json}"
if [ -n "$INIT_USERS" ]; then
    echo "Initializing users from INIT_USERS environment variable..."

    # Create empty users file first
    echo "{}" > "$USERS_FILE"
    chown 1001:1001 "$USERS_FILE"

    # Use the server's -add-user command to properly hash passwords
    # Expected format: username1:password1,username2:password2
    IFS=','
    USER_COUNT=0
    for user_entry in $INIT_USERS; do
        username=$(echo "$user_entry" | cut -d: -f1)
        password=$(echo "$user_entry" | cut -d: -f2-)

        # Add user using the server's built-in command (which hashes the password)
        /app/pgedge-pg-mcp-svr -add-user -username "$username" -password "$password" -user-file "$USERS_FILE" -user-note "Auto-generated user"
        USER_COUNT=$((USER_COUNT + 1))
    done

    echo "Created users file with $USER_COUNT user(s)"
fi

# Ensure all data files are owned by user 1001
if [ -f "${PGEDGE_TOKEN_FILE:-/app/data/tokens.json}" ]; then
    chown 1001:1001 "${PGEDGE_TOKEN_FILE:-/app/data/tokens.json}"
fi
if [ -f "${PGEDGE_USERS_FILE:-/app/data/users.json}" ]; then
    chown 1001:1001 "${PGEDGE_USERS_FILE:-/app/data/users.json}"
fi

# Start the MCP server with all arguments
echo "Starting MCP server with arguments: $ARGS"

# Switch to mcp user and exec the server (use runuser which is available in ubi9-minimal)
exec runuser mcp /bin/sh -c "exec /app/pgedge-pg-mcp-svr $ARGS"
