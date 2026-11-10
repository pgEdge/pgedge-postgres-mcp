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

import "errors"

// errDeletesUnsaved reports that row deletions kept from an earlier
// failure are still not written, so a restart would restore what they
// remove.
var errDeletesUnsaved = errors.New("deletions kept from an earlier failure are still not saved")

// persister writes each row-level change the Store makes to durable
// storage. The Store calls it under its own lock, before (or, for
// deletions without an error return, alongside) the in-memory change.
// Every method is a single statement; tx groups several into one
// transaction for changes that touch more than one row.
type persister interface {
	putClient(c *Client) error
	deleteClient(id string) error
	putCode(c *AuthCode) error
	deleteCode(hash string) error
	putUsedCode(codeHash string, u usedCode) error
	deleteUsedCode(codeHash string) error
	putDevice(d *DeviceCode) error
	deleteDevice(hash string) error
	putToken(t *Token) error
	deleteToken(hash string) error
	putRotated(hash string, r rotatedRefresh) error
	deleteRotated(hash string) error

	// tx runs fn with a persister whose changes commit together, or are
	// all rolled back when fn returns an error.
	tx(fn func(persister) error) error
}

// nopPersister discards every write; NewStore uses it so that a Store
// built without a database behaves as a pure in-memory one.
type nopPersister struct{}

func (nopPersister) putClient(*Client) error                 { return nil }
func (nopPersister) deleteClient(string) error               { return nil }
func (nopPersister) putCode(*AuthCode) error                 { return nil }
func (nopPersister) deleteCode(string) error                 { return nil }
func (nopPersister) putUsedCode(string, usedCode) error      { return nil }
func (nopPersister) deleteUsedCode(string) error             { return nil }
func (nopPersister) putDevice(*DeviceCode) error             { return nil }
func (nopPersister) deleteDevice(string) error               { return nil }
func (nopPersister) putToken(*Token) error                   { return nil }
func (nopPersister) deleteToken(string) error                { return nil }
func (nopPersister) putRotated(string, rotatedRefresh) error { return nil }
func (nopPersister) deleteRotated(string) error              { return nil }
func (n nopPersister) tx(fn func(persister) error) error     { return fn(n) }

// maxRetryDeletes bounds the row deletions kept for retry after a failed
// write, so that a database that stays unwritable cannot grow the queue
// without limit.
const maxRetryDeletes = 10000

// pendingWrites collects the row deletions and updates that go with an
// in-memory change spanning several rows, so that commitLocked can run
// them in a single transaction. Memory is changed as the writes are
// queued, which is what keeps a deletion effective when the write fails.
type pendingWrites struct {
	fns     []func(persister) error
	deletes []func(persister) error
}

// add queues an upsert. A failed upsert is not retried: replaying a stale
// snapshot later could restore a row that has been deleted since.
func (w *pendingWrites) add(fn func(persister) error) { w.fns = append(w.fns, fn) }

// addDelete queues a row deletion. Every key the store deletes by is
// random and never reused, so a deletion is safe to repeat at any later
// time, and one whose transaction fails is kept for retry.
func (w *pendingWrites) addDelete(fn func(persister) error) {
	w.fns = append(w.fns, fn)
	w.deletes = append(w.deletes, fn)
}

// runAll returns a transaction body that runs each of fns in turn.
func runAll(fns []func(persister) error) func(persister) error {
	return func(p persister) error {
		for _, fn := range fns {
			if err := fn(p); err != nil {
				return err
			}
		}
		return nil
	}
}

// commitLocked runs w in one transaction, logging what on failure and
// keeping its deletions for retry. The in-memory change has already been
// made, so a failure here costs durability until a retry succeeds: a
// revoked credential the database still holds would be restored by a
// restart before then. Deletions kept from earlier failures are retried
// first. Callers must hold s.mu.
func (s *Store) commitLocked(w *pendingWrites, what string) error {
	s.retryDeletesLocked()
	if len(w.fns) == 0 {
		return nil
	}
	err := s.p.tx(runAll(w.fns))
	if err != nil {
		s.logError("oauth: %s: %v", what, err)
		s.queueRetryLocked(w.deletes)
	}
	return err
}

// commitDeletesLocked is commitLocked for a revocation, which is durable
// only once every deletion it depends on is written. It also fails while
// deletions kept from an earlier failure remain unwritten, since one of
// them may be this revocation's own row from an earlier attempt.
// Callers must hold s.mu.
func (s *Store) commitDeletesLocked(w *pendingWrites, what string) error {
	if err := s.commitLocked(w, what); err != nil {
		return err
	}
	if len(s.retryDeletes) > 0 {
		return errDeletesUnsaved
	}
	return nil
}

// deleteNowLocked runs one row deletion outside a transaction, logging
// what and keeping the deletion for retry on failure. Callers must hold
// s.mu.
func (s *Store) deleteNowLocked(fn func(persister) error, what string) {
	s.retryDeletesLocked()
	if err := fn(s.p); err != nil {
		s.logError("oauth: %s: %v", what, err)
		s.queueRetryLocked([]func(persister) error{fn})
	}
}

// queueRetryLocked keeps fns for retry, dropping the oldest kept
// deletions once there are more than maxRetryDeletes. Callers must hold
// s.mu.
func (s *Store) queueRetryLocked(fns []func(persister) error) {
	s.retryDeletes = append(s.retryDeletes, fns...)
	if over := len(s.retryDeletes) - maxRetryDeletes; over > 0 {
		s.retryDeletes = append([]func(persister) error(nil), s.retryDeletes[over:]...)
		s.logError("oauth: dropped %d deletions kept for retry; their rows may be restored at the next start", over)
	}
}

// retryDeletesLocked runs the deletions kept from earlier failures in one
// transaction, forgetting them once it commits. A failure was logged when
// each was first kept, so another is not logged here. Callers must hold
// s.mu.
func (s *Store) retryDeletesLocked() {
	if len(s.retryDeletes) == 0 {
		return
	}
	if s.p.tx(runAll(s.retryDeletes)) == nil {
		s.retryDeletes = nil
	}
}
