/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Status Banner
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
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
    Typography,
    IconButton,
    Collapse,
    Paper,
    useTheme,
    Tooltip,
    alpha,
} from '@mui/material';
import {
    CheckCircle as CheckCircleIcon,
    Error as ErrorIcon,
    ExpandMore as ExpandMoreIcon,
    ExpandLess as ExpandLessIcon,
    Storage as StorageIcon,
    Warning as WarningIcon,
} from '@mui/icons-material';
import { useAuth } from '../contexts/AuthContext';
import { useLLMProcessing } from '../contexts/LLMProcessingContext';
import { useDatabaseContext } from '../contexts/DatabaseContext';
import { MCPClient } from '../lib/mcp-client';
import DatabaseSelectorPopover from './DatabaseSelectorPopover';

const MCP_SERVER_URL = '/mcp/v1';

const StatusBanner = () => {
    const { sessionToken, forceLogout } = useAuth();
    const { isProcessing } = useLLMProcessing();
    const theme = useTheme();
    const [systemInfo, setSystemInfo] = useState(null);
    const [expanded, setExpanded] = useState(false);
    const [error, setError] = useState('');
    const [dbPopoverAnchor, setDbPopoverAnchor] = useState(null);

    const isDark = theme.palette.mode === 'dark';

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

    const fetchSystemInfo = async () => {
        try {
            // Create MCP client with session token
            const client = new MCPClient(MCP_SERVER_URL, sessionToken);

            // Read the pg://system_info resource via JSON-RPC
            const resource = await client.readResource('pg://system_info');

            // Parse system info from resource content
            if (!resource.contents || resource.contents.length === 0) {
                throw new Error('No system information available');
            }

            const info = JSON.parse(resource.contents[0].text);
            setSystemInfo(info);
            setError('');
        } catch (err) {
            console.error('System info fetch error:', err);
            setError(err.message || 'Failed to load system information');

            // If this is a 401 error (session expired), log out
            if (err.message.includes('401') || err.message.includes('Unauthorized')) {
                console.log('Session invalidated during system info fetch, logging out...');
                forceLogout();
            }

            // If this is a network error (server disconnected), log out and show message
            if (err.message.includes('fetch') || err.message.includes('Failed to fetch')) {
                console.log('Server appears to be disconnected, logging out...');
                sessionStorage.setItem('disconnectMessage', 'Your session was ended because the server disconnected. Please try again.');
                forceLogout();
            }
        }
    };

    const connected = systemInfo && !error;

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
                    bgcolor: connected
                        ? (isDark ? alpha('#22C55E', 0.15) : alpha('#22C55E', 0.1))
                        : (isDark ? alpha('#EF4444', 0.15) : alpha('#EF4444', 0.1)),
                    borderBottom: expanded ? '1px solid' : 'none',
                    borderColor: isDark ? '#334155' : '#E5E7EB',
                }}
            >
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        {connected ? (
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
                                color: connected
                                    ? (isDark ? '#4ADE80' : '#16A34A')
                                    : (isDark ? '#F87171' : '#DC2626'),
                            }}
                        >
                            {connected ? 'Connected' : 'Disconnected'}
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
        </Paper>
    );
};

export default StatusBanner;
