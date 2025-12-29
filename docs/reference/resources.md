# MCP Resources

Resources provide read-only access to Postgres system information. Resources
are accessed via the `read_resource` tool or through MCP protocol resource
methods.

## Disabling Resources

Individual resources can be disabled via configuration to restrict what the
LLM can access. See [Enabling/Disabling Built-in Features](../guide/feature_config.md)
for details.

When a resource is disabled:

* It is not advertised to the LLM in the `resources/list` response.
* Attempts to read it return an error message.

You can access resources with the [read_resource](tools.md#read_resource) tool or with Natural Language (and the Claude Desktop).

### Accessing Resources with the read_resource Tool

In the following example, the [read_resource](tools.md#read_resource) tool retrieves system information using the URI:

```json
{
  "uri": "pg://system_info"
}
```

In the following example, the `read_resource` tool lists all available resources:

```json
{
  "list": true
}
```

### Accessing Resources with Natural Language (Claude Desktop)

You can access a resource by simply asking Claude to read that resource; for example the following requests return system information:

* "Show me the output from pg://system_info"
* "What's the current PostgreSQL version?" (uses pg://system_info)
* "What version of PostgreSQL is running?" (uses pg://system_info)


## Using pg://system_info

`pg://system_info` returns the Postgres version, operating system details, and build architecture information. The resource provides a quick and efficient way to check server version and platform details without executing natural language queries.

**Use Cases:**

* Quickly check PostgreSQL version without natural language queries.
* Verify server platform and architecture.
* Audit server build information.
* Troubleshoot compatibility issues.

When you read the resource to view PostgreSQL system information, the result is a JSON object with detailed system information. For example, the following JSON output contains PostgreSQL system information:

```json
{
  "postgresql_version": "15.4",
  "version_number": "150004",
  "full_version": "PostgreSQL 15.4 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 11.2.0, 64-bit",
  "operating_system": "linux",
  "architecture": "x86_64-pc-linux-gnu",
  "compiler": "gcc (GCC) 11.2.0",
  "bit_version": "64-bit"
}
```

**Properties**

| Name | Description |
|------|-------------|
| `postgresql_version` | Short version string (e.g., `15.4`). |
| `version_number` | Numeric version identifier (e.g., `150004`). |
| `full_version` | Complete version string from PostgreSQL version() function. |
| `operating_system` | Operating system (e.g., `linux`, `darwin`, `mingw32`). |
| `architecture` | Full architecture string (e.g., `x86_64-pc-linux-gnu`, `aarch64-apple-darwin`). |
| `compiler` | Compiler used to build PostgreSQL (e.g., `gcc (GCC) 11.2.0`). |
| `bit_version` | Architecture bit version (e.g., `64-bit`, `32-bit`). |


## Finding Schema Information

To find database schema information (tables, columns, constraints, etc.), use the `get_schema_info` tool instead of resources. The `get_schema_info` tool provides:

* Detailed column information with data types.
* Primary key, foreign key, and unique constraints.
* Index information.
* Identity column detection.
* Default values.
* Vector column detection for similarity search.
* TSV output format for token efficiency.

See [Tools Reference](tools.md#get_schema_info) for details.
