/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Preferences Popover Component
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Styled to match pgEdge Cloud product aesthetics
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
    useTheme,
    alpha,
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
    const theme = useTheme();
    const isDark = theme.palette.mode === 'dark';

    const switchStyles = {
        '& .MuiSwitch-switchBase': {
            '&.Mui-checked': {
                color: '#FFFFFF',
                '& + .MuiSwitch-track': {
                    backgroundColor: '#15AABF',
                    opacity: 1,
                },
            },
        },
        '& .MuiSwitch-track': {
            backgroundColor: isDark ? '#475569' : '#D1D5DB',
            opacity: 1,
        },
    };

    return (
        <Popover
            open={open}
            anchorEl={anchorEl}
            onClose={onClose}
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
            <Box sx={{ p: 2.5, minWidth: 250 }}>
                <Typography
                    variant="h6"
                    sx={{
                        mb: 2,
                        color: isDark ? '#F1F5F9' : '#1F2937',
                        fontWeight: 600,
                        fontSize: '1.125rem',
                    }}
                >
                    Preferences
                </Typography>

                <Divider sx={{ mb: 2, borderColor: isDark ? '#334155' : '#E5E7EB' }} />

                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
                    <FormControlLabel
                        control={
                            <Switch
                                checked={showActivity}
                                onChange={(e) => onActivityChange(e.target.checked)}
                                size="small"
                                sx={switchStyles}
                            />
                        }
                        label={
                            <Typography
                                variant="body2"
                                sx={{ color: isDark ? '#F1F5F9' : '#374151' }}
                            >
                                Show Activity
                            </Typography>
                        }
                        sx={{ mx: 0 }}
                    />

                    <FormControlLabel
                        control={
                            <Switch
                                checked={renderMarkdown}
                                onChange={(e) => onMarkdownChange(e.target.checked)}
                                size="small"
                                sx={switchStyles}
                            />
                        }
                        label={
                            <Typography
                                variant="body2"
                                sx={{ color: isDark ? '#F1F5F9' : '#374151' }}
                            >
                                Render Markdown
                            </Typography>
                        }
                        sx={{ mx: 0 }}
                    />

                    <FormControlLabel
                        control={
                            <Switch
                                checked={debug}
                                onChange={(e) => onDebugChange(e.target.checked)}
                                size="small"
                                sx={switchStyles}
                            />
                        }
                        label={
                            <Typography
                                variant="body2"
                                sx={{ color: isDark ? '#F1F5F9' : '#374151' }}
                            >
                                Debug Messages
                            </Typography>
                        }
                        sx={{ mx: 0 }}
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
