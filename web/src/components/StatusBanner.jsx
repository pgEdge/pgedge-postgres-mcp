/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Status Banner
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Styled to match pgEdge Cloud product aesthetics
 *
 *-------------------------------------------------------------------------
 */

import React, { useState, useEffect } from 'react';
import {
    Box,
    Chip,
    CircularProgress,
    Typography,
    IconButton,
    Collapse,
    Paper,
    useTheme,
    Tooltip,
    alpha,
    Menu,
    MenuItem,
    ListItemIcon,
    ListItemText,
    Dialog,
    DialogTitle,
    DialogContent,
    DialogContentText,
    DialogActions,
    Button,
} from '@mui/material';
import {
    CheckCircle as CheckCircleIcon,
    Error as ErrorIcon,
    ExpandMore as ExpandMoreIcon,
    ExpandLess as ExpandLessIcon,
    Storage as StorageIcon,
    Warning as WarningIcon,
    MoreVert as MoreVertIcon,
    SaveAlt as SaveIcon,
    ClearAll as ClearIcon,
} from '@mui/icons-material';
import { useAuth } from '../contexts/AuthContext';
import { useLLMProcessing } from '../contexts/LLMProcessingContext';
import { useDatabaseContext } from '../contexts/DatabaseContext';
import { useConversationActions } from '../contexts/ConversationActionsContext';
import { MCPClient } from '../lib/mcp-client';
import DatabaseSelectorPopover from './DatabaseSelectorPopover';

const MCP_SERVER_URL = '/mcp/v1';

// Maximum retry attempts for transient database states
const MAX_RETRY_ATTEMPTS = 5;
const RETRY_DELAY_MS = 500;

const StatusBanner = () => {
    const { sessionToken, forceLogout } = useAuth();
    const { isProcessing } = useLLMProcessing();
    const { hasMessages, onSave, onClear } = useConversationActions();
    const theme = useTheme();
    const [systemInfo, setSystemInfo] = useState(null);
    const [expanded, setExpanded] = useState(false);
    const [error, setError] = useState('');
    const [dbPopoverAnchor, setDbPopoverAnchor] = useState(null);
    const [isSwitchingDatabase, setIsSwitchingDatabase] = useState(false);
    const [actionsMenuAnchor, setActionsMenuAnchor] = useState(null);
    const [clearDialogOpen, setClearDialogOpen] = useState(false);

    const isDark = theme.palette.mode === 'dark';

    // Conversation actions menu handlers
    const handleActionsMenuOpen = (event) => {
        setActionsMenuAnchor(event.currentTarget);
    };

    const handleActionsMenuClose = () => {
        setActionsMenuAnchor(null);
    };

    const handleSaveClick = () => {
        handleActionsMenuClose();
        onSave?.();
    };

    const handleClearClick = () => {
        handleActionsMenuClose();
        setClearDialogOpen(true);
    };

    const handleConfirmClear = () => {
        onClear?.();
        setClearDialogOpen(false);
    };

    const handleCancelClear = () => {
        setClearDialogOpen(false);
    };

    const dialogPaperProps = {
        sx: {
            bgcolor: isDark ? '#1E293B' : '#FFFFFF',
            border: '1px solid',
            borderColor: isDark ? '#334155' : '#E5E7EB',
            borderRadius: 1,
        },
    };

    // Database management (shared context)
    const {
        databases,
        currentDatabase,
        loading: dbLoading,
        error: dbError,
        fetchDatabases,
        selectDatabase,
    } = useDatabaseContext();

    useEffect(() => {
        if (sessionToken) {
            fetchSystemInfo();
            fetchDatabases();
            // Refresh every 30 seconds
            const interval = setInterval(fetchSystemInfo, 30000);
            return () => clearInterval(interval);
        }
    }, [sessionToken]);

    // Refresh system info when currentDatabase changes (e.g., from conversation restore)
    useEffect(() => {
        if (sessionToken && currentDatabase) {
            fetchSystemInfo();
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [currentDatabase]);

    // Handler for opening database selector
    const handleDbSelectorOpen = (event) => {
        setDbPopoverAnchor(event.currentTarget);
        fetchDatabases(); // Refresh list when opening
    };

    // Handler for selecting a database
    const handleDatabaseSelect = async (dbName) => {
        const success = await selectDatabase(dbName);
        if (success) {
            // Refresh system info to show updated connection
            fetchSystemInfo();
        }
    };

    const fetchSystemInfo = async (retryCount = 0) => {
        try {
            // Create MCP client with session token
            const client = new MCPClient(MCP_SERVER_URL, sessionToken);

            // Read the pg://system_info resource via JSON-RPC
            const resource = await client.readResource('pg://system_info');

            // Parse system info from resource content
            if (!resource.contents || resource.contents.length === 0) {
                throw new Error('No system information available');
            }

            const contentText = resource.contents[0].text;

            // Try to parse the response as JSON
            let info;
            try {
                info = JSON.parse(contentText);
            } catch (parseErr) {
                // Response is not valid JSON - could be an error message
                console.warn('System info response is not valid JSON:', contentText);

                // If we're in a retry scenario, keep trying
                if (retryCount < MAX_RETRY_ATTEMPTS) {
                    console.log(`Invalid JSON response, retrying in ${RETRY_DELAY_MS}ms (attempt ${retryCount + 1}/${MAX_RETRY_ATTEMPTS})`);
                    setIsSwitchingDatabase(true);
                    setTimeout(() => {
                        fetchSystemInfo(retryCount + 1);
                    }, RETRY_DELAY_MS);
                    return;
                }

                // Check if it's an error message we should handle specially
                if (contentText.toLowerCase().includes('error')) {
                    setError(contentText);
                    setIsSwitchingDatabase(false);
                    return;
                }

                throw parseErr;
            }

            // Check for JSON error response (e.g., DATABASE_NOT_READY)
            if (info.error === true) {
                const isRetryable = info.retryable === true;
                const isDatabaseNotReady = info.code === 'DATABASE_NOT_READY';

                if (isRetryable && retryCount < MAX_RETRY_ATTEMPTS) {
                    console.log(`${info.message || 'Retryable error'}, retrying in ${RETRY_DELAY_MS}ms (attempt ${retryCount + 1}/${MAX_RETRY_ATTEMPTS})`);
                    setIsSwitchingDatabase(isDatabaseNotReady);
                    setError(''); // Clear any previous error during switching
                    setTimeout(() => {
                        fetchSystemInfo(retryCount + 1);
                    }, RETRY_DELAY_MS);
                    return;
                } else if (isDatabaseNotReady) {
                    // Max retries exceeded for database switching
                    console.warn('Database switch taking longer than expected');
                    setIsSwitchingDatabase(false);
                    setError(info.message || 'Database switch in progress...');
                    return;
                } else {
                    // Non-retryable error
                    setError(info.message || 'An error occurred');
                    setIsSwitchingDatabase(false);
                    return;
                }
            }

            setSystemInfo(info);
            setError('');
            setIsSwitchingDatabase(false);
        } catch (err) {
            console.error('System info fetch error:', err);
            setIsSwitchingDatabase(false);

            // If this is a 401 error (session expired), log out
            if (err.message && (err.message.includes('401') || err.message.includes('Unauthorized'))) {
                console.log('Session invalidated during system info fetch, logging out...');
                setError(err.message || 'Failed to load system information');
                forceLogout();
                return;
            }

            // If this is a network error (server disconnected), log out and show message
            if (err.message && (err.message.includes('fetch') || err.message.includes('Failed to fetch'))) {
                console.log('Server appears to be disconnected, logging out...');
                sessionStorage.setItem('disconnectMessage', 'Your session was ended because the server disconnected. Please try again.');
                setError(err.message || 'Failed to load system information');
                forceLogout();
                return;
            }

            // For other errors, just set the error state without logging out
            setError(err.message || 'Failed to load system information');
        }
    };

    const connected = systemInfo && !error && !isSwitchingDatabase;

    // Format connection string for display
    const getConnectionString = () => {
        if (!systemInfo) return '';
        const { user, host, port, database } = systemInfo;
        const portStr = port && port !== 0 ? `:${port}` : '';
        return `${user}@${host}${portStr}/${database}`;
    };

    return (
        <Paper
            elevation={0}
            sx={{
                mb: 2,
                borderRadius: 1,
                overflow: 'hidden',
                border: '1px solid',
                borderColor: isDark ? '#334155' : '#E5E7EB',
            }}
        >
            {/* Write Access Warning Banner */}
            {systemInfo?.allow_writes && (
                <Box
                    sx={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 1,
                        px: 2,
                        py: 1,
                        bgcolor: isDark ? alpha('#F59E0B', 0.2) : alpha('#F59E0B', 0.15),
                        borderBottom: '1px solid',
                        borderColor: isDark ? alpha('#F59E0B', 0.4) : alpha('#F59E0B', 0.3),
                    }}
                >
                    <WarningIcon
                        sx={{
                            fontSize: 18,
                            color: '#F59E0B',
                        }}
                    />
                    <Typography
                        variant="body2"
                        sx={{
                            fontWeight: 600,
                            color: isDark ? '#FCD34D' : '#B45309',
                        }}
                    >
                        Write Access Enabled
                    </Typography>
                    <Typography
                        variant="body2"
                        sx={{
                            color: isDark ? '#FDE68A' : '#92400E',
                        }}
                    >
                        — The AI can modify data in this database
                    </Typography>
                </Box>
            )}

            <Box
                sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    px: 2,
                    py: 1.25,
                    bgcolor: isSwitchingDatabase
                        ? (isDark ? alpha('#F59E0B', 0.15) : alpha('#F59E0B', 0.1))
                        : connected
                            ? (isDark ? alpha('#22C55E', 0.15) : alpha('#22C55E', 0.1))
                            : (isDark ? alpha('#EF4444', 0.15) : alpha('#EF4444', 0.1)),
                    borderBottom: expanded ? '1px solid' : 'none',
                    borderColor: isDark ? '#334155' : '#E5E7EB',
                }}
            >
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        {isSwitchingDatabase ? (
                            <CircularProgress
                                size={16}
                                thickness={4}
                                sx={{
                                    color: '#F59E0B',
                                }}
                            />
                        ) : connected ? (
                            <CheckCircleIcon
                                sx={{
                                    fontSize: 18,
                                    color: '#22C55E',
                                }}
                            />
                        ) : (
                            <ErrorIcon
                                sx={{
                                    fontSize: 18,
                                    color: '#EF4444',
                                }}
                            />
                        )}
                        <Typography
                            variant="body2"
                            sx={{
                                fontWeight: 600,
                                color: isSwitchingDatabase
                                    ? (isDark ? '#FCD34D' : '#B45309')
                                    : connected
                                        ? (isDark ? '#4ADE80' : '#16A34A')
                                        : (isDark ? '#F87171' : '#DC2626'),
                            }}
                        >
                            {isSwitchingDatabase ? 'Switching Database...' : connected ? 'Connected' : 'Disconnected'}
                        </Typography>
                    </Box>
                    {connected && systemInfo && (
                        <>
                            <Chip
                                label={`PostgreSQL ${systemInfo.postgresql_version}`}
                                size="small"
                                sx={{
                                    display: { xs: 'none', sm: 'flex' },
                                    height: 24,
                                    bgcolor: 'transparent',
                                    color: isDark ? '#94A3B8' : '#6B7280',
                                    fontSize: '0.75rem',
                                    fontWeight: 500,
                                    '& .MuiChip-label': {
                                        px: 1.5,
                                    },
                                }}
                            />
                            <Typography
                                variant="body2"
                                sx={{
                                    display: { xs: 'none', md: 'block' },
                                    fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                                    fontSize: '0.8rem',
                                    color: isDark ? '#94A3B8' : '#6B7280',
                                }}
                            >
                                {getConnectionString()}
                            </Typography>
                        </>
                    )}
                    {error && (
                        <Typography
                            variant="body2"
                            sx={{
                                color: isDark ? '#F87171' : '#DC2626',
                            }}
                        >
                            {error}
                        </Typography>
                    )}
                </Box>
                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    {connected && databases.length > 1 && (
                        <Tooltip title={isProcessing ? "Cannot change database while processing" : "Select database"}>
                            <span>
                                <IconButton
                                    size="small"
                                    onClick={handleDbSelectorOpen}
                                    disabled={isProcessing}
                                    sx={{
                                        color: isDark ? '#94A3B8' : '#6B7280',
                                        mr: 0.5,
                                        '&:hover': {
                                            bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                            color: '#15AABF',
                                        },
                                        '&.Mui-disabled': {
                                            color: isDark ? '#475569' : '#D1D5DB',
                                        },
                                    }}
                                >
                                    <StorageIcon fontSize="small" />
                                </IconButton>
                            </span>
                        </Tooltip>
                    )}
                    <Tooltip title="Conversation actions">
                        <IconButton
                            size="small"
                            onClick={handleActionsMenuOpen}
                            aria-label="Conversation actions"
                            sx={{
                                color: isDark ? '#94A3B8' : '#6B7280',
                                mr: 0.5,
                                '&:hover': {
                                    bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                    color: '#15AABF',
                                },
                            }}
                        >
                            <MoreVertIcon fontSize="small" />
                        </IconButton>
                    </Tooltip>
                    <Menu
                        anchorEl={actionsMenuAnchor}
                        open={Boolean(actionsMenuAnchor)}
                        onClose={handleActionsMenuClose}
                        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
                        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
                        slotProps={{
                            paper: {
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
                        <MenuItem
                            onClick={handleSaveClick}
                            disabled={!hasMessages || !onSave}
                            sx={{
                                color: isDark ? '#F1F5F9' : '#1F2937',
                                '&:hover': {
                                    bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                },
                            }}
                        >
                            <ListItemIcon sx={{ color: 'inherit', minWidth: 36 }}>
                                <SaveIcon fontSize="small" />
                            </ListItemIcon>
                            <ListItemText primary="Save conversation" />
                        </MenuItem>
                        <MenuItem
                            onClick={handleClearClick}
                            disabled={!hasMessages || !onClear}
                            sx={{
                                color: isDark ? '#F1F5F9' : '#1F2937',
                                '&:hover': {
                                    bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                },
                            }}
                        >
                            <ListItemIcon sx={{ color: 'inherit', minWidth: 36 }}>
                                <ClearIcon fontSize="small" />
                            </ListItemIcon>
                            <ListItemText primary="Clear conversation" />
                        </MenuItem>
                    </Menu>
                    <IconButton
                        size="small"
                        onClick={() => setExpanded(!expanded)}
                        sx={{
                            color: isDark ? '#94A3B8' : '#6B7280',
                            '&:hover': {
                                bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                color: '#15AABF',
                            },
                        }}
                    >
                        {expanded ? <ExpandLessIcon /> : <ExpandMoreIcon />}
                    </IconButton>
                </Box>
            </Box>

            <Collapse in={expanded}>
                <Box sx={{ p: 2.5, bgcolor: 'background.paper' }}>
                    {connected && systemInfo ? (
                        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 2.5 }}>
                            <Box>
                                <Typography
                                    variant="caption"
                                    sx={{
                                        color: isDark ? '#64748B' : '#9CA3AF',
                                        textTransform: 'uppercase',
                                        letterSpacing: '0.05em',
                                        fontWeight: 600,
                                        fontSize: '0.65rem',
                                    }}
                                >
                                    Database
                                </Typography>
                                <Typography
                                    variant="body2"
                                    sx={{
                                        fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                                        color: isDark ? '#F1F5F9' : '#1F2937',
                                        mt: 0.25,
                                    }}
                                >
                                    {systemInfo.database || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography
                                    variant="caption"
                                    sx={{
                                        color: isDark ? '#64748B' : '#9CA3AF',
                                        textTransform: 'uppercase',
                                        letterSpacing: '0.05em',
                                        fontWeight: 600,
                                        fontSize: '0.65rem',
                                    }}
                                >
                                    User
                                </Typography>
                                <Typography
                                    variant="body2"
                                    sx={{
                                        fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                                        color: isDark ? '#F1F5F9' : '#1F2937',
                                        mt: 0.25,
                                    }}
                                >
                                    {systemInfo.user || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography
                                    variant="caption"
                                    sx={{
                                        color: isDark ? '#64748B' : '#9CA3AF',
                                        textTransform: 'uppercase',
                                        letterSpacing: '0.05em',
                                        fontWeight: 600,
                                        fontSize: '0.65rem',
                                    }}
                                >
                                    Host
                                </Typography>
                                <Typography
                                    variant="body2"
                                    sx={{
                                        fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                                        color: isDark ? '#F1F5F9' : '#1F2937',
                                        mt: 0.25,
                                    }}
                                >
                                    {systemInfo.host || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography
                                    variant="caption"
                                    sx={{
                                        color: isDark ? '#64748B' : '#9CA3AF',
                                        textTransform: 'uppercase',
                                        letterSpacing: '0.05em',
                                        fontWeight: 600,
                                        fontSize: '0.65rem',
                                    }}
                                >
                                    Port
                                </Typography>
                                <Typography
                                    variant="body2"
                                    sx={{
                                        fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                                        color: isDark ? '#F1F5F9' : '#1F2937',
                                        mt: 0.25,
                                    }}
                                >
                                    {systemInfo.port && systemInfo.port !== 0 ? systemInfo.port : 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography
                                    variant="caption"
                                    sx={{
                                        color: isDark ? '#64748B' : '#9CA3AF',
                                        textTransform: 'uppercase',
                                        letterSpacing: '0.05em',
                                        fontWeight: 600,
                                        fontSize: '0.65rem',
                                    }}
                                >
                                    PostgreSQL Version
                                </Typography>
                                <Typography
                                    variant="body2"
                                    sx={{
                                        color: isDark ? '#F1F5F9' : '#1F2937',
                                        mt: 0.25,
                                    }}
                                >
                                    {systemInfo.postgresql_version || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography
                                    variant="caption"
                                    sx={{
                                        color: isDark ? '#64748B' : '#9CA3AF',
                                        textTransform: 'uppercase',
                                        letterSpacing: '0.05em',
                                        fontWeight: 600,
                                        fontSize: '0.65rem',
                                    }}
                                >
                                    Operating System
                                </Typography>
                                <Typography
                                    variant="body2"
                                    sx={{
                                        color: isDark ? '#F1F5F9' : '#1F2937',
                                        mt: 0.25,
                                    }}
                                >
                                    {systemInfo.operating_system || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography
                                    variant="caption"
                                    sx={{
                                        color: isDark ? '#64748B' : '#9CA3AF',
                                        textTransform: 'uppercase',
                                        letterSpacing: '0.05em',
                                        fontWeight: 600,
                                        fontSize: '0.65rem',
                                    }}
                                >
                                    Architecture
                                </Typography>
                                <Typography
                                    variant="body2"
                                    sx={{
                                        color: isDark ? '#F1F5F9' : '#1F2937',
                                        mt: 0.25,
                                    }}
                                >
                                    {systemInfo.architecture || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography
                                    variant="caption"
                                    sx={{
                                        color: isDark ? '#64748B' : '#9CA3AF',
                                        textTransform: 'uppercase',
                                        letterSpacing: '0.05em',
                                        fontWeight: 600,
                                        fontSize: '0.65rem',
                                    }}
                                >
                                    Bit Version
                                </Typography>
                                <Typography
                                    variant="body2"
                                    sx={{
                                        color: isDark ? '#F1F5F9' : '#1F2937',
                                        mt: 0.25,
                                    }}
                                >
                                    {systemInfo.bit_version || 'N/A'}
                                </Typography>
                            </Box>
                            {systemInfo.compiler && (
                                <Box>
                                    <Typography
                                        variant="caption"
                                        sx={{
                                            color: isDark ? '#64748B' : '#9CA3AF',
                                            textTransform: 'uppercase',
                                            letterSpacing: '0.05em',
                                            fontWeight: 600,
                                            fontSize: '0.65rem',
                                        }}
                                    >
                                        Compiler
                                    </Typography>
                                    <Typography
                                        variant="body2"
                                        sx={{
                                            color: isDark ? '#F1F5F9' : '#1F2937',
                                            mt: 0.25,
                                        }}
                                    >
                                        {systemInfo.compiler}
                                    </Typography>
                                </Box>
                            )}
                            {systemInfo.full_version && (
                                <Box sx={{ gridColumn: { xs: '1', md: '1 / -1' } }}>
                                    <Typography
                                        variant="caption"
                                        sx={{
                                            color: isDark ? '#64748B' : '#9CA3AF',
                                            textTransform: 'uppercase',
                                            letterSpacing: '0.05em',
                                            fontWeight: 600,
                                            fontSize: '0.65rem',
                                        }}
                                    >
                                        Full Version
                                    </Typography>
                                    <Typography
                                        variant="body2"
                                        sx={{
                                            fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                                            fontSize: '0.7rem',
                                            color: isDark ? '#94A3B8' : '#6B7280',
                                            mt: 0.5,
                                            p: 1.5,
                                            bgcolor: isDark ? '#0F172A' : '#F9FAFB',
                                            borderRadius: 1,
                                            border: '1px solid',
                                            borderColor: isDark ? '#334155' : '#E5E7EB',
                                        }}
                                    >
                                        {systemInfo.full_version}
                                    </Typography>
                                </Box>
                            )}
                        </Box>
                    ) : (
                        <Typography variant="body2" sx={{ color: isDark ? '#64748B' : '#9CA3AF' }}>
                            Unable to load system information
                        </Typography>
                    )}
                </Box>
            </Collapse>

            {/* Database Selector Popover */}
            <DatabaseSelectorPopover
                anchorEl={dbPopoverAnchor}
                open={Boolean(dbPopoverAnchor)}
                onClose={() => setDbPopoverAnchor(null)}
                databases={databases}
                currentDatabase={currentDatabase || systemInfo?.database}
                onSelect={handleDatabaseSelect}
                loading={dbLoading}
                error={dbError}
            />

            {/* Clear Conversation Confirmation Dialog */}
            <Dialog
                open={clearDialogOpen}
                onClose={handleCancelClear}
                PaperProps={dialogPaperProps}
            >
                <DialogTitle sx={{ color: isDark ? '#F1F5F9' : '#1F2937' }}>
                    Clear conversation
                </DialogTitle>
                <DialogContent>
                    <DialogContentText sx={{ color: isDark ? '#94A3B8' : '#6B7280' }}>
                        This clears the chat window and starts a new
                        conversation. The current conversation stays in the
                        conversation list, where you can reopen or delete it.
                    </DialogContentText>
                </DialogContent>
                <DialogActions sx={{ p: 2, pt: 0 }}>
                    <Button
                        onClick={handleCancelClear}
                        sx={{
                            color: isDark ? '#94A3B8' : '#6B7280',
                            textTransform: 'none',
                        }}
                    >
                        Cancel
                    </Button>
                    <Button
                        onClick={handleConfirmClear}
                        variant="contained"
                        sx={{
                            bgcolor: '#15AABF',
                            color: '#FFFFFF',
                            textTransform: 'none',
                            '&:hover': {
                                bgcolor: '#0C8599',
                            },
                        }}
                    >
                        Clear
                    </Button>
                </DialogActions>
            </Dialog>
        </Paper>
    );
};

export default StatusBanner;
