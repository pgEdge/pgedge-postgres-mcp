# Authentication Guide

The MCP server includes built-in authentication with two
methods: API tokens for machine-to-machine communication and
user accounts for interactive authentication.

- Use an [*API Token*](auth_token.md) for direct
  machine-to-machine access. Tokens are long-lived and
  easily managed by administrators.
- Use a [*User Account*](auth_user.md) for interactive
  applications; an account is session-based, and users can
  manage own password access.

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

The address a request is counted against comes from the network
connection itself, so a caller cannot choose it. Deployments behind a
reverse proxy see the proxy's address on every connection and must
configure `http.client_ip` to read the real address from a forwarding
header; the server honours that header only on connections from a
trusted proxy. See [Client Address
Resolution](configuration.md#client-address-resolution) for details.

### Configuring Rate Limiting and Lockout

To configure lockout with a configuration file, add these properties to the file:

```yaml
http:
    auth:
        enabled: true
        token_file: "./postgres-mcp-tokens.yaml"
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
        token_file: "./postgres-mcp-tokens.yaml"
        max_failed_attempts_before_lockout: 5
        rate_limit_window_minutes: 15
        rate_limit_max_attempts: 10
```

With this configuration:

- After 5 failed login attempts, the account will be automatically disabled.
- IP addresses are limited to 10 failed attempts per 15-minute window.
- The server logs show when rate limiting is enabled.

!!! warning "Lockout is permanent and keyed on the username alone"

    Understand two properties of account lockout before enabling it,
    because together they make it a denial-of-service tool as well as a
    safety feature.

    A lockout never expires on its own. The server clears the account's
    `enabled` flag and leaves it clear; there is no timeout after which
    the account recovers. An administrator must run `-enable-user`, as
    described below, so a lockout during an out-of-hours incident lasts
    until someone with shell access attends to it.

    The counter is keyed on the username and takes no account of who is
    making the attempts. Anyone who can reach the server and who knows
    or guesses a valid username can therefore lock that account
    deliberately, by failing authentication against it the configured
    number of times; the attempts need not come from one address, and
    the real owner's own address is never consulted. This is why the
    setting defaults to `0`, which disables lockout entirely and leaves
    brute-force protection to the per-address rate limiting described
    above.

    Weigh those properties against your threat model. Lockout is worth
    enabling where the accounts are few, the administrators are reachable,
    and the usernames are not widely known; it is a poor fit for a server
    exposed to untrusted callers, where it hands them a way to disable a
    known account at will.

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
[AUTH] Reloaded /path/to/postgres-mcp-tokens.yaml
[AUTH] Reloaded /path/to/postgres-mcp-users.yaml
```

Failed reloads are also logged:

```
[AUTH] Failed to reload /path/to/postgres-mcp-tokens.yaml:
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


## Authorization: Database Access Control

The MCP server enforces authorization through the
`available_to_users` field on each database configuration.
This field restricts which authenticated users can access
each database connection.

### Configuring User Access

The `available_to_users` field accepts a list of usernames.
An empty list grants access to all authenticated users.

In the following example, the configuration defines three
databases with different access levels:

```yaml
databases:
    - name: "production"
      host: "prod-db.example.com"
      port: 5432
      database: "myapp"
      user: "readonly_user"
      sslmode: "require"
      available_to_users: []  # All users

    - name: "staging"
      host: "staging-db.example.com"
      port: 5432
      database: "myapp_staging"
      user: "developer"
      available_to_users:
          - "alice"
          - "bob"

    - name: "development"
      host: "localhost"
      port: 5432
      database: "myapp_dev"
      user: "developer"
      available_to_users:
          - "alice"
          - "bob"
          - "charlie"
```

The following table shows database access for each user
with the configuration above:

| User    | production | staging | development |
|---------|------------|---------|-------------|
| alice   | Yes        | Yes     | Yes         |
| bob     | Yes        | Yes     | Yes         |
| charlie | Yes        | No      | Yes         |
| dave    | Yes        | No      | No          |

### Per-Token Database Binding

API tokens can bind to a specific database using the
`-token-database` flag. A bound token cannot switch to
another database during its lifetime.

In the following example, the `-add-token` command creates
a token bound to the `production` database:

```bash
./bin/pgedge-postgres-mcp -add-token \
    -token-note "Production Monitor" \
    -token-database "production" \
    -token-expiry "90d"
```

### Authorization Model Summary

The server combines authentication and authorization to
control database access:

- User authentication and the `available_to_users` field
  determine which databases a user can access.
- Token authentication and the `-token-database` flag
  determine which database a token can reach.
- An empty `available_to_users` list allows all
  authenticated users to access the database.
- API tokens bound to a database cannot switch to
  another database.

For more information about managing multiple databases,
see the
[Multiple Database Configuration](multiple_db_config.md)
guide.


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


## Token Lifecycle Management

The MCP server manages tokens through creation, validation,
and expiration. Understanding the token lifecycle helps
clients maintain uninterrupted access.

### Token Creation

The server supports two methods for creating tokens:

- The `-add-token` CLI command creates long-lived API
  tokens for machine-to-machine access.
- The `authenticate_user` tool creates session tokens
  for interactive user authentication.
- Session tokens remain valid for 24 hours after the
  server creates them.

### Token Validation

The server validates every token before processing a
request. The validation checks the token format, existence,
and expiration timestamp.

- The server validates the token on every request.
- Expired tokens receive a `401 Unauthorized` response.
- Invalid tokens also receive a `401 Unauthorized`
  response.

### Detecting Token Expiration

Clients can check token expiry before sending a request.
This approach avoids unnecessary `401` responses from
the server.

In the following example, the `is_token_expired` function
checks a token expiry timestamp against the current time:

```python
from datetime import datetime, timezone

def is_token_expired(expiry_str):
    """Check if a token has expired."""
    expiry = datetime.fromisoformat(
        expiry_str.replace("Z", "+00:00")
    )
    return datetime.now(timezone.utc) >= expiry

# Check before making a request
if is_token_expired(client.token_expiry):
    client.authenticate(username, password)
```

### Automatic Re-Authentication

Clients should handle expired tokens by catching `401`
errors and re-authenticating automatically. This pattern
ensures seamless recovery from token expiration.

In the following example, the `call_with_retry` function
re-authenticates when the server returns a `401` error:

```python
def call_with_retry(client, tool_name, arguments,
                    username, password):
    """Call a tool with automatic re-authentication."""
    try:
        return client.call_tool(tool_name, arguments)
    except Exception as e:
        if "401" in str(e) or "Unauthorized" in str(e):
            client.authenticate(username, password)
            return client.call_tool(
                tool_name, arguments
            )
        raise
```

### Mid-Conversation Token Expiration

A token can expire while a conversation is in progress.
The server handles this situation gracefully without
losing context.

- The next request receives a `401` response from
  the server.
- The client should re-authenticate and retry the
  failed request.
- The server preserves previous conversation context
  during re-authentication.
- No data is lost during the re-authentication process.

### Best Practices for Programmatic Clients

Programmatic clients should follow these security and
reliability practices for token management:

- Never log tokens or include tokens in error messages.
- Use environment variables to store credentials instead
  of hardcoding them.
- Store session tokens in memory rather than writing
  them to disk.
- Implement token rotation for long-lived background
  processes.
- Check the token expiry before each request to avoid
  unnecessary `401` errors.
- Set reasonable timeout values for all API requests.

For working client examples, see the
[Client Examples](../developers/client-examples.md) page.
For token administration commands, see the
[Token Management](auth_token.md) page.


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
nano postgres-mcp-tokens.yaml

# On save, server automatically detects change:
# [AUTH] Reloaded /path/to/postgres-mcp-tokens.yaml
```


## Error Responses

The following responses may occur as a result of authentication errors:

| Error Type      | JSON Response                          | HTTP Status         |
|-----------------|-----------------------------------------|----------------------|
| Missing Token   | `{ "error": "Unauthorized" }`           | `401 Unauthorized`   |
| Invalid Token   | `{ "error": "Unauthorized" }`           | `401 Unauthorized`   |
| Expired Token   | `{ "error": "Unauthorized" }`           | `401 Unauthorized`   |

**Note:** For security reasons, specific error details are not exposed.


## Endpoints That Skip Authentication

Three endpoints are **always accessible** without a token, even when
authentication is enabled:

| Endpoint | Why it skips authentication |
|----------|------------------------------|
| `/health` | Load balancers and orchestrators need to probe liveness before they hold any credential. |
| `/api/user/info` | The web client calls this to discover whether authentication is enabled at all, which it must do before it can know to ask for a token. |
| `/api/openapi.json` | The API specification is published for discoverability, so that a client can be written against it before credentials are issued. |

In the following example, each endpoint responds without a token.

```bash
# No token required
curl http://localhost:8080/health
curl http://localhost:8080/api/user/info
curl http://localhost:8080/api/openapi.json
```

None of the three exposes database contents or accepts a query, so the
data they return is limited to the server's own status, whether
authentication is switched on, and the shape of the API. Treat the
specification as public information when deciding what network to
expose a server to; if that is not acceptable for your deployment,
restrict these paths at the reverse proxy rather than in the server.


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
