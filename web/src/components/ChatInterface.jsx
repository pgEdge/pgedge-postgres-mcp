/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Chat Interface (Refactored)
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useState, useEffect, useCallback } from 'react';
import { Box, Paper, Alert } from '@mui/material';
import { useAuth } from '../contexts/AuthContext';
import { useLocalStorageBoolean } from '../hooks/useLocalStorage';
import { useQueryHistory } from '../hooks/useQueryHistory';
import { useMCPClient } from '../hooks/useMCPClient';
import { useLLMProviders } from '../hooks/useLLMProviders';
import MessageList from './MessageList';
import MessageInput from './MessageInput';
import ProviderSelector from './ProviderSelector';

const MAX_AGENTIC_LOOPS = 10;

const ChatInterface = () => {
    const { sessionToken, forceLogout } = useAuth();

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

    const [input, setInput] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');

    // Custom hooks for functionality
    const queryHistory = useQueryHistory();
    const { mcpClient, tools, refreshTools } = useMCPClient(sessionToken);
    const llmProviders = useLLMProviders(sessionToken);

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
        setError('');

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

            // Agentic loop
            while (loopCount < MAX_AGENTIC_LOOPS) {
                loopCount++;

                // Call LLM
                const llmResponse = await fetch('/api/llm/chat', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${sessionToken}`,
                    },
                    credentials: 'include',
                    body: JSON.stringify({
                        messages: conversationMessages,
                        tools: tools,
                        provider: llmProviders.selectedProvider,
                        model: llmProviders.selectedModel,
                    }),
                });

                // Handle session invalidation
                if (llmResponse.status === 401) {
                    console.log('Session invalidated, logging out...');
                    forceLogout();
                    setError('Your session has expired. Please log in again.');
                    // Remove thinking message (keep user message for context)
                    setMessages(prev => prev.slice(0, -1));
                    return;
                }

                if (!llmResponse.ok) {
                    const errorText = await llmResponse.text();
                    throw new Error(`LLM request failed: ${llmResponse.status} ${errorText}`);
                }

                const llmData = await llmResponse.json();
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

                        // Update activity
                        activity.push({
                            type: 'tool',
                            name: toolUse.name,
                            timestamp: new Date().toISOString(),
                        });

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
                            toolResults.push({
                                type: 'tool_result',
                                tool_use_id: toolUse.id,
                                content: `Error: ${toolError.message}`,
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

            // Remove thinking message (keep user message for context)
            setMessages(prev => prev.slice(0, -1));

            // Network errors
            if (err.name === 'TypeError' && err.message.includes('fetch')) {
                setError('Cannot connect to server. Please check that the server is running.');
            } else {
                setError(err.message || 'Failed to send message');
            }
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
        setError('');
    }, [queryHistory]);

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
                onClear={handleClear}
            />

            {/* Error Display */}
            {(error || llmProviders.error) && (
                <Alert
                    severity="error"
                    sx={{ mb: 1 }}
                    onClose={() => {
                        setError('');
                        // Note: Can't clear llmProviders.error as it's from the hook
                    }}
                >
                    {error || llmProviders.error}
                </Alert>
            )}

            {/* Input Area */}
            <Paper elevation={2} sx={{ p: 2 }}>
                <MessageInput
                    value={input}
                    onChange={handleInputChange}
                    onSend={handleSend}
                    onKeyDown={handleKeyDown}
                    disabled={loading}
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
                    disabled={loading}
                    loadingModels={llmProviders.loadingModels}
                />
            </Paper>
        </Box>
    );
};

export default ChatInterface;
