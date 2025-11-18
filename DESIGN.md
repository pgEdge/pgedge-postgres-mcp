# Project Design Notes

## Architecture

The project consists of an MCP server that can run in stdio mode or HTTP(S)
SSE mode. At startup, the server connects to the configured PostgreSQL
database which is made available for all client sessions, through the various
tools and resources that are implemented.

The MCP server can use Anthropic, OpenAI, and Ollama LLMs for generating 
embeddings on the fly as needed for natural language similarity search queries.

A command line chat client is implemented in GoLang, with the ability to 
connect to the MCP server over stdio or HTTP/HTTPS.

Additional command line chat clients are implemented in both the documentation
and the examples directory, as *simple* demonstrations of how a developer can
build their own client, using the different communication modes for the MCP
server, and different LLMs.

A web client is included to provide similar functionality to the full featured
command line chat client, via a web interface. The web client uses a three-tier
architecture:

- **Frontend**: React 18 with Material-UI, served via Vite dev server (port 3000 in dev)
- **Backend**: Express.js server (port 3001) for session handling and MCP API proxy
- **MCP Server**: HTTP mode (port 8080) with authentication enabled

The web client uses username/password authentication via the MCP server's
authenticate_user tool, with session tokens stored server-side for security.

## Authentication

In stdio mode, the MCP server doesn't require any authentication.

In HTTP/HTTPS mode, authentication is enabled by default, but can be disabled
in the configuration or on the command line.

Two types of authentication are supported, simultaneously, when authentication
is not explicitly disabled in HTTP/HTTPS mode:

* Service Tokens
* Username and Password

### Service Tokens

Service tokens may be created and maintained using appropriate command line 
options made available in the server binary. They are intended for use by
other long-lived services that may need to use the MCP server.

The service token is passed to the server as a bearer token in HTTP requests,
exactly as it was originally generated.

### Username and Password

End users authenticate by providing a username and password. Users are created
and maintained using appropriate command line options. When a user needs to be
authenticated, the client calls the authenticate_user tool (hidden from the
LLM), which will either return an access denied error, or if the username 
and password are correctly validated, a short-lived session token.

The session token may then be passed to the MCP server with every request, as
a bearer token.

## Per-Token Database Connection Isolation

Both service tokens and session tokens use a token hash to isolate database
connections. When a request arrives with a token (either service or session):

1. The token is hashed using SHA256
2. The hash is stored in the request context
3. The database client manager uses the hash as a key to get or create a
   dedicated database connection for that token
4. All database operations for that request use the isolated connection

This ensures that:
- Different users/services cannot access each other's database connections
- Connection pools are properly managed per token
- Security is maintained through connection isolation
- Resources (like pg://system_info) and tools (like query_database) both work
  correctly with session tokens

  ## LLMs

  The CLI and web client support models from the following providers:

  * Anthropic
  * OpenAI
  * Ollama

  The default model names are (respectively):

  * claude-sonnet-4-5
  * gpt-5.1
  * gemma

The MCP server supports models from the same providers, purely for 
embedding generation tasks. The default model names are (respectively):

* voyage-3
* text-embedding-3-small
* nomic-embed-text