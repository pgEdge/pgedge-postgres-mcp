# MCP Expert Documentation

This directory contains comprehensive documentation about the Model Context Protocol (MCP) implementation in the pgEdge Postgres MCP Server project.

## Purpose

This documentation serves as a knowledge base for understanding, maintaining, and extending the MCP server implementation. It was created to enable expert-level guidance on MCP-related development tasks.

## Documentation Files

### 1. [Protocol Implementation](protocol-implementation.md)

**Topics Covered:**
- JSON-RPC 2.0 foundation
- MCP protocol version and compliance
- Request/response structures
- Transport mechanisms (HTTP POST and SSE)
- Token validation and authentication flow
- Error handling and codes
- Security considerations

**When to Read:**
- Understanding how MCP protocol is implemented
- Debugging protocol-level issues
- Adding new MCP methods
- Understanding request flow

### 2. [Tools Catalog](tools-catalog.md)

**Topics Covered:**
- Complete listing of all 28 MCP tools
- Tool categories and organization
- Input schemas and parameter details
- Authorization requirements per tool
- Implementation details and handlers
- Common response formats

**When to Read:**
- Finding what tools are available
- Understanding tool capabilities
- Checking authorization requirements
- Learning tool calling patterns

### 3. [Resources Catalog](resources-catalog.md)

**Topics Covered:**
- Available MCP resources
- URI scheme and addressing
- Resource discovery and reading
- Data structures and formats
- Security considerations
- Adding new resources

**When to Read:**
- Understanding resource system
- Adding new data resources
- Querying available data
- Understanding security model

### 4. [Authentication and Authorization](authentication.md)

**Topics Covered:**
- Multi-tier authentication system
- Token types (service, user, session)
- Token validation process
- Role-based access control (RBAC)
- Group-based privilege system
- Token scoping
- Security best practices

**When to Read:**
- Implementing authentication checks
- Understanding privilege resolution
- Debugging authorization issues
- Designing secure features
- Understanding group membership

### 5. [Extending the MCP Server](extending-mcp.md)

**Topics Covered:**
- Step-by-step guide for adding tools
- Adding resources and prompts
- Modifying existing functionality
- Code patterns and templates
- Testing your changes
- Best practices and checklists

**When to Read:**
- Adding new MCP tools
- Adding new resources
- Modifying existing tools
- Following development workflow

### 6. [Testing MCP Components](testing-mcp.md)

**Topics Covered:**
- Testing philosophy and strategy
- Unit test patterns
- Integration test patterns
- Test helpers and utilities
- Running tests and coverage
- CI/CD integration
- Debugging tests

**When to Read:**
- Writing tests for new features
- Debugging failing tests
- Improving test coverage
- Setting up CI/CD
- Understanding test patterns

## Quick Reference

### Common Tasks

| Task | Primary Document | Related Documents |
|------|-----------------|-------------------|
| Add a new tool | [extending-mcp.md](extending-mcp.md) | [authentication.md](authentication.md), [testing-mcp.md](testing-mcp.md) |
| Add a new resource | [extending-mcp.md](extending-mcp.md) | [resources-catalog.md](resources-catalog.md) |
| Fix authentication issue | [authentication.md](authentication.md) | [protocol-implementation.md](protocol-implementation.md) |
| Understand privilege check | [authentication.md](authentication.md) | [tools-catalog.md](tools-catalog.md) |
| Write tests | [testing-mcp.md](testing-mcp.md) | [extending-mcp.md](extending-mcp.md) |
| Debug protocol error | [protocol-implementation.md](protocol-implementation.md) | [testing-mcp.md](testing-mcp.md) |

### Key Concepts

- **MCP Protocol Version:** 2024-11-05
- **Transport:** HTTP POST at `/mcp` and SSE at `/sse`
- **Authentication:** Bearer token in Authorization header
- **Token Types:** Service tokens, user tokens, session tokens
- **Authorization:** Superuser or group-based privileges
- **Tools:** 28 available (see tools-catalog.md)
- **Resources:** 2 available (users, service-tokens)
- **Prompts:** Not yet implemented

### File Locations

```
/
├── cmd/
│   ├── pgedge-pg-mcp-svr/        # MCP server entry point
│   ├── pgedge-pg-mcp-cli/        # CLI client entry point
│   └── kb-builder/               # Knowledge base builder
├── internal/
│   ├── mcp/
│   │   ├── types.go              # Protocol types and constructors
│   │   ├── server.go             # MCP server and method handlers
│   │   ├── http_server.go        # HTTP/SSE server
│   │   ├── helpers.go            # Helper functions
│   │   ├── constants.go          # Protocol constants
│   │   └── *_test.go             # Unit tests
│   ├── auth/
│   │   └── auth.go               # Authentication (YAML-based)
│   ├── database/
│   │   └── database.go           # Database connection management
│   ├── tools/
│   │   └── definitions.go        # MCP tool definitions
│   ├── resources/
│   │   └── definitions.go        # MCP resource definitions
│   └── ...
└── ...
```

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         MCP Client                          │
│                    (Claude, MCP Inspector)                  │
└─────────────────────────┬───────────────────────────────────┘
                          │ JSON-RPC over HTTP/SSE
                          │ Bearer Token Authentication
┌─────────────────────────▼───────────────────────────────────┐
│                      HTTP/HTTPS Server                      │
│                    (mcp/http_server.go)                     │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Endpoints: /mcp (POST), /sse (SSE), /health       │   │
│  │  Extracts bearer token from Authorization header    │   │
│  └─────────────────────────┬───────────────────────────┘   │
└────────────────────────────┼───────────────────────────────┘
                             │
┌────────────────────────────▼───────────────────────────────┐
│                       MCP Handler                          │
│                    (mcp/server.go)                        │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  1. Parse JSON-RPC request                           │  │
│  │  2. Validate bearer token (multi-source)             │  │
│  │  3. Route to method handler                          │  │
│  │  4. Execute tool/resource/prompt                     │  │
│  │  5. Format JSON-RPC response                         │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  Method Handlers:                                          │
│  ┌─────────────┬──────────────┬───────────────────────┐   │
│  │ initialize  │ tools/list   │ resources/list        │   │
│  │ ping        │ tools/call   │ resources/read        │   │
│  │             │ prompts/list │                       │   │
│  └─────────────┴──────────────┴───────────────────────┘   │
└────────────────────────────┬───────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌───────▼────────┐  ┌────────▼────────┐  ┌───────▼──────────┐
│  Auth Module   │  │   Tools         │  │   Resources      │
│   (auth/)      │  │   (tools/)      │  │   (resources/)   │
│                │  │                 │  │                  │
│ - YAML config  │  │ - query_db      │  │ - pg://schema    │
│ - Token valid  │  │ - get_schema    │  │ - pg://tables    │
│ - User lookup  │  │ - explain_query │  │ - pg://system    │
└───────┬────────┘  └────────┬────────┘  └───────┬──────────┘
        │                    │                    │
        └────────────────────┼────────────────────┘
                             │
                  ┌──────────▼──────────┐
                  │  User PostgreSQL    │
                  │    Databases        │
                  │                     │
                  │ (User-managed DBs   │
                  │  that the MCP       │
                  │  server connects    │
                  │  to on behalf of    │
                  │  authenticated      │
                  │  users)             │
                  └─────────────────────┘
```

## Development Workflow

### 1. Planning

- Review existing tools and patterns
- Design tool/resource/prompt
- Plan authentication and authorization
- Consider security implications

### 2. Implementation

- Register privilege identifier (if needed)
- Add tool/resource definition
- Implement handler function
- Add routing case
- Implement helper functions

### 3. Testing

- Write unit tests
- Write integration tests
- Run tests locally
- Check coverage
- Run linter

### 4. Documentation

- Update appropriate catalog
- Add usage examples
- Document authorization requirements
- Update this README if needed

### 5. Review

- Self-review code
- Check against best practices
- Run full test suite
- Request peer review

## Best Practices Summary

### Security

1. **Always authenticate** - Except for explicitly public endpoints
2. **Always authorize** - Check permissions before executing
3. **Validate all input** - Never trust client data
4. **Use parameterized queries** - Prevent SQL injection
5. **Never log secrets** - Tokens, passwords, etc.
6. **Use HTTPS** - In production environments

### Code Quality

1. **Follow four-space indentation** - Project standard
2. **Add copyright headers** - All source files
3. **Write descriptive names** - Functions, variables, types
4. **Keep functions focused** - Single responsibility
5. **Handle errors properly** - Don't ignore errors
6. **Add comments** - For complex logic

### Testing

1. **Test before merging** - All tests must pass
2. **Achieve good coverage** - 80%+ target
3. **Test error cases** - Not just happy path
4. **Clean up test data** - Use defer for cleanup
5. **Make tests independent** - Order shouldn't matter
6. **Use table-driven tests** - For multiple cases

## Contributing

When contributing to MCP server development:

1. Read relevant documentation first
2. Follow the patterns established in existing code
3. Write tests for all new functionality
4. Update documentation when adding features
5. Request code review before merging

## Getting Help

If you have questions:

1. **Search this documentation** - Use your editor's search
2. **Check existing code** - Look for similar patterns
3. **Review tests** - See how features are tested
4. **Check MCP specification** - https://modelcontextprotocol.io/
5. **Ask for review** - From another developer

## Maintenance

This documentation should be updated when:

- New tools are added
- New resources are added
- Authentication/authorization changes
- Protocol version changes
- Best practices change
- Major refactoring occurs

---

**Last Updated:** 2025-11-08
**MCP Protocol Version:** 2024-11-05
**Server Version:** 1.0.0
