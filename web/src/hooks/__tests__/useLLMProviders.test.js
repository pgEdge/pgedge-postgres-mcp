/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - useLLMProviders Hook Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { normaliseProviders, useLLMProviders } from '../useLLMProviders';

describe('normaliseProviders', () => {
    it('prefers the API display_name when present', () => {
        const out = normaliseProviders([
            { name: 'anthropic', display_name: 'Anthropic', model: 'claude' },
        ]);
        expect(out[0].display).toBe('Anthropic');
    });

    it('falls back to the local label map when display_name is absent', () => {
        const out = normaliseProviders([{ name: 'anthropic', model: 'claude' }]);
        expect(out[0].display).toBe('Anthropic Claude');
    });

    it('capitalises unknown providers with no display_name', () => {
        const out = normaliseProviders([{ name: 'weird', model: 'x' }]);
        expect(out[0].display).toBe('Weird');
    });
});

describe('useLLMProviders model list failures', () => {
    // The providers fetch seeds selectedModel with the default
    // provider's advertised model, so by the time the models fetch runs
    // there is always a model in place that belongs to some provider.
    // If that fetch then fails, sending is re-enabled regardless, and a
    // model left over from the provider we switched away from would go
    // out paired with the new one.
    const providersPayload = {
        providers: [{ name: 'openai', model: 'gpt-4', default: true }],
        default_provider: 'openai',
    };

    const mockFetch = (modelsResponse) => {
        globalThis.fetch = vi.fn((url) => {
            if (String(url).includes('/v1/providers')) {
                return Promise.resolve({
                    ok: true,
                    status: 200,
                    json: () => Promise.resolve(providersPayload),
                });
            }
            return Promise.resolve(modelsResponse);
        });
    };

    beforeEach(() => {
        localStorage.clear();
        vi.clearAllMocks();
    });

    afterEach(() => {
        delete globalThis.fetch;
        localStorage.clear();
    });

    it('clears the selected model when the model list fails to load', async () => {
        mockFetch({
            ok: false,
            status: 500,
            text: () => Promise.resolve('boom'),
        });

        const { result } = renderHook(() => useLLMProviders('test-token'));

        await waitFor(() => expect(result.current.error).toContain('Failed to load models'));
        await waitFor(() => expect(result.current.loadingModels).toBe(false));

        // Without the clear, this holds the previous provider's 'gpt-4',
        // seeded by the providers fetch, and sending is enabled again.
        expect(result.current.selectedModel).toBe('');
    });

    it('clears the selected model when the provider advertises no models', async () => {
        mockFetch({
            ok: true,
            status: 200,
            json: () => Promise.resolve({ models: [] }),
        });

        const { result } = renderHook(() => useLLMProviders('test-token'));

        await waitFor(() => expect(result.current.providers).toHaveLength(1));
        await waitFor(() => expect(result.current.loadingModels).toBe(false));

        expect(result.current.selectedModel).toBe('');
    });
});
