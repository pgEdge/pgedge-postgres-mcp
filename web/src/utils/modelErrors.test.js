/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Unusable Model Error Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect } from 'vitest';
import { describeUnusableModelError, formatChatError } from './modelErrors';

// Captured from the live Gemini API through the proxy, rather than written
// from the documentation, so that the patterns match what actually arrives.
const TTS_ERROR = 'gemini (400): The requested combination of response '
    + 'modalities (TEXT) is not supported by the model. '
    + 'models/gemini-2.5-flash-preview-tts accepts the following combination '
    + 'of response modalities:\n* AUDIO\n';
const INTERACTIONS_ERROR = 'gemini (400): This model only supports '
    + 'Interactions API.';
const GENERIC_ERROR = 'gemini (400): Request contains an invalid argument.';
const RATE_LIMIT_ERROR = 'gemini (429): HTTP 429';

describe('describeUnusableModelError', () => {
    it('explains a text-to-speech model in terms of what it does produce', () => {
        const out = describeUnusableModelError(TTS_ERROR, 'gemini-2.5-flash-preview-tts');
        expect(out).toContain('gemini-2.5-flash-preview-tts');
        expect(out).toContain('cannot be used for chat');
        expect(out).toContain('AUDIO');
        expect(out).toContain('Choose a different model');
    });

    it('explains a model that is only reachable through another API', () => {
        const out = describeUnusableModelError(INTERACTIONS_ERROR, 'deep-research-preview-04-2026');
        expect(out).toContain('deep-research-preview-04-2026');
        expect(out).toContain('Interactions API');
        expect(out).toContain('Choose a different model');
    });

    it('explains a model that does not support multi-turn chat', () => {
        const out = describeUnusableModelError(
            'gemini (400): Multiturn chat is not enabled for models/some-model',
            'some-model'
        );
        expect(out).toContain('multi-turn');
        expect(out).toContain('some-model');
    });

    it('falls back to a generic phrase when the model is not known', () => {
        const out = describeUnusableModelError(TTS_ERROR);
        expect(out).toContain('The selected model');
    });

    // The point of returning null is that everything unrecognised reaches the
    // user in the provider's own words, rather than being flattened into a
    // guess about the model.
    it.each([
        ['a generic invalid-argument error', GENERIC_ERROR],
        ['a rate limit', RATE_LIMIT_ERROR],
        ['an unrelated failure', 'gemini (500): internal error'],
        ['an empty string', ''],
        ['a non-string', null],
    ])('leaves %s alone', (_name, input) => {
        expect(describeUnusableModelError(input, 'gemini-2.5-flash')).toBeNull();
    });
});

describe('formatChatError', () => {
    it('describes a recognised unusable-model error', () => {
        const out = formatChatError(new Error(TTS_ERROR), 'gemini-2.5-flash-preview-tts');
        expect(out).toContain('cannot be used for chat');
    });

    it('passes an unrecognised error through unchanged', () => {
        expect(formatChatError(new Error(GENERIC_ERROR), 'gemini-2.5-flash'))
            .toBe(GENERIC_ERROR);
    });

    it('falls back when the error carries no message', () => {
        expect(formatChatError(new Error(''), 'gemini-2.5-flash'))
            .toBe('Failed to send message');
        expect(formatChatError(undefined)).toBe('Failed to send message');
    });

    // Prompt execution and message sending word their fallback differently,
    // so the caller supplies it rather than having one imposed.
    it('uses a caller-supplied fallback', () => {
        expect(formatChatError(new Error(''), '', 'Failed to execute prompt'))
            .toBe('Failed to execute prompt');
    });
});
