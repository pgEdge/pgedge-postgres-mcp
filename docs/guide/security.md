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
it does not do. Two of the layers above are themselves PostgreSQL
settings: the session-level `default_transaction_read_only` value
and the read-only access mode requested on `BEGIN`. PostgreSQL lets
a connected role change either one through ordinary SQL, which is
why the guard exists to intercept the attempt. The other two layers
are not settings at all: enforcing the extended query protocol and
running the statement guard both happen in server code, and no
client action can reach or disable them. The guard's own limitation
is different and narrower: it can only reject the constructs it
recognises, so read it as a way of closing known bypasses and
recording attempts, not as a guarantee.

The only enforcement a client cannot undo comes from database
privileges, and that guarantee extends only as far as the
privileges you actually revoke. Grant the server a role whose
`INSERT`, `UPDATE`, and `DELETE` privileges have been revoked, as
described in [Security Management](security_mgmt.md), and no
amount of transaction-mode manipulation can route a write through
that role, because the restriction no longer depends on a setting
the client controls. Revoking those privileges does not close
every avenue by itself; schema-modification privileges such as
`CREATE`, `DROP`, and `ALTER`, grants on future objects, and
execute privileges on transaction-escaping functions remain the
deploying operator's responsibility to lock down separately. Do
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

The bundled CLI and Web UI prompt for user confirmation before
executing recognized or server-advertised write operations on
write-enabled databases: `query_database` statements classified as
DDL (`CREATE`, `DROP`, `ALTER`, `TRUNCATE`) or DML (`INSERT`,
`UPDATE`, `DELETE`), and any custom tool the server advertises with
`readOnlyHint: false`.

Confirmation is a property of these two clients, not of the
server; the server itself executes any call it accepts. A
third-party MCP client applies its own approval model, so treat
`allow_writes: true` as granting write access to whichever client
holds the connection, whether or not that client asks anybody
first.

The confirmation behavior differs by client:

- The CLI displays the operation and prompts
  `Execute this operation? [y/N]:` with No as the default.
- The Web UI shows a dialog with Cancel and Execute buttons,
  containing the SQL query for `query_database` or the tool
  name and its arguments for a custom tool.
- Declining the operation prevents execution and instructs the
  LLM not to retry it.
- The client treats unknown query types as writes for safety.

To let any client make the same distinction, the server publishes
MCP annotations on each tool. It sets `destructiveHint: true` and
`readOnlyHint: false` on `query_database` when writes are enabled,
and on a custom tool whenever that tool could modify the database:
that is, on any `pl-do` or `pl-func` tool, and on a `sql` tool
whose statement is not plainly a read. A tool that publishes no
annotation is not treated as a write, so a client cannot rely on
their absence to mean anything.

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

## Untrusted Database Content

Everything the server returns to a client, including query results
and the document text returned by similarity search, was written by
whoever populated the database. That is not necessarily the person
asking the question, so retrieved content can carry instructions of
its own. A document that asks an assistant to copy itself into the
table it came from will be read again by the next session that
searches for it, and an assistant that follows those instructions
propagates the document rather than reporting it.

This is addressed in the only two places it can be, one in the CLI
client and one in the server:

- The system prompt the CLI sends states that everything returned
  by a tool is untrusted content, that retrieved text is data to
  report rather than instructions to follow, and that only the
  user's own messages direct the assistant's actions. The web
  client sends no system prompt at all, so it carries neither this
  instruction nor the pre-existing read-only safety block.
- The server publishes annotations on tools that can modify the
  database, and both bundled clients hold such a call for the
  user's confirmation before it runs.

Neither measure is a guarantee. A model can be argued out of any
instruction it has been given, and a client is free to ignore an
annotation, so treat both as mitigations that reduce the chance of
a compliant assistant rather than as controls that prevent one.

What does prevent a document from propagating itself is the absence
of write access. A connection in read-only mode cannot copy a row,
and a database role with write privileges revoked cannot do so even
if read-only mode is defeated. Configure production connections
that way, as described in
[Security Management](security_mgmt.md), and this class of attack
cannot complete regardless of what any retrieved document says or
which client is driving the server.

## Provider Credentials in Error Messages

An LLM or embedding provider decides what its own error responses say,
and several of them quote the credential they rejected. OpenAI's reply
to a failed authentication names the key it was given, in a partially
masked form that still reveals its opening characters. The shared
pgEdge LLM library relays a provider's message into the error it
returns, so that text reaches anywhere the error is reported.

The server removes credentials from such text before it leaves the
process. Redaction applies in three places:

- Responses from the `/api/llm/` endpoints, which the library's proxy
  generates from the provider's message.
- Trace entries, which persist in the file named by `-trace-file` and
  are often attached to support tickets.
- Errors that the CLI reports to its own operator, which may be
  redirected to a log.

Both the credentials the server was configured with and anything
matching a known key format are replaced with `[REDACTED]`. The
surrounding message survives, so the provider, the status code, and
the reason for the failure are still reported.

Understand the limits of this. Filtering the `/api/llm/` response is
best-effort, not absolute: it inspects each write to the response body
on its own, so a credential split across two writes would evade it.
This does not arise for a JSON error body, which is encoded in one
go, or a server-sent event, which is written as a unit, but it is a
genuine limit of filtering a stream rather than fixing the source.
Redaction is a filter over text that
should not have contained a credential in the first place, so it
recognises the formats this project knows about and the values it was
given; a provider inventing a new format, or quoting a key in a way
these patterns do not match, would not be caught. The durable fix
belongs in the provider client library, so that a credential never
reaches an error value at all. Treat this as a safety net beneath
that, and continue to keep provider keys out of any output you
publish.
