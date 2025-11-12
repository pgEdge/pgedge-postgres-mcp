# Building Chatbot Clients

This guide shows how to build chatbot applications that use the pgEdge Postgres MCP Server to interact with PostgreSQL databases through natural language.

## Overview

An MCP chatbot client connects to the pgEdge Postgres MCP Server and uses an LLM to translate natural language queries into database operations. The basic architecture looks like this:

```
User Query → Your Client → LLM API → MCP Server → PostgreSQL
                  ↑______________|
```

The client:

1. Connects to the MCP server (via stdio or HTTP)
2. Retrieves available tools from the server
3. Sends user queries to an LLM along with tool definitions
4. Executes tools requested by the LLM
5. Returns results to the LLM for final response
6. Presents the answer to the user

## Available Examples

We provide two complete chatbot examples demonstrating different approaches:

### Stdio + Anthropic Claude

A chatbot that connects to the MCP server via stdio (standard input/output) and uses Anthropic's Claude for natural language processing.

- **Best for**: Desktop applications, local development, simple deployments
- **Requires**: Anthropic API key
- **Connection**: Direct process communication via stdio

[View Stdio + Anthropic Claude Example →](stdio-anthropic-chatbot.md)

### HTTP + Ollama

A chatbot that connects to the MCP server via HTTP and uses Ollama for local LLM inference.

- **Best for**: Distributed systems, microservices, privacy-sensitive applications
- **Requires**: Ollama installed locally
- **Connection**: HTTP REST API

[View HTTP + Ollama Example →](http-ollama-chatbot.md)

## Choosing an Approach

| Feature | Stdio + Anthropic | HTTP + Ollama |
|---------|------------------|---------------|
| **LLM** | Anthropic Claude (cloud) | Ollama (local) |
| **Connection** | Stdio (process) | HTTP (network) |
| **Deployment** | Single machine | Distributed |
| **Privacy** | Data sent to Anthropic | Data stays local |
| **Cost** | Per-token pricing | Free (after hardware) |
| **Performance** | Fast (cloud GPUs) | Depends on local hardware |
| **Scalability** | Limited by API rate limits | Limited by local resources |

## General Prerequisites

**For all examples:**

- Python 3.10+
- The pgEdge Postgres MCP Server built and available
- A PostgreSQL database (connections can be configured via environment variable or through the chatbot)

**Example-specific requirements:**

- **Stdio + Anthropic**: Anthropic API key
- **HTTP + Ollama**: Ollama installed with a model pulled

## Best Practices

1. **Error Handling**: Always handle connection errors, tool execution failures, and LLM API errors gracefully

2. **Token Management**: Be mindful of context window limits - summarize conversation history if needed

3. **Security**: Never commit API keys or database credentials to version control

4. **Tool Result Formatting**: Format tool results clearly for the LLM to understand

5. **User Feedback**: Show users when tools are being executed so they understand what's happening

6. **Timeouts**: Set appropriate timeouts for both LLM calls and tool executions

7. **Logging**: Log errors and important events for debugging

## Next Steps

Choose the example that best fits your needs:

- [Stdio + Anthropic Claude Example](stdio-anthropic-chatbot.md) - Great for getting started quickly
- [HTTP + Ollama Example](http-ollama-chatbot.md) - Perfect for privacy-sensitive or offline applications

For more information about the MCP server itself, see:

- [Configuration Guide](configuration.md)
- [Available Tools](tools.md)
- [Authentication](authentication.md)
