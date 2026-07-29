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

import "testing"

// TestValidateCountRowsWhereClause_AllowsSimplePredicates confirms that
// every filter shown in the tool's own documented examples, plus other
// ordinary predicate shapes, still passes.
func TestValidateCountRowsWhereClause_AllowsSimplePredicates(t *testing.T) {
	cases := []string{
		"",
		"status = 'pending'",
		"created_at > '2024-01-01'",
		"status = 'active' AND created_at > '2024-01-01'",
		"status IN ('a', 'b', 'c')",
		"name LIKE 'A%'",
		"price BETWEEN 10 AND 100",
		"deleted_at IS NULL",
		"NOT (status = 'archived')",
		// A string literal that merely contains the word "select" must not
		// trip the check; stripSQLNoise blanks literals before matching.
		"description = 'please select an option'",
		"notes = 'a select-box widget'",
		// VALUES is deliberately not blocked: it cannot reference a table
		// (no FROM clause is legal in a VALUES construct, so it can only
		// build a literal row, never read one) and, unlike SELECT and
		// TABLE, it is not fully reserved in PostgreSQL — it can
		// legitimately be an unquoted column name.
		"values = 5",
		"values > 0 AND values < 100",
		// A quoted identifier literally named "table" (the only way to use
		// that word as an identifier, since TABLE is fully reserved) must
		// not trip the check either: stripSQLNoise blanks quoted
		// identifiers the same way it blanks string literals.
		`"table" = 'active'`,
	}

	for _, where := range cases {
		if err := validateCountRowsWhereClause(where); err != nil {
			t.Errorf("where=%q: expected no error, got: %v", where, err)
		}
	}
}

// TestValidateCountRowsWhereClause_RejectsSubqueries confirms that every
// subquery shape used in the issue #200 proof-of-concept, plus a few
// evasion attempts, is rejected.
func TestValidateCountRowsWhereClause_RejectsSubqueries(t *testing.T) {
	cases := []string{
		// The exact boolean-blind cross-table oracle from issue #200.
		"1=1 AND (SELECT COUNT(*) FROM customers WHERE email = 'alice@example.com') = 1",
		// The exact error-based blind oracle from issue #200.
		"1=1 AND CAST((CASE WHEN (SELECT substr(email,1,1) FROM customers WHERE id=1) = 'a' THEN '1' ELSE 'not-a-number' END) AS int) = 1",
		// EXISTS is itself a subquery.
		"EXISTS (SELECT 1 FROM secret_table)",
		// Case-insensitivity.
		"1=1 and (select 1 from secret_table) is not null",
		"1=1 AND (SeLeCt 1 FROM secret_table) IS NOT NULL",
		// A line comment preceding the keyword must not hide it.
		"1=1 AND (-- comment\nSELECT 1 FROM secret_table) IS NOT NULL",
		// TABLE tablename is PostgreSQL shorthand for SELECT * FROM
		// tablename and reads exactly the same data — confirmed directly
		// against a live server: "(TABLE secret_table) = value" leaks a
		// single-column table's contents through the same boolean oracle
		// as a SELECT-based subquery, with the word SELECT never
		// appearing anywhere in the clause. This was a real gap in an
		// earlier version of this check.
		"(TABLE secret_table) = true",
		"1=1 AND (TABLE secret_table) IS NOT NULL",
		"1 = (table secret_table limit 1)",
	}

	for _, where := range cases {
		if err := validateCountRowsWhereClause(where); err == nil {
			t.Errorf("where=%q: expected a rejection, got none", where)
		}
	}
}

// TestValidateCountRowsWhereClause_SplitCommentIsNotASelect documents why
// "SEL/**/ECT" is rejected for a different reason than the other cases: the
// comment breaks the token into "SEL" and "ECT", neither of which matches
// \bSELECT\b on its own. It is caught anyway because stripSQLNoise inserts
// a separating space in place of the comment, matching how PostgreSQL's own
// lexer treats a comment as a token boundary rather than simple deletion —
// so "SEL/**/ECT" was never a working SELECT keyword to begin with, in this
// guard or in Postgres itself.
func TestValidateCountRowsWhereClause_SplitCommentIsNotASelect(t *testing.T) {
	residue, _ := stripSQLNoise("SEL/**/ECT 1")
	if residue == "SELECT 1" {
		t.Fatalf("comment removal must not merge SEL and ECT into SELECT: got %q", residue)
	}
}
