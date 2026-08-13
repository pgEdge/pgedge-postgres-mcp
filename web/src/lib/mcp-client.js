/*-------------------------------------------------------------------------
 *
 * MCP Client - JSON-RPC communication with MCP server
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Web client version, injected from web/package.json by the define in
// vite.config.js and vitest.config.js. It is deliberately not a literal
// here: this constant is shown in the help panel, and as a literal it
// sat at 1.0.0-alpha5 through five subsequent releases because nothing
// tied it to anything that a release already had to touch.
export const CLIENT_VERSION = __APP_VERSION__;

// Protocol revision every request this client sends declares itself
// as speaking. 2026-07-28 removed the connection-scoped initialize
// handshake in favor of a stateless, per-request model (see
// internal/mcp/modern.go on the server side) -- there is no older
// revision to fall back to, since this client only ever talks to this
// project's own server.
export const MODERN_PROTOCOL_VERSION = '2026-07-28';

// The Streamable HTTP transport spec requires an Mcp-Name header,
// mirroring params.name or params.uri, on exactly these methods.
const METHODS_REQUIRING_MCP_NAME = new Set(['tools/call', 'resources/read', 'prompts/get']);

// modernMeta returns the _meta object every modern request must carry.
// clientCapabilities is a plain object literal, not built through any
// helper that might apply JSON.stringify-time omission of an empty
// value -- the server only checks for the key's presence.
function modernMeta() {
    return {
        'io.modelcontextprotocol/protocolVersion': MODERN_PROTOCOL_VERSION,
        'io.modelcontextprotocol/clientCapabilities': {}
    };
}

// nameOrURI extracts the value an Mcp-Name header must mirror from an
// arbitrary request's params.
function nameOrURI(params) {
    if (!params) {
        return '';
    }
    return params.name || params.uri || '';
}

// needsEscaping mirrors needsEscaping in internal/chat/mcp_client.go: it
// reports whether value cannot round-trip safely as a raw HTTP header
// value and must be base64-wrapped instead: any non-printable-ASCII
// character, leading/trailing whitespace (stripped by HTTP header
// parsing on both ends), or a value that already looks like the base64
// sentinel (which decodeHeaderValue in internal/mcp/modern.go would
// otherwise wrongly unwrap).
function needsEscaping(value) {
    if (!/^[\x20-\x7E]*$/.test(value)) {
        return true;
    }
    if (value !== value.trim()) {
        return true;
    }
    const prefix = '=?base64?';
    const suffix = '?=';
    if (value.length >= prefix.length + suffix.length &&
        value.startsWith(prefix) && value.endsWith(suffix)) {
        return true;
    }
    return false;
}

// encodeMcpNameHeader mirrors decodeHeaderValue in internal/mcp/modern.go:
// a value that round-trips safely as a raw HTTP header value is sent
// as-is; anything else is base64-encoded and wrapped in the spec's
// sentinel.
function encodeMcpNameHeader(value) {
    if (!needsEscaping(value)) {
        return value;
    }
    const bytes = new TextEncoder().encode(value);
    const base64 = btoa(String.fromCharCode(...bytes));
    return `=?base64?${base64}?=`;
}

/**
 * MCP Client for communicating with MCP server via JSON-RPC
 * Mirrors the HTTP client implementation in internal/chat/mcp_client.go
 */
export class MCPClient {
    /**
     * Create a new MCP client
     * @param {string} baseURL - Base URL of MCP server (e.g., '/mcp/v1')
     * @param {string|null} token - Session token for authentication (optional)
     */
    constructor(baseURL, token = null) {
        this.baseURL = baseURL;
        this.token = token;
        this.requestID = 0;
        this.serverInfo = null;
    }

    /**
     * Send JSON-RPC request to MCP server
     * @param {string} method - JSON-RPC method name
     * @param {object|null} params - Method parameters
     * @returns {Promise<any>} - Response result
     */
    async sendRequest(method, params = null) {
        this.requestID++;

        const requestParams = { ...(params || {}), _meta: modernMeta() };

        const request = {
            jsonrpc: '2.0',
            id: this.requestID,
            method: method,
            params: requestParams
        };

        const headers = {
            'Content-Type': 'application/json',
            'Accept': 'application/json, text/event-stream',
            'MCP-Protocol-Version': MODERN_PROTOCOL_VERSION,
            'Mcp-Method': method
        };

        if (METHODS_REQUIRING_MCP_NAME.has(method)) {
            headers['Mcp-Name'] = encodeMcpNameHeader(nameOrURI(params));
        }

        // Add Authorization header if token is present
        if (this.token) {
            headers['Authorization'] = `Bearer ${this.token}`;
        }

        const response = await fetch(this.baseURL, {
            method: 'POST',
            headers: headers,
            body: JSON.stringify(request)
        });

        if (!response.ok) {
            // Handle server unavailability errors with user-friendly messages
            if (response.status === 502 || response.status === 503 || response.status === 504) {
                throw new Error(
                    'Unable to connect to the server. ' +
                    'Please ensure the MCP server is running and try again.'
                );
            }

            // Handle other HTTP errors
            const errorText = await response.text();

            // Check if error text looks like HTML (proxy error pages)
            if (errorText.includes('<!DOCTYPE') || errorText.includes('<html')) {
                throw new Error(
                    `Server error (${response.status}). ` +
                    'Please try again or contact support if the issue persists.'
                );
            }

            throw new Error(`HTTP error ${response.status}: ${errorText}`);
        }

        const jsonResp = await response.json();

        if (jsonResp.error) {
            // Extract error message, preferring error.data over error.message
            const errorMessage = jsonResp.error.data || jsonResp.error.message || 'MCP server error';
            throw new Error(`RPC error ${jsonResp.error.code}: ${errorMessage}`);
        }

        return jsonResp.result;
    }

    /**
     * Discover the server's identity and capabilities under the modern
     * (2026-07-28) protocol. Replaces the legacy initialize handshake --
     * 2026-07-28 removed the connection-scoped handshake in favor of a
     * stateless, per-request model, and server/discover is how a client
     * learns server identity/capabilities without one.
     * @returns {Promise<object>} - Discover result
     */
    async initialize() {
        const result = await this.sendRequest('server/discover');

        const serverInfo = result && result._meta &&
            result._meta['io.modelcontextprotocol/serverInfo'];
        if (serverInfo) {
            this.serverInfo = serverInfo;
        }

        return result;
    }

    /**
     * Get server information from initialization
     * @returns {object|null} - Server info object with name and version, or null if not initialized
     */
    getServerInfo() {
        return this.serverInfo;
    }

    /**
     * List available tools
     * @returns {Promise<Array>} - Array of tool objects
     */
    async listTools() {
        const result = await this.sendRequest('tools/list');
        return result.tools || [];
    }

    /**
     * Call a tool
     * @param {string} name - Tool name
     * @param {object} args - Tool arguments
     * @returns {Promise<object>} - Tool response
     */
    async callTool(name, args) {
        return await this.sendRequest('tools/call', {
            name: name,
            arguments: args || {}
        });
    }

    /**
     * List available resources
     * @returns {Promise<Array>} - Array of resource objects
     */
    async listResources() {
        const result = await this.sendRequest('resources/list');
        return result.resources || [];
    }

    /**
     * Read a resource
     * @param {string} uri - Resource URI
     * @returns {Promise<object>} - Resource content
     */
    async readResource(uri) {
        return await this.sendRequest('resources/read', {
            uri: uri
        });
    }

    /**
     * List available prompts
     * @returns {Promise<Array>} - Array of prompt objects
     */
    async listPrompts() {
        const result = await this.sendRequest('prompts/list');
        return result.prompts || [];
    }

    /**
     * Get a prompt with optional arguments
     * @param {string} name - Prompt name
     * @param {object} args - Prompt arguments (key-value pairs)
     * @returns {Promise<object>} - Prompt result with messages
     */
    async getPrompt(name, args = {}) {
        return await this.sendRequest('prompts/get', {
            name: name,
            arguments: args
        });
    }

    /**
     * Authenticate user and get session token
     * Static method - creates temporary client without token to call authenticate_user
     * @param {string} baseURL - Base URL of MCP server
     * @param {string} username - Username
     * @param {string} password - Password
     * @returns {Promise<object>} - Auth result with session_token
     */
    static async authenticate(baseURL, username, password) {
        // Create temporary client without token
        const tempClient = new MCPClient(baseURL, null);

        let response;
        try {
            // Call authenticate_user tool
            response = await tempClient.callTool('authenticate_user', {
                username: username,
                password: password
            });
        } catch (error) {
            // Log original error for debugging
            console.error('Authentication error:', error);

            // Convert technical error messages to user-friendly ones
            const errorMsg = (error?.message ?? String(error)).toLowerCase();

            if (errorMsg.includes('invalid username or password') ||
                errorMsg.includes('invalid credentials') ||
                errorMsg.includes('authentication failed')) {
                throw new Error('Invalid username or password. Please try again.');
            }

            if (errorMsg.includes('account is disabled') ||
                errorMsg.includes('account has been disabled')) {
                throw new Error('Your account has been disabled. Please contact support.');
            }

            if (errorMsg.includes('too many failed') ||
                errorMsg.includes('rate limit')) {
                throw new Error('Too many failed login attempts. Please wait a moment and try again.');
            }

            // Re-throw with a generic message for other errors
            throw new Error('Unable to log in. Please check your connection and try again.');
        }

        // Parse result
        if (!response.content || response.content.length === 0) {
            throw new Error('Invalid username or password. Please try again.');
        }

        // The response content is an array of content items
        const contentItem = response.content[0];

        // Parse JSON from text content
        let authResult;
        try {
            authResult = JSON.parse(contentItem.text);
        } catch {
            throw new Error('Received an unexpected response from the server. Please try again.');
        }

        if (!authResult.success || !authResult.session_token) {
            throw new Error(authResult.message || 'Authentication failed. Please try again.');
        }

        return {
            sessionToken: authResult.session_token,
            username: authResult.username || username,
            expiresAt: authResult.expires_at,
            message: authResult.message
        };
    }

    /**
     * Set the authentication token
     * @param {string} token - Session token
     */
    setToken(token) {
        this.token = token;
    }

    /**
     * Clear the authentication token
     */
    clearToken() {
        this.token = null;
    }
}

export default MCPClient;
