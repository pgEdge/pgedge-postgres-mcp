# Configuring and Using Knowledgebase Search

The `search_knowledgebase` tool provides semantic search over pre-built
documentation databases, allowing you to search PostgreSQL documentation,
pgEdge product documentation, and other technical resources.

Unlike the `similarity_search` which searches your own data in PostgreSQL, the
`search_knowledgebase` tool searches curated documentation that has been
pre-processed and indexed for efficient semantic retrieval.

Use `search_knowledgebase` when you need information about:

- PostgreSQL features, syntax, and functions.
- pgEdge products and capabilities.
- Other documented technologies included in the Knowledgebase.

Use `similarity_search` when you need to search your own data stored in
PostgreSQL tables.

**Best Practices**

* **Start broad**: Begin with general queries, then refine based on results.
* **Use filters**: Add project/version filters when you know what you're
   looking for.
* **Check multiple results**: Review several results for comprehensive
   information.
* **Combine with other tools**: Use with `query_database` to apply
   documentation knowledge to actual queries.

!!! Limitations

    - Results are limited to pre-built documentation.
    - Database must be periodically rebuilt to include new documentation.
    - Requires storage space for the knowledgebase database file.
    - Search quality depends on embedding provider consistency.


## Configuring Knowledgebase Search

To enable Knowledgebase search, add the following code snippet to your server configuration:

```yaml
knowledgebase:
    enabled: true
    database_path: "./pgedge-nla-kb.db"
    embedding_provider: "voyage"  # or "openai", "ollama"
    embedding_model: "voyage-3"

    # API keys (independent from embedding and LLM sections)
    # Option 1: API key file (RECOMMENDED)
    embedding_voyage_api_key_file: "~/.voyage-api-key"
    # embedding_openai_api_key_file: "~/.openai-api-key"

    # Option 2: Environment variables
    # PGEDGE_KB_VOYAGE_API_KEY or VOYAGE_API_KEY
    # PGEDGE_KB_OPENAI_API_KEY or OPENAI_API_KEY

    # Option 3: Direct config (NOT RECOMMENDED)
    # embedding_voyage_api_key: ""
    # embedding_openai_api_key: ""
```

**IMPORTANT:** The Knowledgebase embedding configuration is **completely
independent** from the `embedding` and `llm` sections. This allows you to:

- Use different embedding providers for semantic search vs. the
    `generate_embeddings` tool
- Use different API keys for Knowledgebase search
- Configure each section separately via environment variables
    (`PGEDGE_KB_*` prefix for Knowledgebase)

**Requirements:**

- A pre-built Knowledgebase database file (`.db` file).
- Embedding provider configured for Knowledgebase search.
- Same embedding provider and model used to build the database.

**See also:**

- [Server Configuration Example](../reference/config-examples/server.md) - Complete server configuration with Knowledgebase section.
- [KB Builder Configuration](../reference/config-examples/kb-builder.md) - Building the
    knowledgebase database.


## Using the Tool

The `search_knowledgebase` tool supports several search patterns to help you find relevant documentation.

### Basic Search

The simplest way to search is with just a query string.

```
Tool: search_knowledgebase
Args:
  query: "PostgreSQL window functions"
```

### Performing a Filtered Search

You can narrow your search results by filtering on project name or version.

Search within a specific project:

```
Tool: search_knowledgebase
Args:
  query: "replication setup"
  project_name: "pgEdge"
```

Search a specific version:

```
Tool: search_knowledgebase
Args:
  query: "JSON functions"
  project_name: "PostgreSQL"
  project_version: "17"
```

### Adjusting the Result Count

You can control how many results are returned.

```
Tool: search_knowledgebase
Args:
  query: "authentication methods"
  top_n: 10
```

Default is 5 results, maximum is 20.

**Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Natural language search query |
| `project_name` | string | no | Filter by project name |
| `project_version` | string | no | Filter by project version |
| `top_n` | integer | no | Number of results (default: 5, max: 20) |

**Output Format**

Results include:

- **Text**: The relevant documentation chunk.
- **Title**: Document title.
- **Section**: Section heading within the document.
- **Project**: Project name and version.
- **Similarity**: Relevance score (0-1, higher is more relevant).

## Examples

The following examples demonstrate common use cases for Knowledgebase search.

### Example 1: General Query

In the following example, the `search_knowledgebase` tool uses a general query to find documentation about composite indexes in PostgreSQL.

```
Query: "How do I create a composite index in PostgreSQL?"

Results:
- PostgreSQL 17 documentation chunk on CREATE INDEX
- Example of composite index syntax
- Performance considerations
```

### Example 2: Version-Specific Query

In the following example, the `search_knowledgebase` tool uses project and version filters to find documentation specific to PostgreSQL 17.

```
Query: "MERGE statement"
Project: PostgreSQL
Version: 17

Results:
- PostgreSQL 17 MERGE statement documentation
- Syntax and examples
- Comparison with INSERT...ON CONFLICT
```

### Example 3: Product-Specific Query

In the following example, the `search_knowledgebase` tool uses a project filter to find pgEdge-specific documentation about multi-master replication.

```
Query: "multi-master replication"
Project: pgEdge

Results:
- pgEdge replication architecture
- Configuration steps
- Conflict resolution
```

## Building a Knowledgebase

Knowledgebase databases are built using the `kb-builder` tool. This is an
internal tool for project developers - contact your administrator if you need
a custom Knowledgebase built.

The standard Knowledgebase includes:

- PostgreSQL official documentation (multiple versions).
- pgEdge product documentation.
- Related tools and extensions.

## See Also

- [Server Configuration Example](../reference/config-examples/server.md) - Complete server configuration with a knowledgebase section.
- [KB Builder Configuration](../reference/config-examples/kb-builder.md) - Building the
    Knowledgebase database.
- [Available Tools](../reference/tools.md) - Overview of all MCP tools.
