/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"fmt"
	"regexp"

	"pgedge-postgres-mcp/internal/sqltext"
)

// This file implements the textual half of the read-only guardrails. It is
// deliberately the weakest of the layers protecting a read-only connection,
// and it must never be the only one.
//
// The layers, weakest to strongest:
//
//  1. This guard, which inspects statement text and rejects constructs known
//     to escape read-only mode. Being text based it can only ever reject what
//     it recognises.
//  2. The transaction access mode, set atomically with BEGIN by every tool
//     that runs caller-supplied SQL.
//  3. The session default (default_transaction_read_only), applied when a
//     pooled connection is created and re-asserted when it is released.
//  4. Database privileges: a role with INSERT, UPDATE, DELETE and DDL
//     revoked, which no amount of transaction-mode trickery can defeat.
//
// Layers 1 to 3 all live inside this process and all share one weakness: they
// are settings that the connected role is entitled to change. Only layer 4 is
// a genuine constraint, and it is the documented recommendation in
// docs/guide/security_mgmt.md. Treat additions to this guard as a way of
// closing known holes and generating audit evidence, not as a substitute.
//
// A structural protection worth more than every pattern below is enforced in
// query_database.go: on a read-only connection, caller SQL is executed
// exclusively through the extended query protocol, which permits precisely
// one statement per message. That removes the statement-smuggling channel
// that made most of these patterns exploitable in the first place.

// readOnlyRejection describes a construct that is refused on a read-only
// connection, together with the reason reported to the caller.
type readOnlyRejection struct {
	pattern *regexp.Regexp
	reason  string
}

// readOnlyRejections lists constructs refused on a read-only connection.
//
// Every pattern is matched against the "residue" of the statement: comments
// removed, and string literals, dollar-quoted blocks and quoted identifiers
// replaced by empty placeholders. Matching the residue means a keyword buried
// in a string literal cannot trigger a rejection, and, more importantly, that
// a comment cannot be used to split a keyword and slip past.
var readOnlyRejections = []readOnlyRejection{
	// Transaction access mode. SET TRANSACTION READ WRITE succeeds whenever
	// the transaction has not yet taken a snapshot, and BEGIN READ WRITE or
	// START TRANSACTION READ WRITE overrides the session default outright.
	{
		pattern: regexp.MustCompile(`(?i)\bREAD\s+WRITE\b`),
		reason:  "read-write transaction mode cannot be requested on a read-only connection",
	},
	// SET SESSION CHARACTERISTICS AS TRANSACTION READ WRITE changes the
	// session default without naming default_transaction_read_only at all.
	{
		pattern: regexp.MustCompile(`(?i)\bSESSION\s+CHARACTERISTICS\b`),
		reason:  "session transaction characteristics cannot be changed on a read-only connection",
	},
	// RESET ALL and DISCARD ALL both clear the session default that is
	// applied when the pooled connection is established.
	{
		pattern: regexp.MustCompile(`(?i)\bRESET\s+ALL\b`),
		reason:  "RESET ALL is not permitted on a read-only connection",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bDISCARD\b`),
		reason:  "DISCARD is not permitted on a read-only connection",
	},
	// Transaction control ends the read-only transaction the server opened,
	// leaving anything that follows to run under different rules. Anchored at
	// the start of the statement because multi-statement input is rejected
	// separately, so these can only appear as the whole statement; that also
	// avoids false positives on unreserved keywords used as identifiers.
	{
		pattern: regexp.MustCompile(`(?i)^\s*(BEGIN|START|COMMIT|END|ROLLBACK|ABORT|SAVEPOINT|RELEASE|PREPARE\s+TRANSACTION)\b`),
		reason:  "transaction control statements are not permitted on a read-only connection",
	},
	// Changing the effective role can pick up privileges the configured role
	// does not have, including a role for which writes were never revoked.
	{
		pattern: regexp.MustCompile(`(?i)\bSESSION\s+AUTHORIZATION\b`),
		reason:  "SET SESSION AUTHORIZATION is not permitted on a read-only connection",
	},
	{
		pattern: regexp.MustCompile(`(?i)^\s*SET\s+(SESSION\s+|LOCAL\s+)?ROLE\b`),
		reason:  "SET ROLE is not permitted on a read-only connection",
	},
	// ALTER ROLE ... SET and ALTER DATABASE ... SET persist a changed default
	// in the catalogue, surviving restarts. These are catalogue writes and so
	// are already refused by a read-only transaction; they are listed here so
	// that an attempt is reported and logged as a bypass attempt rather than
	// as an incidental permissions error.
	{
		pattern: regexp.MustCompile(`(?i)^\s*ALTER\s+(ROLE|USER|DATABASE)\b`),
		reason:  "ALTER ROLE, ALTER USER and ALTER DATABASE are not permitted on a read-only connection",
	},
	// The remaining entries cover writes that happen outside the transaction's
	// scope, which a read-only transaction does not prevent at all.
	//
	// A DO block executes arbitrary procedural code. Its SQL writes are
	// blocked by the read-only transaction, but an untrusted language such as
	// plperlu or plpython3u reaches the filesystem and the operating system,
	// where the transaction access mode means nothing.
	{
		pattern: regexp.MustCompile(`(?i)^\s*DO\b`),
		reason:  "DO blocks are not permitted on a read-only connection; use a configured custom tool instead",
	},
	// COPY ... TO PROGRAM only reads from the database, so a read-only
	// transaction permits it, whilst it executes a shell command server-side.
	{
		pattern: regexp.MustCompile(`(?i)\bCOPY\b[\s\S]*\bPROGRAM\b`),
		reason:  "COPY ... TO PROGRAM is not permitted on a read-only connection",
	},
	// Server-side file writes are likewise outside the transaction's scope.
	{
		pattern: regexp.MustCompile(`(?i)\b(LO_EXPORT|PG_FILE_WRITE|PG_FILE_UNLINK|PG_FILE_RENAME)\s*\(`),
		reason:  "server-side file modification is not permitted on a read-only connection",
	},
	// dblink and postgres_fdw open a fresh session that carries none of this
	// transaction's read-only state, so a write issued through them succeeds
	// unless privileges forbid it.
	{
		pattern: regexp.MustCompile(`(?i)\bDBLINK\w*\s*\(`),
		reason:  "dblink is not permitted on a read-only connection",
	},
	// set_config() changes a session or transaction setting the same way SET
	// does, but as an ordinary function call rather than a keyword, so its
	// arguments can be built at runtime instead of written as literal text:
	// SELECT set_config(chr(100)||chr(101)||..., 'off', false) sets
	// default_transaction_read_only just as effectively as a literal SET, and
	// the setting name never appears anywhere in the statement text for
	// readOnlyGUCPattern to match. No fixed pattern can distinguish that from
	// a harmless call, so every call is refused outright, the same blanket
	// treatment DO blocks get, rather than trying to recognise which calls
	// are dangerous.
	{
		pattern: regexp.MustCompile(`(?i)\bSET_CONFIG\s*\(`),
		reason:  "set_config() is not permitted on a read-only connection",
	},
}

// readOnlyGUCPattern matches the settings that control read-only behaviour.
// This is checked against the comment-stripped statement with literals left
// intact, because the usual way of reaching these settings is through a
// string literal: set_config('default_transaction_read_only', 'off', true).
var readOnlyGUCPattern = regexp.MustCompile(`(?i)\b(DEFAULT_)?TRANSACTION_READ_ONLY\b`)

// settingChangePattern matches the constructs capable of changing a setting.
// Requiring one of these alongside a read-only setting name means an ordinary
// query that merely mentions the name, such as a lookup in a configuration
// table, is not rejected.
var settingChangePattern = regexp.MustCompile(`(?i)(\bSET\b|\bRESET\b|\bSET_CONFIG\s*\()`)

// validateReadOnlyQuery checks whether a statement attempts to escape
// read-only mode, and returns an error describing the first problem found.
// It is called only when the connection does not allow writes.
//
// The check errs towards rejection: where the text cannot be scanned
// unambiguously the scanner leaves the doubtful section in place to be matched
// as code, so malformed input is more likely to be refused than admitted.
func validateReadOnlyQuery(query string) error {
	residue, bare := stripSQLNoise(query)

	if hasMultipleStatements(residue) {
		return fmt.Errorf(
			"query rejected: multiple SQL statements in a single request " +
				"are not permitted on a read-only connection")
	}

	for _, rejection := range readOnlyRejections {
		if rejection.pattern.MatchString(residue) {
			return fmt.Errorf("query rejected: %s", rejection.reason)
		}
	}

	// The setting name is looked for with literals intact, so that a name
	// passed to set_config() is still found, including from inside a DO block.
	if readOnlyGUCPattern.MatchString(bare) && settingChangePattern.MatchString(bare) {
		return fmt.Errorf(
			"query rejected: queries cannot change " +
				"'transaction_read_only' or 'default_transaction_read_only' " +
				"when the database connection is in read-only mode")
	}

	return nil
}

// hasMultipleStatements reports whether the residue holds more than one
// statement. Trailing separators are ignored so that a single statement
// written with a terminating semicolon is accepted.
func hasMultipleStatements(residue string) bool {
	return sqltext.HasMultipleStatements(residue)
}

// stripSQLNoise scans a statement into the two normalised forms the checks in
// this package match against. The scanner itself lives in internal/sqltext,
// which internal/chat also uses so that the write-confirmation prompt reasons
// about a statement the same way this guard does.
func stripSQLNoise(query string) (residue string, bare string) {
	return sqltext.Strip(query)
}
