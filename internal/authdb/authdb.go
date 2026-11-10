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
	"fmt"
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

	// Create the file ourselves so its mode does not depend on the umask.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		return fail(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fail(err)
	}

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
	db, err := sql.Open("sqlite", path+"?"+q.Encode())
	if err != nil {
		return fail(err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return fail(err)
	}
	return db, path, nil
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
