# Authentication Guide

The MCP server includes built-in authentication with two methods: API tokens
for machine-to-machine communication and user accounts for interactive
authentication.

- Use an [*API Token*](auth_token.md) for direct machine-to-machine access.  Tokens are long-lived and easily managed by administrators.
- Use a [*User Account*](auth_user.md) for interactive applications; an account is session-based, and users can manage own password access.

- **API Tokens**: For machine-to-machine communication (direct HTTP/HTTPS access)
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

When configuring authentication:

* test your authentication in development, and verify file edits before any production changes.
* monitor your logfiles, watching for reload confirmations and errors.
* use tools that write atomically (most editors do) so you don't lose edits.
* keep backups before making any major changes or bulk edits.
* use `-list-tokens` or `-list-users` to confirm that authentication changes are performing as expected.

For help resolving an authentication issue, visit the [Troubleshooting](troubleshooting.md#authentication-errors) page.

Note: The `/mcp/v1` endpoint **requires authentication** (unless `-no-auth` is specified during endpoint configuration):

```bash
# Without token - returns 401 Unauthorized
curl -X POST http://localhost:8080/mcp/v1 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": {}}'

# Response:
{"error": "Unauthorized"}
```


## Rate Limiting and Account Lockout

The MCP server includes built-in protection against brute force attacks through per-IP rate limiting and automatic account lockout.  When a valid username is provided, the MCP server tracks the number of failed login attempts for the account and locks that account if authentication is not successful.

Automatic lockout disables an account after a specified number of consecutive failed attempts.  The configurable threshold allows you to specify the maximum failed attempts (default: 0 = disabled).  An administrator can** use the `-enable-user` CLI command to re-enable locked accounts.

Failed authentication attempts are tracked per IP address to prevent brute force attacks:

- By default, 10 failed attempts per 15-minute window per IP address invokes a lockout.
- This value is configurable - you can customize both the time window and attempt limit.
- Automatic cleanup ensures that old attempts are automatically removed from memory.
- Lockout is status-blind - rate limiting applies regardless of whether the username exists.

### Configuring Rate Limiting and Lockout

To configure lockout with a configuration file, add these properties to the file:

```yaml
http:
    auth:
        enabled: true
        token_file: "./pgedge-postgres-mcp-tokens.yaml"
        # Rate limiting settings
        rate_limit_window_minutes: 15  # Time window for rate limiting
        rate_limit_max_attempts: 10  # Max attempts per IP per window
        # Account lockout settings
        max_failed_attempts_before_lockout: 5  # 0 = disabled
```

**Example: Enabling Account Lockout**

```yaml
http:
    auth:
        enabled: true
        token_file: "./pgedge-postgres-mcp-tokens.yaml"
        max_failed_attempts_before_lockout: 5
        rate_limit_window_minutes: 15
        rate_limit_max_attempts: 10
```

With this configuration:

- After 5 failed login attempts, the account will be automatically disabled.
- IP addresses are limited to 10 failed attempts per 15-minute window.
- The server logs show when rate limiting is enabled.

You can also configure lockout with the following environment variables:

```bash
export PGEDGE_AUTH_MAX_FAILED_ATTEMPTS_BEFORE_LOCKOUT=5
export PGEDGE_AUTH_RATE_LIMIT_WINDOW_MINUTES=15
export PGEDGE_AUTH_RATE_LIMIT_MAX_ATTEMPTS=10
```

**Recovering a Locked Account**

The following command enables a locked account:

```bash
# Re-enable a locked account
./bin/pgedge-postgres-mcp -enable-user -username alice

# Reset failed attempts counter
# (automatically reset on successful login)
```


## Automatic File Reloading

The MCP server automatically detects and reloads changes to token
and user files without requiring a server restart. This enables hot updates
to authentication credentials while the server is running.

The server uses file system notifications (via `fsnotify`) to monitor the
token and user files for changes. When a file is modified, the server
automatically reloads the credentials:

- **Instant updates**: Changes take effect within 100ms
- **No downtime**: Server continues running during reload
- **Thread-safe**: Uses read-write locks to prevent race conditions
- **Editor-friendly**: Handles file deletion/recreation during saves
- **Session preservation**: Active user sessions remain valid during reload
- **Debouncing**: Batches rapid file changes to avoid excessive reloads

The server watches the directory containing the auth files (not the files
directly) because many editors delete and recreate files when saving. This
ensures that the watcher continues working after file edits.

During the reload process:

1. File system event detected (Write or Create)
2. Debounce timer (100ms) starts to batch rapid changes
3. Reload function executes with write lock
4. New credentials loaded from disk
5. Old credentials replaced atomically
6. Active sessions preserved (for user files)
7. Confirmation logged to server output

**Thread Safety**

All reload operations use read-write locks (`sync.RWMutex`) to ensure:

- Multiple concurrent read operations (authentication checks) can proceed
- Write operations (reloads) block all other operations temporarily
- No race conditions between authentication and reload
- Atomic replacement of credential data

**Monitoring Reload Events**

Reload events are added to the server logs as shown below:

```
[AUTH] Reloaded /path/to/pgedge-postgres-mcp-tokens.yaml
[AUTH] Reloaded /path/to/pgedge-postgres-mcp-users.yaml
```

Failed reloads are also logged:

```
[AUTH] Failed to reload /path/to/pgedge-postgres-mcp-tokens.yaml:
permission denied
```

**Auto-Reload Limitations**

- **File must exist**: Deleting the file entirely will cause errors.
- **Valid YAML required**: Syntax errors can prevent files from reloading (old data is retained).
- **Same location**: Moving the file to a different location requires a restart.
- **No cascade**: Changing the token file path in your configuration requires a restart.

### Implementation

The auto-reload feature is implemented using:

- **fsnotify**: Cross-platform file system notifications
- **Watcher goroutine**: Background monitoring in separate thread
- **Debounce timer**: 100ms delay to batch rapid changes
- **RWMutex locks**: Thread-safe data structure access
- **Reload callbacks**: TokenStore.Reload() and UserStore.Reload()

For implementation details, see:

- [internal/auth/watcher.go](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/internal/auth/watcher.go) - File watching
- [internal/auth/auth.go](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/internal/auth/auth.go) - Token store reload
- [internal/auth/users.go](https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/internal/auth/users.go) - User store reload


## Authentication Flow

For an interactive application that uses password authentication, authentication follows a two-step process.  During authentication, the user authenticates, and is then assigned a token.  That token is used for secure authentication for the session:

**Step 1: Authenticate the User with a Password**

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

The returned token is:

- valid for 24 hours from authentication
- a Base64-encoded random 32-byte token
- strongly random, cryptographically secure

After 24 hours, the user is required to re-authenticate to get a new session token.

**Step 2: Use a Session Token to Authenticate**

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


## Client Implementation Example

The following example demonstrates implementing authentication in a Python client.

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


## Updating Passwords and Tokens

You can use the following command to update a user password:

```bash
# Server running with active user sessions

# Update user password
./bin/pgedge-postgres-mcp -update-user \
  -username alice \
  -password "NewSecurePassword456!"

# Server reloads user file
# Alice's active session remains valid
# New password required for next login
```

To perform a bulk update of session tokens, you can edit the token file directly:

```bash
# Edit token file directly for bulk changes
nano pgedge-postgres-mcp-tokens.yaml

# On save, server automatically detects change:
# [AUTH] Reloaded /path/to/pgedge-postgres-mcp-tokens.yaml
```


## Error Responses

The following responses may occur as a result of authentication errors:

| Error Type      | JSON Response                          | HTTP Status         |
|-----------------|-----------------------------------------|----------------------|
| Missing Token   | `{ "error": "Unauthorized" }`           | `401 Unauthorized`   |
| Invalid Token   | `{ "error": "Unauthorized" }`           | `401 Unauthorized`   |
| Expired Token   | `{ "error": "Unauthorized" }`           | `401 Unauthorized`   |

**Note:** For security reasons, specific error details are not exposed.


## Health Endpoint

The `/health` endpoint is **always accessible** without authentication:

```bash
# No token required
curl http://localhost:8080/health
```


## To Disable Authentication (Development Only)

!!! warning

     Never disable authentication in production!

The following command disables authentication:

```bash
./bin/pgedge-postgres-mcp -http -no-auth
```


## Security Considerations

See the [Security Guide](security.md) for comprehensive security best practices related to:

- Token storage and protection
- HTTPS requirements
- Network security
- Audit logging
- Incident response