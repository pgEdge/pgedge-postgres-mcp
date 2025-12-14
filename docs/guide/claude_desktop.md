# Using the MCP Server with Claude Desktop

After installing and configuring the MCP server, you can connect with the Claude Desktop.  To add connection details to your Claude Desktop configuration file, edit the file (located by default in):

**macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
**Linux:** `~/.config/Claude/claude_desktop_config.json`
**Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

Add your connection details for the Postgres server to the `mcpServers` property:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/path/to/bin/pgedge-mcp-server",
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

To specify your connection details in a .yaml file, use the `args` property to include the `--config` option and a path to the configuration file in the `mcpServers` property:

```json
{
  "mcpServers": {
    "pgedge": {
      "command": "/Users/user_name/git/pgedge-nla/bin/pgedge-postgres-mcp",
      "args": [
        "-config",
        "/Users/user_name/git/pgedge-nla/bin/pgedge-postgres-mcp-stdio.yaml"
      ]
    }
  }
}
```

**Important Notes:**

- Replace the path specified in the `command` property with the full path to your project directory.
- Database connections are configured at server startup via environment variables, config file, or command-line flags.
- Claude Desktop's LLM handles natural language to SQL translation, then this server executes the SQL queries.

After modifying the configuration file, restart Claude Desktop and start asking questions about your database.

!!! hint

    If you use Claude/Claude Code, Claude will only use the first database configured in your configuration file.

## Troubleshooting Claude Desktop Configuration Issues

If you're having trouble connecting with Claude Desktop, you should:

1. Check the JSON syntax in `claude_desktop_config.json`.
2. Ensure that properties point to absolute paths (not relative).
3. Restart Claude Desktop after making configuration changes.
4. Check the Claude Desktop logs for errors.

For more troubleshooting help, see the [Troubleshooting Guide](troubleshooting.md).

