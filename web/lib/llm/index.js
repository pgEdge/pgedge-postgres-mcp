/*-------------------------------------------------------------------------
 *
 * LLM Client Factory
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { AnthropicClient } from './anthropic.js';
import { OpenAIClient } from './openai.js';
import { OllamaClient } from './ollama.js';

/**
 * Create an LLM client based on provider configuration
 * @param {Object} config - LLM configuration object
 * @returns {Object} LLM client instance
 */
export function createLLMClient(config) {
    const provider = config.provider.toLowerCase();

    switch (provider) {
        case 'anthropic':
            if (!config.anthropicAPIKey) {
                throw new Error('Anthropic API key is required');
            }
            return new AnthropicClient(
                config.anthropicAPIKey,
                config.model || 'claude-sonnet-4-5',
                config.maxTokens || 4096,
                config.temperature || 0.7
            );

        case 'openai':
            if (!config.openaiAPIKey) {
                throw new Error('OpenAI API key is required');
            }
            return new OpenAIClient(
                config.openaiAPIKey,
                config.model || 'gpt-5-main',
                config.maxTokens || 4096,
                config.temperature || 0.7
            );

        case 'ollama':
            return new OllamaClient(
                config.ollamaURL || 'http://localhost:11434',
                config.model || 'llama3'
            );

        default:
            throw new Error(`Unsupported LLM provider: ${provider}`);
    }
}

/**
 * Validate LLM configuration
 * @param {Object} config - LLM configuration object
 * @throws {Error} If configuration is invalid
 */
export function validateLLMConfig(config) {
    if (!config || !config.provider) {
        throw new Error('LLM provider is required');
    }

    const provider = config.provider.toLowerCase();
    const validProviders = ['anthropic', 'openai', 'ollama'];

    if (!validProviders.includes(provider)) {
        throw new Error(`Invalid LLM provider: ${provider}. Must be one of: ${validProviders.join(', ')}`);
    }

    if (provider === 'anthropic' && !config.anthropicAPIKey) {
        throw new Error('Anthropic API key is required (set ANTHROPIC_API_KEY environment variable or anthropicAPIKey in config)');
    }

    if (provider === 'openai' && !config.openaiAPIKey) {
        throw new Error('OpenAI API key is required (set OPENAI_API_KEY environment variable or openaiAPIKey in config)');
    }

    if (provider === 'ollama' && !config.ollamaURL) {
        config.ollamaURL = 'http://localhost:11434'; // Set default
    }

    // Set default model if not specified
    if (!config.model) {
        switch (provider) {
            case 'anthropic':
                config.model = 'claude-sonnet-4-5';
                break;
            case 'openai':
                config.model = 'gpt-5-main';
                break;
            case 'ollama':
                config.model = 'llama3';
                break;
        }
    }

    // Set defaults for other parameters
    if (!config.maxTokens) {
        config.maxTokens = 4096;
    }
    if (config.temperature === undefined) {
        config.temperature = 0.7;
    }
}

export { AnthropicClient, OpenAIClient, OllamaClient };
