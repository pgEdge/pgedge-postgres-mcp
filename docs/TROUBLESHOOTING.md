# Troubleshooting Guide

## Server Exits Immediately After Initialize

### Symptoms

- Claude Desktop logs show: "Server transport closed unexpectedly"
- Server starts but disconnects immediately after `initialize` response

### Common Causes

#### 1. Database Connection Issues

**Check the logs for these errors:**

```
[pgedge-postgres-mcp] ERROR: Failed to connect to database: ...
```

**Solutions:**

a) **Verify connection string format:**
    ```bash
    # Correct format:
    postgres://username:password@host:port/database?sslmode=disable

    # Examples:
    postgres://postgres:mypassword@localhost:5432/mydb?sslmode=disable
    postgres://user@localhost/dbname?sslmode=disable  # local trusted auth
    ```

b) **Test PostgreSQL connection directly:**
    ```bash
    # Using psql
    psql "postgres://username:password@localhost:5432/database"

    # Or test connection string directly
    psql "postgres://user:pass@localhost:5432/db?sslmode=disable"
    ```

c) **Common connection string issues:**
    - Missing `?sslmode=disable` for local development
    - Wrong port (default is 5432)
    - Wrong database name
    - Invalid username/password
    - Database not running

d) **Check PostgreSQL is running:**
    ```bash
    # macOS (Homebrew)
    brew services list | grep postgresql

    # Linux (systemd)
    systemctl status postgresql

    # Check if port 5432 is listening
    lsof -i :5432
    # or
    netstat -an | grep 5432
    ```

#### 2. Missing Environment Variables

**Required for LLM functionality:**
- `ANTHROPIC_API_KEY` - Claude API key (if using Anthropic)
- Or Ollama configuration (if using Ollama)

**Check your MCP config file:**

macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/pgedge-postgres-mcp",
      "env": {
        "ANTHROPIC_API_KEY": "sk-ant-your-key-here"
      }
    }
  }
}
```

**Important:**
- Use absolute paths (not `~` or relative paths)
- Check for typos in environment variable names
- Restart Claude Desktop after config changes
- Database connections are configured at runtime via `set_database_connection` tool

#### 3. Database Metadata Loading Issues

**Check the logs for:**
```
[pgedge-postgres-mcp] ERROR: Failed to load database metadata: ...
```

**Solutions:**

a) **Check database permissions:**
```sql
-- Your user needs permission to read system catalogs
SELECT * FROM pg_class LIMIT 1;
SELECT * FROM pg_namespace LIMIT 1;
```

b) **Verify database has tables:**
```sql
-- Check for tables in non-system schemas
SELECT schemaname, tablename
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema');
```

c) **Empty database:**
If your database is empty (no user tables), the server will still start but won't have any metadata. This is OK, you'll just need to add some tables.

## Tools Not Appearing in Claude

### Symptoms
- Server connects but tools don't appear in Claude UI
- No `query_database` or `get_schema_info` tools available

### Solutions

1. **Verify server is connected:**

    - Check Claude Desktop logs
    - Look for `[pgedge] [info] Server started and connected successfully`

2. **Restart Claude Desktop:**

    - Changes to MCP config require a full restart
    - Quit completely (not just close window)
    - Reopen Claude Desktop

3. **Check MCP config syntax:**

    ```json
    {
        "mcpServers": {
        "pgedge": {
            "command": "/full/path/to/pgedge-postgres-mcp",
            "env": {
            "ANTHROPIC_API_KEY": "..."
            }
        }
        }
    }
    ```

    - Must be valid JSON (use a JSON validator)
    - No trailing commas
    - All strings quoted

4. **Test manually:**

    ```bash
    export ANTHROPIC_API_KEY="..."
    echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ./pgedge-postgres-mcp
    # Then use set_database_connection tool to connect to database
    ```

## Natural Language Queries Not Working

### Symptoms
- `query_database` tool exists but returns errors
- Error: "ANTHROPIC_API_KEY not set"

### Solutions

1. **Set API key in config:**
   ```json
   "env": {
     "ANTHROPIC_API_KEY": "sk-ant-your-actual-key-here"
   }
   ```

2. **Get API key:**

    - Visit https://console.anthropic.com/
    - Create account or sign in
    - Go to API Keys section
    - Create new key

3. **Verify API key works:**

    ```bash
    curl https://api.anthropic.com/v1/messages \
        -H "x-api-key: $ANTHROPIC_API_KEY" \
        -H "anthropic-version: 2023-06-01" \
        -H "content-type: application/json" \
        -d '{
        "model": "claude-sonnet-4-5",
        "max_tokens": 1024,
        "messages": [{"role": "user", "content": "Hello"}]
        }'
    ```

4. **Check API credits:**

    - Ensure your Anthropic account has credits
    - Check usage at https://console.anthropic.com/

## Viewing Logs

### Claude Desktop Logs

**macOS:**
```bash
tail -f ~/Library/Logs/Claude/mcp*.log
```

**Windows:**
```
%APPDATA%\Claude\logs\
```

**Linux:**
```bash
~/.config/Claude/logs/
```

### Server Logs

All server output goes to stderr, which appears in the Claude Desktop logs with `[pgedge]` prefix.

Look for:

- `[pgedge-postgres-mcp] Starting server...` - Server startup
- `[pgedge-postgres-mcp] Database connected successfully` - DB connected
- `[pgedge-postgres-mcp] Loaded metadata for X tables/views` - Metadata loaded
- `[pgedge-postgres-mcp] Starting stdio server loop...` - Ready for requests
- `[pgedge-postgres-mcp] ERROR:` - Error messages

## SQL Generation Issues

### Symptoms

- Query returns wrong results
- Generated SQL doesn't match expectations
- SQL syntax errors

### Solutions

1. **Add database comments:**

    The quality of generated SQL depends heavily on schema comments.

    ```sql
    COMMENT ON TABLE customers IS 'Customer accounts and contact information';
    COMMENT ON COLUMN customers.status IS 'Account status: active, inactive, or suspended';
    ```

    See `example_comments.sql` for more examples.

2. **Check schema info:**

    Ask Claude: "Show me the database schema"

    This will reveal what information the LLM has about your database.

3. **Be more specific:**

    Instead of: "Show me recent data"
    Try: "Show me all orders from the last 7 days ordered by date"

4. **Review generated SQL:**

    The response includes the generated SQL. If it's wrong, you can:
    
    - Provide feedback in your next message
    - Add more schema comments
    - Rephrase your question

## Build Issues

### Go Version
Requires Go 1.21 or higher:
```bash
go version
```

### Dependency Issues
```bash
go mod tidy
go mod download
```

### Clean Build
```bash
make clean
make build
# or
go clean
go build -o pgedge-postgres-mcp
```

## Testing the Server

### Test Script
```bash
./test-connection.sh
```

### Manual Testing
```bash
# Set environment
export ANTHROPIC_API_KEY="sk-ant-..."

# Test initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./pgedge-postgres-mcp

# Test tools list (in another terminal, or after initialize response)
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./pgedge-postgres-mcp
```

## Common Error Messages

### "Failed to connect to database: connection refused"
- PostgreSQL is not running
- Wrong host/port in connection string
- Firewall blocking connection

### "Failed to connect to database: authentication failed"
- Wrong username or password
- Check pg_hba.conf for authentication rules
- Try different authentication method (trust, md5, scram-sha-256)

### "Failed to connect to database: database does not exist"
- Database name is wrong
- Database not created yet
- Check available databases: `psql -l`

### "Parse error"
- Invalid JSON in request
- Check Claude Desktop logs for the actual request sent

### "Method not found"
- Unknown MCP method
- Check protocol version compatibility
- Update server if using old version

## Getting Help

If you're still having issues:

1. **Check the logs** with timestamps and error messages
2. **Test the database connection** independently
3. **Verify environment variables** are set correctly
4. **Try the test script**: `./test-connection.sh`
5. **Check PostgreSQL logs** for connection attempts

## Debug Checklist

- [ ] PostgreSQL is running
- [ ] Can connect with psql using connection string
- [ ] ANTHROPIC_API_KEY is set in MCP config
- [ ] Database connection configured via `set_database_connection` tool
- [ ] Path to binary is absolute (not relative)
- [ ] Claude Desktop has been restarted
- [ ] Checked Claude Desktop logs for errors
- [ ] Server logs show "Starting stdio server loop..."
- [ ] ANTHROPIC_API_KEY is set (for NL queries)
- [ ] Database has at least one user table
- [ ] User has permissions to read pg_catalog
