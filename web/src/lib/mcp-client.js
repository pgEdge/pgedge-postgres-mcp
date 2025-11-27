/*-------------------------------------------------------------------------
 *
 * MCP Client - JSON-RPC communication with MCP server
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

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
    }

    /**
     * Send JSON-RPC request to MCP server
     * @param {string} method - JSON-RPC method name
     * @param {object|null} params - Method parameters
     * @returns {Promise<any>} - Response result
     */
    async sendRequest(method, params = null) {
        this.requestID++;

        const request = {
            jsonrpc: '2.0',
            id: this.requestID,
            method: method,
            params: params || {}
        };

        const headers = {
            'Content-Type': 'application/json'
        };

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
            const errorText = await response.text();
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
     * Initialize MCP connection
     * @returns {Promise<object>} - Initialize result
     */
    async initialize() {
        return await this.sendRequest('initialize', {
            protocolVersion: '2024-11-05',
            capabilities: {},
            clientInfo: {
                name: 'pgedge-nla-web',
                version: '1.0.0-alpha2'
            }
        });
    }

    /**
     * Send initialized notification
     * Note: This is a notification, not a request, so it doesn't expect a response
     */
    async sendInitializedNotification() {
        // For HTTP mode, we still send this as a request (the server handles it)
        await this.sendRequest('notifications/initialized', {});
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

        // Call authenticate_user tool
        const response = await tempClient.callTool('authenticate_user', {
            username: username,
            password: password
        });

        // Parse result
        if (!response.content || response.content.length === 0) {
            throw new Error('Invalid credentials');
        }

        // The response content is an array of content items
        const contentItem = response.content[0];

        // Parse JSON from text content
        const authResult = JSON.parse(contentItem.text);

        if (!authResult.success || !authResult.session_token) {
            throw new Error(authResult.message || 'Authentication failed');
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
