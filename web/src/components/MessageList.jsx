/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Message List Component
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Styled to match pgEdge Cloud product aesthetics
 *
 *-------------------------------------------------------------------------
 */

import React, { useRef, useEffect } from 'react';
import PropTypes from 'prop-types';
import { Box, Paper, Typography, useTheme, alpha } from '@mui/material';
import { SmartToy as BotIcon } from '@mui/icons-material';
import Message from './Message';

const MessageList = React.memo(({ messages, showActivity, renderMarkdown, debug }) => {
    const messagesEndRef = useRef(null);
    const theme = useTheme();
    const isDark = theme.palette.mode === 'dark';

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    };

    useEffect(() => {
        scrollToBottom();
    }, [messages]);

    return (
        <Paper
            elevation={0}
            sx={{
                flex: 1,
                overflow: 'auto',
                p: 2,
                mb: 1,
                bgcolor: isDark ? '#0F172A' : '#FFFFFF',
                border: '1px solid',
                borderColor: isDark ? '#334155' : '#E5E7EB',
                borderRadius: 3,
                position: 'relative',
            }}
        >
            {messages.length === 0 ? (
                <Box
                    sx={{
                        display: 'flex',
                        flexDirection: 'column',
                        alignItems: 'center',
                        justifyContent: 'center',
                        height: '100%',
                    }}
                >
                    <Box
                        sx={{
                            width: 80,
                            height: 80,
                            borderRadius: '50%',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            bgcolor: isDark ? alpha('#22B8CF', 0.1) : alpha('#15AABF', 0.08),
                            mb: 2,
                        }}
                    >
                        <BotIcon sx={{
                            fontSize: 40,
                            color: isDark ? '#22B8CF' : '#15AABF',
                        }} />
                    </Box>
                    <Typography
                        variant="h6"
                        gutterBottom
                        sx={{
                            color: isDark ? '#F1F5F9' : '#1F2937',
                            fontWeight: 600,
                        }}
                    >
                        Start a conversation
                    </Typography>
                    <Typography
                        variant="body2"
                        sx={{
                            color: isDark ? '#64748B' : '#9CA3AF',
                            textAlign: 'center',
                            maxWidth: 300,
                        }}
                    >
                        Ask questions about your PostgreSQL database using natural language
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
                            debug={debug}
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
    debug: PropTypes.bool.isRequired,
};

export default MessageList;
