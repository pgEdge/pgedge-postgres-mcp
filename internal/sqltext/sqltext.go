/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package sqltext provides the textual scanning shared by the checks that
// need to reason about a SQL statement without parsing it.
//
// Two callers rely on it, for different purposes. The read-only guardrails in
// internal/tools use it to decide whether a statement may run at all, and the
// write classifier in internal/chat uses it to decide whether the user should
// be asked to confirm a statement before it runs. Both need the same thing:
// the statement's code with its comments and literals taken out of the way, so
// that a keyword inside a string cannot be mistaken for the real thing, and a
// comment cannot be used to hide one.
//
// The scanner is deliberately conservative rather than complete. It does not
// parse SQL, and it is not a substitute for the transaction access mode that
// actually enforces read-only behaviour at the server.
package sqltext

import "strings"

// Strip scans a statement once and returns two normalised forms.
//
// residue has comments removed and every string literal, dollar-quoted block
// and quoted identifier replaced by an empty placeholder, leaving only what
// PostgreSQL would treat as code. bare has comments removed but literals left
// intact, for the checks that must see inside a literal.
//
// The scanner is deliberately conservative rather than complete. Where a
// construct is ambiguous or unterminated it stops treating the text as a
// literal and lets the remainder fall through to the residue as code, so the
// failure mode is a rejected valid query rather than an admitted hostile one.
// Backslash escapes are honoured inside an E'...' escape string constant and
// nowhere else. In a plain '...' literal a backslash is an ordinary character
// under standard_conforming_strings, which is the default, so treating it as
// an escape there would run the literal past its real end and hide the code
// that follows.
func Strip(query string) (residue string, bare string) {
	var res, bar strings.Builder
	res.Grow(len(query))
	bar.Grow(len(query))

	i := 0
	for i < len(query) {
		switch {
		case isLineCommentStart(query, i):
			i = skipLineCommentBody(query, i)
			writeNoiseSeparator(&res, &bar)

		case isBlockCommentStart(query, i):
			i = skipBlockComment(query, i)
			writeNoiseSeparator(&res, &bar)

		case escapeStringStart(query, i):
			// Write the E through to both forms, then consume the literal
			// itself with backslash escapes honoured.
			res.WriteByte(query[i])
			bar.WriteByte(query[i])
			i = consumeQuoted(query, i+1, '\'', true, &res, &bar)

		case query[i] == '\'':
			i = consumeQuoted(query, i, '\'', false, &res, &bar)

		case query[i] == '"':
			i = consumeQuoted(query, i, '"', false, &res, &bar)

		case query[i] == '$':
			if end, ok := consumeDollarQuote(query, i, &res, &bar); ok {
				i = end
				continue
			}
			// Not a dollar quote, or never closed: treat as ordinary text so
			// that anything following is still scanned as code.
			res.WriteByte(query[i])
			bar.WriteByte(query[i])
			i++

		default:
			res.WriteByte(query[i])
			bar.WriteByte(query[i])
			i++
		}
	}

	return res.String(), bar.String()
}

// HasMultipleStatements reports whether the residue holds more than one
// statement. Trailing separators are ignored so that a single statement
// written with a terminating semicolon is accepted.
func HasMultipleStatements(residue string) bool {
	return strings.Contains(strings.TrimRight(residue, "; \t\r\n"), ";")
}

// writeNoiseSeparator writes a single space to both builders, in place of a
// dropped comment, so that the comment cannot join the tokens on either
// side of it.
func writeNoiseSeparator(res, bar *strings.Builder) {
	res.WriteByte(' ')
	bar.WriteByte(' ')
}

// skipLineCommentBody returns the index just past a "--" line comment
// starting at i: the position of the next newline, or the end of the
// string if the comment runs to the end of it.
func skipLineCommentBody(query string, i int) int {
	if end := strings.IndexByte(query[i:], '\n'); end >= 0 {
		return i + end
	}
	return len(query)
}

// consumeQuoted returns the index just past a quoted literal or identifier
// starting at i, having written the original text to bar and an empty
// same-quote placeholder to res.
func consumeQuoted(query string, i int, quote byte, escapes bool, res, bar *strings.Builder) int {
	end := endOfQuoted(query, i, quote, escapes)
	bar.WriteString(query[i:end])
	res.WriteByte(quote)
	res.WriteByte(quote)
	return end
}

// consumeDollarQuote reports whether a dollar-quoted block starts at i, and
// if so returns the index just past it, having written the original text to
// bar and the tag doubled to res. Reports false if there is no valid, closed
// dollar-quote at i, in which case the caller falls back to treating
// query[i] as ordinary text.
func consumeDollarQuote(query string, i int, res, bar *strings.Builder) (int, bool) {
	tag, ok := dollarQuoteTag(query, i)
	if !ok {
		return 0, false
	}
	end := endOfDollarQuote(query, i, tag)
	if end <= 0 {
		return 0, false
	}
	bar.WriteString(query[i:end])
	res.WriteString(tag)
	res.WriteString(tag)
	return end, true
}

func isLineCommentStart(query string, i int) bool {
	return query[i] == '-' && i+1 < len(query) && query[i+1] == '-'
}

func isBlockCommentStart(query string, i int) bool {
	return query[i] == '/' && i+1 < len(query) && query[i+1] == '*'
}

// skipBlockComment returns the index just past a block comment, honouring the
// nesting that PostgreSQL allows. An unterminated comment consumes the rest of
// the statement, which is what the server would do with it too.
func skipBlockComment(query string, i int) int {
	depth := 0
	for i < len(query) {
		if isBlockCommentStart(query, i) {
			depth++
			i += 2
			continue
		}
		if query[i] == '*' && i+1 < len(query) && query[i+1] == '/' {
			depth--
			i += 2
			if depth == 0 {
				return i
			}
			continue
		}
		i++
	}
	return len(query)
}

// endOfQuoted returns the index just past a literal or quoted identifier that
// starts at i, treating a doubled quote character as an escaped one. An
// unterminated quote consumes the remainder of the statement.
//
// When escapes is set the literal is an escape string constant, E'...', in
// which a backslash escapes the character after it. Honouring that matters:
// in E'\” the backslash escapes the first quote and the second closes the
// literal, whereas reading the pair as a doubled quote runs the literal on to
// the end of the statement and hides whatever follows it.
func endOfQuoted(query string, i int, quote byte, escapes bool) int {
	i++
	for i < len(query) {
		if escapes && query[i] == '\\' && i+1 < len(query) {
			i += 2
			continue
		}
		if query[i] != quote {
			i++
			continue
		}
		if i+1 < len(query) && query[i+1] == quote {
			i += 2
			continue
		}
		return i + 1
	}
	return len(query)
}

// escapeStringStart reports whether an E'...' escape string constant starts at
// i, that is whether query[i] is an E introducing a literal rather than an
// ordinary identifier character. Backslash escapes are honoured only for these
// literals: with standard_conforming_strings on, which is the default, a
// backslash in a plain '...' literal is an ordinary character, and treating it
// as an escape there would run that literal past its real end.
func escapeStringStart(query string, i int) bool {
	if query[i] != 'E' && query[i] != 'e' {
		return false
	}
	if i+1 >= len(query) || query[i+1] != '\'' {
		return false
	}
	// An E that continues an identifier, as in "table_e'x'", is not a prefix.
	return i == 0 || !isIdentifierByte(query[i-1])
}

func isIdentifierByte(c byte) bool {
	return c == '_' || isASCIILetter(c) || isASCIIDigit(c)
}

// dollarQuoteTag returns the opening tag of a dollar-quoted string starting at
// i, for example "$$" or "$body$". It reports false for anything else,
// including positional parameter references such as $1, whose digit start is
// not a valid tag.
func dollarQuoteTag(query string, i int) (string, bool) {
	j := i + 1
	for j < len(query) {
		c := query[j]
		switch {
		case c == '$':
			return query[i : j+1], true
		case c == '_' || isASCIILetter(c):
			j++
		case isASCIIDigit(c) && j > i+1:
			// Digits are allowed in a tag, just not as its first character.
			j++
		default:
			return "", false
		}
	}
	return "", false
}

// endOfDollarQuote returns the index just past the closing tag of a
// dollar-quoted string, or 0 if the string is never closed.
func endOfDollarQuote(query string, i int, tag string) int {
	body := query[i+len(tag):]
	end := strings.Index(body, tag)
	if end < 0 {
		return 0
	}
	return i + len(tag) + end + len(tag)
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
