/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - HelpPanel Component Tests
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import HelpPanel from '../HelpPanel';

describe('HelpPanel Component', () => {
    const mockOnClose = vi.fn();

    beforeEach(() => {
        mockOnClose.mockClear();
    });

    it('renders nothing when closed', () => {
        const { container } = render(<HelpPanel open={false} onClose={mockOnClose} />);

        // Drawer is rendered but hidden when closed
        expect(screen.queryByText('Help & Documentation')).not.toBeInTheDocument();
    });

    it('renders help panel when open', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        expect(screen.getByText('Help & Documentation')).toBeInTheDocument();
    });

    it('displays Getting Started section', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        expect(screen.getByText('Getting Started')).toBeInTheDocument();
        expect(screen.getByText(/allows you to interact with your PostgreSQL database/i)).toBeInTheDocument();
    });

    it('displays Chat Interface section', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        expect(screen.getByText('Chat Interface')).toBeInTheDocument();
        expect(screen.getByText('Sending Messages')).toBeInTheDocument();
        expect(screen.getByText('Query History')).toBeInTheDocument();
        expect(screen.getByText('Clear Conversation')).toBeInTheDocument();
    });

    it('displays Settings & Options section', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        expect(screen.getByText('Settings & Options')).toBeInTheDocument();
        expect(screen.getByText('LLM Provider')).toBeInTheDocument();
        expect(screen.getByText('Model Selection')).toBeInTheDocument();
        expect(screen.getByText('Show Activity')).toBeInTheDocument();
        expect(screen.getByText('Render Markdown')).toBeInTheDocument();
    });

    it('displays Tips & Best Practices section', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        expect(screen.getByText('Tips & Best Practices')).toBeInTheDocument();
        expect(screen.getByText('Be Specific')).toBeInTheDocument();
        expect(screen.getByText('Follow-up Questions')).toBeInTheDocument();
        expect(screen.getByText('Review Activity')).toBeInTheDocument();
        expect(screen.getByText('Preferences Saved')).toBeInTheDocument();
    });

    it('displays Database Connection section', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        expect(screen.getByText('Database Connection')).toBeInTheDocument();
        expect(screen.getByText(/Connection details are shown in the status banner/i)).toBeInTheDocument();
    });

    it('displays copyright information', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        // More specific check for copyright text (using getAllByText since "pgEdge Natural Language Agent" appears multiple times)
        const copyrightText = screen.getByText(/Copyright © 2025, pgEdge, Inc\./i);
        expect(copyrightText).toBeInTheDocument();
    });

    it('calls onClose when close button is clicked', async () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);
        const user = userEvent.setup();

        const closeButton = screen.getByLabelText('close help');
        await user.click(closeButton);

        expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('explains keyboard shortcuts for chat', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        expect(screen.getByText(/Shift\+Enter for new lines/i)).toBeInTheDocument();
        expect(screen.getByText(/up and down arrow keys to navigate/i)).toBeInTheDocument();
    });

    it('provides information about theme toggle', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        expect(screen.getByText('Theme')).toBeInTheDocument();
        expect(screen.getByText(/sun\/moon icon in the header/i)).toBeInTheDocument();
    });

    it('mentions all LLM providers', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        const llmProviderText = screen.getByText(/Anthropic, OpenAI, or Ollama/i);
        expect(llmProviderText).toBeInTheDocument();
    });

    it('explains activity display feature', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        const activityText = screen.getByText(/tools and resources being used by the AI/i);
        expect(activityText).toBeInTheDocument();
    });

    it('explains markdown rendering toggle', () => {
        render(<HelpPanel open={true} onClose={mockOnClose} />);

        const markdownText = screen.getByText(/enable\/disable markdown rendering/i);
        expect(markdownText).toBeInTheDocument();
    });

});
