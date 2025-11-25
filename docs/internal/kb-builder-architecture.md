# kb-builder architecture

**Audience**: Project hackers and contributors

This document describes the internal architecture of the `kb-builder` tool,
which processes documentation and creates searchable knowledgebase databases.

## overview

The kb-builder is a standalone Go binary that:

1. Fetches documentation from various sources (Git repos, local paths)
2. Converts multiple document formats to Markdown
3. Intelligently chunks documents with context preservation
4. Generates embeddings using multiple providers
5. Stores everything in an optimized SQLite database

## architecture

```
┌────────────────────────────────────────────────────────────┐
│                         kb-builder                         │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌──────────────┐      ┌──────────────┐                    │
│  │  CLI Parser  │─────▶│ Config Loader│                    │
│  └──────────────┘      └──────┬───────┘                    │
│                               │                            │
│  ┌─────────────────────────────▼──────────────────────┐    │
│  │            Source Fetcher (kbsource)               │    │
│  │  • Git clone/pull with branch/tag support          │    │
│  │  • Local directory scanning                        │    │
│  └─────────────────────────┬──────────────────────────┘    │
│                            │                               │
│  ┌─────────────────────────▼──────────────────────────┐    │
│  │         Document Converter (kbconverter)           │    │
│  │  • HTML → Markdown                                 │    │
│  │  • RST → Markdown                                  │    │
│  │  • SGML/DocBook → Markdown                         │    │
│  │  • Markdown (passthrough with title extraction)    │    │
│  └─────────────────────────┬──────────────────────────┘    │
│                            │                               │
│  ┌─────────────────────────▼──────────────────────────┐    │
│  │           Document Chunker (kbchunker)             │    │
│  │  • Section-aware splitting                         │    │
│  │  • 800-token target, 1000 max                      │    │
│  │  • 200-token overlap                               │    │
│  │  • Sentence boundary detection                     │    │
│  └─────────────────────────┬──────────────────────────┘    │
│                            │                               │
│  ┌─────────────────────────▼──────────────────────────┐    │
│  │        Embedding Generator (kbembed)               │    │
│  │  • OpenAI API (batch processing)                   │    │
│  │  • Voyage AI API (batch processing)                │    │
│  │  • Ollama (sequential processing)                  │    │
│  └─────────────────────────┬──────────────────────────┘    │
│                            │                               │
│  ┌─────────────────────────▼──────────────────────────┐    │
│  │          Database Writer (kbdatabase)              │    │
│  │  • SQLite with transaction batching                │    │
│  │  • BLOB storage for embeddings                     │    │
│  │  • Indexes for project/version filtering           │    │
│  └────────────────────────────────────────────────────┘    │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

## components

### kbconfig

**Location**: `internal/kbconfig/`

**Responsibility**: Configuration parsing and validation

**Key features**:
- YAML configuration file parsing
- API key loading from separate files
- Path expansion (~ to home directory)
- Default value application
- Multi-source configuration support

**Configuration structure**:
```yaml
database_path: string
doc_source_path: string
sources: []DocumentSource
embeddings:
  openai: OpenAIConfig
  voyage: VoyageConfig
  ollama: OllamaConfig
```

### kbsource

**Location**: `internal/kbsource/`

**Responsibility**: Fetching documentation from sources

**Supported sources**:
- Git repositories (with branch/tag support)
- Local filesystem paths

**Key operations**:
- `FetchAll()`: Process all configured sources
- `gitClone()`: Clone repository if not exists
- `gitPull()`: Update existing repository
- `gitCheckout()`: Switch to specific branch/tag

**Design notes**:
- Uses `exec.Command` for git operations
- Creates timestamped directories for each source
- Sanitizes project names for safe directory names

### kbconverter

**Location**: `internal/kbconverter/`

**Responsibility**: Convert various document formats to Markdown

**Supported formats**:
- HTML (`.html`, `.htm`)
- Markdown (`.md`)
- reStructuredText (`.rst`)
- SGML/DocBook (`.sgml`, `.sgm`)
- DocBook XML (`.xml`)

**Key algorithms**:

**HTML conversion**:
- Uses `html-to-markdown` library
- Shifts heading levels (H1→H2, etc.) to reserve H1 for title
- Extracts title from `<title>` tag
- Decodes HTML entities

**RST conversion**:
- Pattern matching for heading underlines
- Maintains heading hierarchy
- Converts common RST directives
- Handles both overline+underline and underline-only headings

**SGML conversion** (PostgreSQL DocBook):
- Pattern-based tag conversion
- Handles chapter, sect1-4, refsect1-2
- Converts emphasis tags to Markdown equivalents
- Preserves code blocks with ``` fences

**Design notes**:
- All converters return (markdown, title, error)
- Title extraction is format-specific
- Conversion preserves structure for chunking

### kbchunker

**Location**: `internal/kbchunker/`

**Responsibility**: Intelligent document chunking

**Chunking algorithm**:

1. **Section parsing**: Split markdown by headings
2. **Size evaluation**: Check if section fits in one chunk
3. **Smart splitting**: For large sections:
   - Use sliding window (800 tokens target, 1000 max)
   - 200-token overlap between chunks
   - Break at sentence boundaries when possible
4. **Context preservation**: Include section heading in all chunks

**Token estimation**:
- Simple word + punctuation counting
- ~4 characters per token approximation
- Conservative estimates to avoid truncation

**Design notes**:
- Respects document structure (sections)
- Overlap ensures context preservation
- Sentence boundary detection prevents mid-sentence cuts

### kbembed

**Location**: `internal/kbembed/`

**Responsibility**: Generate embeddings from multiple providers

**Providers**:

**OpenAI**:
- API: `https://api.openai.com/v1/embeddings`
- Batch size: 100 texts per request
- Model: `text-embedding-3-small` (default)
- Dimensions: 1536 (configurable)

**Voyage AI**:
- API: `https://api.voyageai.com/v1/embeddings`
- Batch size: 100 texts per request
- Model: `voyage-3` (default)

**Ollama**:
- API: `http://localhost:11434/api/embeddings`
- Sequential processing (one at a time)
- Model: `nomic-embed-text` (default)

**Design notes**:
- Each provider processed sequentially
- Progress reporting every batch/10 items
- Embeddings stored as float32 for efficiency
- All enabled providers must succeed

### kbdatabase

**Location**: `internal/kbdatabase/`

**Responsibility**: SQLite database operations

**Schema**:
```sql
CREATE TABLE chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    text TEXT NOT NULL,
    title TEXT,
    section TEXT,
    project_name TEXT NOT NULL,
    project_version TEXT NOT NULL,
    file_path TEXT,
    openai_embedding BLOB,
    voyage_embedding BLOB,
    ollama_embedding BLOB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_project ON chunks(project_name, project_version);
CREATE INDEX idx_title ON chunks(title);
CREATE INDEX idx_section ON chunks(section);
```

**Embedding storage**:
- Float32 arrays serialized to BLOB
- Little-endian byte order
- 4 bytes per dimension
- Separate column per provider

**Design notes**:
- Uses transactions for batch inserts
- Indexes optimize filtering queries
- BLOB storage more efficient than JSON arrays
- Stats query for progress reporting

### kbtypes

**Location**: `internal/kbtypes/`

**Responsibility**: Shared type definitions

**Key types**:
```go
type Document struct {
    Title, Content string
    ProjectName, ProjectVersion string
    FilePath string
    DocType DocumentType
}

type Chunk struct {
    Text, Title, Section string
    ProjectName, ProjectVersion string
    FilePath string
    OpenAIEmbedding, VoyageEmbedding, OllamaEmbedding []float32
}
```

## build process

### typical workflow

1. **Configure sources** in `kb-builder.yaml`
2. **Run builder**: `./bin/kb-builder --config kb-builder.yaml`
3. **Process executes**:
   - Fetch all sources (git clone/pull or local scan)
   - For each source:
     - Walk directory tree
     - Filter supported file types
     - Convert to Markdown
     - Chunk with overlap
   - Generate embeddings (all chunks, all providers)
   - Store in SQLite database
4. **Output**: `pgedge-mcp-kb.db` (typically 300-500MB)

### performance characteristics

**PostgreSQL 17 documentation** (~3000 pages):
- Chunks created: ~30,000
- Embedding time (OpenAI): ~5-10 minutes
- Database size: ~250MB
- Search performance: <100ms for top-5

**Multiple versions** (PG 13-17):
- Chunks created: ~150,000
- Embedding time (OpenAI): ~25-50 minutes
- Database size: ~500MB

### error handling

- Non-fatal: Skip unsupported files, continue processing
- Fatal: API key missing, network errors, database errors
- Transactional: Database writes are all-or-nothing per source

## testing

### unit tests

Each component has unit tests:
- `kbconfig_test.go`: Configuration parsing
- `kbconverter_test.go`: Format conversions
- `kbchunker_test.go`: Chunking algorithms
- `kbdatabase_test.go`: Database operations

### integration tests

Full pipeline tests:
- Sample documentation processing
- Multi-provider embedding generation
- Database creation and search

### test data

Located in `test/fixtures/`:
- Sample HTML, Markdown, RST, SGML documents
- Small test configuration
- Expected output chunks

## extending

### adding new document formats

1. Add format detection in `DetectDocumentType()`
2. Implement converter function: `convertXYZ(content []byte) (string, string, error)`
3. Add to `Convert()` switch statement
4. Add file extensions to `GetSupportedExtensions()`
5. Add tests with sample documents

### adding new embedding providers

1. Add config struct to `kbconfig.EmbeddingConfig`
2. Implement generation in `kbembed.EmbeddingGenerator`
3. Add BLOB column to database schema
4. Update `kbtypes.Chunk` structure
5. Add provider selection in `search_knowledgebase.go`

### customizing chunking

Adjust constants in `internal/kbchunker/chunker.go`:
```go
const (
    TargetChunkSize = 800   // Target tokens per chunk
    MaxChunkSize = 1000     // Maximum tokens per chunk
    OverlapSize = 200       // Overlap between chunks
)
```

## maintenance

### rebuilding databases

To update documentation:
1. Edit `kb-builder.yaml` (update branch/tag or local paths)
2. Run `kb-builder` again
3. Replace old database file with new one
4. Restart MCP server to use new database

### incremental updates

Current implementation: Full rebuild required

Future optimization: Track file modification times and only reprocess changed
files.

### database optimization

SQLite VACUUM recommended after large updates:
```bash
sqlite3 pgedge-mcp-kb.db "VACUUM;"
```

## troubleshooting

### git clone failures

- Check network connectivity
- Verify repository URL
- Check authentication for private repos
- Ensure sufficient disk space

### embedding API errors

- Verify API keys are present and valid
- Check rate limits (OpenAI: 3000 req/min)
- Verify network connectivity to API endpoints
- For Ollama: ensure service is running

### out of memory

For large documentation sets:
- Process sources one at a time (modify to sequential processing)
- Reduce batch sizes in embedding generation
- Use streaming for large files

### database corruption

- Check disk space during writes
- Verify filesystem supports large files
- Use transactions (already implemented)
- Keep backups of working databases

## see also

- `docs/knowledgebase.md` - User-facing documentation
- `KB-README.md` - Quick start guide
- `kb-builder.yaml.example` - Example configuration
