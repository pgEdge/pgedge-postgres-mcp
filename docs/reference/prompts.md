# Using MCP Prompts

Prompts are reusable workflow templates that guide LLMs through complex
multi-step processes. They provide structured guidance for common tasks like
setting up semantic search, exploring unfamiliar databases, or diagnosing
query issues. Prompts are designed to:

* ensure consistent workflows so important steps aren't missed.
* reduce back-and-forth by providing complete guidance upfront.
* encode domain expertise in reusable templates.
* accept arguments to customize workflows for specific contexts.

!!! note

    MCP Prompts are a feature introduced in the Model Context Protocol (MCP) version 2024-11-05. Unlike tools that execute specific actions, prompts provide pre-written instructions that guide the LLM through a systematic workflow. Think of them as expert playbooks that ensure consistent,
    thorough analysis.

The following prompts are implemented in `internal/prompts/`:

| File | Description |
|------|-------------|
| `design_schema.go` | Schema design workflow. |
| `diagnose_query_issue.go` | Query diagnosis workflow. |
| `explore_database.go` | Database exploration workflow. |
| `registry.go` | Prompt registration and management. |
| `setup_semantic_search.go` | Semantic search workflow. |


Each prompt returns a `mcp.PromptResult` object, containing:

| Field | Description |
|-------|-------------|
| `description` | Human-readable description of what the prompt will do. |
| `messages` | Array of message objects (role + content) sent to the LLM. |

!!! note

    Each individual prompt can be disabled to restrict what the LLM can access. When disabled:

        * it is not advertised to the LLM in the `prompts/list` response.
        * the prompt dropdown in the web UI will not show it.
        * attempts to use it via `/prompt` return an error.

    See [Enabling/Disabling Built-in Features](../guide/feature_config.md) for details.


## Using Prompts to Manage Token and Rate Limits

Prompts are designed to optimize token usage and avoid rate limits; follow these best practices when working with prompts:

* Use prompts instead of freeform queries for complex workflows.
* Start with conservative token limits.
* Filter results with WHERE clauses when possible.
* Use `limit` parameter in queries.
* Avoid multiple large operations in one conversation turn.

Prompts are designed to help manage token usage and avoid rate limit errors by enforcing the following behaviors.

**Conversation History Compaction**

Both the CLI and Web clients automatically compact conversation history before each LLM call:

* The clients keep the first user message for context.
* The clients keep the last 10 messages.
* The clients prevent exponential token growth in multi-turn conversations.

**Prompt-Specific Token Guidance**

Each prompt includes token budget recommendations:

* The `diagnose-query-issue` prompt prioritizes quick checks before expensive operations.
* The `explore-database` prompt uses schema filtering and sample limits.
* The `setup-semantic-search` prompt limits similarity_search to 1000 tokens by default.


## Using Built-in Prompts

The MCP server provides the following prompts to guide the LLM through common database workflows.

### design-schema

`design-schema` guides the LLM through designing a PostgreSQL database schema based on user requirements. This prompt uses best practices, appropriate normalization levels based on your specifications, and recommends PostgreSQL extensions where beneficial. `design_schema`:

1. searches for relevant schema design best practices if a knowledge base is available.
2. analyzes data entities, relationships, and access patterns.
3. applies appropriate normalization based on use case (3NF+ for OLTP, denormalized for OLAP).
4. recommends PostgreSQL-specific types (UUID, TIMESTAMPTZ, NUMERIC, JSONB, TEXT, VECTOR).
5. suggests indexes based on query patterns.
6. proposes relevant PostgreSQL extensions.
7. produces complete CREATE TABLE statements.

The prompt instructs the LLM to avoid the following common database design anti-patterns:

* Using SERIAL instead of IDENTITY for auto-increment.
* Generating UUIDs in the database when IDENTITY would suffice.
* Entity-Attribute-Value (EAV) patterns.
* Excessive nullable columns.
* Missing foreign key constraints.
* Inappropriate use of arrays for relationships.
* Over-indexing or under-indexing.
* Assuming extension names without verifying via knowledgebase.
* Over-engineering: adding tables/columns "just in case".
* Using advanced extensions (pgvector) when simpler ones (pg_trgm) suffice.

`design-schema` is useful if you are:

* Designing a new database schema from specific requirements.
* Getting schema recommendations for specific use cases (OLTP, OLAP, etc.).
* Learning Postgres best practices for data modeling.
* Generating `CREATE TABLE` statements with proper types and constraints.

The prompt recommends the following PostgreSQL data types for optimal performance and compatibility:

* Use `GENERATED ALWAYS AS IDENTITY` for auto-increment primary keys.
* Use `UUID` only when the application provides IDs or for distributed systems.
* Use `TIMESTAMPTZ` for timestamps (timezone-aware).
* Use `NUMERIC` for money/precision values.
* Use `JSONB` for flexible/semi-structured data.
* Use `TEXT` instead of VARCHAR (no performance difference).
* Use `VECTOR` for AI embeddings.

`design-schema` takes the following arguments:

| Name | Required | Description |
|------|----------|-------------|
| `requirements` | Required | Description of the application requirements and data needs. |
| `use_case` | Optional | Primary use case - `oltp`, `olap`, `hybrid`, or `general` (default: `general`). |
| `full_featured` | Optional | If `true`, design a comprehensive production-ready schema. If `false` (the default), design a minimal schema meeting only the stated requirements. |

The prompt applies normalization strategies based on your specified `use_case` argument:

* **OLTP**: Third Normal Form (3NF) or higher for data integrity.
* **OLAP**: Strategic denormalization for query performance.
* **Hybrid**: Balanced approach with materialized views.
* **General**: Context-appropriate normalization.

`design-schema` applies design strategies based on your `full_featured` argument:

- **Minimal (false, the default)**: Create only the tables and columns explicitly required.
  Does not add user accounts, audit logs, favorites, or other supporting
  tables unless requested. Prefers simpler solutions (pg_trgm over pgvector
  for basic text search).

- **Full-Featured (true)**: Creates a comprehensive, production-ready schema with
  supporting tables, audit logging, user preferences, and future-proofing
  considerations. Use this option when you want a complete application schema.

`design-schema` considers these extensions where appropriate:

* `vector`: For AI/ML embeddings and semantic search (note: extension name is `vector`, not `pgvector`).
* `postgis`: For geographic/spatial data.
* `pg_trgm`: For fuzzy text search.
* `pgcrypto`: For cryptographic functions.
* `ltree`: For hierarchical data.

The prompt instructs the LLM to verify ALL extension names via the knowledgebase before writing `CREATE EXTENSION` statements, as project names often differ from the actual extension names.

**Examples**

The following examples demonstrate how to use the `design-schema` prompt from both the Web UI and CLI.

**Using design-schema from the Web UI**

1. Click the brain icon next to the send button.
2. Select `design-schema` from the dropdown.
3. Enter your requirements description.
4. Optionally select a use case (`oltp`, `olap`, `hybrid`, `general`).
5. Optionally set `full_featured` to `true` for comprehensive schemas.
6. Click `Execute Prompt`.

**CLI Example - a schema for e-commerce**

In the following example, the prompt designs a schema for an e-commerce platform:

```bash
/prompt design-schema requirements="E-commerce platform with products, orders, and customers"
```

**CLI Example - a schema for OLAP workloads**

In the following example, the prompt designs a schema optimized for OLAP workloads:

```bash
/prompt design-schema requirements="Real-time analytics dashboard" use_case="olap"
```

**CLI Example - a production-ready schema**

In the following example, the prompt designs a comprehensive production-ready schema:

```bash
/prompt design-schema requirements="E-commerce platform" full_featured="true"
```

### diagnose-query-issue

diagnose-query-issue systematically diagnoses why queries are failing or returning unexpected results. `diagnose-query-issue`:

1. verifies which database you're connected to.
2. checks if a target table/schema exists.
3. inspects columns, types, and constraints.
4. verifies table has data.
5. systematically checks typical problems.
6. suggests fixes based on diagnosis.

The prompt guides the LLM through a comprehensive diagnostic checklist that checks for:

* a connection to an incorrect database.
* misspelled table names or incorrect schema references.
* missing vector columns required for similarity_search operations.
* disabled embedding generation configuration.
* empty result sets where queries execute successfully but return no matching data.
* exceeded API rate limits.
* insufficient database permissions.

`diagnose-query-issue` is useful if you are:

* Debugging "table not found" errors.
* Understanding why queries return no results.
* Verifying you're connected to the correct database.
* Troubleshooting permission issues.

`diagnose-query-issue` takes the following arguments:

| Name | Required | Description |
|------|----------|-------------|
| `issue_description` | Optional | A description of the problem (e.g., `table not found`, `no results`, `wrong database`). |

For fastest diagnosis, the prompt prioritizes the following issues:

1. Wrong database: Check system-info, switch if needed.
2. Table doesn't exist: Run `get_schema_info`.
3. No data in table: Sample with `limit=1`.
4. Semantic search in non-vector table: Check vector_tables_only.

**Examples**

**Using diagnose-query-issue from the Web UI**

1. Click the brain icon next to the send button.
2. Select `diagnose-query-issue` from the dropdown.
3. Optionally enter a description of the issue.
4. Click `Execute Prompt`.

**CLI Example - diagnosing a table not found error**

In the following example, the prompt diagnoses a `table not found` error:

```bash
/prompt diagnose-query-issue issue_description="table not found"
```

**CLI Example - general diagnosis**

In the following example, the prompt performs a general diagnosis without a specific issue description:

```bash
/prompt diagnose-query-issue
```

### explore-database

`explore-database` systematically explores an unfamiliar database to understand its structure, capabilities, and available data. `explore-database`:

1. identifies which database you're connected to.
2. retreives comprehensive schema information.
3. identifies table purposes and relationships.
4. detects pgvector columns, JSONB, and foreign keys.
5. optionally queries small samples from key tables.
6. provides findings and suggested next steps.

`explore-database` is useful if you are:

* Understanding a new database you're working with.
* Discovering what data is available.
* Identifying semantic search capabilities.
* Planning queries or analyses.

`explore-database` takes the following arguments:

| Name | Required | Description |
|------|----------|-------------|
| None | N/A | Fully automated workflow with no arguments. |

The prompt includes the following guidance to manage rate limits:

* Use `schema_name="public"` to reduce token usage.
* Use `limit=5` for sample queries.
* Cache results to avoid re-querying.
* Filter by schema for large databases.

The workflow stops early if any of the following conditions occur:

* No tables are found (wrong/empty database).
* Permission to the database is denied.
* Specific data sought but not found.

**Examples**

The following examples demonstrate how to use the `explore-database` prompt from both the Web UI and CLI.

**Using explore-database from the Web UI**

1. Click the brain icon next to the send button.
2. Select `explore-database` from the dropdown.
3. Click `Execute Prompt`.

**CLI Example - exploring database structure**

In the following example, the prompt explores the database structure:

```bash
/prompt explore-database
```

### setup-semantic-search

`setup-semantic-search` sets up semantic search using the [`similarity_search`](tools.md#similarity_search) tool. `setup-semantic-search`:

1. identifies tables with vector columns.
2. chooses the most appropriate table based on schema info.
3. runs [similarity_search](tools.md#similarity_search) with optimal parameters.
4. manages chunking and token budgets to avoid rate limits.

`setup-semantic-search` is useful if you are:

* Setting up semantic search for the first time.
* Finding relevant documentation chunks.
* Searching knowledge bases or Wikipedia-style articles.
* Implementing RAG (Retrieval-Augmented Generation) workflows.

`setup-semantic-search` takes the following arguments:

| Name | Required | Description |
|------|----------|-------------|
| `query_text` | Required | The natural language search query. |

The prompt instructs the LLM to use the following similarity_search parameters:

* `top_n`: 10 (balance between recall and token usage).
* `chunk_size_tokens`: 100 (manageable chunk size).
* `lambda`: 0.6 (balanced relevance vs diversity).
* `max_output_tokens`: 1000 (prevents rate limit issues).

The prompt includes specific guidance on managing token budgets to avoid rate limit errors and instructs the LLM to:

* Start with conservative token limits (1000 tokens).
* Use moderate chunking (100 tokens per chunk).
* Limit initial searches to top 10 results.
* Avoid multiple large searches in the same conversation turn.

**Examples**

The following examples demonstrate how to use the `setup-semantic-search` prompt from both the Web UI and CLI.

**Using setup-semantic-search from the Web UI**

1. Click the brain icon (Psychology icon) next to the send button.
2. Select `setup-semantic-search` from the dropdown.
3. Enter your query text in the query_text field.
4. Click `Execute Prompt`.

**CLI Example - setting up semantic search**

In the following example, the prompt sets up semantic search to find information about pgAgent:

```bash
/prompt setup-semantic-search query_text="What is pgAgent?"
```


## Using Prompts

The following sections explain how to execute prompts in different environments.

### CLI Client

Prompts are executed using the `/prompt` slash command:

**Syntax**

In the following example, the syntax shows how to execute prompts with arguments:

```bash
/prompt <prompt-name> [arg1=value1] [arg2=value2] ...
```

**Examples**

In the following examples, the commands demonstrate various prompt invocations:

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

**Quoted Arguments**

Arguments with spaces must be quoted.

In the following example, the query text is enclosed in double quotes:

```bash
/prompt setup-semantic-search query_text="How does PostgreSQL handle transactions?"
```

Both single and double quotes are supported.

In the following example, the query text is enclosed in single quotes:

```bash
/prompt setup-semantic-search query_text='What is pgAgent?'
```

**List Available Prompts**

In the following example, the command lists all available prompts with their descriptions and arguments:

```bash
/prompts
```

### Web UI Client

The web interface provides a graphical way to execute prompts:

**Access Prompts**

1. Look for the brain icon (Psychology icon) in the input area, between the save button and send button.
2. The icon only appears if prompts are available from the server.

**Execute a Prompt**

1. Click the brain icon to open the prompt popover.
2. Select a prompt from the dropdown menu.
3. Read the prompt description.
4. Fill in any required arguments.
5. Optionally fill in optional arguments.
6. Click "Execute Prompt".

**Features**

* **Validation**: Required arguments are validated before execution.
* **Help Text**: Each argument shows its description as helper text.
* **Auto-Reset**: Form resets after successful execution.
* **Disabled During Execution**: Form is disabled while the prompt runs.

**Visual Indicators**

* Required arguments are marked with an asterisk (*).
* Invalid required fields show an error message.
* Execute button is disabled while running.


## Creating and Managing Custom Prompts

This section provides technical information about how prompts are implemented and exposed through the MCP protocol, including protocol methods, implementation structure, and guidance for creating your own custom prompts.

Prompts are exposed via the MCP protocol's `prompts/list` and `prompts/get` methods:

**List Prompts** (`prompts/list`):

In the following example, the JSON-RPC request lists all available prompts:

```json
{
    "jsonrpc": "2.0",
    "method": "prompts/list",
    "id": 1
}
```

**Get Prompt** (`prompts/get`):

In the following example, the JSON-RPC request retrieves a specific prompt with arguments:

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

### Creating a Custom Prompt

To add a custom prompt:

1. Create a new file in `internal/prompts/`.
2. Implement a function that returns a `Prompt` structure.
3. Define the prompt name, description, and arguments.
4. Implement the handler function that returns a `mcp.PromptResult`.
5. Register the prompt in `registry.go`.

**Example**

In the following example, the Go code defines a custom prompt:

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