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

The AI may execute destructive queries without confirmation. There is no
"are you sure?" prompt before data modifications.

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

When connected to a write-enabled database:

- **Web client**: Shows a prominent yellow warning banner
- **CLI**: Displays `[WRITE-ENABLED]` marker and warning messages
- **System info**: The `allow_writes` field in `pg://system_info` shows `true`

