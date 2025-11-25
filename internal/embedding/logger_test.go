/*-------------------------------------------------------------------------
*
 * pgEdge Natural Language Agent
*
* Portions copyright (c) 2025, pgEdge, Inc.
* This software is released under The PostgreSQL License
*
*-------------------------------------------------------------------------
*/

package embedding

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLogLevel_None(t *testing.T) {
	// Create a logger with LogLevelNone
	var buf bytes.Buffer
	testLogger := &Logger{
		level:  LogLevelNone,
		logger: log.New(&buf, "[LLM] ", log.LstdFlags),
	}

	// Try logging at all levels - nothing should be logged
	testLogger.Info("This is an info message")
	testLogger.Debug("This is a debug message")
	testLogger.Trace("This is a trace message")

	// Buffer should be empty
	if buf.Len() > 0 {
		t.Errorf("Expected no output with LogLevelNone, got: %s", buf.String())
	}
}

func TestLogLevel_Info(t *testing.T) {
	// Create a logger with LogLevelInfo
	var buf bytes.Buffer
	testLogger := &Logger{
		level:  LogLevelInfo,
		logger: log.New(&buf, "[LLM] ", 0), // 0 flags for predictable output
	}

	// Info should be logged
	testLogger.Info("This is an info message")
	if !strings.Contains(buf.String(), "[INFO] This is an info message") {
		t.Errorf("Expected info message to be logged, got: %s", buf.String())
	}

	// Debug and Trace should not be logged
	buf.Reset()
	testLogger.Debug("This is a debug message")
	testLogger.Trace("This is a trace message")
	if buf.Len() > 0 {
		t.Errorf("Expected no debug/trace output with LogLevelInfo, got: %s", buf.String())
	}
}

func TestLogLevel_Debug(t *testing.T) {
	// Create a logger with LogLevelDebug
	var buf bytes.Buffer
	testLogger := &Logger{
		level:  LogLevelDebug,
		logger: log.New(&buf, "[LLM] ", 0),
	}

	// Info and Debug should be logged
	testLogger.Info("This is an info message")
	testLogger.Debug("This is a debug message")
	output := buf.String()
	if !strings.Contains(output, "[INFO] This is an info message") {
		t.Errorf("Expected info message to be logged, got: %s", output)
	}
	if !strings.Contains(output, "[DEBUG] This is a debug message") {
		t.Errorf("Expected debug message to be logged, got: %s", output)
	}

	// Trace should not be logged
	buf.Reset()
	testLogger.Trace("This is a trace message")
	if buf.Len() > 0 {
		t.Errorf("Expected no trace output with LogLevelDebug, got: %s", buf.String())
	}
}

func TestLogLevel_Trace(t *testing.T) {
	// Create a logger with LogLevelTrace
	var buf bytes.Buffer
	testLogger := &Logger{
		level:  LogLevelTrace,
		logger: log.New(&buf, "[LLM] ", 0),
	}

	// All levels should be logged
	testLogger.Info("This is an info message")
	testLogger.Debug("This is a debug message")
	testLogger.Trace("This is a trace message")
	output := buf.String()

	if !strings.Contains(output, "[INFO] This is an info message") {
		t.Errorf("Expected info message to be logged, got: %s", output)
	}
	if !strings.Contains(output, "[DEBUG] This is a debug message") {
		t.Errorf("Expected debug message to be logged, got: %s", output)
	}
	if !strings.Contains(output, "[TRACE] This is a trace message") {
		t.Errorf("Expected trace message to be logged, got: %s", output)
	}
}

func TestSetLogLevel(t *testing.T) {
	// Save original logger
	original := globalLogger

	// Create test logger
	var buf bytes.Buffer
	globalLogger = &Logger{
		level:  LogLevelNone,
		logger: log.New(&buf, "[LLM] ", 0),
	}

	// Set to Info level
	SetLogLevel(LogLevelInfo)
	if globalLogger.level != LogLevelInfo {
		t.Errorf("Expected LogLevelInfo, got %v", globalLogger.level)
	}

	// Verify Info is logged
	globalLogger.Info("Test message")
	if !strings.Contains(buf.String(), "[INFO] Test message") {
		t.Errorf("Expected info message after SetLogLevel, got: %s", buf.String())
	}

	// Restore original logger
	globalLogger = original
}

func TestLogAPICall(t *testing.T) {
	// Save original logger
	original := globalLogger

	// Create test logger with Info level
	var buf bytes.Buffer
	globalLogger = &Logger{
		level:  LogLevelInfo,
		logger: log.New(&buf, "[LLM] ", 0),
	}

	// Test successful API call
	duration := 100 * time.Millisecond
	LogAPICall("openai", "text-embedding-3-small", 50, duration, 1536, nil)
	output := buf.String()

	if !strings.Contains(output, "API call succeeded") {
		t.Errorf("Expected success message, got: %s", output)
	}
	if !strings.Contains(output, "provider=openai") {
		t.Errorf("Expected provider info, got: %s", output)
	}

	// Test failed API call
	buf.Reset()
	LogAPICall("openai", "text-embedding-3-small", 50, duration, 0, os.ErrInvalid)
	output = buf.String()

	if !strings.Contains(output, "API call failed") {
		t.Errorf("Expected failure message, got: %s", output)
	}

	// Restore original logger
	globalLogger = original
}

func TestLogLLMCall(t *testing.T) {
	// Save original logger
	original := globalLogger

	// Create test logger with Info level
	var buf bytes.Buffer
	globalLogger = &Logger{
		level:  LogLevelInfo,
		logger: log.New(&buf, "[LLM] ", 0),
	}

	// Test successful LLM call
	duration := 200 * time.Millisecond
	LogLLMCall("anthropic", "claude-sonnet-4", "chat", 100, 50, duration, nil)
	output := buf.String()

	if !strings.Contains(output, "LLM call succeeded") {
		t.Errorf("Expected success message, got: %s", output)
	}
	if !strings.Contains(output, "input_tokens=100") {
		t.Errorf("Expected input tokens, got: %s", output)
	}
	if !strings.Contains(output, "output_tokens=50") {
		t.Errorf("Expected output tokens, got: %s", output)
	}
	if !strings.Contains(output, "total_tokens=150") {
		t.Errorf("Expected total tokens, got: %s", output)
	}

	// Test failed LLM call
	buf.Reset()
	LogLLMCall("anthropic", "claude-sonnet-4", "chat", 0, 0, duration, os.ErrInvalid)
	output = buf.String()

	if !strings.Contains(output, "LLM call failed") {
		t.Errorf("Expected failure message, got: %s", output)
	}

	// Restore original logger
	globalLogger = original
}

func TestLogLLMCallDetails_OnlyAtDebugLevel(t *testing.T) {
	// Save original logger
	original := globalLogger

	// Test at Info level - should not log
	var buf bytes.Buffer
	globalLogger = &Logger{
		level:  LogLevelInfo,
		logger: log.New(&buf, "[LLM] ", 0),
	}

	LogLLMCallDetails("anthropic", "claude-sonnet-4", "chat", "https://api.anthropic.com", 3)
	if buf.Len() > 0 {
		t.Errorf("Expected no output at Info level, got: %s", buf.String())
	}

	// Test at Debug level - should log
	buf.Reset()
	globalLogger.level = LogLevelDebug
	LogLLMCallDetails("anthropic", "claude-sonnet-4", "chat", "https://api.anthropic.com", 3)
	if !strings.Contains(buf.String(), "Starting LLM call") {
		t.Errorf("Expected debug message at Debug level, got: %s", buf.String())
	}

	// Restore original logger
	globalLogger = original
}

func TestLogLLMResponseTrace_OnlyAtTraceLevel(t *testing.T) {
	// Save original logger
	original := globalLogger

	// Test at Debug level - should not log
	var buf bytes.Buffer
	globalLogger = &Logger{
		level:  LogLevelDebug,
		logger: log.New(&buf, "[LLM] ", 0),
	}

	LogLLMResponseTrace("anthropic", "claude-sonnet-4", "chat", 200, "end_turn")
	if buf.Len() > 0 {
		t.Errorf("Expected no output at Debug level, got: %s", buf.String())
	}

	// Test at Trace level - should log
	buf.Reset()
	globalLogger.level = LogLevelTrace
	LogLLMResponseTrace("anthropic", "claude-sonnet-4", "chat", 200, "end_turn")
	if !strings.Contains(buf.String(), "LLM response details") {
		t.Errorf("Expected trace message at Trace level, got: %s", buf.String())
	}

	// Restore original logger
	globalLogger = original
}
