# Security Guide

This document outlines security considerations and best practices for
deploying and using the Natural Language Agent. Database credentials are
configured when the MCP server starts via:

- command-line options.
- the configuration file (YAML format).
- environment variables (`PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`,
  `PGPASSWORD`).

You should never:

- Use environment variables for sensitive credentials.
- Commit secret files or credentials to version control.
- Use `.gitignore` for configuration files that contain
  credentials.
- Never hardcode security details in scripts.

You should instead:

- Consider using secret management systems (Vault, AWS Secrets Manager,
  etc.).
- In production, use a `~/.pgpass` file or similar secure credential
  storage.

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

