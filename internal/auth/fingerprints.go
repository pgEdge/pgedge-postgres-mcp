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
	"context"
	"crypto/subtle"
	"database/sql"
	"log"
)

// FingerprintStore records, per user, the fingerprint of the password
// hash in force when the server last saw that user. The OAuth state in
// auth.db is bound to a username alone, so comparing these records with
// the users file at start-up is how a password changed, or a user deleted
// and re-added, whilst the server was stopped is detected.
type FingerprintStore interface {
	// All returns every recorded fingerprint, keyed by username.
	All() (map[string]string, error)
	// Replace makes the records exactly fingerprints, atomically.
	Replace(fingerprints map[string]string) error
}

type sqliteFingerprintStore struct {
	db *sql.DB
}

// NewSQLiteFingerprintStore returns a FingerprintStore backed by the
// user_fingerprints table of auth.db.
func NewSQLiteFingerprintStore(db *sql.DB) FingerprintStore {
	return &sqliteFingerprintStore{db: db}
}

func (q *sqliteFingerprintStore) All() (map[string]string, error) {
	rows, err := q.db.QueryContext(context.Background(),
		`SELECT username, password_fingerprint FROM user_fingerprints`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var username, fingerprint string
		if err := rows.Scan(&username, &fingerprint); err != nil {
			return nil, err
		}
		out[username] = fingerprint
	}
	return out, rows.Err()
}

func (q *sqliteFingerprintStore) Replace(fingerprints map[string]string) (err error) {
	ctx := context.Background()
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `DELETE FROM user_fingerprints`); err != nil {
		return err
	}
	for username, fingerprint := range fingerprints {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO user_fingerprints (username, password_fingerprint) VALUES (?, ?)`,
			username, fingerprint); err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

// SetFingerprintStore installs the store password fingerprints are
// recorded in. A UserStore with none set records nothing.
func (s *UserStore) SetFingerprintStore(fs FingerprintStore) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.fingerprints = fs
}

// currentFingerprints returns the fingerprint of every user's password
// hash. Callers must hold s.mu.
func (s *UserStore) currentFingerprints() map[string]string {
	out := make(map[string]string, len(s.Users))
	for name, u := range s.Users {
		out[name] = passwordFingerprint(u.PasswordHash)
	}
	return out
}

// RecordedFingerprints returns the fingerprints last recorded, or none
// when no fingerprint store is installed.
func (s *UserStore) RecordedFingerprints() (map[string]string, error) {
	s.mu.RLock()
	fs := s.fingerprints
	s.mu.RUnlock()

	if fs == nil {
		return map[string]string{}, nil
	}
	return fs.All()
}

// RecordFingerprints records the current fingerprint of every user,
// dropping the records of users no longer present. It does nothing when
// no fingerprint store is installed. Callers must not hold s.mu, and
// must have revoked any user whose fingerprint changed first, so that a
// crash between the two cannot leave a change recorded but unrevoked.
// That ordering does not help when the revocation itself fails to
// commit: the stored grants can then outlive a recorded password change.
func (s *UserStore) RecordFingerprints() error {
	// fpMu orders snapshot-and-write pairs, so an older snapshot is never
	// written over a newer one.
	s.fpMu.Lock()
	defer s.fpMu.Unlock()

	s.mu.RLock()
	fs := s.fingerprints
	current := s.currentFingerprints()
	s.mu.RUnlock()

	if fs == nil {
		return nil
	}
	return fs.Replace(current)
}

// recordFingerprints is RecordFingerprints for the in-process paths that
// change a password or remove a user, which log a failure rather than
// return it. A failed write is safe: the next start sees a mismatch and
// revokes the user's OAuth state again.
func (s *UserStore) recordFingerprints() {
	if err := s.RecordFingerprints(); err != nil {
		log.Printf("[AUTH] Failed to record password fingerprints: %v", err)
	}
}

// StaleSubjects returns those of subjects whose OAuth state must be
// revoked at start-up: a subject that is not an active user, whose
// password fingerprint differs from the one recorded, or that has no
// recorded fingerprint at all.
func (s *UserStore) StaleSubjects(subjects []string, recorded map[string]string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stale []string
	for _, subject := range subjects {
		user, exists := s.Users[subject]
		if !exists || !user.Enabled {
			stale = append(stale, subject)
			continue
		}
		want, ok := recorded[subject]
		got := passwordFingerprint(user.PasswordHash)
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			stale = append(stale, subject)
		}
	}
	return stale
}
