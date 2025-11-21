/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Message Input Component
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React from 'react';
import PropTypes from 'prop-types';
import { Box, TextField, IconButton } from '@mui/material';
import { Send as SendIcon } from '@mui/icons-material';

const MessageInput = ({ value, onChange, onSend, onKeyDown, disabled }) => {
    return (
        <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', mb: 2 }}>
            <TextField
                fullWidth
                multiline
                maxRows={4}
                variant="outlined"
                placeholder="Type your message..."
                value={value}
                onChange={onChange}
                onKeyDown={onKeyDown}
                disabled={disabled}
                sx={{
                    '& .MuiOutlinedInput-root': {
                        borderRadius: 2,
                    },
                }}
            />
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
};

MessageInput.propTypes = {
    value: PropTypes.string.isRequired,
    onChange: PropTypes.func.isRequired,
    onSend: PropTypes.func.isRequired,
    onKeyDown: PropTypes.func.isRequired,
    disabled: PropTypes.bool.isRequired,
};

export default MessageInput;
