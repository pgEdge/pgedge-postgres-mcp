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
