# Authentication Guide

The Natural Language Agent includes built-in authentication with two methods: API tokens for machine-to-machine communication and user accounts for interactive authentication.

## Overview

- **API Tokens**: For machine-to-machine communication (direct HTTP/HTTPS
  access)
- **User Accounts**: For interactive authentication with session tokens
- **Enabled by default** in HTTP/HTTPS mode
- **SHA256/Bcrypt hashing** for secure credential storage
- **Token expiration** with automatic cleanup
- **Per-token connection isolation** for multi-user security
- **Bearer token authentication** using HTTP Authorization header
- **Auto-reload** of token and user files without server restart
- **Rate limiting**: Per-IP protection against brute force attacks
- **Account lockout**: Automatic account disabling after failed attempts
- **Not required** for stdio mode (Claude Desktop)

### Connection Isolation

When authentication is enabled, each API token gets its own isolated database connection pool. This provides:

- **Security**: Prevents users from accessing each other's database sessions
- **Isolation**: Temporary tables and session variables are isolated per token
- **Automatic cleanup**: Database connections are closed when tokens expire
- **Resource management**: Independent connection pools per token

See [Security Guide - Connection Isolation](security.md#connection-isolation) for more details.

## Rate Limiting and Account Lockout

The server includes built-in protection against brute force attacks through
per-IP rate limiting and automatic account lockout.

### Per-IP Rate Limiting

Failed authentication attempts are tracked per IP address to prevent brute
force attacks:

- **Default**: 10 failed attempts per 15-minute window per IP address
- **Configurable**: Customize both the time window and attempt limit
- **Automatic cleanup**: Old attempts are automatically removed from memory
- **Status-blind**: Rate limiting applies regardless of whether the username
  exists

### Account Lockout

When a valid username is provided, failed login attempts are tracked per
account:

- **Automatic lockout**: Account is disabled after N consecutive failed
  attempts
- **Configurable threshold**: Set the maximum failed attempts (default: 0 =
  disabled)
- **Reset on success**: Failed attempt counter is reset after successful login
- **Admin recovery**: Use `-enable-user` CLI command to re-enable locked
  accounts

### Configuration

Add these settings to your server configuration file:

```yaml
http:
    auth:
        enabled: true
        token_file: "./pgedge-mcp-server-tokens.yaml"
        # Rate limiting settings
        rate_limit_window_minutes: 15  # Time window for rate limiting
        rate_limit_max_attempts: 10  # Max attempts per IP per window
        # Account lockout settings
        max_failed_attempts_before_lockout: 5  # 0 = disabled
```

### Example: Enabling Account Lockout

```yaml
http:
    auth:
        enabled: true
        token_file: "./pgedge-mcp-server-tokens.yaml"
        max_failed_attempts_before_lockout: 5
        rate_limit_window_minutes: 15
        rate_limit_max_attempts: 10
```

With this configuration:

- After 5 failed login attempts, the account will be automatically disabled
- IP addresses are limited to 10 failed attempts per 15-minute window
- The server logs show when rate limiting is enabled

### Recovering Locked Accounts

```bash
# Re-enable a locked account
./bin/pgedge-mcp-server -enable-user -username alice

# Reset failed attempts counter
# (automatically reset on successful login)
```

### Environment Variables

You can also configure rate limiting via environment variables:

```bash
export PGEDGE_AUTH_MAX_FAILED_ATTEMPTS_BEFORE_LOCKOUT=5
export PGEDGE_AUTH_RATE_LIMIT_WINDOW_MINUTES=15
export PGEDGE_AUTH_RATE_LIMIT_MAX_ATTEMPTS=10
```

## User Management

User accounts provide interactive authentication with session-based access. Users authenticate with username and password to receive a 24-hour session token.

### When to Use Users vs API Tokens

- **API Tokens**: Direct machine-to-machine access, long-lived, managed by administrators
- **User Accounts**: Interactive applications, session-based, users manage own passwords

### Adding Users

#### Interactive Mode

```bash
# Add user with prompts
./bin/pgedge-mcp-server -add-user
```

You'll be prompted for:

- **Username**: Unique username for the account
- **Password**: Password (hidden, with confirmation)
- **Note**: Optional description (e.g., "Alice Smith - Developer")

#### Command Line Mode

```bash
# Add user with all details specified
./bin/pgedge-mcp-server -add-user \
  -username alice \
  -password "SecurePassword123!" \
  -user-note "Alice Smith - Developer"
```

### Listing Users

```bash
./bin/pgedge-mcp-server -list-users
```

Output:
```
Users:
==========================================================================================
Username             Created                   Last Login           Status      Annotation
------------------------------------------------------------------------------------------
alice                2024-10-30 10:15          2024-11-14 09:30     Enabled     Developer
bob                  2024-10-15 14:20          Never                Enabled     Admin
charlie              2024-09-01 08:00          2024-10-10 16:45     DISABLED    Former emp
==========================================================================================
```

### Updating Users

```bash
# Update password
./bin/pgedge-mcp-server -update-user -username alice

# Update with new password from command line (less secure)
./bin/pgedge-mcp-server -update-user \
  -username alice \
  -password "NewPassword456!"

# Update annotation only
./bin/pgedge-mcp-server -update-user \
  -username alice \
  -user-note "Alice Smith - Senior Developer"
```

### Managing User Status

```bash
# Disable a user account (prevents login)
./bin/pgedge-mcp-server -disable-user -username charlie

# Re-enable a user account
./bin/pgedge-mcp-server -enable-user -username charlie
```

### Deleting Users

```bash
# Delete user (with confirmation prompt)
./bin/pgedge-mcp-server -delete-user -username charlie
```

### Custom User File Location

```bash
# Specify custom user file path
./bin/pgedge-mcp-server -user-file /etc/pgedge/pgedge-mcp-server-users.yaml -list-users
```

### User Storage

- **Default location**: `pgedge-mcp-server-users.yaml` in the same directory as the binary
- **Storage format**: YAML with bcrypt-hashed passwords (cost factor 12)
- **File permissions**: Automatically set to 0600 (owner read/write only)
- **Session tokens**: Generated with crypto/rand (32 bytes, 24-hour validity)

## Token Management

### Adding Tokens

#### Interactive Mode

```bash
# Add token with prompts
./bin/pgedge-mcp-server -add-token
```

You'll be prompted for:

- **Note**: Description/identifier for the token (e.g., "Production API",
  "Dev Environment")
- **Database**: Which database to bind the token to (from configured databases)
- **Expiry**: Duration or "never" (e.g., "30d", "1y", "never")

When multiple databases are configured, the interactive prompt displays
available databases and lets you select by number or name. Leave blank to use
the first configured database (default).

#### Command Line Mode

```bash
# Add token with all details specified
./bin/pgedge-mcp-server -add-token \
  -token-note "Production API" \
  -token-expiry "1y"

# Add token bound to a specific database
./bin/pgedge-mcp-server -add-token \
  -token-note "Staging API" \
  -token-database "staging" \
  -token-expiry "30d"

# Add token with no expiration
./bin/pgedge-mcp-server -add-token \
  -token-note "CI/CD Pipeline" \
  -token-expiry "never"
```

**Token database binding options:**

- `-token-database <name>`: Bind token to a specific configured database
- If not specified in interactive mode: You'll be prompted to select from
  available databases
- If left blank or not specified: Token uses the first configured database
  (default)

**Important**: The generated token is **shown only once**. Save it immediately!

```
Token created successfully:
Token: O9ms9jqTfUdy-DIjvpFWeqd_yH_NEj7me0mgOnOjGdQ=
ID: token-1234567890
Note: Production API
Database: staging
Expiry: 2025-10-30 (365 days from now)
Hash (first 12 chars): b3f805a4c...

Store this token securely. It cannot be retrieved later.
```

### Listing Tokens

```bash
./bin/pgedge-mcp-server -list-tokens
```

Output:
```
API Tokens:
===========

ID: token-1234567890
Note: Production API
Hash (first 12 chars): b3f805a4c...
Created: 2024-10-30 10:15:30
Expires: 2025-10-30 10:15:30
Status: Valid

ID: token-9876543210
Note: Development
Hash (first 12 chars): 7a2f19d8e...
Created: 2024-10-15 14:20:15
Expires: Never
Status: Valid

Total tokens: 2
```

### Removing Tokens

You can remove tokens by ID or hash prefix:

```bash
# Remove by full token ID
./bin/pgedge-mcp-server -remove-token token-1234567890

# Remove by hash prefix (minimum 8 characters)
./bin/pgedge-mcp-server -remove-token b3f805a4

# Remove by partial hash (at least 8 chars)
./bin/pgedge-mcp-server -remove-token b3f805a4c2
```

## Token Expiry Formats

### Time-Based Expiry

- `30d` - 30 days
- `1y` - 1 year
- `2w` - 2 weeks
- `12h` - 12 hours
- `90d` - 90 days
- `6m` - 6 months (not directly supported, use `180d`)

### No Expiration

- `never` - Token never expires (use with caution)

### Examples

```bash
# Short-lived token for testing
./bin/pgedge-mcp-server -add-token -token-note "Test" -token-expiry "1h"

# Standard token for applications
./bin/pgedge-mcp-server -add-token -token-note "API Client" -token-expiry "90d"

# Long-lived token for services
./bin/pgedge-mcp-server -add-token -token-note "Monitoring" -token-expiry "1y"

# Permanent token (requires explicit renewal)
./bin/pgedge-mcp-server -add-token -token-note "Admin" -token-expiry "never"
```

## Using Tokens

### HTTP Authorization Header

Include the token in the `Authorization` header with the `Bearer` scheme:

```bash
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Authorization: Bearer O9ms9jqTfUdy-DIjvpFWeqd_yH_NEj7me0mgOnOjGdQ=" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list",
    "params": {}
  }'
```

### Example Requests

#### Initialize Connection

```bash
curl -X POST https://localhost:8080/mcp/v1 \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {
        "name": "my-client",
        "version": "1.0.0"
      }
    }
  }'
```

#### Execute Query

```bash
curl -X POST https://localhost:8080/mcp/v1 \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "query_database",
      "arguments": {
        "query": "Show me all active users"
      }
    }
  }'
```

### Programming Language Examples

#### Python

```python
import requests

token = "O9ms9jqTfUdy-DIjvpFWeqd_yH_NEj7me0mgOnOjGdQ="
url = "https://localhost:8080/mcp/v1"

headers = {
    "Authorization": f"Bearer {token}",
    "Content-Type": "application/json"
}

payload = {
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list",
    "params": {}
}

response = requests.post(url, json=payload, headers=headers)
print(response.json())
```

#### JavaScript/Node.js

```javascript
const token = "O9ms9jqTfUdy-DIjvpFWeqd_yH_NEj7me0mgOnOjGdQ=";
const url = "https://localhost:8080/mcp/v1";

const response = await fetch(url, {
  method: "POST",
  headers: {
    "Authorization": `Bearer ${token}`,
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    jsonrpc: "2.0",
    id: 1,
    method: "tools/list",
    params: {}
  })
});

const data = await response.json();
console.log(data);
```

#### Go

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func main() {
	token := "O9ms9jqTfUdy-DIjvpFWeqd_yH_NEj7me0mgOnOjGdQ="
	url := "https://localhost:8080/mcp/v1"

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, _ := client.Do(req)
	defer resp.Body.Close()

	// Handle response...
}
```

## Token Configuration

### Custom Token File Location

```bash
# Specify custom token file path
./bin/pgedge-mcp-server -http -token-file /etc/pgedge/pgedge-mcp-server-tokens.yaml
```

### Disable Authentication (Development Only)

**Warning**: Never use this in production!

```bash
./bin/pgedge-mcp-server -http -no-auth
```

### Configuration File

In `pgedge-mcp-server.yaml`:

```yaml
http:
  enabled: true
  address: ":8080"
  auth:
    enabled: true
    token_file: "/path/to/pgedge-mcp-server-tokens.yaml"
```

## Token Storage

### Storage Location

- **Default**: `pgedge-mcp-server-tokens.yaml` in the same directory as the binary
- **Custom**: Specified via `-token-file` flag or config file

### Storage Format

```yaml
tokens:
  - id: token-1234567890
    hash: b3f805a4c2e7d9f1a8b6c3e2d5f4a1b9c8e7d6f5a4b3c2e1d9f8a7b6c5e4d3f2
    note: Production API
    created_at: "2024-10-30T10:15:30Z"
    expires_at: "2025-10-30T10:15:30Z"
```

**Important**:

- Tokens are stored as **SHA256 hashes** (not plaintext)
- File permissions automatically set to **0600** (owner read/write only)
- Original token cannot be retrieved from the file

### Automatic Cleanup

Expired tokens are automatically removed when the server starts:

```
Loaded 3 API token(s) from pgedge-mcp-server-tokens.yaml
Removed 1 expired token(s)
```

## Automatic File Reloading

The Natural Language Agent automatically detects and reloads changes to token
and user files without requiring a server restart. This enables hot updates
to authentication credentials while the server is running.

### How It Works

The server uses file system notifications (via `fsnotify`) to monitor the
token and user files for changes. When a file is modified, the server
automatically reloads the credentials:

- **Instant updates**: Changes take effect within 100ms
- **No downtime**: Server continues running during reload
- **Thread-safe**: Uses read-write locks to prevent race conditions
- **Editor-friendly**: Handles file deletion/recreation during saves
- **Session preservation**: Active user sessions remain valid during reload
- **Debouncing**: Batches rapid file changes to avoid excessive reloads

### Technical Details

#### File Watching

The server watches the directory containing the auth files (not the files
directly) because many editors delete and recreate files when saving. This
ensures that the watcher continues working after file edits.

#### Reload Process

1. File system event detected (Write or Create)
2. Debounce timer (100ms) starts to batch rapid changes
3. Reload function executes with write lock
4. New credentials loaded from disk
5. Old credentials replaced atomically
6. Active sessions preserved (for user files)
7. Confirmation logged to server output

#### Thread Safety

All reload operations use read-write locks (`sync.RWMutex`) to ensure:

- Multiple concurrent read operations (authentication checks) can proceed
- Write operations (reloads) block all other operations temporarily
- No race conditions between authentication and reload
- Atomic replacement of credential data

### Use Cases

#### Adding Tokens While Server Runs

```bash
# Terminal 1: Server is running
./bin/pgedge-mcp-server -http

# Terminal 2: Add a new token without stopping server
./bin/pgedge-mcp-server -add-token \
  -token-note "New Client" \
  -token-expiry "30d"

# Server output shows:
# [AUTH] Reloaded /path/to/pgedge-mcp-server-tokens.yaml

# New token is immediately usable
```

#### Removing Compromised Tokens

```bash
# Server is running in production
# Security team detects compromised token

# Remove the token immediately
./bin/pgedge-mcp-server -remove-token b3f805a4

# Token is revoked within 100ms
# No server restart needed
```

#### Updating User Passwords

```bash
# Server running with active user sessions

# Update user password
./bin/pgedge-mcp-server -update-user \
  -username alice \
  -password "NewSecurePassword456!"

# Server reloads user file
# Alice's active session remains valid
# New password required for next login
```

#### Bulk Updates

```bash
# Edit token file directly for bulk changes
nano pgedge-mcp-server-tokens.yaml

# On save, server automatically detects change:
# [AUTH] Reloaded /path/to/pgedge-mcp-server-tokens.yaml
```

### Monitoring Reload Events

Server logs show reload events:

```
[AUTH] Reloaded /path/to/pgedge-mcp-server-tokens.yaml
[AUTH] Reloaded /path/to/pgedge-mcp-server-users.yaml
```

Failed reloads are also logged:

```
[AUTH] Failed to reload /path/to/pgedge-mcp-server-tokens.yaml:
permission denied
```

### Limitations

- **File must exist**: Deleting the file entirely will cause errors
- **Valid YAML required**: Syntax errors prevent reload (old data retained)
- **Same location**: Moving the file to a different path requires restart
- **No cascade**: Changing token file path in config requires restart

### Best Practices

1. **Test in development**: Verify file edits before production changes
2. **Monitor logs**: Watch for reload confirmations and errors
3. **Atomic edits**: Use tools that write atomically (most editors do)
4. **Backup first**: Keep backups before bulk edits
5. **Verify changes**: Use `-list-tokens` or `-list-users` to confirm

### Implementation

The auto-reload feature is implemented using:

- **fsnotify**: Cross-platform file system notifications
- **Watcher goroutine**: Background monitoring in separate thread
- **Debounce timer**: 100ms delay to batch rapid changes
- **RWMutex locks**: Thread-safe data structure access
- **Reload callbacks**: TokenStore.Reload() and UserStore.Reload()

For implementation details, see:

- [internal/auth/watcher.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/auth/watcher.go) - File watching
- [internal/auth/auth.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/auth/auth.go) - Token store reload
- [internal/auth/users.go](https://github.com/pgEdge/pgedge-mcp/blob/main/internal/auth/users.go) - User store reload

## Authentication Behavior

### Health Endpoint

The `/health` endpoint is **always accessible** without authentication:

```bash
# No token required
curl http://localhost:8080/health
```

### MCP Endpoint

The `/mcp/v1` endpoint **requires authentication** (unless `-no-auth` is used):

```bash
# Without token - returns 401 Unauthorized
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": {}}'

# Response:
{"error": "Unauthorized"}
```

### Error Responses

#### Missing Token

```json
{
  "error": "Unauthorized"
}
```

HTTP Status: `401 Unauthorized`

#### Invalid Token

```json
{
  "error": "Unauthorized"
}
```

HTTP Status: `401 Unauthorized`

#### Expired Token

```json
{
  "error": "Unauthorized"
}
```

HTTP Status: `401 Unauthorized`

**Note**: For security reasons, specific error details are not exposed.

## Best Practices

### 1. Token Lifecycle Management

- **Use expiration dates**: Set appropriate expiry times for all tokens
- **Rotate regularly**: Create new tokens and remove old ones periodically
- **Audit tokens**: Regularly review `list-tokens` output
- **Remove unused tokens**: Clean up tokens that are no longer needed

### 2. Token Security

- **Never commit tokens**: Don't store tokens in version control
- **Use environment variables**: For application secrets
- **Limit scope**: Use different tokens for different services/users
- **Monitor usage**: Watch logs for suspicious activity
- **Protect token file**: Ensure file permissions are 0600

### 3. Production Deployment

```bash
# Good: Short-lived tokens with rotation
./bin/pgedge-mcp-server -add-token \
  -token-note "Web App - Q4 2024" \
  -token-expiry "90d"

# Bad: Never-expiring tokens
./bin/pgedge-mcp-server -add-token \
  -token-note "Web App" \
  -token-expiry "never"
```

### 4. Development vs Production

**Development**:
```bash
# Use -no-auth for local development (localhost only)
./bin/pgedge-mcp-server -http -addr "localhost:8080" -no-auth
```

**Production**:
```bash
# Always use authentication with HTTPS
./bin/pgedge-mcp-server -http -tls \
  -cert /path/to/cert.pem \
  -key /path/to/key.pem
```

### 5. Token Distribution

When distributing tokens to users/services:

1. **Secure channels**: Use encrypted communication
2. **One-time display**: Show token only once during creation
3. **Documentation**: Include expiry date and renewal process
4. **Revocation plan**: Document how to remove compromised tokens

## User Authentication Flow

For interactive applications using user accounts, authentication follows a two-step process:

### Step 1: Authenticate User

Call the `authenticate_user` tool (this tool is NOT advertised to the LLM and is only for direct client use):

```bash
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "authenticate_user",
      "arguments": {
        "username": "alice",
        "password": "SecurePassword123!"
      }
    }
  }'
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"success\": true, \"session_token\": \"AQz9XfK...\", \"expires_at\": \"2024-11-15T09:30:00Z\", \"message\": \"Authentication successful\"}"
      }
    ]
  }
}
```

### Step 2: Use Session Token

Extract the `session_token` from the response and use it as a Bearer token for all subsequent requests:

```bash
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Authorization: Bearer AQz9XfK..." \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "query_database",
      "arguments": {
        "query": "Show me all users"
      }
    }
  }'
```

### Session Token Properties

- **Validity**: 24 hours from authentication
- **Format**: Base64-encoded random 32-byte token
- **Security**: Strongly random, cryptographically secure
- **Expiration**: Tokens expire automatically after 24 hours
- **Renewal**: Re-authenticate to get a new session token

### Client Implementation Example

#### Python Client

```python
import requests
import json

class MCPUserClient:
    def __init__(self, base_url):
        self.base_url = base_url
        self.session_token = None
        self.token_expiry = None

    def authenticate(self, username, password):
        """Authenticate and get session token"""
        response = requests.post(
            f"{self.base_url}/mcp/v1",
            json={
                "jsonrpc": "2.0",
                "id": 1,
                "method": "tools/call",
                "params": {
                    "name": "authenticate_user",
                    "arguments": {
                        "username": username,
                        "password": password
                    }
                }
            }
        )

        result = response.json()
        if "result" in result:
            auth_data = json.loads(result["result"]["content"][0]["text"])
            self.session_token = auth_data["session_token"]
            self.token_expiry = auth_data["expires_at"]
            return True
        return False

    def call_tool(self, tool_name, arguments):
        """Call a tool using the session token"""
        if not self.session_token:
            raise Exception("Not authenticated")

        response = requests.post(
            f"{self.base_url}/mcp/v1",
            headers={
                "Authorization": f"Bearer {self.session_token}",
                "Content-Type": "application/json"
            },
            json={
                "jsonrpc": "2.0",
                "id": 2,
                "method": "tools/call",
                "params": {
                    "name": tool_name,
                    "arguments": arguments
                }
            }
        )
        return response.json()

# Usage
client = MCPUserClient("http://localhost:8080")
if client.authenticate("alice", "SecurePassword123!"):
    result = client.call_tool("query_database", {"query": "Show tables"})
    print(result)
```

### Authentication Errors

#### Invalid Credentials

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

#### Disabled User Account

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

#### Expired Session Token

```json
{
  "error": "Unauthorized"
}
```

HTTP Status: `401 Unauthorized`

**Solution**: Re-authenticate to get a new session token

## User Authentication Best Practices

### 1. Password Security

- **Strong passwords**: Enforce minimum complexity requirements
- **Never log passwords**: Ensure passwords are not logged or displayed
- **Secure transmission**: Always use HTTPS in production
- **Password updates**: Regularly prompt users to update passwords

### 2. Session Management

- **Token storage**: Store session tokens securely (not in localStorage for web apps)
- **Token refresh**: Re-authenticate before token expiration
- **Logout**: Implement proper logout (client-side token deletion)
- **Concurrent sessions**: Consider implementing session limits per user

### 3. Account Security

- **Account lockout**: Configure `max_failed_attempts_before_lockout` to
  automatically disable accounts after repeated failed login attempts (see
  [Rate Limiting and Account Lockout](#rate-limiting-and-account-lockout))
- **Audit logging**: Log authentication events (success and failures)
- **Inactive accounts**: Disable accounts after period of inactivity
- **Role-based access**: Use annotations to track user roles/permissions

### 4. Integration with Applications

```python
# Good: Check token expiration before use
from datetime import datetime, timezone

def is_token_expired(expiry_str):
    expiry = datetime.fromisoformat(expiry_str.replace('Z', '+00:00'))
    return datetime.now(timezone.utc) >= expiry

if not client.session_token or is_token_expired(client.token_expiry):
    client.authenticate(username, password)

# Now use the token
result = client.call_tool("query_database", {...})
```

## Troubleshooting

### Token File Not Found

```bash
# Error message:
ERROR: Token file not found: /path/to/pgedge-mcp-server-tokens.yaml
Create tokens with: ./pgedge-mcp-server -add-token
Or disable authentication with: -no-auth
```

**Solution**:
```bash
# Create first token
./bin/pgedge-mcp-server -add-token
```

### Token Authentication Fails

```bash
# Check token file exists and has correct permissions
ls -la pgedge-mcp-server-tokens.yaml  # Should show -rw------- (600)

# List tokens to verify token exists
./bin/pgedge-mcp-server -list-tokens

# Check for expired tokens
./bin/pgedge-mcp-server -list-tokens | grep "Status: Expired"
```

### Cannot Remove Token

```bash
# Error: Token not found
# Solution: Use at least 8 characters of the hash
./bin/pgedge-mcp-server -list-tokens  # Get the hash
./bin/pgedge-mcp-server -remove-token b3f805a4  # Use 8+ chars
```

### Server Won't Start (Auth Enabled)

If auth is enabled but no token file exists:

```bash
# Option 1: Create a token file
./bin/pgedge-mcp-server -add-token

# Option 2: Disable auth temporarily
./bin/pgedge-mcp-server -http -no-auth
```

## Security Considerations

See the [Security Guide](security.md) for comprehensive security best practices including:

- Token storage and protection
- HTTPS requirements
- Network security
- Audit logging
- Incident response
