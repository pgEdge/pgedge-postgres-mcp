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
} from '@mui/material';
import {
    Send as SendIcon,
    Person as PersonIcon,
    SmartToy as BotIcon,
    Delete as DeleteIcon,
} from '@mui/icons-material';
import { useAuth } from '../contexts/AuthContext';

const ChatInterface = () => {
    const { forceLogout } = useAuth();
    const [messages, setMessages] = useState([]);
    const [input, setInput] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const messagesEndRef = useRef(null);

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
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

        setMessages(prev => [...prev, userMessage]);
        setInput('');
        setLoading(true);
        setError('');

        try {
            const response = await fetch('/api/chat', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                credentials: 'include',
                body: JSON.stringify({
                    message: userMessage.content,
                }),
            });

            // Handle session invalidation
            if (response.status === 401) {
                console.log('Session invalidated, logging out...');
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
            setLoading(false);
        }
    };

    const handleKeyPress = (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            handleSend();
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
                                        <Typography
                                            variant="body1"
                                            sx={{
                                                whiteSpace: 'pre-wrap',
                                                wordBreak: 'break-word',
                                            }}
                                        >
                                            {message.content}
                                        </Typography>
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
                                <CircularProgress size={20} />
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
                    display: 'flex',
                    gap: 1,
                    alignItems: 'flex-end',
                }}
            >
                <TextField
                    fullWidth
                    multiline
                    maxRows={4}
                    variant="outlined"
                    placeholder="Type your message..."
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyPress={handleKeyPress}
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
            </Paper>
        </Box>
    );
};

export default ChatInterface;
