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

import "testing"

func TestClassifyQuery(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantType  QueryType
		wantWrite bool
	}{
		// Read queries
		{"select", "SELECT * FROM users", QueryTypeSelect, false},
		{"select lowercase", "select id from orders", QueryTypeSelect, false},
		{"select mixed case", "Select Name From Products", QueryTypeSelect, false},
		{"select with leading space", "  SELECT 1", QueryTypeSelect, false},
		{"with cte", "WITH cte AS (SELECT 1) SELECT * FROM cte", QueryTypeSelect, false},
		{"table command", "TABLE users", QueryTypeSelect, false},
		{"values expression", "VALUES (1, 'a'), (2, 'b')", QueryTypeSelect, false},
		{"explain", "EXPLAIN SELECT * FROM users", QueryTypeSelect, false},
		{"explain analyze", "EXPLAIN ANALYZE SELECT * FROM users", QueryTypeSelect, false},
		{"show", "SHOW search_path", QueryTypeSelect, false},

		// DDL queries
		{"create table", "CREATE TABLE test (id int)", QueryTypeDDL, true},
		{"create index", "CREATE INDEX idx ON test (id)", QueryTypeDDL, true},
		{"drop table", "DROP TABLE IF EXISTS test", QueryTypeDDL, true},
		{"alter table", "ALTER TABLE test ADD COLUMN name text", QueryTypeDDL, true},
		{"truncate", "TRUNCATE TABLE test", QueryTypeDDL, true},
		{"create lowercase", "create table t (id int)", QueryTypeDDL, true},

		// DML queries
		{"insert", "INSERT INTO users (name) VALUES ('Alice')", QueryTypeDML, true},
		{"update", "UPDATE users SET name = 'Bob' WHERE id = 1", QueryTypeDML, true},
		{"delete", "DELETE FROM users WHERE id = 1", QueryTypeDML, true},
		{"insert lowercase", "insert into t values (1)", QueryTypeDML, true},

		// Other queries (treated as write)
		{"grant", "GRANT SELECT ON users TO reader", QueryTypeOther, true},
		{"revoke", "REVOKE ALL ON users FROM reader", QueryTypeOther, true},
		{"vacuum", "VACUUM ANALYZE users", QueryTypeOther, true},
		{"analyze", "ANALYZE users", QueryTypeOther, true},
		{"begin", "BEGIN", QueryTypeOther, true},
		{"commit", "COMMIT", QueryTypeOther, true},
		{"set", "SET timezone TO 'UTC'", QueryTypeOther, true},

		// Writes hiding behind a reading first keyword. Each of these runs a
		// write on a writable connection, so each must raise the prompt.
		{"select into", "SELECT * INTO backup FROM users", QueryTypeDDL, true},
		{"select into lowercase", "select id into t2 from t1", QueryTypeDDL, true},
		{"select into strict spacing", "SELECT 1 INTO\nnewtbl", QueryTypeDDL, true},
		{"cte writing insert",
			"WITH c AS (SELECT 1 AS n) INSERT INTO t SELECT n FROM c",
			QueryTypeDML, true},
		{"cte returning insert",
			"WITH ins AS (INSERT INTO t VALUES (1) RETURNING *) SELECT * FROM ins",
			QueryTypeDML, true},
		{"cte returning update",
			"WITH u AS (UPDATE t SET n = 1 RETURNING *) SELECT * FROM u",
			QueryTypeDML, true},
		{"cte returning delete",
			"WITH d AS (DELETE FROM t RETURNING *) SELECT * FROM d",
			QueryTypeDML, true},
		{"cte select into", "WITH c AS (SELECT 1) SELECT * INTO t FROM c",
			QueryTypeDDL, true},
		{"explain analyze insert",
			"EXPLAIN ANALYZE INSERT INTO t VALUES (1)", QueryTypeDML, true},
		{"explain analyze options update",
			"EXPLAIN (ANALYZE, BUFFERS) UPDATE t SET n = 1", QueryTypeDML, true},
		{"comment hiding select into",
			"/* harmless */ SELECT * INTO backup FROM users", QueryTypeDDL, true},
		{"merge", "MERGE INTO t USING s ON t.id = s.id " +
			"WHEN MATCHED THEN UPDATE SET n = s.n", QueryTypeDML, true},

		// An escape string constant must not be allowed to swallow the rest of
		// the statement. Read as a doubled quote rather than an escape, E'\''
		// runs to the end of the text and hides the INTO behind it.
		{"escape string hiding select into",
			`SELECT E'\'' INTO backup FROM users`, QueryTypeDDL, true},
		{"escape string hiding select into after a comma",
			`SELECT E'a\'' , x INTO t FROM u`, QueryTypeDDL, true},
		{"lowercase escape string hiding select into",
			`SELECT e'\'' INTO backup FROM users`, QueryTypeDDL, true},

		// A dollar-quote tag glued onto the end of an identifier must not be
		// read as a delimiter: PostgreSQL treats x$tag$ as a single
		// identifier, and misreading it opens a real, attacker-chosen
		// dollar-quoted block that can swallow a smuggled statement.
		{"dollar quote decoy hiding a delete",
			"SELECT 1 AS x$tag$; DELETE FROM t -- $tag$", QueryTypeDML, true},
		{"dollar quote decoy without a tag hiding a delete",
			"SELECT 1 AS x$$; DELETE FROM t -- $$", QueryTypeDML, true},

		// Reads that must stay quiet, so that the prompt keeps its meaning.
		{"explain insert without analyze",
			"EXPLAIN INSERT INTO t VALUES (1)", QueryTypeSelect, false},
		{"explain analyze select",
			"EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM t", QueryTypeSelect, false},
		{"write keyword inside a literal",
			"SELECT * FROM audit WHERE action = 'DELETE'", QueryTypeSelect, false},
		{"write keyword inside an escape string literal",
			`SELECT * FROM audit WHERE action = E'DEL\'ETE'`,
			QueryTypeSelect, false},
		{"backslash in a plain literal is an ordinary character",
			`SELECT * FROM t WHERE path = 'C:\'`, QueryTypeSelect, false},
		{"write keyword inside a quoted identifier",
			`SELECT "delete" FROM t`, QueryTypeSelect, false},
		{"write keyword as part of an identifier",
			"SELECT delete_flag, into_tray FROM t", QueryTypeSelect, false},
		{"select for update", "SELECT * FROM t FOR UPDATE", QueryTypeSelect, false},
		{"select for no key update",
			"SELECT * FROM t FOR NO KEY UPDATE", QueryTypeSelect, false},
		{"select for share", "SELECT * FROM t FOR SHARE", QueryTypeSelect, false},
		{"comment before an ordinary select",
			"-- counting rows\nSELECT count(*) FROM t", QueryTypeSelect, false},
		{"subquery select", "SELECT * FROM t WHERE id IN (SELECT id FROM u)",
			QueryTypeSelect, false},

		// Edge cases
		{"empty string", "", QueryTypeOther, true},
		{"whitespace only", "   ", QueryTypeOther, true},
		{"comment only", "-- nothing here", QueryTypeOther, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotWrite := ClassifyQuery(tt.sql)
			if gotType != tt.wantType {
				t.Errorf("ClassifyQuery(%q) type = %d, want %d",
					tt.sql, gotType, tt.wantType)
			}
			if gotWrite != tt.wantWrite {
				t.Errorf("ClassifyQuery(%q) isWrite = %v, want %v",
					tt.sql, gotWrite, tt.wantWrite)
			}
		})
	}
}
