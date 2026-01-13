/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Prompt Popover Component
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Styled to match pgEdge Cloud product aesthetics
 *
 *-------------------------------------------------------------------------
 */

import React, { useState, useEffect } from 'react';
import PropTypes from 'prop-types';
import {
    Popover,
    Box,
    Typography,
    FormControl,
    InputLabel,
    Select,
    MenuItem,
    TextField,
    Button,
    Alert,
    Divider,
    CircularProgress,
    useTheme,
    alpha,
} from '@mui/material';
import {
    PlayArrow as PlayArrowIcon,
} from '@mui/icons-material';

const PromptPopover = ({
    anchorEl,
    open,
    onClose,
    prompts = [],
    onExecute,
    executing = false,
}) => {
    const [selectedPromptName, setSelectedPromptName] = useState('');
    const [argumentValues, setArgumentValues] = useState({});
    const [validationErrors, setValidationErrors] = useState({});
    const theme = useTheme();
    const isDark = theme.palette.mode === 'dark';

    // Get the selected prompt object
    const selectedPrompt = prompts.find(p => p.name === selectedPromptName) || null;

    // Reset when prompt changes
    useEffect(() => {
        if (selectedPrompt) {
            const initialValues = {};
            if (selectedPrompt.arguments) {
                selectedPrompt.arguments.forEach(arg => {
                    initialValues[arg.name] = '';
                });
            }
            setArgumentValues(initialValues);
            setValidationErrors({});
        }
    }, [selectedPromptName]);

    const handleArgumentChange = (argName, value) => {
        setArgumentValues(prev => ({
            ...prev,
            [argName]: value,
        }));

        // Clear validation error when user starts typing
        if (validationErrors[argName]) {
            setValidationErrors(prev => {
                const newErrors = { ...prev };
                delete newErrors[argName];
                return newErrors;
            });
        }
    };

    const validateArguments = () => {
        const errors = {};
        let isValid = true;

        if (selectedPrompt && selectedPrompt.arguments) {
            selectedPrompt.arguments.forEach(arg => {
                if (arg.required && !argumentValues[arg.name]?.trim()) {
                    errors[arg.name] = 'This field is required';
                    isValid = false;
                }
            });
        }

        setValidationErrors(errors);
        return isValid;
    };

    const handleExecute = () => {
        if (!selectedPrompt) return;

        if (!validateArguments()) {
            return;
        }

        // Only send non-empty arguments
        const filteredArgs = {};
        Object.entries(argumentValues).forEach(([key, value]) => {
            if (value && value.trim()) {
                filteredArgs[key] = value.trim();
            }
        });

        onExecute(selectedPrompt.name, filteredArgs);

        // Reset and close
        setSelectedPromptName('');
        setArgumentValues({});
        setValidationErrors({});
        onClose();
    };

    const handleClose = () => {
        setSelectedPromptName('');
        setArgumentValues({});
        setValidationErrors({});
        onClose();
    };

    const hasArguments = selectedPrompt?.arguments && selectedPrompt.arguments.length > 0;

    const selectStyles = {
        borderRadius: 1,
        bgcolor: isDark ? alpha('#1E293B', 0.5) : '#FFFFFF',
        '& .MuiOutlinedInput-notchedOutline': {
            borderColor: isDark ? '#334155' : '#E5E7EB',
        },
        '&:hover .MuiOutlinedInput-notchedOutline': {
            borderColor: isDark ? '#475569' : '#9CA3AF',
        },
        '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
            borderColor: '#15AABF',
            borderWidth: 2,
        },
        '& .MuiSelect-select': {
            color: isDark ? '#F1F5F9' : '#1F2937',
        },
        '& .MuiSelect-icon': {
            color: isDark ? '#94A3B8' : '#6B7280',
        },
    };

    const textFieldStyles = {
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
        '& .MuiInputLabel-root': {
            color: isDark ? '#94A3B8' : '#6B7280',
            '&.Mui-focused': {
                color: '#15AABF',
            },
        },
        '& .MuiInputBase-input': {
            color: isDark ? '#F1F5F9' : '#1F2937',
        },
        '& .MuiFormHelperText-root': {
            color: isDark ? '#64748B' : '#9CA3AF',
        },
    };

    return (
        <Popover
            open={open}
            anchorEl={anchorEl}
            onClose={handleClose}
            disableScrollLock
            anchorOrigin={{
                vertical: 'top',
                horizontal: 'right',
            }}
            transformOrigin={{
                vertical: 'bottom',
                horizontal: 'right',
            }}
            PaperProps={{
                sx: {
                    bgcolor: isDark ? '#1E293B' : '#FFFFFF',
                    border: '1px solid',
                    borderColor: isDark ? '#334155' : '#E5E7EB',
                    borderRadius: 1,
                    boxShadow: isDark
                        ? '0 10px 15px -3px rgba(0, 0, 0, 0.3)'
                        : '0 10px 15px -3px rgba(0, 0, 0, 0.1)',
                },
            }}
        >
            <Box sx={{ p: 2.5, minWidth: 400, maxWidth: 500 }}>
                <Typography
                    variant="h6"
                    sx={{
                        mb: 2,
                        color: isDark ? '#F1F5F9' : '#1F2937',
                        fontWeight: 600,
                        fontSize: '1rem',
                    }}
                >
                    Execute Prompt
                </Typography>

                <Divider sx={{ mb: 2, borderColor: isDark ? '#334155' : '#E5E7EB' }} />

                {/* Prompt Selection */}
                <FormControl fullWidth size="small" sx={{ mb: 2 }}>
                    <InputLabel
                        id="prompt-popover-select-label"
                        sx={{
                            color: isDark ? '#94A3B8' : '#6B7280',
                            '&.Mui-focused': {
                                color: '#15AABF',
                            },
                        }}
                    >
                        Select Prompt
                    </InputLabel>
                    <Select
                        labelId="prompt-popover-select-label"
                        id="prompt-popover-select"
                        value={selectedPromptName}
                        label="Select Prompt"
                        onChange={(e) => setSelectedPromptName(e.target.value)}
                        disabled={executing}
                        sx={selectStyles}
                        MenuProps={{
                            PaperProps: {
                                sx: {
                                    bgcolor: isDark ? '#1E293B' : '#FFFFFF',
                                    border: '1px solid',
                                    borderColor: isDark ? '#334155' : '#E5E7EB',
                                    borderRadius: 1,
                                },
                            },
                        }}
                    >
                        <MenuItem
                            value=""
                            sx={{ color: isDark ? '#64748B' : '#9CA3AF' }}
                        >
                            <em>Select a prompt...</em>
                        </MenuItem>
                        {[...prompts].sort((a, b) => a.name.localeCompare(b.name)).map((prompt) => (
                            <MenuItem
                                key={prompt.name}
                                value={prompt.name}
                                sx={{
                                    color: isDark ? '#F1F5F9' : '#1F2937',
                                    '&:hover': {
                                        bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                    },
                                    '&.Mui-selected': {
                                        bgcolor: isDark ? alpha('#22B8CF', 0.16) : alpha('#15AABF', 0.08),
                                        '&:hover': {
                                            bgcolor: isDark ? alpha('#22B8CF', 0.24) : alpha('#15AABF', 0.12),
                                        },
                                    },
                                }}
                            >
                                {prompt.name}
                            </MenuItem>
                        ))}
                    </Select>
                </FormControl>

                {/* Selected Prompt Info */}
                {selectedPrompt && (
                    <>
                        {/* Description */}
                        {selectedPrompt.description && (
                            <Alert
                                severity="info"
                                sx={{
                                    mb: 2,
                                    bgcolor: isDark ? alpha('#3B82F6', 0.1) : alpha('#3B82F6', 0.05),
                                    color: isDark ? '#60A5FA' : '#1E40AF',
                                    border: '1px solid',
                                    borderColor: isDark ? alpha('#3B82F6', 0.2) : alpha('#3B82F6', 0.1),
                                    borderRadius: 1,
                                    '& .MuiAlert-icon': {
                                        color: isDark ? '#60A5FA' : '#3B82F6',
                                    },
                                }}
                            >
                                {selectedPrompt.description}
                            </Alert>
                        )}

                        {/* Arguments */}
                        {hasArguments && (
                            <Box sx={{ mb: 2 }}>
                                <Typography
                                    variant="subtitle2"
                                    gutterBottom
                                    sx={{
                                        color: isDark ? '#94A3B8' : '#6B7280',
                                        fontWeight: 500,
                                    }}
                                >
                                    Arguments:
                                </Typography>
                                {selectedPrompt.arguments.map((arg) => (
                                    <TextField
                                        key={arg.name}
                                        fullWidth
                                        size="small"
                                        label={arg.name}
                                        value={argumentValues[arg.name] || ''}
                                        onChange={(e) => handleArgumentChange(arg.name, e.target.value)}
                                        error={!!validationErrors[arg.name]}
                                        helperText={validationErrors[arg.name] || arg.description}
                                        required={arg.required}
                                        disabled={executing}
                                        sx={{ ...textFieldStyles, mb: 2 }}
                                    />
                                ))}
                            </Box>
                        )}

                        {/* Execute Button */}
                        <Button
                            fullWidth
                            variant="contained"
                            onClick={handleExecute}
                            disabled={executing}
                            startIcon={executing ? <CircularProgress size={16} sx={{ color: '#FFFFFF' }} /> : <PlayArrowIcon />}
                            sx={{
                                bgcolor: '#15AABF',
                                color: '#FFFFFF',
                                borderRadius: 1,
                                textTransform: 'none',
                                fontWeight: 500,
                                '&:hover': {
                                    bgcolor: '#0C8599',
                                },
                                '&.Mui-disabled': {
                                    bgcolor: isDark ? '#334155' : '#E5E7EB',
                                    color: isDark ? '#64748B' : '#9CA3AF',
                                },
                            }}
                        >
                            {executing ? 'Executing...' : 'Execute Prompt'}
                        </Button>
                    </>
                )}

                {/* Help Text when no prompt selected */}
                {!selectedPrompt && (
                    <Typography
                        variant="body2"
                        sx={{ color: isDark ? '#64748B' : '#9CA3AF' }}
                    >
                        Select a prompt from the dropdown above to get started.
                    </Typography>
                )}
            </Box>
        </Popover>
    );
};

PromptPopover.propTypes = {
    anchorEl: PropTypes.object,
    open: PropTypes.bool.isRequired,
    onClose: PropTypes.func.isRequired,
    prompts: PropTypes.arrayOf(PropTypes.shape({
        name: PropTypes.string.isRequired,
        description: PropTypes.string,
        arguments: PropTypes.arrayOf(PropTypes.shape({
            name: PropTypes.string.isRequired,
            description: PropTypes.string,
            required: PropTypes.bool,
        })),
    })),
    onExecute: PropTypes.func.isRequired,
    executing: PropTypes.bool,
};

export default PromptPopover;
