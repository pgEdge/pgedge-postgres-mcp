/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Conversation Actions Context Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import {
    ConversationActionsProvider,
    useConversationActions,
} from '../ConversationActionsContext';

const wrapper = ({ children }) => (
    <ConversationActionsProvider>{children}</ConversationActionsProvider>
);

describe('ConversationActionsContext', () => {
    it('provides default values', () => {
        const { result } = renderHook(() => useConversationActions(), { wrapper });

        expect(result.current.hasMessages).toBe(false);
        expect(result.current.onSave).toBeNull();
        expect(result.current.onClear).toBeNull();
        expect(typeof result.current.registerActions).toBe('function');
    });

    it('updates consumed values when registerActions is called', () => {
        const { result } = renderHook(() => useConversationActions(), { wrapper });

        const onSave = () => {};
        const onClear = () => {};

        act(() => {
            result.current.registerActions({ hasMessages: true, onSave, onClear });
        });

        expect(result.current.hasMessages).toBe(true);
        expect(result.current.onSave).toBe(onSave);
        expect(result.current.onClear).toBe(onClear);
    });

    // A re-registration of identical values must not produce new state, or
    // every consumer re-renders for nothing and a caller that registers from
    // an effect can sustain an update loop ("Maximum update depth exceeded").
    it('does not change state when the same actions are re-registered', () => {
        const { result } = renderHook(() => useConversationActions(), { wrapper });

        const onSave = () => {};
        const onClear = () => {};

        act(() => {
            result.current.registerActions({ hasMessages: true, onSave, onClear });
        });

        const afterFirst = {
            hasMessages: result.current.hasMessages,
            onSave: result.current.onSave,
            onClear: result.current.onClear,
        };
        const contextAfterFirst = result.current;

        act(() => {
            result.current.registerActions({ hasMessages: true, onSave, onClear });
            result.current.registerActions({ hasMessages: true, onSave, onClear });
        });

        // Same values, so the consumed context object is untouched.
        expect(result.current).toBe(contextAfterFirst);
        expect(result.current.hasMessages).toBe(afterFirst.hasMessages);
        expect(result.current.onSave).toBe(afterFirst.onSave);
        expect(result.current.onClear).toBe(afterFirst.onClear);

        // A genuine change still propagates.
        act(() => {
            result.current.registerActions({ hasMessages: false, onSave, onClear });
        });
        expect(result.current.hasMessages).toBe(false);
    });

    it('keeps registerActions identity stable across renders', () => {
        const { result, rerender } = renderHook(() => useConversationActions(), { wrapper });

        const firstRegister = result.current.registerActions;

        act(() => {
            result.current.registerActions({ hasMessages: true, onSave: null, onClear: null });
        });
        rerender();

        expect(result.current.registerActions).toBe(firstRegister);
    });

    it('throws when used outside the provider', () => {
        expect(() => renderHook(() => useConversationActions())).toThrow(
            /must be used within a ConversationActionsProvider/
        );
    });
});
