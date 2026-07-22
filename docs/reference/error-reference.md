# Error Reference

The MCP server returns errors at two levels: HTTP status codes
and JSON-RPC error responses. Understanding both levels helps
you diagnose issues quickly.

HTTP status codes indicate transport-level problems such as
authentication failures or malformed requests. JSON-RPC error
codes indicate protocol-level problems such as invalid methods
or bad parameters. Some tool errors arrive inside a successful
HTTP 200 response with the `isError` flag set to `true`.

## HTTP Status Codes

The server uses standard HTTP status codes to communicate the
outcome of each request.

The following table lists the HTTP status codes the server
returns.

| Status | Meaning              | Common Cause               |
|--------|----------------------|----------------------------|
| 200    | OK                   | The request succeeded.     |
| 201    | Created              | The server created a resource. |
| 400    | Bad Request          | The JSON body is invalid.  |
| 401    | Unauthorized         | The token is missing or invalid. |
| 403    | Forbidden            | The user lacks permission. |
| 404    | Not Found            | The resource does not exist. |
| 405    | Method Not Allowed   | The HTTP method is wrong.  |
| 413    | Payload Too Large    | The request body exceeds the 10MB limit. |
| 429    | Too Many Requests    | The rate limit is exceeded. |
| 500    | Internal Server Error| A server-side error occurred. |

### Error Response Format

HTTP error responses use a JSON body with an `error` field.

In the following example, the server returns a 401 error for
a missing token:

```json
{
    "error": "Missing Authorization header"
}
```

In the following example, the server returns a 400 error for
an invalid request body:

```json
{
    "error": "Invalid request body"
}
```

In the following example, the server returns a 404 error for
a missing resource:

```json
{
    "error": "Database not found"
}
```

## Authentication Errors

The authentication system validates API tokens and session
tokens on every request. The server returns HTTP 401 for all
token-related failures.

### Missing Token

The server returns this error when the `Authorization` header
is absent from the request.

In the following example, the response indicates a missing
header:

```json
{
    "error": "Missing Authorization header"
}
```

Add a `Bearer` token to the `Authorization` header to resolve
this error.

### Invalid Token Format

The server returns this error when the `Authorization` header
does not follow the `Bearer <token>` format.

In the following example, the response indicates a malformed
header:

```json
{
    "error": "Invalid Authorization header format. Expected: Bearer <token>"
}
```

Ensure the header value begins with `Bearer` followed by a
space and the token string.

### Invalid or Expired Token

The server returns this error when the token does not match
any stored hash or has passed its expiry date.

In the following example, the response indicates an unknown
token:

```json
{
    "error": "Invalid or unknown token"
}
```

Generate a new token with the `-add-token` flag or
re-authenticate to obtain a fresh session token.

For security, the server returns the same 401 status for
missing, invalid, and expired tokens. Clients cannot
distinguish between these failure modes.

### Rate Limited

The server tracks failed authentication attempts per IP
address. The default limit allows 10 failed attempts per
15-minute window.

When the rate limit is exceeded, the server rejects all
further attempts from that IP address. Wait for the rate
limit window to expire before retrying.

### Account Locked

The server disables a user account after too many consecutive
failed login attempts. An administrator must re-enable the
account using the `-enable-user` CLI command.

## JSON-RPC Errors

The MCP server uses JSON-RPC 2.0 for protocol communication.
JSON-RPC errors arrive as HTTP 200 responses with an `error`
object instead of a `result` object.

The following table lists the standard JSON-RPC error codes.

| Code   | Message          | Description                   |
|--------|------------------|-------------------------------|
| -32700 | Parse error      | The request body has invalid JSON. |
| -32600 | Invalid Request  | A required JSON-RPC field is missing. |
| -32601 | Method not found | The method name is unknown.   |
| -32602 | Invalid params   | The parameters are invalid.   |
| -32603 | Internal error   | A server-side processing error occurred. |

### Error Response Format

JSON-RPC error responses include a numeric code and a message.

In the following example, the server returns a method-not-found
error:

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "error": {
        "code": -32601,
        "message": "Method not found"
    }
}
```

In the following example, the server returns a parse error for
malformed JSON:

```json
{
    "jsonrpc": "2.0",
    "id": null,
    "error": {
        "code": -32700,
        "message": "Parse error",
        "data": "unexpected end of JSON input"
    }
}
```

### Invalid Parameters

The server returns code `-32602` when tool parameters fail
validation. The `data` field provides additional detail about
the specific parameter that failed.

In the following example, the server rejects an invalid
parameter:

```json
{
    "jsonrpc": "2.0",
    "id": 3,
    "error": {
        "code": -32602,
        "message": "Invalid params",
        "data": "database name is required"
    }
}
```

### Internal Errors

The server returns code `-32603` when an unexpected condition
occurs during request processing. Check the server logs for
additional diagnostic information.

In the following example, the server reports an internal error
from a tool execution:

```json
{
    "jsonrpc": "2.0",
    "id": 4,
    "error": {
        "code": -32603,
        "message": "Internal error",
        "data": "connection refused"
    }
}
```

## Tool-Specific Errors

Tool errors arrive inside a successful HTTP 200 response. The
response body contains a `result` object with the `isError`
field set to `true` and an error message in the `content`
array.

In the following example, a tool returns an error within a
successful JSON-RPC response:

```json
{
    "jsonrpc": "2.0",
    "id": 5,
    "result": {
        "content": [
            {
                "type": "text",
                "text": "query_text cannot be empty"
            }
        ],
        "isError": true
    }
}
```

### query_database Errors

The `query_database` tool executes SQL queries against the
configured PostgreSQL database. The following errors may occur.

- The server rejects write operations on a read-only database
  with the message "cannot execute INSERT in a read-only
  transaction."
- The query fails when the configured timeout expires before
  the query completes.
- The server returns a connection error when the database host
  is unreachable.
- The database rejects queries when the user lacks the
  required permissions.
- The database rejects queries that contain invalid SQL syntax.

To resolve read-only violations, configure the database with
`allow_writes: true` in the server configuration file.

### execute_explain Errors

The `execute_explain` tool only accepts `SELECT` statements.
The tool rejects other statement types to prevent unintended
side effects from `EXPLAIN ANALYZE`.

- The tool requires the `query` parameter to be a non-empty
  string.
- The tool rejects non-SELECT statements such as `INSERT`,
  `UPDATE`, or `DELETE`.

### get_schema_info Errors

The `get_schema_info` tool retrieves database schema details.

- The tool returns an error when the specified schema does not
  exist in the database.
- The tool returns an error when the specified table does not
  exist in the schema.

### similarity_search Errors

The `similarity_search` tool performs vector similarity queries
using pgvector columns.

- The tool requires a non-empty `query_text` parameter.
- The tool returns an error when no pgvector columns exist in
  the target table.
- The tool returns an error when the embedding provider is not
  configured.
- The tool returns an error when the embedding API call fails.

### search_knowledgebase Errors

The `search_knowledgebase` tool searches the project knowledge
base for relevant documentation.

- The tool returns an error when the knowledgebase feature is
  not configured.
- The tool returns an error when the knowledgebase database
  file is missing.
- The tool returns an error when the specified project name
  does not exist in the knowledgebase.
- The tool requires the `query` parameter when not listing
  products.

### authenticate_user Errors

The `authenticate_user` tool validates user credentials and
returns a session token.

- The tool returns "invalid username or password" when the
  credentials do not match a stored user.
- The tool returns "user account is disabled" when the account
  has been locked by an administrator.

## Database Access Errors

The database management API provides endpoints for listing and
selecting databases. These endpoints return JSON responses with
a `success` field.

### GET /api/databases

The list endpoint returns all databases accessible to the
current token. The following error may occur.

- The server returns HTTP 401 when the request lacks a valid
  authentication token.

### POST /api/databases/select

The select endpoint switches the active database for the
current session. The following errors may occur.

- The server returns HTTP 400 when the request body is invalid
  or the database name is empty.
- The server returns HTTP 403 when the user lacks access to
  the requested database.
- The server returns HTTP 403 when an API token is bound to a
  different database.
- The server returns HTTP 404 when the specified database name
  does not match any configured database.

In the following example, the server returns a 403 error for
a database access violation:

```json
{
    "success": false,
    "error": "Access denied to this database"
}
```

In the following example, the server returns a 400 error for
a missing database name:

```json
{
    "success": false,
    "error": "Database name is required"
}
```

In the following example, the server returns a 403 error for
a token bound to another database:

```json
{
    "success": false,
    "error": "API token is bound to a different database"
}
```

## Resource Errors

The MCP server returns resource errors when a resource read
operation fails. Resource errors may use plain text or JSON
format depending on the resource type.

In the following example, the server returns a JSON-formatted
resource error:

```json
{
    "error": true,
    "message": "Database is still initializing.",
    "code": "DATABASE_NOT_READY",
    "retryable": true
}
```

The `retryable` field indicates whether the client should
attempt the request again after a short delay.

## Troubleshooting Guide

The following sections provide diagnostic steps for common
error patterns.

### Diagnosing Unauthorized Errors

Follow these steps to diagnose HTTP 401 errors.

1. Verify the `Authorization` header is present in the
   request.
2. Confirm the header uses the `Bearer <token>` format.
3. Check whether the token has passed its expiry date.
4. Verify the token exists in the token configuration file.
5. Check whether the account is locked by running the
   `-list-users` command.
6. Confirm the rate limiter has not blocked the client IP
   address.

### Diagnosing Connection Errors

Follow these steps to diagnose database connection failures.

1. Verify the database host is reachable from the server by
   using `pg_isready`.
2. Confirm the port number matches the PostgreSQL
   configuration.
3. Verify the database credentials in the server configuration
   file.
4. Check the SSL mode configuration if TLS is required.
5. Review the server logs for detailed connection error
   messages.

### Diagnosing Query Errors

Follow these steps to diagnose SQL query failures.

1. Test the SQL query directly against PostgreSQL using
   `psql`.
2. Verify the database user has `SELECT` permissions on the
   target tables.
3. Confirm the table exists in the expected schema.
4. Check for read-only transaction violations if the query
   contains write operations.
5. Verify the query completes within the configured timeout
   period.

### Diagnosing Tool Errors

Follow these steps to diagnose tool execution failures.

1. Check the `isError` field in the tool response for error
   details.
2. Verify the tool parameters match the expected schema.
3. Confirm the tool is enabled in the server configuration.
4. Review the server logs for stack traces and error context.

## See Also

The following resources provide additional information about
related topics.

- The [Authentication Guide](../guide/authentication.md)
  explains token and user account management.
- The [Troubleshooting Guide](../guide/troubleshooting.md)
  covers build and deployment issues.
- The [API Reference](../developers/api-reference.md)
  documents all available endpoints.
- The [Security Guide](../guide/security.md) describes
  security best practices for deployment.
