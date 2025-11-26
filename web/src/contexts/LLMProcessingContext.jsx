/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - LLM Processing Context
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Context to share LLM processing state across components.
 * Used to disable UI elements (like database selector) during LLM queries.
 *
 *-------------------------------------------------------------------------
 */

import React, { createContext, useContext, useState, useMemo } from 'react';
import PropTypes from 'prop-types';

const LLMProcessingContext = createContext(null);

export const LLMProcessingProvider = ({ children }) => {
    const [isProcessing, setIsProcessing] = useState(false);

    const value = useMemo(() => ({
        isProcessing,
        setIsProcessing,
    }), [isProcessing]);

    return (
        <LLMProcessingContext.Provider value={value}>
            {children}
        </LLMProcessingContext.Provider>
    );
};

LLMProcessingProvider.propTypes = {
    children: PropTypes.node.isRequired,
};

export const useLLMProcessing = () => {
    const context = useContext(LLMProcessingContext);
    if (!context) {
        throw new Error('useLLMProcessing must be used within an LLMProcessingProvider');
    }
    return context;
};

export default LLMProcessingContext;
