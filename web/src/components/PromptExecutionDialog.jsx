/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Prompt Execution Dialog
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useState, useEffect } from 'react';
import PropTypes from 'prop-types';
import {
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    Button,
    TextField,
    Box,
    Typography,
    Chip,
    Alert,
    CircularProgress,
} from '@mui/material';
import {
    PlayArrow as PlayArrowIcon,
    Close as CloseIcon,
} from '@mui/icons-material';

const PromptExecutionDialog = ({
    open,
    onClose,
    prompt = null,
    onExecute,
    executing = false,
}) => {
    const [argumentValues, setArgumentValues] = useState({});
    const [validationErrors, setValidationErrors] = useState({});

    // Reset state when prompt changes
    useEffect(() => {
        if (prompt) {
            // Initialize with empty values
            const initialValues = {};
            if (prompt.arguments) {
                prompt.arguments.forEach(arg => {
                    initialValues[arg.name] = '';
                });
            }
            setArgumentValues(initialValues);
            setValidationErrors({});
        }
    }, [prompt]);

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

        if (prompt.arguments) {
            prompt.arguments.forEach(arg => {
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

        onExecute(prompt.name, filteredArgs);
    };

    const handleKeyPress = (event) => {
        if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            handleExecute();
        }
    };

    if (!prompt) {
        return null;
    }

    const hasArguments = prompt.arguments && prompt.arguments.length > 0;

    return (
        <Dialog
            open={open}
            onClose={onClose}
            maxWidth="sm"
            fullWidth
            aria-labelledby="prompt-dialog-title"
        >
            <DialogTitle id="prompt-dialog-title">
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <PlayArrowIcon color="primary" />
                    <Typography variant="h6" component="span">
                        Execute Prompt: {prompt.name}
                    </Typography>
                </Box>
            </DialogTitle>

            <DialogContent>
                {/* Prompt Description */}
                {prompt.description && (
                    <Alert severity="info" sx={{ mb: 3 }}>
                        {prompt.description}
                    </Alert>
                )}

                {/* Arguments Section */}
                {hasArguments ? (
                    <Box sx={{ mt: 2 }}>
                        <Typography variant="subtitle2" gutterBottom sx={{ mb: 2 }}>
                            Arguments:
                        </Typography>
                        {prompt.arguments.map((arg) => (
                            <Box key={arg.name} sx={{ mb: 2 }}>
                                <TextField
                                    fullWidth
                                    label={arg.name}
                                    value={argumentValues[arg.name] || ''}
                                    onChange={(e) => handleArgumentChange(arg.name, e.target.value)}
                                    onKeyPress={handleKeyPress}
                                    error={!!validationErrors[arg.name]}
                                    helperText={validationErrors[arg.name] || arg.description}
                                    required={arg.required}
                                    disabled={executing}
                                    InputProps={{
                                        endAdornment: arg.required && (
                                            <Chip
                                                label="Required"
                                                size="small"
                                                color="error"
                                                variant="outlined"
                                            />
                                        ),
                                    }}
                                />
                            </Box>
                        ))}
                    </Box>
                ) : (
                    <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
                        This prompt has no arguments. Click Execute to run it.
                    </Typography>
                )}
            </DialogContent>

            <DialogActions>
                <Button
                    onClick={onClose}
                    disabled={executing}
                    startIcon={<CloseIcon />}
                >
                    Cancel
                </Button>
                <Button
                    onClick={handleExecute}
                    variant="contained"
                    disabled={executing}
                    startIcon={executing ? <CircularProgress size={16} /> : <PlayArrowIcon />}
                >
                    {executing ? 'Executing...' : 'Execute'}
                </Button>
            </DialogActions>
        </Dialog>
    );
};

PromptExecutionDialog.propTypes = {
    open: PropTypes.bool.isRequired,
    onClose: PropTypes.func.isRequired,
    prompt: PropTypes.shape({
        name: PropTypes.string.isRequired,
        description: PropTypes.string,
        arguments: PropTypes.arrayOf(PropTypes.shape({
            name: PropTypes.string.isRequired,
            description: PropTypes.string,
            required: PropTypes.bool,
        })),
    }),
    onExecute: PropTypes.func.isRequired,
    executing: PropTypes.bool,
};

export default PromptExecutionDialog;
