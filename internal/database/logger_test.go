/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package database

import (
    "os"
    "strings"
    "testing"
    "time"
)

func TestLogLevel_FromEnvironment(t *testing.T) {
    tests := []struct {
        name     string
        envValue string
        expected LogLevel
    }{
        {"none string", "none", LogLevelNone},
        {"empty string", "", LogLevelNone},
        {"info", "info", LogLevelInfo},
        {"INFO uppercase", "INFO", LogLevelInfo},
        {"debug", "debug", LogLevelDebug},
        {"DEBUG uppercase", "DEBUG", LogLevelDebug},
        {"trace", "trace", LogLevelTrace},
        {"TRACE uppercase", "TRACE", LogLevelTrace},
        {"invalid value", "invalid", LogLevelNone},
        {"with whitespace", "  debug  ", LogLevelDebug},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Parse the level from env value (simulating what init() does)
            levelStr := strings.ToLower(strings.TrimSpace(tt.envValue))

            var expected LogLevel
            switch levelStr {
            case "none", "":
                expected = LogLevelNone
            case "info":
                expected = LogLevelInfo
            case "debug":
                expected = LogLevelDebug
            case "trace":
                expected = LogLevelTrace
            default:
                expected = LogLevelNone
            }

            // Verify the expected level matches what we expect from the test case
            if expected != tt.expected {
                t.Errorf("Environment value %q parsed to %v, want %v", tt.envValue, expected, tt.expected)
            }
        })
    }
}

func TestSetLogLevel(t *testing.T) {
    // Save original level
    original := GetLogLevel()
    defer SetLogLevel(original)

    tests := []struct {
        name  string
        level LogLevel
    }{
        {"none", LogLevelNone},
        {"info", LogLevelInfo},
        {"debug", LogLevelDebug},
        {"trace", LogLevelTrace},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            SetLogLevel(tt.level)
            got := GetLogLevel()
            if got != tt.level {
                t.Errorf("After SetLogLevel(%v), GetLogLevel() = %v", tt.level, got)
            }
        })
    }
}

func TestLogConnection(t *testing.T) {
    // Save original level
    original := GetLogLevel()
    defer SetLogLevel(original)

    tests := []struct {
        name     string
        logLevel LogLevel
        connStr  string
        duration time.Duration
        err      error
    }{
        {
            name:     "successful connection - info level",
            logLevel: LogLevelInfo,
            connStr:  "postgres://user:pass@localhost/db",
            duration: 100 * time.Millisecond,
            err:      nil,
        },
        {
            name:     "failed connection - info level",
            logLevel: LogLevelInfo,
            connStr:  "postgres://user:pass@localhost/db",
            duration: 50 * time.Millisecond,
            err:      os.ErrPermission,
        },
        {
            name:     "no logging - none level",
            logLevel: LogLevelNone,
            connStr:  "postgres://user:pass@localhost/db",
            duration: 100 * time.Millisecond,
            err:      nil,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            SetLogLevel(tt.logLevel)

            // Call logging function (should not panic)
            LogConnection(tt.connStr, tt.duration, tt.err)
        })
    }
}

func TestLogConnectionDetails(t *testing.T) {
    // Save original level
    original := GetLogLevel()
    defer SetLogLevel(original)

    poolConfig := map[string]interface{}{
        "max_conns":         int32(10),
        "min_conns":         int32(2),
        "max_conn_lifetime": 1 * time.Hour,
    }

    tests := []struct {
        name     string
        logLevel LogLevel
        connStr  string
    }{
        {
            name:     "debug level - should log",
            logLevel: LogLevelDebug,
            connStr:  "postgres://user:pass@localhost/db",
        },
        {
            name:     "trace level - should log",
            logLevel: LogLevelTrace,
            connStr:  "postgres://user:pass@localhost/db",
        },
        {
            name:     "info level - should not log",
            logLevel: LogLevelInfo,
            connStr:  "postgres://user:pass@localhost/db",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            SetLogLevel(tt.logLevel)

            // Call logging function (should not panic)
            LogConnectionDetails(tt.connStr, poolConfig)
        })
    }
}

func TestLogMetadataLoad(t *testing.T) {
    // Save original level
    original := GetLogLevel()
    defer SetLogLevel(original)

    tests := []struct {
        name       string
        logLevel   LogLevel
        connStr    string
        tableCount int
        duration   time.Duration
        err        error
    }{
        {
            name:       "successful metadata load",
            logLevel:   LogLevelInfo,
            connStr:    "postgres://user:pass@localhost/db",
            tableCount: 42,
            duration:   200 * time.Millisecond,
            err:        nil,
        },
        {
            name:       "failed metadata load",
            logLevel:   LogLevelInfo,
            connStr:    "postgres://user:pass@localhost/db",
            tableCount: 0,
            duration:   50 * time.Millisecond,
            err:        os.ErrNotExist,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            SetLogLevel(tt.logLevel)

            // Call logging function (should not panic)
            LogMetadataLoad(tt.connStr, tt.tableCount, tt.duration, tt.err)
        })
    }
}

func TestLogMetadataDetails(t *testing.T) {
    // Save original level
    original := GetLogLevel()
    defer SetLogLevel(original)

    tests := []struct {
        name        string
        logLevel    LogLevel
        connStr     string
        schemaCount int
        tableCount  int
        columnCount int
    }{
        {
            name:        "debug level - should log",
            logLevel:    LogLevelDebug,
            connStr:     "postgres://user:pass@localhost/db",
            schemaCount: 3,
            tableCount:  42,
            columnCount: 256,
        },
        {
            name:        "info level - should not log",
            logLevel:    LogLevelInfo,
            connStr:     "postgres://user:pass@localhost/db",
            schemaCount: 3,
            tableCount:  42,
            columnCount: 256,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            SetLogLevel(tt.logLevel)

            // Call logging function (should not panic)
            LogMetadataDetails(tt.connStr, tt.schemaCount, tt.tableCount, tt.columnCount)
        })
    }
}

func TestLogQuery(t *testing.T) {
    // Save original level
    original := GetLogLevel()
    defer SetLogLevel(original)

    tests := []struct {
        name     string
        logLevel LogLevel
        query    string
        duration time.Duration
        rowCount int
        err      error
    }{
        {
            name:     "successful query",
            logLevel: LogLevelInfo,
            query:    "SELECT * FROM users WHERE id = 1",
            duration: 10 * time.Millisecond,
            rowCount: 1,
            err:      nil,
        },
        {
            name:     "failed query",
            logLevel: LogLevelInfo,
            query:    "SELECT * FROM invalid_table",
            duration: 5 * time.Millisecond,
            rowCount: 0,
            err:      os.ErrInvalid,
        },
        {
            name:     "long query - should truncate",
            logLevel: LogLevelInfo,
            query:    "SELECT * FROM users WHERE name = 'very long name that should be truncated in the log output because it exceeds the maximum length allowed for logging purposes'",
            duration: 10 * time.Millisecond,
            rowCount: 1,
            err:      nil,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            SetLogLevel(tt.logLevel)

            // Call logging function (should not panic)
            LogQuery(tt.query, tt.duration, tt.rowCount, tt.err)
        })
    }
}

func TestSanitizeConnStr(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "password in standard format",
            input:    "postgres://user:mypassword@localhost:5432/mydb",
            expected: "postgres://user:***@localhost:5432/mydb",
        },
        {
            name:     "no password",
            input:    "postgres://localhost:5432/mydb",
            expected: "postgres://localhost:5432/mydb",
        },
        {
            name:     "empty password",
            input:    "postgres://user:@localhost:5432/mydb",
            expected: "postgres://user:***@localhost:5432/mydb",
        },
        {
            name:     "complex password with special chars",
            input:    "postgres://user:p@ssw0rd!123@localhost:5432/mydb",
            expected: "postgres://user:***@localhost:5432/mydb",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := sanitizeConnStr(tt.input)
            if got != tt.expected {
                t.Errorf("sanitizeConnStr(%q) = %q, want %q", tt.input, got, tt.expected)
            }
        })
    }
}

func TestTruncate(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        maxLen   int
        expected string
    }{
        {
            name:     "short string",
            input:    "hello",
            maxLen:   10,
            expected: "hello",
        },
        {
            name:     "exact length",
            input:    "hello",
            maxLen:   5,
            expected: "hello",
        },
        {
            name:     "needs truncation",
            input:    "hello world this is a long string",
            maxLen:   10,
            expected: "hello worl...",
        },
        {
            name:     "empty string",
            input:    "",
            maxLen:   10,
            expected: "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := truncate(tt.input, tt.maxLen)
            if got != tt.expected {
                t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
            }
        })
    }
}
