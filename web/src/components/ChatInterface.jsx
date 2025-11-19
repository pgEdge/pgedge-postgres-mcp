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
} from '@mui/material';
import {
    Send as SendIcon,
    Person as PersonIcon,
    SmartToy as BotIcon,
    Delete as DeleteIcon,
} from '@mui/icons-material';
import { useAuth } from '../contexts/AuthContext';
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
    const { forceLogout } = useAuth();
    const theme = useTheme();
    const [messages, setMessages] = useState([]);
    const [input, setInput] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const [thinkingMessage, setThinkingMessage] = useState('');
    const messagesEndRef = useRef(null);
    const thinkingIntervalRef = useRef(null);

    // History navigation state
    const [queryHistory, setQueryHistory] = useState([]);
    const [historyIndex, setHistoryIndex] = useState(-1);
    const [tempInput, setTempInput] = useState('');

    // Provider and model selection state
    const [providers, setProviders] = useState([]);
    const [selectedProvider, setSelectedProvider] = useState('');
    const [models, setModels] = useState([]);
    const [selectedModel, setSelectedModel] = useState('');
    const [loadingModels, setLoadingModels] = useState(false);

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
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

    // Fetch available providers on mount
    useEffect(() => {
        const fetchProviders = async () => {
            try {
                console.log('Fetching providers from /api/llm/providers...');
                const response = await fetch('/api/llm/providers', {
                    credentials: 'include',
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

                // Set default provider and model
                const defaultProvider = data.providers?.find(p => p.isDefault);
                if (defaultProvider) {
                    console.log('Setting default provider:', defaultProvider.name, 'model:', data.defaultModel);
                    setSelectedProvider(defaultProvider.name);
                    setSelectedModel(data.defaultModel || '');
                } else {
                    console.warn('No default provider found in response');
                }
            } catch (error) {
                console.error('Error fetching providers:', error);
                setError('Failed to load LLM providers. Please check browser console.');
            }
        };

        fetchProviders();
    }, []);

    // Fetch available models when provider changes
    useEffect(() => {
        if (!selectedProvider) {
            console.log('No provider selected, skipping model fetch');
            return;
        }

        const fetchModels = async () => {
            setLoadingModels(true);
            try {
                console.log('Fetching models for provider:', selectedProvider);
                const response = await fetch(`/api/llm/models?provider=${selectedProvider}`, {
                    credentials: 'include',
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
    }, [selectedProvider]);

    // Custom components for rendering markdown
    const markdownComponents = {
        code({ node, inline, className, children, ...props }) {
            const match = /language-(\w+)/.exec(className || '');
            const language = match ? match[1] : '';

            return !inline ? (
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
                    style={{
                        backgroundColor: 'rgba(0, 0, 0, 0.1)',
                        padding: '2px 6px',
                        borderRadius: '3px',
                        fontFamily: 'monospace',
                        fontSize: '0.875em',
                    }}
                    {...props}
                >
                    {children}
                </code>
            );
        },
        pre({ children }) {
            return <>{children}</>;
        },
        p({ children }) {
            return <Typography variant="body1" sx={{ mb: 1 }}>{children}</Typography>;
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
            return <Box component="ul" sx={{ pl: 2, my: 1 }}>{children}</Box>;
        },
        ol({ children }) {
            return <Box component="ol" sx={{ pl: 2, my: 1 }}>{children}</Box>;
        },
        li({ children }) {
            return <Typography component="li" variant="body1" sx={{ mb: 0.5 }}>{children}</Typography>;
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
        if (!input.trim() || loading) return;

        const userMessage = {
            role: 'user',
            content: input.trim(),
            timestamp: new Date().toISOString(),
        };

        // Add to query history
        setQueryHistory(prev => [...prev, userMessage.content]);
        setHistoryIndex(-1);
        setTempInput('');

        setMessages(prev => [...prev, userMessage]);
        setInput('');
        setLoading(true);
        setError('');
        startThinking();

        try {
            const response = await fetch('/api/chat', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                credentials: 'include',
                body: JSON.stringify({
                    message: userMessage.content,
                    provider: selectedProvider,
                    model: selectedModel,
                }),
            });

            // Handle session invalidation
            if (response.status === 401) {
                console.log('Session invalidated, logging out...');
                stopThinking();
                forceLogout();
                setError('Your session has expired. Please log in again.');
                return;
            }

            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                const errorMsg = errorData.message || 'Failed to send message';

                // Provide helpful error messages
                if (errorMsg.includes('API key')) {
                    throw new Error('Backend configuration error: ' + errorMsg);
                } else if (errorMsg.includes('ECONNREFUSED') || errorMsg.includes('fetch failed')) {
                    throw new Error('Cannot connect to backend server. Please check that the server is running.');
                } else {
                    throw new Error(errorMsg);
                }
            }

            const data = await response.json();

            const assistantMessage = {
                role: 'assistant',
                content: data.response || 'No response received',
                timestamp: new Date().toISOString(),
            };

            setMessages(prev => [...prev, assistantMessage]);
        } catch (err) {
            console.error('Chat error:', err);

            // Network errors
            if (err.name === 'TypeError' && err.message.includes('fetch')) {
                setError('Cannot connect to backend server. Please check that the server is running and configured correctly.');
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

    const handleClear = async () => {
        if (!window.confirm('Clear conversation history?')) return;

        try {
            const response = await fetch('/api/chat/clear', {
                method: 'POST',
                credentials: 'include',
            });

            if (!response.ok) {
                const errorData = await response.json();
                throw new Error(errorData.message || 'Failed to clear conversation');
            }

            setMessages([]);
            setError('');
        } catch (err) {
            setError(err.message || 'Failed to clear conversation');
            console.error('Clear conversation error:', err);
        }
    };

    return (
        <Box
            sx={{
                display: 'flex',
                flexDirection: 'column',
                height: 'calc(100vh - 200px)',
                minHeight: '500px',
            }}
        >
            {/* Chat Messages Area */}
            <Paper
                elevation={1}
                sx={{
                    flex: 1,
                    overflow: 'auto',
                    p: 2,
                    mb: 2,
                    bgcolor: 'background.paper',
                    position: 'relative',
                }}
            >
                {messages.length > 0 && (
                    <Box sx={{ display: 'flex', justifyContent: 'flex-end', mb: 2 }}>
                        <Button
                            size="small"
                            startIcon={<DeleteIcon />}
                            onClick={handleClear}
                            variant="outlined"
                            color="secondary"
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
                                        {message.role === 'user' ? 'You' : 'Assistant'}
                                    </Typography>
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
                                        ) : (
                                            <ReactMarkdown
                                                remarkPlugins={[remarkGfm]}
                                                components={markdownComponents}
                                            >
                                                {message.content}
                                            </ReactMarkdown>
                                        )}
                                    </Paper>
                                </Box>
                            </Box>
                        ))}
                        {loading && (
                            <Box
                                sx={{
                                    display: 'flex',
                                    alignItems: 'center',
                                    mb: 2,
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
                                        bgcolor: 'secondary.main',
                                        color: 'white',
                                        mr: 2,
                                    }}
                                >
                                    <BotIcon sx={{ fontSize: 20 }} />
                                </Box>
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
                            </Box>
                        )}
                        <div ref={messagesEndRef} />
                    </Box>
                )}
            </Paper>

            {/* Error Display */}
            {error && (
                <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
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
                <Box sx={{ display: 'flex', gap: 2 }}>
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
                </Box>
            </Paper>
        </Box>
    );
};

export default ChatInterface;
