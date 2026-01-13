/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - MainContent Component Tests
 *
 * Portions copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import MainContent from '../MainContent';
import { AuthProvider } from '../../contexts/AuthContext';

// Mock the child components to simplify testing
vi.mock('../StatusBanner', () => ({
  default: () => <div data-testid="status-banner">StatusBanner</div>,
}));

vi.mock('../ChatInterface', () => ({
  default: () => <div data-testid="chat-interface">ChatInterface</div>,
}));

describe('MainContent Component', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  const renderMainContent = () => {
    // Mock initial auth check
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        authenticated: true,
        user: 'testuser',
      }),
    });

    return render(
      <AuthProvider>
        <MainContent />
      </AuthProvider>
    );
  };

  it('renders StatusBanner component', async () => {
    renderMainContent();

    await waitFor(() => {
      expect(screen.getByTestId('status-banner')).toBeInTheDocument();
    });
  });

  it('renders ChatInterface component', async () => {
    renderMainContent();

    await waitFor(() => {
      expect(screen.getByTestId('chat-interface')).toBeInTheDocument();
    });
  });

  it('renders both components in the correct order', async () => {
    renderMainContent();

    await waitFor(() => {
      const statusBanner = screen.getByTestId('status-banner');
      const chatInterface = screen.getByTestId('chat-interface');

      expect(statusBanner).toBeInTheDocument();
      expect(chatInterface).toBeInTheDocument();

      // StatusBanner should appear before ChatInterface in the DOM
      const parent = statusBanner.parentElement;
      const children = Array.from(parent?.children || []);
      const statusIndex = children.indexOf(statusBanner);
      const chatIndex = children.indexOf(chatInterface);

      expect(statusIndex).toBeLessThan(chatIndex);
    });
  });

  it('applies correct layout styles', async () => {
    const { container } = renderMainContent();

    await waitFor(() => {
      expect(screen.getByTestId('status-banner')).toBeInTheDocument();
    });

    // Check that the main container has flex layout
    const mainBox = container.querySelector('[class*="MuiBox"]');
    expect(mainBox).toBeInTheDocument();
  });
});
