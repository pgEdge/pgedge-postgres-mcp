/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Login Component Tests
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Login from '../Login';
import { AuthProvider } from '../../contexts/AuthContext';
import { mockAuthenticateSuccess, mockAuthenticateFailure } from '../../test-utils/mcp-mocks';

describe('Login Component', () => {
    beforeEach(() => {
        global.fetch = vi.fn();
        sessionStorage.clear();
        localStorage.clear();
    });

    const renderLogin = () => {
        // No token in localStorage, so AuthProvider's checkAuth returns early
        // without making any fetch calls
        return render(
            <AuthProvider>
                <Login />
            </AuthProvider>
        );
    };

    it('renders login form', async () => {
        renderLogin();

        await waitFor(() => {
            expect(screen.getByRole('heading', { name: /natural language agent/i })).toBeInTheDocument();
        });

        expect(screen.getByText(/sign in to continue/i)).toBeInTheDocument();
        expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
        expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
    });

    it('disables submit button while loading', async () => {
        renderLogin();

        await waitFor(() => {
            expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
        });

        const submitButton = screen.getByRole('button', { name: /sign in/i });
        const usernameInput = screen.getByLabelText(/username/i);
        const passwordInput = screen.getByLabelText(/password/i);

        // Mock delayed response for authenticate_user tool
        global.fetch.mockImplementationOnce(() =>
            new Promise(resolve =>
                setTimeout(() => {
                    resolve(mockAuthenticateSuccess(1, 'testuser'));
                }, 100)
            )
        );

        await userEvent.type(usernameInput, 'testuser');
        await userEvent.type(passwordInput, 'testpass');
        await userEvent.click(submitButton);

        // Button should be disabled and show loading text
        await waitFor(() => {
            expect(submitButton).toBeDisabled();
            expect(screen.getByText(/signing in\.\.\./i)).toBeInTheDocument();
        });

        // Wait for the async operation to complete to avoid act() warnings
        await waitFor(() => {
            expect(submitButton).not.toBeDisabled();
        }, { timeout: 200 });
    });

    it('shows error message on login failure', async () => {
        renderLogin();

        await waitFor(() => {
            expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
        });

        // Mock authentication failure via MCP JSON-RPC
        global.fetch.mockResolvedValueOnce(mockAuthenticateFailure(1, 'Invalid credentials'));

        const usernameInput = screen.getByLabelText(/username/i);
        const passwordInput = screen.getByLabelText(/password/i);
        const submitButton = screen.getByRole('button', { name: /sign in/i });

        await userEvent.type(usernameInput, 'testuser');
        await userEvent.type(passwordInput, 'wrongpass');
        await userEvent.click(submitButton);

        await waitFor(() => {
            expect(screen.getByText(/invalid credentials/i)).toBeInTheDocument();
        });

        // Button should be re-enabled after login fails
        expect(submitButton).not.toBeDisabled();
    });

    it('handles successful login', async () => {
        renderLogin();

        await waitFor(() => {
            expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
        });

        // Mock successful authentication via MCP JSON-RPC
        global.fetch.mockResolvedValueOnce(mockAuthenticateSuccess(1, 'testuser'));

        const usernameInput = screen.getByLabelText(/username/i);
        const passwordInput = screen.getByLabelText(/password/i);
        const submitButton = screen.getByRole('button', { name: /sign in/i });

        await userEvent.type(usernameInput, 'testuser');
        await userEvent.type(passwordInput, 'correctpass');
        await userEvent.click(submitButton);

        // Wait for login to complete
        await waitFor(() => {
            expect(submitButton).not.toBeDisabled();
        });

        // Verify authentication was called via MCP JSON-RPC
        expect(global.fetch).toHaveBeenCalledWith(
            '/mcp/v1',
            expect.objectContaining({
                method: 'POST',
                headers: expect.objectContaining({
                    'Content-Type': 'application/json'
                }),
                body: expect.stringContaining('"method":"tools/call"')
            })
        );

        // Verify the request body contains authenticate_user tool call
        const fetchCall = global.fetch.mock.calls[0];
        const requestBody = JSON.parse(fetchCall[1].body);
        expect(requestBody.params.name).toBe('authenticate_user');
        expect(requestBody.params.arguments).toEqual({
            username: 'testuser',
            password: 'correctpass'
        });
    });

    it('allows submission with empty fields (no client-side validation)', async () => {
        renderLogin();

        await waitFor(() => {
            expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
        });

        // Mock server error for empty credentials
        global.fetch.mockResolvedValueOnce(
            mockAuthenticateFailure(1, 'Username and password are required')
        );

        const submitButton = screen.getByRole('button', { name: /sign in/i });
        await userEvent.click(submitButton);

        // Form submits even with empty fields (server handles validation)
        await waitFor(() => {
            expect(global.fetch).toHaveBeenCalledWith(
                '/mcp/v1',
                expect.objectContaining({
                    method: 'POST',
                })
            );
        });
    });

    it('displays warning message from sessionStorage on mount', async () => {
        sessionStorage.setItem('disconnectMessage', 'You have been disconnected');

        render(
            <AuthProvider>
                <Login />
            </AuthProvider>
        );

        await waitFor(() => {
            expect(screen.getByText(/you have been disconnected/i)).toBeInTheDocument();
        });

        // Message should be removed from sessionStorage
        expect(sessionStorage.getItem('disconnectMessage')).toBeNull();
    });

    it('clears warning and error messages on new submission', async () => {
        sessionStorage.setItem('disconnectMessage', 'You have been disconnected');

        render(
            <AuthProvider>
                <Login />
            </AuthProvider>
        );

        await waitFor(() => {
            expect(screen.getByText(/you have been disconnected/i)).toBeInTheDocument();
        });

        // Now trigger a login failure to show error
        global.fetch.mockResolvedValueOnce(mockAuthenticateFailure(1, 'Invalid credentials'));

        const usernameInput = screen.getByLabelText(/username/i);
        const passwordInput = screen.getByLabelText(/password/i);
        const submitButton = screen.getByRole('button', { name: /sign in/i });

        await userEvent.type(usernameInput, 'test');
        await userEvent.type(passwordInput, 'test');
        await userEvent.click(submitButton);

        await waitFor(() => {
            expect(screen.getByText(/invalid credentials/i)).toBeInTheDocument();
        });

        // Warning should be cleared, only error should remain
        expect(screen.queryByText(/you have been disconnected/i)).not.toBeInTheDocument();

        // Now submit again with success
        global.fetch.mockResolvedValueOnce(mockAuthenticateSuccess(1, 'testuser'));

        await userEvent.click(submitButton);

        // Wait for submission and verify error is cleared
        await waitFor(() => {
            expect(screen.queryByText(/invalid credentials/i)).not.toBeInTheDocument();
        });
    });
});
