# Custom Prompts and Resources

The pgEdge PostgreSQL MCP Server supports user-defined custom prompts and
resources, allowing you to extend the server's functionality without
modifying code.

## Overview

Custom definitions enable you to:

- **Define Prompts**: Create reusable prompt templates that guide the LLM
  through specific workflows
- **SQL Resources**: Expose frequently-used database queries as MCP
  resources
- **Static Resources**: Provide configuration data, documentation, or other
  static information

## Configuration

To enable custom definitions, specify the path to your definitions file in
the server configuration:

### YAML Configuration

```yaml
# In pgedge-pg-mcp-svr.yaml
custom_definitions_path: "/path/to/pgedge-nla-server-custom.yaml"
```

### Environment Variable

```bash
export PGEDGE_CUSTOM_DEFINITIONS_PATH="/path/to/pgedge-nla-server-custom.yaml"
```

### Supported Format

- YAML (`.yaml`, `.yml`)

## File Structure

A definitions file contains two optional sections:

```yaml
prompts:
  - # Prompt definitions
resources:
  - # Resource definitions
```

Both sections are optional - you can define only prompts, only resources,
or both.

## Prompts

### Prompt Structure

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

Use `{{argument_name}}` syntax in message text to interpolate argument
values:

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

When called with `{"table_name": "users"}`, the placeholders are replaced
with "users".

### Message Roles

- **user**: Instructions or questions from the user
- **assistant**: Example responses or context from the assistant
- **system**: System-level instructions or context

### Content Types

- **text**: Plain text with optional template placeholders
- **image**: Base64-encoded image data (requires `data` and `mimeType`
  fields)
- **resource**: Reference to another resource (requires `uri` field)

### Example: Simple Prompt

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

### Example: Prompt with Arguments

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

## Resources

### Resource Types

#### SQL Resources

Execute a SQL query and return results as JSON.

```yaml
resources:
  - uri: custom://resource-name    # Required: Unique URI
    name: Display Name             # Required: Human-readable name
    description: What it returns   # Optional: Description
    mimeType: application/json     # Optional: Default is application/json
    type: sql                      # Required: Resource type
    sql: SELECT * FROM users       # Required: SQL query to execute
```

**Features**:

- Executes query using the appropriate database connection
- Respects per-token connection isolation in authenticated mode
- Returns results as JSON array of objects
- Column names become JSON object keys

**Example**:

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

#### Static Resources

Return predefined static data.

```yaml
resources:
  - uri: custom://resource-name    # Required: Unique URI
    name: Display Name             # Required: Human-readable name
    description: What it contains  # Optional: Description
    mimeType: application/json     # Optional: Default is application/json
    type: static                   # Required: Resource type
    data: value                    # Required: Static data (various formats)
```

**Data Formats**:

1. **Single Value**: Scalar value (string, number, boolean)
2. **Single Row**: Array of values
3. **Multiple Rows**: 2D array (array of arrays)
4. **Object**: Key-value pairs

**Example: Single Value**

```yaml
resources:
  - uri: custom://environment
    name: Environment
    description: Current environment name
    type: static
    data: "production"
```

**Example: Single Row**

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

- Use the `custom://` prefix for user-defined resources
- Use lowercase with hyphens: `custom://my-resource`
- Be descriptive: `custom://active-users` not `custom://users1`
- Avoid conflicts with built-in URIs (`pg://system-info`, etc.)

## Validation Rules

The server validates definitions at startup:

### Prompts

- `name` is required and must be unique
- At least one `message` is required
- Message `role` must be: user, assistant, or system
- Content `type` must be: text, image, or resource
- Template placeholders must reference declared arguments
- Argument `name` is required if arguments are defined

### Resources

- `uri` is required and must be unique
- `name` is required
- `type` is required (sql or static)
- SQL type requires `sql` field with query
- Static type requires `data` field
- `mimeType` defaults to `application/json` if not specified

### Validation Errors

If validation fails, the server logs the error and exits. Check stderr
for details:

```
ERROR: Failed to load custom definitions: prompt 0: name is required
ERROR: Failed to load custom definitions: resource 1: duplicate resource URI: custom://my-resource
```

## Security Considerations

### SQL Injection

SQL resources execute the exact query specified in the definition file.
Ensure queries are:

- Hardcoded and trusted (not accepting runtime user input)
- Read-only when possible (SELECT queries)
- Appropriately restricted (LIMIT clauses, WHERE filters)

**Note**: Future versions may support parameterized queries with runtime
binding.

### Connection Isolation

SQL resources respect per-token connection isolation when authentication
is enabled. Each authenticated user's queries execute with their own
database connection.

### File Security

Protect your definitions file:

- Store in a secure location with appropriate permissions
- Don't expose sensitive data in static resources
- Review SQL queries for potential information disclosure

## Complete Example

See `examples/pgedge-nla-server-custom.yaml` for a comprehensive example
demonstrating all features:

```bash
# View the example file
cat examples/pgedge-nla-server-custom.yaml

# Use it in your configuration
custom_definitions_path: "./examples/pgedge-nla-server-custom.yaml"
```

## Usage

### Discovering Custom Definitions

Custom prompts appear in the prompts list:

```
prompts/list
```

Custom resources appear in the resources list:

```
resources/list
```

### Using Custom Prompts

Execute a custom prompt:

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

Read a custom resource:

```json
{
  "method": "resources/read",
  "params": {
    "uri": "custom://active-users"
  }
}
```

Or via the backward-compatible tool:

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

## Troubleshooting

### File Not Loading

**Problem**: Server logs error about missing file

**Solution**: Check that:

- File path is absolute or relative to server working directory
- File exists and is readable
- File extension is `.json`, `.yaml`, or `.yml`

### Validation Errors

**Problem**: Server exits with validation error

**Solution**:

- Check error message for specific issue
- Verify all required fields are present
- Ensure names/URIs are unique
- Confirm template placeholders reference defined arguments

### SQL Errors

**Problem**: Resource returns SQL error

**Solution**:

- Test query directly in psql
- Check table/column names
- Verify user has necessary permissions
- Ensure query syntax is valid for your PostgreSQL version

### Template Not Interpolating

**Problem**: Seeing literal `{{arg_name}}` in output

**Solution**:

- Verify argument is declared in `arguments` section
- Check argument name matches exactly (case-sensitive)
- Ensure you passed the argument when calling the prompt

## Best Practices

1. **Use Descriptive Names**: Choose clear, self-documenting names for
   prompts and resources
2. **Document Everything**: Provide descriptions for prompts, arguments,
   and resources
3. **Test Queries**: Verify SQL queries work correctly before deploying
4. **Use LIMIT**: Add LIMIT clauses to prevent returning excessive data
5. **Version Control**: Store definitions files in version control
6. **Start Simple**: Begin with a few definitions and expand gradually
7. **Follow Conventions**: Use `custom://` prefix and kebab-case for URIs

## Limitations

Current limitations (may be addressed in future versions):

- SQL resources cannot accept runtime parameters
- No hot-reloading (requires server restart)
- No conditional logic in prompts
- No resource templates with arguments
- Limited to JSON output for resources

## Related Documentation

- [Introduction](index.md) - Getting started with the server
- [API Reference](api-reference.md) - MCP protocol details
- [Built-in Resources](resources.md) - Available built-in resources
- [Built-in Prompts](prompts.md) - Available built-in prompts
