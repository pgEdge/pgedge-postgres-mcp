/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Message List Component
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useRef, useEffect } from 'react';
import PropTypes from 'prop-types';
import { Box, Paper, Button, Typography } from '@mui/material';
import { Delete as DeleteIcon, SmartToy as BotIcon } from '@mui/icons-material';
import Message from './Message';

const MessageList = React.memo(({ messages, showActivity, renderMarkdown, onClear }) => {
    const messagesEndRef = useRef(null);

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    };

    useEffect(() => {
        scrollToBottom();
    }, [messages]);

    return (
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
                        onClick={onClear}
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
                        <Message
                            key={index}
                            message={message}
                            showActivity={showActivity}
                            renderMarkdown={renderMarkdown}
                        />
                    ))}
                    <div ref={messagesEndRef} />
                </Box>
            )}
        </Paper>
    );
});

MessageList.displayName = 'MessageList';

MessageList.propTypes = {
    messages: PropTypes.arrayOf(PropTypes.object).isRequired,
    showActivity: PropTypes.bool.isRequired,
    renderMarkdown: PropTypes.bool.isRequired,
    onClear: PropTypes.func.isRequired,
};

export default MessageList;
