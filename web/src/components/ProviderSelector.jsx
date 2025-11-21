/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Provider Selector Component
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React from 'react';
import PropTypes from 'prop-types';
import {
    Box,
    FormControl,
    InputLabel,
    Select,
    MenuItem,
    FormControlLabel,
    Switch,
} from '@mui/material';

const ProviderSelector = ({
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
    disabled,
    loadingModels,
}) => {
    return (
        <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
            {/* Provider Selection */}
            <FormControl sx={{ minWidth: 200 }} size="small">
                <InputLabel id="provider-select-label">Provider</InputLabel>
                <Select
                    labelId="provider-select-label"
                    id="provider-select"
                    value={selectedProvider}
                    label="Provider"
                    onChange={(e) => onProviderChange(e.target.value)}
                    disabled={disabled}
                >
                    {providers.map((provider) => (
                        <MenuItem key={provider.name} value={provider.name}>
                            {provider.display}
                        </MenuItem>
                    ))}
                </Select>
            </FormControl>

            {/* Model Selection */}
            <FormControl sx={{ minWidth: 300, flex: 1 }} size="small">
                <InputLabel id="model-select-label">Model</InputLabel>
                <Select
                    labelId="model-select-label"
                    id="model-select"
                    value={selectedModel}
                    label="Model"
                    onChange={(e) => onModelChange(e.target.value)}
                    disabled={disabled || loadingModels}
                >
                    {models.map((model) => (
                        <MenuItem key={model.name} value={model.name}>
                            {model.name}
                            {model.description && ` - ${model.description}`}
                        </MenuItem>
                    ))}
                </Select>
            </FormControl>

            {/* Show Activity Toggle */}
            <FormControlLabel
                control={
                    <Switch
                        checked={showActivity}
                        onChange={(e) => onActivityChange(e.target.checked)}
                        size="small"
                    />
                }
                label="Show Activity"
                sx={{ ml: 1, whiteSpace: 'nowrap' }}
            />

            {/* Render Markdown Toggle */}
            <FormControlLabel
                control={
                    <Switch
                        checked={renderMarkdown}
                        onChange={(e) => onMarkdownChange(e.target.checked)}
                        size="small"
                    />
                }
                label="Render Markdown"
                sx={{ ml: 1, whiteSpace: 'nowrap' }}
            />
        </Box>
    );
};

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
    disabled: PropTypes.bool.isRequired,
    loadingModels: PropTypes.bool.isRequired,
};

export default ProviderSelector;
