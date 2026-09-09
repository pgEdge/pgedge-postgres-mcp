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
                const existingOAuthSession = loadSession();
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

    // Schedule a proactive token refresh shortly before the current
    // access token expires. Cleared on unmount and whenever the
    // scheduling inputs change, so at most one timer is ever pending.
    useEffect(() => {
        if (!oauth.enabled || !oauth.meta || !oauthSession) {
            return undefined;
        }

        const delay = Math.max(0, oauthSession.expiresAt - Date.now() - REFRESH_MARGIN_MS);

        refreshTimerRef.current = setTimeout(async () => {
            try {
                const clientId = await ensureClient(oauth.meta);
                const next = await refreshOAuthSession(oauth.meta, clientId, oauthSession);
                saveSession(next);
                setOauthSession(next);
            } catch (err) {
                console.error('Token refresh failed:', err);
                forceLogout();
            }
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
            const clientId = await ensureClient(oauth.meta);
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

        if (oauth.enabled && oauthSession) {
            // Best-effort: the local session is cleared regardless of
            // whether the server accepted the revocation.
            await revokeOAuthSession(oauth.meta, oauthSession);
        }

        clearSession();
        setOauthSession(null);

        setLegacyToken(null);
        localStorage.removeItem(LEGACY_TOKEN_KEY);

        setUser(null);

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

        clearSession();
        setOauthSession(null);

        setLegacyToken(null);
        localStorage.removeItem(LEGACY_TOKEN_KEY);

        setUser(null);
    };

    return (
        <AuthContext.Provider value={{
            user,
            sessionToken,
            loading,
            login,
            logout,
            forceLogout,
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
