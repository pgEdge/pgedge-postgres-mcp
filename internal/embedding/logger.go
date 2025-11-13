/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package embedding

import (
    "fmt"
    "log"
    "os"
    "strings"
    "time"
)

// LogLevel represents the logging verbosity level
type LogLevel int

const (
    // LogLevelNone disables all LLM logging
    LogLevelNone LogLevel = iota
    // LogLevelInfo logs basic information (API calls, errors, token usage)
    LogLevelInfo
    // LogLevelDebug logs detailed information (text lengths, dimensions, timing, models)
    LogLevelDebug
    // LogLevelTrace logs very detailed information (full request/response details)
    LogLevelTrace
)

// Logger handles structured logging for LLM operations (embeddings, chat, SQL generation)
type Logger struct {
    level  LogLevel
    logger *log.Logger
}

var globalLogger *Logger

func init() {
    // Initialize logger with level from environment variable
    // Default is LogLevelNone (no logging) when unset or set to "none"
    levelStr := strings.ToLower(strings.TrimSpace(os.Getenv("PGEDGE_LLM_LOG_LEVEL")))

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
        logger: log.New(os.Stderr, "[LLM] ", log.LstdFlags),
    }
}

// SetLogLevel sets the global embedding log level
func SetLogLevel(level LogLevel) {
    globalLogger.level = level
}

// GetLogLevel returns the current log level
func GetLogLevel() LogLevel {
    return globalLogger.level
}

// Info logs an informational message (API calls, errors)
func (l *Logger) Info(format string, args ...interface{}) {
    if l.level >= LogLevelInfo {
        l.logger.Printf("[INFO] "+format, args...)
    }
}

// Debug logs a debug message (text lengths, dimensions, timing)
func (l *Logger) Debug(format string, args ...interface{}) {
    if l.level >= LogLevelDebug {
        l.logger.Printf("[DEBUG] "+format, args...)
    }
}

// Trace logs a trace message (full request/response details)
func (l *Logger) Trace(format string, args ...interface{}) {
    if l.level >= LogLevelTrace {
        l.logger.Printf("[TRACE] "+format, args...)
    }
}

// LogAPICall logs an embedding API call with timing
func LogAPICall(provider, model string, textLen int, duration time.Duration, dimensions int, err error) {
    if err != nil {
        globalLogger.Info("API call failed: provider=%s, model=%s, text_length=%d, duration=%s, error=%v",
            provider, model, textLen, duration, err)
    } else {
        globalLogger.Info("API call succeeded: provider=%s, model=%s, text_length=%d, dimensions=%d, duration=%s",
            provider, model, textLen, dimensions, duration)
    }
}

// LogAPICallDetails logs detailed information about an API call
func LogAPICallDetails(provider, model, url string, textLen int) {
    globalLogger.Debug("Starting API call: provider=%s, model=%s, url=%s, text_length=%d",
        provider, model, url, textLen)
}

// LogRequestTrace logs trace-level request information
func LogRequestTrace(provider, model string, textPreview string) {
    globalLogger.Trace("Request details: provider=%s, model=%s, text_preview=%s",
        provider, model, truncate(textPreview, 100))
}

// LogResponseTrace logs trace-level response information
func LogResponseTrace(provider, model string, statusCode int, dimensions int) {
    globalLogger.Trace("Response details: provider=%s, model=%s, status_code=%d, dimensions=%d",
        provider, model, statusCode, dimensions)
}

// LogRateLimitError logs rate limit errors with specific details
func LogRateLimitError(provider, model string, statusCode int, responseBody string) {
    globalLogger.Info("RATE LIMIT ERROR: provider=%s, model=%s, status_code=%d, response=%s",
        provider, model, statusCode, truncate(responseBody, 200))
}

// LogConnectionError logs connection errors
func LogConnectionError(provider, url string, err error) {
    globalLogger.Info("Connection failed: provider=%s, url=%s, error=%v",
        provider, url, err)
}

// LogProviderInit logs provider initialization
func LogProviderInit(provider, model string, config map[string]string) {
    if globalLogger.level >= LogLevelDebug {
        configStr := ""
        for k, v := range config {
            if k == "api_key" {
                v = "***REDACTED***"
            }
            configStr += fmt.Sprintf("%s=%s ", k, v)
        }
        globalLogger.Debug("Provider initialized: provider=%s, model=%s, config=%s",
            provider, model, strings.TrimSpace(configStr))
    }
}

// LogLLMCall logs an LLM chat API call with token usage and timing
func LogLLMCall(provider, model, operation string, inputTokens, outputTokens int, duration time.Duration, err error) {
    if err != nil {
        globalLogger.Info("LLM call failed: provider=%s, model=%s, operation=%s, duration=%s, error=%v",
            provider, model, operation, duration, err)
    } else {
        globalLogger.Info("LLM call succeeded: provider=%s, model=%s, operation=%s, input_tokens=%d, output_tokens=%d, total_tokens=%d, duration=%s",
            provider, model, operation, inputTokens, outputTokens, inputTokens+outputTokens, duration)
    }
}

// LogLLMCallDetails logs detailed information about an LLM API call
func LogLLMCallDetails(provider, model, operation, url string, messageCount int) {
    globalLogger.Debug("Starting LLM call: provider=%s, model=%s, operation=%s, url=%s, message_count=%d",
        provider, model, operation, url, messageCount)
}

// LogLLMRequestTrace logs trace-level LLM request information
func LogLLMRequestTrace(provider, model, operation string, requestPreview string) {
    globalLogger.Trace("LLM request details: provider=%s, model=%s, operation=%s, request_preview=%s",
        provider, model, operation, truncate(requestPreview, 200))
}

// LogLLMResponseTrace logs trace-level LLM response information
func LogLLMResponseTrace(provider, model, operation string, statusCode int, stopReason string) {
    globalLogger.Trace("LLM response details: provider=%s, model=%s, operation=%s, status_code=%d, stop_reason=%s",
        provider, model, operation, statusCode, stopReason)
}

// truncate truncates a string to maxLen characters, adding "..." if truncated
func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + "..."
}
