# Using Environment Variables to Specify Options

The server supports environment variables for all configuration options. All environment variables use the **`PGEDGE_`** prefix to avoid collisions with other software.

The following environment variables specify HTTP/HTTPS Server preferences:

- **`PGEDGE_HTTP_ENABLED`**: Enable HTTP transport mode ("true", "1", "yes" to enable)
- **`PGEDGE_HTTP_ADDRESS`**: HTTP server address (default: ":8080")

The following environment variables specify TLS/HTTPS preferences:

- **`PGEDGE_TLS_ENABLED`**: Enable TLS/HTTPS ("true", "1", "yes" to enable)
- **`PGEDGE_TLS_CERT_FILE`**: Path to TLS certificate file
- **`PGEDGE_TLS_KEY_FILE`**: Path to TLS key file
- **`PGEDGE_TLS_CHAIN_FILE`**: Path to TLS certificate chain file (optional)

The following environment variables specify authentication preferences:

- **`PGEDGE_AUTH_ENABLED`**: Enable API token authentication ("true", "1", "yes" to enable)
- **`PGEDGE_AUTH_TOKEN_FILE`**: Path to API token file

If you run into issues with your environment variable settings, check:

```bash
# Verify environment variables are set
env | grep PGEDGE

# Export the variables if you are running in a new shell
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_HTTP_ADDRESS=":8080"
```

## Examples

**Configuring an HTTP server with authentication:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_HTTP_ADDRESS=":8080"
export PGEDGE_AUTH_ENABLED="true"
export PGEDGE_AUTH_TOKEN_FILE="./pgedge-mcp-server-tokens.yaml"

./bin/pgedge-mcp-server
```

**Configuring a HTTPS server:**

```bash
export PGEDGE_HTTP_ENABLED="true"
export PGEDGE_TLS_ENABLED="true"
export PGEDGE_TLS_CERT_FILE="./server.crt"
export PGEDGE_TLS_KEY_FILE="./server.key"

./bin/pgedge-mcp-server
```

**Using Environment Variables for Tests:**

Tests use a separate environment variable to avoid confusion with runtime configuration:

```bash
export TEST_PGEDGE_POSTGRES_CONNECTION_STRING="postgres://localhost/postgres?sslmode=disable"
go test ./...
```