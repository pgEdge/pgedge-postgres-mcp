/*-------------------------------------------------------------------------
 *
 * pgEdge MCP - Structured Logging
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	// currentLevel is the minimum log level to output
	// Default to ERROR to avoid cluttering CLI output with operational logs
	currentLevel = LevelError

	// Environment variable to control log level
	envLogLevel = "PGEDGE_MCP_LOG_LEVEL"
)

func init() {
	// Read log level from environment
	if level := os.Getenv(envLogLevel); level != "" {
		switch strings.ToLower(level) {
		case "debug":
			currentLevel = LevelDebug
		case "info":
			currentLevel = LevelInfo
		case "warn", "warning":
			currentLevel = LevelWarn
		case "error":
			currentLevel = LevelError
		}
	}
}

// levelString returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// logEntry represents a structured log entry
type logEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// log writes a structured log message if the level is enabled
func log(level LogLevel, message string, keyvals ...interface{}) {
	if level < currentLevel {
		return
	}

	entry := logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Message:   message,
		Fields:    make(map[string]interface{}),
	}

	// Parse key-value pairs
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			key := fmt.Sprintf("%v", keyvals[i])
			entry.Fields[key] = keyvals[i+1]
		}
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to marshal log entry: %v\n", err)
		return
	}

	// Write to stderr
	fmt.Fprintln(os.Stderr, string(jsonBytes))
}

// Debug logs a debug-level message with structured fields
func Debug(message string, keyvals ...interface{}) {
	log(LevelDebug, message, keyvals...)
}

// Info logs an info-level message with structured fields
func Info(message string, keyvals ...interface{}) {
	log(LevelInfo, message, keyvals...)
}

// Warn logs a warning-level message with structured fields
func Warn(message string, keyvals ...interface{}) {
	log(LevelWarn, message, keyvals...)
}

// Error logs an error-level message with structured fields
func Error(message string, keyvals ...interface{}) {
	log(LevelError, message, keyvals...)
}

// SetLevel sets the minimum log level to output
func SetLevel(level LogLevel) {
	currentLevel = level
}

// GetLevel returns the current minimum log level
func GetLevel() LogLevel {
	return currentLevel
}
