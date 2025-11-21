/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - useQueryHistory Hook
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { useState } from 'react';
import { useLocalStorage } from './useLocalStorage';

/**
 * Custom hook for managing query history with up/down arrow navigation
 * @returns {Object} Query history state and methods
 */
export const useQueryHistory = () => {
    const [queryHistory, setQueryHistory] = useLocalStorage('query-history', []);
    const [historyIndex, setHistoryIndex] = useState(-1);
    const [tempInput, setTempInput] = useState('');

    const addToHistory = (query) => {
        setQueryHistory(prev => [...prev, query]);
        setHistoryIndex(-1);
        setTempInput('');
    };

    const navigateUp = (currentInput) => {
        if (queryHistory.length === 0) return currentInput;

        // Save current input if we're starting to navigate history
        if (historyIndex === -1) {
            setTempInput(currentInput);
        }

        // Calculate new index (going backwards in history)
        const newIndex = historyIndex === -1
            ? queryHistory.length - 1
            : Math.max(0, historyIndex - 1);

        setHistoryIndex(newIndex);
        return queryHistory[newIndex];
    };

    const navigateDown = (currentInput) => {
        if (historyIndex === -1) return currentInput; // Not navigating history

        // Calculate new index (going forwards in history)
        const newIndex = historyIndex + 1;

        if (newIndex >= queryHistory.length) {
            // Reached the end, restore temporary input
            setHistoryIndex(-1);
            const restored = tempInput;
            setTempInput('');
            return restored;
        } else {
            setHistoryIndex(newIndex);
            return queryHistory[newIndex];
        }
    };

    const resetNavigation = () => {
        setHistoryIndex(-1);
        setTempInput('');
    };

    const clearHistory = () => {
        setQueryHistory([]);
        setHistoryIndex(-1);
        setTempInput('');
    };

    return {
        queryHistory,
        addToHistory,
        navigateUp,
        navigateDown,
        resetNavigation,
        clearHistory,
        isNavigating: historyIndex !== -1,
    };
};
