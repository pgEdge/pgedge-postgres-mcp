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
	"errors"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/authdb"
)

// putFamily stores a refresh token and the access token issued from it,
// both in family fam, for subject alice.
func putFamily(t *testing.T, s *Store, fam, client string, exp time.Time) (refresh, access string) {
	t.Helper()
	refresh, access = "r-"+fam, "a-"+fam
	if err := s.PutToken(&Token{Hash: refresh, ClientID: client, Subject: "alice",
		Scope: "mcp", ExpiresAt: exp, IsRefresh: true, Issued: []string{access}, Family: fam}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutToken(&Token{Hash: access, ClientID: client, Subject: "alice",
		Scope: "mcp", ExpiresAt: exp, RefreshHash: refresh, Family: fam}); err != nil {
		t.Fatal(err)
	}
	return refresh, access
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPersist_TokenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Hour).UTC()
	refresh, access := putFamily(t, s, "fam-1", "client-1", exp)

	s2, _ := reopen(t, dir, db)
	r, ok := s2.GetToken(refresh)
	if !ok || !r.IsRefresh || r.Family != "fam-1" || r.Subject != "alice" ||
		r.ClientID != "client-1" || r.Scope != "mcp" || !r.ExpiresAt.Equal(exp) ||
		len(r.Issued) != 1 || r.Issued[0] != access {
		t.Fatalf("refresh token mismatch: %+v ok=%v", r, ok)
	}
	a, ok := s2.GetToken(access)
	if !ok || a.IsRefresh || a.RefreshHash != refresh || a.Family != "fam-1" {
		t.Fatalf("access token mismatch: %+v ok=%v", a, ok)
	}
}

func TestPersist_DeleteTokenCascadePersists(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Hour)
	refresh, access := putFamily(t, s, "f", "c", exp)
	// Deleting the access token updates the parent's Issued list on disk.
	s.DeleteToken(access)
	s2, db2 := reopen(t, dir, db)
	if r, ok := s2.GetToken(refresh); !ok || len(r.Issued) != 0 {
		t.Fatalf("parent Issued not persisted: %+v ok=%v", r, ok)
	}
	if _, ok := s2.GetToken(access); ok {
		t.Fatal("access token survived deletion")
	}
	// Deleting the refresh token cascades to its children on disk.
	putFamily(t, s2, "g", "c", exp)
	s2.DeleteToken("r-g")
	if n := countRows(t, db2, "oauth_tokens"); n != 1 {
		t.Fatalf("rows after cascade = %d, want 1", n)
	}
	if _, ok := s2.TakeToken(refresh); !ok {
		t.Fatal("take failed")
	}
	if n := countRows(t, db2, "oauth_tokens"); n != 0 {
		t.Fatalf("rows after take = %d, want 0", n)
	}
}

func TestPersist_RotatedRefreshReplayAfterReopen(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Hour)
	putFamily(t, s, "fam-1", "c", exp)
	putFamily(t, s, "fam-2", "c", exp)
	if _, ok := s.TakeToken("r-fam-1"); !ok {
		t.Fatal("take")
	}
	s.MarkRefreshRotated("r-fam-1", "fam-1", exp)
	// A descendant of the same family, still live. It has its own hash:
	// a hash on the rotation list is never live, and load drops it.
	if err := s.PutToken(&Token{Hash: "a-fam-1", ClientID: "c", Subject: "alice",
		ExpiresAt: exp, Family: "fam-1"}); err != nil {
		t.Fatal(err)
	}

	s2, db2 := reopen(t, dir, db)
	fam, ok := s2.RotatedRefreshFamily("r-fam-1")
	if !ok || fam != "fam-1" {
		t.Fatalf("rotated record lost: %q %v", fam, ok)
	}
	revoked := s2.DeleteFamily(fam)
	if len(revoked) != 1 || revoked[0] != "a-fam-1" {
		t.Fatalf("revoked = %v", revoked)
	}
	if _, ok := s2.GetToken("a-fam-2"); !ok {
		t.Fatal("DeleteFamily touched another family")
	}

	s3, _ := reopen(t, dir, db2)
	for _, h := range []string{"r-fam-1", "a-fam-1"} {
		if _, ok := s3.GetToken(h); ok {
			t.Fatalf("family token %s remained after reopen", h)
		}
	}
	if _, ok := s3.GetToken("r-fam-2"); !ok {
		t.Fatal("other family lost")
	}
}

func TestPersist_DeleteBySubjectPersists(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Hour)
	putFamily(t, s, "f", "c", exp)
	if err := s.PutToken(&Token{Hash: "bob-t", Subject: "bob", ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	_ = s.PutCode(&AuthCode{Hash: "alice-code", Subject: "alice", ExpiresAt: exp})
	_ = s.PutCode(&AuthCode{Hash: "bob-code", Subject: "bob", ExpiresAt: exp})
	d := newTestDevice("alice-dev", "AAAA-AAAA", exp)
	d.Subject = "alice"
	_ = s.PutDeviceCode(d)
	_ = s.PutDeviceCode(newTestDevice("anon-dev", "BBBB-BBBB", exp))

	if revoked := s.DeleteBySubject("alice"); len(revoked) != 1 {
		t.Fatalf("revoked = %v", revoked)
	}
	s2, db2 := reopen(t, dir, db)
	if _, ok := s2.GetToken("r-f"); ok {
		t.Fatal("alice token survived")
	}
	if _, ok := s2.GetToken("bob-t"); !ok {
		t.Fatal("bob token lost")
	}
	if _, ok := s2.TakeCode("alice-code"); ok {
		t.Fatal("alice code survived")
	}
	if _, ok := s2.TakeCode("bob-code"); !ok {
		t.Fatal("bob code lost")
	}
	if _, ok := s2.GetDeviceByHash("alice-dev"); ok {
		t.Fatal("alice device survived")
	}
	if _, ok := s2.GetDeviceByHash("anon-dev"); !ok {
		t.Fatal("unrelated device lost")
	}
	if n := countRows(t, db2, "oauth_tokens"); n != 1 {
		t.Fatalf("token rows = %d, want 1", n)
	}
}

func TestPersist_SweepPersists(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	now := time.Now()
	putFamily(t, s, "old", "c", now.Add(time.Minute))
	putFamily(t, s, "new", "c", now.Add(10*time.Hour))
	s.MarkRefreshRotated("rot-old", "old", now.Add(time.Minute))
	s.MarkRefreshRotated("rot-new", "new", now.Add(10*time.Hour))

	revoked := s.Sweep(now.Add(time.Hour))
	if len(revoked) != 1 || revoked[0] != "a-old" {
		t.Fatalf("revoked = %v", revoked)
	}
	s2, db2 := reopen(t, dir, db)
	if _, ok := s2.GetToken("r-old"); ok {
		t.Fatal("expired token survived sweep")
	}
	if _, ok := s2.GetToken("r-new"); !ok {
		t.Fatal("live token lost")
	}
	if _, ok := s2.RotatedRefreshFamily("rot-old"); ok {
		t.Fatal("expired rotated entry survived sweep")
	}
	if _, ok := s2.RotatedRefreshFamily("rot-new"); !ok {
		t.Fatal("live rotated entry lost")
	}
	if n := countRows(t, db2, "oauth_tokens"); n != 2 {
		t.Fatalf("token rows = %d, want 2", n)
	}
}

func TestPersist_TokenExpiredAtLoad(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	now := time.Now()
	putFamily(t, s, "dead", "c", now.Add(-time.Minute))
	putFamily(t, s, "live", "c", now.Add(time.Hour))
	s.MarkRefreshRotated("rot-dead", "dead", now.Add(-time.Minute))

	s2, db2 := reopen(t, dir, db)
	if _, ok := s2.GetToken("a-dead"); ok {
		t.Fatal("expired access token loaded")
	}
	if _, ok := s2.GetToken("r-dead"); ok {
		t.Fatal("expired refresh token loaded")
	}
	if _, ok := s2.RotatedRefreshFamily("rot-dead"); ok {
		t.Fatal("expired rotated entry loaded")
	}
	if _, ok := s2.GetToken("a-live"); !ok {
		t.Fatal("live token lost")
	}
	if n := countRows(t, db2, "oauth_tokens"); n != 2 {
		t.Fatalf("token rows = %d, want 2", n)
	}
	if n := countRows(t, db2, "oauth_rotated"); n != 0 {
		t.Fatalf("rotated rows = %d, want 0", n)
	}
}

func TestPersist_LimitsOnLoad(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Hour)
	for i, h := range []string{"t1", "t2", "t3", "t4", "t5"} {
		if err := s.PutToken(&Token{Hash: h, ClientID: "c", ExpiresAt: exp.Add(time.Duration(i) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
		s.MarkRefreshRotated("rot-"+h, "f", exp.Add(time.Duration(i)*time.Minute))
	}
	_ = db.Close()

	ndb, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ndb.Close()
	s2, err := NewPersistentStore(ndb, Limits{Clients: 10, Codes: 10, DeviceCodes: 10, Tokens: 3},
		time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(s2.tokens); n != 3 {
		t.Fatalf("tokens in memory = %d, want 3", n)
	}
	if n := countRows(t, ndb, "oauth_tokens"); n != 3 {
		t.Fatalf("token rows = %d, want 3", n)
	}
	if _, ok := s2.GetToken("t1"); ok {
		t.Fatal("soonest-expiring token kept over limit")
	}
	if _, ok := s2.GetToken("t5"); !ok {
		t.Fatal("latest-expiring token dropped")
	}
	if n := len(s2.rotated); n != 3 {
		t.Fatalf("rotated in memory = %d, want 3", n)
	}
	if n := countRows(t, ndb, "oauth_rotated"); n != 3 {
		t.Fatalf("rotated rows = %d, want 3", n)
	}
}

// A client with a live token must outlast one without when the client
// table is trimmed at load, and a token never outlives a client that
// had to go anyway.
func TestPersist_ClientTrimKeepsLiveTokenClients(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	now := time.Now()
	// "busy" is the least recently used but holds a live token.
	for id, used := range map[string]time.Time{
		"busy": now.Add(-3 * time.Minute), "idle1": now.Add(-time.Minute), "idle2": now,
	} {
		if err := s.PutClient(&Client{ID: id, CreatedAt: now, LastUsed: used}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PutToken(&Token{Hash: "busy-t", ClientID: "busy", ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	one := Limits{Clients: 1, Codes: 10, DeviceCodes: 10, Tokens: 10}
	ndb, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := NewPersistentStore(ndb, one, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.GetClient("busy"); !ok {
		t.Fatal("client with a live token was trimmed")
	}
	if _, ok := s2.GetToken("busy-t"); !ok {
		t.Fatal("token lost")
	}
	_ = ndb.Close()

	// With room for only one of two token holders, the loser's token
	// goes with its client.
	ndb, _, err = authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ndb.Close()
	if _, err := ndb.Exec(`INSERT INTO oauth_clients (id, name, redirect_uris, created_at, last_used)
        VALUES ('busy2', '', '[]', 0, ?)`, now.UnixNano()); err != nil {
		t.Fatal(err)
	}
	if _, err := ndb.Exec(`INSERT INTO oauth_tokens (hash, client_id, expires_at)
        VALUES ('busy2-t', 'busy2', ?)`, now.Add(time.Hour).UnixNano()); err != nil {
		t.Fatal(err)
	}
	s3, err := NewPersistentStore(ndb, one, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"busy", "busy2"} {
		_, hasClient := s3.GetClient(id)
		_, hasToken := s3.GetToken(id + "-t")
		if hasClient != hasToken {
			t.Fatalf("%s: client present=%v but token present=%v", id, hasClient, hasToken)
		}
	}
	if len(s3.clients) != 1 || len(s3.tokens) != 1 {
		t.Fatalf("clients=%d tokens=%d, want 1 and 1", len(s3.clients), len(s3.tokens))
	}
	if n := countRows(t, ndb, "oauth_tokens"); n != 1 {
		t.Fatalf("token rows = %d, want 1", n)
	}
}

func TestPersist_IdleClientExpiredAtLoad(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	old := time.Now().Add(-48 * time.Hour)
	_ = s.PutClient(&Client{ID: "idle", CreatedAt: old, LastUsed: old})
	_ = s.PutClient(&Client{ID: "held", CreatedAt: old, LastUsed: old})
	_ = s.PutToken(&Token{Hash: "held-t", ClientID: "held", ExpiresAt: time.Now().Add(time.Hour)})
	s2, db2 := reopen(t, dir, db)
	if _, ok := s2.GetClient("idle"); ok {
		t.Fatal("idle client loaded")
	}
	if _, ok := s2.GetClient("held"); !ok {
		t.Fatal("idle client holding a live token was dropped")
	}
	if n := countRows(t, db2, "oauth_clients"); n != 1 {
		t.Fatalf("client rows = %d, want 1", n)
	}
}

func TestPersist_NoRevocationHookOnLoad(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	putFamily(t, s, "dead", "c", time.Now().Add(-time.Minute))
	_ = db.Close()

	ndb, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ndb.Close()
	// The hook can only be installed after NewPersistentStore returns,
	// so build the store by hand with it set before load runs.
	called := false
	p := newSQLitePersister(ndb)
	s2 := newStore(DefaultLimits, time.Hour, p, nil)
	s2.SetRevocationHook(func([]string) { called = true })
	s2.mu.Lock()
	err = p.load(s2, time.Now())
	s2.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("revocation hook called during load")
	}
	if len(s2.tokens) != 0 {
		t.Fatal("expired tokens loaded")
	}
}

func TestPersist_TokenWriteFailures(t *testing.T) {
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
	exp := time.Now().Add(time.Hour)
	for _, f := range []string{"f1", "f2", "f3", "f4"} {
		putFamily(t, s, f, "c", exp)
	}
	_ = db.Close()

	if err := s.PutToken(&Token{Hash: "n", ExpiresAt: exp}); err == nil {
		t.Fatal("PutToken should fail on closed database")
	}
	if _, ok := s.GetToken("n"); ok {
		t.Fatal("failed PutToken left the token in memory")
	}
	s.MarkRefreshRotated("rot", "f1", exp)
	if _, ok := s.RotatedRefreshFamily("rot"); !ok {
		t.Fatal("rotation record must apply in memory despite write failure")
	}
	if logged == 0 {
		t.Fatal("write failure not logged")
	}
	s.DeleteToken("a-f1")
	s.TakeToken("r-f2")
	s.DeleteFamily("f3")
	s.DeleteBySubject("alice")
	s.Sweep(exp.Add(time.Hour))
	if len(s.tokens) != 0 {
		t.Fatalf("%d tokens left in memory after failed deletes", len(s.tokens))
	}
}

// A failure part way through a multi-row change must leave nothing
// half-written on disk, and a panic must release the transaction.
func TestPersist_TxRollsBackOnFailure(t *testing.T) {
	dir := t.TempDir()
	db, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p := newSQLitePersister(db)
	err = p.tx(func(tp persister) error {
		if err := tp.putToken(&Token{Hash: "x"}); err != nil {
			return err
		}
		return errors.New("boom")
	})
	if err == nil || countRows(t, db, "oauth_tokens") != 0 {
		t.Fatalf("tx not rolled back: err=%v", err)
	}
	func() {
		defer func() { _ = recover() }()
		_ = p.tx(func(tp persister) error {
			_ = tp.putToken(&Token{Hash: "y"})
			panic("boom")
		})
	}()
	if n := countRows(t, db, "oauth_tokens"); n != 0 {
		t.Fatalf("panic left %d rows committed", n)
	}
	if err := p.putToken(&Token{Hash: "z"}); err != nil {
		t.Fatal(err)
	}
}
