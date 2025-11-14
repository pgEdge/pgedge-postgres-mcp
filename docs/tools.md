# MCP Tools

The pgEdge MCP Server provides six tools that enable SQL database interaction, semantic search, embedding generation, and resource reading.

## Smart Tool Filtering

The server uses **smart tool filtering** to optimize token usage and improve user experience:

- **Without database connection**: Only 2 stateless tools are shown (`read_resource`, `generate_embedding`)
- **With database connection**: All 6 tools are available (adds `query_database`, `get_schema_info`, `semantic_search`, `search_similar`)

This dynamic tool list reduces token usage when no database is connected, helping you stay within API rate limits.

> **Note:** Database connections are now configured at server startup via environment variables (PGEDGE_DB_* or PG*) or command-line flags, not via tools.

## Available Tools

### query_database

Executes a SQL query against the PostgreSQL database. Supports dynamic connection strings to query different databases.

**IMPORTANT**: Using `AT postgres://...` or `SET DEFAULT DATABASE` for temporary connections does NOT modify saved connections - these are session-only changes.

**Input Examples**:

Basic query:
```json
{
  "query": "SELECT * FROM users WHERE created_at >= NOW() - INTERVAL '7 days' ORDER BY created_at DESC"
}
```

Query with temporary connection:
```json
{
  "query": "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AT postgres://localhost:5433/other_db"
}
```

Set new default connection:
```json
{
  "query": "SET DEFAULT DATABASE postgres://localhost/analytics"
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

Retrieves database schema information including tables, views, columns, data types, and comments from pg_description.

**Input** (optional):
```json
{
  "schema_name": "public"
}
```

**For semantic search** (dramatically reduces output):
```json
{
  "vector_tables_only": true
}
```

This filters to only show tables with vector columns, reducing token usage by showing only relevant tables for semantic search operations.

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

### search_similar

**RECOMMENDED**: Simplified semantic search tool that automatically discovers vector tables and generates embeddings - just provide your search text!

This is the easiest way to perform semantic search. It automatically:

- Discovers tables with vector columns from database metadata
- Generates embeddings from your natural language query
- Executes the semantic search and returns results

**Parameters**:

- `text_query` (required): Natural language search query
- `top_k` (optional): Number of results to return (default: 3)

**Example**:

```json
{
    "text_query": "What is PostgreSQL?",
    "top_k": 5
}
```

**Response**:

```
Auto-discovered vector table: public.wikipedia_articles (column: embedding, dimensions: 768)

Semantic Search Results:
Table: public.wikipedia_articles
Vector Column: embedding (dimensions: 768)
Distance Metric: Cosine Distance
Top K: 5

Results (5 rows):
[
    {
        "id": 123,
        "title": "PostgreSQL",
        "content": "PostgreSQL is an open-source relational database...",
        "distance": 0.123
    },
    ...
]
```

**Requirements**:

- Database connection must be established
- Embedding generation must be enabled in server configuration
- Database must contain at least one table with a pgvector column

**Use Cases**:

- Quick semantic searches without knowing table structure
- Interactive queries where you just want to search by text
- Chatbots and conversational interfaces
- RAG (Retrieval Augmented Generation) systems

---

### semantic_search

**ADVANCED**: Perform semantic similarity search with explicit control over table, column, and search parameters.

This tool enables vector similarity search using either pre-computed embedding vectors or natural language text queries.

**Recommended Workflow**:

1. **Discover vector columns**: Call `get_schema_info(vector_tables_only=true)` to find only tables with vector columns (dramatically reduces output)
2. **Execute search**: Call `semantic_search` with `text_query` parameter to automatically generate embeddings and search

**Key Features**:

- Supports multiple distance metrics (cosine, L2/Euclidean, inner product)
- Automatic detection and validation of pgvector columns
- Dimension matching validation
- Optional filtering with WHERE clause conditions
- Returns top-K most similar results with distance scores
- **NEW**: Automatic embedding generation from text queries using configured provider

**Prerequisites**:

- pgvector extension installed in your PostgreSQL database
- Table with vector column(s) containing pre-computed embeddings
- For `query_vector`: Pre-computed embedding vector (from OpenAI, Anthropic, or other embedding models)
- For `text_query`: Embedding generation must be enabled in server configuration

**Input Examples**:

Basic semantic search:
```json
{
  "table_name": "documents",
  "vector_column": "embedding",
  "query_vector": [0.1, 0.2, 0.3, ...],
  "top_k": 10,
  "distance_metric": "cosine"
}
```

With filtering:
```json
{
  "table_name": "articles",
  "vector_column": "content_embedding",
  "query_vector": [0.1, 0.2, 0.3, ...],
  "top_k": 5,
  "distance_metric": "l2",
  "filter_conditions": "category = 'technology' AND published = true"
}
```

With text query (automatic embedding generation):
```json
{
  "table_name": "documents",
  "vector_column": "embedding",
  "text_query": "What is vector similarity search?",
  "top_k": 10,
  "distance_metric": "cosine"
}
```

**Parameters**:

- `table_name` (required): Name of the table containing the vector column (can include schema: 'schema.table')
- `vector_column` (required): Name of the pgvector column to search
- `query_vector` (required*): Pre-computed embedding vector as an array of floats (must match column dimensions)
- `text_query` (required*): Natural language text to convert to embedding automatically
  - **Note**: Either `query_vector` OR `text_query` must be provided, but not both
  - Requires embedding generation to be enabled in configuration
- `top_k` (optional, default: 10): Number of most similar results to return
- `distance_metric` (optional, default: "cosine"): Distance metric to use
  - `cosine`: Cosine distance (most common for embeddings)
  - `l2` or `euclidean`: L2/Euclidean distance
  - `inner_product` or `inner`: Inner product (negative)
- `filter_conditions` (optional): SQL WHERE clause for filtering results

**Output**:
```
Semantic Search Results:
Table: documents
Vector Column: embedding (dimensions: 1536)
Distance Metric: Cosine Distance
Top K: 10

SQL Query:
SELECT *, (embedding <=> '[0.1,0.2,0.3,...]'::vector) AS distance FROM public.documents ORDER BY embedding <=> '[0.1,0.2,0.3,...]'::vector LIMIT 10

Results (10 rows):
[
  {
    "id": 42,
    "title": "Introduction to Vector Search",
    "content": "...",
    "embedding": "[0.12, 0.18, 0.31, ...]",
    "distance": 0.123
  },
  {
    "id": 87,
    "title": "Semantic Search with pgvector",
    "content": "...",
    "embedding": "[0.15, 0.22, 0.29, ...]",
    "distance": 0.156
  },
  ...
]
```

**Distance Metrics**:

- **Cosine Distance** (`<=>` operator):
  - Range: 0 to 2 (0 = identical, 2 = opposite)
  - Most commonly used for text embeddings
  - Measures angular similarity, invariant to vector magnitude
  - Use for: OpenAI embeddings, sentence embeddings, most NLP tasks

- **L2/Euclidean Distance** (`<->` operator):
  - Range: 0 to infinity (0 = identical)
  - Measures absolute distance in vector space
  - Sensitive to vector magnitude
  - Use for: Image embeddings, when absolute distance matters

- **Inner Product** (`<#>` operator):
  - Range: negative infinity to 0 (0 = most similar)
  - Note: pgvector returns negative inner product for ordering
  - Use for: Normalized vectors, when you need dot product similarity

**Use Cases**:

- **Document Search**: Find similar documents based on content embeddings
- **Question Answering**: Retrieve relevant context for RAG (Retrieval Augmented Generation)
- **Recommendation Systems**: Find similar items based on embedding vectors
- **Semantic Clustering**: Group similar items together
- **Duplicate Detection**: Identify near-duplicate content

**Example Workflows**:

**Option 1: With pre-computed query vectors**:

1. Generate embeddings for your documents (using OpenAI, Anthropic, etc.)
2. Store embeddings in a pgvector column
3. When you have a user query:
   - Generate an embedding for the query using the same model
   - Use `semantic_search` with the query embedding
   - Get the most similar documents with scores

**Option 2: With automatic embedding generation (RECOMMENDED)**:

1. Generate embeddings for your documents using Ollama or Anthropic
2. Store embeddings in a pgvector column
3. Configure embedding generation in the server
4. When you have a user query:
   - **First**: Use `get_schema_info(vector_tables_only=true)` to discover tables with vector columns - **dramatically reduces token usage**
   - **Then**: Use `semantic_search` with `text_query` parameter
   - Server automatically generates the embedding and searches
   - Get the most similar documents with scores

```json
{
  "table_name": "documents",
  "vector_column": "embedding",
  "text_query": "How do I implement RAG?",
  "top_k": 5
}
```

**Error Handling**:

The tool performs extensive validation:

- Verifies table exists in database metadata
- Verifies column exists in the table
- Checks that column is a pgvector column
- Validates query vector dimensions match column dimensions
- Validates distance metric is supported
- Validates top_k is greater than 0

**Security**: All queries are executed in read-only transactions.

**Performance Tips**:

- Create an index on your vector column for faster searches:
  ```sql
  CREATE INDEX ON documents USING ivfflat (embedding vector_cosine_ops);
  -- or for L2 distance:
  CREATE INDEX ON documents USING ivfflat (embedding vector_l2_ops);
  ```
- Use appropriate `top_k` values (10-50 is usually sufficient)
- Apply `filter_conditions` to reduce the search space
- For very large datasets, consider using HNSW index instead of IVFFlat

### generate_embedding

Generate vector embeddings from text using OpenAI, Anthropic Voyage API (cloud), or Ollama (local). Enables converting natural language queries into embedding vectors for semantic search.

**Prerequisites**:

- Embedding generation must be enabled in server configuration
- For OpenAI: Valid API key must be configured
- For Anthropic: Valid API key must be configured
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

Enable in `pgedge-postgres-mcp.yaml`:

```yaml
embedding:
  enabled: true
  provider: "openai"  # Options: "openai", "anthropic", or "ollama"
  model: "text-embedding-3-small"
  openai_api_key: ""  # Set via OPENAI_API_KEY environment variable
```

**Supported Providers and Models**:

OpenAI (Cloud):

- `text-embedding-3-small`: 1536 dimensions (recommended, compatible with most databases)
- `text-embedding-3-large`: 3072 dimensions (higher quality)
- `text-embedding-ada-002`: 1536 dimensions (legacy)

Anthropic Voyage (Cloud):

- `voyage-3`: 1024 dimensions (recommended)
- `voyage-3-lite`: 512 dimensions (cost-effective)
- `voyage-2`: 1024 dimensions
- `voyage-2-lite`: 1024 dimensions

Ollama (Local):

- `nomic-embed-text`: 768 dimensions (recommended)
- `mxbai-embed-large`: 1024 dimensions
- `all-minilm`: 384 dimensions

**Example Workflow**:

```
1. Generate embedding from query text:
   generate_embedding(text="What is vector similarity search?")

2. Use the returned embedding with semantic_search:
   semantic_search(
     table_name="documents",
     vector_column="embedding",
     query_vector=[0.023, -0.145, 0.089, ...],
     top_k=10
   )
```

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
- `pg://stat/activity` - Current connections and queries
- `pg://stat/replication` - Replication status

See [Resources](resources.md) for detailed information about each resource.

