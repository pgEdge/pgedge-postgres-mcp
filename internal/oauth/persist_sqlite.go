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
	"sort"
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
	// Roll back on any exit but a successful commit, a panic in fn
	// included, so the connection is never returned with a transaction
	// still open.
	done := false
	defer func() {
		if !done {
			_ = tx.Rollback()
		}
	}()
	if err := fn(&sqlitePersister{x: tx}); err != nil {
		return err
	}
	done = true
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

func (p *sqlitePersister) putDevice(d *DeviceCode) error {
	return p.exec(`INSERT OR REPLACE INTO oauth_devices
        (device_hash, user_code, client_id, scope, subject, approved, denied,
         expires_at, last_polled, interval_ns) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.DeviceHash, d.UserCode, d.ClientID, d.Scope, d.Subject,
		boolToInt(d.Approved), boolToInt(d.Denied),
		toNanos(d.ExpiresAt), toNanos(d.LastPolled), int64(d.Interval))
}

func (p *sqlitePersister) deleteDevice(hash string) error {
	return p.exec(`DELETE FROM oauth_devices WHERE device_hash = ?`, hash)
}

func (p *sqlitePersister) putToken(t *Token) error {
	issued, err := json.Marshal(t.Issued)
	if err != nil {
		return err
	}
	if t.Issued == nil {
		issued = []byte("[]")
	}
	return p.exec(`INSERT OR REPLACE INTO oauth_tokens
        (hash, client_id, subject, scope, expires_at, refresh_hash, is_refresh,
         issued, family) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Hash, t.ClientID, t.Subject, t.Scope, toNanos(t.ExpiresAt),
		t.RefreshHash, boolToInt(t.IsRefresh), string(issued), t.Family)
}

func (p *sqlitePersister) deleteToken(hash string) error {
	return p.exec(`DELETE FROM oauth_tokens WHERE hash = ?`, hash)
}

func (p *sqlitePersister) putRotated(hash string, r rotatedRefresh) error {
	return p.exec(`INSERT OR REPLACE INTO oauth_rotated (hash, family, until)
        VALUES (?, ?, ?)`, hash, r.Family, toNanos(r.Until))
}

func (p *sqlitePersister) deleteRotated(hash string) error {
	return p.exec(`DELETE FROM oauth_rotated WHERE hash = ?`, hash)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// load reads every table into s's maps. Rows already expired at now are
// deleted, and a table holding more rows than its limit is cut back to
// the newest rows, the count being logged.
func (p *sqlitePersister) load(s *Store, now time.Time) error {
	if err := p.loadClients(s, now); err != nil {
		return fmt.Errorf("load oauth_clients: %w", err)
	}
	if err := p.loadCodes(s, now); err != nil {
		return fmt.Errorf("load oauth_codes: %w", err)
	}
	if err := p.loadUsedCodes(s, now); err != nil {
		return fmt.Errorf("load oauth_used_codes: %w", err)
	}
	if err := p.loadDevices(s, now); err != nil {
		return fmt.Errorf("load oauth_devices: %w", err)
	}
	if err := p.loadTokens(s, now); err != nil {
		return fmt.Errorf("load oauth_tokens: %w", err)
	}
	if err := p.loadRotated(s, now); err != nil {
		return fmt.Errorf("load oauth_rotated: %w", err)
	}
	s.trimmedClients = nil
	return p.dropReplayedLive(s)
}

// dropReplayedLive removes what a failed delete can leave beside its
// replay record: a token whose hash is on the rotation list, and an
// authorisation code whose hash is on the used-code list. Neither can be
// legitimately live, and loading it so would let a stolen, already
// spent credential through ahead of reuse detection. The records stay,
// so presenting the credential is still recognised as reuse. A token's
// cascade removes the access tokens it issued, as it did at runtime.
func (p *sqlitePersister) dropReplayedLive(s *Store) error {
	w := &pendingWrites{}
	for hash := range s.rotated {
		if _, live := s.tokens[hash]; live {
			s.deleteTokenLocked(hash, w)
		}
	}
	for hash := range s.usedCodes {
		if _, live := s.codes[hash]; live {
			h := hash
			w.addDelete(func(tp persister) error { return tp.deleteCode(h) })
			delete(s.codes, hash)
		}
	}
	if len(w.fns) == 0 {
		return nil
	}
	return p.tx(func(tp persister) error {
		for _, fn := range w.fns {
			if err := fn(tp); err != nil {
				return err
			}
		}
		return nil
	})
}

// loadClients applies the idle-client expiry Sweep would, then reads the
// rest. Clients holding a live token sort first, so that trimming to the
// limit never takes one before a client with none, matching PutClient's
// eviction rule; tokens of a client that had to go anyway are deleted by
// loadTokens, so no live token outlasts its client.
func (p *sqlitePersister) loadClients(s *Store, now time.Time) error {
	const live = `EXISTS (SELECT 1 FROM oauth_tokens t
        WHERE t.client_id = oauth_clients.id AND t.expires_at > ?)`
	cutoff := now.Add(-(s.clientRetention + clientIdleGrace))
	if err := p.exec(`DELETE FROM oauth_clients WHERE last_used < ? AND NOT `+live,
		cutoff.UnixNano(), now.UnixNano()); err != nil {
		return err
	}
	rows, err := p.x.QueryContext(context.Background(),
		`SELECT id, name, redirect_uris, created_at, last_used FROM oauth_clients
         ORDER BY `+live+` DESC, last_used DESC, id`, now.UnixNano())
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
			s.trimmedClients[c.ID] = true
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

// loadDevices also rebuilds s.userCodes, which is not stored.
func (p *sqlitePersister) loadDevices(s *Store, now time.Time) error {
	if err := p.exec(`DELETE FROM oauth_devices WHERE expires_at < ?`, now.UnixNano()); err != nil {
		return err
	}
	rows, err := p.x.QueryContext(context.Background(),
		`SELECT device_hash, user_code, client_id, scope, subject, approved, denied,
                expires_at, last_polled, interval_ns
         FROM oauth_devices ORDER BY expires_at DESC, device_hash`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var drop []string
	for rows.Next() {
		var (
			d                         DeviceCode
			approved, denied          int
			expires, polled, interval int64
		)
		if err := rows.Scan(&d.DeviceHash, &d.UserCode, &d.ClientID, &d.Scope,
			&d.Subject, &approved, &denied, &expires, &polled, &interval); err != nil {
			return err
		}
		if len(s.devices) >= s.limits.DeviceCodes {
			drop = append(drop, d.DeviceHash)
			continue
		}
		d.Approved, d.Denied = approved != 0, denied != 0
		d.ExpiresAt, d.LastPolled = fromNanos(expires), fromNanos(polled)
		d.Interval = time.Duration(interval)
		s.devices[d.DeviceHash] = &d
		s.userCodes[d.UserCode] = d.DeviceHash
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_ = rows.Close()
	return p.dropOverLimit("oauth_devices", "device_hash", drop, s)
}

// dropOverLimit deletes the rows in keys, which load found beyond the
// table's limit, and logs how many there were. table and col are
// constants chosen by the callers above, never caller input.
func (p *sqlitePersister) dropOverLimit(table, col string, keys []string, s *Store) error {
	if len(keys) == 0 {
		return nil
	}
	if err := p.tx(func(tp persister) error {
		for _, k := range keys {
			if err := tp.(*sqlitePersister).exec(
				`DELETE FROM `+table+` WHERE `+col+` = ?`, k); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	s.logError("oauth: %s held more rows than its limit; dropped the %d oldest", table, len(keys))
	return nil
}

// loadTokens reads every token, then applies to them what Sweep would
// (expiry, with its cascade) and the Tokens limit (keeping those that
// expire last), so expired rows are neither usable nor counted. Tokens
// whose client loadClients had to trim are removed too. The deletions go
// to disk in one transaction. The revocation hook is deliberately not
// called: nothing was usable before the store existed.
func (p *sqlitePersister) loadTokens(s *Store, now time.Time) error {
	if err := p.readTokens(s); err != nil {
		return err
	}

	w := &pendingWrites{}
	for hash, t := range s.tokens {
		if t.ExpiresAt.Before(now) || s.trimmedClients[t.ClientID] {
			s.deleteTokenLocked(hash, w)
		}
	}
	s.trimTokensToLimit(w)
	return p.applyWrites(w)
}

// readTokens loads every row of oauth_tokens into s.tokens.
func (p *sqlitePersister) readTokens(s *Store) error {
	rows, err := p.x.QueryContext(context.Background(),
		`SELECT hash, client_id, subject, scope, expires_at, refresh_hash,
                is_refresh, issued, family FROM oauth_tokens`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return err
		}
		s.tokens[t.Hash] = t
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_ = rows.Close()
	return nil
}

// scanToken reads one oauth_tokens row from rows.
func scanToken(rows *sql.Rows) (*Token, error) {
	var (
		t         Token
		expires   int64
		isRefresh int
		issued    string
	)
	if err := rows.Scan(&t.Hash, &t.ClientID, &t.Subject, &t.Scope, &expires,
		&t.RefreshHash, &isRefresh, &issued, &t.Family); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(issued), &t.Issued); err != nil {
		return nil, fmt.Errorf("token issued: %w", err)
	}
	if len(t.Issued) == 0 {
		t.Issued = nil
	}
	t.ExpiresAt, t.IsRefresh = fromNanos(expires), isRefresh != 0
	return &t, nil
}

// trimTokensToLimit drops the tokens beyond the Tokens limit, keeping
// those that expire last, and queues the row deletions on w.
func (s *Store) trimTokensToLimit(w *pendingWrites) {
	over := len(s.tokens) - s.limits.Tokens
	if over <= 0 {
		return
	}
	keep := make([]*Token, 0, len(s.tokens))
	for _, t := range s.tokens {
		keep = append(keep, t)
	}
	sort.Slice(keep, func(i, j int) bool {
		if !keep[i].ExpiresAt.Equal(keep[j].ExpiresAt) {
			return keep[i].ExpiresAt.After(keep[j].ExpiresAt)
		}
		return keep[i].Hash < keep[j].Hash
	})
	for _, t := range keep[s.limits.Tokens:] {
		s.deleteTokenLocked(t.Hash, w)
	}
	s.logError("oauth: oauth_tokens held more rows than its limit; dropped the %d oldest", over)
}

// applyWrites runs the queued writes in w as one transaction.
func (p *sqlitePersister) applyWrites(w *pendingWrites) error {
	if len(w.fns) == 0 {
		return nil
	}
	return p.tx(func(tp persister) error {
		for _, fn := range w.fns {
			if err := fn(tp); err != nil {
				return err
			}
		}
		return nil
	})
}

func (p *sqlitePersister) loadRotated(s *Store, now time.Time) error {
	if err := p.exec(`DELETE FROM oauth_rotated WHERE until < ?`, now.UnixNano()); err != nil {
		return err
	}
	rows, err := p.x.QueryContext(context.Background(),
		`SELECT hash, family, until FROM oauth_rotated ORDER BY until DESC, hash`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var drop []string
	for rows.Next() {
		var (
			hash  string
			r     rotatedRefresh
			until int64
		)
		if err := rows.Scan(&hash, &r.Family, &until); err != nil {
			return err
		}
		if len(s.rotated) >= s.limits.Tokens {
			drop = append(drop, hash)
			continue
		}
		r.Until = fromNanos(until)
		s.rotated[hash] = &r
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_ = rows.Close()
	return p.dropOverLimit("oauth_rotated", "hash", drop, s)
}
