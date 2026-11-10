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
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

// MaxSessionsPerUser is the number of live password-login sessions a
// user may hold at once; creating another evicts the oldest.
const MaxSessionsPerUser = 10

// Session is one password-login session. Only the hash of the token is
// ever stored; PasswordFingerprint ties the session to the password hash
// in force when it was issued, so a changed password revokes it.
type Session struct {
	TokenHash           string
	Username            string
	PasswordFingerprint string
	CreatedAt           time.Time
	ExpiresAt           time.Time
}

// SessionStore holds password-login sessions keyed by token hash.
type SessionStore interface {
	// Create stores s, first discarding the user's expired sessions and
	// then evicting the oldest (by CreatedAt) until at most
	// MaxSessionsPerUser remain, atomically.
	Create(s Session) error
	// Get returns the session for tokenHash, expired or not.
	Get(tokenHash string) (Session, bool, error)
	// Delete removes one session; deleting an absent one is not an error.
	Delete(tokenHash string) error
	// DeleteUser removes every session belonging to username.
	DeleteUser(username string) error
	// Sweep removes, in one step, every session that had expired by now
	// and every session whose user is not in activeUsers.
	Sweep(now time.Time, activeUsers []string) error
}

// passwordFingerprint returns the hex SHA-256 of a stored bcrypt hash.
func passwordFingerprint(hash string) string {
	sum := sha256.Sum256([]byte(hash))
	return hex.EncodeToString(sum[:])
}

type memorySessionStore struct {
	mu       sync.Mutex
	sessions map[string]Session
}

// NewMemorySessionStore returns a SessionStore that keeps sessions in
// memory only.
func NewMemorySessionStore() SessionStore {
	return &memorySessionStore{sessions: make(map[string]Session)}
}

func (m *memorySessionStore) Create(s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.sessions[s.TokenHash] = s

	var mine []Session
	for h, existing := range m.sessions {
		if existing.Username != s.Username {
			continue
		}
		if h != s.TokenHash && !existing.ExpiresAt.After(now) {
			delete(m.sessions, h)
			continue
		}
		mine = append(mine, existing)
	}
	if len(mine) > MaxSessionsPerUser {
		// Oldest first; the new session is never the one evicted.
		sort.Slice(mine, func(i, j int) bool {
			if mine[i].TokenHash == s.TokenHash {
				return false
			}
			if mine[j].TokenHash == s.TokenHash {
				return true
			}
			return mine[i].CreatedAt.Before(mine[j].CreatedAt)
		})
		for _, old := range mine[:len(mine)-MaxSessionsPerUser] {
			delete(m.sessions, old.TokenHash)
		}
	}
	return nil
}

func (m *memorySessionStore) Get(tokenHash string) (Session, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[tokenHash]
	return s, ok, nil
}

func (m *memorySessionStore) Delete(tokenHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, tokenHash)
	return nil
}

func (m *memorySessionStore) DeleteUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for h, s := range m.sessions {
		if s.Username == username {
			delete(m.sessions, h)
		}
	}
	return nil
}

func (m *memorySessionStore) Sweep(now time.Time, activeUsers []string) error {
	active := make(map[string]struct{}, len(activeUsers))
	for _, u := range activeUsers {
		active[u] = struct{}{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for h, s := range m.sessions {
		if _, ok := active[s.Username]; !ok || !s.ExpiresAt.After(now) {
			delete(m.sessions, h)
		}
	}
	return nil
}
