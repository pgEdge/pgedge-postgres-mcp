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
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/authdb"
)

// A refresh token whose deletion failed leaves its row beside a rotation
// record. It must not load as live, and presenting it must be seen as
// reuse of a rotated token, which revokes the family.
func TestPersist_RotatedTokenNotLiveAfterReopen(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Hour)
	putFamily(t, s, "fam-1", "c", exp)
	putFamily(t, s, "fam-2", "c", exp)
	// A live descendant of fam-1, as a later rotation would have issued.
	if err := s.PutToken(&Token{Hash: "a-desc", ClientID: "c", Subject: "alice",
		ExpiresAt: exp, Family: "fam-1"}); err != nil {
		t.Fatal(err)
	}
	// Simulate the failed delete: only the rotation record is written.
	if _, err := db.Exec(`INSERT INTO oauth_rotated (hash, family, until) VALUES (?, ?, ?)`,
		"r-fam-1", "fam-1", exp.UnixNano()); err != nil {
		t.Fatal(err)
	}

	s2, db2 := reopen(t, dir, db)
	if _, ok := s2.TakeToken("r-fam-1"); ok {
		t.Fatal("rotated refresh token loaded as live")
	}
	fam, ok := s2.RotatedRefreshFamily("r-fam-1")
	if !ok || fam != "fam-1" {
		t.Fatalf("rotation record lost: %q %v", fam, ok)
	}
	// The access token it issued went with it, as the cascade at runtime.
	if _, ok := s2.GetToken("a-fam-1"); ok {
		t.Fatal("access token of a rotated refresh token loaded as live")
	}
	// Acting on the reuse revokes the family's live descendant.
	if revoked := s2.DeleteFamily(fam); len(revoked) != 1 || revoked[0] != "a-desc" {
		t.Fatalf("family not revoked: %v", revoked)
	}
	if _, ok := s2.GetToken("r-fam-2"); !ok {
		t.Fatal("unrelated family lost")
	}
	if n := countRows(t, db2, "oauth_rotated"); n != 1 {
		t.Fatalf("rotation rows = %d, want 1", n)
	}
	// The stale row is gone from disk, not just from memory.
	var n int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM oauth_tokens WHERE hash = 'r-fam-1'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("stale token row on disk: n=%d err=%v", n, err)
	}
}

func TestPersist_UsedCodeNotLiveAfterReopen(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	exp := time.Now().Add(time.Hour)
	if err := s.PutCode(&AuthCode{Hash: "code-h", ClientID: "c", Subject: "alice", ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutCode(&AuthCode{Hash: "other-h", ClientID: "c", Subject: "alice", ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	// Simulate the failed delete after redemption.
	if _, err := db.Exec(`INSERT INTO oauth_used_codes (code_hash, refresh_hash, family, until)
        VALUES ('code-h', 'refresh-h', 'fam-1', ?)`, exp.UnixNano()); err != nil {
		t.Fatal(err)
	}

	s2, db2 := reopen(t, dir, db)
	if _, ok := s2.TakeCode("code-h"); ok {
		t.Fatal("redeemed code loaded as live")
	}
	rh, fam, ok := s2.TakeUsedCode("code-h")
	if !ok || rh != "refresh-h" || fam != "fam-1" {
		t.Fatalf("replay record lost: %q %q %v", rh, fam, ok)
	}
	if _, ok := s2.TakeCode("other-h"); !ok {
		t.Fatal("unrelated code lost")
	}
	var n int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM oauth_codes WHERE hash = 'code-h'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("stale code row on disk: n=%d err=%v", n, err)
	}
}

// Evicting the soonest-expiring rotation record to stay within the
// limit must reach the disk, or the table outgrows the limit.
func TestPersist_RotatedEvictionPersists(t *testing.T) {
	dir := t.TempDir()
	lim := Limits{Clients: 10, Codes: 10, DeviceCodes: 10, Tokens: 2}
	s, db := openPersistent(t, dir, lim)
	base := time.Now().Add(time.Hour)
	s.MarkRefreshRotated("r1", "f", base.Add(1*time.Minute))
	s.MarkRefreshRotated("r2", "f", base.Add(2*time.Minute))
	s.MarkRefreshRotated("r3", "f", base.Add(3*time.Minute))
	if n := countRows(t, db, "oauth_rotated"); n != 2 {
		t.Fatalf("rotated rows = %d, want 2", n)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM oauth_rotated WHERE hash = 'r1'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("evicted record still on disk: n=%d err=%v", n, err)
	}
}

func TestPersist_TrimmedClientsClearedAfterLoad(t *testing.T) {
	dir := t.TempDir()
	s, db := openPersistent(t, dir, DefaultLimits)
	now := time.Now()
	for _, id := range []string{"a", "b"} {
		_ = s.PutClient(&Client{ID: id, CreatedAt: now, LastUsed: now})
	}
	_ = db.Close()
	ndb, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ndb.Close()
	s2, err := NewPersistentStore(ndb, Limits{Clients: 1, Codes: 10, DeviceCodes: 10, Tokens: 10}, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.trimmedClients) != 0 {
		t.Fatalf("trimmedClients retained after load: %v", s2.trimmedClients)
	}
}
