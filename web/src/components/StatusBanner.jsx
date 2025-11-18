/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Status Banner
 *
 * Copyright (c) 2025, pgEdge, Inc.
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
} from '@mui/material';
import {
    CheckCircle as CheckCircleIcon,
    Error as ErrorIcon,
    ExpandMore as ExpandMoreIcon,
    ExpandLess as ExpandLessIcon,
} from '@mui/icons-material';
import { useAuth } from '../contexts/AuthContext';

const StatusBanner = () => {
    const { forceLogout } = useAuth();
    const [systemInfo, setSystemInfo] = useState(null);
    const [expanded, setExpanded] = useState(false);
    const [error, setError] = useState('');

    useEffect(() => {
        fetchSystemInfo();
        // Refresh every 30 seconds
        const interval = setInterval(fetchSystemInfo, 30000);
        return () => clearInterval(interval);
    }, []);

    const fetchSystemInfo = async () => {
        try {
            const response = await fetch('/api/mcp/system-info', {
                credentials: 'include',
            });

            // Handle session invalidation
            if (response.status === 401) {
                console.log('Session invalidated during system info fetch, logging out...');
                forceLogout();
                return;
            }

            if (!response.ok) {
                throw new Error('Failed to fetch system information');
            }

            const data = await response.json();
            setSystemInfo(data);
            setError('');
        } catch (err) {
            setError(err.message || 'Failed to load system information');
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
                    bgcolor: connected ? 'success.main' : 'error.main',
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
                <IconButton
                    size="small"
                    onClick={() => setExpanded(!expanded)}
                    sx={{ color: 'white' }}
                >
                    {expanded ? <ExpandLessIcon /> : <ExpandMoreIcon />}
                </IconButton>
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
        </Paper>
    );
};

export default StatusBanner;
