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
import { render, screen, waitFor, act } from '@testing-library/react';
import { renderHook } from '@testing-library/react';
import { AuthProvider, useAuth } from '../AuthContext';
import Login from '../../components/Login';
import { STORAGE_PKCE, STORAGE_SESSION, STORAGE_CLIENT, CALLBACK_PATH } from '../../lib/oauth';
import { mockOAuthAbsent, mockOAuthMetadata, mockDiscover, mockListTools, mockUserInfo } from '../../test-utils/mcp-mocks';

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

describe('AuthContext handleUnauthorized', () => {
    const ISSUER = 'http://localhost:8080';

    // Seeds an established OAuth session and a cached dynamic client, so
    // mounting AuthProvider validates the seeded session rather than
    // registering a fresh client.
    const seedOAuthSession = (accessToken, refreshToken = 'refresh-token-1') => {
        localStorage.setItem(STORAGE_SESSION, JSON.stringify({
            accessToken,
            refreshToken,
            expiresAt: Date.now() + 3600 * 1000,
        }));
        localStorage.setItem(STORAGE_CLIENT, JSON.stringify({ clientId: 'client-abc', issuer: ISSUER }));
    };

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

    const mountAuthenticated = async () => {
        // 1. OAuth discovery, 2. MCP server/discover, 3. tools/list,
        // 4. /api/user/info -- the sequence AuthProvider's mount effect
        // runs to validate the seeded session.
        global.fetch.mockResolvedValueOnce(mockOAuthMetadata({ issuer: ISSUER }));
        global.fetch.mockResolvedValueOnce(mockDiscover(1));
        global.fetch.mockResolvedValueOnce(mockListTools(2));
        global.fetch.mockResolvedValueOnce(mockUserInfo('alice'));

        const { result } = renderHook(() => useAuth(), {
            wrapper: AuthProvider,
        });

        await waitFor(() => {
            expect(result.current.loading).toBe(false);
        });

        return result;
    };

    it('refreshes a valid OAuth session and resolves true', async () => {
        seedOAuthSession('access-token-1');
        const result = await mountAuthenticated();

        global.fetch.mockResolvedValueOnce({
            ok: true,
            status: 200,
            json: async () => ({
                access_token: 'access-token-2',
                token_type: 'Bearer',
                expires_in: 1800,
                refresh_token: 'refresh-token-2',
            }),
        });

        let recovered;
        await act(async () => {
            recovered = await result.current.handleUnauthorized();
        });

        expect(recovered).toBe(true);
        expect(result.current.sessionToken).toBe('access-token-2');
        expect(JSON.parse(localStorage.getItem(STORAGE_SESSION)).accessToken).toBe('access-token-2');

        const refreshCall = global.fetch.mock.calls.find(([url]) => url === '/oauth/token');
        expect(refreshCall).toBeDefined();
        const params = new URLSearchParams(refreshCall[1].body);
        expect(params.get('grant_type')).toBe('refresh_token');
        expect(params.get('refresh_token')).toBe('refresh-token-1');
    });

    it('does not retry a refresh already attempted for the same access token', async () => {
        seedOAuthSession('access-token-1');
        const result = await mountAuthenticated();

        // A second 401 for the SAME token arrives before any refresh has
        // replaced it (e.g. two requests failing back-to-back); only the
        // first should reach the network, the second should short-circuit.
        global.fetch.mockResolvedValueOnce({
            ok: true,
            status: 200,
            json: async () => ({
                access_token: 'access-token-2',
                token_type: 'Bearer',
                expires_in: 1800,
                refresh_token: 'refresh-token-2',
            }),
        });

        let firstRecovered;
        let secondRecovered;
        await act(async () => {
            [firstRecovered, secondRecovered] = await Promise.all([
                result.current.handleUnauthorized(),
                result.current.handleUnauthorized(),
            ]);
        });

        // Only the first reaches the network; the second, recognising the
        // same access token already has a refresh in flight/attempted,
        // short-circuits straight to false without a second request. (Which
        // of the two completes last is a genuine race -- by design, the
        // dedup guard's only hard guarantee is a single network call and
        // the second caller being told not to retry.)
        expect(firstRecovered).toBe(true);
        expect(secondRecovered).toBe(false);

        const refreshCalls = global.fetch.mock.calls.filter(([url]) => url === '/oauth/token');
        expect(refreshCalls).toHaveLength(1);
    });

    it('resolves false and clears the session when a refresh attempt fails', async () => {
        seedOAuthSession('access-token-1');
        const result = await mountAuthenticated();

        global.fetch.mockRejectedValueOnce(new Error('network down'));

        let recovered;
        await act(async () => {
            recovered = await result.current.handleUnauthorized();
        });

        expect(recovered).toBe(false);
        expect(result.current.user).toBe(null);
        expect(result.current.sessionToken).toBeFalsy();
        expect(localStorage.getItem(STORAGE_SESSION)).toBeNull();

        // Calling it again afterwards (no OAuth session left) resolves
        // false the same way, without attempting another refresh.
        let recoveredAgain;
        await act(async () => {
            recoveredAgain = await result.current.handleUnauthorized();
        });
        expect(recoveredAgain).toBe(false);
        const refreshCalls = global.fetch.mock.calls.filter(([url]) => url === '/oauth/token');
        expect(refreshCalls).toHaveLength(1);
    });

    it('with no OAuth session, clears the legacy token and resolves false', async () => {
        localStorage.setItem('mcp-session-token', 'legacy-token-1');
        global.fetch.mockResolvedValueOnce(mockOAuthAbsent());
        global.fetch.mockResolvedValueOnce(mockDiscover(1));
        global.fetch.mockResolvedValueOnce(mockListTools(2));
        global.fetch.mockResolvedValueOnce(mockUserInfo('bob'));

        const { result } = renderHook(() => useAuth(), {
            wrapper: AuthProvider,
        });

        await waitFor(() => {
            expect(result.current.loading).toBe(false);
        });
        expect(result.current.sessionToken).toBe('legacy-token-1');

        let recovered;
        await act(async () => {
            recovered = await result.current.handleUnauthorized();
        });

        expect(recovered).toBe(false);
        expect(result.current.user).toBe(null);
        expect(result.current.sessionToken).toBeFalsy();
        expect(localStorage.getItem('mcp-session-token')).toBeNull();
    });
});
