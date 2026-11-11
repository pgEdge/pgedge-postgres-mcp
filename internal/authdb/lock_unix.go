//go:build !windows

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
	"os"
	"syscall"
)

// lockFile takes a non-blocking exclusive flock on f.
func lockFile(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.EWOULDBLOCK):
		return ErrLocked
	case errors.Is(err, syscall.ENOLCK), errors.Is(err, syscall.ENOTSUP),
		errors.Is(err, syscall.EOPNOTSUPP):
		return errors.Join(ErrLockUnsupported, err)
	default:
		return err
	}
}

// unlockFile drops the flock on f.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
