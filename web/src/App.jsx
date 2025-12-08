/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useState, useEffect, useCallback } from 'react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import { Container, Box, CircularProgress, CssBaseline, IconButton, Tooltip } from '@mui/material';
import { ChevronRight as ChevronRightIcon, ChevronLeft as ChevronLeftIcon } from '@mui/icons-material';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import { useMCPClient } from './hooks/useMCPClient';
import { useConversations } from './hooks/useConversations';
import Header from './components/Header';
import MainContent from './components/MainContent';
import ConversationPanel from './components/ConversationPanel';
import Login from './components/Login';

// Light theme for login page (always light mode)
const lightTheme = createTheme({
  palette: {
    mode: 'light',
    primary: {
      main: '#3f51b5',
    },
    background: {
      default: '#ffffff',
      paper: '#ffffff',
    },
    text: {
      primary: 'rgba(0, 0, 0, 0.87)',
      secondary: 'rgba(0, 0, 0, 0.54)',
    },
  },
});

const AppContent = () => {
  const [mode, setMode] = useState(() => {
    // Load theme preference from localStorage
    const savedMode = localStorage.getItem('theme-mode');
    return savedMode || 'light';
  });
  const [conversationPanelOpen, setConversationPanelOpen] = useState(false);
  const { user, loading, sessionToken } = useAuth();
  const { serverInfo } = useMCPClient(sessionToken);
  const conversations = useConversations(sessionToken);

  // Save theme preference to localStorage when it changes
  useEffect(() => {
    localStorage.setItem('theme-mode', mode);
  }, [mode]);

  const theme = createTheme({
    palette: {
      mode,
      ...(mode === 'light' && {
        primary: {
          main: '#3f51b5',
        },
        background: {
          default: '#f5f5f5',
          paper: '#ffffff',
        },
        text: {
          primary: 'rgba(0, 0, 0, 0.87)',
          secondary: 'rgba(0, 0, 0, 0.54)',
        },
      }),
      ...(mode === 'dark' && {
        primary: {
          main: '#1976d2',
        },
        secondary: {
          main: '#dc004e',
        },
        background: {
          default: '#121212',
          paper: '#1e1e1e',
        },
      }),
    },
  });

  const toggleTheme = () => {
    setMode((prevMode) => (prevMode === 'light' ? 'dark' : 'light'));
  };

  const handleConversationsClick = useCallback(() => {
    setConversationPanelOpen(true);
  }, []);

  const handleConversationPanelClose = useCallback(() => {
    setConversationPanelOpen(false);
  }, []);

  if (loading) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Box
          sx={{
            minHeight: '100vh',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            bgcolor: 'background.default',
          }}
        >
          <CircularProgress />
        </Box>
      </ThemeProvider>
    );
  }

  if (!user) {
    return (
      <ThemeProvider theme={lightTheme}>
        <CssBaseline />
        <Login />
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', bgcolor: 'background.default', overflow: 'hidden' }}>
        <Header
          onToggleTheme={toggleTheme}
          mode={mode}
          serverInfo={serverInfo}
        />
        <Box sx={{ flex: 1, display: 'flex', overflow: 'hidden', position: 'relative' }}>
          {/* Vertical tab button for conversation panel */}
          <Tooltip title={conversationPanelOpen ? "Close history" : "Open history"} placement="right">
            <IconButton
              onClick={conversationPanelOpen ? handleConversationPanelClose : handleConversationsClick}
              aria-label="toggle conversation history"
              sx={{
                position: 'absolute',
                left: 0,
                top: 16,
                zIndex: 1200,
                bgcolor: 'background.paper',
                borderRadius: '0 4px 4px 0',
                boxShadow: 2,
                width: 24,
                height: 48,
                minWidth: 24,
                padding: 0,
                '&:hover': {
                  bgcolor: 'action.hover',
                },
              }}
            >
              {conversationPanelOpen ? <ChevronLeftIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
            </IconButton>
          </Tooltip>
          <Container maxWidth="lg" sx={{ flex: 1, display: 'flex', flexDirection: 'column', py: 2, overflow: 'hidden' }}>
            <MainContent conversations={conversations} />
          </Container>
        </Box>
      </Box>
      <ConversationPanel
        open={conversationPanelOpen}
        onClose={handleConversationPanelClose}
        conversations={conversations.conversations}
        currentConversationId={conversations.currentConversationId}
        onSelect={conversations.selectConversation}
        onNewConversation={conversations.startNewConversation}
        onRename={conversations.renameConversation}
        onDelete={conversations.deleteConversation}
        onDeleteAll={conversations.deleteAllConversations}
        loading={conversations.loading}
        disabled={false}
      />
    </ThemeProvider>
  );
};

const App = () => {
  return (
    <AuthProvider>
      <AppContent />
    </AuthProvider>
  );
};

export default App;
