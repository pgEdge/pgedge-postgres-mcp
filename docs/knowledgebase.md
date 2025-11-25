# knowledgebase search

The `search_knowledgebase` tool provides semantic search over pre-built
documentation databases, allowing you to search PostgreSQL documentation,
pgEdge product documentation, and other technical resources.

## overview

Unlike `similarity_search` which searches your own data in PostgreSQL, the
`search_knowledgebase` tool searches curated documentation that has been
pre-processed and indexed for efficient semantic retrieval.

## when to use

Use `search_knowledgebase` when you need information about:

- PostgreSQL features, syntax, and functions
- pgEdge products and capabilities
- Other documented technologies included in the knowledgebase

Use `similarity_search` when you need to search your own data stored in
PostgreSQL tables.

## configuration

To enable knowledgebase search, add to your server configuration:

```yaml
knowledgebase:
    enabled: true
    database_path: "./pgedge-mcp-kb.db"
    embedding_provider: "openai"  # or "voyage", "ollama"
    embedding_model: "text-embedding-3-small"
    embedding_openai_api_key_file: "~/.openai-api-key"
```

**Note:** The knowledgebase embedding configuration is independent of the
`generate_embedding` tool configuration. You can use different providers for
each.

**Requirements:**

- A pre-built knowledgebase database file (`.db` file)
- Embedding provider configured for knowledgebase search
- Same embedding provider and model used to build the database

**See also:**

- [Server Configuration Example](config-example.md) - Complete server config
    with knowledgebase section
- [KB Builder Configuration](kb-builder-config-example.md) - Building the
    knowledgebase database

## usage

### basic search

```
Tool: search_knowledgebase
Args:
  query: "PostgreSQL window functions"
```

### filtered search

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

### adjust result count

```
Tool: search_knowledgebase
Args:
  query: "authentication methods"
  top_n: 10
```

Default is 5 results, maximum is 20.

## parameters

| parameter | type | required | description |
|-----------|------|----------|-------------|
| `query` | string | yes | Natural language search query |
| `project_name` | string | no | Filter by project name |
| `project_version` | string | no | Filter by project version |
| `top_n` | integer | no | Number of results (default: 5, max: 20) |

## output format

Results include:

- **Text**: The relevant documentation chunk
- **Title**: Document title
- **Section**: Section heading within the document
- **Project**: Project name and version
- **Similarity**: Relevance score (0-1, higher is more relevant)

## examples

### example 1: general query

```
Query: "How do I create a composite index in PostgreSQL?"

Results:
- PostgreSQL 17 documentation chunk on CREATE INDEX
- Example of composite index syntax
- Performance considerations
```

### example 2: version-specific query

```
Query: "MERGE statement"
Project: PostgreSQL
Version: 17

Results:
- PostgreSQL 17 MERGE statement documentation
- Syntax and examples
- Comparison with INSERT...ON CONFLICT
```

### example 3: product-specific query

```
Query: "multi-master replication"
Project: pgEdge

Results:
- pgEdge replication architecture
- Configuration steps
- Conflict resolution
```

## building a knowledgebase

Knowledgebase databases are built using the `kb-builder` tool. This is an
internal tool for project developers - contact your administrator if you need
a custom knowledgebase built.

The standard knowledgebase includes:

- PostgreSQL official documentation (multiple versions)
- pgEdge product documentation
- Related tools and extensions

## troubleshooting

### no results found

**Cause**: Query may be too specific or use terminology not in the
documentation.

**Solution**: Try broader search terms or rephrase the query.

### wrong project results

**Cause**: Not filtering by project name.

**Solution**: Add `project_name` parameter to filter results.

### embedding provider mismatch

**Cause**: Server embedding provider differs from the one used to build the
database.

**Solution**: Configure the server to use the same embedding provider. The
database contains embeddings from multiple providers - the server will
automatically use the one that matches its configuration.

### knowledgebase not available

**Cause**: Knowledgebase not enabled in configuration or database file missing.

**Solution**: Check server configuration and verify `database_path` points to a
valid knowledgebase database file.

## comparison with similarity_search

| feature | search_knowledgebase | similarity_search |
|---------|---------------------|-------------------|
| **data source** | pre-built documentation | user's postgresql tables |
| **use case** | technical documentation | user's own data |
| **setup** | requires kb database | requires vector columns |
| **updates** | static (rebuild needed) | dynamic (live data) |
| **scope** | curated content | any table data |

## best practices

1. **Start broad**: Begin with general queries, then refine based on results
2. **Use filters**: Add project/version filters when you know what you're
   looking for
3. **Check multiple results**: Review several results for comprehensive
   information
4. **Combine with other tools**: Use with `query_database` to apply
   documentation knowledge to actual queries

## limitations

- Results limited to pre-built documentation
- Database must be periodically rebuilt to include new documentation
- Requires storage space for the knowledgebase database file
- Search quality depends on embedding provider consistency

## see also

- [Server Configuration Example](config-example.md) - Complete server config
    with knowledgebase section
- [KB Builder Configuration](kb-builder-config-example.md) - Building the
    knowledgebase database
- [Available Tools](tools.md) - Overview of all MCP tools
