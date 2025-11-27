/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Provider Selector Component
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
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

    const handlePreferencesClick = (event) => {
        setPreferencesAnchor(event.currentTarget);
    };

    const handlePreferencesClose = () => {
        setPreferencesAnchor(null);
    };

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

        {/* Preferences Button */}
        <Tooltip title="Preferences">
          <IconButton onClick={handlePreferencesClick} size="small">
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
            <IconButton onClick={onClear} disabled={disabled} size="small">
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
