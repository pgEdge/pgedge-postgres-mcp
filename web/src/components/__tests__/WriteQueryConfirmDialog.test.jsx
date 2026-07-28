/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - WriteQueryConfirmDialog Component Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ThemeProvider, createTheme } from '@mui/material';
import WriteQueryConfirmDialog from '../WriteQueryConfirmDialog';

const theme = createTheme();

const renderWithTheme = (ui) =>
    render(<ThemeProvider theme={theme}>{ui}</ThemeProvider>);

describe('WriteQueryConfirmDialog Component', () => {
    const mockOnClose = vi.fn();
    const mockOnConfirm = vi.fn();
    const defaultQuery = 'UPDATE users SET name = \'test\' WHERE id = 1;';

    beforeEach(() => {
        mockOnClose.mockClear();
        mockOnConfirm.mockClear();
    });

    it('renders dialog when open', () => {
        renderWithTheme(
            <WriteQueryConfirmDialog
                open={true}
                onClose={mockOnClose}
                onConfirm={mockOnConfirm}
                query={defaultQuery}
            />
        );

        expect(screen.getByText('Confirm Write Operation')).toBeInTheDocument();
        expect(screen.getByText(defaultQuery)).toBeInTheDocument();
        expect(screen.getByText('Cancel')).toBeInTheDocument();
        expect(screen.getByText('Execute')).toBeInTheDocument();
    });

    it('does not render when closed', () => {
        renderWithTheme(
            <WriteQueryConfirmDialog
                open={false}
                onClose={mockOnClose}
                onConfirm={mockOnConfirm}
                query={defaultQuery}
            />
        );

        expect(
            screen.queryByText('Confirm Write Operation')
        ).not.toBeInTheDocument();
    });

    it('displays the SQL query', () => {
        const query = 'DROP TABLE users;';

        renderWithTheme(
            <WriteQueryConfirmDialog
                open={true}
                onClose={mockOnClose}
                onConfirm={mockOnConfirm}
                query={query}
            />
        );

        expect(screen.getByText('DROP TABLE users;')).toBeInTheDocument();
    });

    it('calls onConfirm when Execute is clicked', async () => {
        const user = userEvent.setup();

        renderWithTheme(
            <WriteQueryConfirmDialog
                open={true}
                onClose={mockOnClose}
                onConfirm={mockOnConfirm}
                query={defaultQuery}
            />
        );

        await user.click(screen.getByText('Execute'));

        expect(mockOnConfirm).toHaveBeenCalledTimes(1);
    });

    it('calls onClose when Cancel is clicked', async () => {
        const user = userEvent.setup();

        renderWithTheme(
            <WriteQueryConfirmDialog
                open={true}
                onClose={mockOnClose}
                onConfirm={mockOnConfirm}
                query={defaultQuery}
            />
        );

        await user.click(screen.getByText('Cancel'));

        expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('calls onClose when Escape key is pressed', async () => {
        const user = userEvent.setup();

        renderWithTheme(
            <WriteQueryConfirmDialog
                open={true}
                onClose={mockOnClose}
                onConfirm={mockOnConfirm}
                query={defaultQuery}
            />
        );

        await user.keyboard('{Escape}');

        expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('displays long queries with scroll', () => {
        const longQuery = Array.from(
            { length: 50 },
            (_, i) =>
                `INSERT INTO logs (id, message) VALUES (${i}, 'Log entry ${i}');`
        ).join('\n');

        renderWithTheme(
            <WriteQueryConfirmDialog
                open={true}
                onClose={mockOnClose}
                onConfirm={mockOnConfirm}
                query={longQuery}
            />
        );

        // Use partial match since getByText exact matching normalizes
        // whitespace for multiline strings
        expect(
            screen.getByText(/INSERT INTO logs.*VALUES \(0,/)
        ).toBeInTheDocument();
        expect(
            screen.getByText(/INSERT INTO logs.*VALUES \(49,/)
        ).toBeInTheDocument();
    });

    it('displays the descriptive text about modifying data', () => {
        renderWithTheme(
            <WriteQueryConfirmDialog
                open={true}
                onClose={mockOnClose}
                onConfirm={mockOnConfirm}
                query={defaultQuery}
            />
        );

        expect(
            screen.getByText('The following operation can modify data:')
        ).toBeInTheDocument();
    });

    it('renders with an empty query string by default', () => {
        renderWithTheme(
            <WriteQueryConfirmDialog
                open={true}
                onClose={mockOnClose}
                onConfirm={mockOnConfirm}
            />
        );

        expect(screen.getByText('Confirm Write Operation')).toBeInTheDocument();
        expect(screen.getByText('Cancel')).toBeInTheDocument();
        expect(screen.getByText('Execute')).toBeInTheDocument();
    });
});
