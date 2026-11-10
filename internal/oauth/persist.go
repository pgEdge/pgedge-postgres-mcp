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

	// tx runs fn with a persister whose changes commit together, or are
	// all rolled back when fn returns an error.
	tx(fn func(persister) error) error
}

// nopPersister discards every write; NewStore uses it so that a Store
// built without a database behaves as a pure in-memory one.
type nopPersister struct{}

func (nopPersister) putClient(*Client) error             { return nil }
func (nopPersister) deleteClient(string) error           { return nil }
func (nopPersister) putCode(*AuthCode) error             { return nil }
func (nopPersister) deleteCode(string) error             { return nil }
func (nopPersister) putUsedCode(string, usedCode) error  { return nil }
func (nopPersister) deleteUsedCode(string) error         { return nil }
func (nopPersister) putDevice(*DeviceCode) error         { return nil }
func (nopPersister) deleteDevice(string) error           { return nil }
func (n nopPersister) tx(fn func(persister) error) error { return fn(n) }
