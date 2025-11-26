/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Database Selector Popover Component
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
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
    List,
    ListItemButton,
    ListItemText,
    ListItemIcon,
    CircularProgress,
    Alert,
    Divider,
} from '@mui/material';
import {
    Storage as StorageIcon,
    CheckCircle as CheckCircleIcon,
} from '@mui/icons-material';

const DatabaseSelectorPopover = ({
    anchorEl,
    open,
    onClose,
    databases = [],
    currentDatabase,
    onSelect,
    loading = false,
    error = null,
}) => {
    const handleSelect = (dbName) => {
        if (dbName !== currentDatabase) {
            onSelect(dbName);
        }
        onClose();
    };

    return (
        <Popover
            open={open}
            anchorEl={anchorEl}
            onClose={onClose}
            disableScrollLock
            anchorOrigin={{
                vertical: 'bottom',
                horizontal: 'right',
            }}
            transformOrigin={{
                vertical: 'top',
                horizontal: 'right',
            }}
        >
            <Box sx={{ minWidth: 300, maxWidth: 450 }}>
                <Box sx={{ p: 2, pb: 1 }}>
                    <Typography variant="h6" sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <StorageIcon fontSize="small" />
                        Select Database
                    </Typography>
                </Box>

                <Divider />

                {loading && (
                    <Box sx={{ display: 'flex', justifyContent: 'center', p: 3 }}>
                        <CircularProgress size={24} />
                    </Box>
                )}

                {error && (
                    <Alert severity="error" sx={{ m: 2 }}>
                        {error}
                    </Alert>
                )}

                {!loading && !error && databases.length === 0 && (
                    <Box sx={{ p: 2 }}>
                        <Typography variant="body2" color="text.secondary">
                            No databases available
                        </Typography>
                    </Box>
                )}

                {!loading && databases.length > 0 && (
                    <List sx={{ py: 0 }}>
                        {databases.map((db) => {
                            const isCurrent = db.name === currentDatabase;
                            return (
                                <ListItemButton
                                    key={db.name}
                                    onClick={() => handleSelect(db.name)}
                                    selected={isCurrent}
                                    sx={{
                                        '&.Mui-selected': {
                                            bgcolor: 'action.selected',
                                        },
                                    }}
                                >
                                    <ListItemIcon sx={{ minWidth: 36 }}>
                                        {isCurrent ? (
                                            <CheckCircleIcon color="success" fontSize="small" />
                                        ) : (
                                            <StorageIcon fontSize="small" color="action" />
                                        )}
                                    </ListItemIcon>
                                    <ListItemText
                                        primary={
                                            <Typography
                                                variant="body1"
                                                sx={{
                                                    fontWeight: isCurrent ? 600 : 400,
                                                }}
                                            >
                                                {db.name}
                                            </Typography>
                                        }
                                        secondary={
                                            <Typography
                                                variant="caption"
                                                color="text.secondary"
                                                sx={{ fontFamily: 'monospace' }}
                                            >
                                                {db.user}@{db.host}:{db.port}/{db.database}
                                            </Typography>
                                        }
                                    />
                                </ListItemButton>
                            );
                        })}
                    </List>
                )}

                {currentDatabase && (
                    <>
                        <Divider />
                        <Box sx={{ p: 1.5, bgcolor: 'action.hover' }}>
                            <Typography variant="caption" color="text.secondary">
                                Current: <strong>{currentDatabase}</strong>
                            </Typography>
                        </Box>
                    </>
                )}
            </Box>
        </Popover>
    );
};

DatabaseSelectorPopover.propTypes = {
    anchorEl: PropTypes.object,
    open: PropTypes.bool.isRequired,
    onClose: PropTypes.func.isRequired,
    databases: PropTypes.arrayOf(PropTypes.shape({
        name: PropTypes.string.isRequired,
        host: PropTypes.string,
        port: PropTypes.number,
        database: PropTypes.string,
        user: PropTypes.string,
        sslmode: PropTypes.string,
    })),
    currentDatabase: PropTypes.string,
    onSelect: PropTypes.func.isRequired,
    loading: PropTypes.bool,
    error: PropTypes.string,
};

export default DatabaseSelectorPopover;
