/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Prompt Selector Component
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
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
    Tooltip,
    Chip,
} from '@mui/material';
import { Psychology as PsychologyIcon } from '@mui/icons-material';

const PromptSelector = React.memo(({
    prompts,
    onPromptSelect,
    disabled = false,
}) => {
    const [selectedPromptName, setSelectedPromptName] = useState('');

    const handlePromptChange = (event) => {
        const promptName = event.target.value;
        setSelectedPromptName(promptName);

        // Find the full prompt object and pass it to the handler
        const prompt = prompts.find(p => p.name === promptName);
        if (prompt) {
            onPromptSelect(prompt);
            // Reset selection after triggering
            setSelectedPromptName('');
        }
    };

    if (!prompts || prompts.length === 0) {
        return null;
    }

    return (
        <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
            <PsychologyIcon sx={{ color: 'text.secondary' }} />
            <FormControl sx={{ minWidth: 250 }} size="small">
                <InputLabel id="prompt-select-label">Execute Prompt</InputLabel>
                <Select
                    labelId="prompt-select-label"
                    id="prompt-select"
                    value={selectedPromptName}
                    label="Execute Prompt"
                    onChange={handlePromptChange}
                    disabled={disabled}
                    displayEmpty
                >
                    <MenuItem value="" disabled>
                        <em>Select a prompt to execute...</em>
                    </MenuItem>
                    {[...prompts].sort((a, b) => a.name.localeCompare(b.name)).map((prompt) => (
                        <MenuItem key={prompt.name} value={prompt.name}>
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                <span>{prompt.name}</span>
                                {prompt.arguments && prompt.arguments.length > 0 && (
                                    <Chip
                                        label={`${prompt.arguments.length} arg${prompt.arguments.length > 1 ? 's' : ''}`}
                                        size="small"
                                        variant="outlined"
                                        sx={{ height: 20, fontSize: '0.7rem' }}
                                    />
                                )}
                            </Box>
                        </MenuItem>
                    ))}
                </Select>
            </FormControl>
            <Tooltip title={`${prompts.length} prompt${prompts.length !== 1 ? 's' : ''} available`}>
                <Chip
                    label={prompts.length}
                    size="small"
                    color="primary"
                    variant="outlined"
                />
            </Tooltip>
        </Box>
    );
});

PromptSelector.displayName = 'PromptSelector';

PromptSelector.propTypes = {
    prompts: PropTypes.arrayOf(PropTypes.shape({
        name: PropTypes.string.isRequired,
        description: PropTypes.string,
        arguments: PropTypes.arrayOf(PropTypes.shape({
            name: PropTypes.string.isRequired,
            description: PropTypes.string,
            required: PropTypes.bool,
        })),
    })).isRequired,
    onPromptSelect: PropTypes.func.isRequired,
    disabled: PropTypes.bool,
};

export default PromptSelector;
