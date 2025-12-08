/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - DatabaseContext
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { createContext, useContext } from 'react';
import { useDatabases } from '../hooks/useDatabases';
import { useAuth } from './AuthContext';

const DatabaseContext = createContext(null);

/**
 * Provider component that wraps the app and provides database state
 */
export const DatabaseProvider = ({ children }) => {
    const { sessionToken } = useAuth();
    const databaseState = useDatabases(sessionToken);

    return (
        <DatabaseContext.Provider value={databaseState}>
            {children}
        </DatabaseContext.Provider>
    );
};

/**
 * Hook to access database context
 * @returns {Object} Database state and functions
 */
export const useDatabaseContext = () => {
    const context = useContext(DatabaseContext);
    if (!context) {
        throw new Error('useDatabaseContext must be used within a DatabaseProvider');
    }
    return context;
};

export default DatabaseContext;
