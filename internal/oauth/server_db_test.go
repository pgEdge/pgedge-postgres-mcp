/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package oauth

import (
	"database/sql"
	"net/url"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/authdb"
)

func TestNew_WithDBSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	db, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	withDB := func(db *sql.DB) func(*Options) {
		return func(o *Options) {
			o.DB = db
			// Persisted expiry is checked against the real clock at load.
			o.Now = time.Now
		}
	}
	ts := newTestServer(t, withDB(db))
	cid, code := obtainCode(t, ts, claudeCB)
	rec, tr := exchange(ts, codeForm(cid, code, claudeCB))
	if rec.Code != 200 {
		t.Fatalf("exchange: %d %s", rec.Code, rec.Body)
	}
	ts.srv.Close()
	// Close does not close the database; it must still be usable.
	if err := db.Ping(); err != nil {
		t.Fatalf("Server.Close closed the database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db2, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	ts2 := newTestServer(t, withDB(db2))
	sub, gotCID, ok := ts2.srv.ValidateAccessToken(tr.AccessToken)
	if !ok || sub != "alice" || gotCID != cid {
		t.Fatalf("token did not survive restart: %q %q %v", sub, gotCID, ok)
	}
	if rec, _ := exchange(ts2, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tr.RefreshToken}, "client_id": {cid}}); rec.Code != 200 {
		t.Fatalf("refresh after restart: %d %s", rec.Code, rec.Body)
	}
}

func TestNew_BrokenDBFails(t *testing.T) {
	db, _, err := authdb.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if srv, err := New(Options{Config: minimalConfig(), Authenticator: &fakeAuthenticator{}, DB: db}); err == nil {
		srv.Close()
		t.Fatal("New succeeded with a closed database")
	}
}
