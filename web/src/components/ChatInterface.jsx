/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Chat Interface
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useState, useRef, useEffect } from 'react';
import {
    Box,
    Paper,
    TextField,
    IconButton,
    Typography,
    CircularProgress,
    Alert,
    Button,
    useTheme,
    FormControl,
    InputLabel,
    Select,
    MenuItem,
    FormControlLabel,
    Switch,
} from '@mui/material';
import {
    Send as SendIcon,
    Person as PersonIcon,
    SmartToy as BotIcon,
    Delete as DeleteIcon,
} from '@mui/icons-material';
import { useAuth } from '../contexts/AuthContext';
import { MCPClient } from '../lib/mcp-client';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';

// PostgreSQL/Elephant themed action words for thinking animation
const elephantActions = [
    "Thinking with trunks",
    "Consulting the herd",
    "Stampeding through data",
    "Trumpeting queries",
    "Migrating thoughts",
    "Packing memories",
    "Charging through logic",
    "Bathing in wisdom",
    "Roaming the database",
    "Grazing on metadata",
    "Herding ideas",
    "Splashing in pools",
    "Foraging for answers",
    "Wandering savannah",
    "Dusting off schemas",
    "Pondering profoundly",
    "Remembering everything",
    "Trumpeting brilliance",
    "Stomping bugs",
    "Tusking through code",
];

const ChatInterface = () => {
    const { sessionToken, forceLogout } = useAuth();
    const theme = useTheme();
    const [messages, setMessages] = useState(() => {
        // Load saved messages from localStorage
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
    const [input, setInput] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const [thinkingMessage, setThinkingMessage] = useState('');
    const messagesEndRef = useRef(null);
    const thinkingIntervalRef = useRef(null);

    // MCP client and tools
    const [mcpClient, setMcpClient] = useState(null);
    const [tools, setTools] = useState([]);

    // History navigation state
    const [queryHistory, setQueryHistory] = useState(() => {
        // Load saved query history from localStorage
        try {
            const savedHistory = localStorage.getItem('query-history');
            if (savedHistory) {
                return JSON.parse(savedHistory);
            }
        } catch (error) {
            console.error('Error loading query history:', error);
        }
        return [];
    });
    const [historyIndex, setHistoryIndex] = useState(-1);
    const [tempInput, setTempInput] = useState('');

    // Provider and model selection state
    const [providers, setProviders] = useState([]);
    const [selectedProvider, setSelectedProvider] = useState(() => {
        // Load saved provider from localStorage
        return localStorage.getItem('llm-provider') || '';
    });
    const [models, setModels] = useState([]);
    const [selectedModel, setSelectedModel] = useState(() => {
        // Load saved model from localStorage
        return localStorage.getItem('llm-model') || '';
    });
    const [loadingModels, setLoadingModels] = useState(false);
    const [showActivity, setShowActivity] = useState(() => {
        // Load saved activity display preference from localStorage (default to true)
        const saved = localStorage.getItem('show-activity');
        return saved === null ? true : saved === 'true';
    });
    const [renderMarkdown, setRenderMarkdown] = useState(() => {
        // Load saved markdown rendering preference from localStorage (default to true)
        const saved = localStorage.getItem('render-markdown');
        return saved === null ? true : saved === 'true';
    });

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    };

    // Helper function to get short model name
    const getShortModelName = (modelName) => {
        if (!modelName) return '';

        // Extract short name from various model formats
        if (modelName.startsWith('claude-')) {
            // claude-sonnet-4-5 -> Sonnet 4.5
            // claude-3-5-sonnet-20241022 -> Sonnet 3.5
            const parts = modelName.split('-');
            if (parts.includes('sonnet')) {
                const versionIndex = parts.findIndex(p => p === 'sonnet');
                if (versionIndex > 1 && parts[versionIndex - 1].match(/^\d/)) {
                    return `Sonnet ${parts.slice(1, versionIndex).join('.')}`;
                }
                if (versionIndex + 1 < parts.length && parts[versionIndex + 1].match(/^\d/)) {
                    return `Sonnet ${parts[versionIndex + 1].replace(/(\d)(\d)/, '$1.$2')}`;
                }
                return 'Sonnet';
            }
            if (parts.includes('opus')) return 'Opus';
            if (parts.includes('haiku')) return 'Haiku';
        } else if (modelName.startsWith('gpt-')) {
            // gpt-4o -> GPT-4o
            // gpt-3.5-turbo -> GPT-3.5
            return modelName.replace('gpt-', 'GPT-').replace('-turbo', '').toUpperCase();
        } else if (modelName.startsWith('o1-') || modelName.startsWith('o3-')) {
            // o1-preview -> O1
            return modelName.split('-')[0].toUpperCase();
        }

        // For Ollama or other models, return first part or full name if short
        const firstPart = modelName.split(':')[0];
        return firstPart.length <= 15 ? firstPart : modelName.substring(0, 15) + '...';
    };

    // Start thinking animation
    const startThinking = () => {
        // Set initial random message
        setThinkingMessage(elephantActions[Math.floor(Math.random() * elephantActions.length)]);

        // Change message every 2 seconds
        thinkingIntervalRef.current = setInterval(() => {
            setThinkingMessage(elephantActions[Math.floor(Math.random() * elephantActions.length)]);
        }, 2000);
    };

    // Stop thinking animation
    const stopThinking = () => {
        if (thinkingIntervalRef.current) {
            clearInterval(thinkingIntervalRef.current);
            thinkingIntervalRef.current = null;
        }
        setThinkingMessage('');
    };

    // Cleanup on unmount
    useEffect(() => {
        return () => stopThinking();
    }, []);

    // Save provider preference to localStorage when it changes
    useEffect(() => {
        if (selectedProvider) {
            localStorage.setItem('llm-provider', selectedProvider);
        }
    }, [selectedProvider]);

    // Save model preference to localStorage when it changes
    useEffect(() => {
        if (selectedModel) {
            localStorage.setItem('llm-model', selectedModel);
        }
    }, [selectedModel]);

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

    // Save query history to localStorage when it changes
    useEffect(() => {
        try {
            if (queryHistory.length > 0) {
                localStorage.setItem('query-history', JSON.stringify(queryHistory));
            }
        } catch (error) {
            console.error('Error saving query history:', error);
        }
    }, [queryHistory]);

    // Save activity display preference to localStorage when it changes
    useEffect(() => {
        localStorage.setItem('show-activity', showActivity.toString());
    }, [showActivity]);

    // Save markdown rendering preference to localStorage when it changes
    useEffect(() => {
        localStorage.setItem('render-markdown', renderMarkdown.toString());
    }, [renderMarkdown]);

    // Fetch available providers on mount
    useEffect(() => {
        if (!sessionToken) {
            console.log('No session token available, skipping providers fetch');
            return;
        }

        const fetchProviders = async () => {
            try {
                console.log('Fetching providers from /api/llm/providers...');
                const response = await fetch('/api/llm/providers', {
                    credentials: 'include',
                    headers: {
                        'Authorization': `Bearer ${sessionToken}`,
                    },
                });

                console.log('Providers response status:', response.status);
                if (!response.ok) {
                    const errorText = await response.text();
                    console.error('Providers response error:', errorText);
                    throw new Error(`Failed to fetch providers: ${response.status} ${errorText}`);
                }

                const data = await response.json();
                console.log('Providers data:', data);
                setProviders(data.providers || []);

                // Only set default if no saved provider or saved provider is not available
                const savedProvider = localStorage.getItem('llm-provider');
                const savedModel = localStorage.getItem('llm-model');
                const savedProviderExists = data.providers?.some(p => p.name === savedProvider);

                if (!savedProvider || !savedProviderExists) {
                    // No saved preference or saved provider no longer available - use default
                    const defaultProvider = data.providers?.find(p => p.isDefault);
                    if (defaultProvider) {
                        console.log('Setting default provider:', defaultProvider.name, 'model:', data.defaultModel);
                        setSelectedProvider(defaultProvider.name);
                        setSelectedModel(data.defaultModel || '');
                    } else {
                        console.warn('No default provider found in response');
                    }
                } else {
                    console.log('Using saved provider:', savedProvider, 'model:', savedModel);
                    // savedProvider and savedModel are already loaded from localStorage in state initialization
                }
            } catch (error) {
                console.error('Error fetching providers:', error);
                setError('Failed to load LLM providers. Please check browser console.');
            }
        };

        fetchProviders();
    }, [sessionToken]);

    // Fetch available models when provider changes
    useEffect(() => {
        if (!selectedProvider || !sessionToken) {
            console.log('No provider selected or no session token, skipping model fetch');
            return;
        }

        const fetchModels = async () => {
            setLoadingModels(true);
            try {
                console.log('Fetching models for provider:', selectedProvider);
                const response = await fetch(`/api/llm/models?provider=${selectedProvider}`, {
                    credentials: 'include',
                    headers: {
                        'Authorization': `Bearer ${sessionToken}`,
                    },
                });

                console.log('Models response status:', response.status);
                if (!response.ok) {
                    const errorText = await response.text();
                    console.error('Models response error:', errorText);
                    throw new Error(`Failed to fetch models: ${response.status} ${errorText}`);
                }

                const data = await response.json();
                console.log('Models data:', data);
                setModels(data.models || []);

                // Set the first model as selected if current model is not in the list
                if (data.models && data.models.length > 0) {
                    const currentModelExists = data.models.some(m => m.name === selectedModel);
                    if (!currentModelExists) {
                        console.log('Current model not in list, selecting first model:', data.models[0].name);
                        setSelectedModel(data.models[0].name);
                    }
                } else {
                    console.warn('No models returned from API');
                }
            } catch (error) {
                console.error('Error fetching models:', error);
                setModels([]);
                setError('Failed to load models. Please check browser console.');
            } finally {
                setLoadingModels(false);
            }
        };

        fetchModels();
    }, [selectedProvider, sessionToken]);

    // Initialize MCP client and fetch tools when session token is available
    useEffect(() => {
        if (!sessionToken) {
            console.log('No session token available, skipping MCP client initialization');
            return;
        }

        const initializeMCP = async () => {
            try {
                console.log('Initializing MCP client...');
                const client = new MCPClient('/mcp/v1', sessionToken);

                // Initialize the client
                await client.initialize();

                // Fetch available tools
                console.log('Fetching MCP tools...');
                const toolsList = await client.listTools();
                console.log('MCP tools loaded:', toolsList);

                setMcpClient(client);
                setTools(toolsList);
            } catch (error) {
                console.error('Error initializing MCP client:', error);
                setError('Failed to initialize MCP tools. Please check browser console.');
            }
        };

        initializeMCP();
    }, [sessionToken]);

    // Custom components for rendering markdown
    const markdownComponents = {
        code({ node, inline, className, children, ...props }) {
            const match = /language-(\w+)/.exec(className || '');
            const language = match ? match[1] : '';

            // Check if this is truly inline code by checking for newlines
            // Sometimes react-markdown misidentifies inline code as block code
            const childText = String(children);
            const isInline = inline || !childText.includes('\n');

            return !isInline ? (
                <SyntaxHighlighter
                    style={vscDarkPlus}
                    language={language || 'text'}
                    PreTag="div"
                    customStyle={{
                        margin: '1em 0',
                        borderRadius: '4px',
                        fontSize: '0.875rem',
                    }}
                    {...props}
                >
                    {String(children).replace(/\n$/, '')}
                </SyntaxHighlighter>
            ) : (
                <code
                    {...props}
                    style={{
                        display: 'inline',
                        verticalAlign: 'baseline',
                        backgroundColor: theme.palette.mode === 'dark' ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.1)',
                        padding: '2px 6px',
                        borderRadius: '3px',
                        fontFamily: 'monospace',
                        fontSize: '0.875em',
                    }}
                >
                    {children}
                </code>
            );
        },
        pre({ children }) {
            return <>{children}</>;
        },
        p({ children }) {
            return (
                <p
                    style={{
                        marginBottom: theme.spacing(1),
                        fontSize: '1rem',
                        lineHeight: 1.5,
                        color: theme.palette.text.primary,
                    }}
                >
                    {children}
                </p>
            );
        },
        h1({ children }) {
            return <Typography variant="h5" sx={{ mt: 2, mb: 1, fontWeight: 'bold' }}>{children}</Typography>;
        },
        h2({ children }) {
            return <Typography variant="h6" sx={{ mt: 2, mb: 1, fontWeight: 'bold' }}>{children}</Typography>;
        },
        h3({ children }) {
            return <Typography variant="subtitle1" sx={{ mt: 1.5, mb: 1, fontWeight: 'bold' }}>{children}</Typography>;
        },
        ul({ children }) {
            return (
                <ul style={{ paddingLeft: theme.spacing(2), marginTop: theme.spacing(1), marginBottom: theme.spacing(1) }}>
                    {children}
                </ul>
            );
        },
        ol({ children }) {
            return (
                <ol style={{ paddingLeft: theme.spacing(2), marginTop: theme.spacing(1), marginBottom: theme.spacing(1) }}>
                    {children}
                </ol>
            );
        },
        li({ children }) {
            return (
                <li
                    style={{
                        marginBottom: theme.spacing(0.5),
                        fontSize: '1rem',
                        lineHeight: 1.5,
                        color: theme.palette.text.primary,
                    }}
                >
                    {children}
                </li>
            );
        },
        a({ href, children }) {
            return (
                <a href={href} target="_blank" rel="noopener noreferrer" style={{ color: '#1976d2' }}>
                    {children}
                </a>
            );
        },
        table({ children }) {
            return (
                <Box sx={{ overflowX: 'auto', my: 2 }}>
                    <table style={{ borderCollapse: 'collapse', width: '100%' }}>{children}</table>
                </Box>
            );
        },
        th({ children }) {
            return (
                <th style={{
                    border: `1px solid ${theme.palette.mode === 'dark' ? '#555' : '#ddd'}`,
                    padding: '8px',
                    backgroundColor: theme.palette.mode === 'dark' ? '#2a2a2a' : '#f5f5f5',
                    color: theme.palette.mode === 'dark' ? '#fff' : '#000',
                    fontWeight: 'bold',
                    textAlign: 'left',
                }}>
                    {children}
                </th>
            );
        },
        td({ children }) {
            return (
                <td style={{
                    border: `1px solid ${theme.palette.mode === 'dark' ? '#555' : '#ddd'}`,
                    padding: '8px',
                }}>
                    {children}
                </td>
            );
        },
    };

    useEffect(() => {
        scrollToBottom();
    }, [messages]);

    const handleSend = async () => {
        if (!input.trim() || loading || !mcpClient) return;

        const userMessage = {
            role: 'user',
            content: input.trim(),
            timestamp: new Date().toISOString(),
        };

        // Add to query history
        setQueryHistory(prev => [...prev, userMessage.content]);
        setHistoryIndex(-1);
        setTempInput('');

        // Create thinking message placeholder
        const thinkingMessage = {
            role: 'assistant',
            content: '',  // Empty initially
            timestamp: new Date().toISOString(),
            provider: selectedProvider,
            model: selectedModel,
            activity: [], // Will be populated as tool calls happen
            isThinking: true,
        };

        setMessages(prev => [...prev, userMessage, thinkingMessage]);
        setInput('');
        setLoading(true);
        setError('');
        startThinking();

        try {
            // Build conversation history for LLM
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
            const maxLoops = 10; // Prevent infinite loops

            // Agentic loop: LLM -> tool calls -> LLM -> ...
            while (loopCount < maxLoops) {
                loopCount++;

                // Call LLM with conversation history and available tools
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
                        provider: selectedProvider,
                        model: selectedModel,
                    }),
                });

                // Handle session invalidation
                if (llmResponse.status === 401) {
                    console.log('Session invalidated, logging out...');
                    stopThinking();
                    forceLogout();
                    setError('Your session has expired. Please log in again.');
                    setMessages(prev => prev.slice(0, -1));
                    return;
                }

                if (!llmResponse.ok) {
                    const errorText = await llmResponse.text();
                    throw new Error(`LLM request failed: ${llmResponse.status} ${errorText}`);
                }

                const llmData = await llmResponse.json();
                console.log('LLM response:', llmData);

                // Check stop reason
                if (llmData.stop_reason === 'end_turn' || loopCount >= maxLoops) {
                    // Final response - extract text content
                    let textContent = '';

                    // Ensure content is an array
                    const contentArray = Array.isArray(llmData.content) ? llmData.content : [llmData.content];

                    for (const content of contentArray) {
                        if (content && content.type === 'text') {
                            // Ensure text is a string
                            const text = typeof content.text === 'string' ? content.text : String(content.text || '');
                            textContent += text;
                        }
                    }

                    // Ensure we always have a string
                    const finalContent = textContent || 'No response received';

                    // Replace thinking message with final response
                    setMessages(prev => {
                        const newMessages = prev.slice(0, -1); // Remove thinking message
                        return [...newMessages, {
                            role: 'assistant',
                            content: finalContent,
                            timestamp: new Date().toISOString(),
                            provider: selectedProvider,
                            model: selectedModel,
                            activity: activity,
                        }];
                    });
                    break;
                }

                // Handle tool use
                if (llmData.stop_reason === 'tool_use') {
                    // Extract tool use blocks
                    const toolUses = llmData.content.filter(c => c.type === 'tool_use');

                    if (toolUses.length === 0) {
                        throw new Error('LLM indicated tool_use but no tool_use blocks found');
                    }

                    // Execute tools and collect results
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
                            const lastMsg = newMessages[newMessages.length - 1];
                            if (lastMsg && lastMsg.isThinking) {
                                lastMsg.activity = [...activity];
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

                    // Add assistant message with tool uses to conversation
                    conversationMessages.push({
                        role: 'assistant',
                        content: llmData.content,
                    });

                    // Add user message with tool results to conversation
                    conversationMessages.push({
                        role: 'user',
                        content: toolResults,
                    });

                    // Continue loop to get LLM's response to tool results
                    continue;
                }

                // Unknown stop reason
                throw new Error(`Unexpected stop reason: ${llmData.stop_reason}`);
            }

            if (loopCount >= maxLoops) {
                throw new Error('Maximum tool execution loops reached');
            }
        } catch (err) {
            console.error('Chat error:', err);

            // Remove thinking message
            setMessages(prev => prev.slice(0, -1));

            // Network errors
            if (err.name === 'TypeError' && err.message.includes('fetch')) {
                setError('Cannot connect to server. Please check that the server is running.');
            } else {
                setError(err.message || 'Failed to send message');
            }
        } finally {
            stopThinking();
            setLoading(false);
        }
    };

    const handleKeyDown = (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            handleSend();
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            if (queryHistory.length === 0) return;

            // Save current input if we're starting to navigate history
            if (historyIndex === -1) {
                setTempInput(input);
            }

            // Calculate new index (going backwards in history)
            const newIndex = historyIndex === -1
                ? queryHistory.length - 1
                : Math.max(0, historyIndex - 1);

            setHistoryIndex(newIndex);
            setInput(queryHistory[newIndex]);
        } else if (e.key === 'ArrowDown') {
            e.preventDefault();
            if (historyIndex === -1) return; // Not navigating history

            // Calculate new index (going forwards in history)
            const newIndex = historyIndex + 1;

            if (newIndex >= queryHistory.length) {
                // Reached the end, restore temporary input
                setHistoryIndex(-1);
                setInput(tempInput);
                setTempInput('');
            } else {
                setHistoryIndex(newIndex);
                setInput(queryHistory[newIndex]);
            }
        }
    };

    const handleClear = () => {
        if (!window.confirm('Clear conversation history?')) return;

        setMessages([]);
        setQueryHistory([]);
        localStorage.removeItem('chat-messages');
        localStorage.removeItem('query-history');
        setError('');
    };

    return (
        <Box
            sx={{
                display: 'flex',
                flexDirection: 'column',
                flex: 1,
                minHeight: 0,
            }}
        >
            {/* Chat Messages Area */}
            <Paper
                elevation={1}
                sx={{
                    flex: 1,
                    overflow: 'auto',
                    p: 2,
                    mb: 1,
                    bgcolor: 'background.paper',
                    position: 'relative',
                }}
            >
                {messages.length > 0 && (
                    <Box
                        sx={{
                            position: 'sticky',
                            top: 0,
                            display: 'flex',
                            justifyContent: 'flex-end',
                            mb: 2,
                            pt: 1,
                            pb: 1,
                            zIndex: 10,
                        }}
                    >
                        <Button
                            size="small"
                            startIcon={<DeleteIcon />}
                            onClick={handleClear}
                            variant="outlined"
                            color="secondary"
                            sx={{
                                opacity: 0.85,
                                transition: 'opacity 0.2s ease-in-out',
                                '&:hover': {
                                    opacity: 1,
                                },
                            }}
                        >
                            Clear
                        </Button>
                    </Box>
                )}
                {messages.length === 0 ? (
                    <Box
                        sx={{
                            display: 'flex',
                            flexDirection: 'column',
                            alignItems: 'center',
                            justifyContent: 'center',
                            height: '100%',
                            color: 'text.secondary',
                        }}
                    >
                        <BotIcon sx={{ fontSize: 64, mb: 2, opacity: 0.3 }} />
                        <Typography variant="h6" gutterBottom>
                            Start a conversation
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                            Ask questions about your PostgreSQL database
                        </Typography>
                    </Box>
                ) : (
                    <Box>
                        {messages.map((message, index) => (
                            <Box
                                key={index}
                                sx={{
                                    display: 'flex',
                                    mb: 2,
                                    alignItems: 'flex-start',
                                    opacity: message.fromPreviousSession ? 0.6 : 1,
                                    transition: 'opacity 0.3s ease-in-out',
                                }}
                            >
                                <Box
                                    sx={{
                                        width: 32,
                                        height: 32,
                                        borderRadius: '50%',
                                        display: 'flex',
                                        alignItems: 'center',
                                        justifyContent: 'center',
                                        bgcolor: message.role === 'user' ? 'primary.main' : 'secondary.main',
                                        color: 'white',
                                        mr: 2,
                                        flexShrink: 0,
                                    }}
                                >
                                    {message.role === 'user' ? (
                                        <PersonIcon sx={{ fontSize: 20 }} />
                                    ) : (
                                        <BotIcon sx={{ fontSize: 20 }} />
                                    )}
                                </Box>
                                <Box sx={{ flex: 1 }}>
                                    <Typography
                                        variant="caption"
                                        color="text.secondary"
                                        sx={{ display: 'block', mb: 0.5 }}
                                    >
                                        {message.role === 'user'
                                            ? 'You'
                                            : message.provider && message.model
                                                ? `${message.provider.charAt(0).toUpperCase() + message.provider.slice(1)} (${getShortModelName(message.model)})`
                                                : 'Assistant'
                                        }
                                    </Typography>
                                    {showActivity && message.role === 'assistant' && message.activity && message.activity.length > 0 && (
                                        <Box sx={{ mb: 1 }}>
                                            {message.activity.map((activity, idx) => (
                                                <Typography
                                                    key={idx}
                                                    variant="caption"
                                                    sx={{
                                                        display: 'block',
                                                        color: 'text.secondary',
                                                        fontFamily: 'monospace',
                                                        fontSize: '0.7rem',
                                                        mb: 0.2,
                                                    }}
                                                >
                                                    {activity.type === 'tool' && (
                                                        <>🔧 {activity.name}</>
                                                    )}
                                                    {activity.type === 'resource' && (
                                                        <>📄 {activity.uri}</>
                                                    )}
                                                </Typography>
                                            ))}
                                        </Box>
                                    )}
                                    <Paper
                                        elevation={0}
                                        sx={{
                                            p: 2,
                                            bgcolor: message.role === 'user' ? 'primary.light' : 'background.default',
                                            color: message.role === 'user' ? 'primary.contrastText' : 'text.primary',
                                            borderRadius: 2,
                                        }}
                                    >
                                        {message.role === 'user' ? (
                                            <Typography
                                                variant="body1"
                                                sx={{
                                                    whiteSpace: 'pre-wrap',
                                                    wordBreak: 'break-word',
                                                }}
                                            >
                                                {message.content}
                                            </Typography>
                                        ) : message.isThinking ? (
                                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                                <CircularProgress size={20} />
                                                <Typography
                                                    variant="body2"
                                                    sx={{
                                                        color: 'text.secondary',
                                                        fontStyle: 'italic',
                                                    }}
                                                >
                                                    {thinkingMessage}...
                                                </Typography>
                                            </Box>
                                        ) : renderMarkdown ? (
                                            <ReactMarkdown
                                                remarkPlugins={[remarkGfm]}
                                                components={markdownComponents}
                                            >
                                                {message.content}
                                            </ReactMarkdown>
                                        ) : (
                                            <Typography
                                                variant="body1"
                                                sx={{
                                                    whiteSpace: 'pre-wrap',
                                                    wordBreak: 'break-word',
                                                }}
                                            >
                                                {message.content}
                                            </Typography>
                                        )}
                                    </Paper>
                                </Box>
                            </Box>
                        ))}
                        <div ref={messagesEndRef} />
                    </Box>
                )}
            </Paper>

            {/* Error Display */}
            {error && (
                <Alert severity="error" sx={{ mb: 1 }} onClose={() => setError('')}>
                    {error}
                </Alert>
            )}

            {/* Input Area */}
            <Paper
                elevation={2}
                sx={{
                    p: 2,
                }}
            >
                {/* Text Input Row */}
                <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', mb: 2 }}>
                    <TextField
                        fullWidth
                        multiline
                        maxRows={4}
                        variant="outlined"
                        placeholder="Type your message..."
                        value={input}
                        onChange={(e) => {
                            setInput(e.target.value);
                            // Reset history navigation when user types
                            if (historyIndex !== -1) {
                                setHistoryIndex(-1);
                                setTempInput('');
                            }
                        }}
                        onKeyDown={handleKeyDown}
                        disabled={loading}
                        sx={{
                            '& .MuiOutlinedInput-root': {
                                borderRadius: 2,
                            },
                        }}
                    />
                    <IconButton
                        color="primary"
                        onClick={handleSend}
                        disabled={!input.trim() || loading}
                        sx={{
                            bgcolor: 'primary.main',
                            color: 'white',
                            '&:hover': {
                                bgcolor: 'primary.dark',
                            },
                            '&.Mui-disabled': {
                                bgcolor: 'action.disabledBackground',
                                color: 'action.disabled',
                            },
                        }}
                    >
                        <SendIcon />
                    </IconButton>
                </Box>

                {/* Provider and Model Selection Row */}
                <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
                    <FormControl sx={{ minWidth: 200 }} size="small">
                        <InputLabel id="provider-select-label">Provider</InputLabel>
                        <Select
                            labelId="provider-select-label"
                            id="provider-select"
                            value={selectedProvider}
                            label="Provider"
                            onChange={(e) => setSelectedProvider(e.target.value)}
                            disabled={loading}
                        >
                            {providers.map((provider) => (
                                <MenuItem key={provider.name} value={provider.name}>
                                    {provider.display}
                                </MenuItem>
                            ))}
                        </Select>
                    </FormControl>

                    <FormControl sx={{ minWidth: 300, flex: 1 }} size="small">
                        <InputLabel id="model-select-label">Model</InputLabel>
                        <Select
                            labelId="model-select-label"
                            id="model-select"
                            value={selectedModel}
                            label="Model"
                            onChange={(e) => setSelectedModel(e.target.value)}
                            disabled={loading || loadingModels}
                        >
                            {models.map((model) => (
                                <MenuItem key={model.name} value={model.name}>
                                    {model.name}
                                    {model.description && ` - ${model.description}`}
                                </MenuItem>
                            ))}
                        </Select>
                    </FormControl>

                    <FormControlLabel
                        control={
                            <Switch
                                checked={showActivity}
                                onChange={(e) => setShowActivity(e.target.checked)}
                                size="small"
                            />
                        }
                        label="Show Activity"
                        sx={{ ml: 1, whiteSpace: 'nowrap' }}
                    />

                    <FormControlLabel
                        control={
                            <Switch
                                checked={renderMarkdown}
                                onChange={(e) => setRenderMarkdown(e.target.checked)}
                                size="small"
                            />
                        }
                        label="Render Markdown"
                        sx={{ ml: 1, whiteSpace: 'nowrap' }}
                    />
                </Box>
            </Paper>
        </Box>
    );
};

export default ChatInterface;
