/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { createContext, useState, useContext, useEffect, useRef } from 'react';
import { MCPClient } from '../lib/mcp-client';
import {
    discover,
    ensureClient,
    registerClient,
    generatePkce,
    randomState,
    buildAuthorizeUrl,
    exchangeCode,
    refresh as refreshOAuthSession,
    revoke as revokeOAuthSession,
    loadSession,
    saveSession,
    clearSession,
    CALLBACK_PATH,
    STORAGE_PKCE,
} from '../lib/oauth';

const AuthContext = createContext(null);

// MCP server URL (proxied through nginx in production, direct in development)
const MCP_SERVER_URL = '/mcp/v1';

// localStorage key for the legacy username/password session token, kept
// for servers that do not advertise OAuth.
const LEGACY_TOKEN_KEY = 'mcp-session-token';

// How long before expiry the access token is proactively refreshed.
const REFRESH_MARGIN_MS = 60 * 1000;

export const AuthProvider = ({ children }) => {
    const [user, setUser] = useState(null);
    const [legacyToken, setLegacyToken] = useState(() => localStorage.getItem(LEGACY_TOKEN_KEY));
    const [oauthSession, setOauthSession] = useState(() => loadSession());
    const [oauth, setOauth] = useState({ enabled: false, meta: null });
    const [loading, setLoading] = useState(true);
    const [authError, setAuthError] = useState('');
    const refreshTimerRef = useRef(null);

    // The access token a reactive (401-triggered) refresh has already
    // been attempted for, so a still-invalid token cannot trigger an
    // unbounded refresh loop: at most one attempt is made per token.
    const refreshAttemptedForRef = useRef(null);

    // The in-flight refresh, if any: { token, promise }. Every refresh
    // goes through it, whether started by the proactive timer or by a
    // 401, so a second caller wanting the same access token renewed
    // awaits the refresh already under way instead of sending the same
    // refresh token twice. The server consumes refresh tokens atomically,
    // so a duplicate request would lose with invalid_grant and could
    // take the whole token family down with it.
    const refreshInFlightRef = useRef(null);

    // Bumped by forceLogout()/logout(); captured before starting any
    // refresh (scheduled or 401-triggered) so a refresh that resolves
    // after a logout has already happened can recognise itself as stale
    // and not resurrect a session the user just left.
    const sessionGenerationRef = useRef(0);

    // The token consumers should use: an OAuth session's access token
    // takes priority over the legacy username/password session token.
    const sessionToken = oauthSession ? oauthSession.accessToken : legacyToken;

    // validateAndSetUser confirms a token still works against the server
    // and, if so, populates `user`. Returns whether the token was valid.
    const validateAndSetUser = async (token) => {
        try {
            const client = new MCPClient(MCP_SERVER_URL, token);
            await client.initialize();
            await client.listTools();

            const response = await fetch('/api/user/info', {
                headers: {
                    'Authorization': `Bearer ${token}`
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
            return true;
        } catch (error) {
            console.error('Auth check failed:', error);
            return false;
        }
    };

    // handleOAuthCallback runs once, on the mount that lands on
    // CALLBACK_PATH: it verifies the returned state against the one
    // stashed before redirecting, exchanges the code for a session, and
    // replaces the URL so a reload does not replay the callback. Returns
    // the new access token on success, or null on any failure (with
    // authError set for the login page to display).
    const handleOAuthCallback = async (meta) => {
        const params = new URLSearchParams(window.location.search);
        const code = params.get('code');
        const state = params.get('state');
        const error = params.get('error');
        const errorDescription = params.get('error_description');

        let pkce = null;
        try {
            pkce = JSON.parse(sessionStorage.getItem(STORAGE_PKCE) || 'null');
        } catch {
            pkce = null;
        }
        sessionStorage.removeItem(STORAGE_PKCE);

        // Replace the callback URL with the app root regardless of outcome,
        // so a reload after a failed sign-in does not replay the callback.
        window.history.replaceState({}, '', '/');

        if (error) {
            setAuthError(errorDescription || error);
            return null;
        }

        if (!pkce || !state || state !== pkce.state) {
            setAuthError('Sign-in failed: the returned state did not match. Please try again.');
            return null;
        }

        try {
            const clientId = await ensureClient(meta);
            const redirectUri = window.location.origin + CALLBACK_PATH;
            const session = await exchangeCode(meta, clientId, code, pkce.verifier, redirectUri);
            saveSession(session);
            setOauthSession(session);
            return session.accessToken;
        } catch (err) {
            setAuthError(err.message || 'Sign-in failed. Please try again.');
            return null;
        }
    };

    useEffect(() => {
        let cancelled = false;

        const run = async () => {
            const meta = await discover();
            if (cancelled) {
                return;
            }
            setOauth({ enabled: !!meta, meta });

            let token = null;

            if (meta && window.location.pathname === CALLBACK_PATH) {
                token = await handleOAuthCallback(meta);
                if (cancelled) {
                    return;
                }
            }

            if (!token) {
                let existingOAuthSession = loadSession();

                // A session that expired whilst the tab was closed is
                // not a session to throw away: renew it before deciding
                // whether the user is signed in, and only fall through
                // to clearing it if the renewal itself fails.
                if (meta && existingOAuthSession
                    && existingOAuthSession.refreshToken
                    && existingOAuthSession.expiresAt <= Date.now()) {
                    try {
                        const clientId = await ensureClient(meta);
                        const next = await refreshOAuthSession(meta, clientId, existingOAuthSession);
                        if (cancelled) {
                            return;
                        }
                        saveSession(next);
                        setOauthSession(next);
                        existingOAuthSession = next;
                    } catch (err) {
                        console.error('Stored OAuth session could not be refreshed:', err);
                    }
                }

                token = existingOAuthSession ? existingOAuthSession.accessToken : legacyToken;
            }

            if (!token) {
                setLoading(false);
                return;
            }

            const valid = await validateAndSetUser(token);
            if (cancelled) {
                return;
            }

            if (!valid) {
                // Invalid or expired session - clear whichever kind it was.
                if (loadSession()) {
                    clearSession();
                    setOauthSession(null);
                } else {
                    localStorage.removeItem(LEGACY_TOKEN_KEY);
                    setLegacyToken(null);
                }
                setUser(null);
            }

            setLoading(false);
        };

        run();

        return () => {
            cancelled = true;
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    // startRefresh renews the given session's access token, and is the
    // single path every refresh takes: the proactive timer below and the
    // 401 handler further down both call it, so only one refresh for a
    // given access token is ever in flight, and a caller arriving whilst
    // one is under way joins it rather than sending the same refresh
    // token a second time. Resolves true if the session was renewed and
    // persisted, and false if the refresh failed (in which case it has
    // already forced a logout) or if its result arrived too late to be
    // used. Never rejects, so a caller that does not care about the
    // outcome can ignore the returned promise.
    const startRefresh = (session) => {
        const token = session.accessToken;

        const existing = refreshInFlightRef.current;
        if (existing && existing.token === token) {
            return existing.promise;
        }

        const generation = sessionGenerationRef.current;
        const entry = { token, promise: null };

        entry.promise = (async () => {
            // Registered before the first await, so a caller that
            // arrives whilst ensureClient() is still going to the
            // network joins this refresh instead of starting another.
            refreshInFlightRef.current = entry;
            try {
                const clientId = await ensureClient(oauth.meta);
                const next = await refreshOAuthSession(oauth.meta, clientId, session);
                if (sessionGenerationRef.current !== generation) {
                    // A logout happened while this refresh was in
                    // flight; the result is stale and must not
                    // resurrect a session the user already left.
                    return false;
                }
                saveSession(next);
                setOauthSession(next);
                return true;
            } catch (err) {
                console.error('Token refresh failed:', err);
                if (sessionGenerationRef.current === generation) {
                    forceLogout();
                }
                return false;
            } finally {
                // Only clear our own entry: a later refresh may already
                // have replaced it.
                if (refreshInFlightRef.current === entry) {
                    refreshInFlightRef.current = null;
                }
            }
        })();

        return entry.promise;
    };

    // Schedule a proactive token refresh shortly before the current
    // access token expires. Cleared on unmount and whenever the
    // scheduling inputs change, so at most one timer is ever pending.
    // The delay can be zero for an already-expiring token, so the timer
    // may well fire alongside a 401 for the same token; both go through
    // startRefresh, which coalesces them into one request.
    useEffect(() => {
        if (!oauth.enabled || !oauth.meta || !oauthSession) {
            return undefined;
        }

        const delay = Math.max(0, oauthSession.expiresAt - Date.now() - REFRESH_MARGIN_MS);

        refreshTimerRef.current = setTimeout(() => {
            startRefresh(oauthSession);
        }, delay);

        return () => {
            if (refreshTimerRef.current) {
                clearTimeout(refreshTimerRef.current);
                refreshTimerRef.current = null;
            }
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [oauth.enabled, oauth.meta, oauthSession]);

    const login = async (username, password) => {
        try {
            // Authenticate via JSON-RPC authenticate_user tool
            const authResult = await MCPClient.authenticate(MCP_SERVER_URL, username, password);

            // Store session token in state and localStorage
            setLegacyToken(authResult.sessionToken);
            localStorage.setItem(LEGACY_TOKEN_KEY, authResult.sessionToken);

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

    // startOAuthLogin registers a dynamic client if needed, generates a
    // fresh PKCE pair and state, stashes them for the callback to verify,
    // and sends the browser to the server's own login page.
    const startOAuthLogin = async () => {
        if (!oauth.meta) {
            return;
        }
        try {
            // Always register afresh: the cached client id may name a
            // client the server has since forgotten.
            const clientId = await registerClient(oauth.meta);
            const { verifier, challenge } = await generatePkce();
            const state = randomState();
            sessionStorage.setItem(STORAGE_PKCE, JSON.stringify({ verifier, state }));

            const redirectUri = window.location.origin + CALLBACK_PATH;
            const authorizeUrl = buildAuthorizeUrl(oauth.meta, clientId, challenge, state, redirectUri);
            window.location.assign(authorizeUrl);
        } catch (err) {
            setAuthError(err.message || 'Unable to start sign-in. Please try again.');
        }
    };

    const logout = async () => {
        if (refreshTimerRef.current) {
            clearTimeout(refreshTimerRef.current);
            refreshTimerRef.current = null;
        }

        refreshAttemptedForRef.current = null;
        sessionGenerationRef.current += 1;

        const sessionToRevoke = oauth.enabled ? oauthSession : null;
        const metaForRevoke = oauth.meta;

        // Clear all local state and storage first, unconditionally: the
        // user must see themselves logged out immediately, regardless of
        // whether the server is slow, unreachable, or hung.
        clearSession();
        setOauthSession(null);

        setLegacyToken(null);
        localStorage.removeItem(LEGACY_TOKEN_KEY);

        setUser(null);

        if (sessionToRevoke) {
            // Best-effort and fire-and-forget: never await this, so a
            // slow or hung server cannot delay the UI or leave the user
            // looking logged in. revoke() itself is bounded by
            // REVOKE_TIMEOUT_MS; the catch here is just for the
            // vanishingly unlikely case it rejects outside that.
            revokeOAuthSession(metaForRevoke, sessionToRevoke).catch(() => {});
        }

        // Note: for the legacy flow, we don't need to call a logout API -
        // the session token simply expires on the server after its TTL.
    };

    // Force logout without any cleanup (used when a session is invalidated,
    // e.g. a refresh failure) - clears state immediately, without waiting
    // on a revocation round trip.
    const forceLogout = () => {
        if (refreshTimerRef.current) {
            clearTimeout(refreshTimerRef.current);
            refreshTimerRef.current = null;
        }

        refreshAttemptedForRef.current = null;
        sessionGenerationRef.current += 1;

        clearSession();
        setOauthSession(null);

        setLegacyToken(null);
        localStorage.removeItem(LEGACY_TOKEN_KEY);

        setUser(null);
    };

    // handleUnauthorized responds to a 401 from an MCP call: with an
    // OAuth session, it attempts one refresh (never more than one per
    // access token, so a token that keeps coming back invalid cannot
    // loop) and reports whether the caller can retry with the refreshed
    // token; without one -- or once a refresh for this token has
    // already been tried and is no longer in flight, or the refresh
    // itself fails -- it forces a logout and reports that the caller
    // should give up. For the legacy username/password flow, this is
    // exactly what forceLogout always did.
    //
    // Two 401s for the same token arriving close together (e.g. two
    // requests in flight when the token expired) are the common case,
    // not a sign the token is unrecoverable: the second caller shares
    // the first's in-flight refresh instead of starting a duplicate
    // request or being logged out from under the first. The same applies
    // to a 401 that races the proactive refresh timer, since both paths
    // share startRefresh.
    const handleUnauthorized = async () => {
        if (oauth.enabled && oauthSession) {
            const token = oauthSession.accessToken;

            // Joining an in-flight refresh (started by another 401 or
            // by the proactive timer) is checked before the
            // single-attempt rule below, so sharing a refresh never
            // counts as a second attempt for this token.
            if (refreshInFlightRef.current && refreshInFlightRef.current.token === token) {
                return refreshInFlightRef.current.promise;
            }

            if (refreshAttemptedForRef.current === token) {
                forceLogout();
                return false;
            }
            refreshAttemptedForRef.current = token;

            return startRefresh(oauthSession);
        }

        forceLogout();
        return false;
    };

    return (
        <AuthContext.Provider value={{
            user,
            sessionToken,
            loading,
            login,
            logout,
            forceLogout,
            handleUnauthorized,
            oauthEnabled: oauth.enabled,
            startOAuthLogin,
            authError,
        }}>
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
