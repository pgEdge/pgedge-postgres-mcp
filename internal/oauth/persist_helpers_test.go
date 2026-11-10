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
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/authdb"
)

// noErr fails the test at once when err is not nil.
func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// mustOpenDB opens auth.db in dir, failing the test on error. The caller
// closes the database.
func mustOpenDB(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, _, err := authdb.Open(dir)
	noErr(t, err)
	return db
}

// mustPersistentStore builds a persistent store on db with the given
// limits, failing the test on error.
func mustPersistentStore(t *testing.T, db *sql.DB, limits Limits) *Store {
	t.Helper()
	s, err := NewPersistentStore(db, limits, time.Hour, nil)
	noErr(t, err)
	return s
}

// openLoggingStore opens a persistent store in a fresh directory whose
// log function counts its calls in *logged. The caller closes the
// returned database, usually to make later writes fail.
func openLoggingStore(t *testing.T, logged *int) (*Store, *sql.DB) {
	t.Helper()
	db := mustOpenDB(t, t.TempDir())
	s, err := NewPersistentStore(db, DefaultLimits, time.Hour,
		func(string, ...any) { *logged++ })
	noErr(t, err)
	return s, db
}

// rowCountQueries maps each table that tests count to its full query, so
// that no SQL is assembled from strings.
var rowCountQueries = map[string]string{
	"oauth_clients":    `SELECT COUNT(*) FROM oauth_clients`,
	"oauth_codes":      `SELECT COUNT(*) FROM oauth_codes`,
	"oauth_used_codes": `SELECT COUNT(*) FROM oauth_used_codes`,
	"oauth_devices":    `SELECT COUNT(*) FROM oauth_devices`,
	"oauth_tokens":     `SELECT COUNT(*) FROM oauth_tokens`,
	"oauth_rotated":    `SELECT COUNT(*) FROM oauth_rotated`,
}

// countRows returns the number of rows in one of the tables listed in
// rowCountQueries.
func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	query, ok := rowCountQueries[table]
	if !ok {
		t.Fatalf("no count query for table %q", table)
	}
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("counting %s: %v", table, err)
	}
	return n
}

// requireRows fails the test unless table holds want rows; label names
// the rows in the failure message.
func requireRows(t *testing.T, db *sql.DB, table, label string, want int) {
	t.Helper()
	if n := countRows(t, db, table); n != want {
		t.Fatalf("%s rows = %d, want %d", label, n, want)
	}
}

// requireNoRows fails the test unless the query returns a count of zero.
func requireNoRows(t *testing.T, db *sql.DB, query, what string) {
	t.Helper()
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%s: n=%d err=%v", what, n, err)
	}
}

// requirePresence fails the test with msg when ok differs from want.
func requirePresence(t *testing.T, ok, want bool, msg string) {
	t.Helper()
	if ok != want {
		t.Fatal(msg)
	}
}

// requireToken fails with msg unless GetToken finds hash exactly when want.
func requireToken(t *testing.T, s *Store, hash string, want bool, msg string) {
	t.Helper()
	_, ok := s.GetToken(hash)
	requirePresence(t, ok, want, msg)
}

// requireClient fails with msg unless GetClient finds id exactly when want.
func requireClient(t *testing.T, s *Store, id string, want bool, msg string) {
	t.Helper()
	_, ok := s.GetClient(id)
	requirePresence(t, ok, want, msg)
}

// requireTakeCode fails with msg unless TakeCode finds hash exactly when
// want. A successful take consumes the code.
func requireTakeCode(t *testing.T, s *Store, hash string, want bool, msg string) {
	t.Helper()
	_, ok := s.TakeCode(hash)
	requirePresence(t, ok, want, msg)
}

// requireTakeToken fails with msg unless TakeToken finds hash exactly when
// want. A successful take consumes the token.
func requireTakeToken(t *testing.T, s *Store, hash string, want bool, msg string) {
	t.Helper()
	_, ok := s.TakeToken(hash)
	requirePresence(t, ok, want, msg)
}

// requireDevice fails with msg unless GetDeviceByHash finds hash exactly
// when want.
func requireDevice(t *testing.T, s *Store, hash string, want bool, msg string) {
	t.Helper()
	_, ok := s.GetDeviceByHash(hash)
	requirePresence(t, ok, want, msg)
}

// requireDeviceUserCode fails with msg unless GetDeviceByUserCode finds
// code exactly when want.
func requireDeviceUserCode(t *testing.T, s *Store, code string, want bool, msg string) {
	t.Helper()
	_, ok := s.GetDeviceByUserCode(code)
	requirePresence(t, ok, want, msg)
}

// requireRotated fails unless hash is on the rotation list under family.
func requireRotated(t *testing.T, s *Store, hash, family string) {
	t.Helper()
	fam, ok := s.RotatedRefreshFamily(hash)
	if !ok || fam != family {
		t.Fatalf("rotated record lost: %q %v", fam, ok)
	}
}

// requireRotatedIs fails with msg unless hash is on the rotation list
// exactly when want.
func requireRotatedIs(t *testing.T, s *Store, hash string, want bool, msg string) {
	t.Helper()
	_, ok := s.RotatedRefreshFamily(hash)
	requirePresence(t, ok, want, msg)
}

// mustPutClient stores c, failing the test on error.
func mustPutClient(t *testing.T, s *Store, c *Client) {
	t.Helper()
	noErr(t, s.PutClient(c))
}

// mustPutCode stores c, failing the test on error.
func mustPutCode(t *testing.T, s *Store, c *AuthCode) {
	t.Helper()
	noErr(t, s.PutCode(c))
}

// mustPutToken stores tok, failing the test on error.
func mustPutToken(t *testing.T, s *Store, tok *Token) {
	t.Helper()
	noErr(t, s.PutToken(tok))
}

// requireFails fails with msg when err is nil.
func requireFails(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatal(msg)
	}
}

// requireEq fails the test unless got equals want; what names the value.
func requireEq[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}

// requireRevoked fails the test unless revoked holds exactly the one hash
// want.
func requireRevoked(t *testing.T, revoked []string, want string) {
	t.Helper()
	if len(revoked) != 1 || revoked[0] != want {
		t.Fatalf("revoked = %v, want [%s]", revoked, want)
	}
}

// mustGetToken returns the token stored under hash, failing the test with
// msg when there is none.
func mustGetToken(t *testing.T, s *Store, hash, msg string) *Token {
	t.Helper()
	tok, ok := s.GetToken(hash)
	if !ok {
		t.Fatal(msg)
	}
	return tok
}

// mustGetDevice returns the device stored under hash, failing the test
// with msg when there is none.
func mustGetDevice(t *testing.T, s *Store, hash, msg string) *DeviceCode {
	t.Helper()
	d, ok := s.GetDeviceByHash(hash)
	if !ok {
		t.Fatal(msg)
	}
	return d
}

// requireTakeDevice fails with msg unless TakeDeviceIfApproved succeeds
// exactly when want.
func requireTakeDevice(t *testing.T, s *Store, hash string, want bool, msg string) {
	t.Helper()
	_, ok := s.TakeDeviceIfApproved(hash)
	requirePresence(t, ok, want, msg)
}

// mustExec runs a statement on db, failing the test on error.
func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

// withDB returns an Options setter that backs the server with db.
func withDB(db *sql.DB) func(*Options) {
	return func(o *Options) {
		o.DB = db
		// Persisted expiry is checked against the real clock at load.
		o.Now = time.Now
	}
}
