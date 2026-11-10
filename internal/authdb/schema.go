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

// SchemaVersion is the schema version this build of the server writes
// and understands. A database recording a higher version was written by
// a newer server and is refused.
const SchemaVersion = 1

// migrations holds the statements for each schema version; entry i
// upgrades a database from version i to version i+1. All times are Unix
// nanoseconds UTC, with a zero time.Time stored as 0.
var migrations = [][]string{
	// Version 1: OAuth state and password-login sessions.
	{
		`CREATE TABLE oauth_clients (
            id            TEXT PRIMARY KEY,
            name          TEXT NOT NULL DEFAULT '',
            redirect_uris TEXT NOT NULL DEFAULT '[]',
            created_at    INTEGER NOT NULL DEFAULT 0,
            last_used     INTEGER NOT NULL DEFAULT 0
        )`,
		`CREATE TABLE oauth_codes (
            hash           TEXT PRIMARY KEY,
            client_id      TEXT NOT NULL,
            redirect_uri   TEXT NOT NULL DEFAULT '',
            subject        TEXT NOT NULL DEFAULT '',
            scope          TEXT NOT NULL DEFAULT '',
            code_challenge TEXT NOT NULL DEFAULT '',
            expires_at     INTEGER NOT NULL DEFAULT 0
        )`,
		`CREATE TABLE oauth_used_codes (
            code_hash    TEXT PRIMARY KEY,
            refresh_hash TEXT NOT NULL DEFAULT '',
            family       TEXT NOT NULL DEFAULT '',
            until        INTEGER NOT NULL DEFAULT 0
        )`,
		`CREATE TABLE oauth_devices (
            device_hash TEXT PRIMARY KEY,
            user_code   TEXT NOT NULL UNIQUE,
            client_id   TEXT NOT NULL DEFAULT '',
            scope       TEXT NOT NULL DEFAULT '',
            subject     TEXT NOT NULL DEFAULT '',
            approved    INTEGER NOT NULL DEFAULT 0,
            denied      INTEGER NOT NULL DEFAULT 0,
            expires_at  INTEGER NOT NULL DEFAULT 0,
            last_polled INTEGER NOT NULL DEFAULT 0,
            interval_ns INTEGER NOT NULL DEFAULT 0
        )`,
		`CREATE TABLE oauth_tokens (
            hash         TEXT PRIMARY KEY,
            client_id    TEXT NOT NULL DEFAULT '',
            subject      TEXT NOT NULL DEFAULT '',
            scope        TEXT NOT NULL DEFAULT '',
            expires_at   INTEGER NOT NULL DEFAULT 0,
            refresh_hash TEXT NOT NULL DEFAULT '',
            is_refresh   INTEGER NOT NULL DEFAULT 0,
            issued       TEXT NOT NULL DEFAULT '[]',
            family       TEXT NOT NULL DEFAULT ''
        )`,
		`CREATE INDEX oauth_tokens_family ON oauth_tokens(family)`,
		`CREATE INDEX oauth_tokens_subject ON oauth_tokens(subject)`,
		`CREATE TABLE oauth_rotated (
            hash   TEXT PRIMARY KEY,
            family TEXT NOT NULL DEFAULT '',
            until  INTEGER NOT NULL DEFAULT 0
        )`,
		`CREATE TABLE sessions (
            token_hash           TEXT PRIMARY KEY,
            username             TEXT NOT NULL,
            password_fingerprint TEXT NOT NULL,
            created_at           INTEGER NOT NULL,
            expires_at           INTEGER NOT NULL
        )`,
		`CREATE INDEX sessions_username ON sessions(username)`,
	},
}
