# MCP Tools

The pgEdge MCP Server provides ten tools that enable SQL database interaction, configuration management, connection management, and server information.

## Available Tools

### server_info

Get information about the MCP server itself, including server name, company, and version.

**Input**: None (no parameters required)

**Output**:
```
Server Information:
===================

Server Name:    pgEdge PostgreSQL MCP Server
Company:        pgEdge, Inc.
Version:        1.0.0

Description:    An MCP (Model Context Protocol) server that enables AI assistants to interact with PostgreSQL databases through SQL queries and schema exploration.

License:        PostgreSQL License
Copyright:      © 2025, pgEdge, Inc.
```

**Use Cases**:

- Verify server version for compatibility and troubleshooting
- Get quick reference to server information during support requests

### query_database

Executes a SQL query against the PostgreSQL database. Supports dynamic connection strings to query different databases.

**IMPORTANT**: Using `AT postgres://...` or `SET DEFAULT DATABASE` for temporary connections does NOT modify saved connections - these are session-only changes.

**Input Examples**:

Basic query:
```json
{
  "query": "SELECT * FROM users WHERE created_at >= NOW() - INTERVAL '7 days' ORDER BY created_at DESC"
}
```

Query with temporary connection:
```json
{
  "query": "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AT postgres://localhost:5433/other_db"
}
```

Set new default connection:
```json
{
  "query": "SET DEFAULT DATABASE postgres://localhost/analytics"
}
```

**Output**:
```
SQL Query: SELECT * FROM users WHERE created_at >= NOW() - INTERVAL '7 days' ORDER BY created_at DESC

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

**Note**: When using MCP clients like Claude Desktop, the client's LLM can translate natural language into SQL queries that are then executed by this server.

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
- `pg://stat/replication` - Replication status

See [Resources](resources.md) for detailed information about each resource.
## Connection Management Tools

### add_database_connection

Save a database connection with an alias for later use. Connections are persisted and available across sessions. Passwords are encrypted using AES-256-GCM encryption.

**Input**:
```json
{
  "alias": "production",
  "host": "prod-host.example.com",
  "port": 5432,
  "user": "dbuser",
  "password": "securepassword",
  "dbname": "mydb",
  "sslmode": "verify-full",
  "sslrootcert": "/path/to/ca.crt",
  "description": "Production database server"
}
```

**Parameters**:

- `alias` (required): Friendly name for the connection (e.g., "production", "staging")
- `host` (required): Database hostname or IP address
- `port` (optional, default: 5432): Database port number
- `user` (required): Database username
- `password` (optional): Database password (will be encrypted before storage)
- `dbname` (optional, default: same as user): Database name
- `sslmode` (optional): SSL mode - disable, allow, prefer, require, verify-ca, verify-full
- `sslcert` (optional): Path to client certificate file
- `sslkey` (optional): Path to client key file
- `sslrootcert` (optional): Path to root CA certificate file
- `sslpassword` (optional): Password for client key (will be encrypted before storage)
- `sslcrl` (optional): Path to certificate revocation list
- `connect_timeout` (optional): Connection timeout in seconds
- `application_name` (optional): Application name for connection tracking
- `description` (optional): Notes about this connection

**Output**:
```
Successfully saved connection 'production'
Host: prod-host.example.com:5432
User: dbuser
Database: mydb
SSL Mode: verify-full
Description: Production database server
```

**Security**:

- Passwords are encrypted using AES-256-GCM before storage
- Encryption key is stored in a separate secret file (default: `pgedge-postgres-mcp.secret`)
- Secret file is auto-generated on first run with restricted permissions (0600)

**Storage**:

- **With authentication enabled**: Stored per-token in `pgedge-postgres-mcp-server-tokens.yaml`
- **With authentication disabled**: Stored globally in preferences file `pgedge-postgres-mcp-prefs.yaml`

### remove_database_connection

Remove a saved database connection by its alias.

**Input**:
```json
{
  "alias": "staging"
}
```

**Output**:
```
Successfully removed connection 'staging'
```

### list_database_connections

List all saved database connections for the current user/session.

**Input**: None (no parameters required)

**Output**:
```
Saved Database Connections:
============================

Alias: production
  Host: prod-host.example.com:5432
  User: dbuser
  Database: mydb
  Maintenance DB: postgres
  SSL Mode: verify-full
  SSL Root Cert: /path/to/ca.crt
  Description: Production database
  Created: 2025-01-15 10:00:00
  Last Used: 2025-01-15 14:30:00

Alias: staging
  Host: staging-host.example.com:5432
  User: dbuser
  Database: mydb
  Maintenance DB: postgres
  SSL Mode: require
  Description: Staging environment
  Created: 2025-01-15 10:05:00
  Last Used: Never

Total: 2 saved connection(s)
```

**Security**:

- Passwords are never displayed in output
- All passwords are stored encrypted using AES-256-GCM encryption
- Connection details are displayed without sensitive credential information

### edit_database_connection

Permanently modify an existing saved connection's configuration. You can update any or all connection parameters. Only non-empty fields will be updated.

**IMPORTANT**: Only use this tool when explicitly asked to update, change, or edit a saved connection. To temporarily connect to a different database, use `set_database_connection` with a full connection string instead.

**Input**:
```json
{
  "alias": "production",
  "host": "new-prod-host.example.com",
  "port": 5433,
  "user": "newuser",
  "password": "newpassword",
  "dbname": "newdb",
  "sslmode": "verify-full",
  "sslrootcert": "/path/to/new-ca.crt",
  "description": "Updated production server"
}
```

**Parameters**:

- `alias` (required): The alias of the connection to update
- `host` (optional): New database hostname or IP address
- `port` (optional): New database port number
- `user` (optional): New database username
- `password` (optional): New database password (will be encrypted before storage)
- `dbname` (optional): New database name
- `sslmode` (optional): New SSL mode
- `sslcert` (optional): New path to client certificate file
- `sslkey` (optional): New path to client key file
- `sslrootcert` (optional): New path to root CA certificate file
- `sslpassword` (optional): New password for client key (will be encrypted before storage)
- `sslcrl` (optional): New path to certificate revocation list
- `connect_timeout` (optional): New connection timeout in seconds
- `application_name` (optional): New application name
- `description` (optional): New description

**Output**:
```
Successfully updated connection 'production'
Updated: host, port, user, password, sslmode, description
```

**Note**: Only provided parameters will be updated. Empty or omitted parameters will retain their current values.

### set_database_connection (Enhanced)

Set the database connection for the current session. Now supports both connection strings and aliases.

**IMPORTANT**: This tool does NOT modify saved connections - it only sets which connection to use for this session. To connect to a different database temporarily, provide a full connection string (e.g., `postgres://user@host/different_database`).

**Input with alias**:
```json
{
  "connection_string": "production"
}
```

**Input with full connection string**:
```json
{
  "connection_string": "postgres://user:pass@host:5432/database"
}
```

**Behavior**:

- If the input looks like an alias (no `postgres://` or `postgresql://` prefix), it attempts to resolve it from saved connections
- If the alias is found, it uses the saved connection string
- **Smart hostname matching**: If a connection string is provided and the hostname matches a saved connection (by hostname or alias), it automatically uses the saved connection's credentials while allowing you to override the database name
- If not found, it treats the input as a literal connection string
- Successfully used aliases are marked with a "last used" timestamp

**Examples of smart hostname matching**:

```json
// You have a saved connection "kielbasa" with host "kielbasa.example.com"
// and credentials user:password, connected to database "tenaciousdd"

// Connect to a different database on the same server:
{
  "connection_string": "postgres://user@kielbasa/postgres"
}
// This will automatically use the saved password from "kielbasa" connection

// Or using the full hostname:
{
  "connection_string": "postgres://user@kielbasa.example.com/newdb"
}
// Also uses saved credentials, connects to "newdb" instead
```

**Output with alias**:
```
Successfully connected to database using alias 'production'
Loaded metadata for 142 tables/views.
```

**Output with connection string**:
```
Successfully connected to database.
Loaded metadata for 142 tables/views.
```

## Connection Management Workflow

Here's a typical workflow for managing database connections:

```
1. Save connections with friendly names:
   add_database_connection(
     alias="prod",
     connection_string="postgres://...",
     description="Production DB"
   )

2. List saved connections:
   list_database_connections()

3. Connect using an alias:
   set_database_connection(connection_string="prod")

4. Work with the database:
   query_database(query="Show me...")
   get_schema_info()

5. Update a connection if needed:
   edit_database_connection(
     alias="prod",
     new_description="Production DB - Updated"
   )

6. Remove old connections:
   remove_database_connection(alias="old_staging")
```

## Security Considerations

- **Authentication Enabled (per-token connections)**:
    - Each API token has its own isolated set of saved connections
    - Users cannot see or access connections from other tokens
    - Connections are stored in `pgedge-postgres-mcp-server-tokens.yaml` with the token

- **Authentication Disabled (global connections)**:
    - All connections are stored in the preferences file (`pgedge-postgres-mcp-prefs.yaml`)
    - All users share the same set of saved connections
    - Suitable for single-user or trusted environments

- **Password Encryption**:
    - All passwords are encrypted using AES-256-GCM encryption before storage
    - Encryption key is stored in a separate secret file (default: `pgedge-postgres-mcp.secret`)
    - Secret file is auto-generated on first run with restricted permissions (0600)
    - Both database passwords and SSL key passwords are encrypted
    - Passwords are never displayed in tool outputs or logs

- **Connection Storage Security**:
    - Use appropriate file permissions (0600 for tokens and preferences files)
    - Connection parameters are stored in YAML files with encrypted passwords
    - Never commit secret file or preferences files with real credentials to version control
    - Secret file should be backed up securely and separately from configuration files
    - Consider using SSL client certificates instead of passwords for authentication

