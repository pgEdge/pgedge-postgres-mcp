/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Message Component
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React from 'react';
import PropTypes from 'prop-types';
import { Box, Paper, Typography, useTheme } from '@mui/material';
import { Person as PersonIcon, SmartToy as BotIcon } from '@mui/icons-material';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { createMarkdownComponents } from './MarkdownComponents';
import ThinkingIndicator from './ThinkingIndicator';

/**
 * Helper function to get short model name for display
 */
const getShortModelName = (modelName) => {
    if (!modelName) return '';

    if (modelName.startsWith('claude-')) {
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
        return modelName.replace('gpt-', 'GPT-').replace('-turbo', '').toUpperCase();
    } else if (modelName.startsWith('o1-') || modelName.startsWith('o3-')) {
        return modelName.split('-')[0].toUpperCase();
    }

    const firstPart = modelName.split(':')[0];
    return firstPart.length <= 15 ? firstPart : modelName.substring(0, 15) + '...';
};

const Message = React.memo(({ message, showActivity, renderMarkdown }) => {
    const theme = useTheme();
    const markdownComponents = createMarkdownComponents(theme);

    return (
        <Box
            sx={{
                display: 'flex',
                mb: 2,
                alignItems: 'flex-start',
                opacity: message.fromPreviousSession ? 0.6 : 1,
                transition: 'opacity 0.3s ease-in-out',
            }}
        >
            {/* Avatar */}
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

            {/* Message Content */}
            <Box sx={{ flex: 1 }}>
                {/* Header */}
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

                {/* Activity Log */}
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

                {/* Message Body */}
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
                        <ThinkingIndicator isThinking={true} />
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
    );
});

Message.displayName = 'Message';

Message.propTypes = {
    message: PropTypes.shape({
        role: PropTypes.oneOf(['user', 'assistant']).isRequired,
        content: PropTypes.string.isRequired,
        timestamp: PropTypes.string,
        provider: PropTypes.string,
        model: PropTypes.string,
        activity: PropTypes.arrayOf(PropTypes.shape({
            type: PropTypes.string,
            name: PropTypes.string,
            uri: PropTypes.string,
        })),
        isThinking: PropTypes.bool,
        fromPreviousSession: PropTypes.bool,
    }).isRequired,
    showActivity: PropTypes.bool.isRequired,
    renderMarkdown: PropTypes.bool.isRequired,
};

export default Message;
