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
 * Supports prefix-based filtering
 * @returns {Object} Query history state and methods
 */
export const useQueryHistory = () => {
    const [queryHistory, setQueryHistory] = useLocalStorage('query-history', []);
    const [historyIndex, setHistoryIndex] = useState(-1);
    const [tempInput, setTempInput] = useState('');
    const [searchPrefix, setSearchPrefix] = useState('');

    const addToHistory = (query) => {
        setQueryHistory(prev => [...prev, query]);
        setHistoryIndex(-1);
        setTempInput('');
        setSearchPrefix('');
    };

    // Get filtered history based on search prefix
    const getFilteredHistory = (prefix) => {
        if (!prefix) return queryHistory;
        return queryHistory.filter(query => query.startsWith(prefix));
    };

    const navigateUp = (currentInput) => {
        if (queryHistory.length === 0) return currentInput;

        // If we're starting to navigate, save the current input and use it as the prefix
        if (historyIndex === -1) {
            setTempInput(currentInput);
            setSearchPrefix(currentInput);
        }

        // Use the search prefix from when we started navigating
        const prefix = historyIndex === -1 ? currentInput : searchPrefix;
        const filteredHistory = getFilteredHistory(prefix);

        if (filteredHistory.length === 0) return currentInput;

        // Find the current position in the filtered history
        let currentPos = -1;
        if (historyIndex !== -1) {
            const currentQuery = queryHistory[historyIndex];
            currentPos = filteredHistory.indexOf(currentQuery);
        }

        // Calculate new position (going backwards in filtered history)
        const newPos = currentPos === -1
            ? filteredHistory.length - 1
            : Math.max(0, currentPos - 1);

        // Find the actual index in the full history
        const selectedQuery = filteredHistory[newPos];
        const newIndex = queryHistory.indexOf(selectedQuery);

        setHistoryIndex(newIndex);
        return selectedQuery;
    };

    const navigateDown = (currentInput) => {
        if (historyIndex === -1) return currentInput; // Not navigating history

        // Get filtered history using the saved search prefix
        const filteredHistory = getFilteredHistory(searchPrefix);

        if (filteredHistory.length === 0) {
            // No matches, restore temporary input
            setHistoryIndex(-1);
            setSearchPrefix('');
            const restored = tempInput;
            setTempInput('');
            return restored;
        }

        // Find current position in filtered history
        const currentQuery = queryHistory[historyIndex];
        const currentPos = filteredHistory.indexOf(currentQuery);

        // Calculate new position (going forwards in filtered history)
        const newPos = currentPos + 1;

        if (newPos >= filteredHistory.length) {
            // Reached the end, restore temporary input
            setHistoryIndex(-1);
            setSearchPrefix('');
            const restored = tempInput;
            setTempInput('');
            return restored;
        } else {
            // Find the actual index in the full history
            const selectedQuery = filteredHistory[newPos];
            const newIndex = queryHistory.indexOf(selectedQuery);
            setHistoryIndex(newIndex);
            return selectedQuery;
        }
    };

    const resetNavigation = () => {
        setHistoryIndex(-1);
        setTempInput('');
        setSearchPrefix('');
    };

    const clearHistory = () => {
        setQueryHistory([]);
        setHistoryIndex(-1);
        setTempInput('');
        setSearchPrefix('');
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
