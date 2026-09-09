/*-------------------------------------------------------------------------
 *
 * MCP Client - OAuth 2.0 Authorization Code (PKCE) flow helpers
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// The well-known path the server publishes OAuth metadata at. Fetched via
// the same-origin path, since the SPA is served through nginx or the Vite
// dev proxy and the issuer in the metadata may be a different origin.
export const METADATA_PATH = '/.well-known/oauth-authorization-server';

// The client-side route the server redirects back to once the user has
// authenticated. Handled entirely inside AuthContext, without a router.
export const CALLBACK_PATH = '/oauth/callback';

// localStorage key for the current OAuth session (survives reloads).
export const STORAGE_SESSION = 'mcp-oauth-session';

// localStorage key for the dynamically registered client, cached per issuer.
export const STORAGE_CLIENT = 'mcp-oauth-client';

// sessionStorage key for the PKCE verifier and state, only needed for the
// duration of a single sign-in round trip.
export const STORAGE_PKCE = 'mcp-oauth-pkce';

// How long revoke() waits for the server before giving up. Revocation is
// best-effort and must never hold up the UI, so a hung server aborts the
// request rather than leaving the caller waiting indefinitely.
const REVOKE_TIMEOUT_MS = 5000;

// toPath reduces an absolute URL to its same-origin path, so the browser
// calls the proxied route (/oauth/register, /oauth/token, /oauth/revoke)
// rather than the issuer's own origin, which may not be reachable directly
// or may not have CORS configured for the SPA.
function toPath(absoluteUrl) {
    return new URL(absoluteUrl).pathname;
}

// base64url encodes a byte buffer using the URL-safe, unpadded alphabet
// required by PKCE (RFC 7636) and by state/verifier generation.
function base64url(buffer) {
    const bytes = new Uint8Array(buffer);
    let binary = '';
    for (let i = 0; i < bytes.length; i++) {
        binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary)
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/, '');
}

/**
 * discover fetches the server's OAuth metadata document. Returns null if
 * OAuth is not configured (404), or if the request fails for any other
 * reason (network error, non-JSON body) -- both are treated as "OAuth
 * absent" so the caller can fall back to the username/password form.
 * @param {typeof fetch} fetchImpl - fetch implementation (for testing)
 * @returns {Promise<object|null>} - metadata object, or null
 */
export async function discover(fetchImpl = fetch) {
    try {
        const response = await fetchImpl(METADATA_PATH);
        if (!response.ok) {
            return null;
        }
        const meta = await response.json();
        if (!meta || typeof meta !== 'object' || !meta.authorization_endpoint || !meta.token_endpoint) {
            return null;
        }
        return meta;
    } catch {
        return null;
    }
}

/**
 * generatePkce creates a fresh PKCE verifier/challenge pair (RFC 7636,
 * S256 method): a random 32-byte verifier, and its SHA-256 digest,
 * both base64url-encoded.
 * @returns {Promise<{verifier: string, challenge: string}>}
 */
export async function generatePkce() {
    const verifierBytes = new Uint8Array(32);
    crypto.getRandomValues(verifierBytes);
    const verifier = base64url(verifierBytes);

    const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier));
    const challenge = base64url(digest);

    return { verifier, challenge };
}

/**
 * randomState returns a random, unguessable state value to bind the
 * authorization request to the callback that eventually arrives.
 * @returns {string}
 */
export function randomState() {
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    return base64url(bytes);
}

/**
 * ensureClient returns a client_id for the given issuer, registering a
 * new dynamic client only if none is cached for that issuer yet.
 * @param {object} meta - OAuth metadata (from discover())
 * @param {typeof fetch} fetchImpl - fetch implementation (for testing)
 * @returns {Promise<string>} - client_id
 */
export async function ensureClient(meta, fetchImpl = fetch) {
    const cached = JSON.parse(localStorage.getItem(STORAGE_CLIENT) || 'null');
    if (cached && cached.issuer === meta.issuer && cached.clientId) {
        return cached.clientId;
    }

    const redirectUri = window.location.origin + CALLBACK_PATH;
    const response = await fetchImpl(toPath(meta.registration_endpoint), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            redirect_uris: [redirectUri],
            client_name: 'pgEdge web client',
            token_endpoint_auth_method: 'none',
            grant_types: ['authorization_code', 'refresh_token'],
            response_types: ['code'],
        }),
    });

    if (!response.ok) {
        throw new Error('Failed to register OAuth client');
    }

    const body = await response.json();
    localStorage.setItem(STORAGE_CLIENT, JSON.stringify({ clientId: body.client_id, issuer: meta.issuer }));
    return body.client_id;
}

/**
 * buildAuthorizeUrl builds the absolute authorization endpoint URL the
 * browser should be redirected to.
 * @param {object} meta - OAuth metadata (from discover())
 * @param {string} clientId
 * @param {string} challenge - PKCE code_challenge
 * @param {string} state
 * @param {string} redirectUri
 * @returns {string}
 */
export function buildAuthorizeUrl(meta, clientId, challenge, state, redirectUri) {
    const url = new URL(meta.authorization_endpoint);
    url.searchParams.set('response_type', 'code');
    url.searchParams.set('client_id', clientId);
    url.searchParams.set('redirect_uri', redirectUri);
    url.searchParams.set('state', state);
    url.searchParams.set('scope', 'mcp');
    url.searchParams.set('code_challenge', challenge);
    url.searchParams.set('code_challenge_method', 'S256');
    return url.toString();
}

// postForm posts an application/x-www-form-urlencoded body to a
// same-origin path and returns the parsed JSON body, raising a
// descriptive error if the server rejects the request.
async function postForm(fetchImpl, path, params) {
    const response = await fetchImpl(path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams(params).toString(),
    });

    const body = await response.json().catch(() => null);

    if (!response.ok) {
        const description = (body && (body.error_description || body.error)) || 'OAuth request failed';
        throw new Error(description);
    }

    return body;
}

// toSession converts a token endpoint response into the session shape
// this module persists, computing an absolute expiry from the relative
// expires_in the server returns.
function toSession(tokenResponse) {
    return {
        accessToken: tokenResponse.access_token,
        refreshToken: tokenResponse.refresh_token,
        expiresAt: Date.now() + (tokenResponse.expires_in || 0) * 1000,
    };
}

/**
 * exchangeCode exchanges an authorization code for a token, verifying
 * the PKCE code_verifier the server stored the challenge against.
 * @param {object} meta - OAuth metadata (from discover())
 * @param {string} clientId
 * @param {string} code
 * @param {string} verifier - PKCE code_verifier
 * @param {string} redirectUri
 * @param {typeof fetch} fetchImpl - fetch implementation (for testing)
 * @returns {Promise<object>} - session {accessToken, refreshToken, expiresAt}
 */
export async function exchangeCode(meta, clientId, code, verifier, redirectUri, fetchImpl = fetch) {
    const body = await postForm(fetchImpl, toPath(meta.token_endpoint), {
        grant_type: 'authorization_code',
        code,
        client_id: clientId,
        redirect_uri: redirectUri,
        code_verifier: verifier,
    });

    return toSession(body);
}

/**
 * refresh exchanges a refresh token for a new access token.
 * @param {object} meta - OAuth metadata (from discover())
 * @param {string} clientId
 * @param {object} session - current session, holding refreshToken
 * @param {typeof fetch} fetchImpl - fetch implementation (for testing)
 * @returns {Promise<object>} - new session
 */
export async function refresh(meta, clientId, session, fetchImpl = fetch) {
    const body = await postForm(fetchImpl, toPath(meta.token_endpoint), {
        grant_type: 'refresh_token',
        refresh_token: session.refreshToken,
        client_id: clientId,
    });

    const next = toSession(body);
    // Some servers omit refresh_token on renewal, meaning the existing
    // one stays valid; keep it rather than dropping it.
    if (!next.refreshToken) {
        next.refreshToken = session.refreshToken;
    }
    return next;
}

/**
 * revoke asks the server to invalidate the session's refresh token. The
 * revocation endpoint always returns 200 per RFC 7009, so this never
 * throws for a rejected token, only for a network failure -- and even
 * then, callers should treat it as best-effort. Bounded by
 * REVOKE_TIMEOUT_MS, so a hung server cannot hold a caller open
 * indefinitely; callers that must not be delayed at all should not
 * await this and should catch any rejection themselves regardless.
 * @param {object} meta - OAuth metadata (from discover())
 * @param {object} session - session holding refreshToken
 * @param {typeof fetch} fetchImpl - fetch implementation (for testing)
 * @returns {Promise<void>}
 */
export async function revoke(meta, session, fetchImpl = fetch) {
    if (!session || !session.refreshToken) {
        return;
    }
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), REVOKE_TIMEOUT_MS);
    try {
        await fetchImpl(toPath(meta.revocation_endpoint), {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: new URLSearchParams({ token: session.refreshToken }).toString(),
            signal: controller.signal,
        });
    } catch {
        // Best-effort: the local session is cleared regardless.
    } finally {
        clearTimeout(timeoutId);
    }
}

/**
 * loadSession reads the persisted OAuth session, if any.
 * @returns {object|null}
 */
export function loadSession() {
    try {
        return JSON.parse(localStorage.getItem(STORAGE_SESSION) || 'null');
    } catch {
        return null;
    }
}

/**
 * saveSession persists the OAuth session.
 * @param {object} session
 */
export function saveSession(session) {
    localStorage.setItem(STORAGE_SESSION, JSON.stringify(session));
}

/**
 * clearSession removes the persisted OAuth session.
 */
export function clearSession() {
    localStorage.removeItem(STORAGE_SESSION);
}
