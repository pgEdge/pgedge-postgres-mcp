/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - SQL Text Scanner Tests
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect } from 'vitest';
import { stripSqlNoise } from './sqlText';

// These cases mirror TestStrip in internal/sqltext/sqltext_test.go, so that
// the two scanners can be compared directly as either one changes. The Go
// scanner also returns a "bare" form with literals intact; this one does not,
// because no web client check needs to see inside a literal.
describe('stripSqlNoise', () => {
    const cases = [
        ['leaves a plain statement unchanged',
            'SELECT 1', 'SELECT 1'],
        ['blanks a string literal',
            "SELECT 'DELETE'", "SELECT ''"],
        ['blanks a quoted identifier',
            'SELECT "delete" FROM t', 'SELECT "" FROM t'],
        ['does not end a literal early on a doubled quote',
            "SELECT 'it''s DELETE'", "SELECT ''"],
        ['turns a line comment into a separator',
            'SELECT 1 -- DROP TABLE t\n', 'SELECT 1  \n'],
        ['turns a block comment into a separator',
            'SEL/* hidden */ECT 1', 'SEL ECT 1'],
        ['consumes a nested block comment whole',
            'SELECT /* a /* b */ c */ 1', 'SELECT   1'],
        ['blanks a dollar quoted body',
            'DO $tag$ DROP TABLE t $tag$', 'DO $tag$$tag$'],
        ['leaves a positional parameter alone',
            'SELECT * FROM t WHERE id = $1', 'SELECT * FROM t WHERE id = $1'],
        ['lets an unterminated literal consume the remainder',
            "SELECT 'oops", "SELECT ''"],
        ['lets an unclosed dollar quote fall through as code',
            'SELECT $tag$ DROP TABLE t', 'SELECT $tag$ DROP TABLE t'],
        ['does not let a $ continuing an identifier start a tag',
            'SELECT 1 AS x$tag$; DELETE FROM t -- $tag$',
            'SELECT 1 AS x$tag$; DELETE FROM t  '],
        ['does not let a dollar quote without a tag continue an identifier',
            'SELECT 1 AS x$$; DELETE FROM t -- $$',
            'SELECT 1 AS x$$; DELETE FROM t  '],
        ['still opens a dollar quote that follows a space',
            'DO $ $tag$ DROP TABLE t $tag$', 'DO $ $tag$$tag$'],
        ['does not let a $ continuing an identifier hide a SELECT INTO',
            'SELECT 1 AS x$tag$ INTO backup FROM users -- $tag$',
            'SELECT 1 AS x$tag$ INTO backup FROM users  '],
    ];

    it.each(cases)('%s', (_name, input, expected) => {
        expect(stripSqlNoise(input)).toBe(expected);
    });

    // Read as a doubled quote rather than an escape, E'\'' would run to the
    // end of the statement and hide the INTO behind it, which is a write
    // going unseen by the confirmation prompt.
    describe('escape string constants', () => {
        it('does not let a backslash-quote close the literal', () => {
            expect(stripSqlNoise("SELECT E'\\'' INTO backup FROM users"))
                .toBe("SELECT E'' INTO backup FROM users");
        });

        it('handles a backslash escape before the closing quote', () => {
            expect(stripSqlNoise("SELECT E'a\\'' , x INTO t FROM u"))
                .toBe("SELECT E'' , x INTO t FROM u");
        });

        it('accepts a lowercase e prefix', () => {
            expect(stripSqlNoise("SELECT e'\\'' INTO t"))
                .toBe("SELECT e'' INTO t");
        });

        it('does not let an escaped backslash escape the closing quote', () => {
            expect(stripSqlNoise("SELECT E'a\\\\' INTO t"))
                .toBe("SELECT E'' INTO t");
        });

        it('does not honour a backslash in a plain literal', () => {
            expect(stripSqlNoise("SELECT 'a\\' , x INTO t FROM u"))
                .toBe("SELECT '' , x INTO t FROM u");
        });

        it('does not treat an E ending an identifier as a prefix', () => {
            expect(stripSqlNoise("SELECT * FROM table_e'a\\' , x"))
                .toBe("SELECT * FROM table_e'' , x");
        });
    });
});
