# MCP Tools

The pgEdge MCP Server provides four tools that enable natural language database interaction and configuration management.

## Available Tools

### query_database

Executes a natural language query against the PostgreSQL database. Supports dynamic connection strings to query different databases.

**Input Examples**:

Basic query:
```json
{
  "query": "Show me all users created in the last week"
}
```

Query with temporary connection:
```json
{
  "query": "Show me table list at postgres://localhost:5433/other_db"
}
```

Set new default connection:
```json
{
  "query": "Set default database to postgres://localhost/analytics"
}
```

**Output**:
```
Natural Language Query: Show me all users created in the last week

Generated SQL:
SELECT * FROM users WHERE created_at >= NOW() - INTERVAL '7 days' ORDER BY created_at DESC

Results (15 rows):
[
  {
    "id": 123,
    "username": "john_doe",
    "created_at": "2024-10-25T14:30:00Z",
    ...
  },
  ...
]
```

**Security**: All queries are executed in read-only transactions using `SET TRANSACTION READ ONLY`, preventing INSERT, UPDATE, DELETE, and other data modifications. Write operations will fail with "cannot execute ... in a read-only transaction".

### get_schema_info

Retrieves database schema information including tables, views, columns, data types, and comments from pg_description.

**Input** (optional):
```json
{
  "schema_name": "public"
}
```

**Output**:
```
Database Schema Information:
============================

public.users (TABLE)
  Description: User accounts and authentication
  Columns:
    - id: bigint
    - username: character varying(255)
      Description: Unique username for login
    - created_at: timestamp with time zone (nullable)
      Description: Account creation timestamp
    ...
```

### set_pg_configuration

Sets PostgreSQL server configuration parameters using ALTER SYSTEM SET. Changes persist across server restarts. Some parameters require a restart to take effect.

**Input**:
```json
{
  "parameter": "max_connections",
  "value": "200"
}
```

Use "DEFAULT" as the value to reset to default:
```json
{
  "parameter": "work_mem",
  "value": "DEFAULT"
}
```

**Output**:
```
Configuration parameter 'max_connections' updated successfully.

Parameter: max_connections
Description: Sets the maximum number of concurrent connections
Type: integer
Context: postmaster

Previous value: 100
New value: 200

⚠️  WARNING: This parameter requires a server restart to take effect.
The change has been saved to postgresql.auto.conf but will not be active until the server is restarted.

SQL executed: ALTER SYSTEM SET max_connections = '200'
```

**Security Considerations**:
- Requires PostgreSQL superuser privileges
- Changes persist across server restarts via `postgresql.auto.conf`
- Test configuration changes in development before applying to production
- Some parameters require a server restart to take effect
- Keep backups of configuration files before making changes

### read_resource

Reads MCP resources by their URI. Provides access to system information and statistics.

**Input Examples**:

List all available resources:
```json
{
  "list": true
}
```

Read a specific resource:
```json
{
  "uri": "pg://system_info"
}
```

**Available Resource URIs**:
- `pg://settings` - PostgreSQL configuration parameters
- `pg://system_info` - PostgreSQL version, OS, and build architecture
- `pg://stat/activity` - Current connections and queries
- `pg://stat/database` - Database-wide statistics
- `pg://stat/user_tables` - Per-table statistics
- `pg://stat/user_indexes` - Index usage statistics
- `pg://stat/replication` - Replication status
- `pg://stat/bgwriter` - Background writer statistics
- `pg://stat/wal` - WAL statistics (PostgreSQL 14+ only)

See [RESOURCES.md](RESOURCES.md) for detailed information about each resource.
