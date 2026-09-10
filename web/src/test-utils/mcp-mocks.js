/*-------------------------------------------------------------------------
 *
 * MCP Test Utilities - Mock helpers for MCP JSON-RPC responses
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

/**
 * Create a successful MCP JSON-RPC response
 * @param {number} id - Request ID
 * @param {object} result - Result object
 * @returns {object} - Mock fetch response
 */
export function createMCPResponse(id, result) {
    return {
        ok: true,
        json: async () => ({
            jsonrpc: '2.0',
            id: id,
            result: result
        })
    };
}

/**
 * Create an MCP JSON-RPC error response
 * @param {number} id - Request ID
 * @param {number} code - Error code
 * @param {string} message - Error message
 * @param {string} data - Additional error data
 * @returns {object} - Mock fetch response
 */
export function createMCPError(id, code, message, data = null) {
    return {
        ok: true,
        json: async () => ({
            jsonrpc: '2.0',
            id: id,
            error: {
                code: code,
                message: message,
                data: data
            }
        })
    };
}

/**
 * Create an HTTP error response (not JSON-RPC)
 * @param {number} status - HTTP status code
 * @param {string} text - Error text
 * @returns {object} - Mock fetch response
 */
export function createHTTPError(status, text) {
    return {
        ok: false,
        status: status,
        text: async () => text
    };
}

/**
 * Mock the OAuth discovery request
 * (GET /.well-known/oauth-authorization-server) as absent: a 404, as the
 * server returns when OAuth is not configured. AuthContext calls this
 * unconditionally on mount, ahead of any legacy MCP calls, so tests that
 * queue a fixed sequence of fetch responses need this mocked first.
 * @returns {object} - Mock fetch response
 */
export function mockOAuthAbsent() {
    return createHTTPError(404, 'not found');
}

/**
 * Mock the OAuth discovery request as present, returning a metadata
 * document shaped like GET /.well-known/oauth-authorization-server.
 * @param {object} overrides - Fields to override on the default metadata
 * @returns {object} - Mock fetch response
 */
export function mockOAuthMetadata(overrides = {}) {
    return {
        ok: true,
        status: 200,
        json: async () => ({
            issuer: 'http://localhost:8080',
            authorization_endpoint: 'http://localhost:8080/oauth/authorize',
            token_endpoint: 'http://localhost:8080/oauth/token',
            registration_endpoint: 'http://localhost:8080/oauth/register',
            revocation_endpoint: 'http://localhost:8080/oauth/revoke',
            device_authorization_endpoint: 'http://localhost:8080/oauth/device',
            ...overrides
        })
    };
}

/**
 * Create a mock for the server/discover method (the modern,
 * 2026-07-28 replacement for the legacy initialize handshake). Mirrors
 * the real DiscoverResult shape built by handleDiscoverHTTP in
 * internal/mcp/http_server.go (resultType, ttlMs, cacheScope,
 * supportedVersions, capabilities, and _meta.serverInfo).
 * @param {number} id - Request ID
 * @returns {object} - Mock fetch response
 */
export function mockDiscover(id = 1) {
    return createMCPResponse(id, {
        resultType: 'complete',
        ttlMs: 86400000,
        cacheScope: 'public',
        supportedVersions: ['2026-07-28'],
        capabilities: {
            tools: {}
        },
        _meta: {
            'io.modelcontextprotocol/serverInfo': {
                name: 'pgedge-postgres-mcp',
                version: '1.0.0-alpha2'
            }
        }
    });
}

/**
 * Create a mock for tools/list method
 * @param {number} id - Request ID
 * @param {Array} tools - Array of tool objects
 * @returns {object} - Mock fetch response
 */
export function mockListTools(id = 2, tools = []) {
    return createMCPResponse(id, {
        tools: tools
    });
}

/**
 * Create a mock for successful authentication
 * @param {number} id - Request ID
 * @param {string} username - Username
 * @param {string} sessionToken - Session token
 * @param {string} expiresAt - Expiration timestamp
 * @returns {object} - Mock fetch response
 */
export function mockAuthenticateSuccess(id, username, sessionToken = 'test-session-token', expiresAt = null) {
    const authResult = {
        success: true,
        session_token: sessionToken,
        username: username,
        message: 'Login successful'
    };

    if (expiresAt) {
        authResult.expires_at = expiresAt;
    }

    return createMCPResponse(id, {
        content: [{
            type: 'text',
            text: JSON.stringify(authResult)
        }]
    });
}

/**
 * Create a mock for failed authentication
 * @param {number} id - Request ID
 * @param {string} errorMessage - Error message
 * @returns {object} - Mock fetch response
 */
export function mockAuthenticateFailure(id, errorMessage = 'Invalid credentials') {
    return createMCPError(id, -32000, 'Tool execution failed', errorMessage);
}

/**
 * Create a mock for user info endpoint (REST API)
 * @param {string} username - Username
 * @returns {object} - Mock fetch response
 */
export function mockUserInfo(username) {
    return {
        ok: true,
        json: async () => ({
            username: username
        })
    };
}
