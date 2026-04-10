# Changelog

All notable changes to the pgEdge Natural Language Agent will be
documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Schema metadata cache now refreshes automatically based on a
  configurable TTL. The `metadata_ttl` database option controls
  how long cached metadata remains valid (default: 5 minutes).
  This fixes an issue where `get_schema_info` returned stale
  results when tables were created outside the MCP server or
  when using read-only database connections.

## [1.0.0] - 2026-03-27

### Changed

- Docker container now defaults to stdio mode instead of HTTP
  mode. HTTP mode requires setting `PGEDGE_HTTP_ENABLED=true`.
  This allows the Docker image to work with stdio-based MCP
  clients such as the Docker Desktop MCP Toolkit, Claude Code,
  and Claude Desktop.

- Docker init script output now goes to `stderr` instead of
  `stdout`; this keeps `stdout` clean for the MCP protocol in
  stdio mode.

- User and token initialization (`INIT_USERS`, `INIT_TOKENS`)
  now only runs when HTTP mode is enabled. Stdio mode does not
  use HTTP authentication.

- Quickstart demo files (`docker-compose.yml`, `.env.example`,
  `pgedge-ait-demo.sh`) are now served from the GitHub
  repository instead of `downloads.pgedge.com`. The Northwind
  example database download is unchanged.

### Fixed

- Queries with trailing semicolons no longer produce a SQL syntax
  error when the server auto-appends a `LIMIT` clause. The server
  now strips trailing semicolons before appending `LIMIT`/`OFFSET`.

- MCP tools (`query_database`, `count_rows`, `get_schema_info`) now
  load metadata synchronously on the first call instead of returning
  a "database is still initializing" error. This eliminates the
  unnecessary LLM retry that previously occurred on every first
  tool call.

- Database connection timeout now defaults to 10 seconds instead of
  blocking for 60+ seconds when a target host is unreachable. A new
  `connect_timeout` configuration option allows customization of
  the timeout duration.

### Security

- The server now rejects queries that reference the
  `transaction_read_only` or `default_transaction_read_only`
  settings when the database connection is in read-only mode.
  This prevents single-statement bypass attacks (such as
  PL/pgSQL `DO` blocks with `set_config()`) that could
  circumvent the `SET TRANSACTION READ ONLY` guardrail.

- The system prompt sent to all LLM providers (Anthropic,
  OpenAI, Ollama) now includes explicit safety instructions
  that forbid attempts to bypass read-only mode when the
  database connection does not allow writes.

### Added

- MCP tool selection guidance for AI agents. The server now sends
  a server-level `instructions` field during the MCP initialize
  handshake, directing agents to prefer MCP tools over `psql` and
  shell commands. Tool descriptions include explicit "use this
  instead of..." language to steer tool selection. A new
  documentation page covers recommended `CLAUDE.md` and
  `.cursorrules` configuration for reinforcing tool preference.

- Cursor IDE plugin manifest and setup guide.

- OpenAPI 3.0.3 specification and interactive API browser. The
  server now provides a programmatic OpenAPI specification covering
  all REST endpoints. The specification is available at the
  `/api/openapi.json` endpoint (no authentication required),
  through the `-openapi` CLI flag, and as a static file in the
  documentation. Use `make openapi` to regenerate the static
  copy. The MkDocs site includes a ReDoc-powered interactive API
  browser under For Developers. API responses include an RFC 8631
  `Link` header for automatic discovery by tools such as
  `restish`.

- Write query confirmation in the CLI and Web UI. When a database
  has write access enabled, the user is prompted to confirm DDL and
  DML queries before the server executes the queries. Declining a
  query returns an error to the LLM without executing the query.

- MCP tool annotations on the `query_database` tool. The server
  sets `destructiveHint` and `readOnlyHint` annotations per the
  MCP 2025-03-26 specification; third-party MCP clients that
  support annotations can use the annotations to prompt for user
  confirmation.

- Partitioned table support in `get_schema_info`. The tool now
  recognizes partitioned parent tables (shown as `PARTITIONED TABLE`)
  and hides child partitions by default. Use the new
  `include_partitions` parameter to reveal child partitions when
  needed. This reduces context window usage for databases with
  daily or other time-based partitioning schemes.

- Example DBA toolkit as a drop-in YAML custom definitions file
  (`examples/pgedge-postgres-mcp-dba.yaml`). The toolkit provides
  three pl-do tools: `get_top_queries` for top resource-consuming
  query analysis, `analyze_db_health` for seven-category database
  health checks, and `recommend_indexes` for two-tier index
  recommendations with optional HypoPG simulation.

- Multi-host database connection support for high availability
  and failover. The `hosts` array replaces the single `host` and
  `port` fields when connecting to multiple PostgreSQL servers.
  The server generates a libpq-compatible multi-host connection
  string and passes the list to pgx for automatic failover.

- The `target_session_attrs` option controls read-write routing
  for multi-host connections. Accepted values include
  `any`, `read-write`, `read-only`, `primary`, `standby`, and
  `prefer-standby`.

- Pool health check and connection lifetime settings for
  database connections. The `pool_health_check_period` option
  sets the interval for background health checks; the
  `pool_max_conn_lifetime` option sets the maximum age of a
  pooled connection before the server closes the connection.

- The `PGEDGE_DB_HOSTS` environment variable configures
  multi-host connections as a comma-separated list of
  `host:port` pairs.

- The `--db-hosts` CLI flag specifies multiple database hosts
  as a comma-separated `host:port` list. The
  `--db-target-session-attrs` flag sets the session routing
  attribute for multi-host connections.

- Configuration validation rejects entries that specify both
  the single-host `host` field and the multi-host `hosts`
  array. The validator also checks that `target_session_attrs`
  contains a recognized value.

- The web client displays multi-host connection details in the
  database selector, showing each configured host and port
  alongside the connection status.

- GitHub Codespaces demo environment for one-click evaluation of
  the MCP server in a browser-based development environment.

- One-command installers for Claude Code and Claude Desktop that
  automate binary download, configuration generation, and client
  registration.

- `--max-retries` flag for the kb-builder controls how many times
  transient embedding API errors are retried. The default is 5;
  set to 0 for unlimited retries. Backoff is capped at 60 seconds.

- Connection status field (`connected` or `unavailable`) in the
  `list_database_connections` tool response; databases with status
  `unavailable` are connected on demand when selected.

- Trace file logging for deep diagnostics of MCP interactions.
  Enable with `-trace-file <path>`, the `trace_file` configuration
  option, or the `PGEDGE_TRACE_FILE` environment variable. The
  server writes JSONL entries for tool calls, resource reads, prompt
  executions, HTTP requests, LLM interactions, database switches,
  configuration reloads, and session events.

### Internal

- Replaced the CGO SQLite driver with a pure Go driver, enabling
  fully static binaries without a C compiler dependency.

### Improved

- Expanded the configuration reference in `configuration.md` with
  database connection options (`allow_writes`, `allow_llm_switching`,
  `allowed_pl_languages`, pool settings, and access control), LLM
  proxy options, and previously undocumented CLI flags (`-debug`,
  `-db-*`, user management flags). Added missing entries to the
  environment variable reference and example configuration file.

- Comprehensive documentation expansion to improve Context7 benchmark
  coverage across all ten benchmark categories:

    - New `row-level-security.md` guide covering PostgreSQL RLS/CLS
      integration with the MCP server, including per-user database
      connections, session variable patterns, column-level security
      with grants and views, and a multi-tenant worked example.

    - New `distributed-deployment.md` guide covering multi-instance
      deployment with shared filesystem and object storage patterns,
      nginx and AWS ALB load balancer configuration, Docker Compose
      multi-instance example, Kubernetes deployment with ConfigMap
      and init containers, and knowledge base synchronization.

    - New `custom-knowledgebase-tutorial.md` with an end-to-end
      tutorial for building custom knowledge bases from domain
      documentation, including schema documentation patterns,
      business rules glossaries, KB builder configuration, and
      the internal SQLite database schema.

    - New `client-examples.md` with complete Python and JavaScript
      client implementations covering authentication, schema
      retrieval, query execution, database switching, TSV parsing,
      knowledgebase search, token lifecycle management, and error
      handling with automatic retry.

    - New `error-reference.md` documenting all HTTP status codes,
      JSON-RPC error codes, authentication errors, tool-specific
      errors, database access errors, and troubleshooting steps.

    - Expanded `claude_desktop.md` with a getting started guide,
      build instructions, YAML configuration examples, natural
      language query flow explanation, command-line flags reference,
      setup verification checklist, and detailed troubleshooting.

    - Expanded `authentication.md` with a database access control
      section documenting `available_to_users` authorization, a
      per-token database binding section, an authorization model
      summary, and a token lifecycle management section covering
      expiration detection, automatic re-authentication, and best
      practices for programmatic clients.

    - Expanded `api-reference.md` with schema retrieval examples
      including `curl` commands with authentication, TSV response
      parsing in Python and JavaScript, query execution examples
      with comprehensive error handling and retry logic, a query
      error reference table, and result format documentation.

    - Expanded `deploy_docker.md` with a complete consolidated
      `docker-compose.yml`, a full environment variable reference,
      a quick start guide, and Docker health check documentation.

    - Expanded `multiple_db_config.md` with Python and JavaScript
      client integration examples, access denied error handling,
      and a configuration settings reference clarifying the
      relationship between `llm_connection_selection` and
      `allow_llm_switching`.

### Fixed

- The server no longer exits when configured databases are
  unreachable at startup ([#82](https://github.com/pgEdge/pgedge-postgres-mcp/issues/82)).
  In STDIO mode, each database connection is now attempted
  independently; failures are logged as warnings and the server
  starts with whichever databases are reachable. Unreachable
  databases are connected on demand when a tool or the user
  selects them. The `list_database_connections` tool reports
  each database as `connected` or `unavailable`.

- Closed database clients are no longer returned from the client
  manager cache. Previously, background cleanup or database switching
  could close a client while tool registries still held a reference,
  causing intermittent "Connection pool not found" errors. Retrieval
  points now check a closed flag and transparently create a fresh
  client when needed.

- The `default_transaction_read_only` session parameter is now set
  with a `SET` command after connection instead of as a startup
  parameter. Connection poolers such as PgBouncer and HAProxy do
  not support arbitrary startup parameters; the previous approach
  caused connections to fail with an "unsupported startup parameter"
  error.

- Ollama embedding generation no longer retries or fails the entire
  batch when a chunk exceeds the model's context length. The builder
  detects the error immediately, progressively truncates the text at
  word boundaries (75 %, 50 %, 25 %), and skips the chunk with a
  warning if all attempts fail.

- Custom `pl-do` and `pl-func` tools no longer appear in `tools/list`
  when their language is not in `allowed_pl_languages`. Previously the
  language check only happened at execution time; the server now filters
  PL tools at registration time so clients only see tools they can use.

- Fixed PL/Perl custom tools (`pl-func` and `pl-do`) failing with
  "Unable to load JSON.pm into plperl" when using trusted `plperl`.
  Trusted `plperl` cannot load external Perl modules, so the wrapper
  now uses PostgreSQL's `jsonb_each_text()` via SPI to parse arguments
  instead of `JSON.pm`. Untrusted `plperlu` continues to use `JSON.pm`
  as before.

- Fixed Web GUI losing connection when switching between databases. The
  server now returns proper JSON error responses when the database is
  temporarily unavailable during switching, and the client handles these
  transient states gracefully with automatic retry logic instead of showing
  a disconnection error.

- SIGHUP configuration reload now invalidates stale database
  connections. Previously, reloading the configuration did not close
  connections whose parameters had changed, leaving the server with
  outdated connection settings until restart.

- The `-add-user`, `-add-token`, and related user/token management commands
  now respect the `user_file` and `token_file` paths from the server
  configuration file. Previously, these commands used hardcoded default paths
  regardless of the configuration, which could cause users or tokens to be
  added to the wrong file when custom paths were configured. The commands use
  the priority order: CLI flag > config file > default path. When `-user-file`
  or `-token-file` is explicitly provided on the command line, no configuration
  file is required (except for `-add-token` which needs database names from
  the config). This allows Docker containers and scripts to use these commands
  without a configuration file by specifying paths directly.

## [1.0.0-beta3] - 2026-01-21

### Added

#### Custom Tools

- New custom tools feature for defining database operations as callable MCP
  tools via YAML configuration
- Three tool types are supported:
    - `sql`: Execute parameterized SQL queries with `$1`, `$2`, etc.
      placeholders
    - `pl-do`: Execute PL/* DO blocks (anonymous functions) with automatic
      result handling via `set_config`/`current_setting`
    - `pl-func`: Create temporary PL/* functions with proper RETURN types
- Security controls via `allowed_pl_languages` configuration per database to
  restrict which procedural languages can be used
- Language support includes plpgsql, plpython3u, plv8, and plperl with
  automatic code wrapping and `mcp_return()` helper function
- Configurable per-tool timeout support
- Comprehensive validation of tool definitions at startup

#### LLM Database Connection Switching

- New `list_database_connections` tool allows LLMs to discover available
  database connections
- New `select_database_connection` tool allows LLMs to switch between databases
  during a conversation
- New `llm_connection_selection` configuration option to enable/disable the
  feature (disabled by default for security)
- New `allow_llm_switching` per-database option to exclude specific connections
  from LLM switching (defaults to true when feature is enabled)
- Real-time UI updates in web client when LLM switches databases
- CLI notification message when LLM switches databases

#### Prompt Argument Types

- Prompt arguments now support a `type` field with values `string` (default)
  or `boolean`
- Boolean arguments render as toggle switches in the web GUI instead of text
  fields
- Custom prompts in YAML can specify argument types for improved UI rendering

### Fixed

- The conversation history panel is now expanded by default when the web GUI
  loads, improving accessibility to past conversations.

- Fixed Web GUI database switching causing JSON parse error and disconnect loop.
  The `selectDatabase` function in `useDatabases.js` now checks `response.ok`
  before parsing the response as JSON; the auth middleware and database API
  handlers now return consistent JSON error responses instead of plain text.

- Improved login error messages in the web GUI. Authentication failures now
  display user-friendly messages like "Invalid username or password. Please
  try again." instead of technical RPC error codes.

- Standardized default configuration file paths for consistency. All config
  files now use the `postgres-mcp` prefix and search `/etc/pgedge/` first:
    - Config: `postgres-mcp.yaml` (previously `pgedge-postgres-mcp.yaml`)
    - Tokens: `postgres-mcp-tokens.yaml` (previously `pgedge-postgres-mcp-tokens.yaml`)
    - Users: `postgres-mcp-users.yaml` (previously `pgedge-postgres-mcp-users.yaml`)
    - Secret: `postgres-mcp.secret` (previously `pgedge-postgres-mcp.secret`)

- Improved error messages when the MCP server is unavailable. The web GUI now
  displays user-friendly messages for 502/503/504 errors instead of showing
  raw HTML error pages from the proxy.

- Fixed DDL and DML statements silently failing when `allow_writes` is enabled.
  The `query_database` tool now uses `tx.Exec()` for DDL (CREATE, DROP, ALTER,
  TRUNCATE) and DML (INSERT, UPDATE, DELETE) statements instead of `tx.Query()`,
  which could cause statements to not execute properly due to pgx's prepared
  statement caching behavior. DML statements with RETURNING clauses continue
  to use `tx.Query()` to capture returned rows.

## [1.0.0-beta2] - 2026-01-13

### Added

#### Write Access Mode

- New `allow_writes` configuration option for database connections
    - Disabled by default (read-only mode) for safety
    - When enabled, allows the LLM to execute DDL (CREATE, DROP, ALTER) and
      DML (INSERT, UPDATE, DELETE) statements
    - Automatic schema metadata refresh after DDL operations to keep
      `get_schema_info` results current
- Visual warnings for write-enabled databases:
    - Web client: Prominent amber warning banner when connected to a
      write-enabled database
    - Web client: Warning chip indicator in database selector popover
    - CLI: `[WRITE-ENABLED]` indicator in `/list databases` output
    - CLI: Warning message when switching to a write-enabled database
- Added `allow_writes` field to `pg://system_info` resource output
- Updated `query_database` tool description to dynamically indicate
  write access status

#### Token Management

- New `count_rows` tool for lightweight row counting before querying large
  tables
- Pagination support (`offset` parameter) in `query_database` tool for paging
  through large result sets
- Truncation detection in query results (fetches limit+1 rows to show "more
  data available" indicator)

#### Configuration Templates

- Added example configuration files in `examples/` directory:
    - `pgedge-postgres-mcp-http.yaml.example` - MCP server HTTP mode config
    - `pgedge-postgres-mcp-stdio.yaml.example` - MCP server stdio mode config
    - `pgedge-nla-cli-http.yaml.example` - CLI client HTTP mode config
    - `pgedge-nla-cli-stdio.yaml.example` - CLI client stdio mode config
    - `postgres-mcp-users.yaml.example` - User authentication template
    - `postgres-mcp-tokens.yaml.example` - Token authentication template

#### CLI Features

- Added `-mcp-server-config` command line flag for specifying the MCP server
  config file path in stdio mode

#### CI/CD

- Claude PR review GitHub Action workflow for automated code reviews
- CodeRabbit configuration for additional PR analysis

#### Knowledgebase Builder

- Hybrid chunking algorithm for improved RAG quality:

    - Two-pass algorithm: Pass 1 splits at semantic boundaries, Pass 2 merges
      undersized chunks
    - Structural element preservation: Code blocks, tables, lists, and
      blockquotes are kept intact when possible
    - Full heading hierarchy tracking: Chunks include breadcrumb context
      (e.g., "API Reference > Authentication > OAuth")
    - Smart splitting for oversized elements: Large code blocks split at line
      boundaries, tables at row boundaries, paragraphs at sentence boundaries
    - Chunk metadata now includes `HeadingPath` (full hierarchy) and
      `ElementTypes` (structural element types in chunk)

- Maintains Ollama compatibility with existing size limits (300 words / 3000
  chars)

### Changed

#### CLI Command Consistency

- Simplified LLM command names:
    - `/set llm-provider` → `/set provider`
    - `/set llm-model` → `/set model`
    - `/show llm-provider` → `/show provider`
    - `/show llm-model` → `/show model`
- Moved standalone listing commands under `/list`:
    - `/tools` → `/list tools`
    - `/resources` → `/list resources`
    - `/prompts` → `/list prompts`
- Added `/list providers` command to list available LLM providers
- Reorganized `/help` output into logical sections

#### Token Efficiency

- Query results now returned in TSV format instead of JSON for better token
  efficiency
- Custom SQL resource data returned in TSV format
- `get_schema_info` tool returns results in TSV format with additional relevant
  information and supports more targeted calls
- Removed redundant resource for retrieving schema info

#### Model Selection

- Model family matching when reloading saved conversations (handles
  date-suffixed model names like `claude-opus-4-5-20251101`)
- Web UI now uses family matching for model selection persistence
- CLI now restores database preference on load
- Added debug messages for model loading troubleshooting

#### Documentation

- Comprehensive documentation restructuring for online publication
- Added configuration setup instructions to README Web Client and CLI sections
- Added Quickstart guide
- Updated security documentation
- Added conversations API and database selection API documentation
- Fixed various documentation formatting issues and environment variable
  references

#### Docker

- Renamed `mcp-server` to `postgres-mcp` in Docker configuration (#12)

### Fixed

- CLI preference saving now works correctly
- Fixed test expecting wrong number of resources (1 instead of 2)
- Updated tests to expect 7 tools after count_rows addition
- Various typo fixes in documentation and configuration

## [1.0.0-beta1] - 2025-12-15

### Changed

This release marks the transition from alpha to beta status, indicating the
software is now feature-complete and ready for broader testing.

#### Internal

- Updated Claude Code configuration

## [1.0.0-alpha6] - 2025-12-12

### Added

#### CLI Features

- Added `none` authentication mode for CLI client to connect to servers with
  authentication disabled (`-mcp-auth-mode none`)

#### Knowledgebase

- Release workflow now builds kb.db with embeddings from all three providers
  (OpenAI, Voyage AI, Ollama)

### Changed

#### Naming

- Renamed server binary from `pgedge-postgres-mcp` to `pgedge-nla-svr`

#### Knowledgebase Builder

- Reduced chunk sizes to avoid hitting Ollama model token limits (250 words
  target, 300 max)
- Added character-based chunk limiting (3000 chars max) for technical content
  with high character-to-word ratios (XML/SGML)
- Improved markdown cleanup when building knowledgebase (removes images, link
  URLs, simplifies table borders)
- Added ASCII table border simplification to reduce token usage

### Fixed

- Fixed lint warnings in test files (unused types and unusedwrite warnings)
- Fixed tests that failed without database connection
- Fixed git branch handling when building knowledgebase (uses checkout -B to
  handle behind branches)
- Improved git pull handling when checking out branches for knowledgebase
  building

### Infrastructure

- Added Claude Code instructions file for development workflow

## [1.0.0-alpha5] - 2025-12-11

### Added

#### CLI Features

- Ability to cancel in-flight LLM queries by pressing Escape key
- Support for enabling/disabling colorization via configuration
- Terminal sanitization at startup to recover from broken terminal states

#### Web UI

- Login page animation

### Changed

#### Cross-Platform Compatibility

- Refactored syscall package usage into platform-specific files for proper
  cross-platform support (darwin, linux, windows)
- Improved terminal raw mode handling in escape key detection

#### UI Improvements

- Updated web UI styling to better match pgEdge Cloud design
- Removed unnecessary checks for LLM environment variables

### Fixed

#### Critical Bug Fixes

- **CLI Output Bug**: Fixed staircase indentation issue where CLI output
  progressively indented to the right
- **Terminal State**: Fixed terminal being left in broken state after CLI
  exit due to raw mode not being properly restored
- **Compaction Bug**: Fixed tool_use and tool_result messages being
  separated during conversation compaction, which caused Anthropic API
  errors (400 Bad Request with "tool_use_id not found")

#### Other Fixes

- Fixed first load of a conversation not displaying correctly in web UI
- Fixed broken documentation URLs in README after docs restructuring

### Infrastructure

- GitHub Actions workflow improvements:
    - Build kb.db using goreleaser
    - Include kb.db in kb-builder archive
    - Use token to pull private repos
    - Create bin directory before using it in release workflow
    - Fix build command issues
    - Fix dirty git state error in workflow
    - Use architecture-specific runners for release builds

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
  `examples/pgedge-postgres-mcp-custom.yaml`
- Support for custom user-defined resources in custom definitions file
- New `execute_explain` tool for query performance analysis
- Enhanced tool descriptions with usage examples and best practices
- Added a schema-design prompt for helping design database schemas

### Changed

#### Naming & Organization

- Renamed the project to *pgEdge Natural Language Agent*
- Renamed all binaries and configuration files for consistency:
    - Server: `pgedge-pg-mcp-svr` -> `pgedge-postgres-mcp`
    - CLI: `pgedge-pg-mcp-cli` -> `pgedge-nla-cli`
    - Web UI: `pgedge-mcp-web` -> `pgedge-nla-web`
    - KB Builder: `kb-builder` -> `pgedge-nla-kb-builder`
- Default server configuration files now use `pgedge-postgres-mcp-*.yaml` naming
- Default CLI configuration files now uses `pgedge-nla-cli.yaml` naming
- Custom definitions file: `pgedge-postgres-mcp-custom.yaml`
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
- CLI `/list tools`, `/list resources`, and `/list prompts` commands now
  sort output alphabetically
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

[Unreleased]: https://github.com/pgEdge/pgedge-nla/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/pgEdge/pgedge-nla/compare/v1.0.0-beta3...v1.0.0
[1.0.0-beta3]: https://github.com/pgEdge/pgedge-nla/releases/tag/v1.0.0-beta3
[1.0.0-beta2]: https://github.com/pgEdge/pgedge-nla/releases/tag/v1.0.0-beta2
[1.0.0-beta1]: https://github.com/pgEdge/pgedge-nla/releases/tag/v1.0.0-beta1
[1.0.0-alpha6]: https://github.com/pgEdge/pgedge-nla/releases/tag/v1.0.0-alpha6
[1.0.0-alpha5]: https://github.com/pgEdge/pgedge-nla/releases/tag/v1.0.0-alpha5
[1.0.0-alpha4]: https://github.com/pgEdge/pgedge-postgres-mcp/releases/tag/v1.0.0-alpha4
[1.0.0-alpha3]: https://github.com/pgEdge/pgedge-postgres-mcp/releases/tag/v1.0.0-alpha3
[1.0.0-alpha2]: https://github.com/pgEdge/pgedge-postgres-mcp/releases/tag/v1.0.0-alpha2
[1.0.0-alpha1]: https://github.com/pgEdge/pgedge-postgres-mcp/releases/tag/v1.0.0-alpha1
