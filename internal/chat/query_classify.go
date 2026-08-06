/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
	"regexp"
	"strings"

	"pgedge-postgres-mcp/internal/sqltext"
)

// QueryType represents the type of SQL query
type QueryType int

const (
	QueryTypeSelect QueryType = iota
	QueryTypeDDL
	QueryTypeDML
	QueryTypeOther
)

// readPrefixes are the leading keywords of a statement that reads. Reaching
// one is not on its own enough to call a statement a read: see writeIndicator.
var readPrefixes = []string{
	"SELECT", "WITH", "TABLE", "VALUES", "EXPLAIN", "SHOW",
}

// ddlPrefixes and dmlPrefixes are the leading keywords that make a statement a
// write outright, whatever follows them.
var (
	ddlPrefixes = []string{"CREATE", "DROP", "ALTER", "TRUNCATE"}
	dmlPrefixes = []string{"INSERT", "UPDATE", "DELETE"}
)

// dmlIndicator and ddlIndicator match a keyword that makes a read-prefixed
// statement write after all.
//
// INTO catches SELECT ... INTO, which creates and populates a table. In a
// statement that begins SELECT or WITH, INTO can only be SELECT ... INTO or
// the INTO of a data-modifying INSERT, and both are writes. The remaining
// keywords catch a data-modifying CTE, where the write hides inside a
// statement whose first word is WITH.
// The two halves are matched separately so that a statement caught here can
// still be reported as DML or DDL; dmlIndicator is tried first, so that the
// INTO of an INSERT is not mistaken for the INTO of a SELECT ... INTO.
var (
	dmlIndicator = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|MERGE)\b`)
	ddlIndicator = regexp.MustCompile(
		`(?i)\b(INTO|CREATE|DROP|ALTER|TRUNCATE|GRANT|REVOKE)\b`)
)

// rowLockClause matches the locking clauses of an ordinary SELECT. They take
// row locks but modify nothing, and their UPDATE keyword would otherwise trip
// writeIndicator, so they are removed before it runs.
var rowLockClause = regexp.MustCompile(
	`(?i)\bFOR\s+(NO\s+KEY\s+UPDATE|KEY\s+SHARE|UPDATE|SHARE)\b`)

// analyzeOption matches the ANALYZE option of EXPLAIN, which is what makes
// EXPLAIN run the statement it is given rather than only plan it.
var analyzeOption = regexp.MustCompile(`(?i)\bANALYZE\b`)

// ClassifyQuery determines if a SQL query is a read or write operation.
// Returns the query type and whether the query modifies data.
//
// The result drives the confirmation prompt shown before a statement runs on a
// writable connection, so the cost of the two errors is not symmetric: calling
// a read a write costs a needless prompt, whilst calling a write a read lets
// the statement run unannounced. Anything unrecognised is therefore treated as
// a write, and the checks below lean the same way.
//
// Matching is done against the statement's code with comments removed and
// literals blanked, so that a keyword inside a string cannot be mistaken for
// the real thing and a comment cannot be used to hide one.
//
// This is a client-side prompt and not a security boundary. A statement whose
// writes happen inside a function it calls still reads as a SELECT here, and
// nothing textual could tell otherwise. What actually prevents a write on a
// read-only connection is the transaction access mode set by the server.
func ClassifyQuery(sql string) (QueryType, bool) {
	residue, _ := sqltext.Strip(sql)
	upper := strings.ToUpper(strings.TrimSpace(residue))

	switch {
	case hasPrefix(upper, ddlPrefixes):
		return QueryTypeDDL, true

	case hasPrefix(upper, dmlPrefixes):
		return QueryTypeDML, true

	case hasPrefix(upper, readPrefixes):
		return classifyReadPrefixed(upper)

	default:
		return QueryTypeOther, true
	}
}

// classifyReadPrefixed decides whether a statement whose first keyword reads
// nevertheless writes.
func classifyReadPrefixed(upper string) (QueryType, bool) {
	// TABLE, VALUES and SHOW admit no write: TABLE and VALUES have no target
	// to write to, and SHOW only reports a setting.
	if hasPrefix(upper, []string{"TABLE", "VALUES", "SHOW"}) {
		return QueryTypeSelect, false
	}

	// EXPLAIN only plans its statement unless ANALYZE is given, in which case
	// it runs it. EXPLAIN INSERT is a read; EXPLAIN ANALYZE INSERT is not.
	if strings.HasPrefix(upper, "EXPLAIN") && !analyzeOption.MatchString(upper) {
		return QueryTypeSelect, false
	}

	scanned := rowLockClause.ReplaceAllString(upper, " ")
	switch {
	case dmlIndicator.MatchString(scanned):
		return QueryTypeDML, true
	case ddlIndicator.MatchString(scanned):
		return QueryTypeDDL, true
	default:
		return QueryTypeSelect, false
	}
}

// hasPrefix reports whether the statement begins with any of the given
// keywords, requiring a word boundary after the match so that a table called
// "updates" is not read as an UPDATE.
func hasPrefix(upper string, keywords []string) bool {
	for _, kw := range keywords {
		if !strings.HasPrefix(upper, kw) {
			continue
		}
		rest := upper[len(kw):]
		if rest == "" || !isIdentifierByte(rest[0]) {
			return true
		}
	}
	return false
}

func isIdentifierByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' ||
		c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}
