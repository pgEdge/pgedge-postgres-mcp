/*-------------------------------------------------------------------------
 *
 * Chat Agent - Agentic loop for LLM interaction with MCP tools
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { createLLMClient } from './llm/index.js';

/**
 * Chat Agent handles the agentic loop for LLM interaction with MCP tools
 */
export class ChatAgent {
    constructor(config, mcpClient) {
        this.config = config;
        this.mcpClient = mcpClient;
        this.llmClient = createLLMClient(config.llm);
        this.tools = [];
        this.resources = [];
        this.activity = []; // Track tool and resource activity
    }

    /**
     * Initialize the agent by fetching available tools and resources from MCP
     */
    async initialize() {
        // Fetch available tools
        this.tools = await this.mcpClient.listTools();

        // Fetch available resources
        this.resources = await this.mcpClient.listResources();
    }

    /**
     * Process a user query through the agentic loop
     * @param {string} query - User's question
     * @param {Array} conversationHistory - Previous messages in the conversation
     * @param {Function} onActivity - Optional callback for streaming activity updates
     * @returns {Promise<Object>} Response with text and updated conversation history
     */
    async processQuery(query, conversationHistory = [], onActivity = null) {
        console.log('ChatAgent: Processing query:', query);
        console.log('ChatAgent: Conversation history length:', conversationHistory.length);

        // Reset activity tracking for this query
        this.activity = [];

        // Build messages array for LLM
        const messages = [
            ...conversationHistory,
            {
                role: 'user',
                content: query,
            },
        ];

        // Agentic loop (max 10 iterations to prevent infinite loops)
        for (let iteration = 0; iteration < 10; iteration++) {
            console.log(`ChatAgent: Iteration ${iteration + 1}`);

            // Get response from LLM with available tools
            let response;
            try {
                response = await this.llmClient.chat(messages, this.tools);
                console.log('ChatAgent: LLM response stop reason:', response.stopReason);
            } catch (error) {
                console.error('ChatAgent: LLM error:', error.message);
                console.error('ChatAgent: Last message:', JSON.stringify(messages[messages.length - 1], null, 2));
                throw error;
            }

            // Check if LLM wants to use tools
            if (response.stopReason === 'tool_use') {
                // Extract tool uses from response
                const toolUses = response.content.filter(item => item.type === 'tool_use');
                console.log(`ChatAgent: LLM requested ${toolUses.length} tool call(s)`);

                // Add assistant's message to conversation
                messages.push({
                    role: 'assistant',
                    content: response.content,
                });

                // Execute all tool calls
                const toolResults = [];
                for (const toolUse of toolUses) {
                    console.log(`ChatAgent: Calling tool ${toolUse.name}`);
                    try {
                        // Check if this is a resource access (resources start with resource://)
                        if (toolUse.name === 'read_resource') {
                            // Record resource read activity
                            const activity = {
                                type: 'resource',
                                uri: toolUse.input.uri,
                            };
                            this.activity.push(activity);

                            // Stream activity if callback provided
                            if (onActivity) {
                                onActivity(activity);
                            }

                            const result = await this.mcpClient.readResource(toolUse.input.uri);
                            // result.contents is an array of content blocks
                            toolResults.push({
                                type: 'tool_result',
                                tool_use_id: toolUse.id,
                                content: result.contents,  // Array of content blocks
                            });
                        } else {
                            // Record tool call activity
                            const activity = {
                                type: 'tool',
                                name: toolUse.name,
                            };
                            this.activity.push(activity);

                            // Stream activity if callback provided
                            if (onActivity) {
                                onActivity(activity);
                            }

                            // Regular tool call
                            const result = await this.mcpClient.callTool(toolUse.name, toolUse.input);
                            // result.content is an array of content blocks from MCP
                            toolResults.push({
                                type: 'tool_result',
                                tool_use_id: toolUse.id,
                                content: result.content,  // Array of content blocks
                                is_error: result.isError,
                            });
                        }
                        console.log(`ChatAgent: Tool ${toolUse.name} completed successfully`);
                    } catch (error) {
                        console.error(`ChatAgent: Tool ${toolUse.name} error:`, error.message);
                        toolResults.push({
                            type: 'tool_result',
                            tool_use_id: toolUse.id,
                            content: `Error: ${error.message}`,
                            is_error: true,
                        });
                    }
                }

                // Add tool results to conversation
                messages.push({
                    role: 'user',
                    content: toolResults,
                });

                // Continue the loop to get final response
                continue;
            }

            // Got final response - extract text content
            const textParts = response.content
                .filter(item => item.type === 'text')
                .map(item => item.text);

            const finalText = textParts.join('\n');

            // Add assistant's final response to history
            messages.push({
                role: 'assistant',
                content: finalText,
            });

            return {
                response: finalText,
                conversationHistory: messages,
                usage: response.usage,
                activity: this.activity, // Include tool and resource activity
            };
        }

        throw new Error('Reached maximum number of tool calls (10)');
    }
}

/**
 * MCP Client wrapper for making JSON-RPC calls to MCP server
 */
export class MCPClient {
    constructor(serverURL, token) {
        this.serverURL = serverURL;
        this.token = token;
    }

    /**
     * Make a JSON-RPC call to the MCP server
     * @param {string} method - JSON-RPC method name
     * @param {Object} params - Method parameters
     * @returns {Promise<*>} Result from the server
     */
    async call(method, params = {}) {
        const headers = {
            'Content-Type': 'application/json',
        };

        if (this.token) {
            headers['Authorization'] = `Bearer ${this.token}`;
        }

        const requestBody = {
            jsonrpc: '2.0',
            id: Date.now(),
            method,
            params,
        };

        const response = await fetch(this.serverURL, {
            method: 'POST',
            headers,
            body: JSON.stringify(requestBody),
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`MCP server error: ${response.statusText} - ${errorText}`);
        }

        const data = await response.json();

        if (data.error) {
            const errorMessage = data.error.data || data.error.message || 'MCP server error';
            throw new Error(errorMessage);
        }

        return data.result;
    }

    /**
     * List available tools
     * @returns {Promise<Array>} Array of tool definitions
     */
    async listTools() {
        const result = await this.call('tools/list');
        return result.tools || [];
    }

    /**
     * List available resources
     * @returns {Promise<Array>} Array of resource definitions
     */
    async listResources() {
        const result = await this.call('resources/list');
        return result.resources || [];
    }

    /**
     * Call a tool
     * @param {string} name - Tool name
     * @param {Object} args - Tool arguments
     * @returns {Promise<Object>} Tool result
     */
    async callTool(name, args) {
        return await this.call('tools/call', {
            name,
            arguments: args,
        });
    }

    /**
     * Read a resource
     * @param {string} uri - Resource URI
     * @returns {Promise<Object>} Resource content
     */
    async readResource(uri) {
        return await this.call('resources/read', { uri });
    }
}
