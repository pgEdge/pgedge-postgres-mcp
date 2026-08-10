/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

/**
 * Guards the agentic loop against a model that keeps re-issuing the same
 * failing tool call. Some providers respond to a tool error by repeating
 * the identical call rather than adapting, which burns a request per
 * attempt until the loop hits its iteration cap; the guard spots that
 * pattern early and lets the caller stop with a useful explanation.
 */

// The longest error text quoted back to the user. Tool errors can carry a
// whole server response, and the chat bubble only needs enough of it to
// identify the problem.
const MAX_QUOTED_ERROR_LENGTH = 500;

/**
 * Serialises a value to a string that does not depend on object key
 * order, so that {a: 1, b: 2} and {b: 2, a: 1} produce the same text.
 * Arrays keep their order, because argument order is significant.
 * Cycles are replaced with a marker rather than throwing, since the
 * guard must never be the thing that breaks a conversation.
 * @param {*} value - The value to serialise.
 * @param {WeakSet} [seen] - Containers already being serialised.
 * @returns {string} - A stable textual representation.
 */
export const stableStringify = (value, seen = new WeakSet()) => {
    if (value === null || value === undefined) return 'null';
    if (typeof value === 'number') {
        return Number.isFinite(value) ? String(value) : 'null';
    }
    if (typeof value === 'boolean') return String(value);
    if (typeof value === 'string') return JSON.stringify(value);
    if (typeof value !== 'object') return JSON.stringify(String(value));

    if (seen.has(value)) return '"[circular]"';
    seen.add(value);

    let out;
    if (Array.isArray(value)) {
        out = `[${value.map((item) => stableStringify(item, seen)).join(',')}]`;
    } else {
        const keys = Object.keys(value).sort();
        const parts = keys.map(
            (key) => `${JSON.stringify(key)}:${stableStringify(value[key], seen)}`,
        );
        out = `{${parts.join(',')}}`;
    }

    seen.delete(value);
    return out;
};

/**
 * Builds the key that identifies one specific tool call: the tool name
 * together with its arguments, so a retry with genuinely different
 * arguments counts as a different call.
 * @param {string} name - The tool name.
 * @param {*} input - The arguments passed to the tool.
 * @returns {string} - The identity key for the call.
 */
export const toolCallKey = (name, input) =>
    `${String(name)}::${stableStringify(input)}`;

/**
 * Shortens error text for display, appending an ellipsis when trimmed.
 * @param {string} text - The raw error text.
 * @returns {string} - Text no longer than MAX_QUOTED_ERROR_LENGTH.
 */
const truncateError = (text) => {
    const trimmed = (text || '').trim();
    if (trimmed.length <= MAX_QUOTED_ERROR_LENGTH) return trimmed;
    return `${trimmed.slice(0, MAX_QUOTED_ERROR_LENGTH)}...`;
};

/**
 * Composes the message shown to the user when the guard trips. It names
 * the tool, says how many times it failed, and quotes the last error so
 * that the user can see what needs fixing.
 * @param {object} info - The trip details from the guard.
 * @param {string} info.name - The tool that kept failing.
 * @param {number} info.count - How many times it failed in a row.
 * @param {string} info.errorText - The most recent error text.
 * @returns {string} - The assistant message content.
 */
export const buildRepeatedToolFailureMessage = ({ name, count, errorText }) => {
    const quoted = truncateError(errorText);
    const detail = quoted
        ? ` The last error was: ${quoted}`
        : '';
    return `I have stopped because the \`${name}\` tool failed ${count} times ` +
        `in a row with exactly the same arguments, so repeating it again is ` +
        `unlikely to help.${detail}\n\n` +
        `You may need to address the underlying problem, or rephrase your ` +
        `request so that a different approach is taken.`;
};

/**
 * Creates a guard that tracks consecutive failures per distinct tool
 * call. Record every executed call; the guard trips once one call has
 * failed `limit` times without an intervening success.
 * @param {number} limit - Consecutive failures that trip the guard.
 * @returns {object} - Guard with `record` and `getTripped` methods.
 */
export const createToolRepeatGuard = (limit) => {
    const failures = new Map();
    let tripped = null;

    return {
        /**
         * Records the outcome of one tool call.
         * @param {object} call - The call outcome.
         * @param {string} call.name - The tool name.
         * @param {*} call.input - The arguments the tool was called with.
         * @param {boolean} call.isError - Whether the call failed.
         * @param {string} [call.resultText] - The result or error text.
         * @returns {object|null} - Trip details, or null if not tripped.
         */
        record({ name, input, isError, resultText }) {
            const key = toolCallKey(name, input);

            // A success means the loop is making progress, so forget the
            // earlier failures of that exact call.
            if (!isError) {
                failures.delete(key);
                return tripped;
            }

            const count = (failures.get(key) || 0) + 1;
            failures.set(key, count);

            if (!tripped && count >= limit) {
                tripped = { name, count, errorText: resultText || '' };
            }
            return tripped;
        },

        /**
         * Reports whether the guard has tripped.
         * @returns {object|null} - Trip details, or null if not tripped.
         */
        getTripped() {
            return tripped;
        },
    };
};
