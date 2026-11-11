/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/authdb"
)

const testPassword = "correct-horse-battery"

// sessionBackend builds a user store wired to one SessionStore
// implementation, with alice as a user.
type sessionBackend struct {
	name string
	make func(t *testing.T) (*UserStore, SessionStore)
}

func sessionBackends() []sessionBackend {
	return []sessionBackend{
		{"memory", func(t *testing.T) (*UserStore, SessionStore) {
			t.Helper()
			ss := NewMemorySessionStore()
			us := InitializeUserStore()
			us.SetSessionStore(ss)
			mustAdd(t, us, "alice")
			return us, ss
		}},
		{"sqlite", func(t *testing.T) (*UserStore, SessionStore) {
			t.Helper()
			db, _, err := authdb.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			ss := NewSQLiteSessionStore(db)
			us := InitializeUserStore()
			us.SetSessionStore(ss)
			mustAdd(t, us, "alice")
			return us, ss
		}},
	}
}

func mustAdd(t *testing.T, us *UserStore, name string) {
	t.Helper()
	if err := us.AddUser(name, testPassword, "test"); err != nil {
		t.Fatal(err)
	}
}

func login(t *testing.T, us *UserStore, name string) string {
	t.Helper()
	tok, _, err := us.AuthenticateUser(name, testPassword, 0)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func forEachBackend(t *testing.T, fn func(t *testing.T, us *UserStore, ss SessionStore)) {
	for _, b := range sessionBackends() {
		t.Run(b.name, func(t *testing.T) {
			us, ss := b.make(t)
			fn(t, us, ss)
		})
	}
}

func TestSessions_SeveralPerUser(t *testing.T) {
	forEachBackend(t, func(t *testing.T, us *UserStore, _ SessionStore) {
		a, b := login(t, us, "alice"), login(t, us, "alice")
		for _, tok := range []string{a, b} {
			if u, err := us.ValidateSessionToken(tok); err != nil || u != "alice" {
				t.Fatalf("validate: %q %v", u, err)
			}
		}
	})
}

func TestSessions_CapEvictsOldest(t *testing.T) {
	forEachBackend(t, func(t *testing.T, us *UserStore, _ SessionStore) {
		var toks []string
		for i := 0; i < MaxSessionsPerUser+1; i++ {
			toks = append(toks, login(t, us, "alice"))
		}
		if _, err := us.ValidateSessionToken(toks[0]); err == nil {
			t.Error("oldest session should have been evicted")
		}
		for i, tok := range toks[1:] {
			if _, err := us.ValidateSessionToken(tok); err != nil {
				t.Errorf("session %d should be valid: %v", i+1, err)
			}
		}
	})
}

func TestSessions_DisabledUserRejected(t *testing.T) {
	forEachBackend(t, func(t *testing.T, us *UserStore, ss SessionStore) {
		tok := login(t, us, "alice")
		// Flip the flag directly so the session is not removed by
		// DisableUser, exercising the check at validation.
		us.mu.Lock()
		us.Users["alice"].Enabled = false
		us.mu.Unlock()

		_, err := us.ValidateSessionToken(tok)
		if err == nil || err.Error() != "user account is disabled" {
			t.Fatalf("got %v", err)
		}
		if _, ok, _ := ss.Get(HashToken(tok)); ok {
			t.Error("session should have been deleted")
		}
	})
}

func TestSessions_DeletedUserRejected(t *testing.T) {
	forEachBackend(t, func(t *testing.T, us *UserStore, ss SessionStore) {
		tok := login(t, us, "alice")
		us.mu.Lock()
		delete(us.Users, "alice")
		us.mu.Unlock()

		if _, err := us.ValidateSessionToken(tok); err == nil {
			t.Fatal("expected rejection")
		}
		if _, ok, _ := ss.Get(HashToken(tok)); ok {
			t.Error("session should have been deleted")
		}
	})
}

func TestSessions_LockoutDeletesSessions(t *testing.T) {
	forEachBackend(t, func(t *testing.T, us *UserStore, _ SessionStore) {
		tok := login(t, us, "alice")
		for i := 0; i < 3; i++ {
			_, _, _ = us.AuthenticateUser("alice", "wrong", 3)
		}
		if err := us.EnableUser("alice"); err != nil {
			t.Fatal(err)
		}
		if _, err := us.ValidateSessionToken(tok); err == nil {
			t.Fatal("session survived lockout")
		}
	})
}

func TestSessions_RemoveDisableAndPasswordChangeDeleteSessions(t *testing.T) {
	forEachBackend(t, func(t *testing.T, us *UserStore, _ SessionStore) {
		tok := login(t, us, "alice")
		if err := us.UpdateUser("alice", "another-fake-password", ""); err != nil {
			t.Fatal(err)
		}
		if _, err := us.ValidateSessionToken(tok); err == nil {
			t.Fatal("session survived password change")
		}

		tok = login2(t, us, "another-fake-password")
		if err := us.DisableUser("alice"); err != nil {
			t.Fatal(err)
		}
		_ = us.EnableUser("alice")
		if _, err := us.ValidateSessionToken(tok); err == nil {
			t.Fatal("session survived disable")
		}

		tok = login2(t, us, "another-fake-password")
		if err := us.RemoveUser("alice"); err != nil {
			t.Fatal(err)
		}
		mustAdd(t, us, "alice")
		if _, err := us.ValidateSessionToken(tok); err == nil {
			t.Fatal("session survived removal")
		}
	})
}

func login2(t *testing.T, us *UserStore, password string) string {
	t.Helper()
	tok, _, err := us.AuthenticateUser("alice", password, 0)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestSessions_ExpiredRejectedAndSwept(t *testing.T) {
	forEachBackend(t, func(t *testing.T, us *UserStore, ss SessionStore) {
		us.mu.RLock()
		fp := passwordFingerprint(us.Users["alice"].PasswordHash)
		us.mu.RUnlock()
		now := time.Now()
		if err := ss.Create(Session{TokenHash: HashToken("old"), Username: "alice",
			PasswordFingerprint: fp, CreatedAt: now.Add(-2 * time.Hour),
			ExpiresAt: now.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
		if _, err := us.ValidateSessionToken("old"); err == nil ||
			err.Error() != "session has expired" {
			t.Fatalf("got %v", err)
		}

		// Re-add it, then sweep it away.
		_ = ss.Create(Session{TokenHash: HashToken("old2"), Username: "bob",
			CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)})
		live := login(t, us, "alice")
		if err := ss.Sweep(time.Now(), []string{"alice", "bob"}); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := ss.Get(HashToken("old2")); ok {
			t.Error("expired session not swept")
		}
		if _, err := us.ValidateSessionToken(live); err != nil {
			t.Errorf("live session swept: %v", err)
		}
	})
}

func TestSessions_SweeperRuns(t *testing.T) {
	us := InitializeUserStore()
	ss := NewMemorySessionStore()
	us.SetSessionStore(ss)
	_ = ss.Create(Session{TokenHash: "h", Username: "alice",
		CreatedAt: time.Now().Add(-2 * time.Hour), ExpiresAt: time.Now().Add(-time.Hour)})
	stop := us.StartSessionSweeper(10 * time.Millisecond)
	defer stop()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok, _ := ss.Get("h"); !ok {
			stop()
			stop() // idempotent
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("sweeper did not remove the expired session")
}

func TestSessions_SurviveReopen(t *testing.T) {
	dir := t.TempDir()
	db, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	us := InitializeUserStore()
	us.SetSessionStore(NewSQLiteSessionStore(db))
	mustAdd(t, us, "alice")
	tok := login(t, us, "alice")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db2, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	us.SetSessionStore(NewSQLiteSessionStore(db2))
	if u, err := us.ValidateSessionToken(tok); err != nil || u != "alice" {
		t.Fatalf("after reopen: %q %v", u, err)
	}
}

func reopen(t *testing.T, dir, usersPath string) (*UserStore, *sql.DB) {
	t.Helper()
	db, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	us, err := LoadUserStore(usersPath)
	if err != nil {
		t.Fatal(err)
	}
	us.SetSessionStore(NewSQLiteSessionStore(db))
	return us, db
}

func TestSessions_PasswordChangedWhileClosed(t *testing.T) {
	dir := t.TempDir()
	usersPath := filepath.Join(dir, "users.yaml")

	db, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	us := InitializeUserStore()
	us.SetSessionStore(NewSQLiteSessionStore(db))
	mustAdd(t, us, "alice")
	if err := SaveUserStore(usersPath, us); err != nil {
		t.Fatal(err)
	}
	tok := login(t, us, "alice")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	// -update-user while the server is stopped: no session store, so
	// nothing can delete the session.
	offline, err := LoadUserStore(usersPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := offline.UpdateUser("alice", "another-fake-password", ""); err != nil {
		t.Fatal(err)
	}
	if err := SaveUserStore(usersPath, offline); err != nil {
		t.Fatal(err)
	}

	us2, _ := reopen(t, dir, usersPath)
	if _, err := us2.ValidateSessionToken(tok); err == nil {
		t.Fatal("session survived an offline password change")
	}
}

func TestSessions_DeleteAndReAddWhileClosed(t *testing.T) {
	dir := t.TempDir()
	usersPath := filepath.Join(dir, "users.yaml")

	db, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	us := InitializeUserStore()
	us.SetSessionStore(NewSQLiteSessionStore(db))
	mustAdd(t, us, "alice")
	if err := SaveUserStore(usersPath, us); err != nil {
		t.Fatal(err)
	}
	tok := login(t, us, "alice")
	_ = db.Close()

	offline, _ := LoadUserStore(usersPath)
	if err := offline.RemoveUser("alice"); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, offline, "alice") // same password, new salt
	if err := SaveUserStore(usersPath, offline); err != nil {
		t.Fatal(err)
	}

	us2, _ := reopen(t, dir, usersPath)
	if _, err := us2.ValidateSessionToken(tok); err == nil {
		t.Fatal("session survived delete and re-add")
	}
}

func TestSessions_TokenNotOnDisk(t *testing.T) {
	dir := t.TempDir()
	db, path, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	us := InitializeUserStore()
	us.SetSessionStore(NewSQLiteSessionStore(db))
	mustAdd(t, us, "alice")
	tok := login(t, us, "alice")

	// The row may still be in the write-ahead log or already in the main
	// file, so look at both together.
	var all []byte
	for _, p := range []string{path, path + "-wal"} {
		data, err := os.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		all = append(all, data...)
	}
	if !bytes.Contains(all, []byte(HashToken(tok))) {
		t.Fatal("session row (token hash) never reached disk; test would be vacuous")
	}
	if bytes.Contains(all, []byte(tok)) {
		t.Error("raw token found on disk")
	}
}

// startedWhileStopped sets up alice and bob with live sessions in auth.db,
// closes the database, lets change edit the users file as an offline
// command would, then reopens everything and starts the sweeper.
func startedWhileStopped(t *testing.T, change func(offline *UserStore)) (us *UserStore, aliceTok, bobTok string) {
	t.Helper()
	dir := t.TempDir()
	usersPath := filepath.Join(dir, "users.yaml")

	db, _, err := authdb.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := InitializeUserStore()
	first.SetSessionStore(NewSQLiteSessionStore(db))
	mustAdd(t, first, "alice")
	mustAdd(t, first, "bob")
	if err := SaveUserStore(usersPath, first); err != nil {
		t.Fatal(err)
	}
	aliceTok, bobTok = login(t, first, "alice"), login(t, first, "bob")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	offline, err := LoadUserStore(usersPath)
	if err != nil {
		t.Fatal(err)
	}
	change(offline)
	if err := SaveUserStore(usersPath, offline); err != nil {
		t.Fatal(err)
	}

	us, _ = reopen(t, dir, usersPath)
	stop := us.StartSessionSweeper(time.Hour)
	t.Cleanup(stop)
	return us, aliceTok, bobTok
}

func TestSessions_DisabledWhileClosedStaysRevoked(t *testing.T) {
	us, aliceTok, bobTok := startedWhileStopped(t, func(o *UserStore) {
		if err := o.DisableUser("alice"); err != nil {
			t.Fatal(err)
		}
	})

	if _, ok, _ := us.sessionStore().Get(HashToken(aliceTok)); ok {
		t.Error("disabled user's session row survived startup")
	}
	if err := us.EnableUser("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := us.ValidateSessionToken(aliceTok); err == nil {
		t.Fatal("re-enabling revived a session from before the disable")
	}
	if u, err := us.ValidateSessionToken(bobTok); err != nil || u != "bob" {
		t.Fatalf("live user's session lost: %q %v", u, err)
	}
}

func TestSessions_DeletedWhileClosedRowRemoved(t *testing.T) {
	us, aliceTok, bobTok := startedWhileStopped(t, func(o *UserStore) {
		if err := o.RemoveUser("alice"); err != nil {
			t.Fatal(err)
		}
	})

	if _, ok, _ := us.sessionStore().Get(HashToken(aliceTok)); ok {
		t.Error("deleted user's session row survived startup")
	}
	if u, err := us.ValidateSessionToken(bobTok); err != nil || u != "bob" {
		t.Fatalf("live user's session lost: %q %v", u, err)
	}
}

func TestSessions_SweepKeepsLiveUsers(t *testing.T) {
	forEachBackend(t, func(t *testing.T, us *UserStore, ss SessionStore) {
		tok := login(t, us, "alice")
		stop := us.StartSessionSweeper(time.Hour)
		defer stop()
		if u, err := us.ValidateSessionToken(tok); err != nil || u != "alice" {
			t.Fatalf("live session swept: %q %v", u, err)
		}
	})
}

// countingStore counts Create calls.
type countingStore struct {
	SessionStore
	creates int
}

func (c *countingStore) Create(s Session) error {
	c.creates++
	return c.SessionStore.Create(s)
}

func TestVerifyCredentials_CreatesNoSession(t *testing.T) {
	cs := &countingStore{SessionStore: NewMemorySessionStore()}
	us := InitializeUserStore()
	us.SetSessionStore(cs)
	mustAdd(t, us, "alice")

	if err := us.VerifyCredentials("alice", testPassword, 0); err != nil {
		t.Fatal(err)
	}
	if cs.creates != 0 {
		t.Fatalf("VerifyCredentials created %d sessions", cs.creates)
	}
	us.mu.RLock()
	last := us.Users["alice"].LastLogin
	us.mu.RUnlock()
	if last == nil {
		t.Error("LastLogin not set")
	}
	if err := us.VerifyCredentials("alice", "wrong", 0); err == nil {
		t.Error("expected failure for wrong password")
	}
	if n, _ := us.GetFailedAttempts("alice"); n != 1 {
		t.Errorf("failed attempts = %d", n)
	}
	login(t, us, "alice")
	if cs.creates != 1 {
		t.Fatalf("AuthenticateUser created %d sessions", cs.creates)
	}
}
