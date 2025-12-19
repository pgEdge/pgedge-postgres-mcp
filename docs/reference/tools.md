# MCP Tools

The MCP server provides various tools that enable SQL database
interaction, advanced semantic search, embedding generation, resource reading,
and more.

## Disabling Tools

Individual tools can be disabled via configuration to restrict what the LLM
can access. See [Enabling/Disabling Built-in Features](../guide/feature_config.md)
for details.

When a tool is disabled:

- It is not advertised to the LLM in the `tools/list` response
- Attempts to execute it return an error message

**Note**: The `read_resource` tool is always enabled as it's required for
listing resources.

## Available Tools

### execute_explain

Executes EXPLAIN ANALYZE on a SQL query to analyze query performance and
execution plans.

**Prerequisites**:

- Query must be a SELECT statement
- Queries are executed in read-only transactions

**Parameters**:

- `query` (required): The SELECT query to analyze
- `analyze` (optional): Run EXPLAIN ANALYZE for actual timing (default: true)
- `buffers` (optional): Include buffer usage statistics (default: true)
- `format` (optional): Output format - "text" or "json" (default: "text")

**Input Example**:

```json
{
  "query": "SELECT * FROM users WHERE email LIKE '%@example.com'",
  "analyze": true,
  "buffers": true,
  "format": "text"
}
```

**Output**:

```
EXPLAIN ANALYZE Results
=======================

Query: SELECT * FROM users WHERE email LIKE '%@example.com'

Execution Plan:
---------------
Seq Scan on users  (cost=0.00..25.00 rows=6 width=540)
                   (actual time=0.015..0.089 rows=12 loops=1)
  Filter: (email ~~ '%@example.com'::text)
  Rows Removed by Filter: 988
  Buffers: shared hit=15
Planning Time: 0.085 ms
Execution Time: 0.112 ms

Analysis:
---------
- Sequential scan detected on 'users' table
- Consider adding an index if this query runs frequently
- Filter removed 988 rows - WHERE clause selectivity is low
```

**Use Cases**:

- **Query Optimization**: Identify slow queries and bottlenecks
- **Index Planning**: Determine which indexes would improve performance
- **Understanding Execution**: Learn how PostgreSQL processes your queries
- **Debugging**: Diagnose why queries are slower than expected

**Security**: Queries are executed in read-only transactions. Only SELECT
statements are allowed.

### generate_embedding

Generate vector embeddings from text using OpenAI, Voyage AI (cloud), or Ollama (local). Enables converting natural language queries into embedding vectors for semantic search.

**Prerequisites**:

- Embedding generation must be enabled in server configuration
- For OpenAI: Valid API key must be configured
- For Voyage AI: Valid API key must be configured
- For Ollama: Ollama must be running with an embedding model installed

**Input**:

```json
{
  "text": "What is vector similarity search?"
}
```

**Parameters**:

- `text` (required): The text to convert into an embedding vector

**Output**:

```
Generated Embedding:
Provider: ollama
Model: nomic-embed-text
Dimensions: 768
Text Length: 33 characters

Embedding Vector (first 10 dimensions):
[0.023, -0.145, 0.089, 0.234, -0.067, 0.178, -0.112, 0.045, 0.198, -0.156, ...]

Full embedding vector returned with 768 dimensions.
```

**Use Cases**:

- **Semantic Search**: Generate query embeddings for vector similarity search
- **RAG Systems**: Convert questions into embeddings to find relevant context
- **Document Clustering**: Generate embeddings for grouping similar documents
- **Content Recommendation**: Create embeddings for matching similar content

**Configuration**:

Enable in your server configuration file:

```yaml
embedding:
  enabled: true
  provider: "openai"  # Options: "openai", "voyage", or "ollama"
  model: "text-embedding-3-small"
  openai_api_key: ""  # Set via OPENAI_API_KEY environment variable
```

**Supported Providers and Models**:

OpenAI (Cloud):

- `text-embedding-3-small`: 1536 dimensions (recommended, compatible with most databases)
- `text-embedding-3-large`: 3072 dimensions (higher quality)
- `text-embedding-ada-002`: 1536 dimensions (legacy)

Voyage AI (Cloud):

- `voyage-3`: 1024 dimensions (recommended)
- `voyage-3-lite`: 512 dimensions (cost-effective)
- `voyage-2`: 1024 dimensions
- `voyage-2-lite`: 1024 dimensions

Ollama (Local):

- `nomic-embed-text`: 768 dimensions (recommended)
- `mxbai-embed-large`: 1024 dimensions
- `all-minilm`: 384 dimensions

**Example Usage**:

```json
{
  "text": "What is vector similarity search?"
}
```

Returns an embedding vector that can be used for semantic search operations or stored in a pgvector column.

**Error Handling**:

- Returns error if embedding generation is not enabled in configuration
- Returns error if embedding provider is not accessible (Ollama not running, invalid API key)
- Returns error if text is empty
- Returns error if API request fails (rate limits, network issues)

**Debugging**:

Enable logging to debug embedding API calls:

```bash
export PGEDGE_LLM_LOG_LEVEL="info"  # or "debug" or "trace"
```

See the [documentation](../guide/configuration.md) for configuration details.

### get_schema_info

**PRIMARY TOOL for discovering database tables and schema information.** Retrieves
detailed database schema information including tables, views, columns, data
types, constraints, indexes, identity columns, default values, and comments
from pg_description. **ALWAYS use this tool first when you need to know what
tables exist in the database.**

**Parameters**:

- `schema_name` (optional): Filter to a specific schema (e.g., `"public"`)
- `table_name` (optional): Filter to a specific table. Requires `schema_name`
  to also be provided
- `vector_tables_only` (optional): If `true`, only return tables with pgvector
  columns. Reduces output significantly (default: `false`)
- `compact` (optional): If `true`, return table names only without column
  details. Use for quick overview (default: `false`)

**Output Format**:

Results are returned in TSV (tab-separated values) format for token efficiency.
The columns are:

- `schema` - Schema name
- `table` - Table name
- `type` - TABLE, VIEW, or MATERIALIZED VIEW
- `table_desc` - Table description from pg_description
- `column` - Column name
- `data_type` - PostgreSQL data type
- `nullable` - YES or NO
- `col_desc` - Column description
- `is_pk` - true if part of primary key
- `is_unique` - true if has unique constraint (excluding PK)
- `fk_ref` - Foreign key reference in format "schema.table.column" if FK
- `is_indexed` - true if column is part of any index
- `identity` - "a" for GENERATED ALWAYS, "d" for BY DEFAULT, empty otherwise
- `default` - Default value expression if any
- `is_vector` - true if pgvector column
- `vector_dims` - Number of dimensions for vector columns (0 if not vector)

**Auto-Summary Mode**:

When called without filters on databases with >10 tables, automatically returns
a compact summary showing table counts per schema and suggested next calls.
This prevents overwhelming token usage on large databases.

**Input Examples**:

Get all schema info (returns summary if >10 tables):

```json
{}
```

Get details for a specific schema:

```json
{
  "schema_name": "public"
}
```

Get columns for a specific table:

```json
{
  "schema_name": "public",
  "table_name": "users"
}
```

Find tables with vector columns:

```json
{
  "vector_tables_only": true
}
```

Quick table list (no column details):

```json
{
  "compact": true
}
```

**Output Example** (single table):

```
Database: postgres://user@localhost/mydb

schema	table	type	table_desc	column	data_type	nullable	col_desc	is_pk	is_unique	fk_ref	is_indexed	identity	default	is_vector	vector_dims
public	users	TABLE	User accounts	id	bigint	NO	Primary key	true	false		true	a		false	0
public	users	TABLE	User accounts	email	text	NO	User email	false	true		true			false	0
public	users	TABLE	User accounts	created_at	timestamptz	YES		false	false		false		now()	false	0
```

**Use Cases**:

- **Discover Tables**: Find what tables exist before querying
- **Understand Relationships**: Use `fk_ref` to understand table joins
- **Query Optimization**: Check `is_indexed` to write efficient queries
- **Vector Search Setup**: Use `vector_tables_only` to find tables for
  `similarity_search`

### query_database

Executes a SQL query against the PostgreSQL database.

**Input Examples**:

Basic query:

```json
{
  "query": "SELECT * FROM users WHERE created_at >= NOW() - INTERVAL '7 days' ORDER BY created_at DESC"
}
```

**Output**:

```
SQL Query: SELECT * FROM users WHERE created_at >= NOW() - INTERVAL '7 days' ORDER BY created_at DESC

Results (15 rows):
[
  {
    "id": 123,
    "username": "john_doe",
    "created_at": "2024-10-25T14:30:00Z",
    ...
  },
  ...
]
```

**Note**: When using MCP clients like Claude Desktop, the client's LLM can translate natural language into SQL queries that are then executed by this server.

**Security**: All queries are executed in read-only transactions using `SET TRANSACTION READ ONLY`, preventing INSERT, UPDATE, DELETE, and other data modifications. Write operations will fail with "cannot execute ... in a read-only transaction".

### read_resource

Reads MCP resources by their URI. Provides access to system information and statistics.

**Input Examples**:

List all available resources:

```json
{
  "list": true
}
```

Read a specific resource:

```json
{
  "uri": "pg://system_info"
}
```

**Available Resource URIs**:

- `pg://system_info` - PostgreSQL version, OS, and build architecture

See [Resources](resources.md) for detailed information.

### search_knowledgebase

Search the pre-built documentation knowledgebase for relevant information about
PostgreSQL, pgEdge products, and other documented technologies.

**Prerequisites**:

- Knowledgebase must be enabled in server configuration
- A knowledgebase database must be built using the `kb-builder` tool

**Parameters**:

- `query` (required unless `list_products` is true): Natural language search
  query
- `project_names` (optional): Array of project/product names to filter by
  (e.g., `["PostgreSQL"]`, `["pgEdge", "pgAdmin"]`)
- `project_versions` (optional): Array of project/product versions to filter
  by (e.g., `["17"]`, `["16", "17"]`)
- `top_n` (optional): Number of results to return (default: 5, max: 20)
- `list_products` (optional): If true, returns only the list of available
  products and versions in the knowledgebase (ignores other parameters)

**Input Examples**:

List available products:

```json
{
  "list_products": true
}
```

Search with single product filter:

```json
{
  "query": "PostgreSQL window functions",
  "project_names": ["PostgreSQL"],
  "top_n": 10
}
```

Search across multiple products and versions:

```json
{
  "query": "backup and restore",
  "project_names": ["PostgreSQL", "pgEdge"],
  "project_versions": ["16", "17"]
}
```

**Output** (list_products):

```
Available Products in Knowledgebase
==================================================

Product: PostgreSQL
  - Version 16 (1245 chunks)
  - Version 17 (1312 chunks)

Product: pgEdge
  - Version 5.0 (423 chunks)

==================================================
Total: 2980 chunks across all products
```

**Output** (search):

```
Knowledgebase Search Results: "PostgreSQL window functions"
Filter - Projects: PostgreSQL; Versions: 17
================================================================================

Found 5 relevant chunks:

Result 1/5
Project: PostgreSQL 17
Title: SQL Functions
Section: Window Functions
Similarity: 0.892

Window functions provide the ability to perform calculations across sets
of rows that are related to the current query row. Unlike regular aggregate
functions, window functions do not cause rows to become grouped into a
single output row...

--------------------------------------------------------------------------------

Result 2/5
Project: PostgreSQL 17
Title: Tutorial
Section: Window Functions
Similarity: 0.856

A window function performs a calculation across a set of table rows that
are somehow related to the current row. This is comparable to the type of
calculation that can be done with an aggregate function...

--------------------------------------------------------------------------------

================================================================================
Total: 5 results
```

**Use Cases**:

- **PostgreSQL Reference**: Find syntax and usage for SQL features
- **Product Documentation**: Search pgEdge or other product documentation
- **Best Practices**: Find recommendations and guidelines
- **Troubleshooting**: Search for error messages and solutions

**Configuration**:

Enable in your server configuration file:

```yaml
knowledgebase:
  enabled: true
  database_path: "/path/to/knowledgebase.db"
```

See [Knowledgebase Configuration](../advanced/knowledgebase.md) for details on
building and configuring the documentation knowledgebase.

### similarity_search

**Advanced hybrid search** combining vector similarity with BM25 lexical matching and MMR diversity filtering. This tool is ideal for searching through large documents like Wikipedia articles without requiring users to pre-chunk their data.

**IMPORTANT**: If you don't know the exact table name, call `get_schema_info` first to discover available tables with vector columns (use `vector_tables_only=true` to reduce output).

**How It Works**:

1. **Auto-Discovery**: Automatically detects pgvector columns in your table and corresponding text columns
2. **Smart Weighting**: Analyzes column names, descriptions, and sample data to identify title vs content columns, weighting content more heavily (70% vs 30%)
3. **Query Embedding**: Generates embedding from your search query using the configured provider
4. **Vector Search**: Performs weighted semantic search across all vector columns
5. **Intelligent Chunking**: Breaks retrieved documents into overlapping chunks (default: 100 tokens per chunk, 25 token overlap)
6. **BM25 Re-ranking**: Scores chunks using BM25 lexical matching for precision
7. **MMR Diversity**: Applies Maximal Marginal Relevance to avoid returning too many chunks from the same document
8. **Token Budget**: Returns as many relevant chunks as possible within the token limit (default: 1000 tokens)

**Prerequisites**:

- Table must have at least one pgvector column
- Embedding generation must be enabled in server configuration
- Corresponding text columns must exist (e.g., `title` for `title_embedding`)

**Parameters**:

- `table_name` (required): Table to search (can include schema: `'schema.table'`)
- `query_text` (required): Natural language search query
- `top_n` (optional): Number of rows from vector search (default: 10)
- `chunk_size_tokens` (optional): Maximum tokens per chunk (default: 100)
- `lambda` (optional): MMR diversity parameter - 0.0=max diversity, 1.0=max relevance (default: 0.6)
- `max_output_tokens` (optional): Maximum total tokens to return (default: 1000)
- `distance_metric` (optional): `'cosine'`, `'l2'`, or `'inner_product'` (default: `'cosine'`)

**Example** - Wikipedia Search:

```json
{
  "table_name": "wikipedia_articles",
  "query_text": "How does PostgreSQL handle vector similarity search?",
  "top_n": 10,
  "chunk_size_tokens": 150,
  "lambda": 0.6,
  "max_output_tokens": 3000
}
```

**Example Response**:

{% raw %}
```
Similarity Search Results: "How does PostgreSQL handle vector similarity search?"
================================================================================

Configuration:
  - Vector Search: Top 10 rows
  - Chunking: 150 tokens per chunk, 38 token overlap
  - Diversity: λ=0.60 (60% relevance, 40% diversity)
  - Distance Metric: cosine
  - Column Weights:
      title (30.0%) [title]
      content (70.0%) [content]

Result 1/5
Source: wikipedia_articles.content (vector search rank: #1, chunk: 1)
Relevance Score: 8.452
Tokens: ~145

PostgreSQL supports vector similarity search through the pgvector extension.
This extension adds a new data type called 'vector' that can store embedding
vectors of any dimension. The extension provides three distance operators:
<=> for cosine distance, <-> for L2 (Euclidean) distance, and <#> for inner
product (negative). To perform similarity search, you first generate embeddings
for your documents using a model like OpenAI's text-embedding-ada-002...

--------------------------------------------------------------------------------

Result 2/5
Source: wikipedia_articles.content (vector search rank: #2, chunk: 2)
Relevance Score: 7.921
Tokens: ~138

...indexes can dramatically improve query performance. pgvector supports two
index types: IVFFlat and HNSW. IVFFlat uses inverted file indexes with product
quantization, which divides the vector space into lists and searches only the
nearest lists. HNSW (Hierarchical Navigable Small World) creates a multi-layer
graph structure that enables fast approximate nearest neighbor search...

--------------------------------------------------------------------------------

Total: 5 chunks, ~687 tokens
```
{% endraw %}

**Key Features**:

- **No Pre-Chunking Required**: Users don't need to chunk their data in advance - the tool handles it at query time
- **Smart Column Detection**: Automatically identifies title vs content columns and weights them appropriately
- **Hybrid Search**: Combines semantic (vector) and lexical (BM25) matching for better results
- **Diversity Filtering**: Prevents returning redundant chunks from the same document
- **Token-Aware**: Respects token limits to avoid API rate limit issues

**Use Cases**:

- **Knowledge Base Search**: Find relevant documentation chunks for RAG systems
- **Wikipedia/Encyclopedia Search**: Search through large articles efficiently
- **Customer Support**: Search through support articles and FAQs
- **Research**: Find relevant sections in academic papers or reports
- **Code Search**: Find relevant code snippets (if using code embeddings)

**Comparison with Old Tools**:

Unlike the previous `semantic_search` and `search_similar` tools, this new implementation:

- Automatically chunks large documents at query time
- Uses BM25 for improved lexical matching
- Applies MMR diversity to avoid redundancy
- Intelligently weights title vs content columns
- Manages token budgets automatically
- Works with any table structure (no pre-chunking required)

**Performance Tips**:

- Create indexes on vector columns for faster search:
  ```sql
  CREATE INDEX ON wikipedia_articles USING ivfflat (content_embedding vector_cosine_ops);
  ```
- Adjust `top_n` based on your use case (more rows = better recall but slower)
- Use higher `lambda` (0.7-0.8) for focused queries, lower (0.4-0.5) for exploratory search
- Adjust `chunk_size_tokens` based on your documents (smaller chunks for dense content)
