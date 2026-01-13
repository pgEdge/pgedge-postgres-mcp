# Managing Security

You should always keep the principle of least security in mind when managing
users; always use dedicated database users with minimal permissions.

    - Create separate users for different access levels.
    - Grant the minimum required permissions to each user.
    - Regularly audit required permissions.

Operating system users should run as a dedicated user:

    ```bash
    # Run server as dedicated user
    sudo useradd -r -s /bin/false pgedge
    sudo chown -R pgedge:pgedge /opt/pgedge
    ```

Also, be sure your file permissions are restrictive:

    ```bash
    # Binary: 755 (executable by all, writable by owner)
    chmod 755 /opt/pgedge/bin/pgedge-postgres-mcp

    # Config files: 600 (readable/writable by owner only)
    chmod 600 /etc/pgedge/config.yaml
    chmod 600 /etc/pgedge/pgedge-postgres-mcp-tokens.yaml

    # Secret file: 600 (CRITICAL - contains encryption key)
    chmod 600 /etc/pgedge/pgedge-postgres-mcp.secret

    # Certificates: 600 for keys, 644 for certs
    chmod 600 /etc/pgedge/certs/server.key
    chmod 644 /etc/pgedge/certs/server.crt
    ```

Additionally, you should use nginx or HAProxy to enforce rate limiting:

```nginx
# /etc/nginx/conf.d/ratelimit.conf
limit_req_zone $binary_remote_addr zone=mcp:10m rate=10r/s;

server {
    location /mcp/v1 {
        limit_req zone=mcp burst=20 nodelay;
        proxy_pass http://localhost:8080;
    }
}
```

Use the following commands to create secure users:

```sql
-- Create read-only user for MCP server
CREATE USER mcp_readonly WITH PASSWORD 'secure_password';

-- Grant connect permission
GRANT CONNECT ON DATABASE mydb TO mcp_readonly;

-- Grant read-only access to specific schemas
GRANT USAGE ON SCHEMA public TO mcp_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO mcp_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO mcp_readonly;

-- For configuration viewing (optional)
GRANT pg_read_all_settings TO mcp_readonly;
```

Use the following syntax to create a user for configuration management (with
elevated privileges):

```sql
-- Create user with configuration privileges
CREATE USER mcp_admin WITH PASSWORD 'secure_password';
GRANT pg_read_all_settings, pg_write_all_settings TO mcp_admin;
```


## Connection Security

You should always use SSL/TLS for database connections:

```bash
# Require SSL
PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:pass@host/db?sslmode=require"

# Verify CA certificate
PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:pass@host/db?sslmode=verify-ca&sslrootcert=/path/to/ca.crt"

# Full verification (hostname + CA)
PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:pass@host/db?sslmode=verify-full&sslrootcert=/path/to/ca.crt"
```

The MCP Server supports the following SSL Mode Options:

- `disable` - No SSL (only for local development)
- `require` - SSL required, no certificate verification
- `verify-ca` - SSL required, verify server certificate
- `verify-full` - SSL required, verify server certificate and hostname

To ensure connection security, you should use a secrets manager to manage your secrets:

```bash
export PGPASSWORD=$(vault kv get -field=password secret/pgedge-nla)
```


## API Keys

### Anthropic API Key Protection

To ensure key protection, enforce the following best practices:

- Store your keys in environment variables.
- Rotate keys regularly (quarterly recommended).
- Monitor API usage for anomalies.
- Set usage limits in the Anthropic console.
- Use separate keys for development/staging/production environments.

**Example - This demonstrates Secure key usage:**

```bash
# Environment variable
export ANTHROPIC_API_KEY="sk-ant-your-key-here"

# Load from secrets manager
export ANTHROPIC_API_KEY=$(aws secretsmanager get-secret-value --secret-id anthropic-key --query SecretString --output text)
```

**Example - Insecure (DON'T DO THIS):**

```json
// Don't commit API keys in Claude Desktop config
{
  "mcpServers": {
    "pgedge": {
      "env": {
        "ANTHROPIC_API_KEY": "sk-ant-actual-key-committed-to-git"
      }
    }
  }
}
```

You should always monitor your API keys:

- Check the Anthropic Console regularly for unusual activity.
- Set up billing alerts.
- Monitor token usage patterns.
- Revoke and rotate keys if a security breach is suspected.


## Query Safety

The `query_database` tool executes all queries in **read-only transactions**:

```sql
SET TRANSACTION READ ONLY;
-- Your generated SQL here
```

The server prevents:

- `INSERT`, `UPDATE`, `DELETE` operations.
- `TRUNCATE`, `DROP` operations.
- `CREATE`, `ALTER` operations.
- function calls that modify data.

**Examples - Blocked Operations:**

```sql
-- These will fail: "cannot execute INSERT in a read-only transaction"
INSERT INTO users (name) VALUES ('test');
UPDATE users SET active = false;
DELETE FROM logs WHERE created_at < NOW() - INTERVAL '30 days';
CREATE TABLE test (id INT);
DROP TABLE old_data;
```

To  enforce additional safeguards, use a read-only database role:

```sql
-- Even if transaction protection fails, the user lacks permissions
REVOKE INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public FROM mcp_readonly;
```

Use the following queries to monitor your query logs:

```sql
-- Enable query logging in PostgreSQL
ALTER SYSTEM SET log_statement = 'all';
ALTER SYSTEM SET log_duration = on;
SELECT pg_reload_conf();
```

Review any generated SQL:

```bash
# If started with the development scripts (stdout/stderr redirect):
tail -f /tmp/pgedge-postgres-mcp.log | grep "Generated SQL"

# If running under systemd:
journalctl -u pgedge-postgres-mcp -f | grep "Generated SQL"
```

While read-only protection is enforced, additional vigilance should be employed
to detect:

- unusual patterns in natural language queries.
- SQL injection attempts in query text.

You should:

- set query timeout limits in PostgreSQL.
- use connection pooling with query limits.


## Configuration Management

The `set_pg_configuration` tool can modify PostgreSQL settings.  This creates
some risks:

- Changes persist across server restarts.
- Some changes require a server restart to take effect.
- Incorrect settings can impact performance or availability.
- Requires superuser or `pg_write_all_settings` role.

To mitigate risks:

**Use a dedicated configuration user:**

```sql
CREATE USER mcp_config WITH PASSWORD 'secure_password';
GRANT pg_read_all_settings, pg_write_all_settings TO mcp_config;
```

**Backup your configuration before making changes:**

```bash
# Backup postgresql.conf and postgresql.auto.conf
cp /var/lib/postgresql/data/postgresql.conf /backup/
cp /var/lib/postgresql/data/postgresql.auto.conf /backup/
```

**Test any changes in a non-production environment first:**

```bash
# Apply to staging environment
./bin/pgedge-postgres-mcp -db "postgres://staging/db"
# Tool: set_pg_configuration with test values
# Monitor impact before applying to production
```

**Monitor configuration changes:**

```sql
-- Track configuration changes
SELECT name, setting, source, sourcefile
FROM pg_settings
WHERE source = 'configuration file'
ORDER BY name;
```


## Network Security

As part of your network security, you should use firewall rules to restrict
access to systems that need to be in contact with each other.  Additionally
you should:

**Restrict database access**

```bash
# UFW (Ubuntu/Debian)
sudo ufw allow from 10.0.1.0/24 to any port 5432
sudo ufw deny 5432

# iptables
sudo iptables -A INPUT -s 10.0.1.0/24 -p tcp --dport 5432 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 5432 -j DROP
```

**Restrict HTTP/HTTPS server access**

```bash
# Allow only from specific IPs
sudo ufw allow from 203.0.113.0/24 to any port 8080

# Allow only on private network
sudo ufw allow from 10.0.0.0/8 to any port 8080
```

You should also:

- Run the MCP server in DMZ or an application tier.
- Keep your database in a separate network .
- Use a VPN for remote access.
- Implement network ACLs.

Use Postgres host-based authentication to restrict database access. To use
[Postgres authentication](https://www.postgresql.org/docs/18/auth-pg-hba-conf.html)
, edit the `pg_hba.conf` file:

```
# Allow MCP server from specific IP with SSL
hostssl  mydb  mcp_readonly  10.0.1.100/32  scram-sha-256

# Reject all other connections
host     all   all           0.0.0.0/0      reject
```


## HTTP/HTTPS Mode Security

HTTP sends the following requests in plaintext:

- API tokens
- Database query results
- Natural language queries
- Connection strings (if exposed)

You should always use HTTPS for production environments; never use HTTP for:

- External/public-facing deployments
- Transmission over untrusted networks
- Production environments


### Configuring TLS

**Minimum requirements:**

```bash
# Use TLS 1.2 or higher
# Server automatically enforces this
./bin/pgedge-postgres-mcp -http -tls \
  -cert /path/to/cert.pem \
  -key /path/to/key.pem
```

**Test TLS configuration:**

```bash
# Check TLS version support
nmap --script ssl-enum-ciphers -p 8080 localhost

# Test with specific TLS version
openssl s_client -connect localhost:8080 -tls1_2
```


## Token Security

See [this page](auth_token.md) for detailed information about token
management; the following examples demonstrate best practices for token
management:

**Tokens should have expiration dates**

    ```bash
    # Good: 90-day expiration
    ./bin/pgedge-postgres-mcp -add-token -token-expiry "90d"

    # Avoid: Never-expiring tokens
    ./bin/pgedge-postgres-mcp -add-token -token-expiry "never"
    ```

**Rotate your tokens regularly**

    - Set up calendar reminders for rotation
    - Create new token before old one expires
    - Remove old token after migration

**Use one token per service**

    - Don't share tokens between applications
    - Easier to revoke if compromised
    - Better audit trail

**Store tokens securely**

    ```bash
    # Use environment variables or secrets managers
    export MCP_TOKEN=$(vault kv get -field=token secret/mcp)

    # Never in code or logs
    curl -H "Authorization: Bearer $MCP_TOKEN" ...  # OK
    curl -H "Authorization: Bearer abc123..." ...    # Don't hardcode
    ```

**Set token file permissions restrictively**

    ```bash
    # Verify file permissions
    ls -la pgedge-postgres-mcp-tokens.yaml  # Should be -rw------- (600)

    # Fix if needed
    chmod 600 pgedge-postgres-mcp-tokens.yaml
    ```

### Using Per-Token Database Connections

When authentication is enabled in HTTP/HTTPS mode, the MCP server implements **per-token connection isolation** to ensure security and prevent cross-user data access.

How it works:

- Each API token gets its own dedicated database connection pool
- Database connections are never shared between different tokens
- When a token expires, its database connections are automatically closed
- This prevents one user from accessing database resources opened by another user

Security benefits:

* **Isolation**: Users with different tokens cannot interfere with each other's database sessions
* **Session Security**: Temporary tables, prepared statements, and session variables are isolated per token
* **Automatic Cleanup**: Expired tokens trigger automatic cleanup of their database connections
* **Resource Management**: Connection pools are managed independently for each token

**When connection isolation is active:**

```bash
# Start server with authentication enabled
./bin/pgedge-postgres-mcp -http -tls \
  -cert /path/to/cert.pem \
  -key /path/to/key.pem

# Server log will show:
# Connection isolation: ENABLED (per-token database connections)
```

**When connection isolation is NOT active:**

- Stdio mode (single-user, no authentication)
- HTTP mode with `-no-auth` flag (all requests share one connection)

### Monitoring Connection Pools

It's a good idea to monitor activity in your connection pools. The server logs
connection creation and cleanup:

```
Created new database connection for token hash: 5f4dcc3b5aa7 (total: 3)
Removed database connection for token hash: 5f4dcc3b5aa7 (remaining: 2)
```

**Best practices**

- Always enable authentication for multi-user deployments.
- Monitor server logs for connection pool growth.
- Set appropriate token expiration times to prevent connection pool exhaustion.
- Consider database connection limits when issuing tokens to many users.


## TLS/Certificate Security

Key permissions should be set to limit access to only the owner:

**File permissions**

```bash
# Private keys should be 600 (owner read/write only)
chmod 600 /path/to/server.key

# Verify
ls -la /path/to/server.key  # Should show -rw-------
```

It's a good practice to:

- store keys outside the web root.
- use hardware security modules (HSM) for keys in high-security environments.
- never commit keys to a version control management system.

**Example - Secure Key Storage**

```bash
# Store in /etc with restricted permissions
sudo mkdir -p /etc/pgedge/certs
sudo chmod 700 /etc/pgedge/certs
sudo cp server.key /etc/pgedge/certs/
sudo chmod 600 /etc/pgedge/certs/server.key
sudo chown pgedge:pgedge /etc/pgedge/certs/server.key
```

Certificates should also be managed securely. You should always:

- include intermediate certificates in your certificate chain.
- use the `-chain` flag with full chain file.
- test the certificate chain with the command:

    `openssl s_client -connect localhost:8080 -showcerts`

You should also monitor certificate expiration:

```bash
# Check expiration date
openssl x509 -in server.crt -noout -dates

# Set up automated renewal (Let's Encrypt)
sudo certbot renew --dry-run
```

You can automate renewal with Let's Encrypt:

```bash
# Install certbot
sudo apt-get install certbot  # Ubuntu/Debian

# Set up auto-renewal
sudo systemctl enable certbot.timer
sudo systemctl start certbot.timer

# Verify timer is active
sudo systemctl list-timers | grep certbot
```

## Monitoring and Auditing

### Log Monitoring

**What to monitor:**

- Failed authentication attempts
- Unusual query patterns
- Configuration changes
- Database connection errors
- API rate limiting triggers

**Example - Log analysis:**

```bash
# Monitor authentication failures
journalctl -u pgedge-postgres-mcp | grep "Unauthorized"

# Count auth failures per IP
journalctl -u pgedge-postgres-mcp | grep "Unauthorized" | awk '{print $NF}' | sort | uniq -c | sort -rn

# Monitor configuration changes
journalctl -u pgedge-postgres-mcp | grep "set_pg_configuration"
```

### Audit Trail

**Database query logging:**

```sql
-- Enable comprehensive logging
ALTER SYSTEM SET log_statement = 'all';
ALTER SYSTEM SET log_connections = on;
ALTER SYSTEM SET log_disconnections = on;
ALTER SYSTEM SET log_duration = on;
SELECT pg_reload_conf();
```

**Application logging:**

```bash
# Log to file with timestamps
./bin/pgedge-postgres-mcp -http 2>&1 | tee -a /var/log/pgedge/pgedge-postgres-mcp-server.log
```


## Incident Response

### If a Token is Compromised

1. **Immediate actions:**

    ```bash
    # Remove compromised token
    ./bin/pgedge-postgres-mcp -remove-token <token-id>

    # Create new token
    ./bin/pgedge-postgres-mcp -add-token -token-expiry "30d"

    # Update application with new token
    ```

2. **Investigation:**

    - Check logs for suspicious activity
    - Identify scope of unauthorized access
    - Review database logs for unusual queries

3. **Prevention:**

    - Rotate all tokens
    - Review access controls
    - Update security procedures

### If Database Credentials are Compromised

1. **Immediate actions:**

    ```sql
    -- Change password immediately
    ALTER USER mcp_readonly WITH PASSWORD 'new_secure_password';

    -- Terminate existing connections
    SELECT pg_terminate_backend(pid)
    FROM pg_stat_activity
    WHERE usename = 'mcp_readonly' AND pid <> pg_backend_pid();
    ```

2. **Update the MCP server:**

    ```bash
    # Update connection string
    export PGEDGE_POSTGRES_CONNECTION_STRING="postgres://mcp_readonly:new_password@host/db"

    # Restart server
    sudo systemctl restart pgedge-postgres-mcp
    ```

### If a Server is Compromised

1. **Isolate:**

    ```bash
    # Block network access
    sudo ufw deny 8080

    # Stop service
    sudo systemctl stop pgedge-postgres-mcp
    ```

2. **Investigate:**

    - Review system logs
    - Check for unauthorized files
    - Analyze network connections

3. **Recover:**

    - Rebuild from known good state
    - Rotate all credentials
    - Review and update security measures

