# Embedding Provider Configurations

The server supports generating embeddings from text using three providers: OpenAI (cloud-based), Voyage AI (cloud-based), or Ollama (local, self-hosted). This enables you to convert natural language queries into vector embeddings for semantic search.

## Configuration File

```yaml
embedding:
  enabled: true
  provider: "openai"  # Options: "openai", "voyage", or "ollama"
  model: "text-embedding-3-small"
  openai_api_key: ""  # Set via OPENAI_API_KEY environment variable
```

### Using OpenAI (Cloud Embeddings)

**Advantages**: High quality, industry standard, multiple dimension options

**Configuration**:

```yaml
embedding:
  enabled: true
  provider: "openai"
  model: "text-embedding-3-small"  # 1536 dimensions

  # API key configuration (priority: env vars > key file > direct value)
  # Option 1: Environment variable (recommended for development)
  # Option 2: API key file (recommended for production)
  openai_api_key_file: "~/.openai-api-key"
  # Option 3: Direct value (not recommended - use env var or file)
  # openai_api_key: "sk-proj-your-key-here"
```

**Supported Models**:

- `text-embedding-3-small`: 1536 dimensions (recommended, cost-effective,
  compatible with most existing databases)
- `text-embedding-3-large`: 3072 dimensions (higher quality, larger vectors)
- `text-embedding-ada-002`: 1536 dimensions (legacy model, still supported)

**API Key Configuration**:

API keys are loaded in the following priority order (highest to lowest):

1. **Environment variables**:
   ```bash
   # Use PGEDGE-prefixed environment variable (recommended for isolation)
   export PGEDGE_OPENAI_API_KEY="sk-proj-your-key-here"

   # Or use standard environment variable (also supported)
   export OPENAI_API_KEY="sk-proj-your-key-here"
   ```

2. **API key files** (recommended for production):
   ```bash
   # Create API key file
   echo "sk-proj-your-key-here" > ~/.openai-api-key
   chmod 600 ~/.openai-api-key
   ```

3. **Direct configuration value** (not recommended - use env vars or files
   instead)

**Note**: Both `PGEDGE_OPENAI_API_KEY` and `OPENAI_API_KEY` are supported.
The prefixed version takes priority if both are set.

**Pricing** (as of 2025):

- text-embedding-3-small: $0.020 / 1M tokens
- text-embedding-3-large: $0.130 / 1M tokens

### Using Voyage AI (Cloud Embeddings)

**Advantages**: High quality, managed service

**Configuration**:

```yaml
embedding:
  enabled: true
  provider: "voyage"
  model: "voyage-3"  # 1024 dimensions

  # API key configuration (priority: env vars > key file > direct value)
  # Option 1: Environment variable (recommended for development)
  # Option 2: API key file (recommended for production)
  voyage_api_key_file: "~/.voyage-api-key"
  # Option 3: Direct value (not recommended - use env var or file)
  # voyage_api_key: "pa-your-key-here"
```

**Supported Models**:

- `voyage-3`: 1024 dimensions (recommended, higher quality)
- `voyage-3-lite`: 512 dimensions (cost-effective)
- `voyage-2`: 1024 dimensions
- `voyage-2-lite`: 1024 dimensions

**API Key Configuration**:

API keys are loaded in the following priority order (highest to lowest):

1. **Environment variables**:
   ```bash
   # Use PGEDGE-prefixed environment variable (recommended for isolation)
   export PGEDGE_VOYAGE_API_KEY="pa-your-key-here"

   # Or use standard environment variable (also supported)
   export VOYAGE_API_KEY="pa-your-key-here"
   ```

2. **API key files** (recommended for production):
   ```bash
   # Create API key file
   echo "pa-your-key-here" > ~/.voyage-api-key
   chmod 600 ~/.voyage-api-key
   ```

3. **Direct configuration value** (not recommended - use env vars or files
   instead)

**Note**: Both `PGEDGE_VOYAGE_API_KEY` and `VOYAGE_API_KEY` are supported.
The prefixed version takes priority if both are set.

### Using Ollama (Local Embeddings)

**Advantages**: Free, private, works offline

**Configuration**:

```yaml
embedding:
  enabled: true
  provider: "ollama"
  model: "nomic-embed-text"  # 768 dimensions
  ollama_url: "http://localhost:11434"
```

**Supported Models**:

- `nomic-embed-text`: 768 dimensions (recommended)
- `mxbai-embed-large`: 1024 dimensions
- `all-minilm`: 384 dimensions

**Setup**:

```bash
# Install Ollama from https://ollama.com/

# Pull embedding model
ollama pull nomic-embed-text

# Verify it's running
curl http://localhost:11434/api/tags
```

### Database Operation Logging

To debug database connections, metadata loading, and queries, enable structured logging:

```bash
# Set log level
export PGEDGE_DB_LOG_LEVEL="info"    # Basic info: connections, queries, metadata loading, errors
export PGEDGE_DB_LOG_LEVEL="debug"   # Detailed: pool config, schema counts, query details
export PGEDGE_DB_LOG_LEVEL="trace"   # Very detailed: full queries, row counts, timings

# Run the server
./bin/pgedge-postgres-mcp
```

**Log Levels**:

- `info` (recommended): Logs connections, metadata loading, queries with success/failure and timing
- `debug`: Adds pool configuration, schema/table/column counts, and detailed query information
- `trace`: Adds full query text, arguments, and row counts

**Example Output (info level)**:

```
[DATABASE] [INFO] Connection succeeded: connection=postgres:***@localhost:5432/mydb?sslmode=disable, duration=45ms
[DATABASE] [INFO] Metadata loaded: connection=postgres:***@localhost:5432/mydb?sslmode=disable, table_count=42, duration=123ms
[DATABASE] [INFO] Query succeeded: query=SELECT * FROM users WHERE id = $1, row_count=1, duration=5ms
```

### Embedding Generation Logging

To debug embedding API calls and rate limits, enable structured logging:

```bash
# Set log level
export PGEDGE_LLM_LOG_LEVEL="info"    # Basic info: API calls, errors, token usage
export PGEDGE_LLM_LOG_LEVEL="debug"   # Detailed: text length, dimensions, timing, models
export PGEDGE_LLM_LOG_LEVEL="trace"   # Very detailed: full request/response details

# Run the server
./bin/pgedge-postgres-mcp
```

**Log Levels**:

- `info` (recommended): Logs API calls with success/failure, timing, dimensions, token usage, and rate limit errors
- `debug`: Adds text length, API URLs, and provider initialization
- `trace`: Adds request text previews and full response details

**Example Output (info level)**:

```
[LLM] [INFO] Provider initialized: provider=ollama, model=nomic-embed-text, base_url=http://localhost:11434
[LLM] [INFO] API call succeeded: provider=ollama, model=nomic-embed-text, text_length=245, dimensions=768, duration=156ms
[LLM] [INFO] LLM call succeeded: provider=anthropic, model=claude-3-5-sonnet-20241022, operation=chat, input_tokens=100, output_tokens=50, total_tokens=150, duration=1.2s
[LLM] [INFO] RATE LIMIT ERROR: provider=voyage, model=voyage-3-lite, status_code=429, response={"error":...}
```

### Creating a Configuration File

```bash
# Copy the example to the binary directory
cp configs/pgedge-postgres-mcp.yaml.example bin/pgedge-postgres-mcp.yaml

# Edit with your settings
vim bin/pgedge-postgres-mcp.yaml

# Run the server (automatically loads config from default location)
./bin/pgedge-postgres-mcp
```
