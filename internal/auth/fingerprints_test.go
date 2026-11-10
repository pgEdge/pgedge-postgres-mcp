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
	"database/sql"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"pgedge-postgres-mcp/internal/authdb"
)

// savedUsers writes a users file holding the named users, all with
// testPassword, and returns its path along with the store that wrote it.
func savedUsers(t *testing.T, names ...string) (string, *UserStore) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "users.yaml")
	offline := InitializeUserStore()
	for _, name := range names {
		mustAdd(t, offline, name)
	}
	if err := SaveUserStore(path, offline); err != nil {
		t.Fatal(err)
	}
	return path, offline
}

// fingerprintsOf returns the current fingerprint of each named user.
func fingerprintsOf(t *testing.T, us *UserStore, names ...string) map[string]string {
	t.Helper()
	us.mu.RLock()
	defer us.mu.RUnlock()
	out := make(map[string]string, len(names))
	for _, name := range names {
		u, ok := us.Users[name]
		if !ok {
			t.Fatalf("no user %q", name)
		}
		out[name] = passwordFingerprint(u.PasswordHash)
	}
	return out
}

func TestReload_PasswordChangeRevokes(t *testing.T) {
	path, offline := savedUsers(t, "alice", "bob")
	loaded, err := LoadUserStore(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec revocations
	loaded.SetRevocationHook(rec.record)

	if err := offline.UpdateUser("alice", "another-fake-password", ""); err != nil {
		t.Fatal(err)
	}
	if err := SaveUserStore(path, offline); err != nil {
		t.Fatal(err)
	}
	if err := loaded.Reload(); err != nil {
		t.Fatal(err)
	}

	if !rec.contains("alice") {
		t.Fatal("a password changed in the file was not revoked on reload")
	}
	if rec.contains("bob") {
		t.Fatal("an unchanged user was revoked on reload")
	}
}

func TestUpdateUser_PasswordChangeRevokes(t *testing.T) {
	store, rec := storeWithUser(t, "alice", "correct horse")
	if err := store.UpdateUser("alice", "", "new note"); err != nil {
		t.Fatal(err)
	}
	if rec.contains("alice") {
		t.Fatal("an annotation change revoked the user")
	}
	if err := store.UpdateUser("alice", "battery staple", ""); err != nil {
		t.Fatal(err)
	}
	if !rec.contains("alice") {
		t.Fatal("a password change did not revoke the user")
	}
}

// noErr fails the test at once when err is not nil.
func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// requireRecorded fails unless the fingerprints stored in db, read through
// a fresh store, equal those of the named users in us. It returns them.
func requireRecorded(t *testing.T, db *sql.DB, us *UserStore, label string, names ...string) map[string]string {
	t.Helper()
	recorded, err := NewSQLiteFingerprintStore(db).All()
	noErr(t, err)
	if want := fingerprintsOf(t, us, names...); !reflect.DeepEqual(recorded, want) {
		t.Fatalf("%s = %v, want %v", label, recorded, want)
	}
	return recorded
}

func TestFingerprints_RecordedAndUpdated(t *testing.T) {
	db, _, err := authdb.Open(t.TempDir())
	noErr(t, err)
	t.Cleanup(func() { _ = db.Close() })

	path, offline := savedUsers(t, "alice", "bob")
	us, err := LoadUserStore(path)
	noErr(t, err)

	// No store installed: recording does nothing, and is not an error.
	if err := us.RecordFingerprints(); err != nil {
		t.Fatalf("RecordFingerprints with no store: %v", err)
	}

	us.SetFingerprintStore(NewSQLiteFingerprintStore(db))
	noErr(t, us.RecordFingerprints())
	recorded := requireRecorded(t, db, us, "recorded", "alice", "bob")

	// A reload that changes alice's password and removes bob updates
	// alice's row and drops bob's.
	before := recorded["alice"]
	noErr(t, offline.UpdateUser("alice", "another-fake-password", ""))
	noErr(t, offline.RemoveUser("bob"))
	noErr(t, SaveUserStore(path, offline))
	noErr(t, us.Reload())

	// Read through a fresh store on the same file, as a restart would.
	recorded = requireRecorded(t, db, us, "after reload recorded", "alice")
	if recorded["alice"] == before {
		t.Fatal("alice's fingerprint was not updated")
	}
}

func TestFingerprints_UpdateAndRemoveUserRecord(t *testing.T) {
	db, _, err := authdb.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	us := InitializeUserStore()
	mustAdd(t, us, "alice")
	mustAdd(t, us, "bob")
	us.SetFingerprintStore(NewSQLiteFingerprintStore(db))
	if err := us.RecordFingerprints(); err != nil {
		t.Fatal(err)
	}

	if err := us.UpdateUser("alice", "another-fake-password", ""); err != nil {
		t.Fatal(err)
	}
	if err := us.RemoveUser("bob"); err != nil {
		t.Fatal(err)
	}
	got, err := NewSQLiteFingerprintStore(db).All()
	if err != nil {
		t.Fatal(err)
	}
	if want := fingerprintsOf(t, us, "alice"); !reflect.DeepEqual(got, want) {
		t.Fatalf("recorded = %v, want %v", got, want)
	}
}

func TestStaleSubjects(t *testing.T) {
	us := InitializeUserStore()
	for _, name := range []string{"alice", "bob", "carol", "dave", "erin"} {
		mustAdd(t, us, name)
	}
	recorded := fingerprintsOf(t, us, "alice", "bob", "carol", "dave")
	recorded["frank"] = "0000"

	// bob is disabled; carol's password changed; dave was deleted and
	// re-added with another password; erin has no recorded fingerprint;
	// frank no longer exists; alice is unchanged.
	if err := us.DisableUser("bob"); err != nil {
		t.Fatal(err)
	}
	if err := us.UpdateUser("carol", "another-fake-password", ""); err != nil {
		t.Fatal(err)
	}
	if err := us.RemoveUser("dave"); err != nil {
		t.Fatal(err)
	}
	if err := us.AddUser("dave", "a-different-fake-password", ""); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		subject string
		stale   bool
	}{
		{"unchanged user kept", "alice", false},
		{"inactive user", "bob", true},
		{"changed password", "carol", true},
		{"re-added with a different password", "dave", true},
		{"no recorded fingerprint", "erin", true},
		{"deleted user", "frank", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := us.StaleSubjects([]string{tt.subject}, recorded)
			if stale := len(got) == 1 && got[0] == tt.subject; stale != tt.stale {
				t.Fatalf("StaleSubjects(%q) = %v, want stale=%v", tt.subject, got, tt.stale)
			}
		})
	}

	all := us.StaleSubjects([]string{"alice", "bob", "carol", "dave", "erin", "frank"}, recorded)
	sort.Strings(all)
	if want := []string{"bob", "carol", "dave", "erin", "frank"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("StaleSubjects(all) = %v, want %v", all, want)
	}
}
