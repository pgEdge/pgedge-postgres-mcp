#!/usr/bin/env python3
"""
pgEdge Postgres MCP Chatbot Client (HTTP + Ollama)

A simple chatbot that uses Ollama and connects to the pgEdge Postgres
MCP Server via HTTP to answer questions about your PostgreSQL database using
natural language.

Usage:
    1. Install and run Ollama with a model (e.g., gpt-oss:20b):
       ollama pull gpt-oss:20b
       ollama serve

    2. Start the MCP server in HTTP mode:
       ./bin/pgedge-postgres-mcp -http -addr :8080

    3. Set environment variables:
       export OLLAMA_BASE_URL="http://localhost:11434"  # Default Ollama URL
       export OLLAMA_MODEL="gpt-oss:20b"  # Or another model you've pulled
       export PGEDGE_MCP_SERVER_URL="http://localhost:8080/mcp/v1"

    4. Run the chatbot:
       python chatbot.py

For more details, see: https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/docs/http-ollama-chatbot.md
"""

import asyncio
import json
import os
from typing import Optional, List, Dict, Any

import httpx
from ollama import AsyncClient


class PostgresChatbot:
    def __init__(self):
        # Get Ollama configuration
        self.ollama_base_url = os.getenv("OLLAMA_BASE_URL", "http://localhost:11434")
        self.ollama_model = os.getenv("OLLAMA_MODEL", "gpt-oss:20b")

        # Initialize Ollama client
        self.ollama = AsyncClient(host=self.ollama_base_url)

        # Get MCP server URL (should be http://host:port/mcp/v1)
        self.mcp_server_url = os.getenv("PGEDGE_MCP_SERVER_URL", "http://localhost:8080/mcp/v1")
        if not self.mcp_server_url:
            raise ValueError("PGEDGE_MCP_SERVER_URL environment variable is required")

        # HTTP client for MCP communication
        self.http_client = httpx.AsyncClient(timeout=30.0)
        self.request_id = 0

    def _get_next_id(self) -> int:
        """Get next JSON-RPC request ID."""
        self.request_id += 1
        return self.request_id

    async def _jsonrpc_request(self, method: str, params: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """Send a JSON-RPC request to the MCP server."""
        request = {
            "jsonrpc": "2.0",
            "id": self._get_next_id(),
            "method": method,
            "params": params or {}
        }

        try:
            response = await self.http_client.post(
                self.mcp_server_url,
                json=request,
                headers={"Content-Type": "application/json"}
            )
            response.raise_for_status()
            result = response.json()

            if "error" in result:
                raise Exception(f"JSON-RPC error: {result['error']}")

            return result.get("result", {})
        except httpx.HTTPError as e:
            raise Exception(f"HTTP error: {e}")

    async def list_available_tools(self) -> List[Dict[str, Any]]:
        """Retrieve available tools from the MCP server using JSON-RPC."""
        try:
            result = await self._jsonrpc_request("tools/list")
            tools = result.get("tools", [])
            return tools
        except Exception as e:
            print(f"Error listing tools: {e}")
            return []

    async def call_tool(self, tool_name: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
        """Call a tool on the MCP server using JSON-RPC."""
        try:
            result = await self._jsonrpc_request("tools/call", {
                "name": tool_name,
                "arguments": arguments
            })
            return result
        except Exception as e:
            return {
                "error": str(e),
                "is_error": True
            }

    def format_tools_for_ollama(self, tools: List[Dict[str, Any]]) -> str:
        """Format tools for Ollama's context window."""
        tool_descriptions = []
        for tool in tools:
            tool_desc = f"- {tool['name']}: {tool.get('description', 'No description')}"

            # Add parameter info if available
            if 'inputSchema' in tool:
                schema = tool['inputSchema']
                if 'properties' in schema:
                    params = []
                    for param_name, param_info in schema['properties'].items():
                        param_type = param_info.get('type', 'unknown')
                        param_desc = param_info.get('description', '')
                        params.append(f"{param_name} ({param_type}): {param_desc}")
                    if params:
                        tool_desc += "\n  Parameters:\n    " + "\n    ".join(params)

            tool_descriptions.append(tool_desc)

        return "\n".join(tool_descriptions)

    async def process_query(self, user_query: str, messages: List[Dict[str, str]], tools: List[Dict[str, Any]]) -> str:
        """
        Process a user query by interacting with Ollama and executing tools.

        Args:
            user_query: The user's natural language question
            messages: Conversation history
            tools: Available MCP tools

        Returns:
            Ollama's response as a string
        """
        # Format tools for Ollama
        tools_context = self.format_tools_for_ollama(tools)

        # Create system message with tool information
        system_message = f"""You are a helpful PostgreSQL database assistant. You have access to the following tools:

{tools_context}

IMPORTANT INSTRUCTIONS:
1. When you need to use a tool, respond with ONLY a JSON object - no other text before or after:
{{
    "tool": "tool_name",
    "arguments": {{
        "param1": "value1",
        "param2": "value2"
    }}
}}

2. After calling a tool, you will receive actual results from the database.
3. You MUST base your response ONLY on the actual tool results provided - never make up or guess data.
4. If you receive tool results, format them clearly for the user.
5. Only use tools when necessary to answer the user's question."""

        # Add user query to messages
        messages.append({
            "role": "user",
            "content": user_query
        })

        # Interact with Ollama (max 10 iterations to prevent infinite loops)
        for iteration in range(10):
            # Build full context
            full_messages = [{"role": "system", "content": system_message}] + messages

            # Get response from Ollama
            response = await self.ollama.chat(
                model=self.ollama_model,
                messages=full_messages
            )

            assistant_message = response['message']['content']

            # Try to parse as a tool call
            try:
                tool_call = json.loads(assistant_message.strip())

                if 'tool' in tool_call:
                    tool_name = tool_call['tool']
                    tool_args = tool_call.get('arguments', {})

                    print(f"  → Executing tool: {tool_name}")

                    # Execute the tool
                    tool_result = await self.call_tool(tool_name, tool_args)

                    # Add tool execution to conversation
                    messages.append({
                        "role": "assistant",
                        "content": assistant_message
                    })

                    # Format tool result
                    result_text = json.dumps(tool_result, indent=2)
                    messages.append({
                        "role": "user",
                        "content": f"Tool result:\n{result_text}"
                    })

                    # Continue the loop to get the natural language response
                    continue

            except json.JSONDecodeError:
                pass

            # If we get here, it's a final natural language response
            return assistant_message

        return "I apologize, but I've reached the maximum number of tool calls. Please try rephrasing your question."

    async def chat_loop(self):
        """Run an interactive chat loop."""
        print("\nPostgreSQL Chatbot (type 'quit' or 'exit' to stop)")
        print("=" * 60)
        print("\nExample questions:")
        print("  - List all tables: 'What tables are in my database?'")
        print("  - Show me the schema: 'Describe the users table'")
        print("  - Query data: 'Show me the 10 most recent orders'")
        print("  - Aggregate data: 'What's the total revenue from last month?'")
        print("  - Complex queries: 'Which customers have placed more than 5 orders?'")
        print("  - Search content: 'Find articles about PostgreSQL' (if using vector search)")

        # Get available tools
        tools = await self.list_available_tools()
        if not tools:
            print("\nWarning: No tools available from MCP server")
            return

        messages = []

        while True:
            try:
                user_input = input("\nYou: ").strip()

                if user_input.lower() in ['quit', 'exit', 'q']:
                    print("\nGoodbye!")
                    break

                if not user_input:
                    continue

                print()
                response = await self.process_query(user_input, messages, tools)
                print(f"Assistant: {response}")

                # Add assistant's response to message history
                messages.append({
                    "role": "assistant",
                    "content": response
                })

            except KeyboardInterrupt:
                print("\n\nGoodbye!")
                break
            except Exception as e:
                print(f"\nError: {e}")

    async def cleanup(self):
        """Clean up resources."""
        await self.http_client.aclose()


async def main():
    """Main entry point."""
    # Check for required environment variables
    mcp_server_url = os.getenv("PGEDGE_MCP_SERVER_URL")
    if not mcp_server_url:
        print("Error: PGEDGE_MCP_SERVER_URL environment variable is required")
        print("\nPlease start the MCP server in HTTP mode and set the URL:")
        print("  1. Start server: ./bin/pgedge-postgres-mcp -http -addr :8080")
        print("  2. Set URL: export PGEDGE_MCP_SERVER_URL='http://localhost:8080/mcp/v1'")
        return

    # Check if Ollama is configured
    ollama_model = os.getenv("OLLAMA_MODEL", "gpt-oss:20b")
    print(f"Using Ollama model: {ollama_model}")
    print(f"MCP Server: {mcp_server_url}")

    chatbot = PostgresChatbot()

    try:
        print("✓ Connected to pgEdge Natural Language Agent")

        # Run chat loop
        await chatbot.chat_loop()

    finally:
        await chatbot.cleanup()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass  # Goodbye message already printed from chat_loop
