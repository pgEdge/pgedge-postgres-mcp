/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package authdb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// LockFileName is the file in the data directory that a running server
// holds an exclusive lock on, so that no second server uses the same
// auth.db. Each server keeps revocations and the sessions it has seen in
// memory, so a token revoked through one server would otherwise remain
// valid on the other.
const LockFileName = "auth.db.lock"

// ErrLocked reports that another process holds the data directory lock.
var ErrLocked = errors.New("another server is already using this data directory")

// ErrLockUnsupported reports that the file system holding the data
// directory does not support file locks, so the lock could not be taken.
var ErrLockUnsupported = errors.New("file locks are not supported here")

// DirLock is an exclusive lock on a data directory, held until Release
// or until the process exits, at which point the operating system drops
// it, so a crash never leaves a stale lock behind.
type DirLock struct {
	f *os.File
}

// Lock takes the exclusive lock on dataDir, creating the directory (mode
// 0700) and the lock file (mode 0600) if needed. It fails at once with
// an error wrapping ErrLocked if another process holds the lock, and
// with one wrapping ErrLockUnsupported if the file system cannot lock.
func Lock(dataDir string) (*DirLock, error) {
	path := filepath.Join(dataDir, LockFileName)
	fail := func(err error) (*DirLock, error) {
		return nil, fmt.Errorf("lock file %s: %w", path, err)
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fail(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fail(err)
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		return fail(err)
	}
	return &DirLock{f: f}, nil
}

// Release drops the lock. It is safe to call on a nil lock and more
// than once.
func (l *DirLock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := unlockFile(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	l.f = nil
	return err
}
