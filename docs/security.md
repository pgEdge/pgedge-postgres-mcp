# Security Guide

This document outlines security considerations and best practices for deploying and using the pgEdge MCP Server.

## Table of Contents

- [Database Credentials](#database-credentials)
- [API Keys](#api-keys)
- [Query Safety](#query-safety)
- [Configuration Management](#configuration-management)
- [Network Security](#network-security)
- [HTTP/HTTPS Mode Security](#httphttps-mode-security)
- [Token Security](#token-security)
- [TLS/Certificate Security](#tlscertificate-security)
- [Access Control](#access-control)
- [Monitoring and Auditing](#monitoring-and-auditing)
- [Incident Response](#incident-response)
- [Security Checklist](#security-checklist)

## Database Credentials

### Configuration at Startup

Database credentials are configured when the MCP server starts via:

- Environment variables (PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD)
- Configuration file (YAML format)
- Command-line flags

**Best Practices:**

- Use environment variables for sensitive credentials
- Never commit credentials to version control
- Use `.gitignore` for config files with credentials
- Consider using secret management systems (Vault, AWS Secrets Manager, etc.)
- In production, use `~/.pgpass` file or similar secure credential storage

**Example - Environment Variables:**
```bash
# Set database credentials via environment variables
export PGHOST="localhost"
export PGPORT="5432"
export PGDATABASE="mydb"
export PGUSER="myuser"
export PGPASSWORD="mypassword"

# Or use a secrets manager
export PGPASSWORD=$(vault kv get -field=password secret/pgedge-mcp)
```

**Example - Insecure (DON'T DO THIS):**

```bash
# Never hardcode in scripts
./bin/pgedge-nla-server -db "postgres://admin:SuperSecret123@prod.example.com/maindb"

# Never commit secret files
git add pgedge-pg-mcp-svr.secret  # DON'T DO THIS
```

### Connection Security

**Use SSL/TLS for Database Connections:**

```bash
# Require SSL
PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:pass@host/db?sslmode=require"

# Verify CA certificate
PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:pass@host/db?sslmode=verify-ca&sslrootcert=/path/to/ca.crt"

# Full verification (hostname + CA)
PGEDGE_POSTGRES_CONNECTION_STRING="postgres://user:pass@host/db?sslmode=verify-full&sslrootcert=/path/to/ca.crt"
```

**SSL Mode Options:**

- `disable` - No SSL (only for local development)
- `require` - SSL required, no certificate verification
- `verify-ca` - SSL required, verify server certificate
- `verify-full` - SSL required, verify server certificate and hostname

### Least Privilege

**Use dedicated database users with minimal permissions:**

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

**For configuration management (requires elevated privileges):**

```sql
-- Create user with configuration privileges
CREATE USER mcp_admin WITH PASSWORD 'secure_password';
GRANT pg_read_all_settings, pg_write_all_settings TO mcp_admin;
```

## API Keys

### Anthropic API Key Protection

**Best Practices:**

- Store in environment variables
- Rotate keys regularly (quarterly recommended)
- Monitor API usage for anomalies
- Set usage limits in Anthropic console
- Use separate keys for dev/staging/production

**Example - Secure:**

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

### API Key Monitoring

- Check Anthropic Console regularly for unusual activity
- Set up billing alerts
- Monitor token usage patterns
- Revoke and rotate keys if breach suspected

## Query Safety

### Read-Only Protection

The `query_database` tool executes all queries in **read-only transactions**:

```sql
SET TRANSACTION READ ONLY;
-- Your generated SQL here
```

**Protection:**

- Prevents `INSERT`, `UPDATE`, `DELETE` operations
- Prevents `TRUNCATE`, `DROP` operations
- Prevents `CREATE`, `ALTER` operations
- Prevents function calls that modify data

**Example - Blocked Operations:**

```sql
-- These will fail with: "cannot execute INSERT in a read-only transaction"
INSERT INTO users (name) VALUES ('test');
UPDATE users SET active = false;
DELETE FROM logs WHERE created_at < NOW() - INTERVAL '30 days';
CREATE TABLE test (id INT);
DROP TABLE old_data;
```

### Additional Safeguards

**1. Use read-only database role:**

```sql
-- Even if transaction protection fails, user lacks permissions
REVOKE INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public FROM mcp_readonly;
```

**2. Monitor query logs:**

```sql
-- Enable query logging in PostgreSQL
ALTER SYSTEM SET log_statement = 'all';
ALTER SYSTEM SET log_duration = on;
SELECT pg_reload_conf();
```

**3. Review generated SQL:**

```bash
# Check server logs to see generated queries
journalctl -u pgedge-mcp -f | grep "Generated SQL"
```

### Query Validation

While read-only protection is enforced, additional validation:

- Review natural language queries for unusual patterns
- Monitor for injection attempts in query text
- Set query timeout limits in PostgreSQL
- Use connection pooling with query limits

## Configuration Management

The `set_pg_configuration` tool can modify PostgreSQL settings.

### Risks

- Changes persist across server restarts
- Some changes require restart to take effect
- Incorrect settings can impact performance or availability
- Requires superuser or `pg_write_all_settings` role

### Mitigation

**1. Use dedicated configuration user:**
```sql
CREATE USER mcp_config WITH PASSWORD 'secure_password';
GRANT pg_read_all_settings, pg_write_all_settings TO mcp_config;
```

**2. Backup configuration before changes:**

```bash
# Backup postgresql.conf and postgresql.auto.conf
cp /var/lib/postgresql/data/postgresql.conf /backup/
cp /var/lib/postgresql/data/postgresql.auto.conf /backup/
```

**3. Test in non-production first:**

```bash
# Apply to staging environment
./bin/pgedge-nla-server -db "postgres://staging/db"
# Tool: set_pg_configuration with test values
# Monitor impact before applying to production
```

**4. Monitor configuration changes:**

```sql
-- Track configuration changes
SELECT name, setting, source, sourcefile
FROM pg_settings
WHERE source = 'configuration file'
ORDER BY name;
```

## Network Security

### Firewall Rules

**Restrict database access:**

```bash
# UFW (Ubuntu/Debian)
sudo ufw allow from 10.0.1.0/24 to any port 5432
sudo ufw deny 5432

# iptables
sudo iptables -A INPUT -s 10.0.1.0/24 -p tcp --dport 5432 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 5432 -j DROP
```

**Restrict HTTP/HTTPS server access:**

```bash
# Allow only from specific IPs
sudo ufw allow from 203.0.113.0/24 to any port 8080

# Allow only on private network
sudo ufw allow from 10.0.0.0/8 to any port 8080
```

### Network Segmentation

- Run MCP server in DMZ or application tier
- Database in separate network segment
- Use VPN for remote access
- Implement network ACLs

### PostgreSQL Host-Based Authentication

Edit `pg_hba.conf`:

```
# Allow MCP server from specific IP with SSL
hostssl  mydb  mcp_readonly  10.0.1.100/32  scram-sha-256

# Reject all other connections
host     all   all           0.0.0.0/0      reject
```

## HTTP/HTTPS Mode Security

### Always Use HTTPS in Production

**Never use HTTP for:**

- External/public-facing deployments
- Transmission over untrusted networks
- Production environments

**HTTP sends in plaintext:**

- API tokens
- Database query results
- Natural language queries
- Connection strings (if exposed)

### TLS Configuration

**Minimum requirements:**

```bash
# Use TLS 1.2 or higher
# Server automatically enforces this
./bin/pgedge-nla-server -http -tls \
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

See [Authentication Guide](authentication.md) for detailed token management.

### Token Best Practices

1. **Use expiration dates:**

    ```bash
    # Good: 90-day expiration
    ./bin/pgedge-nla-server -add-token -token-expiry "90d"

    # Avoid: Never-expiring tokens
    ./bin/pgedge-nla-server -add-token -token-expiry "never"
    ```

2. **Rotate tokens regularly:**

    - Set up calendar reminders for rotation
    - Create new token before old one expires
    - Remove old token after migration

3. **One token per service:**

    - Don't share tokens between applications
    - Easier to revoke if compromised
    - Better audit trail

4. **Store tokens securely:**

    ```bash
    # Use environment variables or secrets managers
    export MCP_TOKEN=$(vault kv get -field=token secret/mcp)

    # Never in code or logs
    curl -H "Authorization: Bearer $MCP_TOKEN" ...  # OK
    curl -H "Authorization: Bearer abc123..." ...    # Don't hardcode
    ```

5. **Token file permissions:**

    ```bash
    # Verify file permissions
    ls -la pgedge-nla-server-tokens.yaml  # Should be -rw------- (600)

    # Fix if needed
    chmod 600 pgedge-nla-server-tokens.yaml
    ```

### Connection Isolation

**Per-Token Database Connections:**

When authentication is enabled in HTTP/HTTPS mode, the MCP server implements **per-token connection isolation** to ensure security and prevent cross-user data access.

**How it works:**

- Each API token gets its own dedicated database connection pool
- Database connections are never shared between different tokens
- When a token expires, its database connections are automatically closed
- This prevents one user from accessing database resources opened by another user

**Security benefits:**

1. **Isolation**: Users with different tokens cannot interfere with each other's database sessions
2. **Session Security**: Temporary tables, prepared statements, and session variables are isolated per token
3. **Automatic Cleanup**: Expired tokens trigger automatic cleanup of their database connections
4. **Resource Management**: Connection pools are managed independently for each token

**When connection isolation is active:**

```bash
# Start server with authentication enabled
./bin/pgedge-nla-server -http -tls \
  -cert /path/to/cert.pem \
  -key /path/to/key.pem

# Server log will show:
# Connection isolation: ENABLED (per-token database connections)
```

**When connection isolation is NOT active:**

- Stdio mode (single-user, no authentication)
- HTTP mode with `-no-auth` flag (all requests share one connection)

**Monitoring connection pools:**

The server logs connection creation and cleanup:

```
Created new database connection for token hash: 5f4dcc3b5aa7 (total: 3)
Removed database connection for token hash: 5f4dcc3b5aa7 (remaining: 2)
```

**Best practices:**

- Always enable authentication for multi-user deployments
- Monitor server logs for connection pool growth
- Set appropriate token expiration times to prevent connection pool exhaustion
- Consider database connection limits when issuing tokens to many users

## TLS/Certificate Security

### Private Key Protection

**File permissions:**

```bash
# Private keys should be 600 (owner read/write only)
chmod 600 /path/to/server.key

# Verify
ls -la /path/to/server.key  # Should show -rw-------
```

**Storage:**


- Store keys outside web root
- Use hardware security modules (HSM) for high-security environments
- Never commit keys to version control

**Example - Secure Key Storage:**

```bash
# Store in /etc with restricted permissions
sudo mkdir -p /etc/pgedge-mcp/certs
sudo chmod 700 /etc/pgedge-mcp/certs
sudo cp server.key /etc/pgedge-mcp/certs/
sudo chmod 600 /etc/pgedge-mcp/certs/server.key
sudo chown pgedge:pgedge /etc/pgedge-mcp/certs/server.key
```

### Certificate Management

**Monitor expiration:**

```bash
# Check expiration date
openssl x509 -in server.crt -noout -dates

# Set up automated renewal (Let's Encrypt)
sudo certbot renew --dry-run
```

**Automated renewal with Let's Encrypt:**

```bash
# Install certbot
sudo apt-get install certbot  # Ubuntu/Debian

# Set up auto-renewal
sudo systemctl enable certbot.timer
sudo systemctl start certbot.timer

# Verify timer is active
sudo systemctl list-timers | grep certbot
```

**Certificate chain:**

- Always include intermediate certificates
- Use `-chain` flag with full chain file
- Test certificate chain: `openssl s_client -connect localhost:8080 -showcerts`

## Access Control

### Principle of Least Privilege

1. **Database users:**

    - Separate users for different access levels
    - Grant minimum required permissions
    - Regular audit of permissions

2. **Operating system users:**

    ```bash
    # Run server as dedicated user
    sudo useradd -r -s /bin/false pgedge
    sudo chown -R pgedge:pgedge /opt/pgedge-mcp
    ```

3. **File permissions:**

    ```bash
    # Binary: 755 (executable by all, writable by owner)
    chmod 755 /opt/pgedge-mcp/bin/pgedge-nla-server

    # Config files: 600 (readable/writable by owner only)
    chmod 600 /etc/pgedge-mcp/config.yaml
    chmod 600 /etc/pgedge-mcp/pgedge-nla-server-tokens.yaml

    # Secret file: 600 (CRITICAL - contains encryption key)
    chmod 600 /etc/pgedge-mcp/pgedge-pg-mcp-svr.secret

    # Certificates: 600 for keys, 644 for certs
    chmod 600 /etc/pgedge-mcp/certs/server.key
    chmod 644 /etc/pgedge-mcp/certs/server.crt
    ```

### Reverse Proxy Rate Limiting

Use nginx or HAProxy to add rate limiting:

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
journalctl -u pgedge-mcp | grep "Unauthorized"

# Count auth failures per IP
journalctl -u pgedge-mcp | grep "Unauthorized" | awk '{print $NF}' | sort | uniq -c | sort -rn

# Monitor configuration changes
journalctl -u pgedge-mcp | grep "set_pg_configuration"
```

### Security Monitoring Checklist

- [ ] Set up log aggregation (ELK, Splunk, etc.)
- [ ] Create alerts for authentication failures
- [ ] Monitor API token usage patterns
- [ ] Track database query patterns
- [ ] Set up intrusion detection (fail2ban, etc.)
- [ ] Monitor certificate expiration
- [ ] Regular token audits
- [ ] Review database user permissions

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
./bin/pgedge-nla-server -http 2>&1 | tee -a /var/log/pgedge-mcp/server.log
```

## Incident Response

### If Token is Compromised

1. **Immediate actions:**

    ```bash
    # Remove compromised token
    ./bin/pgedge-nla-server -remove-token <token-id>

    # Create new token
    ./bin/pgedge-nla-server -add-token -token-expiry "30d"

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

2. **Update MCP server:**

    ```bash
    # Update connection string
    export PGEDGE_POSTGRES_CONNECTION_STRING="postgres://mcp_readonly:new_password@host/db"

    # Restart server
    sudo systemctl restart pgedge-mcp
    ```

### If Server is Compromised

1. **Isolate:**

    ```bash
    # Block network access
    sudo ufw deny 8080

    # Stop service
    sudo systemctl stop pgedge-mcp
    ```

2. **Investigate:**

    - Review system logs
    - Check for unauthorized files
    - Analyze network connections

3. **Recover:**

    - Rebuild from known good state
    - Rotate all credentials
    - Review and update security measures

## Security Checklist

### Pre-Deployment

- [ ] Use strong passwords for database users
- [ ] Enable SSL/TLS for database connections
- [ ] Configure firewall rules
- [ ] Use read-only database user for queries
- [ ] Store credentials in environment variables or secrets manager
- [ ] Use HTTPS with valid certificates
- [ ] Set up API token authentication
- [ ] Configure token expiration
- [ ] Test in staging environment

### Production

- [ ] HTTPS enabled with valid CA certificate
- [ ] Authentication enabled (not using `-no-auth`)
- [ ] Tokens have expiration dates
- [ ] Private keys have 600 permissions
- [ ] Token file has 600 permissions
- [ ] Secret file has 600 permissions
- [ ] Secret file is backed up securely
- [ ] Server running as non-root user
- [ ] Firewall rules configured
- [ ] Reverse proxy with rate limiting
- [ ] Monitoring and alerting configured
- [ ] Backup procedures in place
- [ ] Incident response plan documented
- [ ] Regular security audits scheduled

### Ongoing

- [ ] Rotate API tokens quarterly
- [ ] Rotate database passwords quarterly
- [ ] Review access logs weekly
- [ ] Update certificates before expiration
- [ ] Review and update firewall rules
- [ ] Audit database user permissions
- [ ] Review token list for unused tokens
- [ ] Update software and dependencies
- [ ] Test backup and recovery procedures
- [ ] Conduct security training for team

## Related Documentation

- [Authentication Guide](authentication.md) - API token management
- [Deployment Guide](deployment.md) - Production deployment
- [Configuration Guide](configuration.md) - Secure configuration
- [Troubleshooting Guide](troubleshooting.md) - Security-related issues
