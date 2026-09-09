/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - AuthContext OAuth Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { renderHook } from '@testing-library/react';
import { AuthProvider, useAuth } from '../AuthContext';
import Login from '../../components/Login';
import { STORAGE_PKCE, STORAGE_SESSION, CALLBACK_PATH } from '../../lib/oauth';
import { mockOAuthAbsent, mockOAuthMetadata } from '../../test-utils/mcp-mocks';

describe('AuthContext OAuth flow', () => {
    beforeEach(() => {
        global.fetch = vi.fn();
        localStorage.clear();
        sessionStorage.clear();
        window.history.pushState({}, '', '/');
    });

    afterEach(() => {
        localStorage.clear();
        sessionStorage.clear();
        window.history.pushState({}, '', '/');
    });

    it('renders OAuth login when the server advertises it', async () => {
        global.fetch.mockResolvedValueOnce(mockOAuthMetadata());

        render(
            <AuthProvider>
                <Login />
            </AuthProvider>
        );

        await waitFor(() => {
            expect(screen.queryByLabelText(/password/i)).not.toBeInTheDocument();
        });

        expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
    });

    it('falls back to the password form when OAuth is absent', async () => {
        global.fetch.mockResolvedValueOnce(mockOAuthAbsent());

        render(
            <AuthProvider>
                <Login />
            </AuthProvider>
        );

        await waitFor(() => {
            expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
        });

        expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    });

    it('handles the callback path: exchanges code and stores session', async () => {
        sessionStorage.setItem(STORAGE_PKCE, JSON.stringify({ verifier: 'the-verifier', state: 'the-state' }));
        window.history.pushState({}, '', `${CALLBACK_PATH}?code=the-code&state=the-state`);

        // 1. OAuth discovery
        global.fetch.mockResolvedValueOnce(mockOAuthMetadata());
        // 2. Dynamic client registration (no cached client yet)
        global.fetch.mockResolvedValueOnce({
            ok: true,
            status: 201,
            json: async () => ({ client_id: 'client-abc' }),
        });
        // 3. Token exchange
        global.fetch.mockResolvedValueOnce({
            ok: true,
            status: 200,
            json: async () => ({
                access_token: 'access-token-1',
                token_type: 'Bearer',
                expires_in: 3600,
                refresh_token: 'refresh-token-1',
                scope: 'mcp',
            }),
        });
        // 4. MCP server/discover (part of validating the new session)
        global.fetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ jsonrpc: '2.0', id: 1, result: {} }),
        });
        // 5. tools/list
        global.fetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ jsonrpc: '2.0', id: 2, result: { tools: [] } }),
        });
        // 6. /api/user/info
        global.fetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ authenticated: true, username: 'alice', auth_method: 'oauth' }),
        });

        const { result } = renderHook(() => useAuth(), {
            wrapper: AuthProvider,
        });

        await waitFor(() => {
            expect(result.current.loading).toBe(false);
        });

        expect(result.current.user).toEqual({ authenticated: true, username: 'alice' });
        expect(result.current.sessionToken).toBe('access-token-1');
        expect(JSON.parse(localStorage.getItem(STORAGE_SESSION)).accessToken).toBe('access-token-1');
        expect(sessionStorage.getItem(STORAGE_PKCE)).toBeNull();
        expect(window.location.pathname).toBe('/');
    });

    it('rejects a callback with the wrong state', async () => {
        sessionStorage.setItem(STORAGE_PKCE, JSON.stringify({ verifier: 'the-verifier', state: 'the-state' }));
        window.history.pushState({}, '', `${CALLBACK_PATH}?code=the-code&state=wrong-state`);

        global.fetch.mockResolvedValueOnce(mockOAuthMetadata());

        const { result } = renderHook(() => useAuth(), {
            wrapper: AuthProvider,
        });

        await waitFor(() => {
            expect(result.current.loading).toBe(false);
        });

        expect(result.current.user).toBe(null);
        expect(result.current.authError).toMatch(/state/i);
        expect(localStorage.getItem(STORAGE_SESSION)).toBeNull();
    });
});
