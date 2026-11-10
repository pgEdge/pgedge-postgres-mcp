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
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var expectedTables = []string{
	"oauth_clients", "oauth_codes", "oauth_used_codes", "oauth_devices",
	"oauth_tokens", "oauth_rotated", "sessions", "user_fingerprints",
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return fi.Mode() & 0777
}

func TestOpen_CreatesFileWith0600(t *testing.T) {
	db, path, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if filepath.Base(path) != FileName {
		t.Errorf("path %q does not end in %s", path, FileName)
	}
	if m := mode(t, path); m != 0600 {
		t.Errorf("mode = %o, want 0600", m)
	}
}

func TestOpen_TightensLooserPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	db, _, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if m := mode(t, path); m != 0600 {
		t.Errorf("mode = %o, want 0600", m)
	}
}

func TestOpen_CreatesDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	db, _, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if m := mode(t, dir); m != 0700 {
		t.Errorf("dir mode = %o, want 0700", m)
	}
}

func TestOpen_Idempotent(t *testing.T) {
	dir := t.TempDir()
	db, _, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, _, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var v int
	if err := db.QueryRow("SELECT version FROM schema_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 1 {
		t.Errorf("schema_version = %d, want 1", v)
	}
	for _, name := range expectedTables {
		var n int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			name).Scan(&n)
		if err != nil || n != 1 {
			t.Errorf("table %s missing (n=%d, err=%v)", name, n, err)
		}
	}
}

func TestOpen_RejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	db, path, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE schema_version SET version = 99"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	_, _, err = Open(dir)
	if err == nil {
		t.Fatal("expected error for newer schema")
	}
	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "newer") {
		t.Errorf("error %q should name path and mention newer", err)
	}
}

func TestOpen_ErrorNamesPath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := Open(file)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), file) {
		t.Errorf("error %q should contain %s", err, file)
	}
}

func TestOpen_PragmasApplied(t *testing.T) {
	db, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	// Hold several connections at once so the pool opens more than one.
	var conns []*sql.Conn
	for i := 0; i < 3; i++ {
		c, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()
	for i, c := range conns {
		var jm string
		var bt, fk int
		if err := c.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&jm); err != nil {
			t.Fatal(err)
		}
		if err := c.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&bt); err != nil {
			t.Fatal(err)
		}
		if err := c.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatal(err)
		}
		if jm != "wal" || bt != 5000 || fk != 1 {
			t.Errorf("conn %d: journal_mode=%s busy_timeout=%d foreign_keys=%d", i, jm, bt, fk)
		}
	}
}

func TestOpen_ConcurrentOpenSameFile(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db, _, err := Open(dir)
			if err != nil {
				errs <- err
				return
			}
			db.Close()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// Only the database and its WAL files may remain; the temporary
	// files each opener built the database in must all be removed.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), FileName) {
			t.Errorf("left behind %s", e.Name())
		}
	}
}

func TestOpen_NewFileIsInWALMode(t *testing.T) {
	dir := t.TempDir()
	db, path, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Bytes 18 and 19 of the header are the write and read format
	// versions, which SQLite sets to 2 for WAL mode.
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	hdr := make([]byte, 20)
	if _, err := f.ReadAt(hdr, 0); err != nil {
		t.Fatal(err)
	}
	if hdr[18] != 2 || hdr[19] != 2 {
		t.Errorf("format versions = %d/%d, want 2/2 (WAL)", hdr[18], hdr[19])
	}
}

func TestOpen_TwoHandlesWriteConcurrently(t *testing.T) {
	dir := t.TempDir()
	a, _, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, _, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	const perHandle = 50
	var wg sync.WaitGroup
	errs := make(chan error, 2*perHandle)
	for name, db := range map[string]*sql.DB{"a": a, "b": b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perHandle; i++ {
				_, err := db.Exec(
					`INSERT INTO sessions (token_hash, username, password_fingerprint, created_at, expires_at)
					 VALUES (?, 'test-user', 'fp', 1, 2)`,
					fmt.Sprintf("%s-%d", name, i))
				if err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("insert failed: %v", err)
	}

	var n int
	if err := a.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2*perHandle {
		t.Errorf("row count = %d, want %d", n, 2*perHandle)
	}
}
