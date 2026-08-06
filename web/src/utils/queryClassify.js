/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { stripSqlNoise } from './sqlText';

const WRITE_PREFIXES = [
    'CREATE', 'DROP', 'ALTER', 'TRUNCATE',
    'INSERT', 'UPDATE', 'DELETE',
];

const READ_PREFIXES = [
    'SELECT', 'WITH', 'TABLE', 'VALUES',
    'EXPLAIN', 'SHOW',
];

// Prefixes that admit no write at all: TABLE and VALUES have no target to
// write to, and SHOW only reports a setting.
const ALWAYS_READ_PREFIXES = ['TABLE', 'VALUES', 'SHOW'];

// A keyword that makes a statement with a reading first keyword write after
// all. INTO catches SELECT ... INTO, which creates and populates a table; the
// rest catch a data-modifying CTE, where the write hides inside a statement
// whose first word is WITH.
const DML_INDICATOR = /\b(INSERT|UPDATE|DELETE|MERGE)\b/;
const DDL_INDICATOR = /\b(INTO|CREATE|DROP|ALTER|TRUNCATE|GRANT|REVOKE)\b/;

// The locking clauses of an ordinary SELECT. They take row locks but modify
// nothing, and their UPDATE keyword would otherwise trip DML_INDICATOR, so
// they are removed before it runs.
const ROW_LOCK_CLAUSE = /\bFOR\s+(NO\s+KEY\s+UPDATE|KEY\s+SHARE|UPDATE|SHARE)\b/g;

// ANALYZE is what makes EXPLAIN run the statement it is given rather than
// only plan it.
const ANALYZE_OPTION = /\bANALYZE\b/;

/**
 * Reports whether the statement begins with one of the given keywords,
 * requiring a word boundary after the match so that a table called "updates"
 * is not read as an UPDATE.
 */
function hasPrefix(upper, keywords) {
    return keywords.some(kw =>
        upper.startsWith(kw) && !/[A-Z0-9_]/.test(upper.charAt(kw.length)));
}

/**
 * Classifies whether a SQL query is a write (DDL/DML) operation.
 * Read queries (SELECT, WITH, etc.) return false.
 * Write queries (CREATE, DROP, INSERT, etc.) return true.
 * Unknown query types are treated as potentially destructive.
 *
 * The result drives the confirmation prompt shown before a statement runs on a
 * writable connection, so the cost of the two errors is not symmetric: calling
 * a read a write costs a needless prompt, whilst calling a write a read lets
 * the statement run unannounced. The checks therefore lean towards prompting.
 *
 * A reading first keyword is not enough to call the statement a read.
 * SELECT ... INTO creates and populates a table, a CTE can carry an INSERT,
 * UPDATE or DELETE, and EXPLAIN ANALYZE runs the statement it is given rather
 * than only planning it. Matching happens against the statement's code with
 * comments removed and literals blanked, so a keyword inside a string cannot
 * be mistaken for the real thing and a comment cannot hide one.
 *
 * This is a client-side prompt and not a security boundary. A statement whose
 * writes happen inside a function it calls still reads as a SELECT here, and
 * nothing textual could tell otherwise. What actually prevents a write on a
 * read-only connection is the transaction access mode set by the server.
 *
 * @param {string} sql - The SQL query to classify
 * @returns {boolean} - True if the query is a write operation
 */
export function isWriteQuery(sql) {
    if (!sql || typeof sql !== 'string') return false;
    const upper = stripSqlNoise(sql).trim().toUpperCase();

    if (hasPrefix(upper, WRITE_PREFIXES)) return true;
    if (!hasPrefix(upper, READ_PREFIXES)) {
        // Unknown query types are treated as potentially destructive
        return true;
    }

    if (hasPrefix(upper, ALWAYS_READ_PREFIXES)) return false;
    if (hasPrefix(upper, ['EXPLAIN']) && !ANALYZE_OPTION.test(upper)) {
        // EXPLAIN only plans its statement unless ANALYZE is given.
        return false;
    }

    const scanned = upper.replace(ROW_LOCK_CLAUSE, ' ');
    return DML_INDICATOR.test(scanned) || DDL_INDICATOR.test(scanned);
}

/**
 * Reports whether a tool advertises that it can modify the database.
 *
 * The server publishes MCP annotations on each tool, setting readOnlyHint
 * false on query_database when writes are enabled and on any custom tool that
 * could write. Only an explicit false counts here: a tool that publishes no
 * annotation is treated as read-only, which keeps the built-in read-only tools
 * from prompting. Any new write-capable tool must therefore advertise the hint
 * to be caught.
 *
 * @param {Array} tools - The tool list from tools/list
 * @param {string} name - The name of the tool being called
 * @returns {boolean} - True if the tool advertises that it may write
 */
export function toolMayWrite(tools, name) {
    if (!Array.isArray(tools)) return false;
    const tool = tools.find(t => t?.name === name);
    return tool?.annotations?.readOnlyHint === false;
}

/**
 * Renders a tool call for the confirmation dialog. Tools other than
 * query_database have no single statement to display, so the name and
 * arguments are shown instead.
 *
 * @param {string} name - The name of the tool being called
 * @param {object} input - The tool arguments
 * @returns {string} - Text describing the call
 */
export function describeToolCall(name, input) {
    if (!input || Object.keys(input).length === 0) {
        return `${name}()`;
    }
    try {
        return `${name} with arguments:\n${JSON.stringify(input, null, 2)}`;
    } catch {
        return `${name}(...)`;
    }
}

/**
 * Decides whether a tool call needs the user's confirmation before it runs,
 * and returns the text to show them. Call only when the current database
 * permits writes.
 *
 * query_database is judged by classifying the statement, so an ordinary read
 * is not interrupted. Every other tool is judged by the readOnlyHint the
 * server advertises, which covers operator-defined custom tools: those can
 * write too, and were previously never confirmed because this check was keyed
 * to a single tool name.
 *
 * @param {Array} tools - The tool list from tools/list
 * @param {string} name - The name of the tool being called
 * @param {object} input - The tool arguments
 * @returns {{needsConfirmation: boolean, subject: string}}
 */
export function writeConfirmationSubject(tools, name, input) {
    if (name === 'query_database') {
        const query = input?.query;
        if (typeof query !== 'string' || !isWriteQuery(query)) {
            return { needsConfirmation: false, subject: '' };
        }
        return { needsConfirmation: true, subject: query };
    }

    if (!toolMayWrite(tools, name)) {
        return { needsConfirmation: false, subject: '' };
    }
    return { needsConfirmation: true, subject: describeToolCall(name, input) };
}
