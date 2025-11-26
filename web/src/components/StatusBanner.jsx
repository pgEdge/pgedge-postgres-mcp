/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Status Banner
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
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
} from '@mui/material';
import {
    CheckCircle as CheckCircleIcon,
    Error as ErrorIcon,
    ExpandMore as ExpandMoreIcon,
    ExpandLess as ExpandLessIcon,
    Storage as StorageIcon,
} from '@mui/icons-material';
import { useAuth } from '../contexts/AuthContext';
import { useLLMProcessing } from '../contexts/LLMProcessingContext';
import { MCPClient } from '../lib/mcp-client';
import { useDatabases } from '../hooks/useDatabases';
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

    // Database management
    const {
        databases,
        currentDatabase,
        loading: dbLoading,
        error: dbError,
        fetchDatabases,
        selectDatabase,
    } = useDatabases(sessionToken);

    useEffect(() => {
        if (sessionToken) {
            fetchSystemInfo();
            fetchDatabases();
            // Refresh every 30 seconds
            const interval = setInterval(fetchSystemInfo, 30000);
            return () => clearInterval(interval);
        }
    }, [sessionToken]);

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
            elevation={1}
            sx={{
                mb: 2,
                borderRadius: 1,
                overflow: 'hidden',
            }}
        >
            <Box
                sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    px: 2,
                    py: 1,
                    bgcolor: connected
                        ? (theme.palette.mode === 'dark' ? '#1b5e20' : 'success.main')
                        : 'error.main',
                    color: 'white',
                }}
            >
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
                    <Chip
                        icon={connected ? <CheckCircleIcon /> : <ErrorIcon />}
                        label={connected ? 'Connected' : 'Disconnected'}
                        size="small"
                        sx={{
                            bgcolor: 'rgba(255, 255, 255, 0.2)',
                            color: 'white',
                            '& .MuiChip-icon': { color: 'white' },
                        }}
                    />
                    {connected && systemInfo && (
                        <>
                            <Typography variant="body2" sx={{ display: { xs: 'none', sm: 'block' } }}>
                                PostgreSQL {systemInfo.postgresql_version}
                            </Typography>
                            <Typography variant="body2" sx={{ display: { xs: 'none', md: 'block' }, fontFamily: 'monospace', fontSize: '0.85rem' }}>
                                {getConnectionString()}
                            </Typography>
                        </>
                    )}
                    {error && (
                        <Typography variant="body2">
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
                                        color: 'white',
                                        mr: 1,
                                        '&.Mui-disabled': {
                                            color: 'rgba(255, 255, 255, 0.4)',
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
                        sx={{ color: 'white' }}
                    >
                        {expanded ? <ExpandLessIcon /> : <ExpandMoreIcon />}
                    </IconButton>
                </Box>
            </Box>

            <Collapse in={expanded}>
                <Box sx={{ p: 2, bgcolor: 'background.paper' }}>
                    {connected && systemInfo ? (
                        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: '1fr 1fr 1fr' }, gap: 2 }}>
                            <Box>
                                <Typography variant="caption" color="text.secondary">
                                    Database
                                </Typography>
                                <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                                    {systemInfo.database || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography variant="caption" color="text.secondary">
                                    User
                                </Typography>
                                <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                                    {systemInfo.user || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography variant="caption" color="text.secondary">
                                    Host
                                </Typography>
                                <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                                    {systemInfo.host || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography variant="caption" color="text.secondary">
                                    Port
                                </Typography>
                                <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                                    {systemInfo.port && systemInfo.port !== 0 ? systemInfo.port : 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography variant="caption" color="text.secondary">
                                    PostgreSQL Version
                                </Typography>
                                <Typography variant="body2">
                                    {systemInfo.postgresql_version || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography variant="caption" color="text.secondary">
                                    Operating System
                                </Typography>
                                <Typography variant="body2">
                                    {systemInfo.operating_system || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography variant="caption" color="text.secondary">
                                    Architecture
                                </Typography>
                                <Typography variant="body2">
                                    {systemInfo.architecture || 'N/A'}
                                </Typography>
                            </Box>
                            <Box>
                                <Typography variant="caption" color="text.secondary">
                                    Bit Version
                                </Typography>
                                <Typography variant="body2">
                                    {systemInfo.bit_version || 'N/A'}
                                </Typography>
                            </Box>
                            {systemInfo.compiler && (
                                <Box>
                                    <Typography variant="caption" color="text.secondary">
                                        Compiler
                                    </Typography>
                                    <Typography variant="body2">
                                        {systemInfo.compiler}
                                    </Typography>
                                </Box>
                            )}
                            {systemInfo.full_version && (
                                <Box sx={{ gridColumn: { xs: '1', md: '1 / -1' } }}>
                                    <Typography variant="caption" color="text.secondary">
                                        Full Version
                                    </Typography>
                                    <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>
                                        {systemInfo.full_version}
                                    </Typography>
                                </Box>
                            )}
                        </Box>
                    ) : (
                        <Typography variant="body2" color="text.secondary">
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
