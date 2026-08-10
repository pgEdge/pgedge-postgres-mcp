/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - ChatInterface Repeated Tool Failure Guard Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

// The component pulls in a lot of application context; stub every hook and
// child so the tests can concentrate on the agentic loop itself.
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

vi.mock('../../hooks/useLLMProviders', () => ({
    useLLMProviders: () => ({
        providers: ['anthropic'],
        selectedProvider: 'anthropic',
        models: ['test-model'],
        selectedModel: 'test-model',
        setSelectedProvider: vi.fn(),
        setSelectedModel: vi.fn(),
        loadingModels: false,
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
            <textarea aria-label="prompt" value={value} onChange={onChange} />
            <button type="button" onClick={onSend} disabled={disabled}>
                Send
            </button>
        </div>
    ),
}));

vi.mock('../ProviderSelector', () => ({ default: () => null }));
// The popover is reduced to a single button so the prompt-driven agentic
// loop can be exercised without driving the real popover UI.
vi.mock('../PromptPopover', () => ({
    default: ({ onExecute }) => (
        <button type="button" onClick={() => onExecute('test_prompt', {})}>
            Run prompt
        </button>
    ),
}));
vi.mock('../WriteQueryConfirmDialog', () => ({ default: () => null }));

const ChatInterface = (await import('../ChatInterface')).default;

/**
 * Builds an LLM response that asks for one tool call.
 * @param {string} id - The tool_use id.
 * @param {object} input - The arguments for the call.
 * @returns {object} - A tool_use response envelope.
 */
const toolUseResponse = (id, input) => ({
    stop_reason: 'tool_use',
    content: [{
        type: 'tool_use',
        tool_use: { id, name: 'query_database', input },
    }],
    usage: { prompt_tokens: 10, completion_tokens: 5, total_tokens: 15 },
});

/**
 * Builds a final LLM response carrying plain text.
 * @param {string} text - The assistant's reply.
 * @returns {object} - An end_turn response envelope.
 */
const endTurnResponse = (text) => ({
    stop_reason: 'end_turn',
    content: [{ type: 'text', text }],
    usage: { prompt_tokens: 10, completion_tokens: 5, total_tokens: 15 },
});

/**
 * A failing MCP tool result.
 * @param {string} text - The error text.
 * @returns {object} - An MCP result marked as an error.
 */
const errorResult = (text) => ({
    content: [{ type: 'text', text }],
    isError: true,
});

/**
 * A successful MCP tool result.
 * @param {string} text - The result text.
 * @returns {object} - An MCP result.
 */
const okResult = (text) => ({
    content: [{ type: 'text', text }],
    isError: false,
});

/**
 * Renders the component and sends a message through it.
 * @returns {Promise<void>} - Resolves once the send has been dispatched.
 */
const sendMessage = async () => {
    render(<ChatInterface />);
    fireEvent.change(screen.getByLabelText('prompt'), {
        target: { value: 'how many users are there?' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Send' }));
};

/**
 * Waits until the conversation has settled, then returns the text of the
 * final assistant message.
 * @returns {Promise<string>} - The final message text.
 */
const finalMessageText = async () => {
    await waitFor(() => {
        const messages = screen.getAllByTestId('message');
        const last = messages[messages.length - 1];
        expect(last.dataset.thinking).toBe('false');
        expect(last.dataset.role).toBe('assistant');
    }, { timeout: 5000 });
    const messages = screen.getAllByTestId('message');
    return messages[messages.length - 1].textContent;
};

describe('ChatInterface repeated tool failure guard', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        vi.spyOn(console, 'log').mockImplementation(() => {});
        vi.spyOn(console, 'error').mockImplementation(() => {});
    });

    it('stops after three identical failing calls', async () => {
        mockSseChat.mockImplementation(async () =>
            toolUseResponse('call-1', { sql: 'SELECT count(*) FROM users' }));
        mockCallTool.mockResolvedValue(
            errorResult('ERROR: relation "users" does not exist'));

        await sendMessage();
        const text = await finalMessageText();

        expect(mockCallTool).toHaveBeenCalledTimes(3);
        expect(mockSseChat).toHaveBeenCalledTimes(3);
        expect(text).toContain('query_database');
        expect(text).toContain('3 times');
        expect(text).toContain('relation "users" does not exist');
    });

    it('ignores argument key ordering when matching repeats', async () => {
        const inputs = [
            { sql: 'SELECT 1', limit: 10 },
            { limit: 10, sql: 'SELECT 1' },
            { sql: 'SELECT 1', limit: 10 },
            { limit: 10, sql: 'SELECT 1' },
        ];
        let call = 0;
        mockSseChat.mockImplementation(async () =>
            toolUseResponse(`call-${call}`, inputs[call++] || inputs[0]));
        mockCallTool.mockResolvedValue(errorResult('ERROR: broken'));

        await sendMessage();
        const text = await finalMessageText();

        expect(mockCallTool).toHaveBeenCalledTimes(3);
        expect(text).toContain('3 times');
    });

    it('does not trip when the arguments differ each time', async () => {
        let call = 0;
        mockSseChat.mockImplementation(async () => {
            call++;
            if (call > 4) return endTurnResponse('I gave up gracefully.');
            return toolUseResponse(`call-${call}`, { sql: `SELECT ${call}` });
        });
        mockCallTool.mockResolvedValue(errorResult('ERROR: broken'));

        await sendMessage();
        const text = await finalMessageText();

        expect(mockCallTool).toHaveBeenCalledTimes(4);
        expect(text).toBe('I gave up gracefully.');
    });

    it('resets the count when the same call later succeeds', async () => {
        const outcomes = [
            errorResult('ERROR: broken'),
            errorResult('ERROR: broken'),
            okResult('42'),
            errorResult('ERROR: broken'),
            errorResult('ERROR: broken'),
        ];
        let call = 0;
        mockSseChat.mockImplementation(async () => {
            call++;
            if (call > outcomes.length) {
                return endTurnResponse('All done.');
            }
            return toolUseResponse(`call-${call}`, { sql: 'SELECT 1' });
        });
        let outcome = 0;
        mockCallTool.mockImplementation(async () => outcomes[outcome++]);

        await sendMessage();
        const text = await finalMessageText();

        expect(text).toBe('All done.');
        expect(mockCallTool).toHaveBeenCalledTimes(5);
    });

    it('guards the prompt-driven loop as well', async () => {
        mockGetPrompt.mockResolvedValue({
            messages: [{ role: 'user', content: { text: 'audit the database' } }],
        });
        mockSseChat.mockImplementation(async () =>
            toolUseResponse('call-1', { sql: 'SELECT 1' }));
        mockCallTool.mockResolvedValue(errorResult('ERROR: broken'));

        render(<ChatInterface />);
        fireEvent.click(screen.getByRole('button', { name: 'Run prompt' }));
        const text = await finalMessageText();

        expect(mockCallTool).toHaveBeenCalledTimes(3);
        expect(mockSseChat).toHaveBeenCalledTimes(3);
        expect(text).toContain('query_database');
        expect(text).toContain('3 times');
        expect(text).toContain('ERROR: broken');
    });

    it('leaves a successful conversation untouched', async () => {
        let call = 0;
        mockSseChat.mockImplementation(async () => {
            call++;
            if (call === 1) {
                return toolUseResponse('call-1', { sql: 'SELECT 1' });
            }
            return endTurnResponse('There are 42 users.');
        });
        mockCallTool.mockResolvedValue(okResult('42'));

        await sendMessage();
        const text = await finalMessageText();

        expect(mockCallTool).toHaveBeenCalledTimes(1);
        expect(mockSseChat).toHaveBeenCalledTimes(2);
        expect(text).toBe('There are 42 users.');
    });
});
