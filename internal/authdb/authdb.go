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
	"net/url"
	"os"
	"path/filepath"
	"time"

	"modernc.org/sqlite" // Pure Go SQLite driver
	sqlite3 "modernc.org/sqlite/lib"
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

	if err := createIfMissing(path); err != nil {
		return fail(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fail(err)
	}

	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return fail(err)
	}

	if err := migrateRetryingBusy(db); err != nil {
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

// createIfMissing creates an empty database file at path, with mode
// 0600 whatever the umask, if none exists there yet. The file is left
// at zero length so that SQLite discards any stale auth.db-wal it finds
// beside it, which is what makes deleting auth.db alone a reset.
func createIfMissing(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	return f.Close()
}

// Concurrent openers of a new file race to switch it to WAL mode, which
// needs an exclusive lock that SQLite refuses at once with SQLITE_BUSY,
// ignoring busy_timeout, because waiting could deadlock. The loser
// finds the file already in WAL mode on its next attempt.
const (
	busyAttempts = 50
	busyDelay    = 20 * time.Millisecond
)

// isBusy reports whether err is SQLITE_BUSY, including its extended
// result codes, which share the low byte.
func isBusy(err error) bool {
	var se *sqlite.Error
	return errors.As(err, &se) && se.Code()&0xff == sqlite3.SQLITE_BUSY
}

// migrateRetryingBusy runs migrate, trying again when the first
// connection to a new file loses the race to enable WAL mode.
func migrateRetryingBusy(db *sql.DB) error {
	var err error
	for i := 0; i < busyAttempts; i++ {
		if err = migrate(db); err == nil || !isBusy(err) {
			return err
		}
		time.Sleep(busyDelay)
	}
	return err
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

	if err := applyMigrations(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// applyMigrations brings the schema up to SchemaVersion on conn, which
// must already be inside a write transaction.
func applyMigrations(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}

	current, hasRow, err := storedVersion(ctx, conn)
	if err != nil {
		return err
	}
	if current > SchemaVersion {
		return fmt.Errorf("schema version %d is newer than the %d this server supports",
			current, SchemaVersion)
	}
	if err := runMigrations(ctx, conn, current); err != nil {
		return err
	}
	if current == SchemaVersion {
		return nil
	}
	return recordVersion(ctx, conn, hasRow)
}

// storedVersion reads the schema version, which is 0 when the table holds
// no row yet; hasRow reports whether there is one to update.
func storedVersion(ctx context.Context, conn *sql.Conn) (current int, hasRow bool, err error) {
	err = conn.QueryRowContext(ctx, "SELECT version FROM schema_version").Scan(&current)
	if err != nil && err != sql.ErrNoRows {
		return 0, false, err
	}
	return current, err == nil, nil
}

// runMigrations runs every migration from version current up to
// SchemaVersion.
func runMigrations(ctx context.Context, conn *sql.Conn, current int) error {
	for v := current; v < SchemaVersion; v++ {
		for _, stmt := range migrations[v] {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migrating to schema version %d: %w", v+1, err)
			}
		}
	}
	return nil
}

// recordVersion stores SchemaVersion, updating the existing row when
// hasRow and inserting one otherwise.
func recordVersion(ctx context.Context, conn *sql.Conn, hasRow bool) error {
	stmt := "INSERT INTO schema_version (version) VALUES (?)"
	if hasRow {
		stmt = "UPDATE schema_version SET version = ?"
	}
	_, err := conn.ExecContext(ctx, stmt, SchemaVersion)
	return err
}
