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
	"context"
	"errors"
	"testing"

	"pgedge-postgres-mcp/internal/auth"
)

func newUserStore(t *testing.T) *auth.UserStore {
	t.Helper()
	s := auth.InitializeUserStore()
	if err := s.AddUser("alice", "correct horse", "test"); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestUserStoreAuthenticatorSuccess(t *testing.T) {
	a := &UserStoreAuthenticator{Users: newUserStore(t)}
	sub, err := a.Authenticate(context.Background(), "alice", "correct horse", "192.0.2.1")
	if err != nil || sub != "alice" {
		t.Fatal(sub, err)
	}
}

func TestUserStoreAuthenticatorBadPassword(t *testing.T) {
	a := &UserStoreAuthenticator{Users: newUserStore(t)}
	_, err := a.Authenticate(context.Background(), "alice", "wrong", "192.0.2.1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal(err)
	}
	_, err = a.Authenticate(context.Background(), "nobody", "wrong", "192.0.2.1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal(err)
	}
}

// TestUserStoreAuthenticatorEqualisesUnknownUser covers the timing
// enumeration finding: an unknown or disabled username must still take
// a bcrypt comparison, so its response time does not distinguish it
// from a known one. There is no timing assertion here (that would be
// flaky); the test pins the behaviour that both paths fail the same
// way and that the dummy comparison is reachable.
func TestUserStoreAuthenticatorEqualisesUnknownUser(t *testing.T) {
	store := newUserStore(t)
	a := &UserStoreAuthenticator{Users: store}

	if _, err := a.Authenticate(context.Background(), "nobody", "whatever", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown user: %v", err)
	}
	if _, err := a.Authenticate(context.Background(), "alice", "wrong", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("known user, bad password: %v", err)
	}

	if err := store.DisableUser("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Authenticate(context.Background(), "alice", "correct horse", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("disabled user: %v", err)
	}

	if store.CanVerifyPassword("nobody") {
		t.Error("CanVerifyPassword should be false for an unknown user")
	}
	if store.CanVerifyPassword("alice") {
		t.Error("CanVerifyPassword should be false for a disabled user")
	}
}

func TestUserStoreAuthenticatorRateLimited(t *testing.T) {
	rl := auth.NewRateLimiter(15, 2)
	defer rl.Stop()
	a := &UserStoreAuthenticator{Users: newUserStore(t), RateLimiter: rl}
	for i := 0; i < 2; i++ {
		_, _ = a.Authenticate(context.Background(), "alice", "wrong", "192.0.2.1")
	}
	_, err := a.Authenticate(context.Background(), "alice", "correct horse", "192.0.2.1")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatal(err)
	}
	// a different address is unaffected
	if _, err := a.Authenticate(context.Background(), "alice", "correct horse", "192.0.2.2"); err != nil {
		t.Fatal(err)
	}
}

func TestUserStoreAuthenticatorLockout(t *testing.T) {
	a := &UserStoreAuthenticator{Users: newUserStore(t), MaxFailedAttempts: 1}
	_, _ = a.Authenticate(context.Background(), "alice", "wrong", "")
	_, err := a.Authenticate(context.Background(), "alice", "correct horse", "")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("locked account should fail generically")
	}
}
