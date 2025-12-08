/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - useConversations Hook
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import { ConversationsAPI } from '../lib/conversations-api';

/**
 * Custom hook for managing conversations
 * @param {string} sessionToken - Authentication session token
 * @returns {object} Conversations state and methods
 */
export const useConversations = (sessionToken) => {
    const [conversations, setConversations] = useState([]);
    const [currentConversationId, setCurrentConversationId] = useState(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);

    // Keep API client in a ref to avoid recreating on every render
    const apiRef = useRef(null);

    // Initialize API client when token changes
    useEffect(() => {
        if (sessionToken) {
            apiRef.current = new ConversationsAPI(sessionToken);
        } else {
            apiRef.current = null;
        }
    }, [sessionToken]);

    // Load conversations list
    const loadConversations = useCallback(async () => {
        if (!apiRef.current) return;

        setLoading(true);
        setError(null);

        try {
            const result = await apiRef.current.list();
            setConversations(result.conversations || []);
        } catch (err) {
            console.error('Failed to load conversations:', err);
            setError(err.message);
        } finally {
            setLoading(false);
        }
    }, []);

    // Load conversations on mount and when token changes
    useEffect(() => {
        if (sessionToken) {
            loadConversations();
        } else {
            setConversations([]);
            setCurrentConversationId(null);
        }
    }, [sessionToken, loadConversations]);

    // Get a specific conversation
    const getConversation = useCallback(async (id) => {
        if (!apiRef.current) return null;

        try {
            return await apiRef.current.get(id);
        } catch (err) {
            console.error('Failed to get conversation:', err);
            setError(err.message);
            return null;
        }
    }, []);

    // Save current conversation (create or update)
    const saveConversation = useCallback(async (messages, conversationId = null, provider = '', model = '', connection = '') => {
        if (!apiRef.current || messages.length === 0) return null;

        try {
            let result;
            if (conversationId) {
                // Update existing conversation
                result = await apiRef.current.update(conversationId, messages, provider, model, connection);
            } else {
                // Create new conversation
                result = await apiRef.current.create(messages, provider, model, connection);
            }

            // Refresh the conversation list
            await loadConversations();

            return result;
        } catch (err) {
            console.error('Failed to save conversation:', err);
            setError(err.message);
            return null;
        }
    }, [loadConversations]);

    // Rename a conversation
    const renameConversation = useCallback(async (id, title) => {
        if (!apiRef.current) return false;

        try {
            await apiRef.current.rename(id, title);

            // Refresh the conversation list
            await loadConversations();

            return true;
        } catch (err) {
            console.error('Failed to rename conversation:', err);
            setError(err.message);
            return false;
        }
    }, [loadConversations]);

    // Delete a conversation
    const deleteConversation = useCallback(async (id) => {
        if (!apiRef.current) return false;

        try {
            await apiRef.current.delete(id);

            // Clear current conversation if it was deleted
            if (currentConversationId === id) {
                setCurrentConversationId(null);
            }

            // Refresh the conversation list
            await loadConversations();

            return true;
        } catch (err) {
            console.error('Failed to delete conversation:', err);
            setError(err.message);
            return false;
        }
    }, [currentConversationId, loadConversations]);

    // Delete all conversations
    const deleteAllConversations = useCallback(async () => {
        if (!apiRef.current) return false;

        try {
            await apiRef.current.deleteAll();
            setConversations([]);
            setCurrentConversationId(null);
            return true;
        } catch (err) {
            console.error('Failed to delete all conversations:', err);
            setError(err.message);
            return false;
        }
    }, []);

    // Start a new conversation
    const startNewConversation = useCallback(() => {
        setCurrentConversationId(null);
    }, []);

    // Select a conversation
    const selectConversation = useCallback((id) => {
        setCurrentConversationId(id);
    }, []);

    return {
        conversations,
        currentConversationId,
        loading,
        error,
        loadConversations,
        getConversation,
        saveConversation,
        renameConversation,
        deleteConversation,
        deleteAllConversations,
        startNewConversation,
        selectConversation,
        setCurrentConversationId,
    };
};

export default useConversations;
