# Testing the MCP Server Deployment

This page explains how to test the MCP server with a script or manually to verify that the server is working correctly. All deployment methods expose a health endpoint at:

```bash
curl http://localhost:8080/health
```

When you navigate to that address, you should see the following response:

```json
{"status": "ok", "server": "pgedge-postgres-mcp", "version": "1.0.0-beta1"}
```

**Verifying Server Functionality**

You can manually test the server with JSON-RPC requests. In the following example, the commands set the API key environment variable, send an initialize request to the server, and then send a tools list request.

```bash
# Set environment
export ANTHROPIC_API_KEY="sk-ant-..."

# Test initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/pgedge-postgres-mcp

# Test tools list (in another terminal, or after initialize response)
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./bin/pgedge-postgres-mcp
```
