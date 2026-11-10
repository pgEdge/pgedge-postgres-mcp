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
	"database/sql"
	"errors"
	"time"
)

type sqliteSessionStore struct {
	db *sql.DB
}

// NewSQLiteSessionStore returns a SessionStore backed by the sessions
// table of auth.db. Times are stored as Unix nanoseconds UTC.
func NewSQLiteSessionStore(db *sql.DB) SessionStore {
	return &sqliteSessionStore{db: db}
}

func (q *sqliteSessionStore) Create(s Session) (err error) {
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

	if _, err = tx.ExecContext(ctx,
		`DELETE FROM sessions WHERE username = ? AND expires_at <= ?`,
		s.Username, time.Now().UnixNano()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO sessions
            (token_hash, username, password_fingerprint, created_at, expires_at)
         VALUES (?, ?, ?, ?, ?)`,
		s.TokenHash, s.Username, s.PasswordFingerprint,
		s.CreatedAt.UnixNano(), s.ExpiresAt.UnixNano()); err != nil {
		return err
	}
	// Keep the newest MaxSessionsPerUser; rowid breaks created_at ties in
	// insertion order, so the session just added is never the one dropped.
	if _, err = tx.ExecContext(ctx,
		`DELETE FROM sessions WHERE username = ? AND token_hash NOT IN (
            SELECT token_hash FROM sessions WHERE username = ?
            ORDER BY created_at DESC, rowid DESC LIMIT ?)`,
		s.Username, s.Username, MaxSessionsPerUser); err != nil {
		return err
	}
	err = tx.Commit()
	return err
}

func (q *sqliteSessionStore) Get(tokenHash string) (Session, bool, error) {
	var s Session
	var created, expires int64
	err := q.db.QueryRowContext(context.Background(),
		`SELECT token_hash, username, password_fingerprint, created_at, expires_at
         FROM sessions WHERE token_hash = ?`, tokenHash).
		Scan(&s.TokenHash, &s.Username, &s.PasswordFingerprint, &created, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	s.CreatedAt = time.Unix(0, created).UTC()
	s.ExpiresAt = time.Unix(0, expires).UTC()
	return s, true, nil
}

func (q *sqliteSessionStore) Delete(tokenHash string) error {
	_, err := q.db.ExecContext(context.Background(),
		`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (q *sqliteSessionStore) DeleteUser(username string) error {
	_, err := q.db.ExecContext(context.Background(),
		`DELETE FROM sessions WHERE username = ?`, username)
	return err
}

func (q *sqliteSessionStore) Sweep(now time.Time) error {
	_, err := q.db.ExecContext(context.Background(),
		`DELETE FROM sessions WHERE expires_at <= ?`, now.UnixNano())
	return err
}
