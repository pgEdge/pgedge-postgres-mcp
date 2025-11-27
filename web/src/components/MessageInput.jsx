/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Message Input Component
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useRef, useEffect, useCallback } from 'react';
import PropTypes from 'prop-types';
import { Box, TextField, IconButton, Tooltip } from '@mui/material';
import { Send as SendIcon, Psychology as PsychologyIcon, SaveAlt as SaveIcon } from '@mui/icons-material';

const MessageInput = React.memo(({
    value,
    onChange,
    onSend,
    onKeyDown,
    disabled,
    onPromptClick,
    hasPrompts = false,
    messages = [],
    showActivity = false,
    debug = false,
}) => {
    const inputRef = useRef(null);

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
                placeholder="Type your message..."
                value={value}
                onChange={onChange}
                onKeyDown={onKeyDown}
                disabled={disabled}
                autoFocus
                sx={{
                    '& .MuiOutlinedInput-root': {
                        borderRadius: 2,
                    },
                }}
            />
            {messages.length > 0 && (
                <Tooltip title="Save Chat History">
                    <IconButton
                        onClick={handleSave}
                        size="small"
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
                    >
                        <PsychologyIcon />
                    </IconButton>
                </Tooltip>
            )}
            <IconButton
                color="primary"
                onClick={onSend}
                disabled={!value.trim() || disabled}
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
    );
});

MessageInput.displayName = 'MessageInput';

MessageInput.propTypes = {
    value: PropTypes.string.isRequired,
    onChange: PropTypes.func.isRequired,
    onSend: PropTypes.func.isRequired,
    onKeyDown: PropTypes.func.isRequired,
    disabled: PropTypes.bool.isRequired,
    onPromptClick: PropTypes.func,
    hasPrompts: PropTypes.bool,
    messages: PropTypes.arrayOf(PropTypes.object),
    showActivity: PropTypes.bool,
    debug: PropTypes.bool,
};

export default MessageInput;
