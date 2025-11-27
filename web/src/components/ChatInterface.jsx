/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Chat Interface (Refactored)
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useState, useEffect, useCallback } from 'react';
import { Box, Paper } from '@mui/material';
import { useAuth } from '../contexts/AuthContext';
import { useLLMProcessing } from '../contexts/LLMProcessingContext';
import { useLocalStorageBoolean } from '../hooks/useLocalStorage';
import { useQueryHistory } from '../hooks/useQueryHistory';
import { useMCPClient } from '../hooks/useMCPClient';
import { useLLMProviders } from '../hooks/useLLMProviders';
import MessageList from './MessageList';
import MessageInput from './MessageInput';
import ProviderSelector from './ProviderSelector';
import PromptPopover from './PromptPopover';

const MAX_AGENTIC_LOOPS = 50;
// Compact if estimated tokens exceed this threshold.
// Note: Anthropic rate limits are typically 30k-60k input tokens/minute cumulative.
// Setting lower allows multiple requests within the rate limit window.
const TOKEN_COMPACTION_THRESHOLD = 15000;
const RATE_LIMIT_RETRY_DELAY_MS = 60000; // 60 seconds

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
 * Estimates the number of tokens in a string.
 * Uses a rough heuristic of ~4 characters per token for English text.
 * @param {string} text - The text to estimate tokens for
 * @returns {number} - Estimated token count
 */
const estimateTokens = (text) => {
    if (!text) return 0;
    // Rough heuristic: ~4 characters per token for English, ~3 for code/JSON
    // Use 3.5 as a middle ground to be conservative
    return Math.ceil(text.length / 3.5);
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
            total += estimateTokens(msg.content);
        } else if (Array.isArray(msg.content)) {
            // Handle tool_use and tool_result arrays
            for (const item of msg.content) {
                if (item.text) {
                    total += estimateTokens(item.text);
                } else if (item.input) {
                    total += estimateTokens(JSON.stringify(item.input));
                } else if (item.content) {
                    // Tool results can have content as string or array
                    if (typeof item.content === 'string') {
                        total += estimateTokens(item.content);
                    } else if (Array.isArray(item.content)) {
                        for (const c of item.content) {
                            if (c.text) total += estimateTokens(c.text);
                        }
                    }
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

const ChatInterface = () => {
    const { sessionToken, forceLogout } = useAuth();
    const { setIsProcessing } = useLLMProcessing();

    // State management using custom hooks
    // Initialize messages with fromPreviousSession flag for loaded messages
    const [messages, setMessages] = useState(() => {
        try {
            const savedMessages = localStorage.getItem('chat-messages');
            if (savedMessages) {
                const parsed = JSON.parse(savedMessages);
                // Mark all loaded messages as from previous session and ensure content is a string
                return parsed.map(msg => ({
                    ...msg,
                    content: typeof msg.content === 'string' ? msg.content : JSON.stringify(msg.content),
                    fromPreviousSession: true
                }));
            }
        } catch (error) {
            console.error('Error loading chat messages:', error);
        }
        return [];
    });

    const [showActivity, setShowActivity] = useLocalStorageBoolean('show-activity', true);
    const [renderMarkdown, setRenderMarkdown] = useLocalStorageBoolean('render-markdown', true);
    const [debug, setDebug] = useLocalStorageBoolean('debug', false);

    const [input, setInput] = useState('');
    const [loading, setLoading] = useState(false);

    // Prompt popover state
    const [promptPopoverAnchor, setPromptPopoverAnchor] = useState(null);
    const [executingPrompt, setExecutingPrompt] = useState(false);

    // Custom hooks for functionality
    const queryHistory = useQueryHistory();
    const { mcpClient, tools, prompts, refreshTools, refreshPrompts } = useMCPClient(sessionToken);
    const llmProviders = useLLMProviders(sessionToken);

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

    // Save messages to localStorage when they change
    useEffect(() => {
        try {
            // Don't save if messages array is empty
            if (messages.length > 0) {
                // Remove the fromPreviousSession flag before saving
                const messagesToSave = messages.map(({ fromPreviousSession, ...msg }) => msg);
                localStorage.setItem('chat-messages', JSON.stringify(messagesToSave));
            }
        } catch (error) {
            console.error('Error saving chat messages:', error);
        }
    }, [messages]);

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

        try {
            // Build conversation history
            const conversationMessages = [];

            // Add all previous messages
            for (const msg of messages) {
                if (msg.role === 'user') {
                    conversationMessages.push({
                        role: 'user',
                        content: msg.content
                    });
                } else if (msg.role === 'assistant' && msg.content) {
                    conversationMessages.push({
                        role: 'assistant',
                        content: msg.content
                    });
                }
            }

            // Add current user message
            conversationMessages.push({
                role: 'user',
                content: userMessage.content
            });

            const activity = [];
            let loopCount = 0;
            let rateLimitRetryCount = 0;

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

                // Call LLM with compacted history
                // Note: Always send debug=true to get token usage for rate limit tracking
                const llmResponse = await fetch('/api/llm/chat', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${sessionToken}`,
                    },
                    credentials: 'include',
                    body: JSON.stringify({
                        messages: compactedMessages,
                        tools: tools,
                        provider: llmProviders.selectedProvider,
                        model: llmProviders.selectedModel,
                        debug: true,
                    }),
                });

                // Handle session invalidation
                if (llmResponse.status === 401) {
                    console.log('Session invalidated, logging out...');
                    // Convert thinking message to error message before logout
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

                if (!llmResponse.ok) {
                    const errorText = await llmResponse.text();

                    // Check for rate limit error
                    if (isRateLimitError(llmResponse.status, errorText)) {
                        rateLimitRetryCount++;
                        const rateLimitDetails = parseRateLimitError(errorText);
                        const estimatedTokens = estimateTotalTokens(compactedMessages);
                        const cumulativeTokens = tokenUsageTracker.getInputTokensLastMinute();
                        const requestCount = tokenUsageTracker.getRequestCountLastMinute();

                        if (rateLimitRetryCount === 1) {
                            // First rate limit hit - pause and retry
                            console.log('[Rate Limit] First hit, pausing for 60 seconds before retry...');
                            console.log(`[Rate Limit] Cumulative tokens in last minute: ${cumulativeTokens}, requests: ${requestCount}`);

                            // Add rate limit activity
                            activity.push({
                                type: 'rate_limit_pause',
                                timestamp: new Date().toISOString(),
                                message: rateLimitDetails.message,
                                estimatedTokens: estimatedTokens,
                                cumulativeTokens: cumulativeTokens,
                                requestCount: requestCount,
                            });

                            // Update thinking message to show we're waiting
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

                            // Wait 60 seconds
                            await delay(RATE_LIMIT_RETRY_DELAY_MS);

                            // Don't increment loopCount for rate limit retries
                            loopCount--;
                            continue;
                        } else {
                            // Second rate limit hit - give up with friendly message
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

                    throw new Error(`LLM request failed: ${llmResponse.status} ${errorText}`);
                }

                const llmData = await llmResponse.json();

                // Track token usage for rate limit awareness
                console.log('[Token Debug] llmData.token_usage:', llmData.token_usage);
                console.log('[Token Debug] llmData.usage:', llmData.usage);
                tokenUsageTracker.record(llmData.token_usage || llmData.usage);

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
                    setMessages(prev => {
                        const newMessages = prev.slice(0, -1);
                        return [...newMessages, {
                            role: 'assistant',
                            content: finalContent,
                            timestamp: new Date().toISOString(),
                            provider: llmProviders.selectedProvider,
                            model: llmProviders.selectedModel,
                            activity: activity,
                            tokenUsage: llmData.token_usage,
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

                    // Execute tools
                    const toolResults = [];
                    for (const toolUse of toolUses) {
                        console.log('Executing tool:', toolUse.name, 'with args:', toolUse.input);

                        // Add initial activity entry (token count will be updated after execution)
                        const activityIndex = activity.length;
                        const activityEntry = {
                            type: 'tool',
                            name: toolUse.name,
                            timestamp: new Date().toISOString(),
                            tokens: null, // Will be updated after execution
                        };
                        // For read_resource, capture the URI being accessed
                        if (toolUse.name === 'read_resource' && toolUse.input?.uri) {
                            activityEntry.uri = toolUse.input.uri;
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

                        try {
                            // Execute tool via MCP
                            const result = await mcpClient.callTool(toolUse.name, toolUse.input);
                            console.log('Tool result:', result);

                            // Estimate tokens in the result
                            const resultTokens = estimateToolResultTokens(result.content);
                            activity[activityIndex].tokens = resultTokens;
                            activity[activityIndex].isError = result.isError || false;

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

                            toolResults.push({
                                type: 'tool_result',
                                tool_use_id: toolUse.id,
                                content: result.content,
                            });

                            // Refresh tools if manage_connections was called
                            if (toolUse.name === 'manage_connections' && !result.isError) {
                                await refreshTools();
                            }
                        } catch (toolError) {
                            console.error('Tool execution error:', toolError);
                            const errorContent = `Error: ${toolError.message}`;
                            activity[activityIndex].tokens = estimateToolResultTokens(errorContent);
                            activity[activityIndex].isError = true;

                            toolResults.push({
                                type: 'tool_result',
                                tool_use_id: toolUse.id,
                                content: errorContent,
                                is_error: true,
                            });
                        }
                    }

                    // Add assistant message with tool uses
                    conversationMessages.push({
                        role: 'assistant',
                        content: llmData.content,
                    });

                    // Add user message with tool results
                    conversationMessages.push({
                        role: 'user',
                        content: toolResults,
                    });

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
        }
    }, [input, loading, mcpClient, messages, sessionToken, tools, llmProviders.selectedProvider, llmProviders.selectedModel, queryHistory, forceLogout, refreshTools]);

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

    // Handle clear conversation
    const handleClear = useCallback(() => {
        if (!window.confirm('Clear conversation history?')) return;

        setMessages([]);
        queryHistory.clearHistory();
    }, [queryHistory]);

    // Handle prompt selection
    const handlePromptClick = useCallback((event) => {
        setPromptPopoverAnchor(event.currentTarget);
    }, []);

    // Handle prompt execution
    const handlePromptExecute = useCallback(async (promptName, args) => {
        if (!mcpClient || loading) return;

        setExecutingPrompt(true);

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

            // Build conversation history (exclude system messages)
            const conversationMessages = [];
            for (const msg of messages) {
                if (msg.role === 'user') {
                    conversationMessages.push({
                        role: 'user',
                        content: msg.content
                    });
                } else if (msg.role === 'assistant' && msg.content) {
                    conversationMessages.push({
                        role: 'assistant',
                        content: msg.content
                    });
                }
            }
            // Add prompt messages to conversation history (but not to UI display)
            if (promptResult.messages) {
                for (const msg of promptResult.messages) {
                    if (msg.role === 'user') {
                        // Add to conversation history (only role and content)
                        conversationMessages.push({
                            role: 'user',
                            content: msg.content.text
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

                // Make LLM request with compacted history
                // Note: Always send debug=true to get token usage for rate limit tracking
                const response = await fetch('/api/llm/chat', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${sessionToken}`,
                    },
                    body: JSON.stringify({
                        messages: compactedMessages,
                        tools: tools,
                        provider: llmProviders.selectedProvider,
                        model: llmProviders.selectedModel,
                        debug: true,
                    }),
                });

                if (!response.ok) {
                    if (response.status === 401) {
                        forceLogout();
                        throw new Error('Session expired. Please login again.');
                    }
                    const errorText = await response.text();

                    // Check for rate limit error
                    if (isRateLimitError(response.status, errorText)) {
                        rateLimitRetryCount++;
                        const rateLimitDetails = parseRateLimitError(errorText);
                        const estimatedTokens = estimateTotalTokens(compactedMessages);
                        const cumulativeTokens = tokenUsageTracker.getInputTokensLastMinute();
                        const requestCount = tokenUsageTracker.getRequestCountLastMinute();

                        if (rateLimitRetryCount === 1) {
                            // First rate limit hit - pause and retry
                            console.log('[Rate Limit] First hit, pausing for 60 seconds before retry...');
                            console.log(`[Rate Limit] Cumulative tokens in last minute: ${cumulativeTokens}, requests: ${requestCount}`);

                            // Add rate limit activity
                            activity.push({
                                type: 'rate_limit_pause',
                                timestamp: new Date().toISOString(),
                                message: rateLimitDetails.message,
                                estimatedTokens: estimatedTokens,
                                cumulativeTokens: cumulativeTokens,
                                requestCount: requestCount,
                            });

                            // Update thinking message to show we're waiting
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

                            // Wait 60 seconds
                            await delay(RATE_LIMIT_RETRY_DELAY_MS);

                            // Don't increment loopCount for rate limit retries
                            loopCount--;
                            continue;
                        } else {
                            // Second rate limit hit - give up with friendly message
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

                    throw new Error(`Server error: ${errorText}`);
                }

                const llmData = await response.json();

                // Track token usage for rate limit awareness
                console.log('[Token Debug] llmData.token_usage:', llmData.token_usage);
                console.log('[Token Debug] llmData.usage:', llmData.usage);
                tokenUsageTracker.record(llmData.token_usage || llmData.usage);

                // Reset rate limit retry counter after successful response
                rateLimitRetryCount = 0;

                // Handle end_turn
                if (llmData.stop_reason === 'end_turn') {
                    const finalContent = llmData.content
                        .filter(c => c.type === 'text')
                        .map(c => c.text)
                        .join('\n');

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
                            tokenUsage: llmData.token_usage,
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

                    // Execute tools
                    const toolResults = [];
                    for (const toolUse of toolUses) {
                        // Add initial activity entry (token count will be updated after execution)
                        const activityIndex = activity.length;
                        const activityEntry = {
                            type: 'tool',
                            name: toolUse.name,
                            timestamp: new Date().toISOString(),
                            tokens: null, // Will be updated after execution
                        };
                        // For read_resource, capture the URI being accessed
                        if (toolUse.name === 'read_resource' && toolUse.input?.uri) {
                            activityEntry.uri = toolUse.input.uri;
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

                        try {
                            // Execute tool via MCP
                            const result = await mcpClient.callTool(toolUse.name, toolUse.input);

                            // Estimate tokens in the result
                            const resultTokens = estimateToolResultTokens(result.content);
                            activity[activityIndex].tokens = resultTokens;
                            activity[activityIndex].isError = result.isError || false;

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

                            toolResults.push({
                                type: 'tool_result',
                                tool_use_id: toolUse.id,
                                content: result.content,
                            });

                            // Refresh tools if manage_connections was called
                            if (toolUse.name === 'manage_connections' && !result.isError) {
                                await refreshTools();
                            }
                        } catch (toolError) {
                            console.error('Tool execution error:', toolError);
                            const errorContent = `Error: ${toolError.message}`;
                            activity[activityIndex].tokens = estimateToolResultTokens(errorContent);
                            activity[activityIndex].isError = true;

                            toolResults.push({
                                type: 'tool_result',
                                tool_use_id: toolUse.id,
                                content: errorContent,
                                is_error: true,
                            });
                        }
                    }

                    // Add assistant message with tool uses
                    conversationMessages.push({
                        role: 'assistant',
                        content: llmData.content,
                    });

                    // Add user message with tool results
                    conversationMessages.push({
                        role: 'user',
                        content: toolResults,
                    });

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
        }
    }, [mcpClient, loading, messages, sessionToken, tools, llmProviders.selectedProvider, llmProviders.selectedModel, forceLogout, refreshTools]);

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
            <Paper elevation={2} sx={{ p: 2 }}>
                <MessageInput
                    value={input}
                    onChange={handleInputChange}
                    onSend={handleSend}
                    onKeyDown={handleKeyDown}
                    disabled={loading}
                    onPromptClick={handlePromptClick}
                    hasPrompts={prompts && prompts.length > 0}
                    messages={messages}
                    showActivity={showActivity}
                    debug={debug}
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
                    onClear={handleClear}
                    hasMessages={messages.length > 0}
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
        </Box>
    );
};

export default ChatInterface;
