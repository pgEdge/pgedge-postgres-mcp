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

    it('refreshes an expired stored session on mount rather than clearing it', async () => {
        const issuer = 'http://localhost:8080';
        localStorage.setItem(STORAGE_SESSION, JSON.stringify({
            accessToken: 'expired-access',
            refreshToken: 'refresh-token-1',
            expiresAt: Date.now() - 1000,
        }));
        localStorage.setItem(STORAGE_CLIENT, JSON.stringify({ clientId: 'client-abc', issuer }));

        // 1. discovery, 2. refresh, then the validation sequence.
        global.fetch.mockResolvedValueOnce(mockOAuthMetadata({ issuer }));
        global.fetch.mockResolvedValueOnce({
            ok: true,
            status: 200,
            json: async () => ({
                access_token: 'fresh-access',
                token_type: 'Bearer',
                expires_in: 3600,
                refresh_token: 'refresh-token-2',
            }),
        });
        global.fetch.mockResolvedValueOnce(mockDiscover(1));
        global.fetch.mockResolvedValueOnce(mockListTools(2));
        global.fetch.mockResolvedValueOnce(mockUserInfo('alice'));

        const { result } = renderHook(() => useAuth(), { wrapper: AuthProvider });

        await waitFor(() => {
            expect(result.current.loading).toBe(false);
        });

        expect(result.current.sessionToken).toBe('fresh-access');
        expect(JSON.parse(localStorage.getItem(STORAGE_SESSION)).accessToken).toBe('fresh-access');
        expect(result.current.user).not.toBe(null);
    });

    it('clears an expired stored session when the refresh fails', async () => {
        const issuer = 'http://localhost:8080';
        localStorage.setItem(STORAGE_SESSION, JSON.stringify({
            accessToken: 'expired-access',
            refreshToken: 'refresh-token-1',
            expiresAt: Date.now() - 1000,
        }));
        localStorage.setItem(STORAGE_CLIENT, JSON.stringify({ clientId: 'client-abc', issuer }));

        global.fetch.mockResolvedValueOnce(mockOAuthMetadata({ issuer }));
        global.fetch.mockResolvedValueOnce({
            ok: false,
            status: 400,
            json: async () => ({ error: 'invalid_grant' }),
        });

        const { result } = renderHook(() => useAuth(), { wrapper: AuthProvider });

        await waitFor(() => {
            expect(result.current.loading).toBe(false);
        });

        expect(result.current.user).toBe(null);
        expect(localStorage.getItem(STORAGE_SESSION)).toBeNull();
    });

    it('registers a fresh client each time a sign-in is started', async () => {
        const issuer = 'http://localhost:8080';
        localStorage.setItem(STORAGE_CLIENT, JSON.stringify({ clientId: 'stale-client', issuer }));

        global.fetch.mockResolvedValueOnce(mockOAuthMetadata({ issuer }));
        global.fetch.mockResolvedValueOnce({
            ok: true,
            status: 201,
            json: async () => ({ client_id: 'client-fresh' }),
        });

        const assign = vi.fn();
        const originalLocation = window.location;
        delete window.location;
        window.location = { ...originalLocation, assign, origin: originalLocation.origin };

        try {
            const { result } = renderHook(() => useAuth(), { wrapper: AuthProvider });
            await waitFor(() => {
                expect(result.current.loading).toBe(false);
            });

            await act(async () => {
                await result.current.startOAuthLogin();
            });

            const registerCall = global.fetch.mock.calls.find(([url]) => url === '/oauth/register');
            expect(registerCall).toBeDefined();
            expect(JSON.parse(localStorage.getItem(STORAGE_CLIENT)).clientId).toBe('client-fresh');
            expect(assign).toHaveBeenCalled();
            expect(assign.mock.calls[0][0]).toContain('client_id=client-fresh');
        } finally {
            window.location = originalLocation;
        }
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

    it('shares one in-flight refresh across two concurrent 401s for the same token', async () => {
        seedOAuthSession('access-token-1');
        const result = await mountAuthenticated();

        // Two requests fail with 401 for the same still-current token at
        // roughly the same time (the common case, not a sign the token is
        // unrecoverable): both should be told the retry succeeded, and
        // only one refresh should actually reach the network.
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

        expect(firstRecovered).toBe(true);
        expect(secondRecovered).toBe(true);
        expect(result.current.sessionToken).toBe('access-token-2');

        const refreshCalls = global.fetch.mock.calls.filter(([url]) => url === '/oauth/token');
        expect(refreshCalls).toHaveLength(1);
    });

    it('logout clears the session immediately even if revoke never resolves', async () => {
        seedOAuthSession('access-token-1');
        const result = await mountAuthenticated();

        // Simulate a hung server: the revocation request never settles.
        global.fetch.mockImplementationOnce(() => new Promise(() => {}));

        await act(async () => {
            await result.current.logout();
        });

        expect(result.current.user).toBe(null);
        expect(result.current.sessionToken).toBeFalsy();
        expect(localStorage.getItem(STORAGE_SESSION)).toBeNull();
    });

    it('a refresh that resolves after forceLogout does not resurrect the session', async () => {
        seedOAuthSession('access-token-1');
        const result = await mountAuthenticated();

        // The refresh's own fetch call is left pending, simulating a
        // slow token endpoint, so forceLogout() below runs while it is
        // still in flight.
        let resolveRefresh;
        const pendingRefresh = new Promise((resolve) => {
            resolveRefresh = resolve;
        });
        global.fetch.mockImplementationOnce(() => pendingRefresh);

        let handleUnauthorizedPromise;
        act(() => {
            handleUnauthorizedPromise = result.current.handleUnauthorized();
            result.current.forceLogout();
        });

        await act(async () => {
            resolveRefresh({
                ok: true,
                status: 200,
                json: async () => ({
                    access_token: 'access-token-2',
                    token_type: 'Bearer',
                    expires_in: 1800,
                    refresh_token: 'refresh-token-2',
                }),
            });
            await handleUnauthorizedPromise;
        });

        // The refresh technically succeeded, but it is stale by the time
        // it resolves: forceLogout() already moved the session on, so its
        // result must be discarded rather than resurrecting a session.
        expect(result.current.user).toBe(null);
        expect(result.current.sessionToken).toBeFalsy();
        expect(localStorage.getItem(STORAGE_SESSION)).toBeNull();
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
