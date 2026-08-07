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
 * Textual scanning for SQL statements, mirroring internal/sqltext in the
 * server so that this client reasons about a statement the same way the CLI
 * client and the read-only guardrails do.
 *
 * The scanner is deliberately conservative rather than complete. It does not
 * parse SQL. Where a construct is ambiguous or unterminated it stops treating
 * the text as a literal and lets the remainder fall through as code, so the
 * failure mode is a needless confirmation prompt rather than a missed write.
 *
 * Backslash escapes are honoured inside an E'...' escape string constant and
 * nowhere else. In a plain '...' literal a backslash is an ordinary character
 * under standard_conforming_strings, which is the default, so treating it as
 * an escape there would run the literal past its real end and hide the code
 * that follows.
 */

const isAsciiLetter = c => (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z');
const isAsciiDigit = c => c >= '0' && c <= '9';
const isIdentifierByte = c => c === '_' || isAsciiLetter(c) || isAsciiDigit(c);

/**
 * Returns the index just past a "--" line comment starting at i.
 */
function skipLineComment(sql, i) {
    const end = sql.indexOf('\n', i);
    return end < 0 ? sql.length : end;
}

/**
 * Returns the index just past a block comment, honouring the nesting that
 * PostgreSQL allows. An unterminated comment consumes the rest of the
 * statement, which is what the server would do with it too.
 */
function skipBlockComment(sql, i) {
    let depth = 0;
    while (i < sql.length) {
        if (sql[i] === '/' && sql[i + 1] === '*') {
            depth++;
            i += 2;
            continue;
        }
        if (sql[i] === '*' && sql[i + 1] === '/') {
            depth--;
            i += 2;
            if (depth === 0) return i;
            continue;
        }
        i++;
    }
    return sql.length;
}

/**
 * Returns the index just past a literal or quoted identifier starting at i,
 * treating a doubled quote character as an escaped one. An unterminated quote
 * consumes the remainder of the statement.
 *
 * When escapes is set the literal is an escape string constant, E'...', in
 * which a backslash escapes the character after it. Honouring that matters: in
 * E'\'' the backslash escapes the first quote and the second closes the
 * literal, whereas reading the pair as a doubled quote runs the literal on to
 * the end of the statement and hides whatever follows it.
 */
function endOfQuoted(sql, i, quote, escapes) {
    i++;
    while (i < sql.length) {
        if (escapes && sql[i] === '\\' && i + 1 < sql.length) {
            i += 2;
            continue;
        }
        if (sql[i] !== quote) {
            i++;
            continue;
        }
        if (sql[i + 1] === quote) {
            i += 2;
            continue;
        }
        return i + 1;
    }
    return sql.length;
}

/**
 * Reports whether an E'...' escape string constant starts at i, that is
 * whether sql[i] is an E introducing a literal rather than an ordinary
 * identifier character. Backslash escapes are honoured only for these
 * literals: with standard_conforming_strings on, which is the default, a
 * backslash in a plain '...' literal is an ordinary character, and treating it
 * as an escape there would run that literal past its real end.
 */
function isEscapeStringStart(sql, i) {
    if (sql[i] !== 'E' && sql[i] !== 'e') return false;
    if (sql[i + 1] !== '\'') return false;
    // An E that continues an identifier, as in "table_e'x'", is not a prefix.
    return i === 0 || !/[A-Za-z0-9_]/.test(sql[i - 1]);
}

/**
 * Reports whether a dollar-quote tag may begin at i. PostgreSQL treats $ as a
 * legal, non-initial identifier character, so "x$tag$" lexes as the single
 * identifier x$tag$, not as x followed by a dollar-quote delimiter. Without
 * this check that byte is rejected as a tag on its own and the scanner falls
 * back to consuming it as ordinary text one byte at a time, so the very next
 * $ opens a dollar-quoted block whose attacker-chosen closing tag can swallow
 * arbitrary SQL that follows, including a smuggled statement.
 */
function canStartDollarQuote(sql, i) {
    return i === 0 || (!isIdentifierByte(sql[i - 1]) && sql[i - 1] !== '$');
}

/**
 * Returns the opening tag of a dollar-quoted string starting at i, for example
 * "$$" or "$body$", or null for anything else. Positional parameters such as
 * $1 are not tags, since a digit cannot start one.
 */
function dollarQuoteTag(sql, i) {
    let j = i + 1;
    while (j < sql.length) {
        const c = sql[j];
        if (c === '$') return sql.slice(i, j + 1);
        if (c === '_' || isAsciiLetter(c) || (isAsciiDigit(c) && j > i + 1)) {
            j++;
            continue;
        }
        return null;
    }
    return null;
}

/**
 * Removes comments and blanks every string literal, dollar-quoted block and
 * quoted identifier, leaving only what PostgreSQL would treat as code. A
 * dropped comment becomes a single space, so that it cannot join the tokens on
 * either side of it.
 *
 * @param {string} sql - The SQL statement to scan
 * @returns {string} - The statement's code, with literals blanked
 */
export function stripSqlNoise(sql) {
    let out = '';
    let i = 0;

    while (i < sql.length) {
        const c = sql[i];

        if (c === '-' && sql[i + 1] === '-') {
            i = skipLineComment(sql, i);
            out += ' ';
        } else if (c === '/' && sql[i + 1] === '*') {
            i = skipBlockComment(sql, i);
            out += ' ';
        } else if (isEscapeStringStart(sql, i)) {
            // Keep the E, then consume the literal with backslash escapes
            // honoured.
            out += c;
            i = endOfQuoted(sql, i + 1, '\'', true);
            out += "''";
        } else if (c === '\'' || c === '"') {
            i = endOfQuoted(sql, i, c, false);
            out += c + c;
        } else if (c === '$') {
            const tag = canStartDollarQuote(sql, i) ? dollarQuoteTag(sql, i) : null;
            const body = tag === null ? -1 : sql.indexOf(tag, i + tag.length);
            if (tag !== null && body >= 0) {
                i = body + tag.length;
                out += tag + tag;
            } else {
                // Not a dollar quote, never closed, or continuing an
                // identifier: treat as ordinary text so that anything
                // following is still scanned as code.
                out += c;
                i++;
            }
        } else {
            out += c;
            i++;
        }
    }

    return out;
}
