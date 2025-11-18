/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - MainContent Component Tests
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import MainContent from '../MainContent';

describe('MainContent Component', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  it('shows loading state initially', () => {
    global.fetch.mockImplementation(() => new Promise(() => {})); // Never resolves

    render(<MainContent />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('displays system information on successful fetch', async () => {
    const mockSystemInfo = {
      postgresql_version: '17.4 (Homebrew)',
      operating_system: 'darwin24.2.0',
      architecture: 'aarch64-apple-darwin24.2.0',
      bit_version: '64-bit',
      compiler: 'Apple clang version 16.0.0',
      full_version: 'PostgreSQL 17.4 (Homebrew) on aarch64-apple-darwin24.2.0',
    };

    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockSystemInfo,
    });

    render(<MainContent />);

    await waitFor(() => {
      expect(screen.getByText('17.4 (Homebrew)')).toBeInTheDocument();
      expect(screen.getByText('darwin24.2.0')).toBeInTheDocument();
      expect(screen.getByText('aarch64-apple-darwin24.2.0')).toBeInTheDocument();
      expect(screen.getByText('64-bit')).toBeInTheDocument();
    });

    expect(screen.getByText('Connected')).toBeInTheDocument();
  });

  it('displays error message on fetch failure', async () => {
    global.fetch.mockRejectedValueOnce(new Error('Network error'));

    render(<MainContent />);

    await waitFor(() => {
      expect(screen.getByText(/failed to load system information/i)).toBeInTheDocument();
    });
  });

  it('displays N/A for missing system info fields', async () => {
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        postgresql_version: '17.4',
        // Other fields missing
      }),
    });

    render(<MainContent />);

    await waitFor(() => {
      expect(screen.getByText('17.4')).toBeInTheDocument();
    });

    // Check for N/A in fields that are missing
    const naElements = screen.getAllByText('N/A');
    expect(naElements.length).toBeGreaterThan(0);
  });

  it('refreshes data periodically', async () => {
    vi.useFakeTimers();

    const mockSystemInfo = {
      postgresql_version: '17.4',
      operating_system: 'linux',
    };

    global.fetch.mockResolvedValue({
      ok: true,
      json: async () => mockSystemInfo,
    });

    render(<MainContent />);

    // Wait for initial fetch
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledTimes(1);
    });

    // Fast-forward 30 seconds
    vi.advanceTimersByTime(30000);

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledTimes(2);
    });

    vi.useRealTimers();
  });

  it('displays all system info cards', async () => {
    const mockSystemInfo = {
      postgresql_version: '17.4',
      operating_system: 'linux',
      architecture: 'x86_64',
      bit_version: '64-bit',
      compiler: 'gcc',
      full_version: 'PostgreSQL 17.4',
    };

    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockSystemInfo,
    });

    render(<MainContent />);

    await waitFor(() => {
      expect(screen.getByText('PostgreSQL Version')).toBeInTheDocument();
      expect(screen.getByText('Operating System')).toBeInTheDocument();
      expect(screen.getByText('Architecture')).toBeInTheDocument();
      expect(screen.getByText('Bit Version')).toBeInTheDocument();
      expect(screen.getByText('Compiler')).toBeInTheDocument();
      expect(screen.getByText('Full Version String')).toBeInTheDocument();
    });
  });
});
