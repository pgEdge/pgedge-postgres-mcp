/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

// Package authdb opens the SQLite database (auth.db) that persists OAuth
// state and password-login sessions across server restarts.
package authdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// FileName is the name of the database file within the data directory.
const FileName = "auth.db"

// Open creates the data directory and database file if needed (mode 0700
// and 0600 respectively, tightening an existing looser file), applies the
// connection pragmas to every pooled connection, and migrates the schema
// to SchemaVersion. It returns the database and the full file path.
func Open(dataDir string) (*sql.DB, string, error) {
	path := filepath.Join(dataDir, FileName)
	fail := func(err error) (*sql.DB, string, error) {
		return nil, path, fmt.Errorf("auth database %s: %w", path, err)
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fail(err)
	}

	if err := createIfMissing(dataDir, path); err != nil {
		return fail(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fail(err)
	}

	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return fail(err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return fail(err)
	}
	return db, path, nil
}

// dsn returns the connection string for the database at path, which
// applies the connection pragmas to every pooled connection.
func dsn(path string) string {
	// busy_timeout comes first so the later pragmas wait on a locked
	// file rather than failing when several servers open it at once.
	q := url.Values{}
	for _, p := range []string{
		"busy_timeout(5000)",
		"journal_mode(WAL)",
		"foreign_keys(ON)",
		"synchronous(NORMAL)",
	} {
		q.Add("_pragma", p)
	}
	return path + "?" + q.Encode()
}

// createIfMissing creates the database at path, already in WAL mode and
// migrated, if no file exists there yet.
//
// Switching a file from the rollback journal to WAL takes an exclusive
// lock from inside the shared lock the pragma already holds. When two
// connections do that at once, SQLite returns SQLITE_BUSY to one of
// them at once, without consulting busy_timeout, because waiting could
// deadlock. Concurrent first opens of a new file therefore failed at
// random. Building the file under a temporary name and hard-linking it
// into place, which fails if the name already exists, means every opener
// of path finds a file that is already in WAL mode, where the pragma
// needs no lock upgrade. The file is created by us, rather than SQLite,
// so its mode does not depend on the umask.
func createIfMissing(dataDir, path string) error {
	if _, err := os.Lstat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	tmp, err := os.CreateTemp(dataDir, "."+FileName+".new-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		for _, p := range []string{tmpPath, tmpPath + "-wal", tmpPath + "-shm"} {
			_ = os.Remove(p)
		}
	}()
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		return err
	}

	// Closing the only connection checkpoints the WAL into the file,
	// which keeps WAL mode recorded in its header.
	db, err := sql.Open("sqlite", dsn(tmpPath))
	if err != nil {
		return err
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}

	err = os.Link(tmpPath, path)
	switch {
	case err == nil, errors.Is(err, fs.ErrExist):
		// Ours is in place, or another opener's got there first.
		return nil
	default:
		// A filesystem without hard links: create the file in place,
		// as before, accepting the race on a first concurrent open.
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return err
		}
		return f.Close()
	}
}

// migrate applies every migration above the stored schema version in a
// single transaction, and refuses a database written by a newer server.
func migrate(db *sql.DB) error {
	ctx := context.Background()

	// One connection, with an explicit BEGIN IMMEDIATE, so that
	// concurrent openers queue on the write lock instead of failing
	// when a read transaction is upgraded.
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}()

	if _, err := conn.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}

	current := 0
	err = conn.QueryRowContext(ctx, "SELECT version FROM schema_version").Scan(&current)
	hasRow := err == nil
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if current > SchemaVersion {
		return fmt.Errorf("schema version %d is newer than the %d this server supports",
			current, SchemaVersion)
	}

	for v := current; v < SchemaVersion; v++ {
		for _, stmt := range migrations[v] {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migrating to schema version %d: %w", v+1, err)
			}
		}
	}

	if current != SchemaVersion {
		var stmt string
		if hasRow {
			stmt = "UPDATE schema_version SET version = ?"
		} else {
			stmt = "INSERT INTO schema_version (version) VALUES (?)"
		}
		if _, err := conn.ExecContext(ctx, stmt, SchemaVersion); err != nil {
			return err
		}
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}
