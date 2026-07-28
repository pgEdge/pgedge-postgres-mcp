/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Query Classifier Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect } from 'vitest';
import {
    isWriteQuery,
    toolMayWrite,
    writeConfirmationSubject,
    describeToolCall,
} from './queryClassify';

describe('isWriteQuery', () => {
    describe('read queries return false', () => {
        it('classifies SELECT as read', () => {
            expect(isWriteQuery('SELECT * FROM users')).toBe(false);
        });

        it('classifies WITH (CTE) as read', () => {
            expect(isWriteQuery('WITH cte AS (SELECT 1) SELECT * FROM cte')).toBe(false);
        });

        it('classifies TABLE as read', () => {
            expect(isWriteQuery('TABLE users')).toBe(false);
        });

        it('classifies VALUES as read', () => {
            expect(isWriteQuery('VALUES (1, 2, 3)')).toBe(false);
        });

        it('classifies EXPLAIN as read', () => {
            expect(isWriteQuery('EXPLAIN SELECT * FROM users')).toBe(false);
        });

        it('classifies SHOW as read', () => {
            expect(isWriteQuery('SHOW search_path')).toBe(false);
        });
    });

    describe('DDL write queries return true', () => {
        it('classifies CREATE as write', () => {
            expect(isWriteQuery('CREATE TABLE users (id int)')).toBe(true);
        });

        it('classifies DROP as write', () => {
            expect(isWriteQuery('DROP TABLE users')).toBe(true);
        });

        it('classifies ALTER as write', () => {
            expect(isWriteQuery('ALTER TABLE users ADD COLUMN name text')).toBe(true);
        });

        it('classifies TRUNCATE as write', () => {
            expect(isWriteQuery('TRUNCATE TABLE users')).toBe(true);
        });
    });

    describe('DML write queries return true', () => {
        it('classifies INSERT as write', () => {
            expect(isWriteQuery('INSERT INTO users (name) VALUES (\'test\')')).toBe(true);
        });

        it('classifies UPDATE as write', () => {
            expect(isWriteQuery('UPDATE users SET name = \'test\'')).toBe(true);
        });

        it('classifies DELETE as write', () => {
            expect(isWriteQuery('DELETE FROM users WHERE id = 1')).toBe(true);
        });
    });

    describe('case insensitivity', () => {
        it('handles lowercase SELECT', () => {
            expect(isWriteQuery('select * from users')).toBe(false);
        });

        it('handles mixed case SELECT', () => {
            expect(isWriteQuery('Select * From users')).toBe(false);
        });

        it('handles lowercase INSERT', () => {
            expect(isWriteQuery('insert into users values (1)')).toBe(true);
        });

        it('handles mixed case CREATE', () => {
            expect(isWriteQuery('Create Table test (id int)')).toBe(true);
        });
    });

    describe('leading whitespace', () => {
        it('handles leading spaces before SELECT', () => {
            expect(isWriteQuery('   SELECT * FROM users')).toBe(false);
        });

        it('handles leading tabs before INSERT', () => {
            expect(isWriteQuery('\tINSERT INTO users VALUES (1)')).toBe(true);
        });

        it('handles leading newlines before DELETE', () => {
            expect(isWriteQuery('\n  DELETE FROM users')).toBe(true);
        });
    });

    describe('unknown queries treated as write', () => {
        it('classifies GRANT as write', () => {
            expect(isWriteQuery('GRANT SELECT ON users TO role')).toBe(true);
        });

        it('classifies REVOKE as write', () => {
            expect(isWriteQuery('REVOKE ALL ON users FROM role')).toBe(true);
        });

        it('classifies VACUUM as write', () => {
            expect(isWriteQuery('VACUUM users')).toBe(true);
        });

        it('classifies REINDEX as write', () => {
            expect(isWriteQuery('REINDEX TABLE users')).toBe(true);
        });
    });

    describe('edge cases', () => {
        it('returns false for null', () => {
            expect(isWriteQuery(null)).toBe(false);
        });

        it('returns false for undefined', () => {
            expect(isWriteQuery(undefined)).toBe(false);
        });

        it('returns false for empty string', () => {
            expect(isWriteQuery('')).toBe(false);
        });

        it('returns false for non-string (number)', () => {
            expect(isWriteQuery(42)).toBe(false);
        });

        it('returns false for non-string (object)', () => {
            expect(isWriteQuery({})).toBe(false);
        });

        it('returns false for non-string (array)', () => {
            expect(isWriteQuery([])).toBe(false);
        });

        it('returns false for non-string (boolean)', () => {
            expect(isWriteQuery(true)).toBe(false);
        });
    });
});

describe('toolMayWrite', () => {
    const tools = [
        { name: 'query_database' },
        { name: 'custom_writer', annotations: { readOnlyHint: false } },
        { name: 'custom_reader', annotations: { readOnlyHint: true } },
        { name: 'unannotated' },
    ];

    it('reports true only for an explicit readOnlyHint of false', () => {
        expect(toolMayWrite(tools, 'custom_writer')).toBe(true);
        expect(toolMayWrite(tools, 'custom_reader')).toBe(false);
    });

    it('treats a missing annotation as read-only', () => {
        expect(toolMayWrite(tools, 'unannotated')).toBe(false);
        expect(toolMayWrite(tools, 'query_database')).toBe(false);
    });

    it('handles an unknown tool and a missing tool list', () => {
        expect(toolMayWrite(tools, 'no_such_tool')).toBe(false);
        expect(toolMayWrite(undefined, 'custom_writer')).toBe(false);
        expect(toolMayWrite(null, 'custom_writer')).toBe(false);
    });
});

describe('writeConfirmationSubject', () => {
    const tools = [
        { name: 'query_database' },
        { name: 'custom_writer', annotations: { readOnlyHint: false } },
        { name: 'custom_reader', annotations: { readOnlyHint: true } },
    ];

    it('confirms a write statement and shows the SQL', () => {
        const { needsConfirmation, subject } = writeConfirmationSubject(
            tools, 'query_database', { query: 'DELETE FROM users' });
        expect(needsConfirmation).toBe(true);
        expect(subject).toBe('DELETE FROM users');
    });

    it('does not interrupt a read statement', () => {
        const { needsConfirmation } = writeConfirmationSubject(
            tools, 'query_database', { query: 'SELECT * FROM users' });
        expect(needsConfirmation).toBe(false);
    });

    it('confirms an unclassifiable statement', () => {
        const { needsConfirmation } = writeConfirmationSubject(
            tools, 'query_database', { query: '/* c */ INSERT INTO t VALUES (1)' });
        expect(needsConfirmation).toBe(true);
    });

    it('ignores a query_database call with no query argument', () => {
        expect(writeConfirmationSubject(tools, 'query_database', {})
            .needsConfirmation).toBe(false);
        expect(writeConfirmationSubject(tools, 'query_database', undefined)
            .needsConfirmation).toBe(false);
    });

    // Custom tools can write and were previously never confirmed, because the
    // check was keyed to the query_database tool name.
    it('confirms a custom tool that advertises that it may write', () => {
        const { needsConfirmation, subject } = writeConfirmationSubject(
            tools, 'custom_writer', { id: 7 });
        expect(needsConfirmation).toBe(true);
        expect(subject).toContain('custom_writer');
        expect(subject).toContain('7');
    });

    it('does not confirm a custom tool marked read-only', () => {
        expect(writeConfirmationSubject(tools, 'custom_reader', { id: 7 })
            .needsConfirmation).toBe(false);
    });
});

describe('describeToolCall', () => {
    it('renders a call with no arguments', () => {
        expect(describeToolCall('do_thing', {})).toBe('do_thing()');
        expect(describeToolCall('do_thing', undefined)).toBe('do_thing()');
    });

    it('renders arguments as formatted JSON', () => {
        const text = describeToolCall('do_thing', { a: 1 });
        expect(text).toContain('do_thing');
        expect(text).toContain('"a": 1');
    });
});
