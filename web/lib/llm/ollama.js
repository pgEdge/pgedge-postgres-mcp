/*-------------------------------------------------------------------------
 *
 * Ollama LLM Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

/**
 * Ollama LLM client
 */
export class OllamaClient {
    constructor(baseURL, model) {
        this.baseURL = baseURL || 'http://localhost:11434';
        this.model = model;
    }

    /**
     * Send a chat request to Ollama
     * @param {Array} messages - Array of message objects
     * @param {Array} tools - Array of MCP tool definitions
     * @returns {Promise<Object>} LLM response with content and stop_reason
     */
    async chat(messages, tools = []) {
        // Format tools for Ollama
        const toolsContext = this.formatToolsForOllama(tools);

        // Create system message with tool information
        const systemMessage = `You are a helpful PostgreSQL database assistant. You have access to the following tools:

${toolsContext}

IMPORTANT INSTRUCTIONS:
1. When you need to use a tool, respond with ONLY a JSON object - no other text before or after:
{
    "tool": "tool_name",
    "arguments": {
        "param1": "value1",
        "param2": "value2"
    }
}

2. After calling a tool, you will receive actual results from the database.
3. You MUST base your response ONLY on the actual tool results provided - never make up or guess data.
4. If you receive tool results, format them clearly for the user.
5. Only use tools when necessary to answer the user's question.`;

        // Convert messages to Ollama format
        const ollamaMessages = [
            {
                role: 'system',
                content: systemMessage,
            },
        ];

        for (const msg of messages) {
            if (typeof msg.content === 'string') {
                ollamaMessages.push({
                    role: msg.role,
                    content: msg.content,
                });
            } else if (Array.isArray(msg.content)) {
                // Handle tool results
                const parts = [];
                for (const item of msg.content) {
                    if (item.type === 'tool_result') {
                        const contentStr = this.extractTextFromContent(item.content);
                        parts.push(`Tool result:\n${contentStr}`);
                    }
                }
                if (parts.length > 0) {
                    ollamaMessages.push({
                        role: msg.role,
                        content: parts.join('\n\n'),
                    });
                }
            }
        }

        const requestBody = {
            model: this.model,
            messages: ollamaMessages,
            stream: false,
        };

        const response = await fetch(`${this.baseURL}/api/chat`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(requestBody),
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const errorMessage = errorData.error || `HTTP ${response.status}: ${response.statusText}`;
            throw new Error(`Ollama error: ${errorMessage}`);
        }

        const data = await response.json();
        const content = data.message.content;

        // Try to parse as tool call
        try {
            const trimmed = content.trim();
            const toolCall = JSON.parse(trimmed);
            if (toolCall.tool && toolCall.arguments) {
                return {
                    content: [{
                        type: 'tool_use',
                        id: 'ollama-tool-1', // Ollama doesn't provide IDs
                        name: toolCall.tool,
                        input: toolCall.arguments,
                    }],
                    stopReason: 'tool_use',
                    usage: {
                        inputTokens: 0,  // Ollama doesn't provide token counts
                        outputTokens: 0,
                    },
                };
            }
        } catch (e) {
            // Not a tool call, treat as text response
        }

        // Text response
        return {
            content: [{
                type: 'text',
                text: content,
            }],
            stopReason: 'end_turn',
            usage: {
                inputTokens: 0,  // Ollama doesn't provide token counts
                outputTokens: 0,
            },
        };
    }

    /**
     * Format tools for Ollama system prompt
     * @param {Array} tools - Array of MCP tool definitions
     * @returns {string} Formatted tools description
     */
    formatToolsForOllama(tools) {
        const toolDescriptions = tools.map(tool => {
            let toolDesc = `- ${tool.name}: ${tool.description}`;

            // Add parameter info if available
            if (tool.inputSchema && tool.inputSchema.properties) {
                const params = [];
                for (const [paramName, paramInfo] of Object.entries(tool.inputSchema.properties)) {
                    const paramType = paramInfo.type || 'any';
                    const paramDesc = paramInfo.description || '';
                    params.push(`${paramName} (${paramType}): ${paramDesc}`);
                }
                if (params.length > 0) {
                    toolDesc += '\n  Parameters:\n    ' + params.join('\n    ');
                }
            }

            return toolDesc;
        });

        return toolDescriptions.join('\n');
    }

    /**
     * Extract text from various content formats
     * @param {*} content - Content to extract text from
     * @returns {string} Extracted text
     */
    extractTextFromContent(content) {
        if (typeof content === 'string') {
            return content;
        }
        if (Array.isArray(content)) {
            const texts = content
                .filter(item => item.text)
                .map(item => item.text);
            return texts.join('\n');
        }
        return JSON.stringify(content);
    }
}
