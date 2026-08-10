/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Chat Interface (Refactored)
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Box, Paper, useTheme } from '@mui/material';
import { useAuth } from '../contexts/AuthContext';
import { useLLMProcessing } from '../contexts/LLMProcessingContext';
import { useDatabaseContext } from '../contexts/DatabaseContext';
import { useConversationActions } from '../contexts/ConversationActionsContext';
import { useLocalStorageBoolean } from '../hooks/useLocalStorage';
import { useQueryHistory } from '../hooks/useQueryHistory';
import { useMCPClient } from '../hooks/useMCPClient';
import { useLLMProviders } from '../hooks/useLLMProviders';
import MessageList from './MessageList';
import MessageInput from './MessageInput';
import ProviderSelector from './ProviderSelector';
import PromptPopover from './PromptPopover';
import WriteQueryConfirmDialog from './WriteQueryConfirmDialog';
import { writeConfirmationSubject } from '../utils/queryClassify';
import { sseChat } from '../utils/sseChat';
import {
    createToolRepeatGuard,
    buildRepeatedToolFailureMessage,
} from '../utils/toolRepeatGuard';
import { conversationToMarkdown, downloadMarkdown } from '../lib/conversationExport';

const MAX_AGENTIC_LOOPS = 50;
// Consecutive failures of the same tool, called with the same arguments,
// that end the agentic loop early. Three allows a transient failure plus
// one considered retry, whilst still catching a model that has locked on
// to repeating a call that cannot succeed; without it the loop grinds all
// the way to MAX_AGENTIC_LOOPS, spending a request on every attempt.
const MAX_IDENTICAL_TOOL_FAILURES = 3;
// Compact if estimated tokens exceed this threshold.
// Note: Anthropic rate limits are typically 30k-60k input tokens/minute cumulative.
// Setting lower allows multiple requests within the rate limit window.
const TOKEN_COMPACTION_THRESHOLD = 15000;
const RATE_LIMIT_RETRY_DELAY_MS = 60000; // 60 seconds

/**
 * Wraps a plain string into a typed content-block array for the
 * proxy wire format ({type: "text", text: "..."}). Accepts strings
 * that are already content-block arrays (returns them as-is) so the
 * helper is idempotent for messages already in block form.
 * @param {string|Array} content - Message content to wrap.
 * @returns {Array} - Array of content blocks.
 */
const toTextBlocks = (content) => {
    if (Array.isArray(content)) return content;
    return [{ type: 'text', text: typeof content === 'string' ? content : String(content || '') }];
};

/**
 * Converts an MCP tool result (string or array of {type, text}
 * blocks) into a flat text string for the proxy's tool_result block.
 * The library now expects a single `text` field on tool_result, not
 * a nested content array.
 * @param {*} result - MCP tool result content.
 * @returns {string} - Flattened text payload.
 */
const flattenToolResultText = (result) => {
    if (result == null) return '';
    if (typeof result === 'string') return result;
    if (Array.isArray(result)) {
        return result
            .map((item) => {
                if (typeof item === 'string') return item;
                if (item && typeof item === 'object') {
                    if (typeof item.text === 'string') return item.text;
                    return JSON.stringify(item);
                }
                return '';
            })
            .join('');
    }
    if (typeof result === 'object') {
        if (typeof result.text === 'string') return result.text;
        return JSON.stringify(result);
    }
    return String(result);
};

/**
 * Translates MCP tool descriptors (which use camelCase
 * `inputSchema`) into the snake_case `input_schema` field the
 * library proxy expects.
 * @param {Array} tools - MCP-style tool descriptors.
 * @returns {Array} - Tool descriptors using `input_schema`.
 */
const toProxyTools = (tools) => {
    if (!Array.isArray(tools)) return [];
    return tools.map((t) => {
        if (!t) return t;
        const { inputSchema, input_schema, ...rest } = t;
        return {
            ...rest,
            input_schema: input_schema || inputSchema || {},
        };
    });
};

/**
 * Builds the proxy-format tool_result message: role "tool" with a
 * single tool_result content block carrying tool_use_id, text, and
 * optional is_error.
 * @param {string} toolUseId - The tool_use id this result responds to.
 * @param {*} content - MCP result content.
 * @param {boolean} isError - Whether the result represents an error.
 * @returns {object} - Message envelope ready to send to the proxy.
 */
const buildToolResultMessage = (toolUseId, content, isError) => {
    const block = {
        type: 'tool_result',
        tool_use_id: toolUseId,
        text: flattenToolResultText(content),
    };
    if (isError) {
        block.is_error = true;
    }
    return { role: 'tool', content: [block] };
};

/**
 * Checks if an error is a rate limit error.
 * @param {number} status - HTTP status code
 * @param {string} errorText - Error message text
 * @returns {boolean} - True if this is a rate limit error
 */
const isRateLimitError = (status, errorText) => {
    if (status === 429) return true;
    if (errorText && errorText.toLowerCase().includes('rate limit')) return true;
    if (errorText && errorText.includes('tokens per minute')) return true;
    return false;
};

/**
 * Extracts rate limit details from an error message.
 * @param {string} errorText - Error message text
 * @returns {object} - Rate limit details
 */
const parseRateLimitError = (errorText) => {
    const details = {
        limit: null,
        message: 'Rate limit exceeded',
    };

    // Try to extract token limit from error message
    const tokenMatch = errorText.match(/(\d{1,3}(?:,\d{3})*)\s*(?:input\s+)?tokens?\s+per\s+minute/i);
    if (tokenMatch) {
        details.limit = tokenMatch[1];
        details.message = `Rate limit: ${tokenMatch[1]} input tokens per minute`;
    }

    return details;
};

/**
 * Creates a delay promise.
 * @param {number} ms - Milliseconds to delay
 * @returns {Promise} - Promise that resolves after delay
 */
const delay = (ms) => new Promise(resolve => setTimeout(resolve, ms));

/**
 * Estimates token count for a string using rough heuristic.
 * Uses ~3.5 characters per token (conservative estimate).
 * @param {string} text - Text to estimate
 * @returns {number} - Estimated token count
 */
const estimateTokensForText = (text) => {
    if (!text || typeof text !== 'string') return 0;
    // Rough heuristic: ~3.5 characters per token
    return Math.ceil(text.length / 3.5);
};

/**
 * Estimates token count for tool/resource result content.
 * @param {*} content - Tool result content (string, array, or object)
 * @returns {number} - Estimated token count
 */
const estimateToolResultTokens = (content) => {
    if (!content) return 0;

    // If it's a string, estimate directly
    if (typeof content === 'string') {
        return estimateTokensForText(content);
    }

    // If it's an array of content items (MCP format)
    if (Array.isArray(content)) {
        let total = 0;
        for (const item of content) {
            if (item.type === 'text' && item.text) {
                total += estimateTokensForText(item.text);
            } else if (typeof item === 'string') {
                total += estimateTokensForText(item);
            } else {
                // For other content types, stringify and estimate
                total += estimateTokensForText(JSON.stringify(item));
            }
        }
        return total;
    }

    // For objects, stringify and estimate
    if (typeof content === 'object') {
        return estimateTokensForText(JSON.stringify(content));
    }

    return 0;
};

/**
 * Token usage tracker - tracks actual token usage from LLM responses
 * to provide accurate cumulative counts for rate limit messages.
 */
const tokenUsageTracker = {
    // Array of { timestamp: Date, inputTokens: number, outputTokens: number }
    usageHistory: [],

    /**
     * Records token usage from an LLM response.
     * @param {object} tokenUsage - Token usage from LLM response
     */
    record(tokenUsage) {
        if (!tokenUsage) return;
        // API returns prompt_tokens/completion_tokens (not input_tokens/output_tokens)
        const inputTokens = tokenUsage.prompt_tokens || tokenUsage.input_tokens || 0;
        const outputTokens = tokenUsage.completion_tokens || tokenUsage.output_tokens || 0;
        console.log(`[Token Tracker] Recording: ${inputTokens} input, ${outputTokens} output tokens`);
        this.usageHistory.push({
            timestamp: Date.now(),
            inputTokens: inputTokens,
            outputTokens: outputTokens,
        });
        // Clean up old entries (older than 2 minutes)
        this.cleanup();
    },

    /**
     * Removes entries older than 2 minutes.
     */
    cleanup() {
        const twoMinutesAgo = Date.now() - 120000;
        this.usageHistory = this.usageHistory.filter(u => u.timestamp > twoMinutesAgo);
    },

    /**
     * Gets cumulative input tokens used in the last minute.
     * @returns {number} - Total input tokens in last 60 seconds
     */
    getInputTokensLastMinute() {
        const oneMinuteAgo = Date.now() - 60000;
        return this.usageHistory
            .filter(u => u.timestamp > oneMinuteAgo)
            .reduce((sum, u) => sum + u.inputTokens, 0);
    },

    /**
     * Gets the count of requests in the last minute.
     * @returns {number} - Number of requests in last 60 seconds
     */
    getRequestCountLastMinute() {
        const oneMinuteAgo = Date.now() - 60000;
        return this.usageHistory.filter(u => u.timestamp > oneMinuteAgo).length;
    },

    /**
     * Clears all usage history.
     */
    clear() {
        this.usageHistory = [];
    },
};

/**
 * Estimates total tokens in a message array.
 * @param {Array} messages - Array of message objects
 * @returns {number} - Estimated total token count
 */
const estimateTotalTokens = (messages) => {
    let total = 0;
    for (const msg of messages) {
        if (typeof msg.content === 'string') {
            total += estimateTokensForText(msg.content);
        } else if (Array.isArray(msg.content)) {
            // Handle text, tool_use and tool_result blocks.
            for (const item of msg.content) {
                if (!item) continue;
                if (item.type === 'tool_use' && item.tool_use) {
                    // New format: tool_use details nest under tool_use.
                    if (item.tool_use.input !== undefined) {
                        total += estimateTokensForText(JSON.stringify(item.tool_use.input));
                    }
                } else if (item.type === 'tool_result') {
                    // New format: tool_result uses the `text` field; the
                    // old format kept the payload under `content`.
                    if (typeof item.text === 'string') {
                        total += estimateTokensForText(item.text);
                    } else if (typeof item.content === 'string') {
                        total += estimateTokensForText(item.content);
                    } else if (Array.isArray(item.content)) {
                        for (const c of item.content) {
                            if (c && typeof c.text === 'string') {
                                total += estimateTokensForText(c.text);
                            }
                        }
                    }
                } else if (typeof item.text === 'string') {
                    total += estimateTokensForText(item.text);
                } else if (item.input !== undefined) {
                    // Defensive: legacy flat tool_use shape.
                    total += estimateTokensForText(JSON.stringify(item.input));
                }
            }
        }
        // Add overhead for message structure (~10 tokens per message)
        total += 10;
    }
    return total;
};

/**
 * Performs basic local compaction as a fallback.
 * Strategy: Keep the first user message and the last N messages.
 * @param {Array} messages - The full message history
 * @param {number} maxRecentMessages - Number of recent messages to keep
 * @returns {Array} - Compacted message history
 */
const localCompactMessages = (messages, maxRecentMessages = 10) => {
    const compacted = [];

    // Keep the first user message (original query)
    if (messages.length > 0 && messages[0].role === 'user') {
        compacted.push(messages[0]);
    }

    // Keep the last N messages
    const startIdx = Math.max(1, messages.length - maxRecentMessages);
    compacted.push(...messages.slice(startIdx));

    console.log(`[Local Compaction] Reduced messages from ${messages.length} to ${compacted.length} (kept first + last ${maxRecentMessages})`);

    return compacted;
};

/**
 * Attempts to use server-side smart compaction endpoint.
 * Falls back to local compaction if server call fails.
 * @param {Array} messages - The full message history
 * @param {string} sessionToken - Authentication token
 * @param {number} maxTokens - Maximum tokens allowed
 * @param {number} recentWindow - Number of recent messages to keep
 * @returns {Promise<Array>} - Compacted message history
 */
const compactMessages = async (messages, sessionToken, maxTokens = 100000, recentWindow = 10) => {
    const MAX_RECENT_MESSAGES = recentWindow;
    const MIN_MESSAGES_FOR_COMPACTION = 15; // Don't compact unless we have at least 15 messages
    const MIN_SAVINGS_THRESHOLD = 5; // Only compact if we can save at least 5 messages

    // Estimate total tokens in the conversation
    const estimatedTokens = estimateTotalTokens(messages);

    // Check if we should compact based on token count OR message count
    const shouldCompactByTokens = estimatedTokens > TOKEN_COMPACTION_THRESHOLD;
    const shouldCompactByMessages = messages.length >= MIN_MESSAGES_FOR_COMPACTION;

    // If neither threshold is met, skip compaction
    if (!shouldCompactByTokens && !shouldCompactByMessages) {
        return { messages, compacted: false };
    }

    // Log why we're compacting (for debugging)
    if (shouldCompactByTokens) {
        console.log(`[Compaction] Triggered by token count: ~${estimatedTokens} tokens (threshold: ${TOKEN_COMPACTION_THRESHOLD})`);
    } else {
        console.log(`[Compaction] Triggered by message count: ${messages.length} messages (threshold: ${MIN_MESSAGES_FOR_COMPACTION})`);
    }

    // Estimate if compaction would be worthwhile (only for message-based trigger)
    // With recentWindow=10 and keepAnchors=true, we keep at least: 1 (first) + 10 (recent) = 11
    // So we need at least 11 + MIN_SAVINGS_THRESHOLD messages to make it worthwhile
    // For token-based trigger, always proceed since we need to reduce tokens
    if (!shouldCompactByTokens && messages.length < (11 + MIN_SAVINGS_THRESHOLD)) {
        return { messages, compacted: false };
    }

    // Try server-side smart compaction
    try {
        const response = await fetch('/api/chat/compact', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${sessionToken}`
            },
            body: JSON.stringify({
                messages: messages,
                max_tokens: maxTokens,
                recent_window: recentWindow,
                keep_anchors: true
            })
        });

        if (response.ok) {
            const data = await response.json();
            console.log(`[Server Compaction] ${data.compaction_info.original_count} -> ${data.compaction_info.compacted_count} messages (dropped ${data.compaction_info.dropped_count}, saved ${data.compaction_info.tokens_saved} tokens, ratio ${data.compaction_info.compression_ratio.toFixed(2)})`);

            // Check if compaction actually saved any messages
            const messagesSaved = data.compaction_info.original_count - data.compaction_info.compacted_count;
            if (messagesSaved < MIN_SAVINGS_THRESHOLD) {
                // Compaction didn't save enough, return original messages without tracking
                console.log(`[Server Compaction] Skipped - only saved ${messagesSaved} messages (threshold: ${MIN_SAVINGS_THRESHOLD})`);
                return { messages, compacted: false };
            }

            return {
                messages: data.messages,
                compacted: true,
                originalCount: data.compaction_info.original_count,
                compactedCount: data.compaction_info.compacted_count,
                tokensSaved: data.compaction_info.tokens_saved
            };
        } else {
            console.warn(`[Server Compaction] Failed with status ${response.status}, using local fallback`);
        }
    } catch (error) {
        console.warn('[Server Compaction] Error occurred, using local fallback:', error.message);
    }

    // Fall back to local compaction
    const compactedMsgs = localCompactMessages(messages, MAX_RECENT_MESSAGES);
    const messagesSaved = messages.length - compactedMsgs.length;

    // Check if local compaction actually saved enough messages
    if (messagesSaved < MIN_SAVINGS_THRESHOLD) {
        console.log(`[Local Compaction] Skipped - only saved ${messagesSaved} messages (threshold: ${MIN_SAVINGS_THRESHOLD})`);
        return { messages, compacted: false };
    }

    return {
        messages: compactedMsgs,
        compacted: true,
        originalCount: messages.length,
        compactedCount: compactedMsgs.length,
        local: true
    };
};

const ChatInterface = ({ conversations }) => {
    const { sessionToken, forceLogout } = useAuth();
    const { setIsProcessing } = useLLMProcessing();
    const { registerActions } = useConversationActions();
    const theme = useTheme();
    const isDark = theme.palette.mode === 'dark';

    // State management
    const [messages, setMessages] = useState([]);
    const [isLoadingConversation, setIsLoadingConversation] = useState(false);

    // Ref to track if we're in the middle of loading (to avoid saving back what we just loaded)
    const isLoadingRef = React.useRef(false);
    // Ref to track if we need to save (only after an assistant response)
    const pendingSaveRef = React.useRef(false);
    // Ref to track previous conversation ID for saving before switch
    const previousConversationIdRef = React.useRef(null);
    // Ref to track current messages for saving (since state may be stale in callbacks)
    const messagesRef = React.useRef([]);

    // Keep messagesRef in sync with messages state
    React.useEffect(() => {
        messagesRef.current = messages;
    }, [messages]);

    const [showActivity, setShowActivity] = useLocalStorageBoolean('show-activity', true);
    const [renderMarkdown, setRenderMarkdown] = useLocalStorageBoolean('render-markdown', true);
    const [debug, setDebug] = useLocalStorageBoolean('debug', false);

    const [input, setInput] = useState('');
    const [loading, setLoading] = useState(false);

    // AbortController for cancelling requests
    const abortControllerRef = useRef(null);

    // Prompt popover state
    const [promptPopoverAnchor, setPromptPopoverAnchor] = useState(null);
    const [executingPrompt, setExecutingPrompt] = useState(false);

    // Write query confirmation state
    const [writeConfirmState, setWriteConfirmState] = useState(null);

    // Custom hooks for functionality
    const queryHistory = useQueryHistory();
    const { mcpClient, tools, prompts, refreshTools } = useMCPClient(sessionToken);
    const llmProviders = useLLMProviders(sessionToken);
    const { databases, currentDatabase, selectDatabase, fetchDatabases } = useDatabaseContext();

    // Refresh tools when database changes to get updated tool descriptions
    // (e.g., write access status for query_database tool)
    useEffect(() => {
        if (currentDatabase && refreshTools) {
            refreshTools();
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [currentDatabase]);

    // Log prompts when they're available (for debugging)
    useEffect(() => {
        if (prompts.length > 0) {
            console.log('MCP prompts available:', prompts);
        }
    }, [prompts]);

    // Sync loading state with context for other components to use
    useEffect(() => {
        setIsProcessing(loading);
    }, [loading, setIsProcessing]);

    /**
     * Returns a Promise that resolves to true (confirmed) or false (declined)
     * when the user responds to the write query confirmation dialog.
     * @param {string} query - The SQL query to confirm
     * @returns {Promise<boolean>} - True if user confirmed, false if cancelled
     */
    const requestWriteConfirmation = useCallback((query) => {
        return new Promise((resolve) => {
            setWriteConfirmState({ query, resolve });
        });
    }, []);

    /**
     * Checks whether the current database has write access enabled.
     * @returns {boolean} - True if writes are allowed on the current database
     */
    const isWriteAccessEnabled = useCallback(() => {
        if (!currentDatabase || !databases || databases.length === 0) {
            return false;
        }
        const db = databases.find(d => d.name === currentDatabase);
        return db?.allow_writes === true;
    }, [currentDatabase, databases]);

    // Load conversation when selected conversation changes
    useEffect(() => {
        const loadConversation = async () => {
            if (!conversations) return;

            const newConversationId = conversations.currentConversationId;
            const previousConversationId = previousConversationIdRef.current;

            // Save the previous conversation before switching (if it has messages)
            // This ensures provider/model changes are saved even without new messages
            if (previousConversationId && previousConversationId !== newConversationId) {
                const currentMessages = messagesRef.current;
                if (currentMessages.length > 0) {
                    try {
                        // Remove UI-only properties before saving
                        const messagesToSave = currentMessages.map(({ fromPreviousSession: _fp, isThinking: _it, ...msg }) => msg);
                        await conversations.saveConversation(
                            messagesToSave,
                            previousConversationId,
                            llmProviders.selectedProvider,
                            llmProviders.selectedModel,
                            currentDatabase || ''
                        );
                    } catch (error) {
                        console.error('Failed to save previous conversation:', error);
                    }
                }
            }

            // Update the previous conversation ID ref
            previousConversationIdRef.current = newConversationId;

            // Skip loading if we just saved a new conversation (going from null to an ID)
            // The messages are already in state, no need to reload them
            // But only skip if we actually have messages - otherwise this is a fresh session
            // where the user is selecting an existing conversation to load
            if (!previousConversationId && newConversationId && messagesRef.current.length > 0) {
                return;
            }

            // If no conversation is selected, clear messages for new conversation
            if (!newConversationId) {
                isLoadingRef.current = true;
                setMessages([]);
                queryHistory.clearHistory();
                // Small delay to ensure state update before clearing the flag
                setTimeout(() => { isLoadingRef.current = false; }, 100);
                return;
            }

            isLoadingRef.current = true;
            setIsLoadingConversation(true);
            try {
                const conv = await conversations.getConversation(newConversationId);
                if (conv && conv.messages) {
                    // Mark messages as from previous session
                    setMessages(conv.messages.map(msg => ({
                        ...msg,
                        content: typeof msg.content === 'string' ? msg.content : JSON.stringify(msg.content),
                        fromPreviousSession: true
                    })));

                    // Restore provider and model if they were saved
                    // Use the restoreProviderAndModel function to handle proper sequencing
                    if (llmProviders.restoreProviderAndModel) {
                        const hasProvider = conv.provider && conv.provider.length > 0;
                        const hasModel = conv.model && conv.model.length > 0;
                        if (hasProvider || hasModel) {
                            llmProviders.restoreProviderAndModel(
                                hasProvider ? conv.provider : llmProviders.selectedProvider,
                                hasModel ? conv.model : null
                            );
                        }
                    }

                    // Restore connection if it was saved
                    if (conv.connection && conv.connection.length > 0 && selectDatabase) {
                        selectDatabase(conv.connection);
                    }

                    // Restore query history from user messages
                    const userQueries = conv.messages
                        .filter(msg => msg.role === 'user')
                        .map(msg => typeof msg.content === 'string' ? msg.content : '')
                        .filter(content => content.length > 0);
                    queryHistory.setHistory(userQueries);
                }
            } catch (error) {
                console.error('Failed to load conversation:', error);
            } finally {
                setIsLoadingConversation(false);
                // Small delay to ensure state update before clearing the flag
                setTimeout(() => { isLoadingRef.current = false; }, 100);
            }
        };

        loadConversation();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [conversations?.currentConversationId, conversations?.getConversation]);

    // Save conversation when messages change (after an assistant response)
    useEffect(() => {
        const saveConversation = async () => {
            // Skip if loading, no conversations hook, no messages, or no pending save
            if (isLoadingRef.current || !conversations || messages.length === 0 || !pendingSaveRef.current) {
                return;
            }

            // Check if last message is an assistant message (not thinking)
            const lastMessage = messages[messages.length - 1];
            if (lastMessage.role !== 'assistant' || lastMessage.isThinking) {
                return;
            }

            // Reset the pending save flag
            pendingSaveRef.current = false;

            try {
                // Remove UI-only properties before saving
                const messagesToSave = messages.map(({ fromPreviousSession: _fp, isThinking: _it, ...msg }) => msg);

                // Save or update conversation with current provider/model/connection
                const result = await conversations.saveConversation(
                    messagesToSave,
                    conversations.currentConversationId,
                    llmProviders.selectedProvider,
                    llmProviders.selectedModel,
                    currentDatabase || ''
                );

                // If this was a new conversation, update the current conversation ID
                if (result && !conversations.currentConversationId) {
                    conversations.setCurrentConversationId(result.id);
                }
            } catch (error) {
                console.error('Failed to save conversation:', error);
            }
        };

        saveConversation();
    }, [messages, conversations, llmProviders.selectedProvider, llmProviders.selectedModel, currentDatabase]);

    // Handle message sending
    const handleSend = useCallback(async () => {
        if (!input.trim() || loading || !mcpClient) return;

        const userMessage = {
            role: 'user',
            content: input.trim(),
            timestamp: new Date().toISOString(),
        };

        // Add to history
        queryHistory.addToHistory(userMessage.content);

        // Create thinking message placeholder
        const thinkingMessage = {
            role: 'assistant',
            content: '',
            timestamp: new Date().toISOString(),
            provider: llmProviders.selectedProvider,
            model: llmProviders.selectedModel,
            activity: [],
            isThinking: true,
        };

        setMessages(prev => [...prev, userMessage, thinkingMessage]);
        setInput('');
        setLoading(true);

        // Create AbortController for this request
        const abortController = new AbortController();
        abortControllerRef.current = abortController;

        try {
            // Build conversation history. The proxy wire format expects
            // each message's content as an array of typed content
            // blocks ([{type:"text", text:"..."}]); the UI keeps content
            // as plain strings, so wrap them here on the way out.
            const conversationMessages = [];

            // Add all previous messages
            for (const msg of messages) {
                if (msg.role === 'user') {
                    conversationMessages.push({
                        role: 'user',
                        content: toTextBlocks(msg.content),
                    });
                } else if (msg.role === 'assistant' && msg.content) {
                    conversationMessages.push({
                        role: 'assistant',
                        content: toTextBlocks(msg.content),
                    });
                }
            }

            // Add current user message
            conversationMessages.push({
                role: 'user',
                content: toTextBlocks(userMessage.content),
            });

            const activity = [];
            let loopCount = 0;
            let rateLimitRetryCount = 0;
            const toolRepeatGuard = createToolRepeatGuard(MAX_IDENTICAL_TOOL_FAILURES);

            // Agentic loop
            while (loopCount < MAX_AGENTIC_LOOPS) {
                loopCount++;

                // Compact message history to prevent token overflow
                const compactionResult = await compactMessages(conversationMessages, sessionToken);
                const compactedMessages = compactionResult.messages;

                // Track compaction activity if it occurred
                if (compactionResult.compacted) {
                    activity.push({
                        type: 'compaction',
                        originalCount: compactionResult.originalCount,
                        compactedCount: compactionResult.compactedCount,
                        tokensSaved: compactionResult.tokensSaved,
                        local: compactionResult.local || false
                    });

                    // Update thinking message with compaction activity
                    setMessages(prev => {
                        const newMessages = [...prev];
                        if (newMessages.length > 0) {
                            newMessages[newMessages.length - 1] = {
                                ...newMessages[newMessages.length - 1],
                                activity: [...activity]
                            };
                        }
                        return newMessages;
                    });
                }

                // Call LLM with compacted history via the streaming
                // endpoint. The proxy lives under /api/llm/v1/* and
                // always returns token usage on the done frame. Text
                // chunks arrive incrementally and update the assistant
                // bubble in real time; tool_use blocks are assembled
                // by sseChat into the same shape /v1/chat returns so
                // the agentic loop below is unchanged.
                let streamedText = '';
                let llmData;
                try {
                    llmData = await sseChat({
                        messages: compactedMessages,
                        tools: toProxyTools(tools),
                        provider: llmProviders.selectedProvider,
                        model: llmProviders.selectedModel,
                    }, {
                        signal: abortController.signal,
                        sessionToken,
                        onTextChunk: (chunk) => {
                            if (!chunk) return;
                            streamedText += chunk;
                            // Stream the partial text into the thinking
                            // bubble so the user sees progress as it
                            // arrives. We keep isThinking=true so the
                            // indicator and activity panel keep working
                            // until the final response is committed.
                            setMessages(prev => {
                                const newMessages = [...prev];
                                const last = newMessages[newMessages.length - 1];
                                if (last && last.isThinking) {
                                    newMessages[newMessages.length - 1] = {
                                        ...last,
                                        content: streamedText,
                                    };
                                }
                                return newMessages;
                            });
                        },
                    });
                } catch (streamErr) {
                    // Re-raise AbortError so the outer handler converts
                    // it to a "Request cancelled" message.
                    if (streamErr.name === 'AbortError') {
                        throw streamErr;
                    }

                    // Session expired.
                    if (streamErr.status === 401) {
                        console.log('Session invalidated, logging out...');
                        setMessages(prev => {
                            const newMessages = [...prev];
                            if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                                const thinkingMsg = newMessages[newMessages.length - 1];
                                newMessages[newMessages.length - 1] = {
                                    role: 'assistant',
                                    content: 'Error: Your session has expired. Please log in again.',
                                    timestamp: new Date().toISOString(),
                                    provider: thinkingMsg.provider,
                                    model: thinkingMsg.model,
                                    activity: thinkingMsg.activity || [],
                                    isError: true
                                };
                            }
                            return newMessages;
                        });
                        forceLogout();
                        return;
                    }

                    // Rate limit handling mirrors the non-streaming path.
                    const errorText = streamErr.body || streamErr.message || '';
                    if (isRateLimitError(streamErr.status, errorText)) {
                        rateLimitRetryCount++;
                        const rateLimitDetails = parseRateLimitError(errorText);
                        const estimatedTokens = estimateTotalTokens(compactedMessages);
                        const cumulativeTokens = tokenUsageTracker.getInputTokensLastMinute();
                        const requestCount = tokenUsageTracker.getRequestCountLastMinute();

                        if (rateLimitRetryCount === 1) {
                            // First rate limit hit - pause and retry
                            console.log('[Rate Limit] First hit, pausing for 60 seconds before retry...');
                            console.log(`[Rate Limit] Cumulative tokens in last minute: ${cumulativeTokens}, requests: ${requestCount}`);

                            activity.push({
                                type: 'rate_limit_pause',
                                timestamp: new Date().toISOString(),
                                message: rateLimitDetails.message,
                                estimatedTokens: estimatedTokens,
                                cumulativeTokens: cumulativeTokens,
                                requestCount: requestCount,
                            });

                            setMessages(prev => {
                                const newMessages = [...prev];
                                if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                                    newMessages[newMessages.length - 1] = {
                                        ...newMessages[newMessages.length - 1],
                                        activity: [...activity]
                                    };
                                }
                                return newMessages;
                            });

                            await delay(RATE_LIMIT_RETRY_DELAY_MS);
                            loopCount--;
                            continue;
                        } else {
                            const tokenInfo = cumulativeTokens > 0
                                ? `Tokens used in last minute: ~${cumulativeTokens.toLocaleString()} (${requestCount} requests)`
                                : `Estimated tokens in this request: ~${estimatedTokens.toLocaleString()}`;
                            const friendlyError = `Rate limit exceeded. The API has a limit of ${rateLimitDetails.limit || 'N'} input tokens per minute.\n\n` +
                                `${tokenInfo}\n\n` +
                                `To resolve this:\n` +
                                `1. Clear the conversation history and try again\n` +
                                `2. Wait a minute before sending another request\n` +
                                `3. Try a shorter query or use a different LLM provider`;
                            throw new Error(friendlyError);
                        }
                    }

                    throw new Error(`LLM request failed: ${streamErr.message}`);
                }

                // Track token usage for rate limit awareness. The proxy
                // now returns usage under the `usage` key (was
                // `token_usage`); fall back for safety.
                const usage = llmData.usage || llmData.token_usage;
                console.log('[Token Debug] llmData.usage:', usage);
                tokenUsageTracker.record(usage);

                // Reset rate limit retry counter after successful response
                rateLimitRetryCount = 0;

                console.log('LLM response:', llmData);
                console.log('Loop iteration:', loopCount, 'Stop reason:', llmData.stop_reason);
                if (llmData.stop_reason === 'tool_use') {
                    const toolUseCount = llmData.content.filter(c => c.type === 'tool_use').length;
                    console.log('Number of tool_use blocks in this response:', toolUseCount);
                }

                // Check stop reason
                if (llmData.stop_reason === 'end_turn' || loopCount >= MAX_AGENTIC_LOOPS) {
                    // Final response - extract text content
                    let textContent = '';
                    const contentArray = Array.isArray(llmData.content) ? llmData.content : [llmData.content];

                    for (const content of contentArray) {
                        if (content && content.type === 'text') {
                            const text = typeof content.text === 'string' ? content.text : String(content.text || '');
                            textContent += text;
                        }
                    }

                    const finalContent = textContent || 'No response received';

                    // Replace thinking message with final response
                    console.log('Final activity array:', activity);
                    console.log('Total tool uses tracked:', activity.length);

                    // Mark for saving after message update
                    pendingSaveRef.current = true;

                    setMessages(prev => {
                        const newMessages = prev.slice(0, -1);
                        return [...newMessages, {
                            role: 'assistant',
                            content: finalContent,
                            timestamp: new Date().toISOString(),
                            provider: llmProviders.selectedProvider,
                            model: llmProviders.selectedModel,
                            activity: activity,
                            tokenUsage: usage,
                        }];
                    });
                    break;
                }

                // Handle tool use
                if (llmData.stop_reason === 'tool_use') {
                    const toolUses = llmData.content.filter(c => c.type === 'tool_use');

                    if (toolUses.length === 0) {
                        throw new Error('LLM indicated tool_use but no tool_use blocks found');
                    }

                    // Execute tools. The new wire format nests the call
                    // details under a `tool_use` object; pull them out
                    // once per block.
                    const toolResultMessages = [];
                    for (const toolUseBlock of toolUses) {
                        const toolUse = toolUseBlock.tool_use || toolUseBlock;
                        const toolUseId = toolUse.id;
                        const toolName = toolUse.name;
                        const toolInput = toolUse.input || {};
                        console.log('Executing tool:', toolName, 'with args:', toolInput);

                        // Add initial activity entry (token count will be updated after execution)
                        const activityIndex = activity.length;
                        const activityEntry = {
                            type: 'tool',
                            name: toolName,
                            timestamp: new Date().toISOString(),
                            tokens: null, // Will be updated after execution
                        };
                        // For read_resource, capture the URI being accessed
                        if (toolName === 'read_resource' && toolInput?.uri) {
                            activityEntry.uri = toolInput.uri;
                        }
                        activity.push(activityEntry);

                        // Update thinking message with new activity
                        setMessages(prev => {
                            const newMessages = [...prev];
                            if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                                // Create a new message object instead of mutating
                                newMessages[newMessages.length - 1] = {
                                    ...newMessages[newMessages.length - 1],
                                    activity: [...activity]
                                };
                            }
                            return newMessages;
                        });

                        // Check whether this call needs confirmation. This
                        // covers custom tools that can write, not only
                        // query_database.
                        if (isWriteAccessEnabled()) {
                            const { needsConfirmation, subject } =
                                writeConfirmationSubject(tools, toolName, toolInput);
                            if (needsConfirmation) {
                                const confirmed = await requestWriteConfirmation(subject);
                                if (!confirmed) {
                                    activity[activityIndex].isError = true;
                                    activity[activityIndex].tokens = 0;
                                    const declinedText = 'Execution was declined by the user. Do not retry this call. Ask the user how they would like to proceed.';
                                    toolRepeatGuard.record({
                                        name: toolName,
                                        input: toolInput,
                                        isError: true,
                                        resultText: declinedText,
                                    });
                                    toolResultMessages.push(buildToolResultMessage(
                                        toolUseId,
                                        declinedText,
                                        true,
                                    ));
                                    continue;
                                }
                            }
                        }

                        try {
                            // Execute tool via MCP
                            const result = await mcpClient.callTool(toolName, toolInput);
                            console.log('Tool result:', result);

                            // Estimate tokens in the result
                            const resultTokens = estimateToolResultTokens(result.content);
                            activity[activityIndex].tokens = resultTokens;
                            activity[activityIndex].isError = result.isError || false;

                            // Track repeated identical failures so the loop
                            // can stop rather than retry indefinitely.
                            toolRepeatGuard.record({
                                name: toolName,
                                input: toolInput,
                                isError: Boolean(result.isError),
                                resultText: flattenToolResultText(result.content),
                            });

                            // Update thinking message with token count
                            setMessages(prev => {
                                const newMessages = [...prev];
                                if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                                    newMessages[newMessages.length - 1] = {
                                        ...newMessages[newMessages.length - 1],
                                        activity: [...activity]
                                    };
                                }
                                return newMessages;
                            });

                            toolResultMessages.push(buildToolResultMessage(
                                toolUseId,
                                result.content,
                                Boolean(result.isError),
                            ));

                            // Refresh tools if manage_connections was called
                            if (toolName === 'manage_connections' && !result.isError) {
                                await refreshTools();
                            }

                            // Refresh database state if LLM switched databases
                            if (toolName === 'select_database_connection' && !result.isError) {
                                await fetchDatabases();
                            }
                        } catch (toolError) {
                            console.error('Tool execution error:', toolError);
                            const errorContent = `Error: ${toolError.message}`;
                            activity[activityIndex].tokens = estimateToolResultTokens(errorContent);
                            activity[activityIndex].isError = true;

                            toolRepeatGuard.record({
                                name: toolName,
                                input: toolInput,
                                isError: true,
                                resultText: errorContent,
                            });

                            toolResultMessages.push(buildToolResultMessage(
                                toolUseId,
                                errorContent,
                                true,
                            ));
                        }
                    }

                    // If the same call has now failed repeatedly, stop here
                    // and explain why, rather than sending the model back
                    // round to repeat it.
                    const repeatedFailure = toolRepeatGuard.getTripped();
                    if (repeatedFailure) {
                        pendingSaveRef.current = true;
                        setMessages(prev => {
                            const newMessages = prev.slice(0, -1);
                            return [...newMessages, {
                                role: 'assistant',
                                content: buildRepeatedToolFailureMessage(repeatedFailure),
                                timestamp: new Date().toISOString(),
                                provider: llmProviders.selectedProvider,
                                model: llmProviders.selectedModel,
                                activity: activity,
                                tokenUsage: usage,
                            }];
                        });
                        break;
                    }

                    // Echo the assistant turn (text + tool_use blocks) back
                    // into the conversation history so the next request
                    // includes it.
                    conversationMessages.push({
                        role: 'assistant',
                        content: llmData.content,
                    });

                    // Each tool result is its own role-"tool" message in
                    // the new wire format.
                    for (const toolResultMsg of toolResultMessages) {
                        conversationMessages.push(toolResultMsg);
                    }

                    // Continue loop
                    continue;
                }

                // Unknown stop reason
                throw new Error(`Unexpected stop reason: ${llmData.stop_reason}`);
            }

            if (loopCount >= MAX_AGENTIC_LOOPS) {
                throw new Error('Maximum tool execution loops reached');
            }
        } catch (err) {
            // Check if this was a user cancellation
            if (err.name === 'AbortError') {
                console.log('Request cancelled by user');
                // Convert thinking message to cancelled message
                setMessages(prev => {
                    const newMessages = [...prev];
                    if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                        const thinkingMsg = newMessages[newMessages.length - 1];
                        newMessages[newMessages.length - 1] = {
                            role: 'assistant',
                            content: 'Request cancelled',
                            timestamp: new Date().toISOString(),
                            provider: thinkingMsg.provider,
                            model: thinkingMsg.model,
                            activity: thinkingMsg.activity || [],
                            isCancelled: true
                        };
                    }
                    return newMessages;
                });
                return;
            }

            console.error('Chat error:', err);

            // Convert thinking message to error message (preserve activity for debugging)
            setMessages(prev => {
                const newMessages = [...prev];
                if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                    const thinkingMsg = newMessages[newMessages.length - 1];
                    newMessages[newMessages.length - 1] = {
                        role: 'assistant',
                        content: `Error: ${err.message || 'Failed to send message'}`,
                        timestamp: new Date().toISOString(),
                        provider: thinkingMsg.provider,
                        model: thinkingMsg.model,
                        activity: thinkingMsg.activity || [],
                        isError: true
                    };
                }
                return newMessages;
            });

        } finally {
            setLoading(false);
            abortControllerRef.current = null;
        }
    }, [input, loading, mcpClient, messages, sessionToken, tools, llmProviders.selectedProvider, llmProviders.selectedModel, queryHistory, forceLogout, refreshTools, fetchDatabases, isWriteAccessEnabled, requestWriteConfirmation]);

    // Handle request cancellation
    const handleCancel = useCallback(() => {
        if (abortControllerRef.current) {
            abortControllerRef.current.abort();
        }
        // Dismiss any pending write confirmation dialog
        if (writeConfirmState) {
            writeConfirmState.resolve(false);
            setWriteConfirmState(null);
        }
    }, [writeConfirmState]);

    // Handle keyboard shortcuts
    const handleKeyDown = useCallback((e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            handleSend();
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            const newInput = queryHistory.navigateUp(input);
            setInput(newInput);
        } else if (e.key === 'ArrowDown') {
            e.preventDefault();
            const newInput = queryHistory.navigateDown(input);
            setInput(newInput);
        }
    }, [input, queryHistory, handleSend]);

    // Handle input change
    const handleInputChange = useCallback((e) => {
        setInput(e.target.value);
        // Reset history navigation when user types
        if (queryHistory.isNavigating) {
            queryHistory.resetNavigation();
        }
    }, [queryHistory]);

    // Handle clear conversation. Confirmation is handled by the caller
    // (the StatusBanner header menu shows a confirmation dialog), so this
    // performs the clearing unconditionally.
    const handleClear = useCallback(() => {
        // Clear local state
        isLoadingRef.current = true;
        setMessages([]);
        queryHistory.clearHistory();

        // Start new conversation in the conversations system
        if (conversations) {
            conversations.startNewConversation();
        }

        // Reset loading flag after state update
        setTimeout(() => { isLoadingRef.current = false; }, 100);
    }, [queryHistory, conversations]);

    // Export the current conversation to a Markdown file. No-op when there
    // are no messages to export.
    const handleSaveConversation = useCallback(() => {
        if (messages.length === 0) return;
        const markdown = conversationToMarkdown(messages, { showActivity, debug });
        const filename = `chat-history-${new Date().toISOString().slice(0, 10)}.md`;
        downloadMarkdown(markdown, filename);
    }, [messages, showActivity, debug]);

    // Publish conversation-level actions to the StatusBanner header menu.
    //
    // handleClear and handleSaveConversation are rebuilt on most renders,
    // because useConversations and useQueryHistory each return a fresh object
    // and those objects are their dependencies. Publishing them directly made
    // this effect re-run on every render, and since the context stored a new
    // object each time, that re-rendered every consumer and re-ran the effect:
    // an update loop that React 19 reports as "Maximum update depth exceeded".
    // So keep the live callbacks in refs and publish stable wrappers, leaving
    // hasMessages as the only value that actually changes.
    const handleClearRef = useRef(handleClear);
    const handleSaveConversationRef = useRef(handleSaveConversation);
    handleClearRef.current = handleClear;
    handleSaveConversationRef.current = handleSaveConversation;

    const publishedClear = useCallback(() => handleClearRef.current(), []);
    const publishedSave = useCallback(() => handleSaveConversationRef.current(), []);

    const hasMessages = messages.length > 0;
    useEffect(() => {
        registerActions({
            hasMessages,
            onSave: publishedSave,
            onClear: publishedClear,
        });
    }, [hasMessages, publishedSave, publishedClear, registerActions]);

    // Handle prompt selection
    const handlePromptClick = useCallback((event) => {
        setPromptPopoverAnchor(event.currentTarget);
    }, []);

    // Handle prompt execution
    const handlePromptExecute = useCallback(async (promptName, args) => {
        if (!mcpClient || loading) return;

        setExecutingPrompt(true);

        // Create AbortController for this request so the existing
        // cancel button (wired to handleCancel via abortControllerRef)
        // can terminate an in-flight streaming response. Share the ref
        // with handleSend; only one flow can be active at a time
        // because both gate on the `loading` flag.
        const abortController = new AbortController();
        abortControllerRef.current = abortController;

        try {
            // Get the prompt with arguments from MCP server
            const promptResult = await mcpClient.getPrompt(promptName, args);

            // Add a system message to indicate prompt execution with parameters
            const paramStr = Object.entries(args)
                .filter(([, value]) => value !== '')
                .map(([key, value]) => `${key}="${value}"`)
                .join(', ');
            const systemMessage = {
                role: 'system',
                content: paramStr
                    ? `Executing prompt: ${promptName} (${paramStr})`
                    : `Executing prompt: ${promptName}`,
                timestamp: new Date().toISOString(),
            };
            setMessages(prev => [...prev, systemMessage]);

            // Build conversation history (exclude system messages). The
            // proxy expects each content as an array of typed blocks.
            const conversationMessages = [];
            for (const msg of messages) {
                if (msg.role === 'user') {
                    conversationMessages.push({
                        role: 'user',
                        content: toTextBlocks(msg.content),
                    });
                } else if (msg.role === 'assistant' && msg.content) {
                    conversationMessages.push({
                        role: 'assistant',
                        content: toTextBlocks(msg.content),
                    });
                }
            }
            // Add prompt messages to conversation history (but not to UI display)
            if (promptResult.messages) {
                for (const msg of promptResult.messages) {
                    if (msg.role === 'user') {
                        // MCP returns the prompt body under msg.content.text.
                        conversationMessages.push({
                            role: 'user',
                            content: toTextBlocks(msg.content?.text || ''),
                        });
                    }
                }
            }

            // Create thinking message placeholder
            const thinkingMessage = {
                role: 'assistant',
                content: '',
                timestamp: new Date().toISOString(),
                provider: llmProviders.selectedProvider,
                model: llmProviders.selectedModel,
                isThinking: true,
                activity: [],
            };
            setMessages(prev => [...prev, thinkingMessage]);

            // Trigger the agentic loop with the prompt messages
            setLoading(true);

            // Start agentic loop (similar to handleSend but using prompt messages)
            let loopCount = 0;
            const activity = [];
            let rateLimitRetryCount = 0;
            const toolRepeatGuard = createToolRepeatGuard(MAX_IDENTICAL_TOOL_FAILURES);

            while (loopCount < MAX_AGENTIC_LOOPS) {
                loopCount++;

                // Compact message history to prevent token overflow
                const compactionResult = await compactMessages(conversationMessages, sessionToken);
                const compactedMessages = compactionResult.messages;

                // Track compaction activity if it occurred
                if (compactionResult.compacted) {
                    activity.push({
                        type: 'compaction',
                        originalCount: compactionResult.originalCount,
                        compactedCount: compactionResult.compactedCount,
                        tokensSaved: compactionResult.tokensSaved,
                        local: compactionResult.local || false
                    });

                    // Update thinking message with compaction activity
                    setMessages(prev => {
                        const newMessages = [...prev];
                        if (newMessages.length > 0) {
                            newMessages[newMessages.length - 1] = {
                                ...newMessages[newMessages.length - 1],
                                activity: [...activity]
                            };
                        }
                        return newMessages;
                    });
                }

                // Make LLM request with compacted history via the
                // streaming endpoint. Text chunks update the thinking
                // bubble as they arrive; tool_use blocks are assembled
                // by sseChat into the same shape /v1/chat returns.
                let streamedText = '';
                let llmData;
                try {
                    llmData = await sseChat({
                        messages: compactedMessages,
                        tools: toProxyTools(tools),
                        provider: llmProviders.selectedProvider,
                        model: llmProviders.selectedModel,
                    }, {
                        signal: abortController.signal,
                        sessionToken,
                        onTextChunk: (chunk) => {
                            if (!chunk) return;
                            streamedText += chunk;
                            setMessages(prev => {
                                const newMessages = [...prev];
                                const last = newMessages[newMessages.length - 1];
                                if (last && last.isThinking) {
                                    newMessages[newMessages.length - 1] = {
                                        ...last,
                                        content: streamedText,
                                    };
                                }
                                return newMessages;
                            });
                        },
                    });
                } catch (streamErr) {
                    // Re-raise AbortError so the outer handler converts
                    // it to a "Request cancelled" message.
                    if (streamErr.name === 'AbortError') {
                        throw streamErr;
                    }

                    if (streamErr.status === 401) {
                        forceLogout();
                        throw new Error('Session expired. Please login again.');
                    }

                    const errorText = streamErr.body || streamErr.message || '';
                    if (isRateLimitError(streamErr.status, errorText)) {
                        rateLimitRetryCount++;
                        const rateLimitDetails = parseRateLimitError(errorText);
                        const estimatedTokens = estimateTotalTokens(compactedMessages);
                        const cumulativeTokens = tokenUsageTracker.getInputTokensLastMinute();
                        const requestCount = tokenUsageTracker.getRequestCountLastMinute();

                        if (rateLimitRetryCount === 1) {
                            // First rate limit hit - pause and retry
                            console.log('[Rate Limit] First hit, pausing for 60 seconds before retry...');
                            console.log(`[Rate Limit] Cumulative tokens in last minute: ${cumulativeTokens}, requests: ${requestCount}`);

                            activity.push({
                                type: 'rate_limit_pause',
                                timestamp: new Date().toISOString(),
                                message: rateLimitDetails.message,
                                estimatedTokens: estimatedTokens,
                                cumulativeTokens: cumulativeTokens,
                                requestCount: requestCount,
                            });

                            setMessages(prev => {
                                const newMessages = [...prev];
                                if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                                    newMessages[newMessages.length - 1] = {
                                        ...newMessages[newMessages.length - 1],
                                        activity: [...activity]
                                    };
                                }
                                return newMessages;
                            });

                            await delay(RATE_LIMIT_RETRY_DELAY_MS);
                            loopCount--;
                            continue;
                        } else {
                            const tokenInfo = cumulativeTokens > 0
                                ? `Tokens used in last minute: ~${cumulativeTokens.toLocaleString()} (${requestCount} requests)`
                                : `Estimated tokens in this request: ~${estimatedTokens.toLocaleString()}`;
                            const friendlyError = `Rate limit exceeded. The API has a limit of ${rateLimitDetails.limit || 'N'} input tokens per minute.\n\n` +
                                `${tokenInfo}\n\n` +
                                `To resolve this:\n` +
                                `1. Clear the conversation history and try again\n` +
                                `2. Wait a minute before sending another request\n` +
                                `3. Try a shorter query or use a different LLM provider`;
                            throw new Error(friendlyError);
                        }
                    }

                    throw new Error(`Server error: ${errorText || streamErr.message}`);
                }

                // Track token usage for rate limit awareness. Proxy now
                // returns usage under `usage`; fall back for safety.
                const usage = llmData.usage || llmData.token_usage;
                console.log('[Token Debug] llmData.usage:', usage);
                tokenUsageTracker.record(usage);

                // Reset rate limit retry counter after successful response
                rateLimitRetryCount = 0;

                // Handle end_turn
                if (llmData.stop_reason === 'end_turn') {
                    const finalContent = llmData.content
                        .filter(c => c.type === 'text')
                        .map(c => c.text)
                        .join('\n');

                    // Mark for saving after message update
                    pendingSaveRef.current = true;

                    // Replace thinking message with actual response
                    setMessages(prev => {
                        const newMessages = prev.slice(0, -1);
                        return [...newMessages, {
                            role: 'assistant',
                            content: finalContent,
                            timestamp: new Date().toISOString(),
                            provider: llmProviders.selectedProvider,
                            model: llmProviders.selectedModel,
                            activity: activity,
                            tokenUsage: usage,
                        }];
                    });
                    break;
                }

                // Handle tool use
                if (llmData.stop_reason === 'tool_use') {
                    const toolUses = llmData.content.filter(c => c.type === 'tool_use');

                    if (toolUses.length === 0) {
                        throw new Error('LLM indicated tool_use but no tool_use blocks found');
                    }

                    // Execute tools. The new wire format nests the call
                    // details under a `tool_use` object.
                    const toolResultMessages = [];
                    for (const toolUseBlock of toolUses) {
                        const toolUse = toolUseBlock.tool_use || toolUseBlock;
                        const toolUseId = toolUse.id;
                        const toolName = toolUse.name;
                        const toolInput = toolUse.input || {};

                        // Add initial activity entry (token count will be updated after execution)
                        const activityIndex = activity.length;
                        const activityEntry = {
                            type: 'tool',
                            name: toolName,
                            timestamp: new Date().toISOString(),
                            tokens: null, // Will be updated after execution
                        };
                        // For read_resource, capture the URI being accessed
                        if (toolName === 'read_resource' && toolInput?.uri) {
                            activityEntry.uri = toolInput.uri;
                        }
                        activity.push(activityEntry);

                        // Update thinking message with new activity
                        setMessages(prev => {
                            const newMessages = [...prev];
                            if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                                newMessages[newMessages.length - 1] = {
                                    ...newMessages[newMessages.length - 1],
                                    activity: [...activity]
                                };
                            }
                            return newMessages;
                        });

                        // Check whether this call needs confirmation. This
                        // covers custom tools that can write, not only
                        // query_database.
                        if (isWriteAccessEnabled()) {
                            const { needsConfirmation, subject } =
                                writeConfirmationSubject(tools, toolName, toolInput);
                            if (needsConfirmation) {
                                const confirmed = await requestWriteConfirmation(subject);
                                if (!confirmed) {
                                    activity[activityIndex].isError = true;
                                    activity[activityIndex].tokens = 0;
                                    const declinedText = 'Execution was declined by the user. Do not retry this call. Ask the user how they would like to proceed.';
                                    toolRepeatGuard.record({
                                        name: toolName,
                                        input: toolInput,
                                        isError: true,
                                        resultText: declinedText,
                                    });
                                    toolResultMessages.push(buildToolResultMessage(
                                        toolUseId,
                                        declinedText,
                                        true,
                                    ));
                                    continue;
                                }
                            }
                        }

                        try {
                            // Execute tool via MCP
                            const result = await mcpClient.callTool(toolName, toolInput);

                            // Estimate tokens in the result
                            const resultTokens = estimateToolResultTokens(result.content);
                            activity[activityIndex].tokens = resultTokens;
                            activity[activityIndex].isError = result.isError || false;

                            // Track repeated identical failures so the loop
                            // can stop rather than retry indefinitely.
                            toolRepeatGuard.record({
                                name: toolName,
                                input: toolInput,
                                isError: Boolean(result.isError),
                                resultText: flattenToolResultText(result.content),
                            });

                            // Update thinking message with token count
                            setMessages(prev => {
                                const newMessages = [...prev];
                                if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                                    newMessages[newMessages.length - 1] = {
                                        ...newMessages[newMessages.length - 1],
                                        activity: [...activity]
                                    };
                                }
                                return newMessages;
                            });

                            toolResultMessages.push(buildToolResultMessage(
                                toolUseId,
                                result.content,
                                Boolean(result.isError),
                            ));

                            // Refresh tools if manage_connections was called
                            if (toolName === 'manage_connections' && !result.isError) {
                                await refreshTools();
                            }

                            // Refresh database state if LLM switched databases
                            if (toolName === 'select_database_connection' && !result.isError) {
                                await fetchDatabases();
                            }
                        } catch (toolError) {
                            console.error('Tool execution error:', toolError);
                            const errorContent = `Error: ${toolError.message}`;
                            activity[activityIndex].tokens = estimateToolResultTokens(errorContent);
                            activity[activityIndex].isError = true;

                            toolRepeatGuard.record({
                                name: toolName,
                                input: toolInput,
                                isError: true,
                                resultText: errorContent,
                            });

                            toolResultMessages.push(buildToolResultMessage(
                                toolUseId,
                                errorContent,
                                true,
                            ));
                        }
                    }

                    // If the same call has now failed repeatedly, stop here
                    // and explain why, rather than sending the model back
                    // round to repeat it.
                    const repeatedFailure = toolRepeatGuard.getTripped();
                    if (repeatedFailure) {
                        pendingSaveRef.current = true;
                        setMessages(prev => {
                            const newMessages = prev.slice(0, -1);
                            return [...newMessages, {
                                role: 'assistant',
                                content: buildRepeatedToolFailureMessage(repeatedFailure),
                                timestamp: new Date().toISOString(),
                                provider: llmProviders.selectedProvider,
                                model: llmProviders.selectedModel,
                                activity: activity,
                                tokenUsage: usage,
                            }];
                        });
                        break;
                    }

                    // Echo the assistant turn into the conversation
                    // history for the next iteration.
                    conversationMessages.push({
                        role: 'assistant',
                        content: llmData.content,
                    });

                    // Each tool result is its own role-"tool" message.
                    for (const toolResultMsg of toolResultMessages) {
                        conversationMessages.push(toolResultMsg);
                    }

                    // Continue loop
                    continue;
                }

                // Unknown stop reason
                throw new Error(`Unexpected stop reason: ${llmData.stop_reason}`);
            }

            if (loopCount >= MAX_AGENTIC_LOOPS) {
                throw new Error('Maximum tool execution loops reached');
            }
        } catch (err) {
            // Check if this was a user cancellation
            if (err.name === 'AbortError') {
                console.log('Prompt execution cancelled by user');
                // Convert thinking message to cancelled message
                setMessages(prev => {
                    const newMessages = [...prev];
                    if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                        const thinkingMsg = newMessages[newMessages.length - 1];
                        newMessages[newMessages.length - 1] = {
                            role: 'assistant',
                            content: 'Request cancelled',
                            timestamp: new Date().toISOString(),
                            provider: thinkingMsg.provider,
                            model: thinkingMsg.model,
                            activity: thinkingMsg.activity || [],
                            isCancelled: true
                        };
                    }
                    return newMessages;
                });
                return;
            }

            console.error('Prompt execution error:', err);

            // Convert thinking message to error message (preserve activity for debugging)
            setMessages(prev => {
                const newMessages = [...prev];
                if (newMessages.length > 0 && newMessages[newMessages.length - 1].isThinking) {
                    const thinkingMsg = newMessages[newMessages.length - 1];
                    newMessages[newMessages.length - 1] = {
                        role: 'assistant',
                        content: `Error: ${err.message || 'Failed to execute prompt'}`,
                        timestamp: new Date().toISOString(),
                        provider: thinkingMsg.provider,
                        model: thinkingMsg.model,
                        activity: thinkingMsg.activity || [],
                        isError: true
                    };
                }
                return newMessages;
            });

        } finally {
            setExecutingPrompt(false);
            setLoading(false);
            abortControllerRef.current = null;
        }
    }, [mcpClient, loading, messages, sessionToken, tools, llmProviders.selectedProvider, llmProviders.selectedModel, forceLogout, refreshTools, fetchDatabases, isWriteAccessEnabled, requestWriteConfirmation]);

    return (
        <Box
            sx={{
                display: 'flex',
                flexDirection: 'column',
                flex: 1,
                minHeight: 0,
            }}
        >
            {/* Messages */}
            <MessageList
                messages={messages}
                showActivity={showActivity}
                renderMarkdown={renderMarkdown}
                debug={debug}
            />

            {/* Input Area */}
            <Paper
                elevation={0}
                sx={{
                    p: 2,
                    bgcolor: isDark ? '#1E293B' : '#FFFFFF',
                    border: '1px solid',
                    borderColor: isDark ? '#334155' : '#E5E7EB',
                    borderRadius: 3,
                }}
            >
                <MessageInput
                    value={input}
                    onChange={handleInputChange}
                    onSend={handleSend}
                    onCancel={handleCancel}
                    onKeyDown={handleKeyDown}
                    disabled={loading}
                    isLoading={loading}
                    onPromptClick={handlePromptClick}
                    hasPrompts={prompts && prompts.length > 0}
                />

                <ProviderSelector
                    providers={llmProviders.providers}
                    selectedProvider={llmProviders.selectedProvider}
                    onProviderChange={llmProviders.setSelectedProvider}
                    models={llmProviders.models}
                    selectedModel={llmProviders.selectedModel}
                    onModelChange={llmProviders.setSelectedModel}
                    showActivity={showActivity}
                    onActivityChange={setShowActivity}
                    renderMarkdown={renderMarkdown}
                    onMarkdownChange={setRenderMarkdown}
                    debug={debug}
                    onDebugChange={setDebug}
                    disabled={loading}
                    loadingModels={llmProviders.loadingModels}
                />
            </Paper>

            {/* Prompt Popover */}
            <PromptPopover
                anchorEl={promptPopoverAnchor}
                open={Boolean(promptPopoverAnchor)}
                onClose={() => setPromptPopoverAnchor(null)}
                prompts={prompts}
                onExecute={handlePromptExecute}
                executing={executingPrompt}
            />

            {/* Write Query Confirmation Dialog */}
            <WriteQueryConfirmDialog
                open={writeConfirmState !== null}
                query={writeConfirmState?.query || ''}
                onConfirm={() => {
                    writeConfirmState?.resolve(true);
                    setWriteConfirmState(null);
                }}
                onClose={() => {
                    writeConfirmState?.resolve(false);
                    setWriteConfirmState(null);
                }}
            />
        </Box>
    );
};

export default ChatInterface;
