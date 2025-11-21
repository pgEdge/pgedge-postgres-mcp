/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - useLLMProviders Hook
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { useState, useEffect } from 'react';
import { useLocalStorageString } from './useLocalStorage';

/**
 * Custom hook for managing LLM providers and models
 * @param {string} sessionToken - Authentication session token
 * @returns {Object} Provider and model state and methods
 */
export const useLLMProviders = (sessionToken) => {
    const [providers, setProviders] = useState([]);
    const [selectedProvider, setSelectedProvider] = useLocalStorageString('llm-provider', '');
    const [models, setModels] = useState([]);
    const [selectedModel, setSelectedModel] = useLocalStorageString('llm-model', '');
    const [loadingProviders, setLoadingProviders] = useState(false);
    const [loadingModels, setLoadingModels] = useState(false);
    const [error, setError] = useState('');

    // Fetch available providers on mount
    useEffect(() => {
        if (!sessionToken) {
            console.log('No session token available, skipping providers fetch');
            return;
        }

        const fetchProviders = async () => {
            setLoadingProviders(true);
            setError('');

            try {
                console.log('Fetching providers from /api/llm/providers...');
                const response = await fetch('/api/llm/providers', {
                    credentials: 'include',
                    headers: {
                        'Authorization': `Bearer ${sessionToken}`,
                    },
                });

                console.log('Providers response status:', response.status);
                if (!response.ok) {
                    const errorText = await response.text();
                    console.error('Providers response error:', errorText);
                    throw new Error(`Failed to fetch providers: ${response.status} ${errorText}`);
                }

                const data = await response.json();
                console.log('Providers data:', data);
                setProviders(data.providers || []);

                // Only set default if no saved provider or saved provider is not available
                const savedProviderExists = data.providers?.some(p => p.name === selectedProvider);

                if (!selectedProvider || !savedProviderExists) {
                    // No saved preference or saved provider no longer available - use default
                    const defaultProvider = data.providers?.find(p => p.isDefault);
                    if (defaultProvider) {
                        console.log('Setting default provider:', defaultProvider.name, 'model:', data.defaultModel);
                        setSelectedProvider(defaultProvider.name);
                        setSelectedModel(data.defaultModel || '');
                    } else {
                        console.warn('No default provider found in response');
                    }
                }
            } catch (err) {
                console.error('Error fetching providers:', err);
                setError('Failed to load LLM providers. Please check browser console.');
            } finally {
                setLoadingProviders(false);
            }
        };

        fetchProviders();
    }, [sessionToken]);

    // Fetch available models when provider changes
    useEffect(() => {
        if (!selectedProvider || !sessionToken) {
            console.log('No provider selected or no session token, skipping model fetch');
            return;
        }

        const fetchModels = async () => {
            setLoadingModels(true);
            setError('');

            try {
                console.log('Fetching models for provider:', selectedProvider);
                const response = await fetch(`/api/llm/models?provider=${selectedProvider}`, {
                    credentials: 'include',
                    headers: {
                        'Authorization': `Bearer ${sessionToken}`,
                    },
                });

                console.log('Models response status:', response.status);
                if (!response.ok) {
                    const errorText = await response.text();
                    console.error('Models response error:', errorText);
                    throw new Error(`Failed to fetch models: ${response.status} ${errorText}`);
                }

                const data = await response.json();
                console.log('Models data:', data);
                setModels(data.models || []);

                // Set the first model as selected if current model is not in the list
                if (data.models && data.models.length > 0) {
                    const currentModelExists = data.models.some(m => m.name === selectedModel);
                    if (!currentModelExists) {
                        console.log('Current model not in list, selecting first model:', data.models[0].name);
                        setSelectedModel(data.models[0].name);
                    }
                } else {
                    console.warn('No models returned from API');
                }
            } catch (err) {
                console.error('Error fetching models:', err);
                setModels([]);
                setError('Failed to load models. Please check browser console.');
            } finally {
                setLoadingModels(false);
            }
        };

        fetchModels();
    }, [selectedProvider, sessionToken]);

    return {
        providers,
        selectedProvider,
        setSelectedProvider,
        models,
        selectedModel,
        setSelectedModel,
        loadingProviders,
        loadingModels,
        error,
    };
};
