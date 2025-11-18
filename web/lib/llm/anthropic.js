/*-------------------------------------------------------------------------
 *
 * Anthropic Claude LLM Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

/**
 * Anthropic Claude LLM client
 */
export class AnthropicClient {
    constructor(apiKey, model, maxTokens = 4096, temperature = 0.7) {
        this.apiKey = apiKey;
        this.model = model;
        this.maxTokens = maxTokens;
        this.temperature = temperature;
        this.baseURL = 'https://api.anthropic.com/v1';
    }

    /**
     * Send a chat request to Claude
     * @param {Array} messages - Array of message objects
     * @param {Array} tools - Array of MCP tool definitions
     * @returns {Promise<Object>} LLM response with content and stop_reason
     */
    async chat(messages, tools = []) {
        // Validate messages format
        console.log('Anthropic: Validating messages format...');
        for (let i = 0; i < messages.length; i++) {
            const msg = messages[i];
            if (!msg.role || (msg.role !== 'user' && msg.role !== 'assistant')) {
                console.error(`Invalid message role at index ${i}:`, msg.role);
                throw new Error(`Invalid message role: ${msg.role}`);
            }
            if (msg.content === undefined || msg.content === null) {
                console.error(`Missing content at index ${i}`);
                throw new Error(`Message ${i} missing content`);
            }
        }
        console.log(`Anthropic: ${messages.length} messages validated`);

        // Convert MCP tools to Anthropic format with caching
        const anthropicTools = tools.map((tool, index) => {
            const toolDef = {
                name: tool.name,
                description: tool.description,
                input_schema: tool.inputSchema,
            };

            // Add cache_control to the last tool to cache all tools
            if (index === tools.length - 1) {
                toolDef.cache_control = { type: 'ephemeral' };
            }

            return toolDef;
        });

        const requestBody = {
            model: this.model,
            max_tokens: this.maxTokens,
            messages: messages,
            temperature: this.temperature,
        };

        if (anthropicTools.length > 0) {
            requestBody.tools = anthropicTools;
        }

        const response = await fetch(`${this.baseURL}/messages`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'x-api-key': this.apiKey,
                'anthropic-version': '2023-06-01',
                'anthropic-beta': 'prompt-caching-2024-07-31',
            },
            body: JSON.stringify(requestBody),
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            const errorMessage = errorData.error?.message || `HTTP ${response.status}: ${response.statusText}`;
            console.error('Anthropic API error response:', JSON.stringify(errorData, null, 2));
            console.error('Request body was:', JSON.stringify(requestBody, null, 2));
            throw new Error(`Anthropic API error: ${errorMessage}`);
        }

        const data = await response.json();

        // Log cache usage if present
        if (data.usage?.cache_creation_input_tokens || data.usage?.cache_read_input_tokens) {
            const totalInput = data.usage.input_tokens + (data.usage.cache_read_input_tokens || 0);
            const savePercent = totalInput > 0
                ? ((data.usage.cache_read_input_tokens || 0) / totalInput * 100).toFixed(0)
                : 0;
            console.log(`[LLM] Prompt Cache - Created: ${data.usage.cache_creation_input_tokens || 0} tokens, ` +
                `Read: ${data.usage.cache_read_input_tokens || 0} tokens (saved ~${savePercent}% on input)`);
        }

        return {
            content: data.content,
            stopReason: data.stop_reason,
            usage: data.usage,
        };
    }
}
