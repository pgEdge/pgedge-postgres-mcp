/*-------------------------------------------------------------------------
 *
 * MCP Client - OAuth Helper Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
    discover,
    generatePkce,
    ensureClient,
    buildAuthorizeUrl,
    exchangeCode,
    refresh,
    revoke,
    STORAGE_CLIENT,
} from '../oauth';

// base64url mirrors the encoding oauth.js uses internally, so tests can
// independently compute the expected PKCE challenge from the verifier.
function base64url(buffer) {
    const bytes = new Uint8Array(buffer);
    let binary = '';
    for (let i = 0; i < bytes.length; i++) {
        binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

const META = {
    issuer: 'https://issuer.example.com',
    authorization_endpoint: 'https://issuer.example.com/oauth/authorize',
    token_endpoint: 'https://issuer.example.com/oauth/token',
    registration_endpoint: 'https://issuer.example.com/oauth/register',
    revocation_endpoint: 'https://issuer.example.com/oauth/revoke',
};

function jsonResponse(status, body) {
    return {
        ok: status >= 200 && status < 300,
        status,
        json: async () => body,
    };
}

describe('oauth helpers', () => {
    beforeEach(() => {
        localStorage.clear();
        sessionStorage.clear();
    });

    it('discover returns null on 404', async () => {
        const fetchImpl = vi.fn().mockResolvedValue(jsonResponse(404, { error: 'not_found' }));
        const meta = await discover(fetchImpl);
        expect(meta).toBeNull();
        expect(fetchImpl).toHaveBeenCalledWith('/.well-known/oauth-authorization-server');
    });

    it('discover returns null when fetch throws', async () => {
        const fetchImpl = vi.fn().mockRejectedValue(new Error('network down'));
        const meta = await discover(fetchImpl);
        expect(meta).toBeNull();
    });

    it('discover returns metadata on 200', async () => {
        const fetchImpl = vi.fn().mockResolvedValue(jsonResponse(200, META));
        const meta = await discover(fetchImpl);
        expect(meta).toEqual(META);
    });

    it('generatePkce yields a 43-char S256 challenge of the verifier', async () => {
        const { verifier, challenge } = await generatePkce();
        const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier));
        expect(challenge).toBe(base64url(digest));
        expect(challenge).toHaveLength(43);
    });

    it('ensureClient registers once and caches per issuer', async () => {
        const fetchImpl = vi.fn().mockResolvedValue(jsonResponse(201, { client_id: 'client-123' }));

        const first = await ensureClient(META, fetchImpl);
        const second = await ensureClient(META, fetchImpl);

        expect(first).toBe('client-123');
        expect(second).toBe('client-123');
        expect(fetchImpl).toHaveBeenCalledTimes(1);
        expect(fetchImpl).toHaveBeenCalledWith(
            '/oauth/register',
            expect.objectContaining({ method: 'POST' })
        );
        expect(JSON.parse(localStorage.getItem(STORAGE_CLIENT))).toEqual({
            clientId: 'client-123',
            issuer: META.issuer,
        });
    });

    it('buildAuthorizeUrl includes all required parameters', () => {
        const url = new URL(
            buildAuthorizeUrl(META, 'client-123', 'the-challenge', 'the-state', 'https://app.example.com/oauth/callback')
        );

        expect(url.origin + url.pathname).toBe('https://issuer.example.com/oauth/authorize');
        expect(url.searchParams.get('response_type')).toBe('code');
        expect(url.searchParams.get('client_id')).toBe('client-123');
        expect(url.searchParams.get('redirect_uri')).toBe('https://app.example.com/oauth/callback');
        expect(url.searchParams.get('state')).toBe('the-state');
        expect(url.searchParams.get('scope')).toBe('mcp');
        expect(url.searchParams.get('code_challenge')).toBe('the-challenge');
        expect(url.searchParams.get('code_challenge_method')).toBe('S256');
    });

    it('exchangeCode posts form-encoded body and stores expiry', async () => {
        const now = Date.now();
        const fetchImpl = vi.fn().mockResolvedValue(jsonResponse(200, {
            access_token: 'access-1',
            token_type: 'Bearer',
            expires_in: 3600,
            refresh_token: 'refresh-1',
            scope: 'mcp',
        }));

        const session = await exchangeCode(META, 'client-123', 'the-code', 'the-verifier', 'https://app.example.com/oauth/callback', fetchImpl);

        expect(fetchImpl).toHaveBeenCalledWith('/oauth/token', expect.objectContaining({
            method: 'POST',
            headers: expect.objectContaining({ 'Content-Type': 'application/x-www-form-urlencoded' }),
        }));
        const [, options] = fetchImpl.mock.calls[0];
        const params = new URLSearchParams(options.body);
        expect(params.get('grant_type')).toBe('authorization_code');
        expect(params.get('code')).toBe('the-code');
        expect(params.get('client_id')).toBe('client-123');
        expect(params.get('redirect_uri')).toBe('https://app.example.com/oauth/callback');
        expect(params.get('code_verifier')).toBe('the-verifier');

        expect(session.accessToken).toBe('access-1');
        expect(session.refreshToken).toBe('refresh-1');
        expect(session.expiresAt).toBeGreaterThanOrEqual(now + 3600 * 1000);
    });

    it('refresh posts refresh_token grant', async () => {
        const fetchImpl = vi.fn().mockResolvedValue(jsonResponse(200, {
            access_token: 'access-2',
            token_type: 'Bearer',
            expires_in: 1800,
            refresh_token: 'refresh-2',
        }));

        const session = await refresh(META, 'client-123', { refreshToken: 'refresh-1' }, fetchImpl);

        const [, options] = fetchImpl.mock.calls[0];
        const params = new URLSearchParams(options.body);
        expect(params.get('grant_type')).toBe('refresh_token');
        expect(params.get('refresh_token')).toBe('refresh-1');
        expect(params.get('client_id')).toBe('client-123');
        expect(session.accessToken).toBe('access-2');
        expect(session.refreshToken).toBe('refresh-2');
    });

    it('exchangeCode rejects a 200 response with no access token', async () => {
        const fetchImpl = vi.fn().mockResolvedValue(jsonResponse(200, {
            token_type: 'Bearer',
            expires_in: 3600,
            refresh_token: 'refresh-1',
        }));

        await expect(
            exchangeCode(META, 'client-123', 'the-code', 'the-verifier', 'https://app.example.com/oauth/callback', fetchImpl)
        ).rejects.toThrow(/access token/i);
    });

    it('refresh rejects a 200 response with an empty access token', async () => {
        const fetchImpl = vi.fn().mockResolvedValue(jsonResponse(200, {
            access_token: '',
            token_type: 'Bearer',
            expires_in: 1800,
        }));

        await expect(
            refresh(META, 'client-123', { refreshToken: 'refresh-1' }, fetchImpl)
        ).rejects.toThrow(/access token/i);
    });

    it('revoke posts the refresh token', async () => {
        const fetchImpl = vi.fn().mockResolvedValue(jsonResponse(200, {}));

        await revoke(META, { refreshToken: 'refresh-1' }, fetchImpl);

        expect(fetchImpl).toHaveBeenCalledWith('/oauth/revoke', expect.objectContaining({ method: 'POST' }));
        const [, options] = fetchImpl.mock.calls[0];
        const params = new URLSearchParams(options.body);
        expect(params.get('token')).toBe('refresh-1');
    });

    it('revoke aborts a request that never resolves after its timeout', async () => {
        vi.useFakeTimers();
        try {
            let capturedSignal;
            const fetchImpl = vi.fn((url, options) => {
                capturedSignal = options.signal;
                // Never resolves on its own; only settles if aborted.
                return new Promise((_, reject) => {
                    options.signal.addEventListener('abort', () => {
                        reject(new DOMException('Aborted', 'AbortError'));
                    });
                });
            });

            const revokePromise = revoke(META, { refreshToken: 'refresh-1' }, fetchImpl);

            await vi.advanceTimersByTimeAsync(5000);

            // revoke() swallows the abort, same as any other network
            // failure -- it never throws for the caller.
            await expect(revokePromise).resolves.toBeUndefined();
            expect(capturedSignal.aborted).toBe(true);
        } finally {
            vi.useRealTimers();
        }
    });
});
