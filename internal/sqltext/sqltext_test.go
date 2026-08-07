/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package sqltext

import "testing"

func TestStrip(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantResidue string
		wantBare    string
	}{
		{
			name:        "plain statement is unchanged",
			query:       "SELECT 1",
			wantResidue: "SELECT 1",
			wantBare:    "SELECT 1",
		},
		{
			name:        "string literal is blanked in the residue only",
			query:       "SELECT 'DELETE'",
			wantResidue: "SELECT ''",
			wantBare:    "SELECT 'DELETE'",
		},
		{
			name:        "quoted identifier is blanked in the residue only",
			query:       `SELECT "delete" FROM t`,
			wantResidue: `SELECT "" FROM t`,
			wantBare:    `SELECT "delete" FROM t`,
		},
		{
			name:        "doubled quote does not end a literal early",
			query:       "SELECT 'it''s DELETE'",
			wantResidue: "SELECT ''",
			wantBare:    "SELECT 'it''s DELETE'",
		},
		{
			name:        "line comment becomes a separator",
			query:       "SELECT 1 -- DROP TABLE t\n",
			wantResidue: "SELECT 1  \n",
			wantBare:    "SELECT 1  \n",
		},
		{
			name:        "block comment becomes a separator",
			query:       "SEL/* hidden */ECT 1",
			wantResidue: "SEL ECT 1",
			wantBare:    "SEL ECT 1",
		},
		{
			name:        "nested block comment is consumed whole",
			query:       "SELECT /* a /* b */ c */ 1",
			wantResidue: "SELECT   1",
			wantBare:    "SELECT   1",
		},
		{
			name:        "dollar quoted body is blanked in the residue only",
			query:       "DO $tag$ DROP TABLE t $tag$",
			wantResidue: "DO $tag$$tag$",
			wantBare:    "DO $tag$ DROP TABLE t $tag$",
		},
		{
			name:        "positional parameter is not a dollar quote",
			query:       "SELECT * FROM t WHERE id = $1",
			wantResidue: "SELECT * FROM t WHERE id = $1",
			wantBare:    "SELECT * FROM t WHERE id = $1",
		},
		{
			// Read as a doubled quote the literal would run to the end of the
			// statement and hide the INTO, which is a write going unseen.
			name:        "escape string backslash-quote does not close the literal",
			query:       `SELECT E'\'' INTO backup FROM users`,
			wantResidue: `SELECT E'' INTO backup FROM users`,
			wantBare:    `SELECT E'\'' INTO backup FROM users`,
		},
		{
			name:        "escape string with a trailing backslash escape",
			query:       `SELECT E'a\'' , x INTO t FROM u`,
			wantResidue: `SELECT E'' , x INTO t FROM u`,
			wantBare:    `SELECT E'a\'' , x INTO t FROM u`,
		},
		{
			name:        "lowercase e prefix is also an escape string",
			query:       `SELECT e'\'' INTO t`,
			wantResidue: `SELECT e'' INTO t`,
			wantBare:    `SELECT e'\'' INTO t`,
		},
		{
			name:        "escaped backslash does not escape the closing quote",
			query:       `SELECT E'a\\' INTO t`,
			wantResidue: `SELECT E'' INTO t`,
			wantBare:    `SELECT E'a\\' INTO t`,
		},
		{
			// standard_conforming_strings is on by default, so this backslash
			// is an ordinary character and the literal ends at the next quote.
			name:        "plain literal does not honour a backslash escape",
			query:       `SELECT 'a\' , x INTO t FROM u`,
			wantResidue: `SELECT '' , x INTO t FROM u`,
			wantBare:    `SELECT 'a\' , x INTO t FROM u`,
		},
		{
			name:        "an E ending an identifier is not an escape prefix",
			query:       `SELECT * FROM table_e'a\' , x`,
			wantResidue: `SELECT * FROM table_e'' , x`,
			wantBare:    `SELECT * FROM table_e'a\' , x`,
		},
		{
			name:        "unterminated literal consumes the remainder",
			query:       "SELECT 'oops",
			wantResidue: "SELECT ''",
			wantBare:    "SELECT 'oops",
		},
		{
			name:        "unclosed dollar quote falls through as code",
			query:       "SELECT $tag$ DROP TABLE t",
			wantResidue: "SELECT $tag$ DROP TABLE t",
			wantBare:    "SELECT $tag$ DROP TABLE t",
		},
		{
			// A $ that continues an identifier is not a delimiter: PostgreSQL
			// treats x$tag$ as one identifier. Reading it as one anyway
			// mistook the second $ for the tag's own closing mark, forming a
			// bogus tag "$tag$" whose next occurrence, planted in the
			// trailing comment, closed a body that swallowed the DELETE.
			name:        "dollar sign continuing an identifier does not start a tag",
			query:       "SELECT 1 AS x$tag$; DELETE FROM t -- $tag$",
			wantResidue: "SELECT 1 AS x$tag$; DELETE FROM t  ",
			wantBare:    "SELECT 1 AS x$tag$; DELETE FROM t  ",
		},
		{
			name:        "dollar quote immediately after an identifier without a tag",
			query:       "SELECT 1 AS x$$; DELETE FROM t -- $$",
			wantResidue: "SELECT 1 AS x$$; DELETE FROM t  ",
			wantBare:    "SELECT 1 AS x$$; DELETE FROM t  ",
		},
		{
			// The same decoy hiding a single statement's own INTO, rather
			// than a smuggled second statement, is the more severe case: a
			// SELECT INTO genuinely writes, and unlike a smuggled statement
			// it is not stopped by the one-statement-per-message extended
			// query protocol, since it really is only one statement.
			name:        "dollar sign continuing an identifier does not hide a SELECT INTO",
			query:       "SELECT 1 AS x$tag$ INTO backup FROM users -- $tag$",
			wantResidue: "SELECT 1 AS x$tag$ INTO backup FROM users  ",
			wantBare:    "SELECT 1 AS x$tag$ INTO backup FROM users  ",
		},
		{
			name:        "dollar quote after a space still opens normally",
			query:       "DO $ $tag$ DROP TABLE t $tag$",
			wantResidue: "DO $ $tag$$tag$",
			wantBare:    "DO $ $tag$ DROP TABLE t $tag$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			residue, bare := Strip(tt.query)
			if residue != tt.wantResidue {
				t.Errorf("Strip(%q) residue = %q, want %q",
					tt.query, residue, tt.wantResidue)
			}
			if bare != tt.wantBare {
				t.Errorf("Strip(%q) bare = %q, want %q",
					tt.query, bare, tt.wantBare)
			}
		})
	}
}

func TestHasMultipleStatements(t *testing.T) {
	tests := []struct {
		residue string
		want    bool
	}{
		{"SELECT 1", false},
		{"SELECT 1;", false},
		{"SELECT 1;  \n", false},
		{"SELECT 1; SELECT 2", true},
		{"SELECT 1; COMMIT; SET x = 1", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.residue, func(t *testing.T) {
			if got := HasMultipleStatements(tt.residue); got != tt.want {
				t.Errorf("HasMultipleStatements(%q) = %v, want %v",
					tt.residue, got, tt.want)
			}
		})
	}
}
