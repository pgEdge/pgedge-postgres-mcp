/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Header Component
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Styled to match pgEdge Cloud product aesthetics
 *
 *-------------------------------------------------------------------------
 */

import React, { useState } from 'react';
import {
    AppBar,
    Toolbar,
    Typography,
    IconButton,
    Box,
    Avatar,
    Menu,
    MenuItem,
    ListItemIcon,
    ListItemText,
    Divider,
    Tooltip,
    alpha,
} from '@mui/material';
import {
    DarkMode as DarkModeIcon,
    LightMode as LightModeIcon,
    Logout as LogoutIcon,
    HelpOutline as HelpIcon,
} from '@mui/icons-material';
import logoLight from '../assets/images/logo-light.png';
import logoDark from '../assets/images/logo-dark.png';
import { useAuth } from '../contexts/AuthContext';
import { useMenu } from '../hooks/useMenu';
import HelpPanel from './HelpPanel';

const Header = ({ onToggleTheme, mode, serverInfo }) => {
    const { user, logout } = useAuth();
    const userMenu = useMenu();
    const [helpOpen, setHelpOpen] = useState(false);

    const handleLogout = () => {
        userMenu.handleClose();
        logout();
    };

    const handleHelpOpen = () => {
        setHelpOpen(true);
    };

    const handleHelpClose = () => {
        setHelpOpen(false);
    };

    const getInitials = (name) => {
        if (!name) return '?';
        const parts = name.split(' ');
        if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
        return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
    };

    const isDark = mode === 'dark';

    return (
        <>
            <AppBar
                position="static"
                elevation={0}
                sx={{
                    bgcolor: isDark ? '#1E293B' : '#FFFFFF',
                    borderBottom: '1px solid',
                    borderColor: isDark ? '#334155' : '#E5E7EB',
                }}
            >
                <Toolbar sx={{ minHeight: { xs: 56, sm: 64 } }}>
                    {/* Logo and Title */}
                    <Box sx={{ display: 'flex', alignItems: 'center', flexGrow: 1, gap: 1.5 }}>
                        <Box
                            component="img"
                            src={isDark ? logoDark : logoLight}
                            alt="pgEdge"
                            sx={{
                                height: 28,
                                width: 'auto',
                            }}
                        />
                        <Divider
                            orientation="vertical"
                            flexItem
                            sx={{
                                height: 24,
                                alignSelf: 'center',
                                borderColor: isDark ? '#475569' : '#E5E7EB',
                            }}
                        />
                        <Typography
                            variant="subtitle1"
                            component="div"
                            sx={{
                                fontWeight: 500,
                                color: isDark ? '#F1F5F9' : '#1F2937',
                                letterSpacing: '-0.01em',
                            }}
                        >
                            Natural Language Agent
                        </Typography>
                    </Box>

                    {/* Action Icons */}
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                        {/* Theme Toggle */}
                        <Tooltip title={isDark ? 'Switch to light mode' : 'Switch to dark mode'}>
                            <IconButton
                                onClick={onToggleTheme}
                                aria-label="toggle theme"
                                sx={{
                                    color: isDark ? '#94A3B8' : '#6B7280',
                                    '&:hover': {
                                        bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                        color: '#15AABF',
                                    },
                                }}
                            >
                                {isDark ? <LightModeIcon /> : <DarkModeIcon />}
                            </IconButton>
                        </Tooltip>

                        {/* Help Button */}
                        <Tooltip title="Help">
                            <IconButton
                                onClick={handleHelpOpen}
                                aria-label="open help"
                                sx={{
                                    color: isDark ? '#94A3B8' : '#6B7280',
                                    '&:hover': {
                                        bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                        color: '#15AABF',
                                    },
                                }}
                            >
                                <HelpIcon />
                            </IconButton>
                        </Tooltip>

                        {/* User Avatar */}
                        {user && (
                            <Tooltip title={user.username}>
                                <IconButton
                                    onClick={userMenu.handleOpen}
                                    size="small"
                                    aria-label="user menu"
                                    aria-controls="user-menu"
                                    aria-haspopup="true"
                                    sx={{
                                        ml: 0.5,
                                        p: 0.5,
                                        '&:hover': {
                                            bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                        },
                                    }}
                                >
                                    <Avatar
                                        sx={{
                                            width: 32,
                                            height: 32,
                                            bgcolor: '#15AABF',
                                            fontSize: '0.875rem',
                                            fontWeight: 600,
                                        }}
                                    >
                                        {getInitials(user.username)}
                                    </Avatar>
                                </IconButton>
                            </Tooltip>
                        )}
                    </Box>
                </Toolbar>
            </AppBar>

            {/* User Menu */}
            <Menu
                id="user-menu"
                anchorEl={userMenu.anchorEl}
                open={userMenu.open}
                onClose={userMenu.handleClose}
                anchorOrigin={{
                    vertical: 'bottom',
                    horizontal: 'right',
                }}
                transformOrigin={{
                    vertical: 'top',
                    horizontal: 'right',
                }}
                PaperProps={{
                    sx: {
                        minWidth: 180,
                        mt: 1,
                        borderRadius: 1,
                        border: '1px solid',
                        borderColor: isDark ? '#334155' : '#E5E7EB',
                        boxShadow: isDark
                            ? '0 10px 15px -3px rgba(0, 0, 0, 0.3)'
                            : '0 10px 15px -3px rgba(0, 0, 0, 0.1)',
                    },
                }}
            >
                <Box sx={{ px: 2, py: 1.5 }}>
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
                        Signed in as
                    </Typography>
                    <Typography
                        variant="body2"
                        sx={{
                            fontWeight: 500,
                            color: isDark ? '#F1F5F9' : '#1F2937',
                            mt: 0.25,
                        }}
                    >
                        {user?.username}
                    </Typography>
                </Box>
                <Divider sx={{ borderColor: isDark ? '#334155' : '#E5E7EB' }} />
                <MenuItem
                    onClick={handleLogout}
                    sx={{
                        mx: 1,
                        my: 0.5,
                        borderRadius: 1,
                        color: '#EF4444',
                        '&:hover': {
                            bgcolor: alpha('#EF4444', 0.08),
                        },
                    }}
                >
                    <ListItemIcon sx={{ color: 'inherit' }}>
                        <LogoutIcon fontSize="small" />
                    </ListItemIcon>
                    <ListItemText
                        primary="Sign out"
                        primaryTypographyProps={{
                            fontSize: '0.875rem',
                            fontWeight: 500,
                        }}
                    />
                </MenuItem>
            </Menu>

            {/* Help Panel */}
            <HelpPanel open={helpOpen} onClose={handleHelpClose} serverInfo={serverInfo} />
        </>
    );
};

export default Header;
