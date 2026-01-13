/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Message Input Component
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Styled to match pgEdge Cloud product aesthetics
 *
 *-------------------------------------------------------------------------
 */

import React, { useRef, useEffect, useCallback } from 'react';
import PropTypes from 'prop-types';
import { Box, TextField, IconButton, Tooltip, useTheme, alpha } from '@mui/material';
import { Send as SendIcon, Stop as StopIcon, Psychology as PsychologyIcon, SaveAlt as SaveIcon } from '@mui/icons-material';

const MessageInput = React.memo(({
    value,
    onChange,
    onSend,
    onCancel,
    onKeyDown,
    disabled,
    isLoading = false,
    onPromptClick,
    hasPrompts = false,
    messages = [],
    showActivity = false,
    debug = false,
}) => {
    const inputRef = useRef(null);
    const theme = useTheme();
    const isDark = theme.palette.mode === 'dark';

    // Auto-focus input when it becomes enabled
    useEffect(() => {
        if (!disabled && inputRef.current) {
            // Use setTimeout to ensure the focus happens after the disabled state update
            const timer = setTimeout(() => {
                inputRef.current?.focus();
            }, 0);
            return () => clearTimeout(timer);
        }
    }, [disabled]);

    // Convert messages to Markdown format
    const convertToMarkdown = useCallback(() => {
        const lines = [];
        lines.push('# Chat History');
        lines.push('');
        lines.push(`*Exported: ${new Date().toLocaleString()}*`);
        lines.push('');
        lines.push('---');
        lines.push('');

        for (const msg of messages) {
            // Skip system messages unless debug is enabled
            if (msg.role === 'system' && !debug) {
                continue;
            }

            const timestamp = msg.timestamp
                ? new Date(msg.timestamp).toLocaleString()
                : '';

            if (msg.role === 'user') {
                lines.push('## User');
                if (timestamp) lines.push(`*${timestamp}*`);
                lines.push('');
                lines.push(msg.content);
                lines.push('');
            } else if (msg.role === 'assistant') {
                lines.push('## Assistant');
                if (timestamp) lines.push(`*${timestamp}*`);
                if (msg.provider && msg.model) {
                    lines.push(`*${msg.provider}: ${msg.model}*`);
                }
                lines.push('');

                // Include activity/tool calls if showActivity is enabled
                if (showActivity && msg.activity && msg.activity.length > 0) {
                    lines.push('### Activity');
                    lines.push('');
                    for (const act of msg.activity) {
                        if (act.type === 'tool') {
                            const tokenInfo = act.tokens ? ` (~${act.tokens} tokens)` : '';
                            const errorInfo = act.isError ? ' [ERROR]' : '';
                            if (act.name === 'read_resource' && act.uri) {
                                lines.push(`- **${act.name}**: \`${act.uri}\`${tokenInfo}${errorInfo}`);
                            } else {
                                lines.push(`- **${act.name}**${tokenInfo}${errorInfo}`);
                            }
                        } else if (act.type === 'compaction') {
                            lines.push(`- *Compacted: ${act.originalCount} → ${act.compactedCount} messages*`);
                        } else if (act.type === 'rate_limit_pause') {
                            lines.push(`- *Rate limit pause: ${act.message}*`);
                        }
                    }
                    lines.push('');
                }

                lines.push(msg.content);
                lines.push('');
            } else if (msg.role === 'system' && debug) {
                lines.push('## System');
                if (timestamp) lines.push(`*${timestamp}*`);
                lines.push('');
                lines.push(`> ${msg.content}`);
                lines.push('');
            }

            lines.push('---');
            lines.push('');
        }

        return lines.join('\n');
    }, [messages, showActivity, debug]);

    // Handle save button click
    const handleSave = useCallback(() => {
        if (messages.length === 0) return;

        const markdown = convertToMarkdown();
        const blob = new Blob([markdown], { type: 'text/markdown' });
        const url = URL.createObjectURL(blob);

        // Create a temporary link and trigger download
        const link = document.createElement('a');
        link.href = url;
        link.download = `chat-history-${new Date().toISOString().slice(0, 10)}.md`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);

        // Clean up the URL object
        URL.revokeObjectURL(url);
    }, [messages, convertToMarkdown]);

    return (
        <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', mb: 2 }}>
            <TextField
                inputRef={inputRef}
                fullWidth
                multiline
                maxRows={4}
                variant="outlined"
                placeholder="Ask about your database..."
                value={value}
                onChange={onChange}
                onKeyDown={onKeyDown}
                disabled={disabled}
                autoFocus
                sx={{
                    '& .MuiOutlinedInput-root': {
                        borderRadius: 1,
                        bgcolor: isDark ? alpha('#1E293B', 0.5) : '#FFFFFF',
                        '& fieldset': {
                            borderColor: isDark ? '#334155' : '#E5E7EB',
                        },
                        '&:hover fieldset': {
                            borderColor: isDark ? '#475569' : '#9CA3AF',
                        },
                        '&.Mui-focused fieldset': {
                            borderColor: '#15AABF',
                            borderWidth: 2,
                        },
                    },
                    '& .MuiInputBase-input': {
                        color: isDark ? '#F1F5F9' : '#1F2937',
                        '&::placeholder': {
                            color: isDark ? '#64748B' : '#9CA3AF',
                            opacity: 1,
                        },
                    },
                }}
            />
            {messages.length > 0 && (
                <Tooltip title="Save Chat History">
                    <IconButton
                        onClick={handleSave}
                        size="small"
                        sx={{
                            color: isDark ? '#94A3B8' : '#6B7280',
                            '&:hover': {
                                bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                color: '#15AABF',
                            },
                        }}
                    >
                        <SaveIcon />
                    </IconButton>
                </Tooltip>
            )}
            {hasPrompts && (
                <Tooltip title="Execute Prompt">
                    <IconButton
                        onClick={onPromptClick}
                        disabled={disabled}
                        size="small"
                        sx={{
                            color: isDark ? '#94A3B8' : '#6B7280',
                            '&:hover': {
                                bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                color: '#15AABF',
                            },
                            '&.Mui-disabled': {
                                color: isDark ? '#475569' : '#D1D5DB',
                            },
                        }}
                    >
                        <PsychologyIcon />
                    </IconButton>
                </Tooltip>
            )}
            <Tooltip title={isLoading ? "Cancel request" : "Send message"}>
                <span>
                    <IconButton
                        onClick={isLoading ? onCancel : onSend}
                        disabled={isLoading ? false : (!value.trim() || disabled)}
                        sx={{
                            bgcolor: isLoading ? '#EF4444' : '#15AABF',
                            color: 'white',
                            width: 40,
                            height: 40,
                            '&:hover': {
                                bgcolor: isLoading ? '#DC2626' : '#0C8599',
                            },
                            '&.Mui-disabled': {
                                bgcolor: isDark ? '#334155' : '#E5E7EB',
                                color: isDark ? '#64748B' : '#9CA3AF',
                            },
                        }}
                    >
                        {isLoading ? <StopIcon /> : <SendIcon />}
                    </IconButton>
                </span>
            </Tooltip>
        </Box>
    );
});

MessageInput.displayName = 'MessageInput';

MessageInput.propTypes = {
    value: PropTypes.string.isRequired,
    onChange: PropTypes.func.isRequired,
    onSend: PropTypes.func.isRequired,
    onCancel: PropTypes.func,
    onKeyDown: PropTypes.func.isRequired,
    disabled: PropTypes.bool.isRequired,
    isLoading: PropTypes.bool,
    onPromptClick: PropTypes.func,
    hasPrompts: PropTypes.bool,
    messages: PropTypes.arrayOf(PropTypes.object),
    showActivity: PropTypes.bool,
    debug: PropTypes.bool,
};

export default MessageInput;
