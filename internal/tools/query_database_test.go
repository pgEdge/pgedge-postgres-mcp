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
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestFormatTSVValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "",
		},
		{
			name:     "string value",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "string with tab",
			input:    "hello\tworld",
			expected: "hello\\tworld",
		},
		{
			name:     "string with newline",
			input:    "hello\nworld",
			expected: "hello\\nworld",
		},
		{
			name:     "string with carriage return",
			input:    "hello\rworld",
			expected: "hello\\rworld",
		},
		{
			name:     "string with multiple special chars",
			input:    "a\tb\nc\rd",
			expected: "a\\tb\\nc\\rd",
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "int64",
			input:    int64(9223372036854775807),
			expected: "9223372036854775807",
		},
		{
			name:     "float64",
			input:    3.14159,
			expected: "3.14159",
		},
		{
			name:     "bool true",
			input:    true,
			expected: "true",
		},
		{
			name:     "bool false",
			input:    false,
			expected: "false",
		},
		{
			name:     "byte slice",
			input:    []byte("bytes"),
			expected: "bytes",
		},
		{
			name:     "array",
			input:    []interface{}{"a", "b", "c"},
			expected: `["a","b","c"]`,
		},
		{
			name:     "map",
			input:    map[string]interface{}{"key": "value"},
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTSVValue(tt.input)
			if result != tt.expected {
				t.Errorf("FormatTSVValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatTSVValue_Time(t *testing.T) {
	// Test time formatting separately since we need to construct a specific time
	testTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	result := FormatTSVValue(testTime)
	expected := "2024-06-15T10:30:00Z"
	if result != expected {
		t.Errorf("FormatTSVValue(time) = %q, want %q", result, expected)
	}
}

// TestQueryTypeDetection tests the logic for detecting query types
// This verifies the fix for DDL/DML silent failure bug where Query() was
// used instead of Exec() for non-row-returning statements
func TestQueryTypeDetection(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		isSelectQuery bool
		isDDLQuery    bool
		isDMLQuery    bool
		hasReturning  bool
		expectsRows   bool // true = use Query(), false = use Exec()
	}{
		// SELECT queries - should return rows
		{
			name:          "simple SELECT",
			query:         "SELECT * FROM users",
			isSelectQuery: true,
			expectsRows:   true,
		},
		{
			name:          "SELECT with WHERE",
			query:         "SELECT id, name FROM users WHERE active = true",
			isSelectQuery: true,
			expectsRows:   true,
		},
		{
			name:          "WITH CTE query",
			query:         "WITH active_users AS (SELECT * FROM users) SELECT * FROM active_users",
			isSelectQuery: true,
			expectsRows:   true,
		},
		{
			name:          "TABLE command",
			query:         "TABLE users",
			isSelectQuery: true,
			expectsRows:   true,
		},
		{
			name:          "VALUES expression",
			query:         "VALUES (1, 'a'), (2, 'b')",
			isSelectQuery: true,
			expectsRows:   true,
		},

		// DDL queries - should NOT return rows, use Exec()
		{
			name:        "CREATE SCHEMA",
			query:       "CREATE SCHEMA test",
			isDDLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "CREATE TABLE",
			query:       "CREATE TABLE test (id int)",
			isDDLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "DROP TABLE",
			query:       "DROP TABLE test",
			isDDLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "ALTER TABLE",
			query:       "ALTER TABLE users ADD COLUMN email text",
			isDDLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "TRUNCATE",
			query:       "TRUNCATE TABLE logs",
			isDDLQuery:  true,
			expectsRows: false,
		},

		// DML without RETURNING - should NOT return rows, use Exec()
		{
			name:        "simple INSERT",
			query:       "INSERT INTO users (name) VALUES ('test')",
			isDMLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "INSERT with SELECT",
			query:       "INSERT INTO users_backup SELECT * FROM users",
			isDMLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "simple UPDATE",
			query:       "UPDATE users SET active = false WHERE id = 1",
			isDMLQuery:  true,
			expectsRows: false,
		},
		{
			name:        "simple DELETE",
			query:       "DELETE FROM users WHERE id = 1",
			isDMLQuery:  true,
			expectsRows: false,
		},

		// DML with RETURNING - SHOULD return rows, use Query()
		{
			name:         "INSERT with RETURNING",
			query:        "INSERT INTO users (name) VALUES ('test') RETURNING id",
			isDMLQuery:   true,
			hasReturning: true,
			expectsRows:  true,
		},
		{
			name:         "UPDATE with RETURNING",
			query:        "UPDATE users SET active = false RETURNING id, name",
			isDMLQuery:   true,
			hasReturning: true,
			expectsRows:  true,
		},
		{
			name:         "DELETE with RETURNING",
			query:        "DELETE FROM users WHERE id = 1 RETURNING *",
			isDMLQuery:   true,
			hasReturning: true,
			expectsRows:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upperQuery := strings.ToUpper(strings.TrimSpace(tt.query))

			// Test SELECT detection
			isSelectQuery := strings.HasPrefix(upperQuery, "SELECT") ||
				strings.HasPrefix(upperQuery, "WITH") ||
				strings.HasPrefix(upperQuery, "TABLE") ||
				strings.HasPrefix(upperQuery, "VALUES")
			if isSelectQuery != tt.isSelectQuery {
				t.Errorf("isSelectQuery = %v, want %v", isSelectQuery, tt.isSelectQuery)
			}

			// Test DDL detection
			isDDLQuery := strings.HasPrefix(upperQuery, "CREATE") ||
				strings.HasPrefix(upperQuery, "DROP") ||
				strings.HasPrefix(upperQuery, "ALTER") ||
				strings.HasPrefix(upperQuery, "TRUNCATE")
			if isDDLQuery != tt.isDDLQuery {
				t.Errorf("isDDLQuery = %v, want %v", isDDLQuery, tt.isDDLQuery)
			}

			// Test DML detection
			isDMLQuery := strings.HasPrefix(upperQuery, "INSERT") ||
				strings.HasPrefix(upperQuery, "UPDATE") ||
				strings.HasPrefix(upperQuery, "DELETE")
			if isDMLQuery != tt.isDMLQuery {
				t.Errorf("isDMLQuery = %v, want %v", isDMLQuery, tt.isDMLQuery)
			}

			// Test RETURNING detection
			hasReturning := isDMLQuery && strings.Contains(upperQuery, "RETURNING")
			if hasReturning != tt.hasReturning {
				t.Errorf("hasReturning = %v, want %v", hasReturning, tt.hasReturning)
			}

			// Test final decision: does query return rows?
			returnsRows := isSelectQuery || hasReturning
			if returnsRows != tt.expectsRows {
				t.Errorf("returnsRows = %v, want %v (should use %s)",
					returnsRows, tt.expectsRows,
					map[bool]string{true: "Query()", false: "Exec()"}[tt.expectsRows])
			}
		})
	}
}

// TestStripTrailingSemicolons verifies that trailing semicolons are
// stripped before LIMIT/OFFSET are appended, preventing syntax errors
// like "SELECT 1; LIMIT 101". See GitHub issue #110.
func TestStripTrailingSemicolons(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no semicolon",
			input:    "SELECT 1",
			expected: "SELECT 1",
		},
		{
			name:     "single trailing semicolon",
			input:    "SELECT 1;",
			expected: "SELECT 1",
		},
		{
			name:     "semicolon with trailing space",
			input:    "SELECT 1; ",
			expected: "SELECT 1",
		},
		{
			name:     "multiple trailing semicolons",
			input:    "SELECT 1;;;",
			expected: "SELECT 1",
		},
		{
			name:     "leading and trailing whitespace",
			input:    "  SELECT 1;  ",
			expected: "  SELECT 1",
		},
		{
			name:     "interleaved trailing semicolons and spaces",
			input:    "SELECT 1; ;",
			expected: "SELECT 1",
		},
		{
			name:     "semicolons and spaces only",
			input:    " ; ;;  ",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "trailing semicolon after string literal",
			input:    "SELECT '1;2';",
			expected: "SELECT '1;2'",
		},
		{
			name:     "tabs and newlines",
			input:    "SELECT 1;\n\t",
			expected: "SELECT 1",
		},
		{
			name:     "semicolon in middle preserved",
			input:    "SELECT '1;2'",
			expected: "SELECT '1;2'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripTrailingSemicolons(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// queryHasClauseTests backs TestQueryHasClause. It verifies that LIMIT/OFFSET
// detection only fires on a real clause, not on the word "limit"/"offset"
// appearing in a string literal, a quoted identifier, a comment, or an
// identifier that merely contains it. See GitHub issue #260, where a query
// mentioning "credit limit" was wrongly treated as already capped and
// returned every row instead of the requested number.
var queryHasClauseTests = []struct {
	name           string
	query          string
	pattern        *regexp.Regexp
	expectDetected bool
}{
	{
		name:           "real LIMIT clause",
		query:          "SELECT * FROM t LIMIT 10",
		pattern:        limitKeywordPattern,
		expectDetected: true,
	},
	{
		name:           "real OFFSET clause",
		query:          "SELECT * FROM t OFFSET 10",
		pattern:        offsetKeywordPattern,
		expectDetected: true,
	},
	{
		name:           "word in string literal is not a clause",
		query:          "SELECT * FROM t WHERE note = 'credit limit exceeded'",
		pattern:        limitKeywordPattern,
		expectDetected: false,
	},
	{
		name:           "word in quoted identifier is not a clause",
		query:          `SELECT "credit limit" FROM t`,
		pattern:        limitKeywordPattern,
		expectDetected: false,
	},
	{
		name:           "word in a comment is not a clause",
		query:          "SELECT * FROM t -- check the credit limit\n",
		pattern:        limitKeywordPattern,
		expectDetected: false,
	},
	{
		name:           "column named credit_limit is not a clause",
		query:          "SELECT credit_limit FROM t",
		pattern:        limitKeywordPattern,
		expectDetected: false,
	},
	{
		name:           "word in string literal is not an offset clause",
		query:          "SELECT * FROM t WHERE note = 'apply the offset'",
		pattern:        offsetKeywordPattern,
		expectDetected: false,
	},
	{
		name:           "LIMIT inside a subquery is not an outer clause",
		query:          "SELECT * FROM parent WHERE id IN (SELECT parent_id FROM child LIMIT 1)",
		pattern:        limitKeywordPattern,
		expectDetected: false,
	},
	{
		name:           "OFFSET inside a subquery is not an outer clause",
		query:          "SELECT * FROM parent WHERE id IN (SELECT parent_id FROM child OFFSET 1)",
		pattern:        offsetKeywordPattern,
		expectDetected: false,
	},
	{
		name:           "LIMIT inside a CTE is not an outer clause",
		query:          "WITH recent AS (SELECT * FROM t LIMIT 5) SELECT * FROM recent",
		pattern:        limitKeywordPattern,
		expectDetected: false,
	},
	{
		name:           "real LIMIT clause after a subquery with its own LIMIT",
		query:          "SELECT * FROM (SELECT * FROM t LIMIT 5) sub LIMIT 10",
		pattern:        limitKeywordPattern,
		expectDetected: true,
	},
	{
		name:           "dollar-sign identifier ending in limit is not a clause",
		query:          "SELECT foo$limit FROM t",
		pattern:        limitKeywordPattern,
		expectDetected: false,
	},
	{
		name:           "dollar-sign identifier ending in offset is not a clause",
		query:          "SELECT foo$offset FROM t",
		pattern:        offsetKeywordPattern,
		expectDetected: false,
	},
}

func TestQueryHasClause(t *testing.T) {
	for _, tt := range queryHasClauseTests {
		t.Run(tt.name, func(t *testing.T) {
			got := queryHasClause(tt.pattern, tt.query)
			if got != tt.expectDetected {
				t.Errorf("queryHasClause(%q) = %v, want %v", tt.query, got, tt.expectDetected)
			}
		})
	}
}

func TestFormatResultsAsTSV(t *testing.T) {
	tests := []struct {
		name        string
		columnNames []string
		results     [][]interface{}
		expected    string
	}{
		{
			name:        "empty columns",
			columnNames: []string{},
			results:     [][]interface{}{},
			expected:    "",
		},
		{
			name:        "header only (no results)",
			columnNames: []string{"id", "name", "email"},
			results:     [][]interface{}{},
			expected:    "id\tname\temail",
		},
		{
			name:        "single row",
			columnNames: []string{"id", "name"},
			results:     [][]interface{}{{1, "Alice"}},
			expected:    "id\tname\n1\tAlice",
		},
		{
			name:        "multiple rows",
			columnNames: []string{"id", "name", "active"},
			results: [][]interface{}{
				{1, "Alice", true},
				{2, "Bob", false},
			},
			expected: "id\tname\tactive\n1\tAlice\ttrue\n2\tBob\tfalse",
		},
		{
			name:        "with null values",
			columnNames: []string{"id", "name", "email"},
			results: [][]interface{}{
				{1, "Alice", nil},
				{2, nil, "bob@example.com"},
			},
			expected: "id\tname\temail\n1\tAlice\t\n2\t\tbob@example.com",
		},
		{
			name:        "with special characters",
			columnNames: []string{"id", "notes"},
			results: [][]interface{}{
				{1, "line1\nline2"},
				{2, "col1\tcol2"},
			},
			expected: "id\tnotes\n1\tline1\\nline2\n2\tcol1\\tcol2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatResultsAsTSV(tt.columnNames, tt.results)
			if result != tt.expected {
				t.Errorf("FormatResultsAsTSV() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateReadOnlyQuery(t *testing.T) {
	tests := []struct {
		name string
		// query is the statement handed to the guard.
		query string
		// errContains, when set, is a substring the rejection must mention.
		// An empty value means the query must be accepted.
		errContains string
	}{
		// Ordinary read-only work must pass untouched.
		{
			name:  "safe SELECT query",
			query: "SELECT * FROM users",
		},
		{
			name:  "safe SELECT with WHERE clause",
			query: "SELECT id, name FROM users WHERE active = true",
		},
		{
			name:  "safe SHOW command",
			query: "SHOW server_version",
		},
		{
			name:  "safe query with transaction keyword",
			query: "SELECT * FROM transaction_logs",
		},
		{
			name:  "safe query with read_only keyword",
			query: "SELECT * FROM read_only_replicas",
		},
		{
			name:  "single statement with trailing semicolon",
			query: "SELECT 1;",
		},
		{
			// Previously rejected: the old guard matched the setting name
			// anywhere, including inside a string literal, so an ordinary
			// lookup in a configuration table was refused. The name alone is
			// harmless without something capable of changing a setting.
			name:  "setting name inside a literal is not a change attempt",
			query: "SELECT * FROM config WHERE key = 'transaction_read_only'",
		},
		{
			name:  "setting name in a comment is not a change attempt",
			query: "SELECT 1 -- transaction_read_only",
		},

		// The setting names, reached directly or through set_config. Any
		// set_config() call is now refused outright (see the dedicated
		// set_config test below), so these two specifically confirm that a
		// literal read-only-related setting name is still refused, just
		// through the broader rule rather than the GUC-name check.
		{
			name:        "uppercase TRANSACTION_READ_ONLY",
			query:       "SELECT set_config('TRANSACTION_READ_ONLY', 'off', true)",
			errContains: "set_config",
		},
		{
			name:        "mixed case Transaction_Read_Only",
			query:       "SELECT set_config('Transaction_Read_Only', 'off', true)",
			errContains: "set_config",
		},
		{
			name:        "default_transaction_read_only",
			query:       "SET default_transaction_read_only = off",
			errContains: "transaction_read_only",
		},
		{
			name:        "DEFAULT_TRANSACTION_READ_ONLY uppercase",
			query:       "SET DEFAULT_TRANSACTION_READ_ONLY TO off",
			errContains: "transaction_read_only",
		},
		{
			name:        "quoted setting name",
			query:       `SET "default_transaction_read_only" = off`,
			errContains: "transaction_read_only",
		},
		{
			name:        "RESET of the setting",
			query:       "RESET default_transaction_read_only",
			errContains: "transaction_read_only",
		},

		// The transaction access mode, which the original guard never
		// mentioned and which was the reported bypass.
		{
			name:        "SET TRANSACTION READ WRITE",
			query:       "SET TRANSACTION READ WRITE",
			errContains: "read-write transaction mode",
		},
		{
			name:        "lower case set transaction read write",
			query:       "set transaction read write",
			errContains: "read-write transaction mode",
		},
		{
			name:        "BEGIN READ WRITE",
			query:       "BEGIN READ WRITE",
			errContains: "read-write transaction mode",
		},
		{
			name:        "START TRANSACTION READ WRITE",
			query:       "START TRANSACTION READ WRITE",
			errContains: "read-write transaction mode",
		},
		{
			name:        "READ WRITE split across whitespace and a comment",
			query:       "SET TRANSACTION READ /* sneaky */ WRITE",
			errContains: "read-write transaction mode",
		},
		{
			name:        "SET SESSION CHARACTERISTICS AS TRANSACTION READ WRITE",
			query:       "SET SESSION CHARACTERISTICS AS TRANSACTION READ WRITE",
			errContains: "read-write transaction mode",
		},
		{
			name:        "SET SESSION CHARACTERISTICS on its own",
			query:       "SET SESSION CHARACTERISTICS AS TRANSACTION ISOLATION LEVEL SERIALIZABLE",
			errContains: "session transaction characteristics",
		},

		// Session state resets, which clear the session-level default
		// applied when the pooled connection was established.
		{
			name:        "RESET ALL",
			query:       "RESET ALL",
			errContains: "RESET ALL",
		},
		{
			name:        "DISCARD ALL",
			query:       "DISCARD ALL",
			errContains: "DISCARD",
		},

		// Transaction control, which ends the transaction the server opened.
		{
			name:        "COMMIT",
			query:       "COMMIT",
			errContains: "transaction control",
		},
		{
			name:        "ROLLBACK",
			query:       "ROLLBACK",
			errContains: "transaction control",
		},
		{
			name:        "BEGIN on its own",
			query:       "BEGIN",
			errContains: "transaction control",
		},

		// Statement smuggling, the channel that made the above exploitable.
		{
			name:        "two statements",
			query:       "SELECT 1; DELETE FROM users",
			errContains: "multiple SQL statements",
		},
		{
			name:        "commit then write",
			query:       "COMMIT; BEGIN READ WRITE; CREATE TABLE pwn(i int); COMMIT",
			errContains: "multiple SQL statements",
		},
		{
			name:        "statement hidden behind a leading comment",
			query:       "/* comment */ SELECT 1; CREATE TABLE pwn(i int)",
			errContains: "multiple SQL statements",
		},
		{
			name:        "semicolon inside a literal is not a separator",
			query:       "SELECT * FROM t WHERE note = 'a; b'",
			errContains: "",
		},

		// Role changes.
		{
			name:        "SET ROLE",
			query:       "SET ROLE postgres",
			errContains: "SET ROLE",
		},
		{
			name:        "SET SESSION AUTHORIZATION",
			query:       "SET SESSION AUTHORIZATION postgres",
			errContains: "SESSION AUTHORIZATION",
		},
		{
			name:        "ALTER ROLE persisting a changed default",
			query:       "ALTER ROLE mcp_app SET default_transaction_read_only = off",
			errContains: "ALTER ROLE",
		},

		// Writes that happen outside the transaction's scope, which the
		// read-only access mode does not prevent at all.
		{
			name:        "DO block",
			query:       "DO $$ BEGIN PERFORM 1; END $$",
			errContains: "DO blocks",
		},
		{
			name:        "DO block bypass attempt",
			query:       "DO $$ BEGIN PERFORM set_config('transaction_read_only', 'off', true); EXECUTE 'DELETE FROM users'; END $$",
			errContains: "DO blocks",
		},
		{
			name:        "COPY TO PROGRAM",
			query:       "COPY (SELECT 1) TO PROGRAM 'curl http://example.com'",
			errContains: "COPY ... TO PROGRAM",
		},
		{
			name:        "lo_export",
			query:       "SELECT lo_export(1234, '/tmp/x')",
			errContains: "server-side file modification",
		},
		{
			name:        "dblink write",
			query:       "SELECT dblink_exec('dbname=postgres', 'CREATE TABLE pwn(i int)')",
			errContains: "dblink",
		},
		// set_config() is refused outright, not just when it names a
		// read-only-related setting as literal text: found by manual testing
		// against a live database, a call whose arguments are built from
		// chr()/concatenation at runtime evades the GUC-name check (residue
		// and bare both only ever see the chr()/|| expression, never the
		// literal string "default_transaction_read_only"), and was confirmed
		// to actually flip the session's default_transaction_read_only to
		// "off" against a real server before this rule existed.
		{
			name:        "set_config with a literal setting name",
			query:       "SELECT set_config('some_other_setting', 'value', false)",
			errContains: "set_config",
		},
		{
			name: "set_config with the setting name built at runtime, not written as text",
			query: "SELECT set_config(" +
				"chr(100)||chr(101)||chr(102)||chr(97)||chr(117)||chr(108)||chr(116)||chr(95)||" +
				"chr(116)||chr(114)||chr(97)||chr(110)||chr(115)||chr(97)||chr(99)||chr(116)||" +
				"chr(105)||chr(111)||chr(110)||chr(95)||chr(114)||chr(101)||chr(97)||chr(100)||" +
				"chr(95)||chr(111)||chr(110)||chr(108)||chr(121), chr(111)||chr(102)||chr(102), false)",
			errContains: "set_config",
		},
		{
			name:        "schema-qualified pg_catalog.set_config",
			query:       "SELECT pg_catalog.set_config('some_other_setting', 'value', false)",
			errContains: "set_config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReadOnlyQuery(tt.query)

			if tt.errContains == "" {
				if err != nil {
					t.Errorf("unexpected rejection of %q: %v", tt.query, err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected rejection of %q, got nil", tt.query)
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("rejection of %q should mention %q, got: %v",
					tt.query, tt.errContains, err)
			}
		})
	}
}

func TestStripSQLNoise(t *testing.T) {
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
			name:        "string literal is emptied in the residue only",
			query:       "SELECT 'abc'",
			wantResidue: "SELECT ''",
			wantBare:    "SELECT 'abc'",
		},
		{
			name:        "doubled quote does not end the literal",
			query:       "SELECT 'a''b', 1",
			wantResidue: "SELECT '', 1",
			wantBare:    "SELECT 'a''b', 1",
		},
		{
			name:        "line comment becomes a separator",
			query:       "SELECT 1 -- DROP TABLE t\n, 2",
			wantResidue: "SELECT 1  \n, 2",
			wantBare:    "SELECT 1  \n, 2",
		},
		{
			name:        "nested block comment is removed entirely",
			query:       "SELECT /* a /* b */ c */ 1",
			wantResidue: "SELECT   1",
			wantBare:    "SELECT   1",
		},
		{
			name:        "quoted identifier is emptied in the residue only",
			query:       `SELECT "col" FROM t`,
			wantResidue: `SELECT "" FROM t`,
			wantBare:    `SELECT "col" FROM t`,
		},
		{
			name:        "dollar quoted body is emptied in the residue only",
			query:       "DO $tag$ DELETE FROM t; $tag$",
			wantResidue: "DO $tag$$tag$",
			wantBare:    "DO $tag$ DELETE FROM t; $tag$",
		},
		{
			name:        "positional parameters are not dollar quotes",
			query:       "SELECT $1, $2",
			wantResidue: "SELECT $1, $2",
			wantBare:    "SELECT $1, $2",
		},
		{
			// An unterminated construct must not swallow the rest of the
			// statement into a literal, because that would hide code from
			// every check that runs on the residue.
			name:        "unterminated dollar quote is treated as code",
			query:       "SELECT $tag$ ; DROP TABLE t",
			wantResidue: "SELECT $tag$ ; DROP TABLE t",
			wantBare:    "SELECT $tag$ ; DROP TABLE t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			residue, bare := stripSQLNoise(tt.query)
			if residue != tt.wantResidue {
				t.Errorf("residue: got %q, want %q", residue, tt.wantResidue)
			}
			if bare != tt.wantBare {
				t.Errorf("bare: got %q, want %q", bare, tt.wantBare)
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
		{"SELECT 1;  ", false},
		{"SELECT 1;;", false},
		{"SELECT 1; SELECT 2", true},
		{"SELECT 1; SELECT 2;", true},
	}

	for _, tt := range tests {
		t.Run(tt.residue, func(t *testing.T) {
			if got := hasMultipleStatements(tt.residue); got != tt.want {
				t.Errorf("hasMultipleStatements(%q) = %v, want %v",
					tt.residue, got, tt.want)
			}
		})
	}
}
