/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Preferences Popover Component
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React from 'react';
import PropTypes from 'prop-types';
import {
    Popover,
    Box,
    Typography,
    FormControlLabel,
    Switch,
    Divider,
} from '@mui/material';

const PreferencesPopover = React.memo(({
    anchorEl,
    open,
    onClose,
    showActivity,
    onActivityChange,
    renderMarkdown,
    onMarkdownChange,
    debug,
    onDebugChange,
}) => {
    return (
        <Popover
            open={open}
            anchorEl={anchorEl}
            onClose={onClose}
            anchorOrigin={{
                vertical: 'top',
                horizontal: 'right',
            }}
            transformOrigin={{
                vertical: 'bottom',
                horizontal: 'right',
            }}
        >
            <Box sx={{ p: 2, minWidth: 250 }}>
                <Typography variant="h6" sx={{ mb: 2 }}>
                    Preferences
                </Typography>

                <Divider sx={{ mb: 2 }} />

                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                    <FormControlLabel
                        control={
                            <Switch
                                checked={showActivity}
                                onChange={(e) => onActivityChange(e.target.checked)}
                                size="small"
                            />
                        }
                        label="Show Activity"
                    />

                    <FormControlLabel
                        control={
                            <Switch
                                checked={renderMarkdown}
                                onChange={(e) => onMarkdownChange(e.target.checked)}
                                size="small"
                            />
                        }
                        label="Render Markdown"
                    />

                    <FormControlLabel
                        control={
                            <Switch
                                checked={debug}
                                onChange={(e) => onDebugChange(e.target.checked)}
                                size="small"
                            />
                        }
                        label="Debug Messages"
                    />
                </Box>
            </Box>
        </Popover>
    );
});

PreferencesPopover.displayName = 'PreferencesPopover';

PreferencesPopover.propTypes = {
    anchorEl: PropTypes.object,
    open: PropTypes.bool.isRequired,
    onClose: PropTypes.func.isRequired,
    showActivity: PropTypes.bool.isRequired,
    onActivityChange: PropTypes.func.isRequired,
    renderMarkdown: PropTypes.bool.isRequired,
    onMarkdownChange: PropTypes.func.isRequired,
    debug: PropTypes.bool.isRequired,
    onDebugChange: PropTypes.func.isRequired,
};

export default PreferencesPopover;
