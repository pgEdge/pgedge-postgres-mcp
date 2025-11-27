/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Message Input Component
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useRef, useEffect } from 'react';
import PropTypes from 'prop-types';
import { Box, TextField, IconButton, Tooltip } from '@mui/material';
import { Send as SendIcon, Psychology as PsychologyIcon } from '@mui/icons-material';

const MessageInput = React.memo(({ value, onChange, onSend, onKeyDown, disabled, onPromptClick, hasPrompts = false }) => {
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
};

export default MessageInput;
