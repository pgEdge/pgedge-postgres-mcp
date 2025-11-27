/*-------------------------------------------------------------------------
 *
 * MCP Test Utilities - Mock helpers for MCP JSON-RPC responses
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
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
 * Create a mock for initialize method
 * @param {number} id - Request ID
 * @returns {object} - Mock fetch response
 */
export function mockInitialize(id = 1) {
    return createMCPResponse(id, {
        protocolVersion: '2024-11-05',
        capabilities: {
            tools: {}
        },
        serverInfo: {
            name: 'pgedge-nla-server',
            version: '1.0.0-alpha2'
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
