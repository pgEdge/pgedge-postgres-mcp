/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - useDatabases Hook
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { useState, useCallback } from 'react';
import { useLocalStorageString } from './useLocalStorage';

const API_BASE_URL = '/api';

/**
 * Custom hook for managing database connections
 * @param {string} sessionToken - The current session token
 * @returns {Object} - Database management state and functions
 */
export const useDatabases = (sessionToken) => {
    const [databases, setDatabases] = useState([]);
    const [currentDatabase, setCurrentDatabase] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);
    const [selectedDatabase, setSelectedDatabase] =
        useLocalStorageString('selected-database', '');

    /**
     * Fetch the list of available databases from the server
     */
    const fetchDatabases = useCallback(async () => {
        if (!sessionToken) {
            setError('No session token available');
            return;
        }

        setLoading(true);
        setError(null);

        try {
            const response = await fetch(`${API_BASE_URL}/databases`, {
                method: 'GET',
                headers: {
                    'Authorization': `Bearer ${sessionToken}`,
                    'Content-Type': 'application/json',
                },
            });

            if (!response.ok) {
                const text = await response.text();
                throw new Error(text || `HTTP ${response.status}`);
            }

            const data = await response.json();
            setDatabases(data.databases || []);
            setCurrentDatabase(data.current || '');

            // Update selectedDatabase if it doesn't match current
            if (data.current && selectedDatabase !== data.current) {
                setSelectedDatabase(data.current);
            }
        } catch (err) {
            console.error('Failed to fetch databases:', err);
            setError(err.message || 'Failed to fetch databases');
        } finally {
            setLoading(false);
        }
    }, [sessionToken, selectedDatabase, setSelectedDatabase]);

    /**
     * Select a database connection
     * @param {string} name - The database name to select
     * @returns {Promise<boolean>} - True if selection was successful
     */
    const selectDatabase = useCallback(async (name) => {
        if (!sessionToken) {
            setError('No session token available');
            return false;
        }

        setLoading(true);
        setError(null);

        try {
            const response = await fetch(`${API_BASE_URL}/databases/select`, {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${sessionToken}`,
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ name }),
            });

            const data = await response.json();

            if (!data.success) {
                throw new Error(data.error || 'Failed to select database');
            }

            setCurrentDatabase(data.current || name);
            setSelectedDatabase(data.current || name);
            return true;
        } catch (err) {
            console.error('Failed to select database:', err);
            setError(err.message || 'Failed to select database');
            return false;
        } finally {
            setLoading(false);
        }
    }, [sessionToken, setSelectedDatabase]);

    /**
     * Clear the error state
     */
    const clearError = useCallback(() => {
        setError(null);
    }, []);

    return {
        databases,
        currentDatabase,
        selectedDatabase,
        loading,
        error,
        fetchDatabases,
        selectDatabase,
        clearError,
    };
};

export default useDatabases;
