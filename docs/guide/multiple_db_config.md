# Configuring the MCP Server for Multiple Databases

The MCP server supports configuring multiple PostgreSQL databases,
allowing users to switch between different database connections at runtime.
This is particularly useful for environments with separate development,
staging, and production databases, or when providing access to multiple
projects.

## Configuring Multiple Databases

Each database must have a unique name that users reference when switching
connections:

```yaml
databases:
  - name: "production"
    host: "prod-db.example.com"
    port: 5432
    database: "myapp"
    user: "readonly_user"
    sslmode: "require"
    available_to_users: []  # All users can access

  - name: "staging"
    host: "staging-db.example.com"
    port: 5432
    database: "myapp_staging"
    user: "developer"
    sslmode: "prefer"
    available_to_users:
      - "alice"
      - "bob"
      - "qa_team"

  - name: "development"
    host: "localhost"
    port: 5432
    database: "myapp_dev"
    user: "developer"
    sslmode: "disable"
    available_to_users:
      - "alice"
      - "bob"
```

### Client Certificate Authentication

A database entry can authenticate to PostgreSQL with a client
certificate instead of, or alongside, a password. Set `sslcert` and
`sslkey` together to supply the client certificate and its private
key; set `sslrootcert` to verify the server's certificate against a
trusted CA:

```yaml
databases:
  - name: "secure"
    host: "secure-db.example.com"
    port: 5432
    database: "myapp"
    user: "cert_user"
    sslmode: "verify-full"
    sslcert: "/etc/pgedge/certs/client.crt"
    sslkey: "/etc/pgedge/certs/client.key"
    sslrootcert: "/etc/pgedge/certs/ca.crt"
    available_to_users: []
```

The `sslcert` and `sslkey` fields must be set together; providing
only one causes the server to reject the configuration at startup.
The `sslrootcert` field is independent and can be set on its own to
verify the server's certificate without client certificate
authentication, but only takes effect under `sslmode: "require"`,
`"verify-ca"`, or `"verify-full"`; under `"disable"`, `"allow"`, or
`"prefer"` (the default) the server certificate is never checked
against it and the value is silently ignored, matching libpq and
`psql`.

### Access Control

The `available_to_users` field controls which session users can access each
database:

- **Empty list (`[]`)**: All authenticated users can access the database
- **User list**: Only the specified usernames can access the database
- **API tokens**: Bound to a specific database via the token's `database` field
  (see [Authentication Guide](authentication.md))

**Access control is enforced in HTTP mode only.** In STDIO mode or when
authentication is disabled (`--no-auth`), all databases are accessible to
everyone.

### Startup Behavior

In STDIO mode the server attempts to connect to every configured
database at startup. Each connection is attempted independently;
a failure is logged as a warning and does not prevent the server
from starting. Databases that are unreachable at startup are
marked `unavailable` and connected on demand when a tool or user
selects them.

In HTTP mode with authentication enabled, all database connections
are created on demand; no connections are made at startup.

### Default Database Selection

When a user connects, the system automatically selects a default
database using this priority:

1. **Saved preference**: If the user previously selected a database
   and the database is still accessible, that database is used.
2. **First accessible database**: Otherwise, the first database in
   the configuration list that the user has access to is selected.
3. **No database**: If no databases are accessible, database
   operations fail with an appropriate error message.

**Example scenarios:**

| User | Accessible Databases | Default Selection |
|------|---------------------|-------------------|
| alice | production, staging, development | production (first) |
| bob | production, staging, development | production (first) |
| qa_team | production, staging | production (first) |
| guest | production | production (only option) |
| unknown | (none) | Error: no accessible databases |

### Runtime Database Switching

Users can switch between accessible databases at runtime using the client
interfaces:

**CLI Client:**

```
/list databases        # Show available databases
/show database         # Show current database
/set database staging  # Switch to staging database
```

**Web UI:**

Click the database icon in the status banner to open the database selector.
Select a database from the list to switch connections.

**Note:** Database switching is disabled while an LLM query is being
processed to prevent data consistency issues.

### LLM Database Switching

You can optionally allow the LLM to list and switch databases using MCP
tools. This feature is disabled by default for security reasons.

To enable LLM database switching, add the following to your configuration:

```yaml
builtins:
  tools:
    llm_connection_selection: true
```

When enabled, the LLM has access to two additional tools:

- `list_database_connections`: Lists databases available for switching
- `select_database_connection`: Switches to a specified database

#### Excluding Databases from LLM Switching

You can prevent specific databases from being visible to LLM switching
tools using the `allow_llm_switching` option:

```yaml
databases:
  - name: "production"
    host: "prod-db.example.com"
    database: "myapp"
    allow_llm_switching: false  # Hidden from LLM

  - name: "staging"
    host: "staging-db.example.com"
    database: "myapp_staging"
    # allow_llm_switching defaults to true
```

When `allow_llm_switching: false` is set:

- The database does not appear in `list_database_connections` results
- Attempts to switch to it via `select_database_connection` are denied
- Manual switching via CLI commands or web UI is unaffected
- API token bindings and `available_to_users` restrictions still apply

This allows administrators to grant LLM access to development and staging
databases while keeping production databases accessible only through
manual user selection.

### Database Selection Persistence

When a user selects a database:

- The selection is saved to the user's session preferences
- On subsequent connections, the saved preference is restored (if still
  accessible)
- If the preferred database is no longer accessible (e.g., removed from
  configuration or user permissions changed), the system falls back to the
  first accessible database

## Client Integration Examples

Programmatic clients can list and switch databases using the
REST API endpoints. All requests require Bearer token
authentication.

### Python Example

The `DatabaseManager` class wraps the database API endpoints.

In the following example, the `DatabaseManager` class lists
databases, selects a database, and retrieves the current
connection:

```python
import requests

class DatabaseManager:
    """Manage database connections via the MCP API."""

    def __init__(self, base_url, token):
        self.base_url = base_url
        self.headers = {
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json"
        }

    def list_databases(self):
        """List all accessible databases."""
        response = requests.get(
            f"{self.base_url}/api/databases",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()

    def select_database(self, name):
        """Switch to a different database."""
        response = requests.post(
            f"{self.base_url}/api/databases/select",
            headers=self.headers,
            json={"name": name}
        )
        response.raise_for_status()
        data = response.json()
        if not data.get("success"):
            raise Exception(data.get("error", "Unknown"))
        return data

    def get_current(self):
        """Get the currently selected database name."""
        data = self.list_databases()
        return data.get("current")


# Usage
db = DatabaseManager(
    "http://localhost:8080", "YOUR_TOKEN"
)

# List available databases
info = db.list_databases()
print(f"Current: {info['current']}")
for database in info["databases"]:
    print(f"  - {database['name']} ({database['host']})")

# Switch to staging
result = db.select_database("staging")
print(f"Switched to: {result['current']}")
```

### JavaScript Example

The JavaScript client provides the same functionality using
the `fetch` API.

In the following example, the `DatabaseManager` class lists
and switches databases using asynchronous methods:

```javascript
class DatabaseManager {
    constructor(baseUrl, token) {
        this.baseUrl = baseUrl;
        this.headers = {
            "Authorization": `Bearer ${token}`,
            "Content-Type": "application/json"
        };
    }

    async listDatabases() {
        const response = await fetch(
            `${this.baseUrl}/api/databases`,
            { headers: this.headers }
        );
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        return response.json();
    }

    async selectDatabase(name) {
        const response = await fetch(
            `${this.baseUrl}/api/databases/select`,
            {
                method: "POST",
                headers: this.headers,
                body: JSON.stringify({ name })
            }
        );
        const data = await response.json();
        if (!data.success) {
            throw new Error(data.error);
        }
        return data;
    }
}

// Usage
const db = new DatabaseManager(
    "http://localhost:8080", "YOUR_TOKEN"
);
const info = await db.listDatabases();
console.log(`Current: ${info.current}`);
```

### Handling Access Denied Errors

Clients should handle errors when a user tries to switch to
a database the user cannot access.

In the following example, the client catches access denied
and not found errors when selecting a database:

```python
try:
    db.select_database("production")
except Exception as e:
    if "Access denied" in str(e):
        print("You do not have access to production.")
        print("Available databases:")
        for d in db.list_databases()["databases"]:
            print(f"  - {d['name']}")
    elif "not found" in str(e):
        print("Database not found.")
    else:
        raise
```

## Configuration Settings Reference

The following table summarizes the settings that control
LLM database switching behavior.

| Setting | Scope | Default | Purpose |
|---|---|---|---|
| `llm_connection_selection` | Global | `false` | Enables the LLM database switching tools. |
| `allow_llm_switching` | Per-database | `true` | Controls whether the LLM can see and switch to the database. |

### Decision Flow

The server evaluates three conditions to determine whether
the LLM can access a database.

1. Check whether `llm_connection_selection` is enabled
   globally. If the setting is disabled, the LLM cannot
   see or switch any databases.
2. Check whether `allow_llm_switching` is `true` for the
   specific database. If the setting is `false`, the LLM
   cannot see or switch to that database.
3. Check whether the authenticated user has access through
   `available_to_users`. If the user lacks access, the
   database remains hidden.

### Example Configuration

The following configuration gives the LLM access to
development and staging but not production:

```yaml
builtins:
    tools:
        llm_connection_selection: true

databases:
    - name: "production"
      host: "prod-db.example.com"
      database: "myapp"
      allow_llm_switching: false

    - name: "staging"
      host: "staging-db.example.com"
      database: "myapp_staging"
      # allow_llm_switching defaults to true

    - name: "development"
      host: "localhost"
      database: "myapp_dev"
      # allow_llm_switching defaults to true
```

With this configuration:

- Users can manually switch to any database they
  have access to.
- The LLM can only discover and switch between the
  staging and development databases.
- The production database is protected from
  LLM-initiated switching.

## See Also

The following resources provide additional information:

- The [Authentication Guide](authentication.md) covers
  token management and access control.
- The [API Reference](../developers/api-reference.md)
  documents all available REST endpoints.
- The [Client Examples](../developers/client-examples.md)
  page provides additional integration patterns.
- The [Row-Level Security](../advanced/row-level-security.md)
  guide explains fine-grained data access controls.
