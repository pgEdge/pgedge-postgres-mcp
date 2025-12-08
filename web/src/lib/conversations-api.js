/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Conversations API
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

/**
 * Conversations API client for managing conversation history
 */
export class ConversationsAPI {
    /**
     * Create a new Conversations API client
     * @param {string} sessionToken - Authentication session token
     */
    constructor(sessionToken) {
        this.sessionToken = sessionToken;
        this.baseURL = '/api/conversations';
    }

    /**
     * Update the session token
     * @param {string} token - New session token
     */
    setToken(token) {
        this.sessionToken = token;
    }

    /**
     * Make an authenticated request
     * @param {string} endpoint - API endpoint
     * @param {object} options - Fetch options
     * @returns {Promise<Response>}
     */
    async request(endpoint, options = {}) {
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers,
        };

        if (this.sessionToken) {
            headers['Authorization'] = `Bearer ${this.sessionToken}`;
        }

        const response = await fetch(`${this.baseURL}${endpoint}`, {
            ...options,
            headers,
        });

        return response;
    }

    /**
     * List all conversations for the current user
     * @param {object} options - Pagination options
     * @param {number} options.limit - Maximum number of conversations to return
     * @param {number} options.offset - Number of conversations to skip
     * @returns {Promise<object>} - List of conversation summaries
     */
    async list({ limit = 50, offset = 0 } = {}) {
        const params = new URLSearchParams();
        if (limit) params.set('limit', limit.toString());
        if (offset) params.set('offset', offset.toString());

        const queryString = params.toString();
        const endpoint = queryString ? `?${queryString}` : '';

        const response = await this.request(endpoint, { method: 'GET' });

        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            throw new Error(error.error || `Failed to list conversations: ${response.status}`);
        }

        return response.json();
    }

    /**
     * Get a specific conversation by ID
     * @param {string} id - Conversation ID
     * @returns {Promise<object>} - Full conversation with messages
     */
    async get(id) {
        const response = await this.request(`/${id}`, { method: 'GET' });

        if (!response.ok) {
            if (response.status === 404) {
                throw new Error('Conversation not found');
            }
            const error = await response.json().catch(() => ({}));
            throw new Error(error.error || `Failed to get conversation: ${response.status}`);
        }

        return response.json();
    }

    /**
     * Create a new conversation
     * @param {Array} messages - Array of message objects
     * @param {string} provider - LLM provider name
     * @param {string} model - Model name
     * @param {string} connection - Database connection name
     * @returns {Promise<object>} - Created conversation
     */
    async create(messages, provider = '', model = '', connection = '') {
        const response = await this.request('', {
            method: 'POST',
            body: JSON.stringify({ messages, provider, model, connection }),
        });

        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            throw new Error(error.error || `Failed to create conversation: ${response.status}`);
        }

        return response.json();
    }

    /**
     * Update an existing conversation
     * @param {string} id - Conversation ID
     * @param {Array} messages - Updated array of message objects
     * @param {string} provider - LLM provider name
     * @param {string} model - Model name
     * @param {string} connection - Database connection name
     * @returns {Promise<object>} - Updated conversation
     */
    async update(id, messages, provider = '', model = '', connection = '') {
        const response = await this.request(`/${id}`, {
            method: 'PUT',
            body: JSON.stringify({ messages, provider, model, connection }),
        });

        if (!response.ok) {
            if (response.status === 404) {
                throw new Error('Conversation not found');
            }
            if (response.status === 403) {
                throw new Error('Access denied');
            }
            const error = await response.json().catch(() => ({}));
            throw new Error(error.error || `Failed to update conversation: ${response.status}`);
        }

        return response.json();
    }

    /**
     * Rename a conversation
     * @param {string} id - Conversation ID
     * @param {string} title - New title
     * @returns {Promise<object>} - Result
     */
    async rename(id, title) {
        const response = await this.request(`/${id}`, {
            method: 'PATCH',
            body: JSON.stringify({ title }),
        });

        if (!response.ok) {
            if (response.status === 404) {
                throw new Error('Conversation not found');
            }
            if (response.status === 403) {
                throw new Error('Access denied');
            }
            const error = await response.json().catch(() => ({}));
            throw new Error(error.error || `Failed to rename conversation: ${response.status}`);
        }

        return response.json();
    }

    /**
     * Delete a conversation
     * @param {string} id - Conversation ID
     * @returns {Promise<void>}
     */
    async delete(id) {
        const response = await this.request(`/${id}`, { method: 'DELETE' });

        if (!response.ok) {
            if (response.status === 404) {
                throw new Error('Conversation not found');
            }
            const error = await response.json().catch(() => ({}));
            throw new Error(error.error || `Failed to delete conversation: ${response.status}`);
        }
    }

    /**
     * Delete all conversations for the current user
     * @returns {Promise<object>} - Result with number of deleted conversations
     */
    async deleteAll() {
        const response = await this.request('?all=true', { method: 'DELETE' });

        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            throw new Error(error.error || `Failed to delete conversations: ${response.status}`);
        }

        return response.json();
    }
}

export default ConversationsAPI;
