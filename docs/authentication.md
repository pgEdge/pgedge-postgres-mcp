# Authentication Guide

The pgEdge MCP Server includes built-in API token authentication for HTTP/HTTPS mode, enabled by default.

## Overview

- **Enabled by default** in HTTP/HTTPS mode
- **SHA256 token hashing** for secure storage
- **Token expiration** with automatic cleanup
- **Per-token connection isolation** for multi-user security
- **Bearer token authentication** using HTTP Authorization header
- **Not required** for stdio mode (Claude Desktop)

### Connection Isolation

When authentication is enabled, each API token gets its own isolated database connection pool. This provides:

- **Security**: Prevents users from accessing each other's database sessions
- **Isolation**: Temporary tables and session variables are isolated per token
- **Automatic cleanup**: Database connections are closed when tokens expire
- **Resource management**: Independent connection pools per token

See [Security Guide - Connection Isolation](security.md#connection-isolation) for more details.

## Token Management

### Adding Tokens

#### Interactive Mode

```bash
# Add token with prompts
./bin/pgedge-postgres-mcp -add-token
```

You'll be prompted for:

- **Note**: Description/identifier for the token (e.g., "Production API", "Dev Environment")
- **Expiry**: Duration or "never" (e.g., "30d", "1y", "never")

#### Command Line Mode

```bash
# Add token with all details specified
./bin/pgedge-postgres-mcp -add-token \
  -token-note "Production API" \
  -token-expiry "1y"

# Add token with no expiration
./bin/pgedge-postgres-mcp -add-token \
  -token-note "CI/CD Pipeline" \
  -token-expiry "never"
```

**Important**: The generated token is **shown only once**. Save it immediately!

```
Token created successfully:
Token: O9ms9jqTfUdy-DIjvpFWeqd_yH_NEj7me0mgOnOjGdQ=
ID: token-1234567890
Note: Production API
Expiry: 2025-10-30 (365 days from now)
Hash (first 12 chars): b3f805a4c...

Store this token securely. It cannot be retrieved later.
```

### Listing Tokens

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

### Removing Tokens

You can remove tokens by ID or hash prefix:

```bash
# Remove by full token ID
./bin/pgedge-postgres-mcp -remove-token token-1234567890

# Remove by hash prefix (minimum 8 characters)
./bin/pgedge-postgres-mcp -remove-token b3f805a4

# Remove by partial hash (at least 8 chars)
./bin/pgedge-postgres-mcp -remove-token b3f805a4c2
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
./bin/pgedge-postgres-mcp -add-token -token-note "Test" -token-expiry "1h"

# Standard token for applications
./bin/pgedge-postgres-mcp -add-token -token-note "API Client" -token-expiry "90d"

# Long-lived token for services
./bin/pgedge-postgres-mcp -add-token -token-note "Monitoring" -token-expiry "1y"

# Permanent token (requires explicit renewal)
./bin/pgedge-postgres-mcp -add-token -token-note "Admin" -token-expiry "never"
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
./bin/pgedge-postgres-mcp -http -token-file /etc/pgedge-mcp/tokens.yaml
```

### Disable Authentication (Development Only)

**Warning**: Never use this in production!

```bash
./bin/pgedge-postgres-mcp -http -no-auth
```

### Configuration File

In `pgedge-postgres-mcp.yaml`:

```yaml
http:
  enabled: true
  address: ":8080"
  auth:
    enabled: true
    token_file: "/path/to/pgedge-postgres-mcp-server-tokens.yaml"
```

## Token Storage

### Storage Location

- **Default**: `pgedge-postgres-mcp-server-tokens.yaml` in the same directory as the binary
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
Loaded 3 API token(s) from pgedge-postgres-mcp-server-tokens.yaml
Removed 1 expired token(s)
```

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
./bin/pgedge-postgres-mcp -add-token \
  -token-note "Web App - Q4 2024" \
  -token-expiry "90d"

# Bad: Never-expiring tokens
./bin/pgedge-postgres-mcp -add-token \
  -token-note "Web App" \
  -token-expiry "never"
```

### 4. Development vs Production

**Development**:
```bash
# Use -no-auth for local development (localhost only)
./bin/pgedge-postgres-mcp -http -addr "localhost:8080" -no-auth
```

**Production**:
```bash
# Always use authentication with HTTPS
./bin/pgedge-postgres-mcp -http -tls \
  -cert /path/to/cert.pem \
  -key /path/to/key.pem
```

### 5. Token Distribution

When distributing tokens to users/services:

1. **Secure channels**: Use encrypted communication
2. **One-time display**: Show token only once during creation
3. **Documentation**: Include expiry date and renewal process
4. **Revocation plan**: Document how to remove compromised tokens

## Troubleshooting

### Token File Not Found

```bash
# Error message:
ERROR: Token file not found: /path/to/pgedge-postgres-mcp-server-tokens.yaml
Create tokens with: ./pgedge-postgres-mcp -add-token
Or disable authentication with: -no-auth
```

**Solution**:
```bash
# Create first token
./bin/pgedge-postgres-mcp -add-token
```

### Token Authentication Fails

```bash
# Check token file exists and has correct permissions
ls -la pgedge-postgres-mcp-server-tokens.yaml  # Should show -rw------- (600)

# List tokens to verify token exists
./bin/pgedge-postgres-mcp -list-tokens

# Check for expired tokens
./bin/pgedge-postgres-mcp -list-tokens | grep "Status: Expired"
```

### Cannot Remove Token

```bash
# Error: Token not found
# Solution: Use at least 8 characters of the hash
./bin/pgedge-postgres-mcp -list-tokens  # Get the hash
./bin/pgedge-postgres-mcp -remove-token b3f805a4  # Use 8+ chars
```

### Server Won't Start (Auth Enabled)

If auth is enabled but no token file exists:

```bash
# Option 1: Create a token file
./bin/pgedge-postgres-mcp -add-token

# Option 2: Disable auth temporarily
./bin/pgedge-postgres-mcp -http -no-auth
```

## Security Considerations

See the [Security Guide](security.md) for comprehensive security best practices including:

- Token storage and protection
- HTTPS requirements
- Network security
- Audit logging
- Incident response

## Related Documentation

- [Deployment Guide](deployment.md) - HTTP/HTTPS server setup
- [Configuration Guide](configuration.md) - Configuration file and flags
- [Security Guide](security.md) - Security best practices
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
