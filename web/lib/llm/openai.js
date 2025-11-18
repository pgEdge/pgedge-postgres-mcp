/*-------------------------------------------------------------------------
 *
 * OpenAI GPT LLM Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

/**
 * OpenAI GPT LLM client
 */
export class OpenAIClient {
    constructor(apiKey, model, maxTokens = 4096, temperature = 0.7) {
        this.apiKey = apiKey;
        this.model = model;
        this.maxTokens = maxTokens;
        this.temperature = temperature;
        this.baseURL = 'https://api.openai.com/v1';
    }

    /**
     * Send a chat request to OpenAI
     * @param {Array} messages - Array of message objects
     * @param {Array} tools - Array of MCP tool definitions
     * @returns {Promise<Object>} LLM response with content and stop_reason
     */
    async chat(messages, tools = []) {
        // Convert MCP tools to OpenAI format
        const openaiTools = tools.map(tool => ({
            type: 'function',
            function: {
                name: tool.name,
                description: tool.description,
                parameters: tool.inputSchema,
            },
        }));

        // Convert messages to OpenAI format
        const openaiMessages = this.convertMessages(messages);

        const requestBody = {
            model: this.model,
            messages: openaiMessages,
        };

        // Use max_completion_tokens for newer models (gpt-5, o1-*, o3-*)
        // Use max_tokens for older models (gpt-4, gpt-3.5, etc.)
        const isNewModel = this.model.startsWith('gpt-5') ||
                          this.model.startsWith('o1-') ||
                          this.model.startsWith('o3-');

        if (isNewModel) {
            requestBody.max_completion_tokens = this.maxTokens;
            // GPT-5 and o-series only support temperature=1 (default)
        } else {
            requestBody.max_tokens = this.maxTokens;
            requestBody.temperature = this.temperature;
        }

        if (openaiTools.length > 0) {
            requestBody.tools = openaiTools;
        }

        const response = await fetch(`${this.baseURL}/chat/completions`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${this.apiKey}`,
            },
            body: JSON.stringify(requestBody),
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const errorMessage = errorData.error?.message || `HTTP ${response.status}: ${response.statusText}`;
            throw new Error(`OpenAI API error: ${errorMessage}`);
        }

        const data = await response.json();

        if (!data.choices || data.choices.length === 0) {
            throw new Error('No choices in OpenAI response');
        }

        const choice = data.choices[0];

        // Check if there are tool calls
        if (choice.message.tool_calls && choice.message.tool_calls.length > 0) {
            const content = choice.message.tool_calls.map(tc => ({
                type: 'tool_use',
                id: tc.id,
                name: tc.function.name,
                input: JSON.parse(tc.function.arguments),
            }));

            return {
                content: content,
                stopReason: 'tool_use',
                usage: {
                    inputTokens: data.usage.prompt_tokens,
                    outputTokens: data.usage.completion_tokens,
                },
            };
        }

        // Text response
        return {
            content: [{
                type: 'text',
                text: choice.message.content || '',
            }],
            stopReason: choice.finish_reason === 'stop' ? 'end_turn' : choice.finish_reason,
            usage: {
                inputTokens: data.usage.prompt_tokens,
                outputTokens: data.usage.completion_tokens,
            },
        };
    }

    /**
     * Convert messages to OpenAI format
     * @param {Array} messages - Array of message objects
     * @returns {Array} OpenAI formatted messages
     */
    convertMessages(messages) {
        const openaiMessages = [];

        for (const msg of messages) {
            const openaiMsg = { role: msg.role };

            // Handle different content types
            if (typeof msg.content === 'string') {
                openaiMsg.content = msg.content;
                openaiMessages.push(openaiMsg);
            } else if (Array.isArray(msg.content)) {
                let hasToolCalls = false;
                const toolCalls = [];

                for (const item of msg.content) {
                    if (item.type === 'text') {
                        openaiMsg.content = item.text;
                    } else if (item.type === 'tool_use') {
                        hasToolCalls = true;
                        toolCalls.push({
                            id: item.id,
                            type: 'function',
                            function: {
                                name: item.name,
                                arguments: JSON.stringify(item.input),
                            },
                        });
                    } else if (item.type === 'tool_result') {
                        // Tool results are separate messages in OpenAI format
                        openaiMessages.push({
                            role: 'tool',
                            content: this.extractTextFromContent(item.content),
                            tool_call_id: item.tool_use_id,
                        });
                    }
                }

                if (hasToolCalls) {
                    openaiMsg.tool_calls = toolCalls;
                }

                if (openaiMsg.content || openaiMsg.tool_calls) {
                    openaiMessages.push(openaiMsg);
                }
            }
        }

        return openaiMessages;
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
                .filter(item => item.type === 'text')
                .map(item => item.text);
            return texts.join('\n');
        }
        return JSON.stringify(content);
    }
}
