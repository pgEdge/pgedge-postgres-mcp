# Security Guide

This document outlines security considerations and best practices for
deploying and using the Natural Language Agent. Database credentials are
configured when the MCP server starts via:

- command-line options.
- the configuration file (YAML format).
- environment variables (`PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`,
  `PGPASSWORD`).

You should never:

- Commit `.env` files or credentials to version control.
- Use `.gitignore` for configuration files that contain credentials
  (instead, exclude them entirely and use templates).
- Hardcode security details in scripts.

You should instead:

- Use environment variables in controlled environments (containers,
  orchestrators) for passing credentials at runtime.
- Consider using secret management systems (Vault, AWS Secrets Manager,
  etc.) for production deployments.
- Use a `~/.pgpass` file or similar secure credential storage for local
  development.

## Security Checklist

**Pre-Deployment**

- Use strong passwords for database users.
- Enable SSL/TLS for database connections.
- Configure firewall rules.
- Use read-only database user for queries.
- Store credentials in environment variables or secrets manager.
- Use HTTPS with valid certificates.
- Set up API token authentication.
- Configure token expiration.
- Test in staging environment.

**Production**

- Enable HTTPS with a valid CA certificate.
- Enable authentication (do not use `-no-auth`).
- Configure tokens with expiration dates.
- Set private keys to 600 permissions.
- Set token file to 600 permissions.
- Set secret file to 600 permissions.
- Back up secret file securely.
- Run server as a non-root user.
- Configure firewall rules.
- Configure reverse proxy with rate limiting.
- Configure monitoring and alerting.
- Establish backup procedures.
- Document incident response plan.
- Schedule regular security audits.

**Ongoing**

- Rotate API tokens quarterly.
- Rotate database passwords quarterly.
- Review access logs weekly.
- Update certificates before expiration.
- Review and update firewall rules.
- Audit database user permissions.
- Review token list for unused tokens.
- Update software and dependencies.
- Test backup and recovery procedures.
- Conduct security training for team.

**Security Monitoring Checklist**

- Set up log aggregation (ELK, Splunk, etc.).
- Create alerts for authentication failures.
- Monitor API token usage patterns.
- Track database query patterns.
- Set up intrusion detection (fail2ban, etc.).
- Monitor certificate expiration.
- Conduct regular token audits.
- Review database user permissions.

## Database Write Access

By default, all database connections operate in **read-only mode**. This is
a critical safety feature that prevents the AI from accidentally or
unintentionally modifying your data.

### Read-Only Protection

The server enforces read-only mode through several layers of
defense-in-depth:

- Every statement runs through the PostgreSQL extended query
  protocol, which carries exactly one statement per message. The
  server therefore rejects any attempt to append extra statements
  to a request, which is the channel through which a bypass would
  otherwise smuggle its own transaction control.
- A read-only access mode requested as part of `BEGIN` itself, so
  the transaction is never briefly writable and the mode cannot
  fail to apply separately.
- A session-level `default_transaction_read_only` setting applied
  when each database connection is established, and re-applied
  when the connection returns to the pool. A connection whose
  state cannot be confirmed is discarded rather than reused.
- A statement guard that rejects constructs known to escape
  read-only mode. These include read-write transaction modes,
  `SET SESSION CHARACTERISTICS`, `RESET ALL`, `DISCARD`,
  transaction control, `SET ROLE`, `SET SESSION AUTHORIZATION`,
  changes to the `transaction_read_only` or
  `default_transaction_read_only` settings, and operations whose
  effects fall outside the transaction altogether, such as `DO`
  blocks, `COPY ... TO PROGRAM`, server-side file functions, and
  `dblink`.

When the database connection is in read-only mode, the system
prompt sent to the LLM includes explicit instructions that forbid
attempts to bypass the read-only restrictions.

### The Limits of Read-Only Mode

Read-only mode is a safety feature, and you should understand what
it does not do. Each of the layers above is a setting that the
connected role is entitled to change; the guard in particular can
only reject the constructs it recognises, so it should be read as a
way of closing known bypasses and recording attempts, not as a
guarantee.

The only enforcement a client cannot undo comes from database
privileges. Grant the server a role whose write privileges have
been revoked, as described in
[Security Management](security_mgmt.md), and no amount of
transaction-mode manipulation can reach your data, because the
restriction no longer depends on a setting the client controls. Do
this for any deployment where the client is untrusted, and treat
read-only mode as protection against accident rather than against
an adversary.

### The `allow_writes` Setting

Each database connection can be configured with `allow_writes: true` to
enable write operations. **This setting should be used with extreme caution.**

```yaml
databases:
    - name: "development"
      host: "localhost"
      # ... other settings ...
      allow_writes: true  # DANGEROUS - enables data modifications
```

### Risks of Enabling Write Access

When write access is enabled, the AI can execute:

- `INSERT` - Add new data
- `UPDATE` - Modify existing data
- `DELETE` - Remove data
- `TRUNCATE` - Empty entire tables
- `DROP` - Delete tables, indexes, or other objects
- `ALTER` - Modify table structures
- Any other data-modifying SQL statements

### Write Query Confirmation

The CLI and Web UI prompt for user confirmation before executing
write queries on write-enabled databases. This safeguard applies
to DDL statements (`CREATE`, `DROP`, `ALTER`, `TRUNCATE`) and DML
statements (`INSERT`, `UPDATE`, `DELETE`).

The confirmation behavior differs by client:

- The CLI displays the SQL query and prompts
  `Execute this query? [y/N]:` with No as the default.
- The Web UI shows a dialog containing the SQL query with
  Cancel and Execute buttons.
- Declining the query prevents execution and instructs the
  LLM not to retry the operation.
- The server treats unknown query types as writes for safety.

Third-party MCP clients may also prompt for confirmation. The
server sets `destructiveHint: true` and `readOnlyHint: false`
annotations on the `query_database` tool when writes are enabled.
These annotations follow the MCP specification and signal that
the tool may modify data.

### Recommendations

**Never enable writes on production databases.** Use read-only mode for
production systems to prevent accidental data loss or corruption.

**Only enable writes for:**

- Development/test databases with disposable data
- Sandboxed environments isolated from production
- Specific use cases where write operations are required and understood

**Additional safeguards when using write access:**

- Use a dedicated database user with limited permissions
- Enable database-level audit logging
- Maintain regular backups
- Consider using database snapshots before AI interactions
- Monitor AI-generated queries for unexpected patterns

### UI Indicators

When connected to a write-enabled database, the clients display
warnings and require confirmation for write queries:

- The Web client shows a prominent yellow warning banner and
  a confirmation dialog before executing write queries.
- The CLI displays a `[WRITE-ENABLED]` marker, warning
  messages, and a confirmation prompt before write queries.
- The `allow_writes` field in `pg://system_info` shows
  `true` for write-enabled connections.

