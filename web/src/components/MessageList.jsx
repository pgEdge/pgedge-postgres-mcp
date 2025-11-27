/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Message List Component
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useRef, useEffect } from 'react';
import PropTypes from 'prop-types';
import { Box, Paper, Typography } from '@mui/material';
import { SmartToy as BotIcon } from '@mui/icons-material';
import Message from './Message';

const MessageList = React.memo(({ messages, showActivity, renderMarkdown, debug }) => {
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
