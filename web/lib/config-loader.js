/*-------------------------------------------------------------------------
 *
 * Configuration Loader
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { readFileSync, existsSync } from 'fs';
import { homedir } from 'os';
import { resolve } from 'path';

/**
 * Expand ~ to home directory in file paths
 * @param {string} filepath - Path that may contain ~
 * @returns {string} Expanded path
 */
function expandHome(filepath) {
    if (!filepath) return filepath;
    if (filepath.startsWith('~/')) {
        return resolve(homedir(), filepath.slice(2));
    }
    return filepath;
}

/**
 * Read API key from file
 * @param {string} filepath - Path to key file
 * @returns {string} API key or empty string if file doesn't exist
 */
function readKeyFile(filepath) {
    if (!filepath) return '';

    const expandedPath = expandHome(filepath);

    try {
        if (existsSync(expandedPath)) {
            const content = readFileSync(expandedPath, 'utf-8');
            // Trim whitespace and newlines
            return content.trim();
        }
    } catch (error) {
        console.warn(`Warning: Could not read key file ${expandedPath}: ${error.message}`);
    }

    return '';
}

/**
 * Load configuration from file and environment variables
 * @param {string} configPath - Path to config file
 * @returns {Object} Configuration object
 */
export function loadConfig(configPath) {
    // Load base configuration from file
    const configData = readFileSync(configPath, 'utf-8');
    const config = JSON.parse(configData);

    // Merge with environment variables
    // LLM configuration
    if (!config.llm) {
        config.llm = {};
    }

    // Provider configuration with environment variable fallbacks
    config.llm.provider = process.env.PGEDGE_LLM_PROVIDER || config.llm.provider || 'anthropic';
    config.llm.model = process.env.PGEDGE_LLM_MODEL || config.llm.model;

    // API keys with priority:
    // 1. Environment variable (PGEDGE_* or standard)
    // 2. Key file specified in config
    // 3. Empty string
    config.llm.anthropicAPIKey = process.env.PGEDGE_ANTHROPIC_API_KEY ||
                                  process.env.ANTHROPIC_API_KEY ||
                                  readKeyFile(config.llm.anthropicAPIKeyFile) ||
                                  '';

    config.llm.openaiAPIKey = process.env.PGEDGE_OPENAI_API_KEY ||
                              process.env.OPENAI_API_KEY ||
                              readKeyFile(config.llm.openaiAPIKeyFile) ||
                              '';

    config.llm.ollamaURL = process.env.PGEDGE_OLLAMA_URL ||
                          config.llm.ollamaURL ||
                          'http://localhost:11434';

    // Numeric parameters
    if (process.env.PGEDGE_LLM_MAX_TOKENS) {
        config.llm.maxTokens = parseInt(process.env.PGEDGE_LLM_MAX_TOKENS, 10);
    } else if (!config.llm.maxTokens) {
        config.llm.maxTokens = 4096;
    }

    if (process.env.PGEDGE_LLM_TEMPERATURE) {
        config.llm.temperature = parseFloat(process.env.PGEDGE_LLM_TEMPERATURE);
    } else if (config.llm.temperature === undefined) {
        config.llm.temperature = 0.7;
    }

    // Set default model based on provider if not specified
    if (!config.llm.model) {
        switch (config.llm.provider.toLowerCase()) {
            case 'anthropic':
                config.llm.model = 'claude-sonnet-4-5';
                break;
            case 'openai':
                config.llm.model = 'gpt-5-main';
                break;
            case 'ollama':
                config.llm.model = 'llama3';
                break;
            default:
                config.llm.model = 'claude-sonnet-4-5';
        }
    }

    // Server configuration
    if (!config.server) {
        config.server = {};
    }
    config.server.port = process.env.PORT ? parseInt(process.env.PORT, 10) : (config.server.port || 3001);

    return config;
}

/**
 * Validate configuration
 * @param {Object} config - Configuration object
 * @throws {Error} If configuration is invalid
 */
export function validateConfig(config) {
    // Validate MCP server configuration
    if (!config.mcpServer || !config.mcpServer.url) {
        throw new Error('MCP server URL is required in configuration');
    }

    // Validate session configuration
    if (!config.session || !config.session.secret) {
        throw new Error('Session secret is required in configuration');
    }

    if (config.session.secret === 'change-this-to-a-random-secret-in-production') {
        if (process.env.NODE_ENV === 'production') {
            throw new Error('Default session secret must be changed in production');
        }
        console.warn('WARNING: Using default session secret. Change this in production!');
    }

    // Validate LLM configuration
    if (!config.llm || !config.llm.provider) {
        throw new Error('LLM provider is required in configuration');
    }

    const validProviders = ['anthropic', 'openai', 'ollama'];
    if (!validProviders.includes(config.llm.provider.toLowerCase())) {
        throw new Error(`Invalid LLM provider: ${config.llm.provider}. Must be one of: ${validProviders.join(', ')}`);
    }

    // Provider-specific validation
    if (config.llm.provider.toLowerCase() === 'anthropic' && !config.llm.anthropicAPIKey) {
        throw new Error('Anthropic API key is required. Set ANTHROPIC_API_KEY environment variable or configure in config.json');
    }

    if (config.llm.provider.toLowerCase() === 'openai' && !config.llm.openaiAPIKey) {
        throw new Error('OpenAI API key is required. Set OPENAI_API_KEY environment variable or configure in config.json');
    }

    return true;
}
