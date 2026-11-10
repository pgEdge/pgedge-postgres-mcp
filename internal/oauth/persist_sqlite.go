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
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// execer is the part of *sql.DB and *sql.Tx the persister needs.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// sqlitePersister stores the Store's rows in auth.db. db is nil on the
// persister handed to a tx callback, which is already inside a
// transaction.
type sqlitePersister struct {
	db *sql.DB
	x  execer
}

func newSQLitePersister(db *sql.DB) *sqlitePersister {
	return &sqlitePersister{db: db, x: db}
}

// toNanos converts t to Unix nanoseconds UTC, storing a zero time as 0.
func toNanos(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

// fromNanos is the inverse of toNanos.
func fromNanos(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}

func (p *sqlitePersister) exec(query string, args ...any) error {
	_, err := p.x.ExecContext(context.Background(), query, args...)
	return err
}

func (p *sqlitePersister) tx(fn func(persister) error) error {
	if p.db == nil {
		// Already in a transaction; join it.
		return fn(p)
	}
	tx, err := p.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	if err := fn(&sqlitePersister{x: tx}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (p *sqlitePersister) putClient(c *Client) error {
	uris, err := json.Marshal(c.RedirectURIs)
	if err != nil {
		return err
	}
	if c.RedirectURIs == nil {
		uris = []byte("[]")
	}
	return p.exec(`INSERT OR REPLACE INTO oauth_clients
        (id, name, redirect_uris, created_at, last_used) VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.Name, string(uris), toNanos(c.CreatedAt), toNanos(c.LastUsed))
}

func (p *sqlitePersister) deleteClient(id string) error {
	return p.exec(`DELETE FROM oauth_clients WHERE id = ?`, id)
}

func (p *sqlitePersister) putCode(c *AuthCode) error {
	return p.exec(`INSERT OR REPLACE INTO oauth_codes
        (hash, client_id, redirect_uri, subject, scope, code_challenge, expires_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Hash, c.ClientID, c.RedirectURI, c.Subject, c.Scope, c.CodeChallenge,
		toNanos(c.ExpiresAt))
}

func (p *sqlitePersister) deleteCode(hash string) error {
	return p.exec(`DELETE FROM oauth_codes WHERE hash = ?`, hash)
}

func (p *sqlitePersister) putUsedCode(codeHash string, u usedCode) error {
	return p.exec(`INSERT OR REPLACE INTO oauth_used_codes
        (code_hash, refresh_hash, family, until) VALUES (?, ?, ?, ?)`,
		codeHash, u.RefreshHash, u.Family, toNanos(u.Until))
}

func (p *sqlitePersister) deleteUsedCode(codeHash string) error {
	return p.exec(`DELETE FROM oauth_used_codes WHERE code_hash = ?`, codeHash)
}

// load reads every table into s's maps. Rows already expired at now are
// deleted, and a table holding more rows than its limit is cut back to
// the newest rows, the count being logged.
func (p *sqlitePersister) load(s *Store, now time.Time) error {
	if err := p.loadClients(s); err != nil {
		return fmt.Errorf("load oauth_clients: %w", err)
	}
	if err := p.loadCodes(s, now); err != nil {
		return fmt.Errorf("load oauth_codes: %w", err)
	}
	if err := p.loadUsedCodes(s, now); err != nil {
		return fmt.Errorf("load oauth_used_codes: %w", err)
	}
	return nil
}

func (p *sqlitePersister) loadClients(s *Store) error {
	rows, err := p.x.QueryContext(context.Background(),
		`SELECT id, name, redirect_uris, created_at, last_used FROM oauth_clients
         ORDER BY last_used DESC, id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var drop []string
	for rows.Next() {
		var (
			c             Client
			uris          string
			created, used int64
		)
		if err := rows.Scan(&c.ID, &c.Name, &uris, &created, &used); err != nil {
			return err
		}
		if len(s.clients) >= s.limits.Clients {
			drop = append(drop, c.ID)
			continue
		}
		if err := json.Unmarshal([]byte(uris), &c.RedirectURIs); err != nil {
			return fmt.Errorf("client %s redirect_uris: %w", c.ID, err)
		}
		c.CreatedAt, c.LastUsed = fromNanos(created), fromNanos(used)
		s.clients[c.ID] = &c
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_ = rows.Close()
	return p.dropOverLimit("oauth_clients", "id", drop, s)
}

func (p *sqlitePersister) loadCodes(s *Store, now time.Time) error {
	if err := p.exec(`DELETE FROM oauth_codes WHERE expires_at < ?`, now.UnixNano()); err != nil {
		return err
	}
	rows, err := p.x.QueryContext(context.Background(),
		`SELECT hash, client_id, redirect_uri, subject, scope, code_challenge, expires_at
         FROM oauth_codes ORDER BY expires_at DESC, hash`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var drop []string
	for rows.Next() {
		var (
			c       AuthCode
			expires int64
		)
		if err := rows.Scan(&c.Hash, &c.ClientID, &c.RedirectURI, &c.Subject,
			&c.Scope, &c.CodeChallenge, &expires); err != nil {
			return err
		}
		if len(s.codes) >= s.limits.Codes {
			drop = append(drop, c.Hash)
			continue
		}
		c.ExpiresAt = fromNanos(expires)
		s.codes[c.Hash] = &c
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_ = rows.Close()
	return p.dropOverLimit("oauth_codes", "hash", drop, s)
}

func (p *sqlitePersister) loadUsedCodes(s *Store, now time.Time) error {
	if err := p.exec(`DELETE FROM oauth_used_codes WHERE until < ?`, now.UnixNano()); err != nil {
		return err
	}
	rows, err := p.x.QueryContext(context.Background(),
		`SELECT code_hash, refresh_hash, family, until FROM oauth_used_codes
         ORDER BY until DESC, code_hash`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var drop []string
	for rows.Next() {
		var (
			hash  string
			u     usedCode
			until int64
		)
		if err := rows.Scan(&hash, &u.RefreshHash, &u.Family, &until); err != nil {
			return err
		}
		if len(s.usedCodes) >= s.limits.Codes {
			drop = append(drop, hash)
			continue
		}
		u.Until = fromNanos(until)
		s.usedCodes[hash] = &u
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_ = rows.Close()
	return p.dropOverLimit("oauth_used_codes", "code_hash", drop, s)
}

// dropOverLimit deletes the rows in keys, which load found beyond the
// table's limit, and logs how many there were. table and col are
// constants chosen by the callers above, never caller input.
func (p *sqlitePersister) dropOverLimit(table, col string, keys []string, s *Store) error {
	if len(keys) == 0 {
		return nil
	}
	for _, k := range keys {
		if err := p.exec(`DELETE FROM `+table+` WHERE `+col+` = ?`, k); err != nil {
			return err
		}
	}
	s.logError("oauth: %s held more rows than its limit; dropped the %d oldest", table, len(keys))
	return nil
}
