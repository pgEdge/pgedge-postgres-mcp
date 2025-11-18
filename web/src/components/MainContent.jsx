/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useState, useEffect } from 'react';
import {
  Box,
  Paper,
  Typography,
  CircularProgress,
  Alert,
  Card,
  CardContent,
  Grid,
  Chip,
} from '@mui/material';
import {
  CheckCircle as CheckCircleIcon,
  Error as ErrorIcon,
  Storage as StorageIcon,
} from '@mui/icons-material';

const MainContent = () => {
  const [systemInfo, setSystemInfo] = useState(null);
  const [loading, setLoading] = useState(true);
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

      if (!response.ok) {
        throw new Error('Failed to fetch system information');
      }

      const data = await response.json();
      setSystemInfo(data);
      setError('');
    } catch (err) {
      setError(err.message || 'Failed to load system information');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '400px' }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Alert severity="error" sx={{ mb: 2 }}>
        {error}
      </Alert>
    );
  }

  return (
    <Box>
      <Typography variant="h4" gutterBottom sx={{ mb: 3 }}>
        MCP Server Status
      </Typography>

      <Grid container spacing={3}>
        {/* Server Status Card */}
        <Grid item xs={12}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
                <StorageIcon sx={{ mr: 1, color: 'primary.main' }} />
                <Typography variant="h6">Server Information</Typography>
                <Chip
                  icon={systemInfo ? <CheckCircleIcon /> : <ErrorIcon />}
                  label={systemInfo ? 'Connected' : 'Disconnected'}
                  color={systemInfo ? 'success' : 'error'}
                  size="small"
                  sx={{ ml: 2 }}
                />
              </Box>

              {systemInfo && (
                <Grid container spacing={2}>
                  <Grid item xs={12} sm={6}>
                    <Paper variant="outlined" sx={{ p: 2 }}>
                      <Typography variant="caption" color="text.secondary">
                        PostgreSQL Version
                      </Typography>
                      <Typography variant="body1" sx={{ fontWeight: 500 }}>
                        {systemInfo.postgresql_version || 'N/A'}
                      </Typography>
                    </Paper>
                  </Grid>

                  <Grid item xs={12} sm={6}>
                    <Paper variant="outlined" sx={{ p: 2 }}>
                      <Typography variant="caption" color="text.secondary">
                        Operating System
                      </Typography>
                      <Typography variant="body1" sx={{ fontWeight: 500 }}>
                        {systemInfo.operating_system || 'N/A'}
                      </Typography>
                    </Paper>
                  </Grid>

                  <Grid item xs={12} sm={6}>
                    <Paper variant="outlined" sx={{ p: 2 }}>
                      <Typography variant="caption" color="text.secondary">
                        Architecture
                      </Typography>
                      <Typography variant="body1" sx={{ fontWeight: 500 }}>
                        {systemInfo.architecture || 'N/A'}
                      </Typography>
                    </Paper>
                  </Grid>

                  <Grid item xs={12} sm={6}>
                    <Paper variant="outlined" sx={{ p: 2 }}>
                      <Typography variant="caption" color="text.secondary">
                        Bit Version
                      </Typography>
                      <Typography variant="body1" sx={{ fontWeight: 500 }}>
                        {systemInfo.bit_version || 'N/A'}
                      </Typography>
                    </Paper>
                  </Grid>

                  {systemInfo.compiler && (
                    <Grid item xs={12} sm={6}>
                      <Paper variant="outlined" sx={{ p: 2 }}>
                        <Typography variant="caption" color="text.secondary">
                          Compiler
                        </Typography>
                        <Typography variant="body1" sx={{ fontWeight: 500 }}>
                          {systemInfo.compiler}
                        </Typography>
                      </Paper>
                    </Grid>
                  )}

                  {systemInfo.full_version && (
                    <Grid item xs={12}>
                      <Paper variant="outlined" sx={{ p: 2 }}>
                        <Typography variant="caption" color="text.secondary">
                          Full Version String
                        </Typography>
                        <Typography variant="body2" sx={{ mt: 1, fontFamily: 'monospace', fontSize: '0.875rem' }}>
                          {systemInfo.full_version}
                        </Typography>
                      </Paper>
                    </Grid>
                  )}
                </Grid>
              )}
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </Box>
  );
};

export default MainContent;
