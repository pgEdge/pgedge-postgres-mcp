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
	"errors"
	"sync"
	"testing"
	"time"

	"pgedge-postgres-mcp/internal/authdb"
)

var errUnsaved = errors.New("test: revocation not saved")

// failingDeleteUser is a session store whose DeleteUser fails whilst
// failing is set.
type failingDeleteUser struct {
	SessionStore
	mu      sync.Mutex
	failing bool
}

func (f *failingDeleteUser) setFailing(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing = v
}

func (f *failingDeleteUser) DeleteUser(username string) error {
	f.mu.Lock()
	failing := f.failing
	f.mu.Unlock()
	if failing {
		return errUnsaved
	}
	return f.SessionStore.DeleteUser(username)
}

func TestUnsavedRevocation_LeavesFingerprintUnrecorded(t *testing.T) {
	db, _, err := authdb.Open(t.TempDir())
	noErr(t, err)
	t.Cleanup(func() { _ = db.Close() })

	us := InitializeUserStore()
	mustAdd(t, us, "alice")
	mustAdd(t, us, "bob")
	us.SetFingerprintStore(NewSQLiteFingerprintStore(db))
	noErr(t, us.RecordFingerprints())
	us.SetRevocationHook(func(string) error { return errUnsaved })

	noErr(t, us.UpdateUser("alice", "another-fake-password", ""))

	// alice has no record, so the next start revokes her OAuth state.
	requireRecorded(t, db, us, "recorded", "bob")
	if stale := us.StaleSubjects([]string{"alice", "bob"}, fingerprintsOf(t, us, "bob")); len(stale) != 1 || stale[0] != "alice" {
		t.Fatalf("stale subjects = %v, want [alice]", stale)
	}
}

func TestUnsavedSessionDelete_NotRevivedByEnable(t *testing.T) {
	us := InitializeUserStore()
	mustAdd(t, us, "alice")
	ss := &failingDeleteUser{SessionStore: NewMemorySessionStore()}
	us.SetSessionStore(ss)
	tok := login(t, us, "alice")

	ss.setFailing(true)
	noErr(t, us.DisableUser("alice"))
	noErr(t, us.EnableUser("alice"))
	if _, err := us.ValidateSessionToken(tok); err == nil {
		t.Fatal("a session whose deletion failed was revived by enabling the user")
	}

	// Once the sweeper's retry succeeds, new logins work again.
	ss.setFailing(false)
	stop := us.StartSessionSweeper(time.Hour)
	stop()
	if _, err := us.ValidateSessionToken(login(t, us, "alice")); err != nil {
		t.Fatalf("new session after a successful retry: %v", err)
	}
}
