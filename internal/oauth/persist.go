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

// pendingWrites collects the row deletions and updates that go with an
// in-memory change spanning several rows, so that commitLocked can run
// them in a single transaction. Memory is changed as the writes are
// queued, which is what keeps a deletion effective when the write fails.
type pendingWrites struct{ fns []func(persister) error }

func (w *pendingWrites) add(fn func(persister) error) { w.fns = append(w.fns, fn) }

// commitLocked runs w in one transaction, logging what on failure. The
// in-memory change has already been made, so a failure here costs only
// durability. Callers must hold s.mu.
func (s *Store) commitLocked(w *pendingWrites, what string) {
	if len(w.fns) == 0 {
		return
	}
	if err := s.p.tx(func(p persister) error {
		for _, fn := range w.fns {
			if err := fn(p); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		s.logError("oauth: %s: %v", what, err)
	}
}
