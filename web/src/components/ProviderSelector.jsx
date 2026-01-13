/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Provider Selector Component
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Styled to match pgEdge Cloud product aesthetics
 *
 *-------------------------------------------------------------------------
 */

import React, { useState } from 'react';
import PropTypes from 'prop-types';
import {
    Box,
    FormControl,
    InputLabel,
    Select,
    MenuItem,
    IconButton,
    Tooltip,
    useTheme,
    alpha,
} from '@mui/material';
import {
    Settings as SettingsIcon,
    Delete as DeleteIcon,
} from '@mui/icons-material';
import PreferencesPopover from './PreferencesPopover';

const ProviderSelector = React.memo(({
    providers,
    selectedProvider,
    onProviderChange,
    models,
    selectedModel,
    onModelChange,
    showActivity,
    onActivityChange,
    renderMarkdown,
    onMarkdownChange,
    debug,
    onDebugChange,
    disabled,
    loadingModels,
    onClear,
    hasMessages = false,
}) => {
    const [preferencesAnchor, setPreferencesAnchor] = useState(null);
    const theme = useTheme();
    const isDark = theme.palette.mode === 'dark';

    const handlePreferencesClick = (event) => {
        setPreferencesAnchor(event.currentTarget);
    };

    const handlePreferencesClose = () => {
        setPreferencesAnchor(null);
    };

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

    const labelStyles = {
        color: isDark ? '#94A3B8' : '#6B7280',
        '&.Mui-focused': {
            color: '#15AABF',
        },
    };

    const iconButtonStyles = {
        color: isDark ? '#94A3B8' : '#6B7280',
        '&:hover': {
            bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
            color: '#15AABF',
        },
        '&.Mui-disabled': {
            color: isDark ? '#475569' : '#D1D5DB',
        },
    };

    return (
        <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
            {/* Provider Selection */}
            <FormControl sx={{ minWidth: 200 }} size="small">
                <InputLabel id="provider-select-label" sx={labelStyles}>
                    Provider
                </InputLabel>
                <Select
                    labelId="provider-select-label"
                    id="provider-select"
                    value={selectedProvider}
                    label="Provider"
                    onChange={(e) => onProviderChange(e.target.value)}
                    disabled={disabled}
                    sx={selectStyles}
                    MenuProps={{
                        PaperProps: {
                            sx: {
                                bgcolor: isDark ? '#1E293B' : '#FFFFFF',
                                border: '1px solid',
                                borderColor: isDark ? '#334155' : '#E5E7EB',
                                borderRadius: 1,
                                boxShadow: isDark
                                    ? '0 10px 15px -3px rgba(0, 0, 0, 0.3)'
                                    : '0 10px 15px -3px rgba(0, 0, 0, 0.1)',
                            },
                        },
                    }}
                >
                    {providers.map((provider) => (
                        <MenuItem
                            key={provider.name}
                            value={provider.name}
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
                            {provider.display}
                        </MenuItem>
                    ))}
                </Select>
            </FormControl>

            {/* Model Selection */}
            <FormControl sx={{ minWidth: 300, flex: 1 }} size="small">
                <InputLabel id="model-select-label" sx={labelStyles}>
                    Model
                </InputLabel>
                <Select
                    labelId="model-select-label"
                    id="model-select"
                    value={selectedModel}
                    label="Model"
                    onChange={(e) => onModelChange(e.target.value)}
                    disabled={disabled || loadingModels}
                    sx={selectStyles}
                    MenuProps={{
                        PaperProps: {
                            sx: {
                                bgcolor: isDark ? '#1E293B' : '#FFFFFF',
                                border: '1px solid',
                                borderColor: isDark ? '#334155' : '#E5E7EB',
                                borderRadius: 1,
                                boxShadow: isDark
                                    ? '0 10px 15px -3px rgba(0, 0, 0, 0.3)'
                                    : '0 10px 15px -3px rgba(0, 0, 0, 0.1)',
                                maxHeight: 300,
                            },
                        },
                    }}
                >
                    {models.map((model) => (
                        <MenuItem
                            key={model.name}
                            value={model.name}
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
                            {model.name}
                            {model.description && ` - ${model.description}`}
                        </MenuItem>
                    ))}
                </Select>
            </FormControl>

            {/* Preferences Button */}
            <Tooltip title="Preferences">
                <IconButton onClick={handlePreferencesClick} size="small" sx={iconButtonStyles}>
                    <SettingsIcon />
                </IconButton>
            </Tooltip>

            {/* Preferences Popover */}
            <PreferencesPopover
                anchorEl={preferencesAnchor}
                open={Boolean(preferencesAnchor)}
                onClose={handlePreferencesClose}
                showActivity={showActivity}
                onActivityChange={onActivityChange}
                renderMarkdown={renderMarkdown}
                onMarkdownChange={onMarkdownChange}
                debug={debug}
                onDebugChange={onDebugChange}
            />

            {/* Clear Button */}
            {hasMessages && (
                <Tooltip title="Clear Conversation">
                    <IconButton
                        onClick={onClear}
                        disabled={disabled}
                        size="small"
                        sx={{
                            ...iconButtonStyles,
                            '&:hover': {
                                bgcolor: isDark ? alpha('#EF4444', 0.08) : alpha('#EF4444', 0.04),
                                color: '#EF4444',
                            },
                        }}
                    >
                        <DeleteIcon />
                    </IconButton>
                </Tooltip>
            )}
        </Box>
    );
});

ProviderSelector.displayName = 'ProviderSelector';

ProviderSelector.propTypes = {
    providers: PropTypes.arrayOf(PropTypes.shape({
        name: PropTypes.string.isRequired,
        display: PropTypes.string.isRequired,
    })).isRequired,
    selectedProvider: PropTypes.string.isRequired,
    onProviderChange: PropTypes.func.isRequired,
    models: PropTypes.arrayOf(PropTypes.shape({
        name: PropTypes.string.isRequired,
        description: PropTypes.string,
    })).isRequired,
    selectedModel: PropTypes.string.isRequired,
    onModelChange: PropTypes.func.isRequired,
    showActivity: PropTypes.bool.isRequired,
    onActivityChange: PropTypes.func.isRequired,
    renderMarkdown: PropTypes.bool.isRequired,
    onMarkdownChange: PropTypes.func.isRequired,
    debug: PropTypes.bool.isRequired,
    onDebugChange: PropTypes.func.isRequired,
    disabled: PropTypes.bool.isRequired,
    loadingModels: PropTypes.bool.isRequired,
    onClear: PropTypes.func,
    hasMessages: PropTypes.bool,
};

export default ProviderSelector;
