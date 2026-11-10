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

func TestPersist_TokenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Hour).UTC()
	refresh, access := putFamily(t, s, "fam-1", "client-1", exp)

	s2, _ := reopen(t, dir, db)
	r := mustGetToken(t, s2, refresh, "refresh token lost across reopen")
	requireEq(t, "refresh IsRefresh", r.IsRefresh, true)
	requireEq(t, "refresh family", r.Family, "fam-1")
	requireEq(t, "refresh subject", r.Subject, "alice")
	requireEq(t, "refresh client", r.ClientID, "client-1")
	requireEq(t, "refresh scope", r.Scope, "mcp")
	requireEq(t, "refresh expiry matches", r.ExpiresAt.Equal(exp), true)
	requireRevoked(t, r.Issued, access)
	a := mustGetToken(t, s2, access, "access token lost across reopen")
	requireEq(t, "access IsRefresh", a.IsRefresh, false)
	requireEq(t, "access refresh hash", a.RefreshHash, refresh)
	requireEq(t, "access family", a.Family, "fam-1")
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
	requireTakeToken(t, s, "r-fam-1", true, "take")
	s.MarkRefreshRotated("r-fam-1", "fam-1", exp)
	// A descendant of the same family, still live. It has its own hash:
	// a hash on the rotation list is never live, and load drops it.
	mustPutToken(t, s, &Token{Hash: "a-fam-1", ClientID: "c", Subject: "alice",
		ExpiresAt: exp, Family: "fam-1"})

	s2, db2 := reopen(t, dir, db)
	requireRotated(t, s2, "r-fam-1", "fam-1")
	requireRevoked(t, s2.DeleteFamily("fam-1"), "a-fam-1")
	requireToken(t, s2, "a-fam-2", true, "DeleteFamily touched another family")

	s3, _ := reopen(t, dir, db2)
	requireToken(t, s3, "r-fam-1", false, "family token r-fam-1 remained after reopen")
	requireToken(t, s3, "a-fam-1", false, "family token a-fam-1 remained after reopen")
	requireToken(t, s3, "r-fam-2", true, "other family lost")
}

func TestPersist_DeleteBySubjectPersists(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Hour)
	putFamily(t, s, "f", "c", exp)
	mustPutToken(t, s, &Token{Hash: "bob-t", Subject: "bob", ExpiresAt: exp})
	_ = s.PutCode(&AuthCode{Hash: "alice-code", Subject: "alice", ExpiresAt: exp})
	_ = s.PutCode(&AuthCode{Hash: "bob-code", Subject: "bob", ExpiresAt: exp})
	d := newTestDevice("alice-dev", "AAAA-AAAA", exp)
	d.Subject = "alice"
	_ = s.PutDeviceCode(d)
	_ = s.PutDeviceCode(newTestDevice("anon-dev", "BBBB-BBBB", exp))

	revoked, err := s.DeleteBySubject("alice")
	if err != nil {
		t.Fatal(err)
	}
	requireEq(t, "revoked count", len(revoked), 1)
	s2, db2 := reopen(t, dir, db)
	requireToken(t, s2, "r-f", false, "alice token survived")
	requireToken(t, s2, "bob-t", true, "bob token lost")
	requireTakeCode(t, s2, "alice-code", false, "alice code survived")
	requireTakeCode(t, s2, "bob-code", true, "bob code lost")
	requireDevice(t, s2, "alice-dev", false, "alice device survived")
	requireDevice(t, s2, "anon-dev", true, "unrelated device lost")
	requireRows(t, db2, "oauth_tokens", "token", 1)
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
		mustPutToken(t, s, &Token{Hash: h, ClientID: "c", ExpiresAt: exp.Add(time.Duration(i) * time.Minute)})
		s.MarkRefreshRotated("rot-"+h, "f", exp.Add(time.Duration(i)*time.Minute))
	}
	_ = db.Close()

	ndb := mustOpenDB(t, dir)
	defer ndb.Close()
	s2 := mustPersistentStore(t, ndb, Limits{Clients: 10, Codes: 10, DeviceCodes: 10, Tokens: 3})
	requireEq(t, "tokens in memory", len(s2.tokens), 3)
	requireRows(t, ndb, "oauth_tokens", "token", 3)
	requireToken(t, s2, "t1", false, "soonest-expiring token kept over limit")
	requireToken(t, s2, "t5", true, "latest-expiring token dropped")
	requireEq(t, "rotated in memory", len(s2.rotated), 3)
	requireRows(t, ndb, "oauth_rotated", "rotated", 3)
}

// A client with a live token must outlast one without when the client
// table is trimmed at load, and a token never outlives a client that
// had to go anyway.
func TestPersist_ClientTrimKeepsLiveTokenClients(t *testing.T) {
	dir := t.TempDir()
	now := seedBusyClient(t, dir)

	one := Limits{Clients: 1, Codes: 10, DeviceCodes: 10, Tokens: 10}
	ndb := mustOpenDB(t, dir)
	s2 := mustPersistentStore(t, ndb, one)
	requireClient(t, s2, "busy", true, "client with a live token was trimmed")
	requireToken(t, s2, "busy-t", true, "token lost")
	_ = ndb.Close()

	// With room for only one of two token holders, the loser's token
	// goes with its client.
	ndb = mustOpenDB(t, dir)
	defer ndb.Close()
	mustExec(t, ndb, `INSERT INTO oauth_clients (id, name, redirect_uris, created_at, last_used)
        VALUES ('busy2', '', '[]', 0, ?)`, now.UnixNano())
	mustExec(t, ndb, `INSERT INTO oauth_tokens (hash, client_id, expires_at)
        VALUES ('busy2-t', 'busy2', ?)`, now.Add(time.Hour).UnixNano())
	s3 := mustPersistentStore(t, ndb, one)
	requireClientTokenAgree(t, s3, "busy")
	requireClientTokenAgree(t, s3, "busy2")
	if len(s3.clients) != 1 || len(s3.tokens) != 1 {
		t.Fatalf("clients=%d tokens=%d, want 1 and 1", len(s3.clients), len(s3.tokens))
	}
	requireRows(t, ndb, "oauth_tokens", "token", 1)
}

// seedBusyClient stores three clients in a fresh auth.db in dir, of which
// "busy" is the least recently used but holds a live token, then closes
// the database. It returns the time it used as the present.
func seedBusyClient(t *testing.T, dir string) time.Time {
	t.Helper()
	s, db := openPersistent(t, dir, DefaultLimits)
	now := time.Now()
	for id, used := range map[string]time.Time{
		"busy": now.Add(-3 * time.Minute), "idle1": now.Add(-time.Minute), "idle2": now,
	} {
		mustPutClient(t, s, &Client{ID: id, CreatedAt: now, LastUsed: used})
	}
	mustPutToken(t, s, &Token{Hash: "busy-t", ClientID: "busy", ExpiresAt: now.Add(time.Hour)})
	_ = db.Close()
	return now
}

// requireClientTokenAgree fails unless client id and its token id+"-t" are
// either both present in s or both absent.
func requireClientTokenAgree(t *testing.T, s *Store, id string) {
	t.Helper()
	_, hasClient := s.GetClient(id)
	_, hasToken := s.GetToken(id + "-t")
	if hasClient != hasToken {
		t.Fatalf("%s: client present=%v but token present=%v", id, hasClient, hasToken)
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
	var logged int
	s, db := openLoggingStore(t, &logged)
	exp := time.Now().Add(time.Hour)
	for _, f := range []string{"f1", "f2", "f3", "f4"} {
		putFamily(t, s, f, "c", exp)
	}
	_ = db.Close()

	requireFails(t, s.PutToken(&Token{Hash: "n", ExpiresAt: exp}),
		"PutToken should fail on closed database")
	requireToken(t, s, "n", false, "failed PutToken left the token in memory")
	s.MarkRefreshRotated("rot", "f1", exp)
	requireRotatedIs(t, s, "rot", true, "rotation record must apply in memory despite write failure")
	if logged == 0 {
		t.Fatal("write failure not logged")
	}
	s.DeleteToken("a-f1")
	s.TakeToken("r-f2")
	s.DeleteFamily("f3")
	_, _ = s.DeleteBySubject("alice")
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
