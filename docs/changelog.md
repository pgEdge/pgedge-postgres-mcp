# Changelog

All notable changes to the pgEdge Natural Language Agent will be
documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - Alpha 2

### Added

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

#### Security & Authentication

- Rate limiting for failed authentication attempts (configurable window
  and max attempts)
- Account lockout after repeated failed login attempts
- Per-IP rate limiting to prevent brute force attacks

#### Tools & Resources

- Support for custom user-defined prompts in
  `examples/pgedge-nla-server-custom.yaml`
- Support for custom user-defined resources in custom definitions file
- New `execute_explain` tool for query performance analysis
- Enhanced tool descriptions with usage examples and best practices

### Changed

#### Naming & Organization

- Renamed the project to *pgEdge Natural Language Agent*
- Renamed all binaries and configuration files for consistency:
    - Server: `pgedge-pg-mcp-svr`
    - CLI: `pgedge-pg-mcp-cli`
    - Web UI: `pgedge-mcp-web`
    - KB Builder: `kb-builder`
- Default configuration files now use `pgedge-nla-server-*.yaml` naming
- Custom definitions file: `pgedge-nla-server-custom.yaml`
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

### Fixed

- **Critical**: Fixed Voyage AI API response parsing (was expecting flat
  `embedding` field, actual API returns `data[].embedding`)
- CLI no longer randomly switches to wrong provider/model on startup
- Invalid provider/model combinations in preferences now automatically
  corrected with warnings
- Web UI model selection now persists correctly across provider switches
- Integration tests updated for new tool count (6 tools)
- Applied consistent code formatting with `gofmt`
- Removed unused kb-dedup utility
- Fixed gocritic lint warnings

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

[Unreleased]: https://github.com/pgEdge/pgedge-postgres-mcp/compare/v1.0.0-alpha1...HEAD
[1.0.0-alpha1]: https://github.com/pgEdge/pgedge-postgres-mcp/releases/tag/v1.0.0-alpha1
