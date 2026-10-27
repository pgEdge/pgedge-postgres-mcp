# Using the MCP Server with Cursor

Cursor connects to the pgEdge Postgres MCP Server
using the `stdio` transport. The MCP server runs as a
child process that Cursor launches automatically.

The pgEdge Postgres MCP Server is a locally built Go
binary. Whether you configure Cursor through the
marketplace or manually, you must build the binary
first. The marketplace plugin configures Cursor to
use the binary; the marketplace does not distribute
the binary itself.

## Building the MCP Server

Build the MCP server binary before configuring
Cursor. This section covers the prerequisites and
build steps.

### Prerequisites

Ensure the following software is installed on your
system:

- [Go 1.26.10](https://go.dev/doc/install) or higher is
  required to build the server.
- [PostgreSQL 14](https://www.postgresql.org/download/)
  or higher must be running and accessible.
- [Git](https://git-scm.com/downloads) is required to
  clone the repository.

### Clone and Build

Clone the repository and build the server binary with
the following commands:

```bash
git clone https://github.com/pgEdge/pgedge-postgres-mcp.git
cd pgedge-postgres-mcp
make build
```

The `make build` command compiles the server binary
into the `bin/` directory.

### Verify the Binary

After building, confirm the binary exists by running
the following command:

```bash
ls -la bin/pgedge-postgres-mcp
```

The command should display the binary file with its
size and modification timestamp. Note the absolute
path to the binary; you will need the path when
configuring Cursor.

## Configuring Cursor

After building the binary, configure Cursor to use
the MCP server. Choose one of the following methods.

### Option A: Install from the Cursor Marketplace

The marketplace plugin adds the MCP server
configuration to Cursor automatically. You must
still provide database connection details after
installation.

1. Open Cursor and navigate to Cursor > Settings >
   Cursor Settings > Tools & MCP.
2. Click Marketplace in the left sidebar.
3. Search for "pgEdge Postgres MCP Server".
4. Click Install.
5. Return to Tools & MCP and update the environment
   variables with your database connection details.

The marketplace plugin sets the `command` to
`pgedge-postgres-mcp`. Ensure the binary is on your
`PATH`, or edit the command to use the absolute path
after installation.

### Option B: Manual Configuration

Configure the MCP server manually by editing a JSON
file. Cursor supports two configuration file
locations:

- Project-specific: `.cursor/mcp.json` in the
  project directory.
- Global: `~/.cursor/mcp.json` in your home
  directory.

Open the configuration file and add the MCP server
to the `mcpServers` property. The following example
uses environment variables to configure the database
connection:

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

Replace the `command` value with the absolute path to
your `pgedge-postgres-mcp` binary. Replace the `env`
values with your actual database credentials.

## Configuration File Structure

The Cursor MCP configuration file uses three
properties within each MCP server entry. The
following table describes each property.

| Property  | Required | Description                      |
|-----------|----------|----------------------------------|
| `command` | Yes      | Absolute path to the MCP binary. |
| `args`    | No       | Array of command-line arguments.  |
| `env`     | No       | Environment variables as a map.   |

The `command` property must contain an absolute path
to the MCP server binary. Relative paths will cause
Cursor to fail when launching the server.

The `args` property accepts an array of strings.
Cursor passes these arguments to the binary at
launch.

The `env` property sets environment variables for
the server process. The server reads standard
PostgreSQL environment variables such as `PGHOST`,
`PGPORT`, `PGDATABASE`, `PGUSER`, and `PGPASSWORD`.

## Using a YAML Configuration File

You can store database and feature settings in a
YAML configuration file instead of using environment
variables. The following example shows a `stdio`-mode
configuration for use with Cursor:

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

Save the YAML file to a location on your system.
Then reference the file in your Cursor configuration
using the `args` property with the `-config` flag.

In the following example, the `args` property passes
the `-config` flag to load a YAML configuration
file:

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

Replace both paths with absolute paths on your
system.

## Command-Line Flags

The MCP server accepts command-line flags that you
can pass through the `args` property in the Cursor
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
database connection flags directly to the MCP
server:

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

You can combine flags; for example, add `-debug` to
the `args` array to enable debug logging alongside
database connection flags.

## Verifying Your Setup

After configuring Cursor, verify the MCP server
connection with the following steps:

1. Navigate to Cursor > Settings > Cursor Settings
   > Tools & MCP.
2. Confirm the pgEdge server appears in the list
   and shows a connected status.
3. Open a new chat and ask "What tables are in my
   database?"

If the test question returns a list of tables, the
MCP server is connected and working correctly.

## Troubleshooting

This section addresses common issues when connecting
Cursor to the MCP server.

### MCP Server Not Connecting

Cursor fails to connect when the server process does
not start or crashes during initialization.

Verify the binary path in `command` is correct and
absolute. Run the binary manually from a terminal to
check for startup errors.

In the following example, a JSON-RPC `initialize`
message tests the MCP server binary directly:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",'\
'"params":{"protocolVersion":"2024-11-05",'\
'"capabilities":{},"clientInfo":'\
'{"name":"test","version":"1.0"}}}' \
  | /path/to/bin/pgedge-postgres-mcp
```

The server should respond with a JSON-RPC message
containing server capabilities. An error message or
no output indicates a configuration problem.

### Permission Denied

The operating system returns "permission denied"
when the binary lacks execute permissions.

Set the execute permission on the binary with the
following command:

```bash
chmod +x /path/to/bin/pgedge-postgres-mcp
```

### Connection Refused

The server logs "connection refused" when the
PostgreSQL database is not running or not accepting
connections.

Verify the database is running with the following
command:

```bash
pg_isready -h localhost -p 5432
```

If `pg_isready` reports the server is not accepting
connections, start the PostgreSQL service:

```bash
sudo systemctl start postgresql
```

### JSON Syntax Errors

Cursor fails to load the configuration when the
JSON file contains syntax errors. Validate the JSON
syntax using the following command:

```bash
python3 -m json.tool < .cursor/mcp.json
```

The command prints formatted JSON on success or
displays an error message with the line number of
the syntax issue.

For more troubleshooting help, see the
[Troubleshooting Guide](troubleshooting.md).
