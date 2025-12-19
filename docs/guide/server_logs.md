# Reviewing MCP Server Log Files

You can use the server logs and Claude Desktop logs to diagnose issues.

**Claude Desktop Logs**

You can view the Claude Desktop logs to diagnose connection and server issues. The log file location depends on your operating system.

On macOS, you can use the following command to view the logs in real time:

```bash
tail -f ~/Library/Logs/Claude/mcp*.log
```

On Windows, the logs are located in the following directory:

```
%APPDATA%\Claude\logs\
```

On Linux, the logs are located in the following directory:

```bash
~/.config/Claude/logs/
```

**MCP Server Logs**

All MCP server output is sent to stderr and appears in the Claude Desktop logs with a `[pgedge]` prefix. You should monitor the files for the following message types:

- The `[pgedge-postgres-mcp] Starting server...` message indicates that the server is starting up.
- The `[pgedge-postgres-mcp] Database connected successfully` message indicates that the database connection succeeded.
- The `[pgedge-postgres-mcp] Loaded metadata for X tables/views` message indicates that metadata was loaded successfully.
- The `[pgedge-postgres-mcp] Starting stdio server loop...` message indicates that the server is ready to accept requests.
- The `[pgedge-postgres-mcp] ERROR:` prefix indicates an error message.