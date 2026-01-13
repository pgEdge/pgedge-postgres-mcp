/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { createContext, useState, useContext, useEffect } from 'react';
import { MCPClient } from '../lib/mcp-client';

const AuthContext = createContext(null);

// MCP server URL (proxied through nginx in production, direct in development)
const MCP_SERVER_URL = '/mcp/v1';

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [sessionToken, setSessionToken] = useState(() => {
    // Load session token from localStorage on initialization
    return localStorage.getItem('mcp-session-token');
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    checkAuth();
  }, []);

  const checkAuth = async () => {
    try {
      if (!sessionToken) {
        setLoading(false);
        return;
      }

      // Validate session by trying to list tools via JSON-RPC
      // This will fail with 401 if the session token is invalid
      const client = new MCPClient(MCP_SERVER_URL, sessionToken);
      await client.initialize();
      await client.listTools();

      // Session is valid - fetch user info from server
      const response = await fetch('/api/user/info', {
        headers: {
          'Authorization': `Bearer ${sessionToken}`
        }
      });

      if (!response.ok) {
        throw new Error('Failed to fetch user info');
      }

      const userInfo = await response.json();
      setUser({
        authenticated: true,
        username: userInfo.username
      });
    } catch (error) {
      console.error('Auth check failed:', error);
      // Invalid or expired session - clear it
      setSessionToken(null);
      localStorage.removeItem('mcp-session-token');
      setUser(null);
    } finally {
      setLoading(false);
    }
  };

  const login = async (username, password) => {
    try {
      // Authenticate via JSON-RPC authenticate_user tool
      const authResult = await MCPClient.authenticate(MCP_SERVER_URL, username, password);

      // Store session token in state and localStorage
      setSessionToken(authResult.sessionToken);
      localStorage.setItem('mcp-session-token', authResult.sessionToken);

      // Set user info
      setUser({
        username: authResult.username,
        expiresAt: authResult.expiresAt
      });
    } catch (error) {
      // Re-throw with user-friendly message
      throw new Error(error.message || 'Login failed');
    }
  };

  const logout = () => {
    // Clear session token
    setSessionToken(null);
    localStorage.removeItem('mcp-session-token');
    setUser(null);

    // Note: We don't need to call a logout API - the session token
    // will simply expire on the server after its TTL
  };

  // Force logout without any cleanup (used when session is invalidated)
  const forceLogout = () => {
    setSessionToken(null);
    localStorage.removeItem('mcp-session-token');
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, sessionToken, loading, login, logout, forceLogout }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
