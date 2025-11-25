```yaml
# pgEdge Knowledgebase Builder Configuration
#
# The kb-builder tool processes documentation from multiple sources (Git repos,
# local paths) and builds a searchable SQLite database with vector embeddings.
#
# Configuration Priority (highest to lowest):
#   1. Command line flags (--database, --config)
#   2. Configuration file values (this file)
#   3. Hard-coded defaults
#
# Copy this file to kb-builder.yaml and customize as needed.
# By default, kb-builder looks for config in the same directory as the binary.

# ============================================================================
# OUTPUT DATABASE CONFIGURATION
# ============================================================================
# Path to the output SQLite knowledgebase database
# Default: pgedge-mcp-kb.db in same directory as config file
# Command line flag: --database or -d
database_path: "pgedge-mcp-kb.db"

# ============================================================================
# DOCUMENTATION SOURCE DIRECTORY
# ============================================================================
# Directory for storing downloaded/processed documentation
# Git repositories will be cloned here
# Default: doc-source in same directory as config file
doc_source_path: "doc-source"

# ============================================================================
# DOCUMENTATION SOURCES
# ============================================================================
# List of documentation sources to process
# Each source can be either a Git repository or a local path
sources:
    # -------------------------
    # Git Repository Sources
    # -------------------------
    # Example: PostgreSQL 17 documentation from Git
    - git_url: "https://github.com/postgres/postgres.git"
      branch: "REL_17_STABLE"                 # Git branch to use
      # tag: "REL_17_0"                       # Alternative: use tag instead
      doc_path: "doc/src/sgml"                # Path within repo containing docs
      project_name: "PostgreSQL"              # Project identifier (required)
      project_version: "17"                   # Version identifier (optional)

    # Example: PostgreSQL 16 documentation
    - git_url: "https://github.com/postgres/postgres.git"
      branch: "REL_16_STABLE"
      doc_path: "doc/src/sgml"
      project_name: "PostgreSQL"
      project_version: "16"

    # Example: pgEdge documentation
    # - git_url: "https://github.com/pgEdge/docs.git"
    #   branch: "main"
    #   doc_path: "."                         # Docs at repo root
    #   project_name: "pgEdge"
    #   project_version: "latest"

    # -------------------------
    # Local Path Sources
    # -------------------------
    # Example: Local documentation directory
    # - local_path: "~/projects/my-project"
    #   doc_path: "docs"                      # Optional subdirectory
    #   project_name: "My Project"
    #   project_version: "1.0"

    # Example: Absolute path
    # - local_path: "/opt/documentation/myapp"
    #   doc_path: "."                         # Process entire directory
    #   project_name: "MyApp"
    #   project_version: "2.5"

# ============================================================================
# EMBEDDING PROVIDER CONFIGURATION
# ============================================================================
# Configure one or more embedding providers
# The knowledgebase will store embeddings from all enabled providers
# The MCP server can use any available provider for search
#
# IMPORTANT: Enable at least one provider
embeddings:
    # -------------------------
    # OpenAI Embeddings
    # -------------------------
    openai:
        # Enable OpenAI embeddings
        # Default: false
        enabled: true

        # Path to file containing OpenAI API key
        # Default: ~/.openai-api-key
        # Environment variable: OPENAI_API_KEY (takes priority)
        api_key_file: "~/.openai-api-key"

        # OpenAI embedding model
        # Options: text-embedding-3-small (1536 dim),
        #          text-embedding-3-large (3072 dim),
        #          text-embedding-ada-002 (1536 dim)
        # Default: text-embedding-3-small
        model: "text-embedding-3-small"

        # Embedding dimensions (optional, model-specific)
        # Only needed for models that support variable dimensions
        # Default: 1536 (for text-embedding-3-small)
        dimensions: 1536

    # -------------------------
    # Voyage AI Embeddings
    # -------------------------
    voyage:
        # Enable Voyage AI embeddings
        # Default: false
        enabled: false

        # Path to file containing Voyage API key
        # Default: ~/.voyage-api-key
        # Environment variable: VOYAGE_API_KEY (takes priority)
        api_key_file: "~/.voyage-api-key"

        # Voyage embedding model
        # Options: voyage-3 (1024 dim), voyage-3-lite (512 dim)
        # Default: voyage-3
        model: "voyage-3"

    # -------------------------
    # Ollama Local Embeddings
    # -------------------------
    ollama:
        # Enable Ollama embeddings (local, no API key needed)
        # Default: false
        enabled: false

        # Ollama API endpoint
        # Default: http://localhost:11434
        endpoint: "http://localhost:11434"

        # Ollama embedding model
        # Options: nomic-embed-text (768 dim), mxbai-embed-large (1024 dim)
        # Default: nomic-embed-text
        # Note: Model must be pulled first: ollama pull nomic-embed-text
        model: "nomic-embed-text"

# ============================================================================
# SUPPORTED DOCUMENT FORMATS
# ============================================================================
# The kb-builder automatically detects and converts:
#   - Markdown (.md)
#   - HTML (.html, .htm)
#   - reStructuredText (.rst)
#   - SGML (.sgml, .sgm)
#   - DocBook XML (.xml)
#
# Documents are converted to Markdown, chunked intelligently, and embedded.

# ============================================================================
# COMMAND LINE USAGE
# ============================================================================
# Basic usage:
#   ./kb-builder --config kb-builder.yaml
#
# Override database path:
#   ./kb-builder --config kb-builder.yaml --database /path/to/output.db
#
# Skip git pull for existing repos (faster for development):
#   ./kb-builder --config kb-builder.yaml --skip-updates
#
# Add missing embeddings to existing database:
#   ./kb-builder --config kb-builder.yaml --add-missing-embeddings
#
# Clear embeddings for a specific provider:
#   ./kb-builder --config kb-builder.yaml --clear-embeddings openai
#   ./kb-builder --config kb-builder.yaml --clear-embeddings voyage
#   ./kb-builder --config kb-builder.yaml --clear-embeddings ollama
```

## Configuration Examples

### PostgreSQL Documentation Only

```yaml
database_path: "pgedge-mcp-kb.db"
doc_source_path: "doc-source"

sources:
    - git_url: "https://github.com/postgres/postgres.git"
      branch: "REL_17_STABLE"
      doc_path: "doc/src/sgml"
      project_name: "PostgreSQL"
      project_version: "17"

embeddings:
    openai:
        enabled: true
        api_key_file: "~/.openai-api-key"
        model: "text-embedding-3-small"
        dimensions: 1536

    voyage:
        enabled: false

    ollama:
        enabled: false
```

### Multiple PostgreSQL Versions with Voyage AI

```yaml
database_path: "postgres-multi-version-kb.db"
doc_source_path: "postgres-docs"

sources:
    - git_url: "https://github.com/postgres/postgres.git"
      branch: "REL_17_STABLE"
      doc_path: "doc/src/sgml"
      project_name: "PostgreSQL"
      project_version: "17"

    - git_url: "https://github.com/postgres/postgres.git"
      branch: "REL_16_STABLE"
      doc_path: "doc/src/sgml"
      project_name: "PostgreSQL"
      project_version: "16"

    - git_url: "https://github.com/postgres/postgres.git"
      branch: "REL_15_STABLE"
      doc_path: "doc/src/sgml"
      project_name: "PostgreSQL"
      project_version: "15"

embeddings:
    openai:
        enabled: false

    voyage:
        enabled: true
        api_key_file: "~/.voyage-api-key"
        model: "voyage-3"

    ollama:
        enabled: false
```

### Local Development with Ollama

```yaml
database_path: "local-kb.db"
doc_source_path: "local-docs"

sources:
    - local_path: "~/projects/myapp"
      doc_path: "docs"
      project_name: "MyApp"
      project_version: "dev"

    - local_path: "/opt/docs/internal"
      doc_path: "."
      project_name: "Internal Docs"
      project_version: "latest"

embeddings:
    openai:
        enabled: false

    voyage:
        enabled: false

    ollama:
        enabled: true
        endpoint: "http://localhost:11434"
        model: "nomic-embed-text"
```

Then run:

```bash
# Make sure Ollama is running and model is pulled
ollama pull nomic-embed-text

# Build the knowledgebase
./kb-builder --config kb-builder.yaml
```

### Multiple Embedding Providers (Recommended for Flexibility)

```yaml
database_path: "multi-provider-kb.db"
doc_source_path: "doc-source"

sources:
    - git_url: "https://github.com/postgres/postgres.git"
      branch: "REL_17_STABLE"
      doc_path: "doc/src/sgml"
      project_name: "PostgreSQL"
      project_version: "17"

embeddings:
    # Enable all three providers
    # MCP server can use any one for search
    openai:
        enabled: true
        api_key_file: "~/.openai-api-key"
        model: "text-embedding-3-small"
        dimensions: 1536

    voyage:
        enabled: true
        api_key_file: "~/.voyage-api-key"
        model: "voyage-3"

    ollama:
        enabled: true
        endpoint: "http://localhost:11434"
        model: "nomic-embed-text"
```

This configuration generates embeddings from all three providers, allowing the
MCP server to use any available provider for search based on its own
configuration.

## Environment Variables

The kb-builder supports the following environment variables:

- `OPENAI_API_KEY`: OpenAI API key (overrides api_key_file)
- `VOYAGE_API_KEY`: Voyage AI API key (overrides api_key_file)

## Building Your First Knowledgebase

1. **Install Prerequisites**:

   ```bash
   # For Ollama (optional)
   ollama pull nomic-embed-text
   ```

2. **Set Up API Keys** (for OpenAI or Voyage):

   ```bash
   # For OpenAI
   echo "sk-your-openai-key" > ~/.openai-api-key
   chmod 600 ~/.openai-api-key

   # For Voyage
   echo "pa-your-voyage-key" > ~/.voyage-api-key
   chmod 600 ~/.voyage-api-key
   ```

3. **Create Configuration**:

   ```bash
   cp kb-builder.yaml.example kb-builder.yaml
   # Edit kb-builder.yaml to configure sources and embedding providers
   ```

4. **Build the Knowledgebase**:

   ```bash
   ./kb-builder --config kb-builder.yaml
   ```

5. **Configure MCP Server**:

   Add to your MCP server configuration:

   ```yaml
   knowledgebase:
       enabled: true
       database_path: "./pgedge-mcp-kb.db"
       embedding_provider: "openai"  # Match your kb-builder provider
       embedding_model: "text-embedding-3-small"
       embedding_openai_api_key_file: "~/.openai-api-key"
   ```

## Incremental Updates

The kb-builder supports incremental processing:

- Git repositories are pulled to get latest changes
- Only modified files are reprocessed
- Unchanged files reuse existing chunks and embeddings
- Use `--skip-updates` to skip git pull during development

Example:

```bash
# Initial build (full processing)
./kb-builder --config kb-builder.yaml

# Later update (only changed files)
./kb-builder --config kb-builder.yaml
```

## Managing Embeddings

### Adding Missing Embeddings

If a build fails or you enable a new provider later:

```bash
./kb-builder --config kb-builder.yaml --add-missing-embeddings
```

This will only generate embeddings that are missing, skipping files that
already have embeddings.

### Clearing Embeddings

To clear embeddings for a specific provider:

```bash
# Clear OpenAI embeddings
./kb-builder --config kb-builder.yaml --clear-embeddings openai

# Clear Voyage embeddings
./kb-builder --config kb-builder.yaml --clear-embeddings voyage

# Clear Ollama embeddings
./kb-builder --config kb-builder.yaml --clear-embeddings ollama
```

After clearing, you can rebuild with:

```bash
./kb-builder --config kb-builder.yaml --add-missing-embeddings
```

## See Also

- [Knowledgebase Search](knowledgebase.md) - Using the search_knowledgebase
    tool
- [MCP Server Configuration](config-example.md) - Configure the MCP server to
    use the knowledgebase
