# Knowledgebase Tool

We are creating a tool in the MCP server to allow the LLM to be able to access documentation on all our products. This will consist of two parts; the tool itself, to be implemented in the MCP server, and a document loader to create the knowledgebase database file. Below you will find a discussion with Claude, hashing out how the tool will work. Following that, there is a roughed-out design for the document loader tool.

IMPORTANT: Both parts of this project must be designed to work together. The document loader must create a database that uses the exact schema that the tool expects, and must be optimised to maximise the effectiveness of the tool to give specialist information to the LLM on topics that it might not have been trained on.

## Question

In my MCP server and its clients, I need to be able to support use of models that have had little to no training on our products. Of course, I can use embeddings in PostgreSQL to allow the LLM to search for specific information, but I don’t want to have to include PostgreSQL as part of the application. Is there a way I can supplement the model’s training with product documentation without using a separate database server?

## Answer

Absolutely! There are several ways to supplement model knowledge without requiring a separate database server:

### 1. **Prompt Caching with Documentation** (Simplest)

Since we just discussed prompt caching, this is worth considering: include your product documentation directly in the system prompt and let caching handle the efficiency.

With **Anthropic’s Claude**, you can mark documentation for caching:

```python
{
    "model": "claude-sonnet-4-20250514",
    "system": [
        {
            "type": "text",
            "text": "You are an assistant for Product X..."
        },
        {
            "type": "text", 
            "text": "# Product Documentation\n\n[... huge docs here ...]",
            "cache_control": {"type": "ephemeral"}
        }
    ],
    "messages": [...]
}
```

With **OpenAI**, it happens automatically for prompts >1024 tokens - just structure your prompt with static docs at the beginning.

**Pros**: Zero dependencies, simple  
**Cons**: Limited by context window, docs must fit in memory, cache expiration means reprocessing

### 2. **Embedded Vector Databases** (Most Capable)

Several vector databases can run embedded without a separate server:

**ChromaDB** (my recommendation for simplicity):

```python
import chromadb
from chromadb.utils import embedding_functions

# Runs entirely in-process or persists to local directory
client = chromadb.PersistentClient(path="./chroma_db")
collection = client.get_or_create_collection(
    name="product_docs",
    embedding_function=embedding_functions.OpenAIEmbeddingFunction(
        api_key="your-key",
        model_name="text-embedding-3-small"
    )
)

# Add documents
collection.add(
    documents=["doc text..."],
    ids=["doc1"]
)

# Query
results = collection.query(
    query_texts=["user question"],
    n_results=5
)
```

**Other options**:

- **LanceDB** - Embedded, disk-based, very fast
- **Qdrant** - Has embedded mode
- **DuckDB with vss extension** - SQL interface with vector search
- **FAISS** - Pure in-memory, very fast (from Meta)

### 3. **SQLite with Vector Extensions**

Keep the familiar SQL interface without a server:

**sqlite-vec** or **sqlite-vss**:

```python
import sqlite3
import sqlite_vec

conn = sqlite3.connect('products.db')
conn.enable_load_extension(True)
sqlite_vec.load(conn)

# Create vector table and search
# Similar SQL interface to PostgreSQL's pgvector
```

### 4. **Simple In-Memory BM25** (No Embeddings)

If you want to avoid embedding API costs entirely:

```python
from rank_bm25 import BM25Okapi
import pickle

# Index your docs once
corpus = [doc.split() for doc in documents]
bm25 = BM25Okapi(corpus)

# Save to disk
with open('bm25_index.pkl', 'wb') as f:
    pickle.dump(bm25, f)

# Search
scores = bm25.get_scores(query.split())
top_docs = sorted(enumerate(scores), key=lambda x: x[1], reverse=True)[:5]
```

You mentioned working on hybrid search with BM25 reranking in your pgEdge AI Server project - this approach gives you similar lexical search without any database.

### 5. **Hybrid: Precompute + Bundle**

For relatively static documentation:

1. Precompute embeddings offline
1. Store in simple JSON/numpy files bundled with your MCP server
1. Load into memory on startup
1. Use numpy/scipy for cosine similarity search

```python
import numpy as np
from numpy.linalg import norm

# Load precomputed embeddings
embeddings = np.load('doc_embeddings.npy')
metadata = json.load(open('doc_metadata.json'))

def search(query_embedding, top_k=5):
    # Cosine similarity
    similarities = embeddings @ query_embedding / (
        norm(embeddings, axis=1) * norm(query_embedding)
    )
    top_indices = np.argsort(similarities)[-top_k:][::-1]
    return [metadata[i] for i in top_indices]
```

### My Recommendation

Given your PostgreSQL background and the MCP server context:

- **For small docs (<100k tokens)**: Use **prompt caching** - dead simple, no dependencies
- **For medium docs with good search**: **ChromaDB embedded** - Python-friendly, no server, persists locally
- **For familiar SQL interface**: **SQLite + sqlite-vec** - feels like PostgreSQL, single file
- **For maximum simplicity**: **BM25 + pickle** - no embedding API calls needed

Since you’re building general-purpose tools for the PostgreSQL community, the embedded approach with ChromaDB or SQLite would give users the best experience - familiar concepts, no server setup, just works.

-----

## Follow-up Question

The docs are likely to be quite large - Postgres’ manual is probably a couple of thousand pages for a single release (and we would support 5). Other docs are much smaller, but still likely many thousands of times larger than could be prompt cached in entirety. I like the idea of using SQLite - we would pre-load the database as part of the build process, and simply ship the file as part of the product. I assume we would then make a dedicated similarity search tool in the MCP server that the LLM would use when it wants to find out about something it doesn’t know?

## Detailed Implementation Guide

Exactly right! This is a solid architecture and quite common in production systems. Let me walk through the key pieces:

### Architecture Overview

```
Build Time:
1. Parse documentation → Chunk into passages
2. Generate embeddings (batch via OpenAI/Anthropic)
3. Store in SQLite with vector extension
4. Ship docs.db with your MCP server

Runtime:
1. User asks question about PostgreSQL 
2. LLM calls your similarity_search tool
3. Tool queries SQLite, returns relevant chunks
4. LLM uses chunks to answer question
```

### SQLite Vector Extensions

**sqlite-vec** (newer, recommended):

- Pure extension, very fast
- Good for your use case
- Active development

**sqlite-vss** (alternative):

- Based on FAISS
- More mature but heavier

For PostgreSQL docs across 5 versions, I’d go with **sqlite-vec**.

### Database Schema

```sql
CREATE TABLE documentation (
    id INTEGER PRIMARY KEY,
    version TEXT NOT NULL,           -- e.g., 'pg17', 'pg16'
    doc_type TEXT,                    -- e.g., 'manual', 'release_notes'
    section TEXT,                     -- e.g., 'Chapter 9: Functions'
    chunk_text TEXT NOT NULL,
    metadata TEXT,                    -- JSON: page number, URL, etc.
    embedding BLOB NOT NULL           -- Vector embedding
);

-- Vector index for similarity search
CREATE INDEX idx_doc_embeddings ON documentation 
    USING vec_index(embedding);

-- Regular indexes for filtering
CREATE INDEX idx_version ON documentation(version);
CREATE INDEX idx_doc_type ON documentation(doc_type);
```

### Chunking Strategy

This is critical for large docs. You’ll want to chunk intelligently:

```python
def chunk_postgres_docs(content, version):
    """
    Smart chunking for PostgreSQL documentation.
    Respects section boundaries, overlaps for context.
    """
    chunks = []
    
    # Option 1: Semantic chunking (respect sections/paragraphs)
    sections = parse_html_sections(content)  # Keep headers with content
    
    for section in sections:
        # Target ~500-1000 tokens per chunk
        if len(section.text) > MAX_CHUNK_SIZE:
            # Split long sections with overlap
            for chunk in sliding_window(section.text, 
                                       size=800, 
                                       overlap=200):
                chunks.append({
                    'text': f"{section.heading}\n\n{chunk}",
                    'version': version,
                    'section': section.heading,
                    'metadata': section.metadata
                })
        else:
            chunks.append({
                'text': f"{section.heading}\n\n{section.text}",
                'version': version,
                'section': section.heading,
                'metadata': section.metadata
            })
    
    return chunks
```

Key considerations:

- **Include section headings** in chunks for context
- **Overlap chunks** (200-300 tokens) so context isn’t lost at boundaries
- **Preserve hierarchy** - store chapter/section info in metadata
- **Aim for 500-1000 tokens** per chunk (sweet spot for retrieval)

### MCP Tool Design

```python
# In your MCP server
@server.tool()
async def search_postgres_docs(
    query: str,
    version: str = "latest",  # or specific like "pg17"
    top_k: int = 5,
    doc_type: str = None  # optional filter
) -> list[dict]:
    """
    Search PostgreSQL documentation using semantic similarity.
    
    Use this tool when you need information about PostgreSQL features,
    syntax, functions, or concepts that may not be in your training data.
    
    Args:
        query: Natural language question or search term
        version: PostgreSQL version (pg17, pg16, pg15, pg14, pg13, latest)
        top_k: Number of relevant passages to return (default 5)
        doc_type: Filter by document type (manual, release_notes, etc.)
    
    Returns:
        List of relevant documentation passages with metadata
    """
    
    # Generate query embedding
    query_embedding = await get_embedding(query)
    
    # Build SQL query
    sql = """
        SELECT 
            chunk_text,
            version,
            section,
            metadata,
            vec_distance_cosine(embedding, ?) as distance
        FROM documentation
        WHERE 1=1
    """
    params = [query_embedding]
    
    if version != "latest":
        sql += " AND version = ?"
        params.append(version)
    
    if doc_type:
        sql += " AND doc_type = ?"
        params.append(doc_type)
    
    sql += " ORDER BY distance LIMIT ?"
    params.append(top_k)
    
    results = conn.execute(sql, params).fetchall()
    
    return [
        {
            "text": row[0],
            "version": row[1],
            "section": row[2],
            "metadata": json.loads(row[3]),
            "relevance_score": 1 - row[4]  # Convert distance to similarity
        }
        for row in results
    ]
```

### Build Process

```python
# build_docs.py
import sqlite3
import sqlite_vec
from openai import OpenAI

def build_documentation_db():
    """Run this during your build/release process"""
    
    conn = sqlite3.connect('docs.db')
    conn.enable_load_extension(True)
    sqlite_vec.load(conn)
    
    # Create schema
    conn.executescript(SCHEMA_SQL)
    
    # Process each PostgreSQL version
    for version in ['pg17', 'pg16', 'pg15', 'pg14', 'pg13']:
        print(f"Processing PostgreSQL {version}...")
        
        # Download/parse documentation
        docs = download_postgres_docs(version)
        
        # Chunk documentation
        chunks = chunk_postgres_docs(docs, version)
        print(f"  Created {len(chunks)} chunks")
        
        # Generate embeddings in batches (API limits)
        embeddings = []
        BATCH_SIZE = 2048  # OpenAI limit
        
        for i in range(0, len(chunks), BATCH_SIZE):
            batch = chunks[i:i+BATCH_SIZE]
            texts = [c['text'] for c in batch]
            
            # Batch embed
            response = openai.embeddings.create(
                model="text-embedding-3-small",  # 1536 dims, cheap
                input=texts
            )
            embeddings.extend([e.embedding for e in response.data])
            print(f"  Embedded {len(embeddings)}/{len(chunks)}")
        
        # Insert into database
        for chunk, embedding in zip(chunks, embeddings):
            conn.execute("""
                INSERT INTO documentation 
                (version, doc_type, section, chunk_text, metadata, embedding)
                VALUES (?, ?, ?, ?, ?, ?)
            """, (
                chunk['version'],
                chunk.get('doc_type', 'manual'),
                chunk['section'],
                chunk['text'],
                json.dumps(chunk['metadata']),
                sqlite_vec.serialize_float32(embedding)
            ))
        
        conn.commit()
    
    print(f"Database built: {os.path.getsize('docs.db') / (1024**2):.1f} MB")
    conn.close()
```

### Practical Considerations

**File Size Estimates:**

- PostgreSQL manual ~3000 pages × 5 versions = 15,000 pages
- Chunked at ~1 page per 2 chunks = ~30,000 chunks
- Embedding size: 1536 dims × 4 bytes = 6KB per chunk
- Total: ~180MB + text storage ≈ **300-500MB database file**

This is totally reasonable to ship with your product.

**Hybrid Search Enhancement:**

Since you’ve worked with BM25 reranking, consider adding FTS5:

```sql
-- Add full-text search
CREATE VIRTUAL TABLE docs_fts USING fts5(
    chunk_text,
    section,
    content=documentation,
    content_rowid=id
);

-- Hybrid search: combine vector + BM25
WITH vector_results AS (
    SELECT id, vec_distance_cosine(embedding, ?) as distance
    FROM documentation
    ORDER BY distance LIMIT 20
),
bm25_results AS (
    SELECT rowid as id, rank as bm25_score
    FROM docs_fts
    WHERE docs_fts MATCH ?
    ORDER BY rank LIMIT 20
)
SELECT d.*, 
       COALESCE(v.distance, 1.0) as vec_score,
       COALESCE(b.bm25_score, 0) as bm25_score
FROM documentation d
LEFT JOIN vector_results v ON d.id = v.id
LEFT JOIN bm25_results b ON d.id = b.id
WHERE v.id IS NOT NULL OR b.id IS NOT NULL
ORDER BY (0.7 * (1 - COALESCE(v.distance, 1))) + 
         (0.3 * COALESCE(-b.bm25_score, 0)) DESC
LIMIT 5;
```

**Version Management:**

You could also let users filter by version or search across all versions:

```python
# User query: "How do I use MERGE in PostgreSQL?"
# Tool intelligently searches:
# - Latest version first
# - Falls back to older versions if feature doesn't exist
# - Returns version info so LLM knows when feature was introduced
```

**Update Strategy:**

For new PostgreSQL releases:

1. Run build script for new version
1. Append to existing database OR ship updated database
1. Version your docs.db file (e.g., `docs-v1.2.3.db`)

### MCP Server Integration

Your MCP server would look something like:

```python
from mcp.server import Server
import sqlite3
import sqlite_vec

server = Server("postgres-docs-server")
conn = None

@server.initialize()
async def initialize():
    global conn
    conn = sqlite3.connect('docs.db', check_same_thread=False)
    conn.enable_load_extension(True)
    sqlite_vec.load(conn)

@server.tool()
async def search_postgres_docs(...):
    # As defined above
    pass

# Could also add tools like:
@server.tool()
async def get_postgres_versions():
    """List available PostgreSQL versions in documentation"""
    pass

@server.tool()
async def search_release_notes(version: str, query: str):
    """Search release notes for a specific version"""
    pass
```

### Summary

This architecture gives you:

- ✅ No separate database server required
- ✅ Single file distribution
- ✅ Fast searches (SQLite is optimized for reads)
- ✅ Version filtering
- ✅ Scales to millions of chunks if needed
- ✅ Familiar SQL interface for debugging/maintenance

# IMPORTANT: We don’t know what LLM providers the end-user will have configured in their deployment. Therefore, we must include columns in the table for embedding vectors from each provider, OpenAI, Voyage, Ollama, and use the appropriate column at runtime.

# Knowledgebase loader

We need to create a standalone tool called kb-builder (that will not be shipped to end users). Its code should be completely self-contained, and only documented for the benefit of project hackers, NOT for application developers or end users.

## Configuration

It will take the path to a configuration file on the command line, or default to pgedge-nla-kb-builder.yaml in the same directory as the binary.

It will also take the path to the output SQLite database (with sqlite-vec) as a command line option, or from the configuration file, or defaulting to pgedge-nla-kb.db in the same directory as the binary (in order of priority).

The configuration file will include an array of documentation sources including one of the following two sets of information:

* Git repository URL (as would be passed to “git clone”
* Branch name or tag to pull
* Path within the project containing documentation
* Project name (whatever the user chooses)
* Project version (whatever the user chooses)

Or

* Local path to the directory containing the documentation
* Project name (whatever the user chooses)
* Project version (whatever the user chooses)

The configuration file will also include a local path in which to store files for processing, defaulting to “doc-source” in the same directory as the binary.

The configuration file will also include the required configuration options to connect to the OpenAI, Voyage, and Ollama APIs. API keys will be stored in separate files, pointed to by configuration options, defaulting to ~/.openai-api-key and “~/.voyage-api-key. respectively.

## Basic operation

When run, the tool will iterate through all of the configured documentation sources, pulling the doc files (Markdown, Restructured text, HTML, or SGML) into directories in the doc-source directory (either as configured, or the default).

Once a document set has been downloaded, we will iterate all files of supported types following the following process:

* Convert the text to Markdown and extract the title (in memory - don’t rewrite the source file. IMPORTANT: See the project at ../pgedge-docloader to see how this can be done for HTML, Markdown, and Restructured Text. SGML will have to be implemented from scratch.
* Strip any content that is not useful for semantic searching or conveying information to an LLM, e.g. formatting information, links (keep the link text of course), image tags etc.
* Chunk the text in memory, as pgedge-docloader does.
* Generate embeddings for each chunk, using each of the three LLM providers.
* Store the chunks in the output database.

## Output requirements

The resulting database will be consumed by the knowledgebase tool in the MCP server. The structure, schema, and chunk layout must be compatible and optimised in design for the most effective use as a knowledgebase for LLMs.