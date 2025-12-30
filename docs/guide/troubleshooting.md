# Troubleshooting Guide

Before seeking additional help, you should confirm that the following items are configured correctly:

- [ ] PostgreSQL is running on the system.
- [ ] You can connect with psql using the connection string.
- [ ] The ANTHROPIC_API_KEY is set in the MCP config.
- [ ] The database connection is configured at server startup using environment variables, a config file, or command-line flags.
- [ ] The path to the binary is absolute and not relative.
- [ ] Claude Desktop has been restarted after configuration changes.
- [ ] You have checked the Claude Desktop logs for errors.
- [ ] The server logs show the "Starting stdio server loop..." message.
- [ ] The ANTHROPIC_API_KEY is set for natural language queries.
- [ ] The database has at least one user table.
- [ ] Your user has permissions to read the pg_catalog schema.


If you are still experiencing issues after trying the solutions in this guide, you should follow these steps to gather diagnostic information:

- You should check the logs with timestamps and error messages to understand what is failing.
- You should test the database connection independently using psql or another tool.
- You should verify that all environment variables are set correctly in your configuration.
- You can try running the test script using the command `./test-connection.sh`.
- You should check the PostgreSQL logs for connection attempts and errors.


## Troubleshooting Configuration Issues

This section provides solutions for common configuration file issues.

### Configuration Not Loading

If the configuration file is not loading properly, you can verify the file exists and check permissions. In the following example, the `ls` command checks for the configuration file, the server is started with an explicit path, and the `chmod` command verifies that the file has the correct permissions.

```bash
# Check if config file exists
ls -la bin/pgedge-postgres-mcp.yaml

# Use explicit path
./bin/pgedge-postgres-mcp -config /full/path/to/config.yaml

# Check file permissions
chmod 600 bin/pgedge-postgres-mcp.yaml  # Should be readable
```


## Troubleshooting Build Issues

When building and deploying the MCP server and agent, you might encounter several common issues. This section provides solutions for port conflicts, database connectivity problems, Docker networking, and certificate validation.

### Port Already in Use

If the server fails to start because the port is already in use, you can identify the process. In the following example, the `lsof` command shows which process is using the port; you can then kill the process or use a different port with the `-addr` flag.

```bash
lsof -i :8080
# Kill the process or use a different port with -addr
```

### Database Connection Failed

If the database connection fails during build or deployment, you can test the connection independently. In the following example, the `psql` command tests a direct connection to the database, and the `env` command displays PostgreSQL-related environment variables.

```bash
# Test connection directly
psql -h localhost -U postgres -d mydb -c "SELECT 1"

# Check environment variables
env | grep PG
```

### Docker Can't Reach Host Database

If Docker containers cannot reach the host database, the connection string depends on the operating system. On macOS and Windows, you should use `host.docker.internal` as the hostname. On Linux, you should use `172.17.0.1` or configure a Docker network bridge.

### Certificate Issues

If you encounter SSL certificate errors, you can verify that the certificate matches the key. In the following example, the `openssl` commands generate an MD5 hash for both the certificate and the key; the hashes should match. The second command checks the certificate expiration date.

```bash
# Verify certificate matches key
openssl x509 -noout -modulus -in server.crt | openssl md5
openssl rsa -noout -modulus -in server.key | openssl md5
# Both should match

# Check expiration
openssl x509 -in server.crt -noout -dates
```


## Troubleshooting Web Client Issues

This section addresses common issues when using the web client interface.

### Connection Issues

If you see a red connection indicator in the web client, you should check the following items:

- The MCP server is running.
- The database credentials in the server configuration are correct.
- The network connectivity to the database host is working properly.

### Slow Responses

If the web client experiences slow response times, you can try the following solutions:

- You can try a faster model such as `claude-sonnet` instead of `claude-opus`.
- You can enable response streaming in the server configuration.
- You should check your LLM provider's rate limits to ensure you are not being throttled.

### Authentication Errors

If you encounter authentication errors in the web client, you should verify the following:

- Your username and password are correct.
- The user exists; you can use the `-list-users` flag on the server to verify.
- Authentication is enabled in the server configuration.


## Troubleshooting CLI Client Issues

This section provides solutions for common issues encountered when using the CLI client.

### Connection Errors

If you see the error "Failed to connect to MCP server", you should verify the following items:

- In stdio mode, the server path is correct; use `-mcp-server-path ./bin/pgedge-postgres-mcp`.
- In HTTP mode, the URL is correct; use `-mcp-url http://localhost:8080`.
- The MCP server is running when using HTTP mode.
- The authentication token is set when using HTTP mode with authentication enabled.

### LLM Errors

If you see the error "LLM error: authentication failed", you should check the following items:

- For Anthropic, you should verify that the `ANTHROPIC_API_KEY` environment variable is set correctly.
- For Ollama, you should verify that Ollama is running using `ollama serve` and that the model is pulled using `ollama pull llama3`.
- The model name is correct in your configuration.

If you see the error "Ollama: model not found", you need to pull the required model. In the following example, the `ollama list` command displays available models, and the `ollama pull` command downloads the model you want to use.

```bash
# List available models
ollama list

# Pull the model you want to use
ollama pull llama3
```

### Configuration Issues

If you see the error "Configuration error: invalid mode", you should verify the following:

- The valid modes are `stdio` or `http`.
- Your configuration file or command-line flags are correct.
- The mode must be specified if you are not using the default mode.

If you see the error "Missing API key for Anthropic", you have several options to resolve the issue:

- You can set the `PGEDGE_ANTHROPIC_API_KEY` environment variable.
- You can add the `anthropic_api_key` to your configuration file under the `llm:` section.
- You can use the `-anthropic-api-key` command-line flag when starting the client.

### Terminal and Display Issues

If colors look wrong or garbled in the terminal, you can disable color output using one of the following methods:

- You can disable colors with the `NO_COLOR=1` environment variable.
- You can use the `-no-color` command-line flag.
- You can add `no_color: true` to your configuration file under the `ui:` section.

If command history is not working properly, you should check the following items:

- The `~/.pgedge-nla-cli-history` file is writable.
- The history file is created automatically on first use.
- On some terminals, the readline features may be limited.


## Troubleshooting Authentication Errors

This section describes common authentication error responses and their solutions.

### Invalid Credentials

If you provide incorrect credentials, the server returns an authentication failed error. In the following example, the JSON-RPC response shows a tool execution error with a message indicating invalid username or password.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32603,
    "message": "Tool execution error",
    "data": "authentication failed: invalid username or password"
  }
}
```

### Disabled User Account

If you attempt to authenticate with a disabled user account, the server returns an error. In the following example, the JSON-RPC response indicates that the user account is disabled.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32603,
    "message": "Tool execution error",
    "data": "authentication failed: user account is disabled"
  }
}
```

### Expired Session Token

If your session token has expired, the server returns an unauthorized error with HTTP status 401. In the following example, the JSON response shows an "Unauthorized" error message.

```json
{
  "error": "Unauthorized"
}
```

You should re-authenticate to obtain a new session token.


## Troubleshooting Token Management

This section addresses issues related to authentication tokens and token file management.

### Token File Not Found

If the server cannot find the token file, you will see an error message indicating the file path and suggesting solutions. In the following example, the error message shows the expected token file path and provides commands to create a token or disable authentication.

```bash
# Error message:
ERROR: Token file not found: /path/to/pgedge-postgres-mcp-tokens.yaml
Create tokens with: ./pgedge-postgres-mcp -add-token
Or disable authentication with: -no-auth
```

You can create a new token file using the following command:

```bash
# Create first token
./bin/pgedge-postgres-mcp -add-token
```

### Token/User Files in Unexpected Location

If you find token or user files being created in unexpected locations (such
as `/usr/bin/`), this is due to the default path fallback behavior.

**How default paths work:**

1. The server first checks for system paths:
    - Tokens: `/etc/pgedge/postgres-mcp/pgedge-postgres-mcp-tokens.yaml`
    - Users: `/etc/pgedge/pgedge-postgres-mcp-users.yaml`
2. If these don't exist, it falls back to the binary directory

If your binary is installed in `/usr/bin/`, files will be created there
unless you:

- Create the `/etc/pgedge/` directory structure
- Use a custom path via `-token-file` or `-user-file` flags
- Specify paths in your configuration file
- Set `PGEDGE_AUTH_TOKEN_FILE` environment variable

**Solution for production deployments:**

```bash
# Create system directories
sudo mkdir -p /etc/pgedge/postgres-mcp
sudo chown $USER:$USER /etc/pgedge /etc/pgedge/postgres-mcp
```

### Tokens Created But Authentication Fails

If you create tokens successfully but the server fails to find them, you may
be using different file paths for token management and server startup.

**Cause:** Each command invocation is independent. The server does not
"remember" custom paths used in previous commands.

**Example of the problem:**

```bash
# Create token at custom path
./bin/pgedge-postgres-mcp -token-file /custom/path/tokens.yaml -add-token

# Start server WITHOUT specifying the custom path - FAILS
./bin/pgedge-postgres-mcp -http
# ERROR: Token file not found: /default/path/tokens.yaml
```

**Solution:** Use the same path consistently:

```bash
# Create token
./bin/pgedge-postgres-mcp -token-file /custom/path/tokens.yaml -add-token

# Start server with SAME path
./bin/pgedge-postgres-mcp -http -token-file /custom/path/tokens.yaml
```

Or better, specify the path in your configuration file:

```yaml
http:
  auth:
    token_file: "/custom/path/tokens.yaml"
```

### Token Authentication Fails

If token authentication fails, you should verify that the token file exists and has the correct permissions. In the following example, the commands check the token file permissions, list available tokens, and identify expired tokens.

```bash
# Check token file exists and has correct permissions
ls -la pgedge-postgres-mcp-tokens.yaml  # Should show -rw------- (600)

# List tokens to verify token exists
./bin/pgedge-postgres-mcp -list-tokens

# Check for expired tokens
./bin/pgedge-postgres-mcp -list-tokens | grep "Status: Expired"
```

### Cannot Remove Token

If you cannot remove a token because the hash is not found, you need to use at least 8 characters of the hash. In the following example, the commands list the available tokens to get the full hash, then remove the token using at least 8 characters from the hash.

```bash
# Error: Token not found
# Solution: Use at least 8 characters of the hash
./bin/pgedge-postgres-mcp -list-tokens  # Get the hash
./bin/pgedge-postgres-mcp -remove-token b3f805a4  # Use 8+ chars
```

### Server Won't Start with Authentication Enabled

If authentication is enabled but no token file exists, the server will not start. In the following example, you can either create a token file or disable authentication temporarily.

```bash
# Option 1: Create a token file
./bin/pgedge-postgres-mcp -add-token

# Option 2: Disable auth temporarily
./bin/pgedge-postgres-mcp -http -no-auth
```

### Server Exits Immediately After Initialize

If the server exits immediately after responding to the initialize request, you may see specific symptoms and should investigate several common causes.

### Symptoms

The following symptoms indicate that the server is exiting unexpectedly:

- The Claude Desktop logs show the message "Server transport closed unexpectedly".
- The server starts but disconnects immediately after sending the `initialize` response.


## Database Connection Issues

Database connection problems are a common cause of server exits. You should check the logs for connection errors.

If you see an error message like `[pgedge-postgres-mcp] ERROR: Failed to connect to database: ...`, you should investigate the following issues:

**Verify the connection string format.** The connection string must follow the correct PostgreSQL format. In the following example, the comments show the correct format and provide sample connection strings for different authentication scenarios.

```bash
# Correct format:
postgres://username:password@host:port/database?sslmode=disable

# Examples:
postgres://postgres:mypassword@localhost:5432/mydb?sslmode=disable
postgres://user@localhost/dbname?sslmode=disable  # local trusted auth
```

**Test the PostgreSQL connection directly.** You can verify that the connection string works by testing the connection with psql. In the following example, the `psql` commands test the connection using the full connection string.

```bash
# Using psql
psql "postgres://username:password@localhost:5432/database"

# Or test connection string directly
psql "postgres://user:pass@localhost:5432/db?sslmode=disable"
```

**Check for common connection string issues.** The following items are common problems with connection strings:

- The `?sslmode=disable` parameter is missing for local development.
- The port is wrong; the default PostgreSQL port is 5432.
- The database name is incorrect.
- The username or password is invalid.
- The database is not running.

**Verify that PostgreSQL is running.** You can check if the PostgreSQL service is running on your system. In the following example, the commands check the service status on macOS using Homebrew, on Linux using systemd, and verify that port 5432 is listening.

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

### Missing Environment Variables

If environment variables are missing, the server may fail to start or lack LLM functionality. The following environment variables are required depending on your configuration:

- The `ANTHROPIC_API_KEY` variable is required if you are using Anthropic for Claude API access.
- An Ollama configuration is required if you are using Ollama instead of Anthropic.

You should check your MCP configuration file to ensure the environment variables are set correctly. On macOS, the configuration file is located at `~/Library/Application Support/Claude/claude_desktop_config.json`. In the following example, the JSON configuration shows how to set the API key and specify the server command using an absolute path.

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/absolute/path/to/bin/pgedge-postgres-mcp",
      "env": {
        "ANTHROPIC_API_KEY": "sk-ant-your-key-here"
      }
    }
  }
}
```

When configuring the environment, you should keep the following points in mind:

- You must use absolute paths; relative paths and the `~` shortcut are not supported.
- You should check for typos in environment variable names.
- You must restart Claude Desktop after making configuration changes.
- Database connections are configured at server startup via environment variables, a config file, or command-line flags.

### Database Metadata Loading Issues

If the server fails to load database metadata, you should check the logs for error messages. An error message like `[pgedge-postgres-mcp] ERROR: Failed to load database metadata: ...` indicates a metadata loading problem.

You should verify the following items to resolve metadata loading issues:

**Check database permissions.** Your user account needs permission to read the system catalogs. In the following example, the SQL queries test whether you can read from the `pg_class` and `pg_namespace` system tables.

```sql
-- Your user needs permission to read system catalogs
SELECT * FROM pg_class LIMIT 1;
SELECT * FROM pg_namespace LIMIT 1;
```

**Verify that the database has tables.** The database should contain user tables in non-system schemas. In the following example, the SQL query retrieves a list of tables from schemas that are not system schemas.

```sql
-- Check for tables in non-system schemas
SELECT schemaname, tablename
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema');
```

**Handle empty databases.** If your database is empty and contains no user tables, the server will still start but will not have any metadata. This behavior is expected; you will need to add tables to the database before the metadata becomes available.

## Tools Not Appearing in Claude

This section provides solutions for issues where the server connects but the tools do not appear in the Claude interface.

### Symptoms

The following symptoms indicate that tools are not appearing correctly:

- The server connects but the tools do not appear in the Claude UI.
- The `query_database` or `get_schema_info` tools are not available.

### Solutions

You should try the following solutions to resolve this issue:

**Verify that the server is connected.** You should check the Claude Desktop logs for connection status. Look for a message like `[pgedge] [info] Server started and connected successfully` to confirm the connection.

**Restart Claude Desktop.** Changes to the MCP configuration require a full restart of the application. You should quit Claude Desktop completely rather than just closing the window, then reopen the application.

**Check the MCP config syntax.** The configuration file must be valid JSON. In the following example, the JSON configuration shows the required format with the server command and environment variables.

```json
{
    "mcpServers": {
    "pgedge": {
        "command": "/full/path/to/bin/pgedge-postgres-mcp",
        "env": {
        "ANTHROPIC_API_KEY": "..."
        }
    }
    }
}
```

You should verify the following items in your configuration:

- The configuration must be valid JSON; you can use a JSON validator to check.
- There should be no trailing commas in the JSON structure.
- All strings must be quoted properly.

**Test the server manually.** You can test the server manually to verify that the tools are available. In the following example, the commands set the API key, configure the database connection, and send a JSON-RPC request to list available tools.

```bash
export ANTHROPIC_API_KEY="..."
# Configure database connection via environment variables or config file before running
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ./bin/pgedge-postgres-mcp
```

## Natural Language Queries Not Working

This section addresses issues where the natural language query functionality is not working correctly.

### Symptoms

The following symptoms indicate problems with natural language queries:

- The `query_database` tool exists but returns errors when called.
- You see an error message stating "ANTHROPIC_API_KEY not set".

### Solutions

You should try the following solutions to resolve natural language query issues:

**Set the API key in your configuration.** You need to add the API key to the environment section of your MCP configuration. In the following example, the JSON snippet shows how to set the `ANTHROPIC_API_KEY` environment variable.

```json
"env": {
  "ANTHROPIC_API_KEY": "sk-ant-your-actual-key-here"
}
```

**Obtain an API key.** If you do not have an API key, you can create one by visiting the Anthropic console. You should visit https://console.anthropic.com/ and create an account or sign in. You can then go to the API Keys section and create a new key.

**Verify that the API key works.** You can test the API key to ensure that the API key is valid and that your account has access. In the following example, the `curl` command sends a test request to the Anthropic API to verify the key.

```bash
curl https://api.anthropic.com/v1/messages \
    -H "x-api-key: $PGEDGE_ANTHROPIC_API_KEY" \
    -H "anthropic-version: 2023-06-01" \
    -H "content-type: application/json" \
    -d '{
    "model": "claude-sonnet-4-5",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello"}]
    }'
```

**Check your API credits.** You should ensure that your Anthropic account has available credits. You can check your usage and credit balance at https://console.anthropic.com/.



## SQL Generation Issues

This section provides solutions for problems with automatically generated SQL queries.

### Symptoms

The following symptoms indicate issues with SQL generation:

- A query returns wrong results that do not match your expectations.
- The generated SQL does not match your expectations or intent.
- The generated SQL contains syntax errors.

### Solutions

You should try the following solutions to improve SQL generation quality:

**Add database comments to improve SQL generation.** The quality of generated SQL depends heavily on the presence of schema comments. In the following example, the SQL commands add comments to a table and a column to provide context for the LLM.

```sql
COMMENT ON TABLE customers IS 'Customer accounts and contact information';
COMMENT ON COLUMN customers.status IS 'Account status: active, inactive, or suspended';
```

You can see more examples of effective comments in the `example_comments.sql` file.

**Check the schema information.** You can ask Claude to show you the database schema by sending the message "Show me the database schema". This command will reveal what information the LLM has about your database structure.

**Be more specific in your queries.** Vague requests may produce incorrect results. Instead of asking "Show me recent data", you should try a more specific request like "Show me all orders from the last 7 days ordered by date".

**Review the generated SQL and provide feedback.** The response includes the generated SQL query. If the SQL is wrong, you have several options:

- You can provide feedback in your next message to refine the query.
- You can add more schema comments to provide additional context.
- You can rephrase your question to be more specific.

## Build Issues

This section addresses common problems encountered when building the MCP server from source.

### Go Version Requirements

The project requires Go version 1.21 or higher. You can check your Go version using the following command:

```bash
go version
```

### Dependency Issues

If you encounter dependency problems, you can resolve them by updating and downloading the required modules. In the following example, the commands tidy the module dependencies and download all required packages.

```bash
go mod tidy
go mod download
```

### Clean Build

If the build fails or produces unexpected results, you can perform a clean build. In the following example, the commands remove previous build artifacts and rebuild the project from scratch.

```bash
make clean
make build
# or
go clean
go build -o bin/pgedge-postgres-mcp ./cmd/pgedge-pg-mcp-svr
```

## Testing the Server

This section explains how to test the MCP server to verify that the server is working correctly.

### Using the Test Script

You can run the provided test script to verify basic server functionality. In the following example, the command runs the test connection script.

```bash
./test-connection.sh
```

### Manual Testing

You can manually test the server by sending JSON-RPC requests. In the following example, the commands set the API key environment variable, send an initialize request to the server, and then send a tools list request.

```bash
# Set environment
export ANTHROPIC_API_KEY="sk-ant-..."

# Test initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/pgedge-postgres-mcp

# Test tools list (in another terminal, or after initialize response)
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./bin/pgedge-postgres-mcp
```

## Common Error Messages

This section describes common error messages and their typical causes.

### "Failed to connect to database: connection refused"

This error indicates that the database connection was refused. The following issues may cause this error:

- PostgreSQL is not running on the target system.
- The host or port in the connection string is incorrect.
- A firewall is blocking the connection to the database.

### "Failed to connect to database: authentication failed"

This error indicates that authentication to the database failed. The following issues may cause this error:

- The username or password in the connection string is incorrect.
- The `pg_hba.conf` file has authentication rules that prevent the connection.
- You should try a different authentication method such as trust, md5, or scram-sha-256.

### "Failed to connect to database: database does not exist"

This error indicates that the specified database does not exist. The following issues may cause this error:

- The database name in the connection string is wrong.
- The database has not been created yet.
- You can check the available databases using the command `psql -l`.

### "Parse error"

This error indicates that the JSON in a request is invalid. You should check the Claude Desktop logs to see the actual request that was sent to the server.

### "Method not found"

This error indicates that an unknown MCP method was called. The following issues may cause this error:

- The method name is unknown to the server.
- The protocol version may not be compatible with the server version.
- You should update the server if you are using an old version.

## Embedding Generation Issues

This section addresses problems with the embedding generation feature.

### Symptoms

The following symptoms indicate issues with embedding generation:

- The `generate_embedding` tool is not available.
- Embedding generation returns errors when called.
- You receive rate limit errors from the Anthropic API.
- You are experiencing high embedding API costs.

### Solutions

#### Enable Embedding Logging

To understand embedding API usage and debug rate limits, you can enable structured logging. In the following example, the commands set different log levels and run the server with logging enabled.

```bash
# Set log level
export PGEDGE_LLM_LOG_LEVEL="info"    # Basic info: API calls, errors
export PGEDGE_LLM_LOG_LEVEL="debug"   # Detailed: text length, dimensions, timing
export PGEDGE_LLM_LOG_LEVEL="trace"   # Very detailed: full request/response

# Run the server
./bin/pgedge-postgres-mcp
```

The log output will show detailed information about embedding operations. In the following example, the log messages show provider initialization, successful API calls, and rate limit errors.

```
[LLM] [INFO] Provider initialized: provider=ollama, model=nomic-embed-text, base_url=http://localhost:11434
[LLM] [INFO] API call succeeded: provider=ollama, model=nomic-embed-text, text_length=245, dimensions=768, duration=156ms
[LLM] [INFO] RATE LIMIT ERROR: provider=anthropic, model=voyage-3-lite, status_code=429, response={"error":"rate_limit_error"...}
```

The logging helps you identify the following information:

- The number of embedding API calls being made.
- The text length being embedded, which affects the cost.
- The API response times for performance monitoring.
- The rate limit errors with full details for debugging.

#### Embedding Generation Not Enabled

If you see the error "Embedding generation is not enabled", you need to enable the feature in the configuration file. In the following example, the YAML configuration enables embedding generation and specifies the provider and model.

```yaml
embedding:
  enabled: true
  provider: "ollama"  # or "anthropic"
  model: "nomic-embed-text"
```

#### Ollama Connection Issues

If you see the error "Failed to connect to Ollama", you should verify that Ollama is running and the model is available. In the following example, the commands check if Ollama is running, start the service if needed, and pull the embedding model.

```bash
# Verify Ollama is running
curl http://localhost:11434/api/tags

# Start Ollama if not running
ollama serve

# Pull embedding model if needed
ollama pull nomic-embed-text
```

#### Anthropic Rate Limit Errors

If you see the error "API error 429: rate_limit_error", you are exceeding the API rate limits. You can resolve this issue in several ways:

**Check your API usage.** You should visit https://console.anthropic.com/settings/usage to review your rate limits and current usage levels.

**Switch to Ollama for development.** Ollama provides free local embeddings with no rate limits. In the following example, the YAML configuration switches the embedding provider to Ollama.

```yaml
embedding:
  enabled: true
  provider: "ollama"  # Free, local, no rate limits
  model: "nomic-embed-text"
  ollama_url: "http://localhost:11434"
```

**Use embedding logging to identify high usage.** You can enable logging to understand which operations are generating embeddings. In the following example, the commands enable info-level logging and run the server.

```bash
export PGEDGE_LLM_LOG_LEVEL="info"
./bin/pgedge-postgres-mcp
```

You should review the logs to see the following information:

- The operations that are generating embeddings.
- The amount of text being embedded.
- The frequency of embedding generation requests.

#### Invalid API Key

If you see the error "API request failed with status 401", the API key is invalid or missing. You should verify that the API key is correct and properly configured.

You can set the API key using an environment variable as shown in the following example:

```bash
export PGEDGE_ANTHROPIC_API_KEY="sk-ant-your-key-here"
```

You can also set the API key in the configuration file as shown in the following example:

```yaml
embedding:
  anthropic_api_key: "sk-ant-your-key-here"
```

#### Model Not Found

If you see the Ollama error "Model not found", you need to pull the required model. In the following example, the commands list available models and pull the required embedding model.

```bash
# List available models
ollama list

# Pull the required model
ollama pull nomic-embed-text
```

If you see the Anthropic error "Unknown model", you should check the model name in your configuration. The following models are supported:

- The `voyage-3-lite` model provides 512 dimensions.
- The `voyage-3` model provides 1024 dimensions.
- The `voyage-2` model provides 1024 dimensions.
- The `voyage-2-lite` model provides 1024 dimensions.

#### Dimension Mismatch in Semantic Search

If you see the error "Query vector dimensions (768) don't match column dimensions (1536)", you are using different embedding models for document storage and query generation. You should use the same embedding model and dimensions for both operations.

You should check your document embeddings dimensions first. In the following example, the YAML configuration specifies the Ollama provider with the nomic-embed-text model that produces 768 dimensions.

```yaml
# Match the model used for your documents
embedding:
  enabled: true
  provider: "ollama"
  model: "nomic-embed-text"  # 768 dimensions
```

## Troubleshooting Knowledgebase

Common issues and their solutions are listed below.

### No Results Found

**Cause**: Query may be too specific or use terminology not in the
documentation.

**Solution**: Try broader search terms or rephrase the query.

### Wrong Project Results

**Cause**: Not filtering by project name.

**Solution**: Add `project_name` parameter to filter results.

### Embedding Provider Mismatch

**Cause**: Server embedding provider differs from the one used to build the
database.

**Solution**: Configure the server to use the same embedding provider. The
database contains embeddings from multiple providers - the server will
automatically use the one that matches its configuration.

### Knowledgebase Not Available

**Cause**: Knowledgebase not enabled in configuration or database file missing.

**Solution**: Check server configuration and verify `database_path` points to a
valid knowledgebase database file.

## Comparison with similarity_search

| feature | search_knowledgebase | similarity_search |
|---------|---------------------|-------------------|
| **data source** | pre-built documentation | user's postgresql tables |
| **use case** | technical documentation | user's own data |
| **setup** | requires kb database | requires vector columns |
| **updates** | static (rebuild needed) | dynamic (live data) |
| **scope** | curated content | any table data |


## Troubleshooting Custom Definitions

Common issues and their solutions are listed below.

### File Not Loading

If the server logs an error about a missing file, check that the:

- File path is absolute or relative to server working directory.
- File exists and is readable.
- File extension is `.json`, `.yaml`, or `.yml`.

### Validation Errors

If the server exits with validation error:

- Check the error message for a specific issue.
- Verify all required fields are present.
- Ensure names/URIs are unique.
- Confirm template placeholders reference defined arguments.

### SQL Errors

If a resource returns SQL error:

- Test the query directly in psql.
- Check table and column names.
- Verify the user has necessary permissions.
- Ensure query syntax is valid for your PostgreSQL version.

### Template Not Interpolating

If you are seeing a literal `{{arg_name}}` in output:

- Verify the argument is declared in `arguments` section.
- Check the argument name matches exactly (case-sensitive).
- Ensure you passed the argument when calling the prompt.


## Troubleshooting Queries

### Query Returns Unexpected Results

Try asking Claude to show the generated SQL:
```
"Show me users created today and display the SQL query"
```

### Connection Errors

If a connection fails, verify:

1. Database is accessible from your machine
2. Credentials are correct
3. Firewall rules allow connections
4. SSL settings match server requirements

### Slow Queries

For queries taking too long:

1. Check database indexes
2. Use read replicas for analytics
3. Limit result sets: "Show me top 100 users"


## Troubleshooting Prompts

### Prompt Not Found

**Error**: "Prompt 'prompt-name' not found"

**Solutions**:

* Verify the prompt name using `/prompts` (CLI) or the prompt dropdown (Web UI).
* Check for typos in the prompt name.
* Ensure the server is running the latest version.

### Missing Required Argument

**Error**: "Missing required argument: argument_name"

**Solutions**:

* Check the prompt's required arguments using `/prompts`.
* Provide all required arguments in the command.
* Use quotes around values with spaces.

### Invalid Argument Format

**Error**: "Invalid argument format: ... (expected key=value)"

**Solutions**:

* Use `key=value` format for all arguments.
* Quote values containing spaces: `key="value with spaces"`.
* Don't use spaces around the `=` sign.

### Rate Limit Exceeded

**Error**: "Rate limit reached for ..."

**Solutions**:

* Wait 60 seconds before retrying.
* Use more targeted queries with WHERE clauses.
* Reduce `max_output_tokens` in similarity_search.
* Use `limit` parameter in queries.
* Conversation history is automatically compacted to help prevent this.