# Custom Prompts and Resources

The MCP server supports user-defined custom prompts and resources, allowing
you to extend the server's functionality without modifying code.

For information about using the server's built-in resources and prompts:

- [Built-in Resources](../reference/resources.md) - Available built-in resources.
- [Built-in Prompts](../reference/prompts.md) - Available built-in prompts.

Custom prompt and resource definitions enable you to:

- **Define Prompts**: Create reusable prompt templates that guide the LLM
  through specific workflows
- **SQL Resources**: Expose frequently-used database queries as MCP
  resources
- **Static Resources**: Provide configuration data, documentation, or other
  static information

When defining a prompt or resource:

* **Use Descriptive Names**: Choose clear, self-documenting names for prompts and resources.
* **Document Everything**: Provide descriptions for prompts, arguments, and resources.
* **Test Queries**: Verify SQL queries work correctly before deploying.
* **Use LIMIT**: Add LIMIT clauses to prevent returning excessive data.
* **Version Control**: Store definitions files in version control.
* **Start Simple**: Begin with a few definitions and expand gradually.
* **Follow Conventions**: Use `custom://` prefix and kebab-case for URIs.

!!! note

    Current limitations (that may be addressed in future versions):

    - SQL resources cannot accept runtime parameters.
    - No hot-reloading (requires server restart).
    - No conditional logic in prompts.
    - No resource templates with arguments.
    - Limited to JSON output for resources.

See `examples/pgedge-postgres-mcp-custom.yaml` for a comprehensive example that demonstrates using all of the MCP server features.  The following commands show how to view and use the example definitions file.

```bash
# View the example file
cat examples/pgedge-postgres-mcp-custom.yaml

# Use it in your configuration
custom_definitions_path: "./examples/pgedge-postgres-mcp-custom.yaml"
```


## Configuring Custom Definitions

To enable custom definitions, specify the path to your definitions file in the server configuration. You can configure the path using either YAML configuration or environment variables.

**YAML Configuration**

In the following example, the server configuration uses the `custom_definitions_path` parameter to specify the location of the custom definitions file.

```yaml
# In pgedge-postgres-mcp.yaml
custom_definitions_path: "/path/to/pgedge-postgres-mcp-custom.yaml"
```

**Environment Variable**

In the following example, the `PGEDGE_CUSTOM_DEFINITIONS_PATH` environment variable specifies the location of the custom definitions file.

```bash
export PGEDGE_CUSTOM_DEFINITIONS_PATH="/path/to/pgedge-postgres-mcp-custom.yaml"
```

### Supported Format

- YAML (`.yaml`, `.yml`)

## Writing a Definitions File

A definitions file contains two optional sections; [`prompts`](#defining-prompts) and [`resources`](#defining-resources).  In the following example, the definitions file includes both `prompts` and `resources` sections.

```yaml
prompts:
  - # Prompt definitions
resources:
  - # Resource definitions
```

Both sections are optional - you can define only `prompts`, only `resources`, or both.

## Defining Prompts

Prompts are reusable templates that guide the LLM through specific workflows.

### Prompt Structure

In the following example, the prompt definition includes required and optional fields to define a reusable prompt template.

```yaml
prompts:
  - name: prompt-name               # Required: Unique identifier
    description: Description text   # Optional: What the prompt does
    arguments:                      # Optional: List of arguments
      - name: arg_name              # Required: Argument identifier
        description: Arg description  # Optional: What it's for
        required: true              # Optional: Is it required?
    messages:                       # Required: At least one message
      - role: user                  # Required: user, assistant, or system
        content:
          type: text                # Required: text, image, or resource
          text: "Template {{arg_name}}"  # Template with placeholders
```

### Template Interpolation

Use `{{argument_name}}` syntax in message text to interpolate argument values.

In the following example, the template uses `{{table_name}}` placeholders to interpolate the table name argument.

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

When called with `{"table_name": "users"}`, the placeholders are replaced with "users".

### Message Roles

- **user**: Instructions or questions from the user.
- **assistant**: Example responses or context from the assistant.
- **system**: System-level instructions or context.

### Content Types

- **text**: Plain text with optional template placeholders.
- **image**: Base64-encoded image data (requires `data` and `mimeType` fields).
- **resource**: Reference to another resource (requires `uri` field).

**Example: Simple Prompt**

In the following example, the prompt definition creates a security audit prompt without any arguments.

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

In the following example, the prompt definition uses two required arguments to compare database schemas.

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

Execute a SQL query and return results in TSV (tab-separated values) format
for token efficiency.

In the following example, the resource definition specifies the required fields for a SQL resource.

```yaml
resources:
  - uri: custom://resource-name    # Required: Unique URI
    name: Display Name             # Required: Human-readable name
    description: What it returns   # Optional: Description
    type: sql                      # Required: Resource type
    sql: SELECT * FROM users       # Required: SQL query to execute
```

This example:

- Executes the query using the appropriate database connection.
- Respects per-token connection isolation in authenticated mode.
- Returns results in TSV format (first row is column headers).
- Escapes tabs, newlines, and carriage returns in values.
- Token-efficient output for LLM consumption.

**Example:**

In the following example, the SQL resource queries PostgreSQL to list all active database users.

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

Return predefined static data.

In the following example, the resource definition specifies the required fields for a static resource.

```yaml
resources:
  - uri: custom://resource-name    # Required: Unique URI
    name: Display Name             # Required: Human-readable name
    description: What it contains  # Optional: Description
    mimeType: application/json     # Optional: Default is application/json
    type: static                   # Required: Resource type
    data: value                    # Required: Static data (various formats)
```

**Data Formats:**

* **Single Value**: Scalar value (string, number, boolean).
* **Single Row**: Array of values.
* **Multiple Rows**: 2D array (array of arrays).
* **Object**: Key-value pairs.

**Example: Single Value**

In the following example, the static resource returns a single scalar value representing the environment name.

```yaml
resources:
  - uri: custom://environment
    name: Environment
    description: Current environment name
    type: static
    data: "production"
```

**Example: Single Row**

In the following example, the static resource returns an array of values representing support contact information.

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

In the following example, the static resource returns a 2D array representing a maintenance schedule.

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

In the following example, the `static` resource returns a configuration object with key-value pairs.

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

## URI Conventions

Resource URIs should follow these conventions:

- Use the `custom://` prefix for user-defined resources.
- Use lowercase with hyphens: `custom://my-resource`.
- Be descriptive: `custom://active-users` not `custom://users1`.
- Avoid conflicts with built-in URIs (`pg://system-info`, etc.).

## Validation Rules

The server validates definitions at startup to ensure they meet all requirements.

**Prompt Validation**

The server validates the following requirements for prompt definitions:

- `name` is required and must be unique.
- At least one `message` is required.
- Message `role` must be: user, assistant, or system.
- Content `type` must be: text, image, or resource.
- Template placeholders must reference declared arguments.
- Argument `name` is required if arguments are defined.

**Resource Validation**

The server validates the following requirements for resource definitions:

- `uri` is required and must be unique.
- `name` is required.
- `type` is required (sql or static).
- SQL type requires `sql` field with query.
- Static type requires `data` field.
- `mimeType` defaults to `application/json` if not specified.

**Validation Errors**

If validation fails, the server logs the error and exits; check `stderr` for details.

In the following example, validation error messages indicate specific issues with the definitions file.

```
ERROR: Failed to load custom definitions: prompt 0: name is required
ERROR: Failed to load custom definitions: resource 1: duplicate resource URI: custom://my-resource
```

## Security Considerations

Custom definitions should be designed with security in mind to protect your database and data.

### Protecting Against SQL Injection

SQL resources execute the exact query specified in the definition file. Ensure queries are:

- Hardcoded and trusted (not accepting runtime user input).
- Read-only when possible (SELECT queries).
- Appropriately restricted (LIMIT clauses, WHERE filters).

**Note:** Future versions may support parameterized queries with runtime binding.

### Connection Isolation

SQL resources respect per-token connection isolation when authentication is enabled. Each authenticated user's queries execute with their own database connection.

### File Security

Protect your definitions file:

- Store in a secure location with appropriate permissions.
- Don't expose sensitive data in static resources.
- Review SQL queries for potential information disclosure.


## Using Custom Prompts and Resources

Once defined, custom prompts and resources can be discovered and used through the MCP protocol.

Custom prompts appear in the `prompts` list.  The following `prompts/list` command lists all available prompts:

```
prompts/list
```

Custom resources appear in the `resources` list.  The following `resources/list` command lists all available resources:

```
resources/list
```

### Using Custom Prompts

In the following example, the `prompts/get` method executes a custom prompt with arguments.

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

In the following example, the `resources/read` method retrieves data from a custom resource.

```json
{
  "method": "resources/read",
  "params": {
    "uri": "custom://active-users"
  }
}
```

In the following example, the `read_resource` tool retrieves data from a custom resource.

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
