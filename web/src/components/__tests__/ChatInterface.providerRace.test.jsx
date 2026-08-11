/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - ChatInterface Provider/Model Race Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

const mockCallTool = vi.fn();
const mockGetPrompt = vi.fn();
const mockSseChat = vi.fn();

vi.mock('../../contexts/AuthContext', () => ({
    useAuth: () => ({ sessionToken: 'test-token', forceLogout: vi.fn() }),
}));

vi.mock('../../contexts/LLMProcessingContext', () => ({
    useLLMProcessing: () => ({ setIsProcessing: vi.fn() }),
}));

vi.mock('../../contexts/DatabaseContext', () => ({
    useDatabaseContext: () => ({
        databases: [],
        currentDatabase: null,
        selectDatabase: vi.fn(),
        fetchDatabases: vi.fn(),
    }),
}));

vi.mock('../../contexts/ConversationActionsContext', () => ({
    useConversationActions: () => ({ registerActions: vi.fn() }),
}));

vi.mock('../../hooks/useLocalStorage', () => ({
    useLocalStorageBoolean: () => [false, vi.fn()],
}));

vi.mock('../../hooks/useQueryHistory', () => ({
    useQueryHistory: () => ({
        addToHistory: vi.fn(),
        clearHistory: vi.fn(),
        setHistory: vi.fn(),
        navigateUp: (current) => current,
        navigateDown: (current) => current,
        isNavigating: false,
        resetNavigation: vi.fn(),
    }),
}));

vi.mock('../../hooks/useMCPClient', () => ({
    useMCPClient: () => ({
        mcpClient: { callTool: mockCallTool, getPrompt: mockGetPrompt },
        tools: [{ name: 'query_database', inputSchema: {} }],
        prompts: [{ name: 'test_prompt', arguments: [] }],
        refreshTools: vi.fn(),
    }),
}));

// The case under test: selectedProvider has already flipped to the newly
// chosen provider, but its model list is still loading (loadingModels:
// true) - the exact window in which a provider switch can be paired with
// the *previous* provider's model.
let mockLoadingModels = false;

vi.mock('../../hooks/useLLMProviders', () => ({
    useLLMProviders: () => ({
        providers: [{ name: 'openai', display: 'OpenAI', model: 'gpt-4', isDefault: false }],
        selectedProvider: 'openai',
        // Whilst the fetch is in flight the model is still the previous
        // provider's; once it resolves the hook has replaced it with one
        // the new provider actually advertises.
        selectedModel: mockLoadingModels ? 'gemini-flash-latest' : 'gpt-4',
        setSelectedProvider: vi.fn(),
        models: mockLoadingModels ? [] : [{ name: 'gpt-4', description: '' }],
        setSelectedModel: vi.fn(),
        loadingProviders: false,
        loadingModels: mockLoadingModels,
        error: '',
        restoreProviderAndModel: vi.fn(),
    }),
}));

vi.mock('../../utils/sseChat', () => ({
    sseChat: (...args) => mockSseChat(...args),
}));

vi.mock('../MessageList', () => ({
    default: ({ messages }) => (
        <div>
            {messages.map((msg, index) => (
                <div key={index} data-testid="message" data-role={msg.role}
                    data-thinking={msg.isThinking ? 'true' : 'false'}>
                    {typeof msg.content === 'string'
                        ? msg.content
                        : JSON.stringify(msg.content)}
                </div>
            ))}
        </div>
    ),
}));

vi.mock('../MessageInput', () => ({
    default: ({ value, onChange, onSend, disabled }) => (
        <div>
            <textarea aria-label="prompt" value={value} onChange={onChange} disabled={disabled} />
            <button type="button" onClick={onSend} disabled={disabled}>
                Send
            </button>
        </div>
    ),
}));

vi.mock('../ProviderSelector', () => ({ default: () => null }));
vi.mock('../PromptPopover', () => ({
    default: ({ onExecute }) => (
        <button type="button" onClick={() => onExecute('test_prompt', {})}>
            Run prompt
        </button>
    ),
}));
vi.mock('../WriteQueryConfirmDialog', () => ({ default: () => null }));

const ChatInterface = (await import('../ChatInterface')).default;

describe('ChatInterface provider/model switch race', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        mockLoadingModels = false;
    });

    it("blocks sending while the new provider's model list is still loading", async () => {
        mockLoadingModels = true;
        render(<ChatInterface />);

        const textarea = screen.getByLabelText('prompt');
        expect(textarea).toBeDisabled();
        expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled();

        fireEvent.change(textarea, { target: { value: 'how many users are there?' } });
        fireEvent.click(screen.getByRole('button', { name: 'Send' }));

        await new Promise((r) => setTimeout(r, 50));
        expect(mockSseChat).not.toHaveBeenCalled();
    });

    it('allows sending once the model list has finished loading', async () => {
        mockLoadingModels = false;
        mockSseChat.mockResolvedValue({
            stop_reason: 'end_turn',
            content: [{ type: 'text', text: 'done' }],
            usage: { prompt_tokens: 1, completion_tokens: 1, total_tokens: 2 },
        });
        render(<ChatInterface />);

        const textarea = screen.getByLabelText('prompt');
        expect(textarea).not.toBeDisabled();

        fireEvent.change(textarea, { target: { value: 'how many users are there?' } });
        fireEvent.click(screen.getByRole('button', { name: 'Send' }));

        await waitFor(() => expect(mockSseChat).toHaveBeenCalledTimes(1));

        // Whatever request went out was built with the provider/model
        // pairing the hook actually reports, never a mismatched leftover.
        // Asserting the model matters as much as the provider: the bug
        // being fixed sent the right provider with the wrong model, so a
        // test that checks only the provider would pass on the very
        // payload this PR exists to prevent.
        const [requestBody] = mockSseChat.mock.calls[0];
        expect(requestBody.provider).toBe('openai');
        expect(requestBody.model).toBe('gpt-4');
    });
});
