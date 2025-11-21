/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - useMCPClient Hook
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { useState, useEffect } from 'react';
import { MCPClient } from '../lib/mcp-client';

/**
 * Custom hook for managing MCP client connection and tools
 * @param {string} sessionToken - Authentication session token
 * @returns {Object} MCP client state and methods
 */
export const useMCPClient = (sessionToken) => {
    const [mcpClient, setMcpClient] = useState(null);
    const [tools, setTools] = useState([]);
    const [error, setError] = useState(null);
    const [loading, setLoading] = useState(false);

    useEffect(() => {
        if (!sessionToken) {
            console.log('No session token available, skipping MCP client initialization');
            return;
        }

        const initializeMCP = async () => {
            setLoading(true);
            setError(null);

            try {
                console.log('Initializing MCP client...');
                const client = new MCPClient('/mcp/v1', sessionToken);

                // Initialize the client
                await client.initialize();

                // Fetch available tools
                console.log('Fetching MCP tools...');
                const toolsList = await client.listTools();
                console.log('MCP tools loaded:', toolsList);

                setMcpClient(client);
                setTools(toolsList);
            } catch (err) {
                console.error('Error initializing MCP client:', err);
                setError('Failed to initialize MCP tools. Please check browser console.');
            } finally {
                setLoading(false);
            }
        };

        initializeMCP();
    }, [sessionToken]);

    const refreshTools = async () => {
        if (!mcpClient) return;

        try {
            const toolsList = await mcpClient.listTools();
            setTools(toolsList);
        } catch (err) {
            console.error('Error refreshing tools:', err);
        }
    };

    return {
        mcpClient,
        tools,
        error,
        loading,
        refreshTools,
    };
};
