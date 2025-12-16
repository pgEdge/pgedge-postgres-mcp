#!/usr/bin/env python3
"""
pgEdge Postgres MCP Chatbot Client (Stdio + Anthropic Claude)

A simple chatbot that uses Anthropic's Claude and connects to the pgEdge Postgres
MCP Server via stdio to answer questions about your PostgreSQL database using
natural language.

Usage:
    1. Set environment variables:
       export ANTHROPIC_API_KEY="your-api-key"
       export PGHOST="localhost"
       export PGPORT="5432"
       export PGDATABASE="mydb"
       export PGUSER="myuser"
       export PGPASSWORD="mypass"  # Or use ~/.pgpass file

    2. Run the chatbot:
       python chatbot.py

For more details, see: https://github.com/pgEdge/pgedge-postgres-mcp/blob/main/docs/stdio-anthropic-chatbot.md
"""

import asyncio
import os
from typing import Optional
from contextlib import AsyncExitStack

from anthropic import Anthropic
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client


class PostgresChatbot:
    def __init__(self):
        # Initialize Anthropic client
        api_key = os.getenv("ANTHROPIC_API_KEY")
        if not api_key:
            raise ValueError("ANTHROPIC_API_KEY environment variable is required")

        self.anthropic = Anthropic(api_key=api_key)
        self.session: Optional[ClientSession] = None
        self.exit_stack = AsyncExitStack()

    async def connect_to_server(self, server_path: str):
        """Connect to the pgEdge Natural Language Agent via stdio."""
        # Configure server parameters
        server_params = StdioServerParameters(
            command=server_path,
            args=[],
            env=None  # Inherits parent environment (including PG* or PGEDGE_DB_* variables)
        )

        # Connect to server
        stdio_transport = await self.exit_stack.enter_async_context(
            stdio_client(server_params)
        )
        self.stdio, self.write = stdio_transport

        # Initialize session
        self.session = await self.exit_stack.enter_async_context(
            ClientSession(self.stdio, self.write)
        )

        # Initialize the connection
        await self.session.initialize()

        print("✓ Connected to pgEdge Natural Language Agent")

    async def list_available_tools(self):
        """Retrieve and display available tools from the server."""
        if not self.session:
            raise RuntimeError("Not connected to server")

        response = await self.session.list_tools()

        print(f"\nAvailable tools ({len(response.tools)}):")
        for tool in response.tools:
            print(f"  - {tool.name}: {tool.description}")

        return response.tools

    async def process_query(self, user_query: str, messages: list = None) -> str:
        """
        Process a user query by interacting with Claude and executing tools.

        Args:
            user_query: The user's natural language question
            messages: Conversation history (for multi-turn conversations)

        Returns:
            Claude's response as a string
        """
        if not self.session:
            raise RuntimeError("Not connected to server")

        # Initialize messages if not provided
        if messages is None:
            messages = []

        # Add user query to messages
        messages.append({
            "role": "user",
            "content": user_query
        })

        # Get available tools
        tools_response = await self.session.list_tools()

        # Convert MCP tools to Anthropic tool format
        available_tools = []
        for tool in tools_response.tools:
            available_tools.append({
                "name": tool.name,
                "description": tool.description,
                "input_schema": tool.inputSchema
            })

        # Interact with Claude
        while True:
            response = self.anthropic.messages.create(
                model="claude-sonnet-4-20250514",
                max_tokens=4096,
                messages=messages,
                tools=available_tools
            )

            # Check if Claude wants to use tools
            if response.stop_reason == "tool_use":
                # Add Claude's response to messages
                messages.append({
                    "role": "assistant",
                    "content": response.content
                })

                # Execute all tool calls
                tool_results = []
                for content_block in response.content:
                    if content_block.type == "tool_use":
                        tool_name = content_block.name
                        tool_args = content_block.input

                        print(f"  → Executing tool: {tool_name}")

                        try:
                            # Call the tool via MCP
                            result = await self.session.call_tool(tool_name, tool_args)

                            tool_results.append({
                                "type": "tool_result",
                                "tool_use_id": content_block.id,
                                "content": result.content
                            })
                        except Exception as e:
                            tool_results.append({
                                "type": "tool_result",
                                "tool_use_id": content_block.id,
                                "content": f"Error: {str(e)}",
                                "is_error": True
                            })

                # Add tool results to messages
                messages.append({
                    "role": "user",
                    "content": tool_results
                })

            else:
                # Claude has finished, return the response
                final_response = ""
                for content_block in response.content:
                    if hasattr(content_block, "text"):
                        final_response += content_block.text

                return final_response

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

        messages = []

        while True:
            try:
                user_input = input("\nYou: ").strip()

                if user_input.lower() in ['quit', 'exit', 'q']:
                    print("\nGoodbye!")
                    break

                if not user_input:
                    continue

                print("\nClaude: ", end="", flush=True)
                response = await self.process_query(user_input, messages)
                print(response)

                # Add Claude's response to message history
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
        await self.exit_stack.aclose()


async def main():
    """Main entry point."""
    # Path to your pgEdge Natural Language Agent binary
    # Adjust this path as needed
    server_path = os.getenv("PGEDGE_MCP_SERVER_PATH", "../../bin/pgedge-postgres-mcp")

    # Check if server exists
    if not os.path.exists(server_path):
        print(f"Error: Server not found at {server_path}")
        print("\nPlease either:")
        print("  1. Build the server: cd ../.. && go build -o bin/pgedge-postgres-mcp ./cmd/pgedge-pg-mcp-svr")
        print("  2. Set PGEDGE_MCP_SERVER_PATH environment variable to the correct path")
        return

    # Check for required environment variables
    if not os.getenv("ANTHROPIC_API_KEY"):
        print("Error: ANTHROPIC_API_KEY environment variable is required")
        print("\nGet your API key from: https://console.anthropic.com/")
        print("Then set it: export ANTHROPIC_API_KEY='your-key-here'")
        return


    chatbot = PostgresChatbot()

    try:
        # Connect to server
        await chatbot.connect_to_server(server_path)

        # Run chat loop
        await chatbot.chat_loop()

    finally:
        await chatbot.cleanup()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass  # Goodbye message already printed from chat_loop
