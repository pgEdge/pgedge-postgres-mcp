# MCP Prompts

Prompts are reusable workflow templates that guide LLMs through complex
multi-step processes. They provide structured guidance for common tasks like
setting up semantic search, exploring unfamiliar databases, or diagnosing
query issues.

## Disabling Prompts

Individual prompts can be disabled via configuration to restrict what the
LLM can access. See [Enabling/Disabling Built-in Features](../guide/feature_config.md)
for details.

When a prompt is disabled:

- It is not advertised to the LLM in the `prompts/list` response
- The prompt dropdown in the web UI will not show it
- Attempts to use it via `/prompt` return an error

## What are MCP Prompts?

MCP Prompts are a feature introduced in the Model Context Protocol (MCP)
version 2024-11-05. Unlike tools that execute specific actions, prompts
provide pre-written instructions that guide the LLM through a systematic
workflow. Think of them as expert playbooks that ensure consistent,
thorough analysis.

**Key Benefits**:

- **Consistent workflows**: Ensures important steps aren't missed
- **Token efficiency**: Reduces back-and-forth by providing complete guidance
upfront
- **Best practices**: Encodes domain expertise in reusable templates
- **Parameterized**: Accept arguments to customize workflows for specific
contexts

## Available Prompts

### design-schema

Guides the LLM through designing a PostgreSQL database schema based on
user requirements. Uses best practices, appropriate normalization levels,
and recommends PostgreSQL extensions where beneficial.

**Use Cases**:

- Designing a new database schema from requirements
- Getting schema recommendations for specific use cases (OLTP, OLAP, etc.)
- Learning PostgreSQL best practices for data modeling
- Generating CREATE TABLE statements with proper types and constraints

**Arguments**:

- `requirements` (required): Description of the application requirements
  and data needs
- `use_case` (optional): Primary use case - `oltp`, `olap`, `hybrid`, or
  `general` (default: `general`)
- `full_featured` (optional): If `true`, design a comprehensive
  production-ready schema. If `false` (default), design a minimal schema
  meeting only the stated requirements

**Workflow Overview**:

1. **Knowledge Base Research**: Searches for relevant schema design best
   practices if a knowledge base is available
2. **Requirements Analysis**: Analyzes data entities, relationships, and
   access patterns
3. **Normalization Strategy**: Applies appropriate normalization based on
   use case (3NF+ for OLTP, denormalized for OLAP)
4. **Data Type Selection**: Recommends PostgreSQL-specific types (UUID,
   TIMESTAMPTZ, NUMERIC, JSONB, TEXT, VECTOR)
5. **Index Design**: Suggests indexes based on query patterns
6. **Extension Recommendations**: Proposes relevant PostgreSQL extensions
7. **Schema Generation**: Produces complete CREATE TABLE statements

**CLI Example**:

```bash
/prompt design-schema requirements="E-commerce platform with products, orders, and customers"
```

**CLI Example with Use Case**:

```bash
/prompt design-schema requirements="Real-time analytics dashboard" use_case="olap"
```

**CLI Example for Production-Ready Schema**:

```bash
/prompt design-schema requirements="E-commerce platform" full_featured="true"
```

**Web UI Usage**:

1. Click the brain icon next to the send button
2. Select "design-schema" from the dropdown
3. Enter your requirements description
4. Optionally select a use case (oltp, olap, hybrid, general)
5. Optionally set full_featured to "true" for comprehensive schemas
6. Click "Execute Prompt"

**Normalization Guidance**:

The prompt applies different normalization strategies based on use case:

- **OLTP**: Third Normal Form (3NF) or higher for data integrity
- **OLAP**: Strategic denormalization for query performance
- **Hybrid**: Balanced approach with materialized views
- **General**: Context-appropriate normalization

**Design Modes**:

- **Minimal (default)**: Creates only tables and columns explicitly required.
  Does not add user accounts, audit logs, favorites, or other supporting
  tables unless requested. Prefers simpler solutions (pg_trgm over pgvector
  for basic text search).

- **Full-Featured**: Creates a comprehensive, production-ready schema with
  supporting tables, audit logging, user preferences, and future-proofing
  considerations. Use this when you want a complete application schema.

**PostgreSQL Extensions**:

The prompt considers these extensions where appropriate:

- `vector`: For AI/ML embeddings and semantic search (note: extension name
  is "vector", not "pgvector")
- `postgis`: For geographic/spatial data
- `pg_trgm`: For fuzzy text search
- `pgcrypto`: For cryptographic functions
- `ltree`: For hierarchical data

The prompt instructs the LLM to verify ALL extension names via the
knowledgebase before writing CREATE EXTENSION statements, as project names
often differ from the actual extension names.

**Data Type Recommendations**:

- Use `GENERATED ALWAYS AS IDENTITY` for auto-increment primary keys
- Use `UUID` only when the application provides IDs or for distributed
  systems
- Use `TIMESTAMPTZ` for timestamps (timezone-aware)
- Use `NUMERIC` for money/precision values
- Use `JSONB` for flexible/semi-structured data
- Use `TEXT` instead of VARCHAR (no performance difference)
- Use `VECTOR` for AI embeddings

**Anti-Patterns Avoided**:

- Using SERIAL instead of IDENTITY for auto-increment
- Generating UUIDs in the database when IDENTITY would suffice
- Entity-Attribute-Value (EAV) patterns
- Excessive nullable columns
- Missing foreign key constraints
- Inappropriate use of arrays for relationships
- Over-indexing or under-indexing
- Assuming extension names without verifying via knowledgebase
- Over-engineering: adding tables/columns "just in case"
- Using advanced extensions (pgvector) when simpler ones (pg_trgm) suffice

### diagnose-query-issue

Systematically diagnoses why queries are failing or returning unexpected
results. Helps identify connection, schema, or data issues.

**Use Cases**:

- Debugging "table not found" errors
- Understanding why queries return no results
- Verifying you're connected to the correct database
- Troubleshooting permission issues

**Arguments**:

- `issue_description` (optional): Description of the problem (e.g., "table
not found", "no results", "wrong database")

**Workflow Overview**:

1. **Database Connection**: Verifies which database you're connected to
2. **Schema Availability**: Checks if target table/schema exists
3. **Table Structure**: Inspects columns, types, and constraints
4. **Data Sampling**: Verifies table has data
5. **Common Issues Checklist**: Systematically checks typical problems
6. **Proposed Solutions**: Suggests fixes based on diagnosis

**CLI Example**:

```bash
/prompt diagnose-query-issue issue_description="table not found"
```

**CLI Example without Description**:

```bash
/prompt diagnose-query-issue
```

**Web UI Usage**:

1. Click the brain icon next to the send button
2. Select "diagnose-query-issue" from the dropdown
3. Optionally enter a description of the issue
4. Click "Execute Prompt"

**Common Issues Checklist**:

The prompt guides the LLM to check:

- Wrong database connected
- Table name misspelled or wrong schema
- No vector columns (for similarity_search)
- Embedding generation disabled
- Empty result set (query works but no matching data)
- Rate limit exceeded
- Permission denied

**Quick Checks**:

For fastest diagnosis, the prompt prioritizes:

1. Wrong database: Check system-info, switch if needed
2. Table doesn't exist: Run get_schema_info
3. No data in table: Sample with limit=1
4. Semantic search in non-vector table: Check vector_tables_only

### explore-database

Systematically explores an unfamiliar database to understand its structure,
capabilities, and available data.

**Use Cases**:

- Understanding a new database you're working with
- Discovering what data is available
- Identifying semantic search capabilities
- Planning queries or analyses

**Arguments**: None (fully automated workflow)

**Workflow Overview**:

1. **System Information**: Identifies which database you're connected to
2. **Table Overview**: Gets comprehensive schema information
3. **Structure Analysis**: Identifies table purposes and relationships
4. **Special Capabilities**: Detects pgvector columns, JSONB, foreign keys
5. **Data Sampling**: Optionally queries small samples from key tables
6. **Summary**: Provides findings and suggested next steps

**CLI Example**:

```bash
/prompt explore-database
```

**Web UI Usage**:

1. Click the brain icon next to the send button
2. Select "explore-database" from the dropdown
3. Click "Execute Prompt" (no arguments needed)

**Rate Limit Management**:

The prompt includes guidance to:

- Use `schema_name="public"` to reduce token usage
- Use `limit=5` for sample queries
- Cache results to avoid re-querying
- Filter by schema for large databases

**Early Exit Conditions**:

The workflow stops if:

- No tables found (wrong/empty database)
- Permission denied
- Specific data sought but not found

### setup-semantic-search

Sets up semantic search using the similarity_search tool. Guides the LLM
through discovering vector-capable tables, understanding their structure,
and executing optimal searches.

**Use Cases**:

- First-time semantic search setup
- Finding relevant documentation chunks
- Searching knowledge bases or Wikipedia-style articles
- RAG (Retrieval-Augmented Generation) workflows

**Arguments**:

- `query_text` (required): The natural language search query

**Workflow Overview**:

1. **Discovery**: Identifies tables with pgvector columns
2. **Selection**: Chooses the most appropriate table based on schema info
3. **Execution**: Runs similarity_search with optimal parameters
4. **Token Optimization**: Manages chunking and token budgets to avoid rate
limits

**CLI Example**:

```bash
/prompt setup-semantic-search query_text="What is pgAgent?"
```

**Web UI Usage**:

1. Click the brain icon (Psychology icon) next to the send button
2. Select "setup-semantic-search" from the dropdown
3. Enter your query text in the query_text field
4. Click "Execute Prompt"

**Parameters Guide**:

The prompt instructs the LLM to use these similarity_search parameters:

- `top_n`: 10 (balance between recall and token usage)
- `chunk_size_tokens`: 100 (manageable chunk size)
- `lambda`: 0.6 (balanced relevance vs diversity)
- `max_output_tokens`: 1000 (prevents rate limit issues)

**Token Budget Management**:

This prompt includes specific guidance on managing token budgets to avoid
rate limit errors. It instructs the LLM to:

- Start with conservative token limits (1000 tokens)
- Use moderate chunking (100 tokens per chunk)
- Limit initial searches to top 10 results
- Avoid multiple large searches in the same conversation turn

## Using Prompts

### CLI Client

Prompts are executed using the `/prompt` slash command:

**Syntax**:

```bash
/prompt <prompt-name> [arg1=value1] [arg2=value2] ...
```

**Examples**:

```bash
# Design a database schema
/prompt design-schema requirements="User management with roles and permissions"

# Diagnose issue
/prompt diagnose-query-issue issue_description="table not found"

# Explore database
/prompt explore-database

# Setup semantic search
/prompt setup-semantic-search query_text="What is PostgreSQL?"
```

**Quoted Arguments**:

Arguments with spaces must be quoted:

```bash
/prompt setup-semantic-search query_text="How does PostgreSQL handle transactions?"
```

Both single and double quotes are supported:

```bash
/prompt setup-semantic-search query_text='What is pgAgent?'
```

**List Available Prompts**:

```bash
/prompts
```

This displays all available prompts with their descriptions and arguments.

### Web UI Client

The web interface provides a graphical way to execute prompts:

**Access Prompts**:

1. Look for the brain icon (Psychology icon) in the input area, between the
   save button and send button
2. The icon only appears if prompts are available from the server

**Execute a Prompt**:

1. Click the brain icon to open the prompt popover
2. Select a prompt from the dropdown menu
3. Read the prompt description
4. Fill in any required arguments
5. Optionally fill in optional arguments
6. Click "Execute Prompt"

**Features**:

- **Validation**: Required arguments are validated before execution
- **Help Text**: Each argument shows its description as helper text
- **Auto-Reset**: Form resets after successful execution
- **Disabled During Execution**: Form is disabled while the prompt runs

**Visual Indicators**:

- Required arguments are marked with an asterisk (*)
- Invalid required fields show an error message
- Execute button is disabled while running

## Token Management and Rate Limits

Prompts are designed to help manage token usage and avoid rate limit errors:

**Conversation History Compaction**:

Both CLI and Web clients automatically compact conversation history before
each LLM call:

- Keeps first user message (for context)
- Keeps last 10 messages
- Prevents exponential token growth in multi-turn conversations

**Prompt-Specific Token Guidance**:

Each prompt includes token budget recommendations:

- `diagnose-query-issue`: Prioritizes quick checks before expensive
operations
- `explore-database`: Uses schema filtering and sample limits
- `setup-semantic-search`: Limits similarity_search to 1000 tokens by
default

**Best Practices**:

- Use prompts instead of freeform queries for complex workflows
- Start with conservative token limits
- Filter results with WHERE clauses when possible
- Use `limit` parameter in queries
- Avoid multiple large operations in one conversation turn

## Technical Details

### MCP Protocol Integration

Prompts are exposed via the MCP protocol's `prompts/list` and `prompts/get`
methods:

**List Prompts** (`prompts/list`):

```json
{
    "jsonrpc": "2.0",
    "method": "prompts/list",
    "id": 1
}
```

**Get Prompt** (`prompts/get`):

```json
{
    "jsonrpc": "2.0",
    "method": "prompts/get",
    "params": {
        "name": "setup-semantic-search",
        "arguments": {
            "query_text": "What is PostgreSQL?"
        }
    },
    "id": 2
}
```

### Implementation

Prompts are implemented in `internal/prompts/`:

- `design_schema.go`: Schema design workflow
- `diagnose_query_issue.go`: Query diagnosis workflow
- `explore_database.go`: Database exploration workflow
- `registry.go`: Prompt registration and management
- `setup_semantic_search.go`: Semantic search workflow

Each prompt returns a `mcp.PromptResult` containing:

- `description`: Human-readable description of what the prompt will do
- `messages`: Array of message objects (role + content) sent to the LLM

### Custom Prompts

To add a new prompt:

1. Create a new file in `internal/prompts/`
2. Implement a function returning a `Prompt` struct
3. Define the prompt name, description, and arguments
4. Implement the handler function that returns a `mcp.PromptResult`
5. Register the prompt in `registry.go`

**Example**:

```go
func MyCustomPrompt() Prompt {
    return Prompt{
        Definition: mcp.Prompt{
            Name:        "my-custom-prompt",
            Description: "Does something useful",
            Arguments: []mcp.PromptArgument{
                {
                    Name:        "input",
                    Description: "The input value",
                    Required:    true,
                },
            },
        },
        Handler: func(args map[string]string) mcp.PromptResult {
            return mcp.PromptResult{
                Description: "Custom workflow",
                Messages: []mcp.PromptMessage{
                    {
                        Role: "user",
                        Content: mcp.ContentItem{
                            Type: "text",
                            Text: "Your workflow instructions here",
                        },
                    },
                },
            }
        },
    }
}
```

## Troubleshooting

### Prompt Not Found

**Error**: "Prompt 'prompt-name' not found"

**Solutions**:

- Verify the prompt name using `/prompts` (CLI) or the prompt dropdown (Web
UI)
- Check for typos in the prompt name
- Ensure the server is running the latest version

### Missing Required Argument

**Error**: "Missing required argument: argument_name"

**Solutions**:

- Check the prompt's required arguments using `/prompts`
- Provide all required arguments in the command
- Use quotes around values with spaces

### Invalid Argument Format

**Error**: "Invalid argument format: ... (expected key=value)"

**Solutions**:

- Use `key=value` format for all arguments
- Quote values containing spaces: `key="value with spaces"`
- Don't use spaces around the `=` sign

### Rate Limit Exceeded

**Error**: "Rate limit reached for ..."

**Solutions**:

- Wait 60 seconds before retrying
- Use more targeted queries with WHERE clauses
- Reduce `max_output_tokens` in similarity_search
- Use `limit` parameter in queries
- Conversation history is automatically compacted to help prevent this

## See Also

- [Available Tools](tools.md) - MCP tools for database interaction
- [Available Resources](resources.md) - MCP resources for system information
- [Configuration](../guide/configuration.md) - Server configuration options
- [Using the CLI Client](../guide/cli-client.md) - Command-line interface guide
- [Building Chat Clients](../developers/building-chat-clients.md) - Web interface development
