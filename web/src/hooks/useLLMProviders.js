/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - useLLMProviders Hook
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { useState, useEffect, useRef, useCallback } from 'react';
import { useLocalStorageString } from './useLocalStorage';

// Display labels for known providers. The library proxy now reports
// only the canonical provider name; the UI needs a human-friendly
// label for the dropdown.
const PROVIDER_DISPLAY_LABELS = {
    anthropic: 'Anthropic Claude',
    openai: 'OpenAI',
    ollama: 'Ollama',
    gemini: 'Gemini',
};

const providerDisplayLabel = (name) => {
    if (!name) return '';
    if (PROVIDER_DISPLAY_LABELS[name]) return PROVIDER_DISPLAY_LABELS[name];
    // Fallback: capitalise the first letter.
    return name.charAt(0).toUpperCase() + name.slice(1);
};

// Normalise the provider list returned by the proxy. The wire format
// is `{name, model, default}`; the UI expects a `display` field and
// retains the old `isDefault` alias for compatibility with any
// consumers that still reference it.
export const normaliseProviders = (rawProviders) => {
    if (!Array.isArray(rawProviders)) return [];
    return rawProviders.map((p) => ({
        name: p.name,
        // Prefer the proxy-supplied human-readable name (v0.1.0+ reports
        // `display_name`); accept camelCase defensively. Fall back to the
        // local label map when the API omits it.
        display: p.display_name || p.displayName || providerDisplayLabel(p.name),
        model: p.model || '',
        isDefault: Boolean(p.default),
    }));
};

// Normalise the models list returned by the proxy. The wire format is
// `["model-id", ...]`; the UI expects objects with a `name` field
// (and an optional `description`, which the new API no longer
// provides).
const normaliseModels = (rawModels) => {
    if (!Array.isArray(rawModels)) return [];
    return rawModels.map((m) => {
        if (typeof m === 'string') {
            return { name: m, description: '' };
        }
        return { name: m.name || '', description: m.description || '' };
    });
};

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
 * Extract model family prefix from a model ID.
 * Handles Anthropic's date-suffixed model naming convention.
 * Examples:
 *   - "claude-opus-4-5-20251101" → "claude-opus-4-5-"
 *   - "claude-sonnet-4-20250514" → "claude-sonnet-4-"
 *   - "gpt-4o-mini" → "" (no date suffix pattern)
 * @param {string} model - Model ID
 * @returns {string} Family prefix or empty string if not parseable
 */
const extractModelFamily = (model) => {
    if (!model || model.length < 9) {
        return '';
    }

    // Check if last 8 chars are digits (date: YYYYMMDD)
    const suffix = model.slice(-8);
    if (!/^\d{8}$/.test(suffix)) {
        return '';
    }

    // Check there's a hyphen before the date
    if (model.length < 10 || model[model.length - 9] !== '-') {
        return '';
    }

    // Return everything up to and including the hyphen before the date
    return model.slice(0, -8);
};

/**
 * Find a model in availableModels that matches the family of savedModel.
 * Family matching: "claude-opus-4-5-20251101" matches "claude-opus-4-5-*"
 * Returns the latest (by date suffix) matching model, or empty string if no match.
 * @param {string} savedModel - The saved model preference
 * @param {Array} availableModels - Array of model objects with .name property
 * @returns {string} Matching model name or empty string
 */
const findModelFamilyMatch = (savedModel, availableModels) => {
    if (!availableModels || availableModels.length === 0) {
        return '';
    }

    const family = extractModelFamily(savedModel);
    if (!family) {
        return '';
    }

    // Find all models with the SAME family (exact family match, not prefix)
    const matches = availableModels
        .map(m => m.name)
        .filter(name => extractModelFamily(name) === family);

    if (matches.length === 0) {
        return '';
    }

    // Return the latest version (highest date suffix - alphabetically last)
    matches.sort();
    return matches[matches.length - 1];
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

    // Ref to track when we're using a fallback model that shouldn't be saved
    // This prevents overwriting user's preference when their model isn't available
    const usingFallbackModelRef = useRef(false);

    // Mirror selectedModel into a ref so the models-fetching effect can
    // read the current value without re-running on every model change.
    const selectedModelRef = useRef(selectedModel);
    useEffect(() => {
        selectedModelRef.current = selectedModel;
    }, [selectedModel]);

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
                console.log('Fetching providers from /api/llm/v1/providers...');
                const response = await fetch('/api/llm/v1/providers', {
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

                // Normalise the proxy wire format ({name, model, default})
                // into the {name, display, model, isDefault} shape the UI
                // consumes.
                const normalisedProviders = normaliseProviders(data.providers);
                setProviders(normalisedProviders);

                // Only set default if no saved provider or saved provider is not available
                const savedProviderExists = normalisedProviders.some(p => p.name === selectedProvider);

                if (!selectedProvider || !savedProviderExists) {
                    // No saved preference or saved provider no longer available - use default.
                    // The proxy returns the default provider name in
                    // `default_provider`; if that's missing fall back to
                    // the entry flagged `default`, then to the first one.
                    const defaultProviderName = data.default_provider || '';
                    let defaultProvider = null;
                    if (defaultProviderName) {
                        defaultProvider = normalisedProviders.find(p => p.name === defaultProviderName);
                    }
                    if (!defaultProvider) {
                        defaultProvider = normalisedProviders.find(p => p.isDefault) || normalisedProviders[0];
                    }
                    if (defaultProvider) {
                        console.log('Setting default provider:', defaultProvider.name);
                        setSelectedProvider(defaultProvider.name);
                        // Load remembered model for this provider, falling
                        // back to the provider's advertised default model
                        // (returned by the proxy on each provider entry).
                        const rememberedModel = getPerProviderModel(defaultProvider.name);
                        if (rememberedModel) {
                            console.log('Using remembered model for provider:', rememberedModel);
                            setSelectedModel(rememberedModel);
                        } else if (defaultProvider.model) {
                            console.log('Using provider default model:', defaultProvider.model);
                            setSelectedModel(defaultProvider.model);
                        } else {
                            setSelectedModel('');
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

        // Capture the provider at effect start so we can detect stale
        // responses after a fast provider switch.
        const providerAtStart = selectedProvider;
        const controller = new AbortController();

        const fetchModels = async () => {
            setLoadingModels(true);
            setError('');

            try {
                console.log('Fetching models for provider:', providerAtStart);
                const response = await fetch(`/api/llm/v1/models?provider=${encodeURIComponent(providerAtStart)}`, {
                    credentials: 'include',
                    headers: {
                        'Authorization': `Bearer ${sessionToken}`,
                    },
                    signal: controller.signal,
                });

                console.log('Models response status:', response.status);
                if (!response.ok) {
                    const errorText = await response.text();
                    console.error('Models response error:', errorText);
                    throw new Error(`Failed to fetch models: ${response.status} ${errorText}`);
                }

                const data = await response.json();
                console.log('Models data:', data);

                // Bail if the provider changed while we were waiting; the
                // newer effect run is authoritative.
                if (controller.signal.aborted) {
                    return;
                }

                // The proxy returns models as plain strings; normalise
                // to the {name, description} shape the UI consumes
                // (description is no longer provided by the API).
                const normalisedModels = normaliseModels(data.models);
                setModels(normalisedModels);

                // Load remembered model for this provider or select first available
                if (normalisedModels.length > 0) {
                    // Check if there's a pending model restore (from loading a conversation)
                    const pendingModel = pendingModelRestoreRef.current;
                    pendingModelRestoreRef.current = null; // Clear it after reading

                    if (pendingModel) {
                        // Check if pending model is available for this provider (exact match)
                        const pendingModelExists = normalisedModels.some(m => m.name === pendingModel);
                        if (pendingModelExists) {
                            console.log('Restoring model from conversation:', pendingModel);
                            usingFallbackModelRef.current = false;
                            setSelectedModel(pendingModel);
                            // Don't save to per-provider storage - let user's preference stay
                            return;
                        }

                        // Try family match for conversation model (e.g., claude-opus-4-5-20251101 → claude-opus-4-5-20251217)
                        const pendingFamilyMatch = findModelFamilyMatch(pendingModel, normalisedModels);
                        if (pendingFamilyMatch) {
                            console.log('Restoring model from conversation via family match:', pendingModel, '→', pendingFamilyMatch);
                            usingFallbackModelRef.current = false;
                            setSelectedModel(pendingFamilyMatch);
                            // Don't save - this is conversation restore, not preference change
                            return;
                        }

                        console.log('Pending model not available for provider (no family match):', pendingModel);
                        // Fall through to remembered model logic
                    }

                    const rememberedModel = getPerProviderModel(providerAtStart);

                    if (rememberedModel) {
                        // Check if remembered model is still available (exact match)
                        const rememberedModelExists = normalisedModels.some(m => m.name === rememberedModel);
                        if (rememberedModelExists) {
                            console.log('Using remembered model for provider:', rememberedModel);
                            usingFallbackModelRef.current = false;
                            setSelectedModel(rememberedModel);
                            // No need to save - it's already saved
                        } else {
                            // Try family match (e.g., claude-opus-4-5-20251101 → claude-opus-4-5-20251217)
                            const familyMatch = findModelFamilyMatch(rememberedModel, normalisedModels);
                            if (familyMatch) {
                                console.log('Model updated via family match:', rememberedModel, '→', familyMatch);
                                usingFallbackModelRef.current = false;
                                setSelectedModel(familyMatch);
                                // Save the new version (intentional update to newer model)
                                setPerProviderModel(providerAtStart, familyMatch);
                            } else {
                                // No exact or family match - fall back to first model
                                // but DON'T save - preserve user's original preference
                                console.log('Remembered model not available (no family match), using first model:', normalisedModels[0].name);
                                console.log('(Not saving fallback to preserve user preference:', rememberedModel, ')');
                                usingFallbackModelRef.current = true;
                                setSelectedModel(normalisedModels[0].name);
                            }
                        }
                    } else {
                        // No remembered model - prefer the provider's
                        // advertised default (set by fetchProviders) so we
                        // don't clobber it on first load. Match by name
                        // against the available models; if the default is
                        // absent (or not advertised) fall back to the
                        // first listed model.
                        const providerEntry = providers.find(p => p.name === providerAtStart);
                        const advertisedDefault = providerEntry && providerEntry.model;
                        const defaultExists = advertisedDefault &&
                            normalisedModels.some(m => m.name === advertisedDefault);
                        const chosenModel = defaultExists
                            ? advertisedDefault
                            : normalisedModels[0].name;

                        console.log('No remembered model for this provider, selecting:', chosenModel);
                        usingFallbackModelRef.current = false;
                        setSelectedModel(chosenModel);
                        // Only persist when the choice differs from the
                        // already-selected model. fetchProviders may have
                        // already set selectedModel to the advertised
                        // default; persisting unconditionally would
                        // clobber that into local storage on first load
                        // before the user has expressed a preference.
                        // Read via ref so this effect doesn't depend on
                        // selectedModel (which would cause it to re-run
                        // whenever the model changes).
                        if (chosenModel !== selectedModelRef.current) {
                            setPerProviderModel(providerAtStart, chosenModel);
                        }
                    }
                } else {
                    console.warn('No models returned from API');
                    // Nothing to select for this provider, so the model
                    // left over from the previous one must not survive;
                    // see the note on the catch below.
                    usingFallbackModelRef.current = false;
                    setSelectedModel('');
                }
            } catch (err) {
                if (err.name === 'AbortError') {
                    console.log('Models fetch aborted for provider:', providerAtStart);
                    return;
                }
                console.error('Error fetching models:', err);
                setModels([]);
                setError('Failed to load models. Please check browser console.');
                // selectedModel still holds the model belonging to the
                // provider we have just switched away from, and the
                // finally below re-enables sending. Leaving it in place
                // would let a request go out pairing the new provider
                // with a model it never advertised, which is the same
                // mismatch the loadingModels gate prevents whilst the
                // fetch is in flight. Clearing it makes the proxy fall
                // back to the provider's configured default instead.
                usingFallbackModelRef.current = false;
                setSelectedModel('');
            } finally {
                // Only clear loading state if this effect run is still
                // current. The newer run will manage its own loading
                // state.
                if (!controller.signal.aborted) {
                    setLoadingModels(false);
                }
            }
        };

        fetchModels();

        return () => {
            controller.abort();
        };
    }, [selectedProvider, sessionToken, providers]);

    // Save model when user manually changes it (not when provider changes)
    useEffect(() => {
        if (selectedProvider && selectedModel && models.length > 0) {
            // Skip saving if we're using a fallback model (preserve user's original preference)
            if (usingFallbackModelRef.current) {
                console.log('Skipping save - using fallback model to preserve user preference');
                return;
            }
            // Only save if the model is in the current models list (meaning it's valid for this provider)
            const modelExists = models.some(m => m.name === selectedModel);
            if (modelExists) {
                console.log('Model changed by user, saving for provider:', selectedProvider, 'model:', selectedModel);
                setPerProviderModel(selectedProvider, selectedModel);
            }
        }
    }, [selectedModel]); // Only depend on selectedModel, not selectedProvider

    // Wrapped setter that clears fallback flag when user explicitly changes model
    const handleSetSelectedModel = useCallback((model) => {
        // Clear fallback flag - user is explicitly choosing a model
        usingFallbackModelRef.current = false;
        setSelectedModel(model);
    }, []);

    // Restore provider and model from a conversation without localStorage override
    const restoreProviderAndModel = useCallback((provider, model) => {
        if (!provider) return;

        console.log('Restoring provider and model from conversation:', provider, model);

        // If same provider, just set the model directly
        if (provider === selectedProvider) {
            if (model) {
                usingFallbackModelRef.current = false;
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
        setSelectedModel: handleSetSelectedModel,
        loadingProviders,
        loadingModels,
        error,
        restoreProviderAndModel,
    };
};
