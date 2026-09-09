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
