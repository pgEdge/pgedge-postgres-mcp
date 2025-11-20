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
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel represents the logging verbosity level for database operations
type LogLevel int

const (
	// LogLevelNone disables all database logging
	LogLevelNone LogLevel = iota
	// LogLevelInfo logs basic information (connections, queries, errors)
	LogLevelInfo
	// LogLevelDebug logs detailed information (metadata loading, connection details, query details)
	LogLevelDebug
	// LogLevelTrace logs very detailed information (full queries, row counts, timings)
	LogLevelTrace
)

// Logger handles structured logging for database operations
type Logger struct {
	level  LogLevel
	logger *log.Logger
}

var globalLogger *Logger

func init() {
	// Initialize logger with level from environment variable
	// Default is LogLevelNone (no logging) when unset or set to "none"
	levelStr := strings.ToLower(strings.TrimSpace(os.Getenv("PGEDGE_DB_LOG_LEVEL")))

	var level LogLevel
	switch levelStr {
	case "none", "": // Explicitly handle "none" and empty string
		level = LogLevelNone
	case "info":
		level = LogLevelInfo
	case "debug":
		level = LogLevelDebug
	case "trace":
		level = LogLevelTrace
	default:
		// Invalid value, default to LogLevelNone
		level = LogLevelNone
	}

	globalLogger = &Logger{
		level:  level,
		logger: log.New(os.Stderr, "[DATABASE] ", log.LstdFlags),
	}
}

// SetLogLevel sets the global database log level
func SetLogLevel(level LogLevel) {
	globalLogger.level = level
}

// GetLogLevel returns the current log level
func GetLogLevel() LogLevel {
	return globalLogger.level
}

// Info logs an informational message (connections, basic queries, errors)
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level >= LogLevelInfo {
		l.logger.Printf("[INFO] "+format, args...)
	}
}

// Debug logs a debug message (metadata loading, connection pools, query details)
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level >= LogLevelDebug {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

// Trace logs a trace message (full queries, row counts, detailed timings)
func (l *Logger) Trace(format string, args ...interface{}) {
	if l.level >= LogLevelTrace {
		l.logger.Printf("[TRACE] "+format, args...)
	}
}

// LogConnection logs a database connection attempt
func LogConnection(connStr string, duration time.Duration, err error) {
	// Sanitize connection string to hide password
	sanitized := sanitizeConnStr(connStr)
	if err != nil {
		globalLogger.Info("Connection failed: connection=%s, duration=%s, error=%v",
			sanitized, duration, err)
	} else {
		globalLogger.Info("Connection succeeded: connection=%s, duration=%s",
			sanitized, duration)
	}
}

// LogConnectionDetails logs detailed connection information
func LogConnectionDetails(connStr string, poolConfig map[string]interface{}) {
	sanitized := sanitizeConnStr(connStr)
	configStr := ""
	for k, v := range poolConfig {
		configStr += fmt.Sprintf("%s=%v ", k, v)
	}
	globalLogger.Debug("Connection details: connection=%s, pool_config=%s",
		sanitized, strings.TrimSpace(configStr))
}

// LogMetadataLoad logs metadata loading operation
func LogMetadataLoad(connStr string, tableCount int, duration time.Duration, err error) {
	sanitized := sanitizeConnStr(connStr)
	if err != nil {
		globalLogger.Info("Metadata load failed: connection=%s, duration=%s, error=%v",
			sanitized, duration, err)
	} else {
		globalLogger.Info("Metadata loaded: connection=%s, table_count=%d, duration=%s",
			sanitized, tableCount, duration)
	}
}

// LogMetadataDetails logs detailed metadata loading information
func LogMetadataDetails(connStr string, schemaCount, tableCount, columnCount int) {
	sanitized := sanitizeConnStr(connStr)
	globalLogger.Debug("Metadata details: connection=%s, schema_count=%d, table_count=%d, column_count=%d",
		sanitized, schemaCount, tableCount, columnCount)
}

// LogQuery logs a database query execution
func LogQuery(query string, duration time.Duration, rowCount int, err error) {
	queryPreview := truncate(strings.TrimSpace(query), 100)
	if err != nil {
		globalLogger.Info("Query failed: query=%s, duration=%s, error=%v",
			queryPreview, duration, err)
	} else {
		globalLogger.Info("Query succeeded: query=%s, row_count=%d, duration=%s",
			queryPreview, rowCount, duration)
	}
}

// LogQueryDetails logs detailed query information
func LogQueryDetails(query string, args []interface{}) {
	queryPreview := truncate(strings.TrimSpace(query), 200)
	globalLogger.Debug("Starting query: query=%s, arg_count=%d",
		queryPreview, len(args))
}

// LogQueryTrace logs trace-level query information
func LogQueryTrace(query string, args []interface{}) {
	globalLogger.Trace("Query trace: query=%s, args=%v",
		strings.TrimSpace(query), args)
}

// LogPoolStats logs connection pool statistics
func LogPoolStats(connStr string, acquiredConns, idleConns, maxConns int32) {
	sanitized := sanitizeConnStr(connStr)
	globalLogger.Debug("Pool stats: connection=%s, acquired=%d, idle=%d, max=%d",
		sanitized, acquiredConns, idleConns, maxConns)
}

// sanitizeConnStr removes password from connection string for logging
func sanitizeConnStr(connStr string) string {
	// Handle postgres://user:password@host:port/database?params format
	// Find the scheme (postgres://)
	schemeIdx := strings.Index(connStr, "://")
	if schemeIdx == -1 {
		// No scheme found, return as is
		return connStr
	}

	scheme := connStr[:schemeIdx+3] // Include "://"
	rest := connStr[schemeIdx+3:]

	// Find the @ that separates credentials from host
	// We need to find the LAST @ before any / or ? to handle passwords with @
	hostSepIdx := -1
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == '@' {
			// Check if this @ is before any / or ?
			slashIdx := strings.Index(rest[i:], "/")
			questionIdx := strings.Index(rest[i:], "?")
			if (slashIdx == -1 || slashIdx > 0) && (questionIdx == -1 || questionIdx > 0) {
				hostSepIdx = i
				break
			}
		}
	}

	if hostSepIdx == -1 {
		// No @ found, no credentials
		return connStr
	}

	credentials := rest[:hostSepIdx]
	hostAndRest := rest[hostSepIdx+1:]

	// Find the : that separates user from password
	colonIdx := strings.Index(credentials, ":")
	if colonIdx == -1 {
		// No password, return as is
		return connStr
	}

	user := credentials[:colonIdx]
	// Password is everything after the first :
	return scheme + user + ":***@" + hostAndRest
}

// truncate truncates a string to maxLen characters, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
