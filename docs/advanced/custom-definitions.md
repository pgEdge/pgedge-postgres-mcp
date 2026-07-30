# Creating Custom Definitions

The MCP server supports user-defined custom prompts, resources,
and tools. Custom definitions extend the server without requiring
code changes.

For information about the server's built-in capabilities:

- [Built-in Resources](../reference/resources.md) provides a list
  of available built-in resources.
- [Built-in Prompts](../reference/prompts.md) provides a list of
  available built-in prompts.
- [Built-in Tools](../reference/tools.md) provides a list of
  available built-in tools.

Custom definitions enable you to:

- Define reusable prompt templates that guide the LLM through
  specific workflows.
- Expose frequently-used database queries as MCP resources.
- Provide configuration data, documentation, or other static
  information through resources.
- Create callable MCP tools that execute SQL queries or
  procedural language code.

When defining a prompt, resource, or tool:

- Use descriptive names that clearly communicate the purpose.
- Provide descriptions for prompts, arguments, and resources.
- Verify SQL queries before deploying the definitions file.
- Add `LIMIT` clauses to prevent returning excessive data.
- Store definitions files in version control.
- Start with a few definitions and expand gradually.
- Use the `custom://` prefix and kebab-case for resource URIs.

!!! note

    Current limitations (that may be addressed in future
    versions):

    - SQL resources cannot accept runtime parameters.
    - No hot-reloading (requires server restart).
    - No conditional logic in prompts.
    - No resource templates with arguments.
    - Limited to JSON output for resources.

See `examples/pgedge-postgres-mcp-custom.yaml` for a
comprehensive example that demonstrates all custom definition
types. The following commands show how to view and use the
example definitions file.

```bash
# View the example file
cat examples/pgedge-postgres-mcp-custom.yaml

# Use it in your configuration
custom_definitions_path: "./examples/pgedge-postgres-mcp-custom.yaml"
```


## Configuring Custom Definitions

To enable custom definitions, specify the path to a definitions
file in the server configuration. You can configure the path
using either YAML configuration or environment variables.

**YAML Configuration**

In the following example, the server configuration uses the
`custom_definitions_path` parameter to specify the location
of the custom definitions file.

```yaml
# In postgres-mcp.yaml
custom_definitions_path: "/path/to/pgedge-postgres-mcp-custom.yaml"
```

**Environment Variable**

In the following example, the
`PGEDGE_CUSTOM_DEFINITIONS_PATH` environment variable specifies
the location of the custom definitions file.

```bash
export PGEDGE_CUSTOM_DEFINITIONS_PATH="/path/to/pgedge-postgres-mcp-custom.yaml"
```

### Supported Format

- The server accepts YAML files with `.yaml` or `.yml`
  extensions.

## Writing a Definitions File

A definitions file contains three optional sections:
[`prompts`](#defining-prompts),
[`resources`](#defining-resources), and
[`tools`](#defining-tools). In the following example, the
definitions file includes all three sections.

```yaml
prompts:
  - # Prompt definitions
resources:
  - # Resource definitions
tools:
  - # Tool definitions
```

All sections are optional. You can define any combination of
prompts, resources, and tools.

## Defining Prompts

Prompts are reusable templates that guide the LLM through
specific workflows.

### Prompt Structure

In the following example, the prompt definition includes
required and optional fields to define a reusable prompt
template.

```yaml
prompts:
  - name: prompt-name
    description: Description text
    arguments:
      - name: arg_name
        description: Arg description
        required: true
    messages:
      - role: user
        content:
          type: text
          text: "Template {{arg_name}}"
```

### Template Interpolation

Use `{{argument_name}}` syntax in message text to interpolate
argument values.

In the following example, the template uses `{{table_name}}`
placeholders to interpolate the table name argument.

```yaml
prompts:
  - name: analyze-table
    arguments:
      - name: table_name
        required: true
    messages:
      - role: user
        content:
          type: text
          text: |
            Analyze the {{table_name}} table:
            1. Get schema: get_schema_info(table_name="{{table_name}}")
            2. Sample data: SELECT * FROM {{table_name}} LIMIT 5
```

When called with `{"table_name": "users"}`, the placeholders
are replaced with "users".

### Message Roles

The MCP protocol supports the following message roles:

- The `user` role provides instructions or questions from the
  user.
- The `assistant` role provides example responses or context.
- The `system` role provides system-level instructions or
  context.

### Content Types

The MCP protocol supports the following content types:

- The `text` type provides plain text with optional template
  placeholders.
- The `image` type provides base64-encoded image data and
  requires `data` and `mimeType` fields.
- The `resource` type references another resource and requires
  a `uri` field.

**Example: Simple Prompt**

In the following example, the prompt definition creates a
security audit prompt without any arguments.

```yaml
prompts:
  - name: security-audit
    description: Performs a security audit of the database
    messages:
      - role: user
        content:
          type: text
          text: |
            Perform a security audit:
            1. Check user privileges
            2. Review table access controls
            3. Identify potential vulnerabilities
```

**Example: Prompt with Arguments**

In the following example, the prompt definition uses two
required arguments to compare database schemas.

```yaml
prompts:
  - name: compare-schemas
    description: Compares two database schemas
    arguments:
      - name: schema1
        description: First schema name
        required: true
      - name: schema2
        description: Second schema name
        required: true
    messages:
      - role: user
        content:
          type: text
          text: |
            Compare schemas "{{schema1}}" and "{{schema2}}":
            1. Get info for {{schema1}}
            2. Get info for {{schema2}}
            3. List differences
```

## Defining Resources

Resources expose data or query results to the MCP client.

### Resource Types

### SQL Resources

SQL resources execute a query and return results in TSV format
for token efficiency.

In the following example, the resource definition specifies
the required fields for a SQL resource.

```yaml
resources:
  - uri: custom://resource-name
    name: Display Name
    description: What it returns
    type: sql
    sql: SELECT * FROM users
```

This example:

- Executes the query using the appropriate database connection.
- Respects per-token connection isolation in authenticated mode.
- Returns results in TSV format (first row is column headers).
- Escapes tabs, newlines, and carriage returns in values.
- Token-efficient output for LLM consumption.

**Example:**

In the following example, the SQL resource queries PostgreSQL
to list all active database users.

```yaml
resources:
  - uri: custom://active-users
    name: Active Users
    description: List of all active database users
    type: sql
    sql: |
      SELECT
        usename as username,
        usesuper as is_superuser,
        valuntil as valid_until
      FROM pg_user
      WHERE valuntil IS NULL OR valuntil > NOW()
      ORDER BY usename
```

### Static Resources

Static resources return predefined static data.

In the following example, the resource definition specifies
the required fields for a static resource.

```yaml
resources:
  - uri: custom://resource-name
    name: Display Name
    description: What it contains
    mimeType: application/json
    type: static
    data: value
```

**Data Formats:**

- A single value is a scalar such as a string, number, or
  boolean.
- A single row is an array of values.
- Multiple rows use a 2D array (array of arrays).
- An object contains key-value pairs.

**Example: Single Value**

In the following example, the static resource returns a single
scalar value representing the environment name.

```yaml
resources:
  - uri: custom://environment
    name: Environment
    description: Current environment name
    type: static
    data: "production"
```

**Example: Single Row**

In the following example, the static resource returns an array
of values representing support contact information.

```yaml
resources:
  - uri: custom://support-contact
    name: Support Contact
    type: static
    data:
      - "Support Team"
      - "support@example.com"
      - "+1-555-0123"
```

**Example: Multiple Rows**

In the following example, the static resource returns a 2D
array representing a maintenance schedule.

```yaml
resources:
  - uri: custom://maintenance-schedule
    name: Maintenance Schedule
    type: static
    data:
      - ["2025-02-01", "02:00", "04:00", "Security patches"]
      - ["2025-02-15", "03:00", "05:00", "Version upgrade"]
```

**Example: Object**

In the following example, the `static` resource returns a
configuration object with key-value pairs.

```yaml
resources:
  - uri: custom://db-config
    name: Database Configuration
    type: static
    data:
      max_connections: 100
      shared_buffers: "256MB"
      maintenance_work_mem: "64MB"
```

## Defining Tools

Custom tools expose callable database operations through the
MCP protocol. Three tool types are available: `sql`, `pl-do`,
and `pl-func`.

### Tool Structure

In the following example, the tool definition includes all
available fields for a custom tool.

```yaml
tools:
  - name: tool_name
    description: What the tool does
    type: sql | pl-do | pl-func
    language: plpgsql
    returns: text
    timeout: "30s"
    input_schema:
      type: object
      properties:
        param_name:
          type: string
          description: What the parameter is
      required: []
    sql: "SELECT ..."
    code: |
      ... code here ...
```

The following list describes each field in the tool definition.

- The `name` field is required and must be unique across all
  tools.
- The `description` field is optional and tells the LLM what
  the tool does.
- The `type` field is required; valid values are `sql`,
  `pl-do`, or `pl-func`.
- The `language` field is required for `pl-do` and `pl-func`
  types.
- The `returns` field is required for `pl-func` only and
  specifies the SQL return type.
- The `timeout` field is optional and sets the execution
  timeout.
- The `input_schema` field is required and defines the tool
  parameters.
- The `sql` field is required for `sql` type tools and
  contains the query to execute.
- The `code` field is required for `pl-do` and `pl-func` types
  and contains the procedural code.

### Tool Types

#### SQL Query Tools

SQL tools execute parameterized SQL queries against the
database. Parameters bind to positional placeholders (`$1`,
`$2`) matching `input_schema` property order. The server
returns results in TSV format. SQL tools execute in a
read-only transaction unless writes are explicitly allowed.

In the following example, the `sql` tool queries database
statistics using a parameterized query.

```yaml
tools:
  - name: get_database_stats
    description: Get statistics for a specific database
    type: sql
    input_schema:
      type: object
      properties:
        database_name:
          type: string
          description: The database name
      required: []
    sql: |
      SELECT
        datname AS database_name,
        numbackends AS active_connections,
        xact_commit AS committed_transactions,
        blks_hit AS cache_hits,
        pg_size_pretty(pg_database_size(datname))
          AS database_size
      FROM pg_stat_database
      WHERE datname = COALESCE($1, current_database())
```

#### PL/* DO Block Tools

The `pl-do` type executes anonymous code blocks using the
PostgreSQL `DO` statement. These tools work with read-only
database connections because they do not require `CREATE`
permission. Arguments are available through a pre-defined
`args` variable, and results return through the
`mcp_return(result)` helper function.

In the following example, the `pl-do` tool uses PL/pgSQL
to analyze table bloat and return results as JSON.

```yaml
tools:
  - name: analyze_table_bloat
    description: Analyze table bloat and dead tuples
    type: pl-do
    language: plpgsql
    input_schema:
      type: object
      properties:
        schema_name:
          type: string
          description: Schema to analyze
      required: []
    code: |
      DECLARE
          rec RECORD;
          schema_filter text;
      BEGIN
          result := '[]'::jsonb;
          schema_filter := args->>'schema_name';

          FOR rec IN
              SELECT s.schemaname, s.relname,
                     s.n_dead_tup, s.n_live_tup
              FROM pg_stat_user_tables s
              WHERE schema_filter IS NULL
                 OR s.schemaname = schema_filter
              ORDER BY s.n_dead_tup DESC
              LIMIT 20
          LOOP
              result := result || jsonb_build_object(
                  'schema', rec.schemaname,
                  'table', rec.relname,
                  'dead_tuples', rec.n_dead_tup,
                  'live_tuples', rec.n_live_tup
              );
          END LOOP;

          PERFORM set_config(
              'mcp.tool_result', result::text, true
          );
      END;
```

In the following example, the `pl-do` tool uses PL/Python
to calculate column statistics.

```yaml
tools:
  - name: calculate_column_stats
    description: Calculate statistics for a numeric column
    type: pl-do
    language: plpython3u
    timeout: "60s"
    input_schema:
      type: object
      properties:
        table_name:
          type: string
          description: Table to analyze
        column_name:
          type: string
          description: Numeric column to analyze
      required:
        - table_name
        - column_name
    code: |
      table = args.get('table_name')
      column = args.get('column_name')

      safe_table = plpy.quote_ident(table)
      safe_column = plpy.quote_ident(column)

      query = "SELECT {} as val FROM {} LIMIT 10000".format(
          safe_column, safe_table
      )
      rows = plpy.execute(query)
      values = [float(r['val']) for r in rows
                if r['val'] is not None]
      n = len(values)
      mean = sum(values) / n

      mcp_return({
          "table": table,
          "column": column,
          "count": n,
          "mean": round(mean, 4),
          "min": min(values),
          "max": max(values)
      })
```

#### PL/* Function Tools

The `pl-func` type creates temporary PL/* functions with
proper return types. The server creates the function, calls the
function, and drops the function automatically. Arguments
arrive as a single `args` JSONB parameter. The `returns`
field specifies the SQL return type.

A `pl-func` tool has three separate requirements, and all three
must be met before it will run:

- The database role the server connects as needs `CREATE`
  permission on the schema, because the tool creates and drops a
  function on each call.

- The tool's `language` must be listed in the
  `allowed_pl_languages` setting for the target database, which is
  a server-side configuration rather than a database privilege. The
  default permits `plpgsql` only, so a tool written in any other
  language is silently withheld until an operator adds that
  language; see [Configuring Allowed
  Languages](#configuring-allowed-languages) below.

- The database connection itself needs `allow_writes: true`,
  because creating and dropping the function is a catalogue write
  regardless of what the tool's own SQL does. A `pl-func` tool
  defined against a connection without `allow_writes` is refused
  when called, naming that setting as the fix; use a `pl-do` tool
  instead where the connection must stay read-only.

Missing any one of the three leaves the tool unusable, and not
always in the same way. A tool whose language is not allowed does
not appear in `tools/list` at all, rather than appearing and then
failing, so check the configuration first if a `pl-func` tool you
have defined is missing. A tool that fails the `allow_writes`
check behaves differently: it does appear in `tools/list`, and
only fails, with the reason above, once something calls it.

In the following example, the `pl-func` tool creates a
temporary function that returns a table of row counts.

```yaml
tools:
  - name: get_table_row_counts
    description: Get row counts for tables in a schema
    type: pl-func
    language: plpgsql
    returns: "TABLE(schema_name text, table_name text, row_count bigint)"
    input_schema:
      type: object
      properties:
        schema_pattern:
          type: string
          description: Schema name pattern (LIKE wildcards)
      required: []
    code: |
      DECLARE
          tbl RECORD;
          cnt bigint;
          pattern text;
      BEGIN
          pattern := COALESCE(
              args->>'schema_pattern', 'public'
          );

          FOR tbl IN
              SELECT t.schemaname, t.tablename
              FROM pg_tables t
              WHERE t.schemaname LIKE pattern
              ORDER BY t.schemaname, t.tablename
          LOOP
              EXECUTE format(
                  'SELECT count(*) FROM %I.%I',
                  tbl.schemaname, tbl.tablename
              ) INTO cnt;
              schema_name := tbl.schemaname;
              table_name := tbl.tablename;
              row_count := cnt;
              RETURN NEXT;
          END LOOP;
      END;
```

### Configuring Allowed Languages

Procedural language tools require the language to be listed
in the `allowed_pl_languages` configuration for the target
database. The default setting allows only `plpgsql`.

In the following example, the server configuration enables
multiple procedural languages for a database.

```yaml
databases:
  - name: mydb
    # ... connection details ...
    allowed_pl_languages:
      - plpgsql
      - plpython3u
      - plv8
      - plperl
      - plperlu
```

To allow all installed procedural languages, set the value
to `["*"]`.

```yaml
allowed_pl_languages:
  - "*"
```

### Accessing Arguments in PL/* Code

Each procedural language accesses the `args` variable and
returns results differently. The following sections describe
the conventions for each supported language.

#### PL/pgSQL Arguments

For `pl-do` blocks, the server pre-defines `args` as a
`jsonb` variable and `result` as a `jsonb` variable.

In the following example, a PL/pgSQL `pl-do` block accesses
arguments and returns results.

```sql
DECLARE
    my_value text;
BEGIN
    my_value := args->>'key_name';
    result := jsonb_build_object('value', my_value);
    PERFORM set_config(
        'mcp.tool_result', result::text, true
    );
END;
```

For `pl-func` functions, `args` is the JSONB function
parameter. The function returns results through the standard
`RETURN` mechanism.

In the following example, a PL/pgSQL `pl-func` function
accesses arguments and returns a value.

```sql
DECLARE
    my_value text;
BEGIN
    my_value := args->>'key_name';
    RETURN my_value;
END;
```

#### PL/Python Arguments

For `pl-do` blocks, `args` is a pre-defined Python dict.
The `mcp_return(result)` function is auto-injected and
accepts strings or dicts. Dicts are auto-serialized to JSON.

In the following example, a PL/Python `pl-do` block accesses
arguments and returns results.

```python
value = args.get('key_name')
mcp_return({"result": value})
```

For `pl-func` functions, `args` is a Python dict parsed
from the JSONB parameter. The function returns results
through the Python `return` statement.

In the following example, a PL/Python `pl-func` function
accesses arguments and returns a value.

```python
import json
value = args.get('key_name')
return json.dumps({"result": value})
```

#### PLV8 (JavaScript) Arguments

For `pl-do` blocks, `args` is a pre-defined JavaScript
object. The `mcp_return(result)` function accepts strings
or objects.

In the following example, a PLV8 `pl-do` block accesses
arguments and returns results.

```javascript
var value = args.key_name;
mcp_return({result: value});
```

For `pl-func` functions, `args` is a JavaScript object
parsed from JSON. The function returns results through the
JavaScript `return` statement.

In the following example, a PLV8 `pl-func` function accesses
arguments and returns a value.

```javascript
var value = args.key_name;
return JSON.stringify({result: value});
```

#### PL/Perl Untrusted Arguments

For `pl-do` blocks, `$args` is a pre-defined Perl hash ref
parsed via `JSON.pm`. The `mcp_return($result)` function
accepts strings or hash refs; hash refs are auto-serialized.

In the following example, a `plperlu` `pl-do` block accesses
arguments and returns results.

```perl
my $value = $args->{'key_name'};
mcp_return({result => $value});
```

For `pl-func` functions, `$args` is a Perl hash ref decoded
from JSONB via `JSON.pm`. The function returns results through
the Perl `return` statement.

In the following example, a `plperlu` `pl-func` function
accesses arguments and returns a value.

```perl
my $value = $args->{'key_name'};
return encode_json({result => $value});
```

#### PL/Perl Trusted Arguments

Trusted `plperl` works similarly to `plperlu` but with
restrictions. The server parses arguments using PostgreSQL's
`jsonb_each_text` through SPI instead of `JSON.pm`. All
values are text strings; nested structures are flattened.
Trusted `plperl` cannot load external Perl modules.

For `pl-do` blocks, `$args` is a pre-defined Perl hash ref.
The `mcp_return($result)` function accepts strings or hash
refs.

In the following example, a `plperl` `pl-do` block accesses
arguments and returns results.

```perl
my $value = $args->{'key_name'};
mcp_return({result => $value});
```

For `pl-func` functions, `$args` is a Perl hash ref parsed
via SPI and `jsonb_each_text`. The function returns results
through the Perl `return` statement.

### Language Arguments Reference

The following table summarizes argument access and return
conventions for each language and tool type.

| Language | Type | Args Variable | Access Pattern | Return Method |
|----------|------|---------------|----------------|---------------|
| `plpgsql` | `pl-do` | `args` (jsonb) | `args->>'key'` | `PERFORM set_config(...)` |
| `plpgsql` | `pl-func` | `args` (jsonb) | `args->>'key'` | `RETURN` |
| `plpython3u` | `pl-do` | `args` (dict) | `args.get('key')` | `mcp_return(result)` |
| `plpython3u` | `pl-func` | `args` (dict) | `args.get('key')` | `return` |
| `plv8` | `pl-do` | `args` (object) | `args.key` | `mcp_return(result)` |
| `plv8` | `pl-func` | `args` (object) | `args.key` | `return` |
| `plperlu` | `pl-do` | `$args` (hashref) | `$args->{'key'}` | `mcp_return($result)` |
| `plperlu` | `pl-func` | `$args` (hashref) | `$args->{'key'}` | `return` |
| `plperl` | `pl-do` | `$args` (hashref) | `$args->{'key'}` | `mcp_return($result)` |
| `plperl` | `pl-func` | `$args` (hashref) | `$args->{'key'}` | `return` |

## URI Conventions

Resource URIs should follow these conventions:

- Use the `custom://` prefix for user-defined resources.
- Use lowercase with hyphens: `custom://my-resource`.
- Use descriptive names: `custom://active-users` instead of
  `custom://users1`.
- Avoid conflicts with built-in URIs such as
  `pg://system_info`.

## Validation Rules

The server validates definitions at startup to ensure they
meet all requirements.

**Prompt Validation**

The server validates the following requirements for prompt
definitions:

- The `name` field is required and must be unique.
- At least one `message` is required.
- The message `role` must be `user`, `assistant`, or `system`.
- The content `type` must be `text`, `image`, or `resource`.
- Template placeholders must reference declared arguments.
- The argument `name` is required if arguments are defined.

**Resource Validation**

The server validates the following requirements for resource
definitions:

- The `uri` field is required and must be unique.
- The `name` field is required.
- The `type` field is required; valid values are `sql` or
  `static`.
- The `sql` type requires a `sql` field with a query.
- The `static` type requires a `data` field.
- The `mimeType` defaults to `application/json` if not
  specified.

**Tool Validation**

The server validates the following requirements for tool
definitions:

- The `name` field is required and must be unique across all
  tools.
- The `type` field is required; valid values are `sql`,
  `pl-do`, or `pl-func`.
- The `input_schema.type` must be `object`.
- Property types must be `string`, `integer`, `number`,
  `boolean`, `array`, or `object`.
- Required properties must exist in the properties list.
- The `sql` type requires a `sql` field.
- The `pl-do` type requires `language` and `code` fields.
- The `pl-func` type requires `language`, `code`, and
  `returns` fields.
- The `language` value must be alphanumeric (for example,
  `plpgsql` or `plpython3u`).
- The `returns` value must be a valid SQL type pattern.

**Validation Errors**

If validation fails, the server logs the error and exits.
Check `stderr` for details.

In the following example, validation error messages indicate
specific issues with the definitions file.

```
ERROR: Failed to load custom definitions: prompt 0: name is required
ERROR: Failed to load custom definitions: resource 1: duplicate resource URI: custom://my-resource
ERROR: Failed to load custom definitions: tool 0: type is required
```

## Security Considerations

Custom definitions should be designed with security in mind
to protect the database and data.

### Protecting Against SQL Injection

SQL resources execute the exact query specified in the
definition file. SQL tools use parameterized queries with
`$1`, `$2` placeholders to prevent injection attacks.

When writing SQL resources, follow these guidelines:

- Hardcode queries as trusted content (not accepting runtime
  user input).
- Use read-only queries when possible (`SELECT` queries).
- Apply appropriate restrictions (`LIMIT` clauses, `WHERE`
  filters).

### PL/* Code Security

Procedural language tools execute user-provided code from
the definitions file. Treat the definitions file as trusted
code. Follow these guidelines for PL/* tools:

- Restrict allowed languages using `allowed_pl_languages`
  per database.
- Validate and quote identifiers in PL/* code to prevent
  injection.
- Use `quote_ident()` in PL/pgSQL and `plpy.quote_ident()`
  in PL/Python.
- Use the `%I` format specifier with `format()` in PL/pgSQL.
- The `pl-func` type creates and immediately drops temporary
  functions.

### Connection Isolation

SQL resources and tools respect per-token connection
isolation when authentication is enabled. Each authenticated
user's queries execute with their own database connection.

### File Security

Protect the definitions file with the following measures:

- Store the file in a secure location with appropriate
  permissions.
- Do not expose sensitive data in static resources.
- Review SQL queries for potential information disclosure.


## Using Custom Definitions

Once defined, custom prompts, resources, and tools can be
discovered and used through the MCP protocol.

Custom prompts appear in the `prompts` list. The following
`prompts/list` command lists all available prompts:

```
prompts/list
```

Custom resources appear in the `resources` list. The
following `resources/list` command lists all available
resources:

```
resources/list
```

Custom tools appear in the `tools` list. The following
`tools/list` command lists all available tools:

```
tools/list
```

### Using Custom Prompts

In the following example, the `prompts/get` method executes
a custom prompt with arguments.

```json
{
  "method": "prompts/get",
  "params": {
    "name": "analyze-table",
    "arguments": {
      "table_name": "users"
    }
  }
}
```

### Using Custom Resources

In the following example, the `resources/read` method
retrieves data from a custom resource.

```json
{
  "method": "resources/read",
  "params": {
    "uri": "custom://active-users"
  }
}
```

In the following example, the `read_resource` tool retrieves
data from a custom resource.

```json
{
  "method": "tools/call",
  "params": {
    "name": "read_resource",
    "arguments": {
      "uri": "custom://active-users"
    }
  }
}
```

### Using Custom Tools

In the following example, the `tools/call` method invokes
a custom SQL tool with arguments.

```json
{
  "method": "tools/call",
  "params": {
    "name": "get_database_stats",
    "arguments": {
      "database_name": "production"
    }
  }
}
```

In the following example, the `tools/call` method invokes
a custom `pl-do` tool with arguments.

```json
{
  "method": "tools/call",
  "params": {
    "name": "analyze_table_bloat",
    "arguments": {
      "schema_name": "public"
    }
  }
}
```
