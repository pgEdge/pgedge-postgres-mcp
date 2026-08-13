/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Modern Protocol Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { MCPClient, MODERN_PROTOCOL_VERSION } from '../mcp-client';

// Node and jsdom both supply a global fetch, so put the original back
// rather than deleting the property; other suites sharing this worker
// would otherwise find it missing.
const originalFetch = globalThis.fetch;

describe('MCPClient modern protocol', () => {
    let calls;

    beforeEach(() => {
        calls = [];
        globalThis.fetch = vi.fn((url, options) => {
            const body = JSON.parse(options.body);
            calls.push({ url, options, body });

            let result;
            if (body.method === 'server/discover') {
                result = {
                    _meta: {
                        'io.modelcontextprotocol/serverInfo': {
                            name: 'test-server',
                            version: '1.0.0'
                        }
                    }
                };
            } else if (body.method === 'tools/list') {
                result = { tools: [] };
            } else {
                result = {};
            }

            return Promise.resolve({
                ok: true,
                status: 200,
                json: () => Promise.resolve({ jsonrpc: '2.0', id: body.id, result })
            });
        });
    });

    afterEach(() => {
        globalThis.fetch = originalFetch;
    });

    it('initializes via server/discover, not the legacy handshake', async () => {
        const client = new MCPClient('/mcp/v1');
        await client.initialize();

        expect(calls).toHaveLength(1);
        expect(calls[0].body.method).toBe('server/discover');
        expect(client.getServerInfo()).toEqual({
            name: 'test-server',
            version: '1.0.0'
        });
    });

    it('sends the modern _meta and headers on every request', async () => {
        const client = new MCPClient('/mcp/v1');
        await client.listTools();

        const { body, options } = calls[0];
        expect(body.params._meta).toEqual({
            'io.modelcontextprotocol/protocolVersion': MODERN_PROTOCOL_VERSION,
            'io.modelcontextprotocol/clientCapabilities': {}
        });
        expect(options.headers['MCP-Protocol-Version']).toBe(MODERN_PROTOCOL_VERSION);
        expect(options.headers['Mcp-Method']).toBe('tools/list');
        expect(options.headers['Mcp-Name']).toBeUndefined();
    });

    it('sets Mcp-Name from params.name for tools/call', async () => {
        const client = new MCPClient('/mcp/v1');
        await client.callTool('test_tool', { arg: 'value' });

        expect(calls[0].options.headers['Mcp-Name']).toBe('test_tool');
    });

    it('sets Mcp-Name from params.uri for resources/read', async () => {
        const client = new MCPClient('/mcp/v1');
        await client.readResource('pg://some/resource');

        expect(calls[0].options.headers['Mcp-Name']).toBe('pg://some/resource');
    });

    it('sets Mcp-Name from params.name for prompts/get', async () => {
        const client = new MCPClient('/mcp/v1');
        await client.getPrompt('my_prompt', {});

        expect(calls[0].options.headers['Mcp-Name']).toBe('my_prompt');
    });

    it('base64-wraps a non-ASCII Mcp-Name using the =?base64?...?= sentinel', async () => {
        const client = new MCPClient('/mcp/v1');
        await client.callTool('tést_tool', {});

        const header = calls[0].options.headers['Mcp-Name'];
        expect(header).toMatch(/^=\?base64\?.+\?=$/);

        const encoded = header.slice('=?base64?'.length, -'?='.length);
        const decoded = new TextDecoder().decode(
            Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0))
        );
        expect(decoded).toBe('tést_tool');
    });

    it.each([
        ['leading whitespace', ' leading_space'],
        ['trailing whitespace', 'trailing_space '],
        ['a value already shaped like the base64 sentinel', '=?base64?aGk=?=']
    ])('base64-wraps a name with %s and round-trips', async (_desc, original) => {
        const client = new MCPClient('/mcp/v1');
        await client.callTool(original, {});

        const header = calls[0].options.headers['Mcp-Name'];
        expect(header).toMatch(/^=\?base64\?.+\?=$/);

        const encoded = header.slice('=?base64?'.length, -'?='.length);
        const decoded = new TextDecoder().decode(
            Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0))
        );
        expect(decoded).toBe(original);
    });
});
