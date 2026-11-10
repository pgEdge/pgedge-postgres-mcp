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

func newTestDevice(hash, userCode string, expires time.Time) *DeviceCode {
	return &DeviceCode{
		DeviceHash: hash, UserCode: userCode, ClientID: "client-a",
		Scope: "mcp", ExpiresAt: expires, Interval: 5 * time.Second,
	}
}

func TestPersist_DeviceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(10 * time.Minute).UTC()
	if err := s.PutDeviceCode(newTestDevice("dh1", "ABCD-EFGH", exp)); err != nil {
		t.Fatal(err)
	}

	s2, _ := reopen(t, dir, db)
	d, ok := s2.GetDeviceByHash("dh1")
	if !ok {
		t.Fatal("device not found by hash after reopen")
	}
	if d.UserCode != "ABCD-EFGH" || d.ClientID != "client-a" || d.Scope != "mcp" ||
		!d.ExpiresAt.Equal(exp) || d.Interval != 5*time.Second {
		t.Fatalf("device changed across reopen: %+v", d)
	}
	if _, ok := s2.GetDeviceByUserCode("ABCD-EFGH"); !ok {
		t.Fatal("device not found by user code after reopen")
	}
}

func TestPersist_DeviceApprovalSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	_ = s.PutDeviceCode(newTestDevice("dh1", "ABCD-EFGH", time.Now().Add(time.Minute)))
	if !s.ApproveDevice("dh1", "alice") {
		t.Fatal("approve failed")
	}

	s2, _ := reopen(t, dir, db)
	d, ok := s2.TakeDeviceIfApproved("dh1")
	if !ok || d.Subject != "alice" || !d.Approved {
		t.Fatalf("approved device lost across reopen: %+v ok=%v", d, ok)
	}
}

func TestPersist_DeviceDenialSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	_ = s.PutDeviceCode(newTestDevice("dh1", "ABCD-EFGH", time.Now().Add(time.Minute)))
	if !s.DenyDevice("dh1") {
		t.Fatal("deny failed")
	}

	s2, _ := reopen(t, dir, db)
	d, ok := s2.GetDeviceByHash("dh1")
	if !ok || !d.Denied {
		t.Fatalf("denial lost across reopen: %+v ok=%v", d, ok)
	}
	if _, ok := s2.TakeDeviceIfApproved("dh1"); ok {
		t.Fatal("denied device was taken")
	}
}

func TestPersist_DeviceTakenOnce(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	_ = s.PutDeviceCode(newTestDevice("dh1", "ABCD-EFGH", time.Now().Add(time.Minute)))
	s.ApproveDevice("dh1", "alice")
	if _, ok := s.TakeDeviceIfApproved("dh1"); !ok {
		t.Fatal("first take failed")
	}

	s2, _ := reopen(t, dir, db)
	if _, ok := s2.TakeDeviceIfApproved("dh1"); ok {
		t.Fatal("device taken twice across reopen")
	}
	if _, ok := s2.GetDeviceByUserCode("ABCD-EFGH"); ok {
		t.Fatal("user code survived the take")
	}
}

func TestPersist_DeviceExpiredAtLoad(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	_ = s.PutDeviceCode(newTestDevice("old", "OLD1-OLD1", time.Now().Add(-time.Minute)))
	_ = s.PutDeviceCode(newTestDevice("new", "NEW1-NEW1", time.Now().Add(time.Minute)))

	s2, _ := reopen(t, dir, db)
	if _, ok := s2.GetDeviceByHash("old"); ok {
		t.Fatal("expired device usable after load")
	}
	if _, ok := s2.GetDeviceByUserCode("OLD1-OLD1"); ok {
		t.Fatal("expired user code usable after load")
	}
	if _, ok := s2.GetDeviceByHash("new"); !ok {
		t.Fatal("live device missing after load")
	}
}

func TestPersist_DeviceUpdateAndPollPersist(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Minute)
	_ = s.PutDeviceCode(newTestDevice("dh1", "ABCD-EFGH", exp))
	upd := newTestDevice("dh1", "WXYZ-WXYZ", exp)
	upd.Approved, upd.Subject = true, "alice"
	if err := s.UpdateDevice(upd); err != nil {
		t.Fatal(err)
	}
	polled := time.Now().Add(time.Hour).UTC()
	if _, tooSoon, ok := s.TouchDevicePoll("dh1", polled); !ok || tooSoon {
		t.Fatal("poll rejected")
	}

	s2, _ := reopen(t, dir, db)
	d, ok := s2.GetDeviceByHash("dh1")
	if !ok || d.UserCode != "WXYZ-WXYZ" || d.Subject != "alice" || !d.LastPolled.Equal(polled) {
		t.Fatalf("update or poll lost: %+v ok=%v", d, ok)
	}
	if _, ok := s2.GetDeviceByUserCode("ABCD-EFGH"); ok {
		t.Fatal("stale user code survived")
	}
}

func TestPersist_DeviceSweepAndDeleteFromDisk(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	_ = s.PutDeviceCode(newTestDevice("old", "OLD1-OLD1", time.Now().Add(-time.Minute)))
	_ = s.PutDeviceCode(newTestDevice("gone", "GONE-GONE", time.Now().Add(time.Hour)))
	s.Sweep(time.Now())
	s.DeleteDevice("gone")

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM oauth_devices`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("device rows left on disk: n=%d err=%v", n, err)
	}
}

func TestPersist_DeviceWriteFailures(t *testing.T) {
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
	_ = s.PutDeviceCode(newTestDevice("a", "AAAA-AAAA", exp))
	_ = s.PutDeviceCode(newTestDevice("b", "BBBB-BBBB", exp))
	_ = s.PutDeviceCode(newTestDevice("c", "CCCC-CCCC", exp))
	_ = db.Close()

	if err := s.PutDeviceCode(newTestDevice("n", "NNNN-NNNN", exp)); err == nil {
		t.Fatal("PutDeviceCode should fail on closed database")
	}
	if _, ok := s.GetDeviceByHash("n"); ok {
		t.Fatal("failed PutDeviceCode left the device in memory")
	}
	if err := s.UpdateDevice(newTestDevice("a", "ZZZZ-ZZZZ", exp)); err == nil {
		t.Fatal("UpdateDevice should fail on closed database")
	}
	if d, _ := s.GetDeviceByHash("a"); d.UserCode != "AAAA-AAAA" {
		t.Fatal("failed UpdateDevice changed memory")
	}
	if !s.ApproveDevice("a", "alice") {
		t.Fatal("approve must apply in memory despite write failure")
	}
	if !s.DenyDevice("b") {
		t.Fatal("deny must apply in memory despite write failure")
	}
	if _, ok := s.TakeDeviceIfApproved("a"); !ok {
		t.Fatal("take should still succeed")
	}
	if _, ok := s.GetDeviceByHash("a"); ok {
		t.Fatal("taken device still in memory after failed delete")
	}
	s.DeleteDevice("b")
	if _, ok := s.GetDeviceByUserCode("BBBB-BBBB"); ok {
		t.Fatal("deleted device still in memory after failed delete")
	}
	before := logged
	if _, _, ok := s.TouchDevicePoll("c", time.Now().Add(time.Hour)); !ok {
		t.Fatal("poll failed")
	}
	if logged == before {
		t.Fatal("poll write failure not logged")
	}
}

func TestPersist_DeviceUserCodeCollision(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Hour)
	_ = s.PutDeviceCode(newTestDevice("a", "AAAA-AAAA", exp))
	_ = s.PutDeviceCode(newTestDevice("b", "BBBB-BBBB", exp))

	if err := s.PutDeviceCode(newTestDevice("c", "AAAA-AAAA", exp)); !errors.Is(err, ErrUserCodeInUse) {
		t.Fatalf("PutDeviceCode collision: got %v, want ErrUserCodeInUse", err)
	}
	if err := s.UpdateDevice(newTestDevice("b", "AAAA-AAAA", exp)); !errors.Is(err, ErrUserCodeInUse) {
		t.Fatalf("UpdateDevice collision: got %v, want ErrUserCodeInUse", err)
	}
	// Re-putting a device under its own code is not a collision.
	if err := s.PutDeviceCode(newTestDevice("a", "AAAA-AAAA", exp)); err != nil {
		t.Fatalf("same-hash put: %v", err)
	}

	check := func(st *Store) {
		t.Helper()
		if d, ok := st.GetDeviceByUserCode("AAAA-AAAA"); !ok || d.DeviceHash != "a" {
			t.Fatalf("AAAA-AAAA no longer maps to a: %+v", d)
		}
		if d, ok := st.GetDeviceByHash("b"); !ok || d.UserCode != "BBBB-BBBB" {
			t.Fatalf("b changed: %+v", d)
		}
		if _, ok := st.GetDeviceByHash("c"); ok {
			t.Fatal("c stored")
		}
	}
	check(s)
	s2, _ := reopen(t, dir, db)
	check(s2)
}
