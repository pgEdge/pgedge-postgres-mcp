/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - useLocalStorage Hook
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { useState, useEffect } from 'react';

/**
 * Custom hook for managing localStorage with React state
 * @param {string} key - The localStorage key
 * @param {*} initialValue - Initial value if key doesn't exist
 * @returns {[*, Function]} - [storedValue, setValue]
 */
export const useLocalStorage = (key, initialValue) => {
    // State to store our value
    // Pass initial state function to useState so logic is only executed once
    const [storedValue, setStoredValue] = useState(() => {
        try {
            // Get from local storage by key
            const item = window.localStorage.getItem(key);
            // Parse stored json or if none return initialValue
            return item ? JSON.parse(item) : initialValue;
        } catch (error) {
            // If error also return initialValue
            console.error(`Error loading ${key} from localStorage:`, error);
            return initialValue;
        }
    });

    // Return a wrapped version of useState's setter function that ...
    // ... persists the new value to localStorage.
    const setValue = (value) => {
        try {
            // Allow value to be a function so we have same API as useState
            const valueToStore = value instanceof Function ? value(storedValue) : value;
            // Save state
            setStoredValue(valueToStore);
            // Save to local storage
            window.localStorage.setItem(key, JSON.stringify(valueToStore));
        } catch (error) {
            // A more advanced implementation would handle the error case
            console.error(`Error saving ${key} to localStorage:`, error);
        }
    };

    return [storedValue, setValue];
};

/**
 * Custom hook for managing localStorage string values with React state
 * @param {string} key - The localStorage key
 * @param {string} initialValue - Initial value if key doesn't exist
 * @returns {[string, Function]} - [storedValue, setValue]
 */
export const useLocalStorageString = (key, initialValue) => {
    const [storedValue, setStoredValue] = useState(() => {
        try {
            const item = window.localStorage.getItem(key);
            return item !== null ? item : initialValue;
        } catch (error) {
            console.error(`Error loading ${key} from localStorage:`, error);
            return initialValue;
        }
    });

    const setValue = (value) => {
        try {
            const valueToStore = value instanceof Function ? value(storedValue) : value;
            setStoredValue(valueToStore);
            if (valueToStore !== null && valueToStore !== undefined) {
                window.localStorage.setItem(key, valueToStore);
            } else {
                window.localStorage.removeItem(key);
            }
        } catch (error) {
            console.error(`Error saving ${key} to localStorage:`, error);
        }
    };

    return [storedValue, setValue];
};

/**
 * Custom hook for managing localStorage boolean values with React state
 * @param {string} key - The localStorage key
 * @param {boolean} initialValue - Initial value if key doesn't exist
 * @returns {[boolean, Function]} - [storedValue, setValue]
 */
export const useLocalStorageBoolean = (key, initialValue) => {
    const [storedValue, setStoredValue] = useState(() => {
        try {
            const item = window.localStorage.getItem(key);
            return item === null ? initialValue : item === 'true';
        } catch (error) {
            console.error(`Error loading ${key} from localStorage:`, error);
            return initialValue;
        }
    });

    const setValue = (value) => {
        try {
            const valueToStore = value instanceof Function ? value(storedValue) : value;
            setStoredValue(valueToStore);
            window.localStorage.setItem(key, valueToStore.toString());
        } catch (error) {
            console.error(`Error saving ${key} to localStorage:`, error);
        }
    };

    return [storedValue, setValue];
};
