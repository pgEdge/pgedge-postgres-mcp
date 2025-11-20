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


**Note:** The Go Chat Client is a full-featured application with polished UI, configuration management, and command history. The Python examples are intentionally simple reference implementations to demonstrate the MCP protocol.

We provide three complete chatbot examples demonstrating different approaches:

### Go Chat Client (Recommended)

A full-featured native Go implementation with support for both stdio and HTTP modes, and multiple LLM providers including Anthropic Claude, OpenAI, and Ollama.

- **Best for**: Production use, flexible deployments, native performance
- **Requires**: Go 1.23+ to build, or use pre-built binary
- **Connection**: Supports both stdio and HTTP
- **LLM Support**: Anthropic Claude, OpenAI (GPT-4o, GPT-5, etc.), and Ollama

[View Go Chat Client →](using-cli-client.md)

### Stdio + Anthropic Claude (Python)

A simple Python example that connects to the MCP server via stdio (standard input/output) and uses Anthropic's Claude for natural language processing.

- **Best for**: Quick prototyping, Python developers, simple deployments
- **Requires**: Python 3.10+, Anthropic API key
- **Connection**: Direct process communication via stdio

[View Stdio + Anthropic Claude Example →](stdio-anthropic-chatbot.md)

### HTTP + Ollama (Python)

A simple Python example that connects to the MCP server via HTTP and uses Ollama for local LLM inference.

- **Best for**: Distributed systems, microservices, privacy-sensitive applications
- **Requires**: Python 3.10+, Ollama installed locally
- **Connection**: HTTP REST API

[View HTTP + Ollama Example →](http-ollama-chatbot.md)

## Choosing an Approach

| Feature | Go Chat Client | Stdio + Anthropic (Python) | HTTP + Ollama (Python) |
|---------|----------------|---------------------------|----------------------|
| **Language** | Go (native binary) | Python | Python |
| **LLM** | Anthropic, OpenAI, and Ollama | Anthropic Claude (cloud) | Ollama (local) |
| **Connection** | Both stdio and HTTP | Stdio (process) | HTTP (network) |
| **Deployment** | Flexible | Single machine | Distributed |
| **Privacy** | Configurable | Data sent to Anthropic | Data stays local |
| **Performance** | Native, fast startup | Fast | Depends on hardware |
| **Best for** | Production use | Quick prototyping | Local/privacy-focused |

## General Prerequisites

**For all examples:**

- The pgEdge Postgres MCP Server built and available
- A PostgreSQL database (connections can be configured via environment variable or through the chatbot)

**Example-specific requirements:**

- **Go Chat Client**: Go 1.23+ (to build), or use pre-built binary
- **Stdio + Anthropic (Python)**: Python 3.10+, Anthropic API key
- **HTTP + Ollama (Python)**: Python 3.10+, Ollama installed with a model pulled

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

- [Go Chat Client](using-cli-client.md) - **Recommended** for production use
- [Stdio + Anthropic Claude Example](stdio-anthropic-chatbot.md) - Great for prototyping with Python
- [HTTP + Ollama Example](http-ollama-chatbot.md) - Perfect for privacy-sensitive or offline applications

For more information about the MCP server itself, see:

- [Configuration Guide](configuration.md)
- [Available Tools](tools.md)
- [Authentication](authentication.md)
