/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Message Component
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React from 'react';
import PropTypes from 'prop-types';
import { Box, Paper, Typography, useTheme, Chip } from '@mui/material';
import { Person as PersonIcon, SmartToy as BotIcon, Info as InfoIcon, Psychology as PsychologyIcon } from '@mui/icons-material';
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

const Message = React.memo(({ message, showActivity, renderMarkdown, debug }) => {
    const theme = useTheme();
    const markdownComponents = createMarkdownComponents(theme);

    // System messages have a different layout
    if (message.role === 'system') {
        return (
            <Box sx={{ mb: 2, display: 'flex', justifyContent: 'center' }}>
                <Chip
                    icon={<InfoIcon />}
                    label={message.content}
                    color="info"
                    variant="outlined"
                    sx={{
                        maxWidth: '100%',
                        height: 'auto',
                        py: 1,
                        '& .MuiChip-label': {
                            whiteSpace: 'normal',
                            textAlign: 'center',
                        },
                    }}
                />
            </Box>
        );
    }

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
            <Box sx={{
                flex: 1,
                ...(message.isError && {
                    borderLeft: '3px solid',
                    borderColor: 'error.main',
                    paddingLeft: 2,
                    backgroundColor: 'error.light',
                    opacity: 0.9,
                    borderRadius: 1,
                    padding: 1
                })
            }}>
                {/* Header */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                    <Typography
                        variant="caption"
                        color="text.secondary"
                    >
                        {message.role === 'user'
                            ? 'You'
                            : message.provider && message.model
                                ? `${message.provider.charAt(0).toUpperCase() + message.provider.slice(1)} (${getShortModelName(message.model)})`
                                : 'Assistant'
                        }
                    </Typography>
                    {message.fromPrompt && (
                        <Chip
                            icon={<PsychologyIcon sx={{ fontSize: 14 }} />}
                            label="From Prompt"
                            size="small"
                            color="primary"
                            variant="outlined"
                            sx={{
                                height: 20,
                                fontSize: '0.65rem',
                                '& .MuiChip-icon': {
                                    marginLeft: '4px',
                                },
                            }}
                        />
                    )}
                    {message.isError && (
                        <Chip
                            label="Error"
                            size="small"
                            color="error"
                            variant="filled"
                            sx={{
                                height: 20,
                                fontSize: '0.65rem',
                            }}
                        />
                    )}
                </Box>

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
                                    <>🔧 {activity.name}{activity.uri ? ` (${activity.uri})` : ''}{debug && activity.tokens != null ? ` ~${activity.tokens.toLocaleString()} tokens` : ''}{activity.isError ? ' ❌' : ''}</>
                                )}
                                {activity.type === 'resource' && (
                                    <>📄 {activity.uri}{debug && activity.tokens != null ? ` ~${activity.tokens.toLocaleString()} tokens` : ''}</>
                                )}
                                {activity.type === 'compaction' && (
                                    <>📦 Compacting history: {activity.originalCount} → {activity.compactedCount} messages{activity.tokensSaved ? ` (saved ${activity.tokensSaved} tokens)` : ''}{activity.local ? ' [local]' : ''}</>
                                )}
                                {activity.type === 'rate_limit_pause' && (
                                    <>⏳ {activity.message} {activity.cumulativeTokens > 0
                                        ? `(used ~${activity.cumulativeTokens?.toLocaleString()} tokens in ${activity.requestCount} requests)`
                                        : `(~${activity.estimatedTokens?.toLocaleString()} tokens)`} - pausing 60s before retry...</>
                                )}
                            </Typography>
                        ))}
                    </Box>
                )}

                {/* Token Usage Debug Info */}
                {debug && message.role === 'assistant' && message.tokenUsage && (
                    <Box sx={{ mb: 1 }}>
                        <Typography
                            variant="caption"
                            sx={{
                                display: 'block',
                                color: 'info.main',
                                fontFamily: 'monospace',
                                fontSize: '0.7rem',
                                mb: 0.2,
                            }}
                        >
                            {message.tokenUsage.provider === 'anthropic' && (
                                <>
                                    {message.tokenUsage.cache_creation_tokens > 0 || message.tokenUsage.cache_read_tokens > 0 ? (
                                        <>
                                            <div>📊 Prompt Cache: Created {message.tokenUsage.cache_creation_tokens || 0}, Read {message.tokenUsage.cache_read_tokens || 0} (saved ~{message.tokenUsage.cache_savings_percentage?.toFixed(0)}%)</div>
                                            <div>🔢 Tokens: Input {message.tokenUsage.prompt_tokens}, Output {message.tokenUsage.completion_tokens}, Total {message.tokenUsage.total_tokens}</div>
                                        </>
                                    ) : (
                                        <div>🔢 Tokens: Input {message.tokenUsage.prompt_tokens}, Output {message.tokenUsage.completion_tokens}, Total {message.tokenUsage.total_tokens}</div>
                                    )}
                                </>
                            )}
                            {message.tokenUsage.provider === 'openai' && (
                                <div>🔢 Tokens: Prompt {message.tokenUsage.prompt_tokens}, Completion {message.tokenUsage.completion_tokens}, Total {message.tokenUsage.total_tokens}</div>
                            )}
                            {message.tokenUsage.provider === 'ollama' && (
                                <div>ℹ️ Ollama does not provide token counts</div>
                            )}
                        </Typography>
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
        role: PropTypes.oneOf(['user', 'assistant', 'system']).isRequired,
        content: PropTypes.string.isRequired,
        timestamp: PropTypes.string,
        provider: PropTypes.string,
        model: PropTypes.string,
        activity: PropTypes.arrayOf(PropTypes.shape({
            type: PropTypes.string,
            name: PropTypes.string,
            uri: PropTypes.string,
            tokens: PropTypes.number,
            isError: PropTypes.bool,
        })),
        isThinking: PropTypes.bool,
        isError: PropTypes.bool,
        fromPreviousSession: PropTypes.bool,
        tokenUsage: PropTypes.shape({
            provider: PropTypes.string,
            prompt_tokens: PropTypes.number,
            completion_tokens: PropTypes.number,
            total_tokens: PropTypes.number,
            cache_creation_tokens: PropTypes.number,
            cache_read_tokens: PropTypes.number,
            cache_savings_percentage: PropTypes.number,
        }),
    }).isRequired,
    showActivity: PropTypes.bool.isRequired,
    renderMarkdown: PropTypes.bool.isRequired,
    debug: PropTypes.bool.isRequired,
};

export default Message;
