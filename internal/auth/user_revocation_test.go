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
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// revocations records the usernames a UserStore reported through its
// revocation hook.
type revocations struct {
	mu    sync.Mutex
	names []string
}

func (r *revocations) record(username string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, username)
}

func (r *revocations) contains(username string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range r.names {
		if n == username {
			return true
		}
	}
	return false
}

// storeWithUser builds a store holding one enabled user with a known
// password, along with a recorder for its revocation hook.
func storeWithUser(t *testing.T, username, password string) (*UserStore, *revocations) {
	t.Helper()
	store := InitializeUserStore()
	if err := store.AddUser(username, password, "test user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	var rec revocations
	store.SetRevocationHook(rec.record)
	return store, &rec
}

// TestIsActive covers the check the OAuth server consults on every
// access token validation and refresh.
func TestIsActive(t *testing.T) {
	store, _ := storeWithUser(t, "alice", "correct horse")

	if !store.IsActive("alice") {
		t.Fatal("an enabled user should be active")
	}
	if store.IsActive("nobody") {
		t.Fatal("an unknown user must never be active")
	}
	if err := store.DisableUser("alice"); err != nil {
		t.Fatalf("DisableUser: %v", err)
	}
	if store.IsActive("alice") {
		t.Fatal("a disabled user must not be active")
	}
}

// TestDisableUserRevokes covers the finding that disabling an account
// left its OAuth tokens working until they expired: the store now
// reports the withdrawal, which main routes to the OAuth server.
func TestDisableUserRevokes(t *testing.T) {
	store, rec := storeWithUser(t, "alice", "correct horse")

	if err := store.DisableUser("alice"); err != nil {
		t.Fatalf("DisableUser: %v", err)
	}
	if !rec.contains("alice") {
		t.Fatal("disabling a user did not report a revocation")
	}
}

// TestRemoveUserRevokes is the same for deletion.
func TestRemoveUserRevokes(t *testing.T) {
	store, rec := storeWithUser(t, "alice", "correct horse")

	if err := store.RemoveUser("alice"); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}
	if !rec.contains("alice") {
		t.Fatal("deleting a user did not report a revocation")
	}
}

// TestLockoutRevokes covers the automatic lockout path, which disables
// the account in place after too many failed password attempts.
func TestLockoutRevokes(t *testing.T) {
	store, rec := storeWithUser(t, "alice", "correct horse")

	for i := 0; i < 3; i++ {
		if _, _, err := store.AuthenticateUser("alice", "wrong", 3); err == nil {
			t.Fatal("bad password accepted")
		}
	}
	if store.IsActive("alice") {
		t.Fatal("the account should be locked out")
	}
	if !rec.contains("alice") {
		t.Fatal("lockout did not report a revocation")
	}
}

// TestSuccessfulLoginDoesNotRevoke is the negative control: an ordinary
// sign-in must not withdraw anything.
func TestSuccessfulLoginDoesNotRevoke(t *testing.T) {
	store, rec := storeWithUser(t, "alice", "correct horse")

	if _, _, err := store.AuthenticateUser("alice", "correct horse", 3); err != nil {
		t.Fatalf("AuthenticateUser: %v", err)
	}
	if rec.contains("alice") {
		t.Fatal("a successful login reported a revocation")
	}
}

// TestReloadRevokesRemovedAndDisabledUsers covers the path a running
// server actually takes: the user file is edited and the watcher
// reloads it, so a user disabled or deleted on disk has to lose its
// tokens then and there.
func TestReloadRevokesRemovedAndDisabledUsers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.yaml")

	store := InitializeUserStore()
	for _, name := range []string{"alice", "bob", "carol"} {
		if err := store.AddUser(name, "correct horse", ""); err != nil {
			t.Fatalf("AddUser %s: %v", name, err)
		}
	}
	if err := SaveUserStore(path, store); err != nil {
		t.Fatalf("SaveUserStore: %v", err)
	}

	loaded, err := LoadUserStore(path)
	if err != nil {
		t.Fatalf("LoadUserStore: %v", err)
	}
	var rec revocations
	loaded.SetRevocationHook(rec.record)

	// Disable alice, delete bob, leave carol alone, and reload.
	if err := store.DisableUser("alice"); err != nil {
		t.Fatalf("DisableUser: %v", err)
	}
	if err := store.RemoveUser("bob"); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}
	if err := SaveUserStore(path, store); err != nil {
		t.Fatalf("SaveUserStore: %v", err)
	}
	if err := loaded.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if !rec.contains("alice") {
		t.Fatal("a user disabled in the file was not revoked on reload")
	}
	if !rec.contains("bob") {
		t.Fatal("a user removed from the file was not revoked on reload")
	}
	if rec.contains("carol") {
		t.Fatal("an untouched user was revoked on reload")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("user file: %v", err)
	}
}
