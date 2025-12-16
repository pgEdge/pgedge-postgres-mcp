# Authentication - Token Management

When token authentication is enabled, each API token gets its own isolated database connection pool. This architecture:

- prevents users from accessing each other's database sessions.
- provides isolation for temporary tables and session variables.
- closes database connections when tokens expire.

To include token file details in the configuration file (`pgedge-postgres-mcp.yaml`), use the following syntax:

```yaml
http:
  enabled: true
  address: ":8080"
  auth:
    enabled: true
    token_file: "/path/to/pgedge-postgres-mcp-tokens.yaml"
```

By default, tokens are stored in a file named `pgedge-postgres-mcp-tokens.yaml` in the same directory as the MCP binary. Tokens are stored in the following format:

```yaml
tokens:
  - id: token-1234567890
    hash: b3f805a4c2e7d9f1a8b6c3e2d5f4a1b9c8e7d6f5a4b3c2e1d9f8a7b6c5e4d3f2
    note: Production API
    created_at: "2024-10-30T10:15:30Z"
    expires_at: "2025-10-30T10:15:30Z"
```

To specify a custom file location, use the `-token-file` keyword and the following syntax:

```bash
# Specify custom token file path
./bin/pgedge-postgres-mcp -http -token-file /etc/pgedge/pgedge-postgres-mcp-tokens.yaml
```

**Important**:

- Tokens are stored as **SHA256 hashes** (not plaintext)
- File permissions automatically set to **0600** (owner read/write only)
- Original token cannot be retrieved from the file

When managing tokens, keep the following best practices in mind:

**Token Lifecycle Management**

- Set appropriate expiry times for all tokens.
- Create new tokens and remove old ones periodically.
- Regularly review the `list-tokens` output.
- Clean up tokens that are no longer needed.

**Token Security**

- Don't store tokens in version control.
- Use environment variables for application secrets.
- Use different tokens for different services/users.
- Monitor your log files for suspicious activity.
- Protect token file; ensure that file permissions are set to 0600.

**Token Distribution**

- Use encrypted communication across secure channels.
- Show each token only once during creation.
- Your documentation should include the expiry date and renewal process.
- You should document how to remove compromised tokens in the event of a security breach.

In a production deployment, use short-lived tokens for a secure environment:

```bash
# Good: Short-lived tokens with rotation
./bin/pgedge-postgres-mcp -add-token \
  -token-note "Web App - Q4 2024" \
  -token-expiry "90d"

# Bad: Never-expiring tokens
./bin/pgedge-postgres-mcp -add-token \
  -token-note "Web App" \
  -token-expiry "never"
```

In a development environment, you might omit the need for authentication:

**Development**:
```bash
# Use -no-auth for local development (localhost only)
./bin/pgedge-postgres-mcp -http -addr "localhost:8080" -no-auth
```

In a production environment, you should always use robust authentication across HTTPS:

**Production**:
```bash
# Always use authentication with HTTPS
./bin/pgedge-postgres-mcp -http -tls \
  -cert /path/to/cert.pem \
  -key /path/to/key.pem
```

## Token Management Syntax

To add a token while the server is running, use the following syntax

```bash
# Terminal 1: Server is running
./bin/pgedge-postgres-mcp -http

# Terminal 2: Add a new token without stopping server
./bin/pgedge-postgres-mcp -add-token \
  -token-note "New Client" \
  -token-expiry "30d"

# Server output shows:
# [AUTH] Reloaded /path/to/pgedge-postgres-mcp-tokens.yaml

# New token is immediately usable
```

To remove a compromised token:

```bash
# Server is running in production
# Security team detects compromised token

# Remove the token immediately
./bin/pgedge-postgres-mcp -remove-token b3f805a4

# Token is revoked within 100ms
# No server restart needed
```

To add a token in interactive mode:

```bash
# Add token with prompts
./bin/pgedge-postgres-mcp -add-token
```

You'll be prompted for:

- **Note**: Description/identifier for the token (e.g., "Production API",
  "Dev Environment")
- **Database**: Which database to bind the token to (from configured databases)
- **Expiry**: Duration or "never" (e.g., "30d", "1y", "never")

When multiple databases are configured, the interactive prompt displays
available databases and lets you select by number or name. Leave blank to use
the first configured database (default).

To create a token on the command-line, in non-interactive mode:

```bash
# Add token with all details specified
./bin/pgedge-postgres-mcp -add-token \
  -token-note "Production API" \
  -token-expiry "1y"

# Add token bound to a specific database
./bin/pgedge-postgres-mcp -add-token \
  -token-note "Staging API" \
  -token-database "staging" \
  -token-expiry "30d"

# Add token with no expiration
./bin/pgedge-postgres-mcp -add-token \
  -token-note "CI/CD Pipeline" \
  -token-expiry "never"
```

If you are creating a token in non-interactive mode, your database binding options are:

- Include the `-token-database <name>` option to bind the token to a specific configured database.
- If not specified in interactive mode: You'll be prompted to select from available databases.
- If left blank or not specified: Token uses the first configured database (the default).

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

!!! warning

    The generated token is **shown only once**. Save it immediately!


To generate a list of tokens:

```bash
./bin/pgedge-postgres-mcp -list-tokens
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

To remove a token by ID or hash prefix:

```bash
# Remove by full token ID
./bin/pgedge-postgres-mcp -remove-token token-1234567890

# Remove by hash prefix (minimum 8 characters)
./bin/pgedge-postgres-mcp -remove-token b3f805a4

# Remove by partial hash (at least 8 chars)
./bin/pgedge-postgres-mcp -remove-token b3f805a4c2
```


## Token Expiration Formats

Token expiration is time-based:

- `30d` - 30 days
- `1y` - 1 year
- `2w` - 2 weeks
- `12h` - 12 hours
- `90d` - 90 days
- `6m` - 6 months (not directly supported, use `180d`)

You can also specify that a token never expires:

- `never` - Token never expires (use with caution)

**Examples - Specifying Expiration Formats**

```bash
# Short-lived token for testing
./bin/pgedge-postgres-mcp -add-token -token-note "Test" -token-expiry "1h"

# Standard token for applications
./bin/pgedge-postgres-mcp -add-token -token-note "API Client" -token-expiry "90d"

# Long-lived token for services
./bin/pgedge-postgres-mcp -add-token -token-note "Monitoring" -token-expiry "1y"

# Permanent token (requires explicit renewal)
./bin/pgedge-postgres-mcp -add-token -token-note "Admin" -token-expiry "never"
```


## Using Tokens in an Application

When using a token in your application, include the token in the `Authorization` header with the `Bearer` scheme:

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

To use a token to initialize a connection:

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

To include a token when executing a query:

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


## Programming Language Examples

The following examples demonstrate using tokens in an application header.

### Python

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

### JavaScript/Node.js

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

### Go

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


## Automatic Token Cleanup

Expired tokens are automatically removed when the server starts; a message alerts you:

```
Loaded 3 API token(s) from pgedge-postgres-mcp-tokens.yaml
Removed 1 expired token(s)
```
