/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - useLLMProviders Hook
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { useState, useEffect, useRef, useCallback } from 'react';
import { useLocalStorageString } from './useLocalStorage';

// Helper functions for per-provider model storage
const getProviderModelKey = (provider) => `llm-model-${provider}`;

const getPerProviderModel = (provider) => {
    if (!provider) return '';
    const key = getProviderModelKey(provider);
    return localStorage.getItem(key) || '';
};

const setPerProviderModel = (provider, model) => {
    if (!provider) return;
    const key = getProviderModelKey(provider);
    if (model) {
        localStorage.setItem(key, model);
    } else {
        localStorage.removeItem(key);
    }
};

/**
 * Custom hook for managing LLM providers and models
 * @param {string} sessionToken - Authentication session token
 * @returns {Object} Provider and model state and methods
 */
export const useLLMProviders = (sessionToken) => {
    const [providers, setProviders] = useState([]);
    const [selectedProvider, setSelectedProvider] = useLocalStorageString('llm-provider', '');
    const [models, setModels] = useState([]);
    const [selectedModel, setSelectedModel] = useState('');
    const [loadingProviders, setLoadingProviders] = useState(false);
    const [loadingModels, setLoadingModels] = useState(false);
    const [error, setError] = useState('');

    // Ref to track pending model restore (when loading a conversation)
    const pendingModelRestoreRef = useRef(null);

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
                        console.log('Setting default provider:', defaultProvider.name);
                        setSelectedProvider(defaultProvider.name);
                        // Load remembered model for this provider (or default)
                        const rememberedModel = getPerProviderModel(defaultProvider.name);
                        if (rememberedModel) {
                            console.log('Using remembered model for provider:', rememberedModel);
                            setSelectedModel(rememberedModel);
                        } else {
                            console.log('Using default model:', data.defaultModel);
                            setSelectedModel(data.defaultModel || '');
                        }
                    } else {
                        console.warn('No default provider found in response');
                    }
                } else {
                    // Saved provider exists - load its remembered model
                    const rememberedModel = getPerProviderModel(selectedProvider);
                    if (rememberedModel) {
                        console.log('Loading remembered model for saved provider:', rememberedModel);
                        setSelectedModel(rememberedModel);
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

                // Load remembered model for this provider or select first available
                if (data.models && data.models.length > 0) {
                    // Check if there's a pending model restore (from loading a conversation)
                    const pendingModel = pendingModelRestoreRef.current;
                    pendingModelRestoreRef.current = null; // Clear it after reading

                    if (pendingModel) {
                        // Check if pending model is available for this provider
                        const pendingModelExists = data.models.some(m => m.name === pendingModel);
                        if (pendingModelExists) {
                            console.log('Restoring model from conversation:', pendingModel);
                            setSelectedModel(pendingModel);
                            // Don't save to per-provider storage - let user's preference stay
                            return;
                        } else {
                            console.log('Pending model not available for provider:', pendingModel);
                        }
                    }

                    const rememberedModel = getPerProviderModel(selectedProvider);

                    if (rememberedModel) {
                        // Check if remembered model is still available
                        const rememberedModelExists = data.models.some(m => m.name === rememberedModel);
                        if (rememberedModelExists) {
                            console.log('Using remembered model for provider:', rememberedModel);
                            setSelectedModel(rememberedModel);
                            // No need to save - it's already saved
                        } else {
                            // Remembered model no longer available, use first model
                            console.log('Remembered model not available, selecting first model:', data.models[0].name);
                            setSelectedModel(data.models[0].name);
                            // Save the new selection
                            setPerProviderModel(selectedProvider, data.models[0].name);
                        }
                    } else {
                        // No remembered model - use first model
                        console.log('No remembered model for this provider, selecting first model:', data.models[0].name);
                        setSelectedModel(data.models[0].name);
                        // Save the selection
                        setPerProviderModel(selectedProvider, data.models[0].name);
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

    // Save model when user manually changes it (not when provider changes)
    useEffect(() => {
        if (selectedProvider && selectedModel && models.length > 0) {
            // Only save if the model is in the current models list (meaning it's valid for this provider)
            const modelExists = models.some(m => m.name === selectedModel);
            if (modelExists) {
                console.log('Model changed by user, saving for provider:', selectedProvider, 'model:', selectedModel);
                setPerProviderModel(selectedProvider, selectedModel);
            }
        }
    }, [selectedModel]); // Only depend on selectedModel, not selectedProvider

    // Restore provider and model from a conversation without localStorage override
    const restoreProviderAndModel = useCallback((provider, model) => {
        if (!provider) return;

        console.log('Restoring provider and model from conversation:', provider, model);

        // If same provider, just set the model directly
        if (provider === selectedProvider) {
            if (model) {
                setSelectedModel(model);
            }
            return;
        }

        // Different provider - set pending model before changing provider
        // This will be picked up by fetchModels effect
        if (model) {
            pendingModelRestoreRef.current = model;
        }
        setSelectedProvider(provider);
    }, [selectedProvider, setSelectedProvider]);

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
        restoreProviderAndModel,
    };
};
