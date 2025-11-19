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

**When you need to call a tool:**
- Respond with ONLY a JSON object (no other text):
{
    "tool": "tool_name",
    "arguments": {
        "param1": "value1",
        "param2": "value2"
    }
}

**After you receive tool results:**
- DO NOT call the same tool again
- DO NOT respond with JSON
- Use the actual data from the tool results to formulate your answer
- Present the information clearly in natural language
- Base your response ONLY on the actual tool results - never make up data

**Important:** If you see "Tool returned:" in the conversation, that means you already called the tool and received results. Answer the user's question using those results.`;

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
                // Handle array content (tool_use, tool_result, text)
                const parts = [];
                for (const item of msg.content) {
                    if (item.type === 'tool_use') {
                        // Assistant requested a tool call
                        parts.push(`I called the tool: ${item.name} with arguments: ${JSON.stringify(item.input)}`);
                    } else if (item.type === 'tool_result') {
                        // Tool execution result
                        const contentStr = this.extractTextFromContent(item.content);
                        parts.push(`Tool "${item.tool_use_id}" returned:\n${contentStr}`);
                    } else if (item.type === 'text') {
                        // Text content
                        parts.push(item.text);
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

        // Try to extract and parse tool call from response
        // The model might return JSON directly or embedded in text
        let toolCall = null;

        try {
            // First try: parse entire response as JSON
            const trimmed = content.trim();
            toolCall = JSON.parse(trimmed);
        } catch (e) {
            // Second try: extract JSON block from text using regex
            // Look for {...} pattern that might be a tool call
            const jsonMatch = content.match(/\{[\s\S]*"tool"[\s\S]*"arguments"[\s\S]*\}/);
            if (jsonMatch) {
                try {
                    toolCall = JSON.parse(jsonMatch[0]);
                } catch (e2) {
                    // Failed to parse extracted JSON
                }
            }
        }

        // Check if we found a valid tool call
        if (toolCall && toolCall.tool && toolCall.arguments) {
            console.log('Ollama: Detected tool call:', toolCall.tool);
            return {
                content: [{
                    type: 'tool_use',
                    id: `ollama-tool-${Date.now()}`, // Generate unique ID
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

        // No tool call detected, treat as text response
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

    /**
     * List available models from Ollama
     * @returns {Promise<Array>} Array of model objects with name and details
     */
    async listModels() {
        try {
            const response = await fetch(`${this.baseURL}/api/tags`, {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json',
                },
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();
            // Ollama returns { models: [ ... ] }
            return (data.models || []).map(model => ({
                name: model.name,
                size: model.size,
                modified_at: model.modified_at,
            }));
        } catch (error) {
            console.error('Error listing Ollama models:', error);
            throw error;
        }
    }
}
