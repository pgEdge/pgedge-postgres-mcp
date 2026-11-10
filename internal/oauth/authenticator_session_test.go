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
	"testing"

	"pgedge-postgres-mcp/internal/auth"
)

// countingSessions counts Create calls on a memory session store.
type countingSessions struct {
	auth.SessionStore
	creates int
}

func (c *countingSessions) Create(s auth.Session) error {
	c.creates++
	return c.SessionStore.Create(s)
}

func TestUserStoreAuthenticator_CreatesNoSession(t *testing.T) {
	users := newUserStore(t)
	cs := &countingSessions{SessionStore: auth.NewMemorySessionStore()}
	users.SetSessionStore(cs)

	a := &UserStoreAuthenticator{Users: users}
	if _, err := a.Authenticate(context.Background(), "alice", "correct horse", ""); err != nil {
		t.Fatal(err)
	}
	if cs.creates != 0 {
		t.Fatalf("an OAuth login created %d password sessions", cs.creates)
	}
}
