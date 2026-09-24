/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - AuthContext Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { AuthProvider, useAuth } from '../AuthContext';
import { mockDiscover, mockListTools, mockUserInfo, mockAuthenticateSuccess, mockAuthenticateFailure, mockOAuthAbsent } from '../../test-utils/mcp-mocks';

describe('AuthContext', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
    localStorage.clear();
  });

  afterEach(() => {
    localStorage.clear();
  });

  it('provides initial unauthenticated state', async () => {
    // No legacy token in localStorage, so only the OAuth discovery call
    // is made (unconditionally, on every mount); checkAuth itself returns
    // early without a fetch.
    global.fetch.mockResolvedValueOnce(mockOAuthAbsent());

    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider,
    });

    // Wait for loading to complete (when no token, checkAuth returns immediately)
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.user).toBe(null);
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  it('checks authentication status on mount', async () => {
    // Set a token in localStorage to trigger auth check
    localStorage.setItem('mcp-session-token', 'test-token');

    // Mock the sequence of calls that checkAuth makes:
    // 1. OAuth discovery (absent)
    global.fetch.mockResolvedValueOnce(mockOAuthAbsent());
    // 2. server/discover
    global.fetch.mockResolvedValueOnce(mockDiscover(1));
    // 3. listTools
    global.fetch.mockResolvedValueOnce(mockListTools(2));
    // 4. /api/user/info
    global.fetch.mockResolvedValueOnce(mockUserInfo('testuser'));

    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider,
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.user).toEqual({
      authenticated: true,
      username: 'testuser'
    });
    expect(global.fetch).toHaveBeenCalledTimes(4);
  });

  it('handles login successfully', async () => {
    // No token in localStorage, so initial auth check returns early
    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider,
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Mock authenticate_user tool call
    global.fetch.mockResolvedValueOnce(mockAuthenticateSuccess(1, 'testuser'));

    await result.current.login('testuser', 'testpass');

    await waitFor(() => {
      expect(result.current.user).toEqual({
        username: 'testuser',
        expiresAt: undefined
      });
    });
    expect(localStorage.getItem('mcp-session-token')).toBe('test-session-token');
  });

  it('throws error on login failure', async () => {
    // No token in localStorage, so initial auth check returns early
    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider,
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Mock authenticate_user tool failure
    global.fetch.mockResolvedValueOnce(mockAuthenticateFailure(1, 'Invalid credentials'));

    await expect(result.current.login('testuser', 'wrongpass')).rejects.toThrow(
      'Invalid username or password. Please try again.'
    );

    expect(result.current.user).toBe(null);
  });

  it('handles logout successfully', async () => {
    // Set up initial authenticated state with token
    localStorage.setItem('mcp-session-token', 'test-token');

    // Mock the sequence of calls that checkAuth makes:
    global.fetch.mockResolvedValueOnce(mockOAuthAbsent());
    global.fetch.mockResolvedValueOnce(mockDiscover(1));
    global.fetch.mockResolvedValueOnce(mockListTools(2));
    global.fetch.mockResolvedValueOnce(mockUserInfo('testuser'));

    const { result } = renderHook(() => useAuth(), {
      wrapper: AuthProvider,
    });

    await waitFor(() => {
      expect(result.current.user).toEqual({
        authenticated: true,
        username: 'testuser'
      });
    });

    // Logout with no OAuth session in play is local only, no fetch call
    await result.current.logout();

    await waitFor(() => {
      expect(result.current.user).toBe(null);
    });
    expect(localStorage.getItem('mcp-session-token')).toBe(null);
  });
});
