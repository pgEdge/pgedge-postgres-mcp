/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

const WRITE_PREFIXES = [
    'CREATE', 'DROP', 'ALTER', 'TRUNCATE',
    'INSERT', 'UPDATE', 'DELETE',
];

const READ_PREFIXES = [
    'SELECT', 'WITH', 'TABLE', 'VALUES',
    'EXPLAIN', 'SHOW',
];

/**
 * Classifies whether a SQL query is a write (DDL/DML) operation.
 * Read queries (SELECT, WITH, etc.) return false.
 * Write queries (CREATE, DROP, INSERT, etc.) return true.
 * Unknown query types are treated as potentially destructive.
 *
 * @param {string} sql - The SQL query to classify
 * @returns {boolean} - True if the query is a write operation
 */
export function isWriteQuery(sql) {
    if (!sql || typeof sql !== 'string') return false;
    const upper = sql.trim().toUpperCase();
    if (READ_PREFIXES.some(p => upper.startsWith(p))) return false;
    if (WRITE_PREFIXES.some(p => upper.startsWith(p))) return true;
    // Unknown query types are treated as potentially destructive
    return true;
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
