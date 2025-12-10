# Changelog

All notable changes to the pgEdge Natural Language Agent will be
documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0-alpha4] - 2025-12-08

### Added

#### Conversation History

- Server-side conversation storage using SQLite database for persistent
  chat history
- REST API endpoints for conversation CRUD operations
  (`/api/conversations/*`)
- Web client conversation panel with list, load, rename, and delete
  functionality
- CLI conversation history commands (`/history`, `/new`, `/save`) when
  running in HTTP mode with authentication
- Automatic provider/model restoration when loading saved conversations
- Database connection tracking per conversation
- History replay with muted colors when loading CLI conversations
- Auto-save behavior in web client after first assistant response

#### Configuration

- Configuration options to selectively enable/disable built-in tools,
  resources, and prompts via the `builtins` section in the config file
- Disabled features are not advertised to the LLM and return errors if
  called directly
- The `read_resource` tool is always enabled as it's required for listing
  resources

#### LLM Provider Improvements

- Dynamic model retrieval for Anthropic provider - available models are
  now fetched from the API instead of being hardcoded
- Display client and server version numbers in CLI startup banner

#### Build & Release

- GitHub Actions workflow for automated release artifact generation
  using goreleaser
- Local verification script for goreleaser artifacts

## [1.0.0-alpha3] - 2025-12-03

### Added

- Web client documentation with screenshots demonstrating all UI features
- Documentation comparing RAG (Retrieval-Augmented Generation) and MCP
  approaches
- Optional Docker container variant with pre-built knowledgebase database
  included

### Changed

#### Naming

- Renamed the server to *pgEdge MCP Server* (from *pgEdge NLA Server*)

#### Knowledgebase System

- `search_knowledgebase` tool now accepts arrays for product and version
  filters, allowing searches across multiple products/versions in a single
  query
- Parameter names changed from `project_name`/`project_version` to
  `project_names`/`project_versions` (arrays of strings)
- Added `list_products` parameter to discover available products and versions
  before searching
- Improved `search_knowledgebase` tool prompt with:
    - Critical warning about exact product name matching at the top
    - Step-by-step workflow guidance (discover products first, then search)
    - Troubleshooting section for zero-result scenarios
    - Updated examples showing realistic product names

### Fixed

- Docker Compose health check now uses correctly renamed binary

## [1.0.0-alpha2] - 2025-11-27

### Added

#### Token Usage Optimization

- Smart auto-summary mode for `get_schema_info` tool when database has >10
  tables
- New `compact` parameter for `get_schema_info` to return minimal output
  (table names + column names only)
- Token estimation and tracking for individual tool calls (visible in debug
  mode)
- Resource URI display in activity log for `read_resource` calls
- Proactive compaction triggered by token count threshold (15,000 tokens)
- Rate limit handling with automatic 60-second pause and retry

#### Prompt Improvements

- Added `<fresh_data_required>` guidance to prompts to prevent LLM from
  using stale information when database state may have changed
- Updated `explore-database` prompt with rate limit awareness and tool
  call budget guidance
- Enhanced prompts guide LLMs to minimize tool calls for token efficiency

#### Multiple Database Support

- Configure multiple PostgreSQL database connections with unique names
- Per-user access control via `available_to_users` configuration field
- Automatic default database selection based on user accessibility
- Runtime database switching in both CLI and Web clients
- Database selection persistence across sessions via user preferences
- CLI commands: `/list databases`, `/show database`, `/set database <name>`
- Web UI database selector in status banner with connection details
- Database switching disabled during LLM query processing to prevent
  data consistency issues
- Improved error messages when no databases are accessible to a user
- API token database binding via `-token-database` flag or interactive
  prompt during token creation

#### Knowledgebase System

- Complete knowledgebase system with SQLite backend for offline
  documentation search
- `search_knowledgebase` MCP tool for semantic similarity search across
  pre-built documentation
- KB builder utility for creating knowledgebase from markdown, HTML,
  SGML, and DocBook XML sources
- Support for multiple embedding providers (Voyage AI, OpenAI, Ollama)
  in knowledgebase
- Project name and version filtering for targeted documentation search
- Independent API key configuration for knowledgebase (separate from
  embedding and LLM sections)
- DocBook XML format support for PostGIS and similar documentation
- Optional project version field in documentation sources

#### LLM Provider Management

- Dynamic Ollama model selection with automatic fallback to available
  models
- Per-provider model persistence in CLI (remembers last-used model for
  each provider)
- Per-provider model persistence in Web UI (using localStorage)
- Automatic preference validation and sanitization on load
- Default provider priority order (Anthropic → OpenAI → Ollama)
- Preferred Ollama models list with tool-calling support verification
- Runtime model validation against provider APIs before selection
- Provider selection now validates that provider is actually configured
- Filtered out Claude Opus models from Anthropic (causes tool-calling
  errors)
- Filtered out embedding, audio, and image models from OpenAI model list

#### Security & Authentication

- Rate limiting for failed authentication attempts (configurable window
  and max attempts)
- Account lockout after repeated failed login attempts
- Per-IP rate limiting to prevent brute force attacks

#### Tools, Resources, and Prompts

- Support for custom user-defined prompts in
  `examples/pgedge-mcp-server-custom.yaml`
- Support for custom user-defined resources in custom definitions file
- New `execute_explain` tool for query performance analysis
- Enhanced tool descriptions with usage examples and best practices
- Added a schema-design prompt for helping design database schemas

### Changed

#### Naming & Organization

- Renamed the project to *pgEdge Natural Language Agent*
- Renamed all binaries and configuration files for consistency:
    - Server: `pgedge-pg-mcp-svr` -> `pgedge-mcp-server`
    - CLI: `pgedge-pg-mcp-cli` -> `pgedge-nla-cli`
    - Web UI: `pgedge-mcp-web` -> `pgedge-nla-web`
    - KB Builder: `kb-builder` -> `pgedge-nla-kb-builder`
- Default server configuration files now use `pgedge-mcp-server-*.yaml` naming
- Default CLI configuration files now uses `pgedge-nla-cli.yaml` naming
- Custom definitions file: `pgedge-mcp-server-custom.yaml`
- Updated all documentation and examples to reflect new naming

#### Configuration

- Reduced default similarity_search token budget from 2500 to 1000
- Default OpenAI model changed from `gpt-5-main` to `gpt-5.1`
- Independent API key configuration for knowledgebase, embedding, and
  LLM sections
- Support for KB-specific environment variables:
  `PGEDGE_KB_VOYAGE_API_KEY`, `PGEDGE_KB_OPENAI_API_KEY`

#### UI/UX Improvements

- Enhanced LLM system prompts for better tool usage guidance
- CLI now saves current model when switching providers
- Web UI correctly remembers per-provider model selections
- Improved error messages and warnings for invalid configurations
- CLI `/tools`, `/resources`, and `/prompts` commands now sort output
  alphabetically
- Web UI favicon added
- Web UI: Moved Clear button from floating position to bottom toolbar
  (next to Settings)
- Web UI: Added Save Chat button to export conversation history as
  Markdown
- Web UI: Improved light mode contrast with gray page background for
  paper effect

### Fixed

- **Critical**: Fixed Voyage AI API response parsing (was expecting flat
  `embedding` field, actual API returns `data[].embedding`)
- **Security**: Custom HTTP handlers (`/api/chat/compact`, `/api/llm/chat`)
  now require authentication when auth is enabled (provider/model listing
  endpoints remain public for login page)
- CLI no longer randomly switches to wrong provider/model on startup
- Invalid provider/model combinations in preferences now automatically
  corrected with warnings
- Web UI model selection now persists correctly across provider switches
- Applied consistent code formatting with `gofmt`
- Removed unused kb-dedup utility
- Fixed gocritic lint warnings
- Fixed data race in rate limiter tests

### Infrastructure

- Docker images updated to Go 1.24
- CI/CD workflows upgraded to Go 1.24 with PostgreSQL 18 testing support
- Start scripts refactored with variable references for improved
  maintainability

## [1.0.0-alpha1] - 2025-11-21

### Added

#### Core Features

- Model Context Protocol (MCP) server implementation
- PostgreSQL database connectivity with read-only transaction
  enforcement
- Support for stdio and HTTP/HTTPS transport modes
- TLS support with certificate and key configuration
- Hot-reload capability for authentication files (tokens and users)
- Automatic detection and handling of configuration file changes

#### MCP Tools (5)

- `query_database` - Execute SQL queries in read-only transactions
- `get_schema_info` - Retrieve database schema information
- `hybrid_search` - Advanced search combining BM25 and MMR algorithms
- `generate_embeddings` - Create vector embeddings for semantic search
- `read_resource` - Access MCP resources programmatically

#### MCP Resources (3)

- `pg://stat/activity` - Current database connections and activity
- `pg://stat/database` - Database-level statistics
- `pg://version` - PostgreSQL version information

#### MCP Prompts (3)

- Semantic search setup workflow
- Database exploration guide
- Query diagnostics helper

#### CLI Client

- Production-ready command-line chat interface
- Support for multiple LLM providers (Anthropic, OpenAI, Ollama)
- Anthropic prompt caching (90% cost reduction)
- Dual mode support (stdio subprocess or HTTP API)
- Persistent command history with readline support
- Bash-like Ctrl-R reverse incremental search
- Runtime configuration with slash commands
- User preferences persistence
- Debug mode with LLM token usage logging
- PostgreSQL-themed UI with animations

#### Web Client

- Modern React-based web interface
- AI-powered chat for natural language database interaction
- Real-time PostgreSQL system information display
- Light/dark theme support with system preference detection
- Responsive design for desktop and mobile
- Token usage display for LLM interactions
- Chat history with prefix-based search
- Message persistence and state management
- Debug mode with toggle in preferences popover
- Markdown rendering for formatted responses
- Inline code block rendering
- Auto-scroll with smart positioning

#### Authentication & Security

- Token-based authentication with SHA256 hashing
- User-based authentication with password hashing
- API token management with expiration support
- File permission enforcement (0600 for sensitive files)
- Per-token connection isolation
- Input validation and sanitization
- Secure password storage in `.pgpass` files
- TLS/HTTPS support for encrypted communications

#### Docker Support

- Complete Docker Compose deployment configuration
- Multi-stage Docker builds for optimized images
- Container health checks
- Volume management for persistent data
- Environment-based configuration
- CI/CD pipeline for Docker builds

#### Infrastructure

- Comprehensive CI/CD with GitHub Actions
- Automated testing for server, CLI client, and web client
- Docker build and deployment validation
- Documentation build verification
- Code linting and formatting checks
- Integration tests with real PostgreSQL databases

#### LLM Proxy

- JSON-RPC proxy for LLM interactions from web clients
- Support for multiple LLM providers
- Request/response logging
- Error handling and status reporting
- Dynamic model name loading for Anthropic
- Improved tool call parsing for Ollama

### Documentation

- Comprehensive user guide covering all features
- Configuration examples for server, tokens, and clients
- API reference documentation
- Architecture and internal design documentation
- Security best practices guide
- Troubleshooting guide with common issues
- Docker deployment guide
- Building chat clients tutorial with Python examples
- Query examples demonstrating common use cases
- CI/CD pipeline documentation
- Testing guide for contributors

[Unreleased]: https://github.com/pgEdge/pgedge-mcp/compare/v1.0.0-alpha4...HEAD
[1.0.0-alpha4]: https://github.com/pgEdge/pgedge-mcp/releases/tag/v1.0.0-alpha4
[1.0.0-alpha3]: https://github.com/pgEdge/pgedge-mcp/releases/tag/v1.0.0-alpha3
[1.0.0-alpha2]: https://github.com/pgEdge/pgedge-mcp/releases/tag/v1.0.0-alpha2
[1.0.0-alpha1]: https://github.com/pgEdge/pgedge-mcp/releases/tag/v1.0.0-alpha1
