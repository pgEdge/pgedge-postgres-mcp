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
	"path/filepath"
	"runtime"
	"testing"
)

func TestLockRefusesSecondHolder(t *testing.T) {
	dir := t.TempDir()

	first, err := Lock(dir)
	if err != nil {
		t.Fatalf("first Lock: %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	second, err := Lock(dir)
	if !errors.Is(err, ErrLocked) {
		_ = second.Release()
		t.Fatalf("second Lock: got %v, want ErrLocked", err)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	third, err := Lock(dir)
	if err != nil {
		t.Fatalf("Lock after Release: %v", err)
	}
	if err := third.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestLockCreatesDirectoryAndFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")

	l, err := Lock(dir)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	t.Cleanup(func() { _ = l.Release() })

	info, err := os.Stat(filepath.Join(dir, LockFileName))
	if err != nil {
		t.Fatalf("stat lock file: %v", err)
	}
	// Windows reports only a read-only bit, not POSIX permissions.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm&0077 != 0 {
			t.Errorf("lock file mode %o is accessible to others", perm)
		}
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	var nilLock *DirLock
	if err := nilLock.Release(); err != nil {
		t.Errorf("nil Release: %v", err)
	}

	l, err := Lock(t.TempDir())
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Errorf("second Release: %v", err)
	}
}

func TestLockFailsWhenDirectoryCannotBeCreated(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if l, err := Lock(filepath.Join(file, "data")); err == nil {
		_ = l.Release()
		t.Fatal("Lock under a regular file succeeded")
	}
}
