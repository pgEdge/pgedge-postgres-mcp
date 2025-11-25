# User-Defined Prompts and Resources Implementation Plan

## Overview

Add support for loading user-defined Prompts and Resources from an
external file, enabling users to extend the MCP server without modifying
Go code.

## Requirements

### Prompts

- Load prompt definitions from file
- Support all MCP prompt fields: name, description, arguments
- Support message templates with argument interpolation
- Multiple message support (user, assistant, system roles)

### Resources

- Load resource definitions from file
- Support all MCP resource fields: uri, name, description, mimeType
- Three data source types:
  1. **SQL Query**: Execute query and return results
  2. **Static Single Value**: Return a single scalar value
  3. **Static Row/Rows**: Return array or 2D array of values

### Configuration

- Add config field specifying path to definitions file
- File is optional (server works without it)
- Support both JSON and YAML formats

## File Format Design

### JSON Structure

```json
{
  "prompts": [
    {
      "name": "string (required)",
      "description": "string (optional)",
      "arguments": [
        {
          "name": "string (required)",
          "description": "string (optional)",
          "required": true|false
        }
      ],
      "messages": [
        {
          "role": "user|assistant|system",
          "content": {
            "type": "text|image|resource",
            "text": "Template with {{arg_name}} placeholders",
            "data": "base64 for images",
            "mimeType": "for images",
            "uri": "for resources"
          }
        }
      ]
    }
  ],
  "resources": [
    {
      "uri": "string (required, e.g., custom://my-resource)",
      "name": "string (required)",
      "description": "string (optional)",
      "mimeType": "string (default: application/json)",
      "type": "sql|static",
      "sql": "SELECT query (when type=sql)",
      "data": "value|[values]|[[values]] (when type=static)"
    }
  ]
}
```

### YAML Structure

```yaml
prompts:
  - name: example-prompt
    description: Description text
    arguments:
      - name: arg_name
        description: Argument description
        required: true
    messages:
      - role: user
        content:
          type: text
          text: "Template with {{arg_name}}"

resources:
  - uri: custom://example-sql
    name: Example SQL Resource
    description: Query results
    mimeType: application/json
    type: sql
    sql: SELECT * FROM users LIMIT 10

  - uri: custom://static-value
    name: Static Single Value
    type: static
    data: "Hello World"

  - uri: custom://static-row
    name: Static Row
    type: static
    data: ["col1", "col2", "col3"]

  - uri: custom://static-rows
    name: Static Multiple Rows
    type: static
    data:
      - ["Alice", 30]
      - ["Bob", 25]
```

## Implementation Details

### 1. Configuration Extension

**File**: `internal/config/config.go`

Add field:
```go
type Config struct {
    // ... existing fields ...
    CustomDefinitionsPath string `yaml:"custom_definitions_path"`
}
```

### 2. Definitions Loader Package

**New Package**: `internal/definitions/`

**Files**:
- `loader.go` - Main loader logic
- `types.go` - Definition structures
- `validator.go` - Validation logic

**Key Types**:
```go
type Definitions struct {
    Prompts   []PromptDefinition   `json:"prompts" yaml:"prompts"`
    Resources []ResourceDefinition `json:"resources" yaml:"resources"`
}

type PromptDefinition struct {
    Name        string           `json:"name" yaml:"name"`
    Description string           `json:"description" yaml:"description"`
    Arguments   []ArgumentDef    `json:"arguments" yaml:"arguments"`
    Messages    []MessageDef     `json:"messages" yaml:"messages"`
}

type ArgumentDef struct {
    Name        string `json:"name" yaml:"name"`
    Description string `json:"description" yaml:"description"`
    Required    bool   `json:"required" yaml:"required"`
}

type MessageDef struct {
    Role    string     `json:"role" yaml:"role"`
    Content ContentDef `json:"content" yaml:"content"`
}

type ContentDef struct {
    Type     string `json:"type" yaml:"type"`
    Text     string `json:"text,omitempty" yaml:"text,omitempty"`
    Data     string `json:"data,omitempty" yaml:"data,omitempty"`
    MimeType string `json:"mimeType,omitempty" yaml:"mimeType,omitempty"`
    URI      string `json:"uri,omitempty" yaml:"uri,omitempty"`
}

type ResourceDefinition struct {
    URI         string      `json:"uri" yaml:"uri"`
    Name        string      `json:"name" yaml:"name"`
    Description string      `json:"description" yaml:"description"`
    MimeType    string      `json:"mimeType" yaml:"mimeType"`
    Type        string      `json:"type" yaml:"type"` // "sql" or "static"
    SQL         string      `json:"sql,omitempty" yaml:"sql,omitempty"`
    Data        interface{} `json:"data,omitempty" yaml:"data,omitempty"`
}
```

**Loader Function**:
```go
func LoadDefinitions(path string) (*Definitions, error)
```

### 3. Prompt Registry Extension

**File**: `internal/prompts/registry.go`

Add method:
```go
func (r *Registry) RegisterStatic(def PromptDefinition) error
```

**Handler Creation**:
- Parse message templates
- Implement {{arg_name}} substitution
- Build PromptResult with interpolated messages

### 4. Resource Registry Extension

**File**: `internal/resources/registry.go` or new file

Add methods:
```go
func (r *Registry) RegisterSQL(def ResourceDefinition, db *sql.DB) error
func (r *Registry) RegisterStatic(def ResourceDefinition) error
```

**SQL Handler**:
- Execute query using connection from context
- Marshal results to JSON
- Return as ResourceContent

**Static Handler**:
- Format data based on type (value, array, 2D array)
- Marshal to JSON
- Return as ResourceContent

### 5. Context-Aware Resource Integration

**Consideration**: SQL resources need database access

**Options**:
1. Pass ClientManager to loader
2. Create handlers that accept context and fetch connection
3. Register with ContextAwareRegistry

**Recommended**: Option 3 - Register SQL resources with
ContextAwareRegistry so they respect per-token isolation.

### 6. Startup Integration

**File**: `cmd/pgedge-pg-mcp-svr/main.go`

After creating registries, before server start:
```go
// Load custom definitions if configured
if cfg.CustomDefinitionsPath != "" {
    defs, err := definitions.LoadDefinitions(cfg.CustomDefinitionsPath)
    if err != nil {
        log.Fatalf("Failed to load custom definitions: %v", err)
    }

    // Register custom prompts
    for _, promptDef := range defs.Prompts {
        if err := promptRegistry.RegisterStatic(promptDef); err != nil {
            log.Fatalf("Failed to register prompt %s: %v",
                promptDef.Name, err)
        }
    }

    // Register custom resources
    for _, resDef := range defs.Resources {
        if resDef.Type == "sql" {
            if err := contextAwareResourceProvider.RegisterSQL(
                resDef); err != nil {
                log.Fatalf("Failed to register resource %s: %v",
                    resDef.URI, err)
            }
        } else {
            if err := contextAwareResourceProvider.RegisterStatic(
                resDef); err != nil {
                log.Fatalf("Failed to register resource %s: %v",
                    resDef.URI, err)
            }
        }
    }
}
```

## Documentation

### Location

**File**: `docs/custom-definitions.md`

### Content Outline

1. **Overview**: What are custom prompts and resources
2. **Configuration**: How to specify the definitions file path
3. **File Format**: JSON and YAML examples
4. **Prompts Section**:
   - Structure explanation
   - Argument interpolation syntax
   - Multiple messages
   - Role types
   - Content types
5. **Resources Section**:
   - Structure explanation
   - SQL resources
   - Static resources (value, row, rows)
   - URI conventions (recommend custom:// prefix)
6. **Complete Examples**: Full working examples
7. **Validation Rules**: What makes a valid definition
8. **Troubleshooting**: Common errors

### Sample Files

**Location**: `examples/`

**Files**:
- `custom-definitions.json` - Complete JSON example
- `custom-definitions.yaml` - Complete YAML example
- `prompts-only.json` - Just prompts
- `resources-only.json` - Just resources

## Testing Strategy

### Unit Tests

**File**: `internal/definitions/loader_test.go`
- Test JSON parsing
- Test YAML parsing
- Test validation errors
- Test missing optional fields
- Test various data formats

**File**: `internal/prompts/registry_test.go`
- Test static prompt registration
- Test argument interpolation
- Test multiple messages
- Test missing arguments error

**File**: `internal/resources/registry_test.go` or new test file
- Test SQL resource registration
- Test static resource registration (value, row, rows)
- Test SQL execution
- Test data formatting

### Integration Tests

**File**: `cmd/pgedge-pg-mcp-svr/main_test.go` or new file
- Test loading definitions at startup
- Test prompt execution via MCP
- Test resource reading via MCP
- Test with authentication enabled
- Test per-token isolation for SQL resources

### Test Data

**Location**: `testdata/`
- `valid-definitions.json`
- `valid-definitions.yaml`
- `invalid-definitions.json` (various error cases)
- `prompts-only.json`
- `resources-only.json`

## Validation Rules

### Prompts

1. `name` is required and unique
2. `arguments[].name` is required
3. `messages` array has at least one message
4. `messages[].role` is one of: user, assistant, system
5. `messages[].content.type` is one of: text, image, resource
6. Template placeholders reference declared arguments

### Resources

1. `uri` is required and unique
2. `name` is required
3. `type` is required, must be "sql" or "static"
4. If `type` == "sql", `sql` field is required
5. If `type` == "static", `data` field is required
6. `mimeType` defaults to "application/json" if not specified
7. SQL queries should not be destructive (optional warning for
   INSERT/UPDATE/DELETE)

## Error Handling

- Invalid file format: Log error, exit at startup
- File not found: Log error, exit at startup
- Validation errors: Log specific error, exit at startup
- SQL execution error: Return error in ResourceContent
- Missing argument in prompt: Return error in PromptResult
- Duplicate names/URIs: Log error, exit at startup

## Security Considerations

1. **SQL Injection**: Since SQL is user-provided, document that queries
   should not accept user input or should use parameterized queries
   (future enhancement)
2. **Resource Isolation**: SQL resources must respect per-token database
   connections in auth mode
3. **File Path**: Validate that custom definitions file path is absolute
   or relative to server working directory

## Future Enhancements (Out of Scope)

1. Parameterized SQL queries (bind variables from context)
2. Hot-reloading of definitions without restart
3. Resource templates with arguments
4. Conditional prompts based on database state
5. Custom resource content types (CSV, XML, etc.)
6. Prompt argument validation (type checking)

## Implementation Order

1. Create definitions package with types and loader
2. Add validation logic
3. Extend prompt registry with static registration
4. Extend resource registry with SQL and static registration
5. Add configuration field
6. Integrate at startup
7. Create sample files
8. Write documentation
9. Write tests
10. Test end-to-end with sample files

## Success Criteria

- [ ] User can specify definitions file path in config
- [ ] Server loads JSON and YAML files successfully
- [ ] Custom prompts appear in prompts/list
- [ ] Custom prompts execute with argument interpolation
- [ ] Custom SQL resources execute queries and return results
- [ ] Custom static resources return formatted data
- [ ] Resources respect per-token isolation in auth mode
- [ ] All tests pass
- [ ] Documentation is complete with examples
- [ ] Sample files demonstrate all features
