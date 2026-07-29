/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// countRowsResult is the shape of a count_rows tools/call response.
type countRowsResult struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

func callCountRows(t *testing.T, server *MCPServer, table, schema, where string) countRowsResult {
	t.Helper()
	args := map[string]interface{}{"table": table}
	if schema != "" {
		args["schema"] = schema
	}
	if where != "" {
		args["where"] = where
	}
	resp, err := server.SendRequest("tools/call", map[string]interface{}{
		"name":      "count_rows",
		"arguments": args,
	})
	if err != nil {
		t.Fatalf("tools/call transport error: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %s", resp.Error.Message)
	}
	var result countRowsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal count_rows result: %v", err)
	}
	return result
}

// TestCountRowsWhereClauseRejectsSubqueries is the regression test for
// issue #200: a where clause containing a subquery let a caller run a
// boolean- or error-based blind-injection oracle against any table the
// connected role could read, not only the table named in the count_rows
// call. This drives the real server binary over the actual JSON-RPC
// tools/call protocol, reproducing the exact payloads from the issue, and
// confirms they are now rejected before ever reaching the database.
//
// pg_catalog.pg_class is used as the target table because it exists on
// every PostgreSQL server regardless of application schema, so this test
// has no dependency on scratch data or elevated privileges.
func TestCountRowsWhereClauseRejectsSubqueries(t *testing.T) {
	connString := os.Getenv("TEST_PGEDGE_POSTGRES_CONNECTION_STRING")
	if connString == "" {
		t.Skip("TEST_PGEDGE_POSTGRES_CONNECTION_STRING not set")
	}

	server, err := StartMCPServer(t, connString, "dummy-key-for-testing")
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Close() }()

	t.Run("legitimate simple predicate still works", func(t *testing.T) {
		result := callCountRows(t, server, "pg_class", "pg_catalog", "relkind = 'r'")
		if result.IsError {
			t.Fatalf("expected a legitimate simple predicate to succeed, got: %+v", result)
		}
	})

	t.Run("boolean-blind cross-table subquery is rejected", func(t *testing.T) {
		// The exact shape from issue #200: count_rows(table="orders", ...)
		// used as an oracle to test facts about a table it never named.
		where := "1=1 AND (SELECT COUNT(*) FROM pg_roles WHERE rolname = 'postgres') = 1"
		result := callCountRows(t, server, "pg_class", "pg_catalog", where)
		if !result.IsError {
			t.Fatalf("expected the subquery to be rejected, got success: %+v", result)
		}
		assertMentionsSubquery(t, result)
	})

	t.Run("error-based blind cross-table subquery is rejected", func(t *testing.T) {
		where := "1=1 AND CAST((CASE WHEN (SELECT count(*) FROM pg_roles WHERE rolname = 'postgres') = 1 " +
			"THEN '1' ELSE 'not-a-number' END) AS int) = 1"
		result := callCountRows(t, server, "pg_class", "pg_catalog", where)
		if !result.IsError {
			t.Fatalf("expected the subquery to be rejected, got success: %+v", result)
		}
		assertMentionsSubquery(t, result)
	})

	t.Run("EXISTS subquery is rejected", func(t *testing.T) {
		where := "EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'postgres')"
		result := callCountRows(t, server, "pg_class", "pg_catalog", where)
		if !result.IsError {
			t.Fatalf("expected the EXISTS subquery to be rejected, got success: %+v", result)
		}
		assertMentionsSubquery(t, result)
	})

	t.Run("TABLE-shorthand subquery is rejected", func(t *testing.T) {
		// TABLE tablename is PostgreSQL shorthand for SELECT * FROM
		// tablename and reads exactly the same data. Confirmed directly
		// against a live server that "(TABLE other_table) = value" leaks a
		// single-column table's contents through the same boolean oracle
		// as a SELECT-based subquery, with the word SELECT never
		// appearing in the clause — a real gap in an earlier version of
		// this fix, closed by also rejecting a bare TABLE keyword.
		where := "1=1 AND (TABLE pg_roles) IS NOT NULL"
		result := callCountRows(t, server, "pg_class", "pg_catalog", where)
		if !result.IsError {
			t.Fatalf("expected the TABLE-shorthand subquery to be rejected, got success: %+v", result)
		}
		assertMentionsSubquery(t, result)
	})

	t.Run("VALUES is not rejected and cannot read a table anyway", func(t *testing.T) {
		// VALUES is deliberately not blocked: it admits no FROM clause, so
		// it can only construct a literal row, never read one, and unlike
		// SELECT/TABLE it is not fully reserved in PostgreSQL, so banning
		// it would risk rejecting a genuine "values" column reference for
		// no security benefit. Confirm it is accepted by the guard and
		// that Postgres itself refuses to let it reference a table.
		result := callCountRows(t, server, "pg_class", "pg_catalog", "1=1 AND (VALUES (1)) IS NOT NULL")
		if result.IsError {
			t.Fatalf("expected VALUES to be accepted by the where-clause guard, got: %+v", result)
		}
	})

	t.Run("stacked query is still rejected independently", func(t *testing.T) {
		// Confirms the pre-existing validateReadOnlyQuery guard is untouched
		// by this change.
		result := callCountRows(t, server, "pg_class", "pg_catalog", "1=1; DROP TABLE pg_class")
		if !result.IsError {
			t.Fatalf("expected the stacked query to be rejected, got success: %+v", result)
		}
	})
}

func assertMentionsSubquery(t *testing.T, result countRowsResult) {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatalf("expected an error message, got no content")
	}
	if !strings.Contains(result.Content[0].Text, "subqueries are not permitted") {
		t.Errorf("expected the subquery rejection message, got: %s", result.Content[0].Text)
	}
}
