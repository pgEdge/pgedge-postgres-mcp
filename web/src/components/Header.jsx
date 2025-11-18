/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React from 'react';
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
} from '@mui/material';
import {
  Brightness4 as Brightness4Icon,
  Brightness7 as Brightness7Icon,
  Logout as LogoutIcon,
} from '@mui/icons-material';
import logoLight from '../assets/images/logo-light.png';
import logoDark from '../assets/images/logo-dark.png';
import { useAuth } from '../contexts/AuthContext';
import { useMenu } from '../hooks/useMenu';

const Header = ({ onToggleTheme, mode }) => {
  const { user, logout } = useAuth();
  const userMenu = useMenu();

  const handleLogout = () => {
    userMenu.handleClose();
    logout();
  };

  const getInitials = (name) => {
    if (!name) return '?';
    const parts = name.split(' ');
    if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
    return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
  };

  return (
    <>
      <AppBar
        position="static"
        sx={{
          bgcolor: mode === 'light' ? '#ffffff' : '#141519',
          color: mode === 'light' ? '#212121' : '#fefefe',
          boxShadow: mode === 'light' ? 1 : 4,
        }}
      >
        <Toolbar>
          <Box sx={{ display: 'flex', alignItems: 'center', flexGrow: 1, gap: 2 }}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 2,
              }}
            >
              <img
                src={mode === 'dark' ? logoDark : logoLight}
                alt="pgEdge MCP Client"
                style={{ height: '32px' }}
              />
              <Typography variant="h6" component="div">
                MCP Client
              </Typography>
            </Box>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <IconButton
              color="inherit"
              onClick={onToggleTheme}
              aria-label="toggle theme"
            >
              {mode === 'dark' ? <Brightness7Icon /> : <Brightness4Icon />}
            </IconButton>
            {user && (
              <IconButton
                onClick={userMenu.handleOpen}
                size="small"
                aria-label="user menu"
                aria-controls="user-menu"
                aria-haspopup="true"
              >
                <Avatar
                  sx={{
                    width: 32,
                    height: 32,
                    bgcolor: mode === 'light' ? '#3f51b5' : '#1976d2',
                  }}
                >
                  {getInitials(user.username)}
                </Avatar>
              </IconButton>
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
      >
        <Box sx={{ px: 2, py: 1 }}>
          <Typography variant="subtitle2">{user?.username}</Typography>
        </Box>
        <Divider />
        <MenuItem onClick={handleLogout}>
          <ListItemIcon>
            <LogoutIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Logout</ListItemText>
        </MenuItem>
      </Menu>
    </>
  );
};

export default Header;
