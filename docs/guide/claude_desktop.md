# Using the MCP Server with Claude Desktop

Claude Desktop connects to the pgEdge Postgres MCP Server
using the `stdio` transport. The MCP server runs as a child
process that Claude Desktop launches automatically. This
guide walks through building the server, configuring Claude
Desktop, and verifying the connection.

## Getting Started

Before configuring Claude Desktop, you must build the MCP
server binary. This section covers the prerequisites and
build steps.

### Prerequisites

Ensure the following software is installed on your system:

- [Go 1.21](https://go.dev/doc/install) or higher is
  required to build the server.
- [PostgreSQL 14](https://www.postgresql.org/download/) or
  higher must be running and accessible.
- [Git](https://git-scm.com/downloads) is required to
  clone the repository.

### Building the MCP Server

Clone the repository and build the server binary with the
following commands:

```bash
git clone https://github.com/pgEdge/pgedge-postgres-mcp.git
cd pgedge-postgres-mcp
make build
```

The `make build` command compiles the server binary into
the `bin/` directory.

### Verifying the Binary

After building, confirm the binary exists by running the
following command:

```bash
ls -la bin/pgedge-postgres-mcp
```

The command should display the binary file with its size
and modification timestamp. Note the absolute path to the
binary; you will need the path for Claude Desktop
configuration.

## Configuring Claude Desktop

Claude Desktop stores MCP server connections in a JSON
configuration file. The file location depends on your
operating system:

- macOS:
  `~/Library/Application Support/Claude/claude_desktop_config.json`
- Linux:
  `~/.config/Claude/claude_desktop_config.json`
- Windows:
  `%APPDATA%\Claude\claude_desktop_config.json`

Open the configuration file and add the MCP server to the
`mcpServers` property. The following example uses
environment variables to configure the database connection:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/path/to/bin/pgedge-postgres-mcp",
      "env": {
        "PGHOST": "localhost",
        "PGPORT": "5432",
        "PGDATABASE": "mydb",
        "PGUSER": "myuser",
        "PGPASSWORD": "mypass"
      }
    }
  }
}
```

Replace the `command` value with the absolute path to your
`pgedge-postgres-mcp` binary. Replace the `env` values with
your actual database credentials.

After modifying the configuration file, restart Claude
Desktop to apply the changes.

## Connecting as a Remote Connector

Claude Desktop can also reach the MCP server as a remote
connector over HTTP, instead of launching it locally over
`stdio`. Use this route for a server deployed on another
machine, or where OAuth sign-in is preferred over managing a
local process.

Open Settings, choose Connectors, then select Add custom
connector. Enter the server's URL, for example
`https://mcp.example.com`. Claude Desktop opens the server's
login page in a browser; sign in there to complete the
connection. See [Authentication -
OAuth](auth_oauth.md#claude-desktop-and-claude-mobile) for
what happens during sign-in.

The Claude mobile apps share the same connector list as
Claude Desktop, so a connector added on the desktop appears
on mobile without repeating these steps.

## Configuration File Structure

The Claude Desktop configuration file uses three
properties within each MCP server entry. The following
table describes each property.

| Property  | Required | Description                      |
|-----------|----------|----------------------------------|
| `command` | Yes      | Absolute path to the MCP binary. |
| `args`    | No       | Array of command-line arguments.  |
| `env`     | No       | Environment variables as a map.   |

The `command` property must contain an absolute path to the
MCP server binary. Relative paths will cause Claude Desktop
to fail when launching the server.

The `args` property accepts an array of strings. Claude
Desktop passes these arguments to the binary at launch.

The `env` property sets environment variables for the
server process. The server reads standard PostgreSQL
environment variables such as `PGHOST`, `PGPORT`,
`PGDATABASE`, `PGUSER`, and `PGPASSWORD`.

## Using a YAML Configuration File

You can store database and feature settings in a YAML
configuration file instead of using environment variables.
The following example shows a `stdio`-mode configuration
for use with Claude Desktop:

```yaml
# Database connection settings
databases:
    - name: "mydb"
      host: "localhost"
      port: 5432
      database: "mydb"
      user: "myuser"
      password: "mypass"

# Embedding provider settings (optional)
embedding:
    enabled: true
    provider: "ollama"
    model: "nomic-embed-text"

# Knowledgebase settings (optional)
knowledgebase:
    enabled: false
```

Save the YAML file to a location on your system. Then
reference the file in your Claude Desktop configuration
using the `args` property with the `-config` flag.

In the following example, the `args` property passes the
`-config` flag to load a YAML configuration file:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/path/to/bin/pgedge-postgres-mcp",
      "args": [
        "-config",
        "/path/to/pgedge-postgres-mcp.yaml"
      ]
    }
  }
}
```

Replace both paths with absolute paths on your system.

## How Natural Language Queries Work

Claude Desktop uses the MCP server to translate your
questions into database queries. The following steps
describe the query flow:

1. You type a natural language question in Claude Desktop.
2. Claude generates a SQL query using the
   `query_database` tool.
3. The MCP server executes the SQL in a read-only
   transaction.
4. The database returns the result set to the MCP server.
5. The MCP server sends the results back to Claude.
6. Claude interprets the data and presents a natural
   language answer.

All queries run in read-only transactions by default. The
server prevents data modifications unless the database
configuration explicitly enables write access with the
`allow_writes` option.

## Command-Line Flags

The MCP server accepts command-line flags that you can
pass through the `args` property in the Claude Desktop
configuration. The following table lists the flags
relevant to `stdio` mode.

| Flag           | Description                         |
|----------------|-------------------------------------|
| `-config`      | Path to a YAML configuration file.  |
| `-debug`       | Enable debug logging output.        |
| `-trace-file`  | Path to write a trace log file.     |
| `-db-host`     | Database hostname.                  |
| `-db-port`     | Database port number.               |
| `-db-name`     | Database name.                      |
| `-db-user`     | Database username.                  |
| `-db-password` | Database password.                  |
| `-db-sslmode`  | SSL connection mode.                |

In the following example, the `args` property passes
database connection flags directly to the MCP server:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/path/to/pgedge-postgres-mcp",
      "args": [
        "-db-host", "localhost",
        "-db-port", "5432",
        "-db-name", "mydb",
        "-db-user", "myuser",
        "-db-password", "mypass"
      ]
    }
  }
}
```

You can combine flags; for example, add `-debug` to the
`args` array to enable debug logging alongside database
connection flags.

## Verifying Your Setup

After configuring Claude Desktop, verify the setup with
the following checklist:

- Confirm the binary path in `command` is absolute and
  correct.
- Verify database credentials by connecting with `psql`
  independently.
- Restart Claude Desktop after saving configuration
  changes.
- Check that the MCP server appears in the Claude Desktop
  tool list.
- Ask Claude a test question such as "What tables are in
  my database?"

If the test question returns a list of tables, the MCP
server is connected and working correctly.

## Troubleshooting

This section addresses common issues when connecting
Claude Desktop to the MCP server.

### MCP Server Disconnected

Claude Desktop displays "MCP server disconnected" when
the server process fails to start or crashes.

Verify the binary path in `command` is correct and
absolute. Run the binary manually from a terminal to check
for startup errors.

In the following example, a JSON-RPC `initialize` message
tests the MCP server binary directly:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",'\
'"params":{"protocolVersion":"2024-11-05",'\
'"capabilities":{},"clientInfo":'\
'{"name":"test","version":"1.0"}}}' \
  | /path/to/bin/pgedge-postgres-mcp
```

The server should respond with a JSON-RPC message
containing server capabilities. An error message or no
output indicates a configuration problem.

### Permission Denied

The operating system returns "permission denied" when the
binary lacks execute permissions.

Set the execute permission on the binary with the
following command:

```bash
chmod +x /path/to/bin/pgedge-postgres-mcp
```

### Connection Refused

The server logs "connection refused" when the PostgreSQL
database is not running or not accepting connections.

Verify the database is running with the following command:

```bash
pg_isready -h localhost -p 5432
```

If `pg_isready` reports the server is not accepting
connections, start the PostgreSQL service:

```bash
sudo systemctl start postgresql
```

### Checking Claude Desktop Logs

Claude Desktop writes log files that can reveal MCP server
errors. The log file locations depend on your operating
system:

- macOS:
  `~/Library/Logs/Claude/mcp*.log`
- Linux:
  `~/.config/Claude/logs/mcp*.log`
- Windows:
  `%APPDATA%\Claude\logs\mcp*.log`

Review the log files for error messages related to the
MCP server. Common errors include invalid JSON syntax,
missing binary files, and database connection failures.

### JSON Syntax Errors

Claude Desktop fails to load the configuration when the
JSON file contains syntax errors. Validate the JSON syntax
using the following command:

```bash
python3 -m json.tool < claude_desktop_config.json
```

The command prints formatted JSON on success or displays
an error message with the line number of the syntax issue.

For more troubleshooting help, see the
[Troubleshooting Guide](troubleshooting.md).

!!! hint

    If you use Claude or Claude Code, the LLM will
    only use the first database configured in your
    configuration file.
