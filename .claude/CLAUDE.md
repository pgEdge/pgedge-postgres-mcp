# Claude Standing Instructions

> Standing instructions for Claude Code when working on this project.

## Project Structure

The pgEdge Postgres MCP Server is a single Go project with an embedded
React web client:

- `/cmd` - Command-line entry points (server, CLI client, KB builder).

- `/internal` - Core Go packages (MCP server, tools, resources, auth, chat).

- `/web` - Web client application (React/JavaScript).

- `/docs` - Documentation in markdown format with lowercase filenames.

- `/examples` - Example configuration files and demos.

## Key Files

Reference these files for project context:

- `docs/changelog.md` - Notable changes by release.

- `mkdocs.yml` - Documentation site navigation.

- `Makefile` - Build and test commands.

- `.claude/CLAUDE.md` - This file; project guidelines.

## Sub-Agents

Specialized sub-agents in `/.claude/agents/` handle complex domain tasks.
Use sub-agents proactively to preserve context in the main conversation.

### When to Use Sub-Agents

**Proactively delegate to sub-agents** for:

- Any Go implementation task (use **golang-expert**)
- Any React/JavaScript implementation task (use **react-expert**)
- Complex domain questions requiring research
- Tasks that would consume significant context in the main conversation

Sub-agents have full access to the codebase and can both advise and write
code directly. Delegating implementation work preserves context in the main
conversation for coordination and higher-level decisions.

### Available Sub-Agents

**Implementation Agents** (can write code):

- **golang-expert** - Go development: features, bugs, architecture, review.

- **react-expert** - React/MUI development: components, features, bugs.

- **documentation-writer** - Documentation following project style guide.

**Advisory Agents** (research and recommend):

- **postgres-expert** - PostgreSQL administration, tuning, troubleshooting.

- **mcp-server-expert** - MCP protocol, tool implementation, debugging.

- **testing-framework-architect** - Test strategies for Go and React.

- **security-auditor** - Security review, vulnerability detection, OWASP.

- **code-reviewer** - Code quality, bug detection, anti-patterns.

- **codebase-navigator** - Finding code, tracing data flow, structure.

Each sub-agent has a knowledge base in `/.claude/<agent-name>/` containing
domain-specific patterns and project conventions.

## Task Workflow

Follow this workflow for implementation tasks:

1. Read relevant code before proposing changes.

2. Use sub-agents for complex domain questions.

3. Run `make test` before marking implementation complete.

4. Review security implications for auth, input handling, or query changes.

5. Update `docs/changelog.md` for user-facing changes.

## General

- Always kill the MCP server and Vite server when no longer needed if they
  have been started to test something.

## Documentation

### General Guidelines

- Place comprehensive documentation in `/docs`.

- Create an `index.md` as the entry point; link to this from the README.

- Wrap all markdown files at 79 characters or less.

- Use lowercase filenames for all files in `/docs`.

### Writing Style

- Use active voice.

- Write grammatically correct sentences between 7 and 20 words.

- Use semicolons to link related ideas or manage long sentences.

- Use articles (a, an, the) appropriately.

- Avoid ambiguous pronoun references; only use "it" when the referent is
  in the same sentence.

### Document Structure

- Use one first-level heading per file with multiple second-level headings.

- Limit third and fourth-level headings to prominent content only.

- Include an introductory sentence or paragraph after each heading.

- For Features or Overview sections, use the format: "The MCP Server
  includes the following features:" followed by a bulleted list.

### Lists

- Leave a blank line before the first item in any list or sub-list.

- Write each bullet as a complete sentence with articles.

- Do not bold bullet items.

- Use numbered lists only for sequential steps.

### Code Snippets

- Precede code with an explanatory sentence: "In the following example,
  the `command_name` command uses..."

- Use backticks for inline code: `SELECT * FROM table;`

- Use fenced code blocks with language tags for multi-line code:

  ```sql
  SELECT * FROM code;
  ```

- Format `stdio`, `stdin`, `stdout`, and `stderr` in backticks.

- Capitalise SQL keywords; use lowercase for variables.

### Links and References

- Link files outside `/docs` to their GitHub location.

- Include third-party installation/documentation links in Prerequisites.

- Link to the GitHub repo when referencing cloning or project work.

- Do not link to github.io.

### README.md Files

At the top of each README:

- GitHub Action badges for repository actions.

- Table of Contents mirroring the `mkdocs.yml` nav section.

- Link to online docs at docs.pgedge.com.

README body content:

- Getting started steps.

- Prerequisites with commands and third-party links.

- Build/install commands and minimal configuration notes.

At the end of each README:

- Issues link: "To report an issue with the software, visit:"

- Online docs link: "For more information, visit
  [docs.pgedge.com](https://docs.pgedge.com)"

- License (final line): "This project is licensed under the
  [PostgreSQL License](LICENSE.md)."

### Additional Documentation Requirements

- Match all sample output to actual output.

- Document all command-line options.

- Include well-commented examples for all configuration options.

- Keep documentation synchronized with code for CLI options, configuration,
  and environment variables.

- Update `changelog.md` with notable changes since the last release.

## Versioning

A release must update the version in every place below. **Treat this list
as a starting point and not as authoritative**; it records where versions
lived when it was written, and a new one can appear at any time without
this file being updated. Always finish with a manual check, searching the
tree for the outgoing version string and reading what each hit means:

```bash
git grep -n "1\.2\.3"   # the version being replaced
```

The minimal set of places to update:

- `web/package.json`, along with the `version` fields in
  `web/package-lock.json`. Run `npm version <version> --no-git-tag-version`
  in `web/` rather than editing them by hand.

- `internal/mcp/server.go`, in the `ServerVersion` constant. The server
  reports it over MCP `initialize`, on `/health`, and in the OpenAPI
  specification.

- `internal/chat/mcp_client.go`, in the `ClientVersion` constant. The CLI
  reports it in `--version` and its welcome banner.

- `docs/changelog.md`, which needs a new release heading with the date, and
  the entries moved out of `[Unreleased]`.

- `examples/helm/pgedge-nla/Chart.yaml`, in both `version` and
  `appVersion`.

- The git tag itself, which is what the release workflow and the RPM and
  Debian packaging derive their version from: `pkg/build-rpm.sh` and
  `pkg/build-deb.sh` read it out of the tag, so neither needs editing.
  Note that `pkg/common.sh` carries a hardcoded fallback for when
  `COMPONENT_VERSION` is unset, which affects local package builds only;
  update it if you rely on those.

Two things deliberately need no update, and should not be given one:

- The web client's `CLIENT_VERSION` in `web/src/lib/mcp-client.js` is
  injected from `web/package.json` by the `define` in `vite.config.js` and
  `vitest.config.js`. Bumping the manifest is enough. It was previously a
  literal, which is exactly how it came to sit at `1.0.0-alpha5` through
  five subsequent releases whilst every other version moved on.

- The `version` label in `Dockerfile.cli`, `Dockerfile.server`, and
  `Dockerfile.web` carries only `major.minor`, so a patch release leaves it
  alone.

## Tests

- Provide unit and integration tests for Go packages.

- Execute tests with `go test` or `npm test` (for web client).

- Write automated tests for all functions and features; use mocking where
  needed.

- Run all tests after any changes; check for errors and warnings that may
  be hidden by output redirection or truncation.

- Clean up temporary test files on completion; retain log files for
  debugging.

- Modify existing tests only when the tested functionality changes or to
  fix bugs.

- Include linting in standard test suites using locally installable tools.

- Enable coverage checking in standard test suites.

- Run `gofmt` on all Go files.

- Ensure `make test` runs all test suites.

- Do not skip database tests when testing changes.

## Security

- Maintain isolation between user sessions.

- Restrict database connections to their owning users or tokens.

- Protect against injection attacks at client and server; the exception is
  MCP tools that execute arbitrary SQL queries.

- Follow industry best practices for defensive secure coding.

- Review all changes for security implications; report potential issues.

- Enforce authentication when enabled; never bypass auth checks.

- Maintain per-token connection isolation.

- Respect token expiry settings; validate tokens before allowing access.

## Code Style

- Use four spaces for indentation.

- Write readable, extensible, and appropriately modularised code.

- Minimise code duplication; refactor as needed.

- Follow language-specific best practices.

- Remove unused code.

- Include this copyright notice at the top of every source file (not
  configuration files); adjust comment style for the language:

  ```
  /*-------------------------------------------------------------------------
   *
   * pgEdge Natural Language Agent
   *
   * Copyright (c) 2025 - 2026, pgEdge, Inc.
   * This software is released under The PostgreSQL License
   *
   *-------------------------------------------------------------------------
   */
  ```

## MCP Resources

### read_resource Tool

- The `read_resource` tool must always be present in the tool registry.

- It must properly advertise all available resources.

- Keep this working even when making other changes.

### Resource Discovery

Ensure resources are discoverable through:

- Native `resources/read` MCP endpoint.

- Backward-compatible `read_resource` tool.

- Proper resource registration in the registry.

## Example Checklist

When making changes, verify:

- [ ] Code uses 4-space indentation
- [ ] Tests added for new functionality
- [ ] All tests pass (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Documentation updated in `/docs`
- [ ] Markdown files properly formatted (79 chars, blank lines before lists)
- [ ] Security considerations addressed
- [ ] `read_resource` tool still works
- [ ] No temporary files left behind

## Questions?

If you're unsure about any of these guidelines, refer to:

- Existing code patterns in the repository.

- Documentation in `/docs`.

- Recent git commits for context.
