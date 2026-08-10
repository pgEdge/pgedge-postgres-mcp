/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect } from 'vitest';
import {
    createToolRepeatGuard,
    buildRepeatedToolFailureMessage,
    stableStringify,
    toolCallKey,
} from './toolRepeatGuard';

/**
 * Records a failing call against the guard.
 * @param {object} guard - The guard under test.
 * @param {string} name - The tool name.
 * @param {*} input - The tool arguments.
 * @param {string} [text] - The error text.
 * @returns {object|null} - Trip details, or null.
 */
const fail = (guard, name, input, text = 'boom') =>
    guard.record({ name, input, isError: true, resultText: text });

/**
 * Records a successful call against the guard.
 * @param {object} guard - The guard under test.
 * @param {string} name - The tool name.
 * @param {*} input - The tool arguments.
 * @returns {object|null} - Trip details, or null.
 */
const succeed = (guard, name, input) =>
    guard.record({ name, input, isError: false, resultText: 'fine' });

describe('stableStringify', () => {
    it('produces the same text regardless of key order', () => {
        expect(stableStringify({ a: 1, b: 2 }))
            .toBe(stableStringify({ b: 2, a: 1 }));
    });

    it('sorts nested object keys too', () => {
        expect(stableStringify({ outer: { x: 1, y: 2 }, z: 3 }))
            .toBe(stableStringify({ z: 3, outer: { y: 2, x: 1 } }));
    });

    it('preserves array order', () => {
        expect(stableStringify([1, 2])).not.toBe(stableStringify([2, 1]));
    });

    it('distinguishes values of different types', () => {
        expect(stableStringify({ a: 1 })).not.toBe(stableStringify({ a: '1' }));
    });

    it('treats null and undefined alike', () => {
        expect(stableStringify(null)).toBe('null');
        expect(stableStringify(undefined)).toBe('null');
    });

    it('handles circular structures without throwing', () => {
        const cyclic = { name: 'loop' };
        cyclic.self = cyclic;
        expect(() => stableStringify(cyclic)).not.toThrow();
        expect(stableStringify(cyclic)).toContain('[circular]');
    });
});

describe('toolCallKey', () => {
    it('matches identical calls whose keys are ordered differently', () => {
        expect(toolCallKey('query_database', { sql: 'SELECT 1', db: 'x' }))
            .toBe(toolCallKey('query_database', { db: 'x', sql: 'SELECT 1' }));
    });

    it('separates different tools carrying the same arguments', () => {
        expect(toolCallKey('tool_a', { q: 1 }))
            .not.toBe(toolCallKey('tool_b', { q: 1 }));
    });
});

describe('createToolRepeatGuard', () => {
    it('does not trip before the limit is reached', () => {
        const guard = createToolRepeatGuard(3);
        expect(fail(guard, 'query_database', { sql: 'SELECT 1' })).toBeNull();
        expect(fail(guard, 'query_database', { sql: 'SELECT 1' })).toBeNull();
        expect(guard.getTripped()).toBeNull();
    });

    it('trips on the third identical failure', () => {
        const guard = createToolRepeatGuard(3);
        fail(guard, 'query_database', { sql: 'SELECT 1' }, 'relation missing');
        fail(guard, 'query_database', { sql: 'SELECT 1' }, 'relation missing');
        const tripped = fail(
            guard, 'query_database', { sql: 'SELECT 1' }, 'relation missing');

        expect(tripped).toEqual({
            name: 'query_database',
            count: 3,
            errorText: 'relation missing',
        });
        expect(guard.getTripped()).toEqual(tripped);
    });

    it('ignores argument key ordering when matching calls', () => {
        const guard = createToolRepeatGuard(3);
        fail(guard, 'query_database', { sql: 'SELECT 1', limit: 10 });
        fail(guard, 'query_database', { limit: 10, sql: 'SELECT 1' });
        expect(guard.getTripped()).toBeNull();

        fail(guard, 'query_database', { sql: 'SELECT 1', limit: 10 });
        expect(guard.getTripped()?.count).toBe(3);
    });

    it('does not trip when the same tool fails with different arguments', () => {
        const guard = createToolRepeatGuard(3);
        fail(guard, 'query_database', { sql: 'SELECT 1' });
        fail(guard, 'query_database', { sql: 'SELECT 2' });
        fail(guard, 'query_database', { sql: 'SELECT 3' });
        fail(guard, 'query_database', { sql: 'SELECT 4' });
        expect(guard.getTripped()).toBeNull();
    });

    it('does not trip when different tools fail with the same arguments', () => {
        const guard = createToolRepeatGuard(3);
        fail(guard, 'tool_a', { q: 1 });
        fail(guard, 'tool_b', { q: 1 });
        fail(guard, 'tool_c', { q: 1 });
        expect(guard.getTripped()).toBeNull();
    });

    it('resets the count for a call that later succeeds', () => {
        const guard = createToolRepeatGuard(3);
        fail(guard, 'query_database', { sql: 'SELECT 1' });
        fail(guard, 'query_database', { sql: 'SELECT 1' });
        succeed(guard, 'query_database', { sql: 'SELECT 1' });

        fail(guard, 'query_database', { sql: 'SELECT 1' });
        fail(guard, 'query_database', { sql: 'SELECT 1' });
        expect(guard.getTripped()).toBeNull();

        fail(guard, 'query_database', { sql: 'SELECT 1' });
        expect(guard.getTripped()?.count).toBe(3);
    });

    it('only resets the call that succeeded', () => {
        const guard = createToolRepeatGuard(3);
        fail(guard, 'query_database', { sql: 'SELECT 1' });
        fail(guard, 'query_database', { sql: 'SELECT 1' });
        succeed(guard, 'query_database', { sql: 'SELECT 2' });
        fail(guard, 'query_database', { sql: 'SELECT 1' });

        expect(guard.getTripped()?.count).toBe(3);
    });

    it('never trips when every call succeeds', () => {
        const guard = createToolRepeatGuard(3);
        for (let i = 0; i < 10; i++) {
            succeed(guard, 'query_database', { sql: 'SELECT 1' });
        }
        expect(guard.getTripped()).toBeNull();
    });

    it('keeps the first trip details once tripped', () => {
        const guard = createToolRepeatGuard(2);
        fail(guard, 'tool_a', { q: 1 }, 'first error');
        fail(guard, 'tool_a', { q: 1 }, 'first error');
        fail(guard, 'tool_a', { q: 1 }, 'later error');

        expect(guard.getTripped()).toEqual({
            name: 'tool_a',
            count: 2,
            errorText: 'first error',
        });
    });

    it('treats missing and empty arguments consistently', () => {
        const guard = createToolRepeatGuard(3);
        fail(guard, 'list_databases', {});
        fail(guard, 'list_databases', {});
        fail(guard, 'list_databases', {});
        expect(guard.getTripped()?.name).toBe('list_databases');
    });

    it('honours a limit of one', () => {
        const guard = createToolRepeatGuard(1);
        expect(fail(guard, 'tool_a', { q: 1 })?.count).toBe(1);
    });
});

describe('buildRepeatedToolFailureMessage', () => {
    it('names the tool, the count, and the last error', () => {
        const message = buildRepeatedToolFailureMessage({
            name: 'query_database',
            count: 3,
            errorText: 'relation "users" does not exist',
        });

        expect(message).toContain('query_database');
        expect(message).toContain('3 times');
        expect(message).toContain('relation "users" does not exist');
    });

    it('omits the error detail when there is no error text', () => {
        const message = buildRepeatedToolFailureMessage({
            name: 'query_database',
            count: 3,
            errorText: '',
        });

        expect(message).toContain('query_database');
        expect(message).not.toContain('The last error was');
    });

    it('truncates very long error text', () => {
        const message = buildRepeatedToolFailureMessage({
            name: 'query_database',
            count: 3,
            errorText: 'x'.repeat(2000),
        });

        expect(message.length).toBeLessThan(1000);
        expect(message).toContain('...');
    });

    it('contains no emdash', () => {
        const message = buildRepeatedToolFailureMessage({
            name: 'query_database',
            count: 3,
            errorText: 'boom',
        });

        expect(message).not.toContain('—');
    });
});
