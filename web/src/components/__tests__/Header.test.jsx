/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Header Component Tests
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Header from '../Header';
import { AuthProvider } from '../../contexts/AuthContext';
import { mockInitialize, mockListTools, mockUserInfo } from '../../test-utils/mcp-mocks';

// Mock the logo imports
vi.mock('../../assets/images/logo-light.png', () => ({
    default: 'logo-light.png',
}));

vi.mock('../../assets/images/logo-dark.png', () => ({
    default: 'logo-dark.png',
}));

describe('Header Component', () => {
    const mockToggleTheme = vi.fn();

    beforeEach(() => {
        mockToggleTheme.mockClear();
        global.fetch = vi.fn();
        localStorage.clear();
    });

    afterEach(() => {
        localStorage.clear();
    });

    const renderHeader = (mode = 'light', user = { username: 'testuser' }) => {
        // Mock authenticated state via MCP JSON-RPC protocol
        localStorage.setItem('mcp-session-token', 'test-token');

        // Mock the sequence of calls that checkAuth makes:
        global.fetch.mockResolvedValueOnce(mockInitialize(1));
        global.fetch.mockResolvedValueOnce(mockListTools(2));
        global.fetch.mockResolvedValueOnce(mockUserInfo(user.username));

        return render(
            <AuthProvider>
                <Header onToggleTheme={mockToggleTheme} mode={mode} />
            </AuthProvider>
        );
    };

    it('renders header with logo and title', async () => {
        renderHeader();

        await waitFor(() => {
            expect(screen.getByAltText('pgEdge Natural Language Agent')).toBeInTheDocument();
            expect(screen.getByText('Natural Language Agent')).toBeInTheDocument();
        });
    });

    it('displays correct logo based on theme mode', async () => {
        const { rerender } = renderHeader('light');

        await waitFor(() => {
            const logo = screen.getByAltText('pgEdge Natural Language Agent');
            expect(logo).toHaveAttribute('src', 'logo-light.png');
        });

        // Re-render with dark mode (token still in localStorage, need to mock auth check again)
        global.fetch.mockResolvedValueOnce(mockInitialize(3));
        global.fetch.mockResolvedValueOnce(mockListTools(4));
        global.fetch.mockResolvedValueOnce(mockUserInfo('testuser'));

        rerender(
            <AuthProvider>
                <Header onToggleTheme={mockToggleTheme} mode="dark" />
            </AuthProvider>
        );

        await waitFor(() => {
            const logo = screen.getByAltText('pgEdge Natural Language Agent');
            expect(logo).toHaveAttribute('src', 'logo-dark.png');
        });
    });

    it('toggles theme when theme button is clicked', async () => {
        renderHeader();
        const user = userEvent.setup();

        await waitFor(() => {
            expect(screen.getByLabelText('toggle theme')).toBeInTheDocument();
        });

        const themeButton = screen.getByLabelText('toggle theme');
        await user.click(themeButton);

        expect(mockToggleTheme).toHaveBeenCalledTimes(1);
    });

    it('displays correct theme icon based on mode', async () => {
        const { rerender } = renderHeader('light');

        await waitFor(() => {
            // In light mode, should show dark mode icon (moon)
            const themeButton = screen.getByLabelText('toggle theme');
            expect(themeButton).toBeInTheDocument();
        });

        // Re-render with dark mode (token still in localStorage, need to mock auth check again)
        global.fetch.mockResolvedValueOnce(mockInitialize(3));
        global.fetch.mockResolvedValueOnce(mockListTools(4));
        global.fetch.mockResolvedValueOnce(mockUserInfo('testuser'));

        rerender(
            <AuthProvider>
                <Header onToggleTheme={mockToggleTheme} mode="dark" />
            </AuthProvider>
        );

        await waitFor(() => {
            // In dark mode, should show light mode icon (sun)
            const themeButton = screen.getByLabelText('toggle theme');
            expect(themeButton).toBeInTheDocument();
        });
    });

    it('opens help panel when help button is clicked', async () => {
        renderHeader();
        const user = userEvent.setup();

        await waitFor(() => {
            expect(screen.getByLabelText('open help')).toBeInTheDocument();
        });

        const helpButton = screen.getByLabelText('open help');
        await user.click(helpButton);

        // Wait for help panel to appear
        await waitFor(() => {
            expect(screen.getByText('Help & Documentation')).toBeInTheDocument();
        });
    });

    it('closes help panel when close button is clicked', async () => {
        renderHeader();
        const user = userEvent.setup();

        await waitFor(() => {
            expect(screen.getByLabelText('open help')).toBeInTheDocument();
        });

        // Open help panel
        const helpButton = screen.getByLabelText('open help');
        await user.click(helpButton);

        await waitFor(() => {
            expect(screen.getByText('Help & Documentation')).toBeInTheDocument();
        });

        // Close help panel
        const closeButton = screen.getByLabelText('close help');
        await user.click(closeButton);

        await waitFor(() => {
            expect(screen.queryByText('Help & Documentation')).not.toBeInTheDocument();
        });
    });

    it('opens user menu when avatar is clicked', async () => {
        renderHeader('light', { username: 'testuser' });
        const user = userEvent.setup();

        await waitFor(() => {
            expect(screen.getByLabelText('user menu')).toBeInTheDocument();
        });

        const avatarButton = screen.getByLabelText('user menu');
        await user.click(avatarButton);

        await waitFor(() => {
            expect(screen.getByText('Logout')).toBeInTheDocument();
        });
    });

    it('calls logout when logout menu item is clicked', async () => {
        renderHeader();
        const user = userEvent.setup();

        await waitFor(() => {
            expect(screen.getByLabelText('user menu')).toBeInTheDocument();
        });

        // Open user menu
        const avatarButton = screen.getByLabelText('user menu');
        await user.click(avatarButton);

        await waitFor(() => {
            expect(screen.getByText('Logout')).toBeInTheDocument();
        });

        // Click logout
        const logoutButton = screen.getByText('Logout');
        await user.click(logoutButton);

        // Logout is local-only, verify localStorage is cleared
        await waitFor(() => {
            expect(localStorage.getItem('mcp-session-token')).toBe(null);
        });
    });

    it('closes user menu after logout', async () => {
        renderHeader();
        const user = userEvent.setup();

        await waitFor(() => {
            expect(screen.getByLabelText('user menu')).toBeInTheDocument();
        });

        // Open user menu
        const avatarButton = screen.getByLabelText('user menu');
        await user.click(avatarButton);

        await waitFor(() => {
            expect(screen.getByText('Logout')).toBeInTheDocument();
        });

        // Click logout
        const logoutButton = screen.getByText('Logout');
        await user.click(logoutButton);

        // Logout is local-only, verify localStorage is cleared
        await waitFor(() => {
            expect(localStorage.getItem('mcp-session-token')).toBe(null);
        });
    });
});
