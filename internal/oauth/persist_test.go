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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/authdb"
)

// openPersistent opens auth.db in dir and builds a persistent store.
func openPersistent(t *testing.T, dir string, limits Limits) (*Store, *sql.DB) {
	t.Helper()
	db, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatalf("authdb.Open: %v", err)
	}
	s, err := NewPersistentStore(db, limits, time.Hour, nil)
	if err != nil {
		_ = db.Close()
		t.Fatalf("NewPersistentStore: %v", err)
	}
	return s, db
}

// reopen closes db and reopens the same directory with a fresh store.
func reopen(t *testing.T, dir string, db *sql.DB) (*Store, *sql.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	s, ndb := openPersistent(t, dir, DefaultLimits)
	t.Cleanup(func() { _ = ndb.Close() })
	return s, ndb
}

func TestPersist_ClientRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	now := time.Now().UTC()
	c := &Client{ID: "client-1", Name: "Test Client",
		RedirectURIs: []string{"https://example.com/cb", "http://127.0.0.1/cb"},
		CreatedAt:    now, LastUsed: now}
	if err := s.PutClient(c); err != nil {
		t.Fatal(err)
	}

	s2, _ := reopen(t, dir, db)
	got, ok := s2.GetClient("client-1")
	if !ok {
		t.Fatal("client lost across reopen")
	}
	if got.Name != c.Name || len(got.RedirectURIs) != 2 ||
		got.RedirectURIs[1] != c.RedirectURIs[1] ||
		!got.CreatedAt.Equal(now) || !got.LastUsed.Equal(now) {
		t.Fatalf("client mismatch: %+v", got)
	}
}

func TestPersist_TouchClientPersists(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	now := time.Now().UTC()
	if err := s.PutClient(&Client{ID: "c", CreatedAt: now, LastUsed: now}); err != nil {
		t.Fatal(err)
	}
	later := now.Add(10 * time.Minute)
	s.TouchClient("c", later)

	s2, _ := reopen(t, dir, db)
	got, ok := s2.GetClient("c")
	if !ok || !got.LastUsed.Equal(later) {
		t.Fatalf("LastUsed not persisted: %+v ok=%v", got, ok)
	}
}

func TestPersist_CodeTakenOnceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	code := &AuthCode{Hash: hashToken("raw-code"), ClientID: "c", RedirectURI: "https://example.com/cb",
		Subject: "alice", Scope: "mcp", CodeChallenge: "chal", ExpiresAt: time.Now().Add(time.Minute)}
	if err := s.PutCode(code); err != nil {
		t.Fatal(err)
	}

	s2, db2 := reopen(t, dir, db)
	got, ok := s2.TakeCode(code.Hash)
	if !ok || got.Subject != "alice" || got.CodeChallenge != "chal" || got.Scope != "mcp" {
		t.Fatalf("code not restored: %+v ok=%v", got, ok)
	}

	s3, _ := reopen(t, dir, db2)
	if _, ok := s3.TakeCode(code.Hash); ok {
		t.Fatal("code usable a second time after reopen")
	}
}

func TestPersist_UsedCodeReplayAfterReopen(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	s.MarkCodeUsed("code-h", "refresh-h", "fam-1", time.Now().Add(time.Hour))

	s2, db2 := reopen(t, dir, db)
	rh, fam, ok := s2.TakeUsedCode("code-h")
	if !ok || rh != "refresh-h" || fam != "fam-1" {
		t.Fatalf("got %q %q %v", rh, fam, ok)
	}

	s3, _ := reopen(t, dir, db2)
	if _, _, ok := s3.TakeUsedCode("code-h"); ok {
		t.Fatal("replay record survived being taken")
	}
}

func TestPersist_CodeExpiredAtLoad(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	if err := s.PutCode(&AuthCode{Hash: "old", ClientID: "c", ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	s.MarkCodeUsed("old-used", "r", "f", time.Now().Add(-time.Minute))

	s2, db2 := reopen(t, dir, db)
	if _, ok := s2.TakeCode("old"); ok {
		t.Fatal("expired code usable after load")
	}
	if _, _, ok := s2.TakeUsedCode("old-used"); ok {
		t.Fatal("expired replay record usable after load")
	}
	var n int
	for _, tbl := range []string{"oauth_codes", "oauth_used_codes"} {
		if err := db2.QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s rows after load: %d err=%v", tbl, n, err)
		}
	}
}

func TestPersist_LimitAtLoadKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	base := time.Now().Add(time.Hour)
	for i, h := range []string{"a", "b", "c"} {
		if err := s.PutCode(&AuthCode{Hash: h, ClientID: "c", ExpiresAt: base.Add(time.Duration(i) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}
	_ = db.Close()

	ndb, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ndb.Close()
	var logged []string
	s2, err := NewPersistentStore(ndb, Limits{Clients: 10, Codes: 2, DeviceCodes: 10, Tokens: 10},
		time.Hour, func(f string, a ...any) { logged = append(logged, f) })
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.TakeCode("a"); ok {
		t.Fatal("oldest code kept over limit")
	}
	if _, ok := s2.TakeCode("c"); !ok {
		t.Fatal("newest code dropped")
	}
	if len(logged) == 0 {
		t.Fatal("trimming not logged")
	}
}

func TestPersist_IdleClientSweptFromDisk(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	now := time.Now()
	if err := s.PutClient(&Client{ID: "idle", CreatedAt: now, LastUsed: now}); err != nil {
		t.Fatal(err)
	}
	s.Sweep(now.Add(time.Hour + clientIdleGrace + time.Minute))

	s2, _ := reopen(t, dir, db)
	if _, ok := s2.GetClient("idle"); ok {
		t.Fatal("swept client came back after reopen")
	}
}

func TestPersist_DeleteFailsClosed(t *testing.T) {
	dir := t.TempDir()
	var logged int
	db, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewPersistentStore(db, DefaultLimits, time.Hour,
		func(string, ...any) { logged++ })
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutCode(&AuthCode{Hash: "h", ClientID: "c", ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	s.MarkCodeUsed("u", "r", "f", time.Now().Add(time.Hour))
	_ = db.Close()

	if _, ok := s.TakeCode("h"); !ok {
		t.Fatal("first TakeCode should still return the code")
	}
	if _, ok := s.TakeCode("h"); ok {
		t.Fatal("code still in memory after failed delete")
	}
	if _, _, ok := s.TakeUsedCode("u"); !ok {
		t.Fatal("used code lost")
	}
	if _, _, ok := s.TakeUsedCode("u"); ok {
		t.Fatal("used code still in memory after failed delete")
	}
	if logged == 0 {
		t.Fatal("write failures not logged")
	}
	// Writes that return errors fail and leave memory unchanged.
	if err := s.PutCode(&AuthCode{Hash: "n", ClientID: "c"}); err == nil {
		t.Fatal("PutCode should fail on closed database")
	}
	if _, ok := s.TakeCode("n"); ok {
		t.Fatal("failed PutCode left the code in memory")
	}
}

func TestPersist_PlaintextNeverOnDisk(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	const raw = "RAW-CODE-SECRET-0123456789"
	if err := s.PutCode(&AuthCode{Hash: hashToken(raw), ClientID: "c",
		ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	s.MarkCodeUsed(hashToken(raw), hashToken("raw-refresh"), "fam", time.Now().Add(time.Hour))

	// Read the WAL before closing, as closing checkpoints it away.
	var all []byte
	for _, name := range []string{authdb.FileName, authdb.FileName + "-wal"} {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			all = append(all, b...)
		}
	}
	_ = db.Close()
	if b, err := os.ReadFile(filepath.Join(dir, authdb.FileName)); err == nil {
		all = append(all, b...)
	}

	if strings.Contains(string(all), raw) {
		t.Fatal("raw code found on disk")
	}
	if !strings.Contains(string(all), hashToken(raw)) {
		t.Fatal("hash not found on disk; test is not looking at the data")
	}
}
