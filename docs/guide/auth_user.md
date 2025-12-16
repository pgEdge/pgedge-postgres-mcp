# Authentication - User Management

User accounts provide interactive authentication with session-based access. Users authenticate with username and password to receive a 24-hour session token.

- Use an [*API Token*](auth_token.md) for direct machine-to-machine access.  Tokens are long-lived and easily managed by administrators.
- Use a [*User Account*](auth_user.md) for interactive applications; an account is session-based, and users can manage own password access.

By default, user details are stored in a file named `pgedge-postgres-mcp-users.yaml` in the same directory as the MCP server binary.  The file has bcrypt-hashed passwords (cost factor 12), and requires that permissions are set to `0600` (owner read/write only).  Each session token generated as a result of user authentication is 32 bytes, and is valid for 24-hour validity.

When configuring user authentication, you should keep the following best practices in mind:

**Password Security**

- Enforce minimum complexity requirements for passwords.
- Ensure passwords are not logged or displayed.
- Always use HTTPS for secure password transmission in production.
- Regularly prompt your users to update passwords.

**Session Management**

- Store session tokens securely (not in localStorage for web apps)
- Users should re-authenticate before a token expires.
- Implement proper logout processing, with client-side token deletion.
- Consider implementing per-user session limits for the concurrent session count.

**Account Security**

- Configure `max_failed_attempts_before_lockout` to automatically disable accounts after repeated failed login attempts.
- Log successful and failed authentication events.
- Disable accounts after a period of inactivity.
- Use annotations to track user roles and permissions.

When integrating authentication with your applications, check the session token's expiration date and time before use; if the token is valid, connect with the token:

```python
# Good: Check token expiration before use
from datetime import datetime, timezone

def is_token_expired(expiry_str):
    expiry = datetime.fromisoformat(expiry_str.replace('Z', '+00:00'))
    return datetime.now(timezone.utc) >= expiry

if not client.session_token or is_token_expired(client.token_expiry):
    client.authenticate(username, password)

# Now use the token
result = client.call_tool("query_database", {...})
```


## User Account Management Syntax

You can add a user at the command line in interactive mode with the following command:

```bash
# Add user with prompts
./bin/pgedge-postgres-mcp -add-user
```

The server will prompt you for:

- a unique username for the account
- a password (hidden, with confirmation)
- optionally, a description (e.g., "Alice Smith - Developer")

You can also add username/password pairs non-interactively with the following syntax:

```bash
# Add user with all details specified
./bin/pgedge-postgres-mcp -add-user \
  -username alice \
  -password "SecurePassword123!" \
  -user-note "Alice Smith - Developer"
```

To generate a list of users, use the command:

```bash
./bin/pgedge-postgres-mcp -list-users
```

Output:
```
Users:
==========================================================================================
Username             Created                   Last Login           Status      Annotation
------------------------------------------------------------------------------------------
alice                2024-10-30 10:15          2024-11-14 09:30     Enabled     Developer
bob                  2024-10-15 14:20          Never                Enabled     Admin
charlie              2024-09-01 08:00          2024-10-10 16:45     DISABLED    Former emp
==========================================================================================
```

To update a user account, use the following syntax variations:

```bash
# Update password
./bin/pgedge-postgres-mcp -update-user -username alice

# Update with new password from command line (less secure)
./bin/pgedge-postgres-mcp -update-user \
  -username alice \
  -password "NewPassword456!"

# Update annotation only
./bin/pgedge-postgres-mcp -update-user \
  -username alice \
  -user-note "Alice Smith - Senior Developer"
```

To disable or enable a user account, use the following commands:

```bash
# Disable a user account (prevents login)
./bin/pgedge-postgres-mcp -disable-user -username charlie

# Re-enable a user account
./bin/pgedge-postgres-mcp -enable-user -username charlie
```

To delete a user account:

```bash
# Delete user (with confirmation prompt)
./bin/pgedge-postgres-mcp -delete-user -username charlie
```

To specify the location of a custom user file:

```bash
# Specify custom user file path
./bin/pgedge-postgres-mcp -user-file /etc/pgedge/pgedge-postgres-mcp-users.yaml -list-users
```