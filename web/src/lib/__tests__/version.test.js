/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Client Version Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect } from 'vitest';
import { CLIENT_VERSION } from '../mcp-client';
import pkg from '../../../package.json';

// CLIENT_VERSION was a literal in mcp-client.js and sat at 1.0.0-alpha5
// through five subsequent releases, showing a stale version in the help
// panel and, more importantly, sending one as clientInfo.version in every
// MCP initialize handshake. It is now injected from package.json, and these
// tests fail if anyone reintroduces a literal that can drift from it.
describe('CLIENT_VERSION', () => {
    it('matches the version in package.json', () => {
        expect(CLIENT_VERSION).toBe(pkg.version);
    });

    it('is a non-empty semver-shaped string', () => {
        // Guards against the define being dropped from a config, which would
        // otherwise surface as undefined in the handshake rather than failing
        // anything.
        expect(typeof CLIENT_VERSION).toBe('string');
        expect(CLIENT_VERSION).toMatch(/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/);
    });
});
