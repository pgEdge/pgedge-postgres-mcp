/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - useMenu Hook Tests
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useMenu } from '../useMenu';

describe('useMenu Hook', () => {
    it('returns initial state with null anchorEl', () => {
        const { result } = renderHook(() => useMenu());

        expect(result.current.anchorEl).toBeNull();
        expect(result.current.open).toBe(false);
    });

    it('opens menu when handleOpen is called with event', () => {
        const { result } = renderHook(() => useMenu());

        const mockEvent = {
            currentTarget: document.createElement('button'),
        };

        act(() => {
            result.current.handleOpen(mockEvent);
        });

        expect(result.current.anchorEl).toBe(mockEvent.currentTarget);
        expect(result.current.open).toBe(true);
    });

    it('closes menu when handleClose is called', () => {
        const { result } = renderHook(() => useMenu());

        const mockEvent = {
            currentTarget: document.createElement('button'),
        };

        // Open menu first
        act(() => {
            result.current.handleOpen(mockEvent);
        });

        expect(result.current.open).toBe(true);

        // Close menu
        act(() => {
            result.current.handleClose();
        });

        expect(result.current.anchorEl).toBeNull();
        expect(result.current.open).toBe(false);
    });

    it('can open and close menu multiple times', () => {
        const { result } = renderHook(() => useMenu());

        const mockEvent1 = {
            currentTarget: document.createElement('button'),
        };

        const mockEvent2 = {
            currentTarget: document.createElement('div'),
        };

        // First open/close cycle
        act(() => {
            result.current.handleOpen(mockEvent1);
        });
        expect(result.current.open).toBe(true);

        act(() => {
            result.current.handleClose();
        });
        expect(result.current.open).toBe(false);

        // Second open/close cycle with different element
        act(() => {
            result.current.handleOpen(mockEvent2);
        });
        expect(result.current.anchorEl).toBe(mockEvent2.currentTarget);
        expect(result.current.open).toBe(true);

        act(() => {
            result.current.handleClose();
        });
        expect(result.current.open).toBe(false);
    });

    it('updates anchorEl when opening with different elements', () => {
        const { result } = renderHook(() => useMenu());

        const mockEvent1 = {
            currentTarget: document.createElement('button'),
        };

        const mockEvent2 = {
            currentTarget: document.createElement('div'),
        };

        act(() => {
            result.current.handleOpen(mockEvent1);
        });
        expect(result.current.anchorEl).toBe(mockEvent1.currentTarget);

        act(() => {
            result.current.handleOpen(mockEvent2);
        });
        expect(result.current.anchorEl).toBe(mockEvent2.currentTarget);
    });

    it('correctly calculates open state based on anchorEl', () => {
        const { result } = renderHook(() => useMenu());

        // Initially closed
        expect(result.current.open).toBe(false);

        const mockEvent = {
            currentTarget: document.createElement('button'),
        };

        // Open
        act(() => {
            result.current.handleOpen(mockEvent);
        });
        expect(result.current.open).toBe(true);
        expect(result.current.anchorEl).not.toBeNull();

        // Close
        act(() => {
            result.current.handleClose();
        });
        expect(result.current.open).toBe(false);
        expect(result.current.anchorEl).toBeNull();
    });

    it('provides all required properties', () => {
        const { result } = renderHook(() => useMenu());

        expect(result.current).toHaveProperty('anchorEl');
        expect(result.current).toHaveProperty('open');
        expect(result.current).toHaveProperty('handleOpen');
        expect(result.current).toHaveProperty('handleClose');
        expect(typeof result.current.handleOpen).toBe('function');
        expect(typeof result.current.handleClose).toBe('function');
    });
});
