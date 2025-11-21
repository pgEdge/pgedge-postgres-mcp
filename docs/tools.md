# MCP Tools

The pgEdge MCP Server provides five tools that enable SQL database interaction, advanced semantic search, embedding generation, and resource reading.

## Available Tools

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

### get_schema_info

**PRIMARY TOOL for discovering database tables and schema information.** Retrieves detailed database schema information including tables, views, columns, data types, constraints (primary/foreign keys), and comments from pg_description. **ALWAYS use this tool first when you need to know what tables exist in the database.**

**Input** (optional):
```json
{
  "schema_name": "public"
}
```

**Output**:
```
Database Schema Information:
============================

public.users (TABLE)
  Description: User accounts and authentication
  Columns:
    - id: bigint
    - username: character varying(255)
      Description: Unique username for login
    - created_at: timestamp with time zone (nullable)
      Description: Account creation timestamp
    ...
```

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

Enable in `pgedge-pg-mcp-svr.yaml`:

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

See [Configuration Guide](configuration.md#embedding-generation-logging) for details.

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
- `pg://database/schema` - Lightweight overview of all database tables (names and owners)

See [Resources](resources.md) for detailed information.

